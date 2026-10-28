package emasettings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/notify"
	"bigwaves/internal/platform/scheduler"
)

// klineCount candles feed the averages: the seed no longer matters after a few dozen.
const klineCount = 300

func openOrderID(id int64) string  { return fmt.Sprintf("bw_es_%d_open", id) }
func closeOrderID(id int64) string { return fmt.Sprintf("bw_es_%d_close", id) }

func viewOf(r dbq.GetEmaSettingRow) pairView {
	return pairView{ID: r.EmaSetting.PairID, Name: r.PairName, Coin: r.CoinName, Status: r.PairStatus, Price: r.CurrentPrice,
		StepSize: r.StepSize, MinNotional: r.MinNotional, IsSpot: r.IsSpot, IsIsolated: r.IsIsolated}
}

func pct(d money.Dec) float64 { return toFloat(d) }

// Check is the 5-minute scan of every ACTIVE setting. A failing setting is reported and does not
// stop the others.
func (s *Service) Check(ctx context.Context) (string, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return "", err
	}
	if !cfg.Enabled {
		return "", scheduler.Skipf("EMA Settings is off")
	}
	rows, err := dbq.New(s.DB.Read).ListEmaSettings(ctx, StatusActive)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", scheduler.Skipf("no ACTIVE setting")
	}
	var opened, closed int
	var errs []error
	for _, r := range rows {
		row := dbq.GetEmaSettingRow(r)
		o, c, err := s.checkOne(ctx, row, cfg)
		opened, closed = opened+o, closed+c
		if err != nil {
			if errors.Is(err, binance.ErrBanned) || ctx.Err() != nil {
				return "", err
			}
			errs = append(errs, fmt.Errorf("#%d %s: %w", row.EmaSetting.ID, row.PairName, err))
		}
	}
	s.changed()
	msg := fmt.Sprintf("%d setting(s), %d opened, %d closed", len(rows), opened, closed)
	if len(errs) > 0 {
		return msg, fmt.Errorf("%s; %d failed: %w", msg, len(errs), errors.Join(errs...))
	}
	return msg, nil
}

