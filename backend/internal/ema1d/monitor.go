package ema1d

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/scheduler"
)

const (
	marginLevelKey     = "ema1d:margin_level"
	alertEvery         = time.Hour
	syncExpectedByHour = 12 // Asia/Ho_Chi_Minh
)

var vnTZ = mustLoadLocation("Asia/Ho_Chi_Minh")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}

// MarginReading is the last cross margin level seen by the monitor.
type MarginReading struct {
	Value *money.Dec `json:"value"`
	At    *time.Time `json:"at"`
}

func (s *Service) lastMarginLevel(ctx context.Context) MarginReading {
	row, err := dbq.New(s.DB.Read).GetState(ctx, dbq.GetStateParams{Key: marginLevelKey, ExpiresAt: tptr(s.now())})
	if err != nil {
		return MarginReading{}
	}
	var m MarginReading
	_ = json.Unmarshal(row.Value, &m)
	return m
}

// notifyOnce sends msg at most once per ttl for key.
func (s *Service) notifyOnce(ctx context.Context, key, msg string, ttl time.Duration) {
	now := s.now()
	key = "ema1d:notified:" + key
	first := false
	err := s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		_, err := q.GetState(ctx, dbq.GetStateParams{Key: key, ExpiresAt: tptr(now)})
		if err == nil || !isNoRows(err) {
			return err
		}
		first = true
		return q.SetState(ctx, dbq.SetStateParams{Key: key, Value: dbtype.JSON("1"), ExpiresAt: tptr(now.Add(ttl))})
	})
	if err == nil && first {
		s.notify(ctx, msg)
	}
}

// syncMissingToday reports that the current UTC candle date has no real sync although it is past
// noon in Vietnam.
func (s *Service) syncMissingToday(ctx context.Context) bool {
	now := s.now()
	if now.In(vnTZ).Hour() < syncExpectedByHour {
		return false
	}
	n, err := dbq.New(s.DB.Read).CountRealSyncsForDate(ctx, dateOnly(now))
	return err == nil && n == 0
}

// Monitor runs every 15 minutes: prices and unrealized PnL of open positions, the cross margin
// level (alerts at most once an hour per threshold), and a reminder when no sync came by noon.
func (s *Service) Monitor(ctx context.Context) (string, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return "", err
	}
	rows, err := dbq.New(s.DB.Read).ListActivePositions(ctx)
	if err != nil {
		return "", err
	}
	var open []dbq.ListActivePositionsRow
	for _, r := range rows {
		if r.Ema1dPosition.Status == PosOpen {
			open = append(open, r)
		}
	}
	if len(open) == 0 && !cfg.Enabled {
		return "", scheduler.Skipf("no open position and EMA1D disabled")
	}

	prices := map[string]money.Dec{}
	if len(open) > 0 {
		if prices, err = s.Binance.TickerPrices(ctx); err != nil {
			return "", err
		}
	}
	acc, err := s.Binance.CrossMarginAccount(ctx)
	if err != nil {
		return "", err
	}
	now := s.now()
	reading := MarginReading{At: &now}
	if acc.MarginLevel.IsPositive() {
		lvl := acc.MarginLevel
		reading.Value = &lvl
	}
	raw, _ := json.Marshal(reading)
	q := dbq.New(s.DB.Write)
	if err := q.SetState(ctx, dbq.SetStateParams{Key: marginLevelKey, Value: dbtype.JSON(raw)}); err != nil {
		return "", err
	}

	for _, r := range open {
		price, ok := prices[r.PairName]
		if !ok {
			continue
		}
		if err := q.SetPairPrice(ctx, dbq.SetPairPriceParams{CurrentPrice: &price, PriceUpdatedAt: tptr(now), ID: r.Ema1dPosition.PairID}); err != nil {
			return "", err
		}
		pos := r.Ema1dPosition
		interest := money.Zero
		if pos.Side == SideShort {
			interest = acc.Asset(r.CoinName).Interest
		}
		pos.UnrealizedProfit = unrealized(pos, price, interest).Round(8)
		if err := savePosition(ctx, q, pos); err != nil {
			return "", err
		}
	}

	if len(open) > 0 && reading.Value != nil {
		lvl := *reading.Value
		switch {
		case cfg.MarginLevelAlert.IsPositive() && lvl.LessThan(cfg.MarginLevelAlert):
			s.notifyOnce(ctx, "margin_alert", fmt.Sprintf("EMA1D ALERT: cross margin level %s < %s — short squeeze / liquidation risk", lvl.Round(3), cfg.MarginLevelAlert), alertEvery)
		case cfg.MarginLevelWarn.IsPositive() && lvl.LessThan(cfg.MarginLevelWarn):
			s.notifyOnce(ctx, "margin_warn", fmt.Sprintf("EMA1D warning: cross margin level %s < %s", lvl.Round(3), cfg.MarginLevelWarn), alertEvery)
		}
	}
	if cfg.Enabled && s.syncMissingToday(ctx) {
		today := dateOnly(now)
		s.notifyOnce(ctx, "sync_missing:"+today, fmt.Sprintf("EMA1D: chưa có sync cho %s lúc %d:00 VN — daily check có chạy không?", today, syncExpectedByHour), 24*time.Hour)
	}
	s.changed()
	return fmt.Sprintf("%d open position(s), margin level %v", len(open), reading.Value), nil
}