func (s *Service) checkOne(ctx context.Context, row dbq.GetEmaSettingRow, cfg Config) (opened, closed int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, pv := row.EmaSetting, viewOf(row)
	closes, price, err := s.closes(ctx, pv.Name, set.Period)
	if err != nil {
		return 0, 0, err
	}
	r := ReadingFrom(closes, set.CheckType)
	window := EMAWindow(set.CheckType, set.Period, s.now())
	p := cfg.Params(set.Period)
	openPct, closePct, ignorePct := pct(p.OpenPercent), pct(p.ClosePercent), pct(p.IgnoreOverEmaPercent)
	var errs []error

	// Phase 1: OPEN → CLOSED on stop loss, target or the cross back.
	trades, err := s.trades(ctx, set.ID)
	if err != nil {
		return 0, 0, err
	}
	for _, t := range trades {
		if t.Status != TradeOpen {
			continue
		}
		reason := CloseReason(t.Side, price, t.SlPrice, t.TargetPrice, r, closePct, window)
		if reason == "" {
			continue
		}
		if err := s.closeTrade(ctx, &t, pv, reason); err != nil {
			errs = append(errs, s.failed(ctx, &t, "close", err))
			continue
		}
		closed++
		s.notify(ctx, fmt.Sprintf("EMA %s: %s AUTO CLOSED %s #%d (reason=%s) @ %s · PnL %s USDT",
			set.Period, pv.Name, t.Side, t.ID, reason, deref(t.ClosePrice).Round(8), t.RealizedProfit.Round(2)))
	}

	// Phase 2: nothing held or waiting → a new trade on the side of the cross.
	if set.NextTradeType == NewTrade {
		if trades, err = s.trades(ctx, set.ID); err != nil {
			return opened, closed, err
		}
		if !hasActive(trades) {
			if side := EntrySide(set.TradeType, toFloat(price), r, openPct, ignorePct, window); side != "" {
				amount := money.Zero
				if price.IsPositive() {
					amount = money.FloorToStep(set.Budget.Div(price), pv.step())
				}
				if _, err := s.createTrade(ctx, set, newTrade{Side: side, Status: TradeOpening, Amount: amount, Auto: true}); err != nil {
					return opened, closed, err
				}
			}
		}
	}

	// Phase 3: OPENING → OPEN when the price passed the entry or the cross holds.
	if trades, err = s.trades(ctx, set.ID); err != nil {
		return opened, closed, err
	}
	for _, t := range trades {
		if t.Status != TradeOpening || !ShouldOpen(t.Side, t.EntryPrice, price, r, openPct, ignorePct, window) {
			continue
		}
		if err := s.openTrade(ctx, set, &t, pv, cfg); err != nil {
			errs = append(errs, s.failed(ctx, &t, "open", err))
			continue
		}
		opened++
		s.notify(ctx, fmt.Sprintf("EMA %s: %s AUTO OPENED %s #%d @ %s · amount %s · SL %s",
			set.Period, pv.Name, t.Side, t.ID, deref(t.OpenPrice).Round(8), t.Amount, deref(t.SlPrice)))
	}

	// Phase 4: mark the OPEN trades to market.
	if trades, err = s.trades(ctx, set.ID); err != nil {
		return opened, closed, err
	}
	q := dbq.New(s.DB.Write)
	for i := range trades { // by index: phase 5 reads the updated PnL
		t := &trades[i]
		if t.Status != TradeOpen || t.OpenPrice == nil {
			continue
		}
		if u := Unrealized(t.Side, *t.OpenPrice, price, t.Amount, t.FeeUsdt); !u.Equal(t.UnrealizedProfit) {
			t.UnrealizedProfit = u
			if err := saveTrade(ctx, q, *t); err != nil {
				return opened, closed, err
			}
		}
	}

	// Phase 5: the setting's own take profit / stop loss on its total PnL.
	if hit := SettingHit(set.TpPercent, set.SlPercent, Summarize(set.Budget, trades).TotalPercent); hit != "" {
		if err := s.closeSetting(ctx, set, pv, hit); err != nil {
			errs = append(errs, fmt.Errorf("close setting (%s): %w", hit, err))
		}
	}
	return opened, closed, errors.Join(errs...)
}

// failed keeps the error on the trade and alerts (throttled per trade).
func (s *Service) failed(ctx context.Context, t *dbq.EmaSettingTrade, action string, err error) error {
	t.LastError = err.Error()
	_ = saveTrade(ctx, dbq.New(s.DB.Write), *t)
	slog.ErrorContext(ctx, "emasettings", "trade", t.ID, "action", action, "err", err)
	if s.Alerter != nil {
		s.Alerter.Report(ctx, fmt.Sprintf("emasettings trade %d", t.ID), fmt.Errorf("EMA Settings trade #%d %s failed: %w", t.ID, action, err))
	}
	return fmt.Errorf("trade #%d %s: %w", t.ID, action, err)
}