// unrealized is the PnL of an open position at price, on the same terms as the realized PnL (open
// fee already paid, borrow interest owed so far); the close fee is not known yet.
func unrealized(pos dbq.Ema1dPosition, price, interestCoin money.Dec) money.Dec {
	gross := price.Sub(deref(pos.OpenPrice)).Mul(pos.Amount)
	if pos.Side == SideShort {
		gross = gross.Neg()
	}
	return gross.Sub(pos.FeeUsdt).Sub(interestCoin.Mul(price))
}

func cost(pos dbq.Ema1dPosition) money.Dec { return deref(pos.OpenPrice).Mul(pos.Amount) }

// slippagePercent compares a fill with the strategy price, signed so that positive is always worse
// for us: paid more on a buy, received less on a sell.
func slippagePercent(price, ref *money.Dec, selling bool) *money.Dec {
	if price == nil || ref == nil || !price.IsPositive() || !ref.IsPositive() {
		return nil
	}
	change := price.Div(*ref).Sub(money.One).Mul(money.FromInt(100))
	if selling {
		change = change.Neg()
	}
	r := change.Round(3)
	return &r
}

// Snapshot records the strategy capital marked to market, for the equity curve. Runs daily.
func (s *Service) Snapshot(ctx context.Context) (string, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return "", err
	}
	q := dbq.New(s.DB.Read)
	rows, err := q.ListActivePositions(ctx)
	if err != nil {
		return "", err
	}
	equity, err := s.equity(ctx, q, cfg)
	if err != nil {
		return "", err
	}
	long, short := money.Zero, money.Zero
	for _, r := range rows {
		if r.Ema1dPosition.Status != PosOpen {
			continue
		}
		equity = equity.Add(r.Ema1dPosition.UnrealizedProfit)
		v := r.Ema1dPosition.Amount.Mul(deref(r.CurrentPrice))
		if r.Ema1dPosition.Side == SideLong {
			long = long.Add(v)
		} else {
			short = short.Add(v)
		}
	}
	date := dateOnly(s.now())
	err = dbq.New(s.DB.Write).UpsertSnapshot(ctx, dbq.UpsertSnapshotParams{
		Date: date, EquityUsdt: equity.Round(8), MarginLevel: s.lastMarginLevel(ctx).Value,
		OpenLongNotional: long.Round(8), OpenShortNotional: short.Round(8),
	})
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("%s equity %s USDT", date, equity.Round(2))
	if checks, err := s.CheckPairs(ctx, nil); err != nil {
		msg += "; pair check failed: " + err.Error()
	} else {
		msg += fmt.Sprintf("; %d pair(s) checked", len(checks))
	}
	return msg, nil
}

var errNoRows = sql.ErrNoRows

func isNoRows(err error) bool { return errors.Is(err, errNoRows) }