// closeSetting closes a whole setting: waiting trades are cancelled (nothing on the exchange),
// held ones are closed at market one by one, then the setting is DONE. A failed close leaves the
// setting as it is, to be tried again; orders already filled stay booked (no DB transaction
// around exchange calls, unlike Rails).
func (s *Service) closeSetting(ctx context.Context, set dbq.EmaSetting, pv pairView, reason string) error {
	trades, err := s.trades(ctx, set.ID)
	if err != nil {
		return err
	}
	q := dbq.New(s.DB.Write)
	var errs []error
	for _, t := range trades {
		switch t.Status {
		case TradeOpening:
			if err := cancelTrade(ctx, q, &t, s.now()); err != nil {
				errs = append(errs, err)
			}
		case TradeOpen:
			if err := s.closeTrade(ctx, &t, pv, reason); err != nil {
				errs = append(errs, s.failed(ctx, &t, "close", err))
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if err := q.SetEmaSettingStatus(ctx, dbq.SetEmaSettingStatusParams{Status: StatusDone, ID: set.ID}); err != nil {
		return err
	}
	if trades, err = s.trades(ctx, set.ID); err != nil {
		return err
	}
	sum := Summarize(set.Budget, trades)
	s.notify(ctx, fmt.Sprintf("EMA %s: %s setting #%d CLOSED (reason=%s, total PnL %s USDT)",
		set.Period, pv.Name, set.ID, reason, sum.Total.Round(2)))
	return nil
}

// cancelTrade drops an OPENING trade without touching the exchange. One whose short borrow was
// sent cannot be dropped: the coin may be borrowed and not sold.
func cancelTrade(ctx context.Context, q *dbq.Queries, t *dbq.EmaSettingTrade, now time.Time) error {
	if t.BorrowStep != "" {
		return refused("trade #%d: the %s borrow was sent (%s); open it or repay by hand first", t.ID, t.Side, t.BorrowStep)
	}
	t.Status, t.CloseReason, t.UnrealizedProfit = TradeClosed, ReasonCancelled, money.Zero
	t.CloseAt = ptr(dbtype.NewTime(now))
	return saveTrade(ctx, q, *t)
}

func hasActive(trades []dbq.EmaSettingTrade) bool {
	for _, t := range trades {
		if t.Status == TradeOpening || t.Status == TradeOpen {
			return true
		}
	}
	return false
}

func (s *Service) trades(ctx context.Context, settingID int64) ([]dbq.EmaSettingTrade, error) {
	return dbq.New(s.DB.Read).ListEmaSettingTrades(ctx, settingID)
}

// closes returns the closes of the period's last klineCount candles up to the running one, oldest
// first, and the running close (the current price).
func (s *Service) closes(ctx context.Context, symbol, period string) ([]float64, money.Dec, error) {
	interval, step := "1d", 24*time.Hour
	if period == PeriodH4 {
		interval, step = "4h", 4*time.Hour
	}
	running := s.now().Truncate(step)
	ks, err := s.Binance.Klines(ctx, symbol, interval, running.Add(-time.Duration(klineCount-1)*step), klineCount)
	if err != nil {
		return nil, money.Zero, err
	}
	for len(ks) > 0 && ks[len(ks)-1].OpenTime.After(running) {
		ks = ks[:len(ks)-1]
	}
	if len(ks) == 0 {
		return nil, money.Zero, fmt.Errorf("no %s candle for %s", period, symbol)
	}
	if ks[len(ks)-1].OpenTime.Before(running) {
		return nil, money.Zero, fmt.Errorf("no %s candle since %s UTC (trading halted?)", period, ks[len(ks)-1].OpenTime.Format("2006-01-02 15:04"))
	}
	out := make([]float64, len(ks))
	for i, k := range ks {
		out[i] = toFloat(k.Close)
	}
	return out, ks[len(ks)-1].Close, nil
}

func (s *Service) notify(ctx context.Context, msg string) {
	if err := s.Notifier.Send(ctx, notify.Orders, msg); err != nil {
		slog.ErrorContext(ctx, "emasettings notify", "err", err)
	}
}

// Summary is the PnL of a setting: realized over every trade, unrealized over the held ones.
type Summary struct {
	Realized, Unrealized, Total money.Dec
	// TotalPercent of the budget; nil without a budget.
	TotalPercent *money.Dec
}

func Summarize(budget money.Dec, trades []dbq.EmaSettingTrade) Summary {
	var realized, unrealized []money.Dec
	for _, t := range trades {
		realized = append(realized, t.RealizedProfit)
		if t.Status == TradeOpen {
			unrealized = append(unrealized, t.UnrealizedProfit)
		}
	}
	sum := Summary{Realized: money.Sum(realized...), Unrealized: money.Sum(unrealized...)}
	sum.Total = sum.Realized.Add(sum.Unrealized)
	if budget.IsPositive() {
		sum.TotalPercent = ptr(sum.Total.Div(budget).Mul(money.FromInt(100)).Round(4))
	}
	return sum
}
