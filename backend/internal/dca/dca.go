// Package dca runs DCA ladders: on one pair the user sets each rung (buy at, sell at, amount) and
// the bot only watches the price and fills MARKET orders. Ported from app/services/dca_* with
// three changes: the trigger prices are kept apart from the fill prices, fees are charged to the
// PnL (and the sell only sells the coin really held), and an optional capital cap.
package dca

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/events"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/notify"
	"bigwaves/internal/platform/scheduler"
)

const StrategyKey = "dca"

const (
	SettingActive   = "ACTIVE"
	SettingInactive = "INACTIVE"
	SettingDone     = "DONE"

	TradeOpening = "OPENING" // waiting for price <= buy_price
	TradeOpen    = "OPEN"    // held, waiting for price >= sell_price
	TradeClosed  = "CLOSED"
)

// Config of the DCA strategy.
type Config struct {
	// Enabled lets the price check place orders. Off: ladders are kept but nothing fills.
	Enabled bool `json:"enabled"`
	// MaxCapitalUSDT caps the USDT held in open rungs across all ladders (0 = no cap).
	MaxCapitalUSDT money.Dec `json:"maxCapitalUsdt"`
}

func DefaultConfig() Config { return Config{Enabled: true} }

func (c Config) Validate() error {
	if c.MaxCapitalUSDT.IsNegative() {
		return errors.New("maxCapitalUsdt must not be negative")
	}
	return nil
}

type Service struct {
	DB       *db.DB
	Binance  *binance.Client
	Configs  *configstore.Store
	Notifier notify.Notifier
	Alerter  *notify.Alerter
	Events   *events.Broker
	Now      func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) Config(ctx context.Context) (Config, int64, error) {
	c := DefaultConfig()
	v, err := s.Configs.Load(ctx, StrategyKey, &c)
	return c, v, err
}

func (s *Service) SaveConfig(ctx context.Context, version int64, c Config, user string) (int64, error) {
	if err := c.Validate(); err != nil {
		return 0, httpx.Validation(err.Error(), nil)
	}
	v, err := s.Configs.Save(ctx, StrategyKey, version, c, user)
	if err == nil {
		s.changed()
	}
	return v, err
}

func (s *Service) changed() { s.Events.Publish("dca.updated", nil) }

// settingView is a ladder with what it needs from its pair.
type settingView struct {
	dbq.DcaSetting
	PairName     string
	CurrentPrice *money.Dec
	StepSize     *money.Dec
	MinNotional  *money.Dec
	IsSpot       bool
	IsIsolated   bool
}

func (v settingView) account() binance.Account {
	switch {
	case v.IsSpot:
		return binance.Spot
	case v.IsIsolated:
		return binance.Isolated
	}
	return binance.Cross
}

func (v settingView) baseAsset() string { return strings.TrimSuffix(v.PairName, "USDT") }

func (s *Service) getSetting(ctx context.Context, q *dbq.Queries, id int64) (settingView, error) {
	r, err := q.GetDcaSetting(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return settingView{}, httpx.NewError(404, "dca_not_found", "không tìm thấy DCA")
	}
	if err != nil {
		return settingView{}, err
	}
	return settingView{r.DcaSetting, r.PairName, r.CurrentPrice, r.StepSize, r.MinNotional, r.IsSpot, r.IsIsolated}, nil
}

func openOrderID(id int64) string  { return fmt.Sprintf("bw_dca_%d_open", id) }
func closeOrderID(id int64) string { return fmt.Sprintf("bw_dca_%d_close", id) }

func saveTrade(ctx context.Context, q *dbq.Queries, t dbq.DcaTrade) error {
	return q.SaveDcaTrade(ctx, dbq.SaveDcaTradeParams{
		Status: t.Status, BuyPrice: t.BuyPrice, SellPrice: t.SellPrice, Amount: t.Amount, HeldAmount: t.HeldAmount,
		OpenPrice: t.OpenPrice, ClosePrice: t.ClosePrice, BuyQuote: t.BuyQuote, SellQuote: t.SellQuote, FeeUsdt: t.FeeUsdt,
		ExtraFeeUsdt: t.ExtraFeeUsdt, RealizedProfit: t.RealizedProfit, UnrealizedProfit: t.UnrealizedProfit,
		LastError: t.LastError, OpenAt: t.OpenAt, CloseAt: t.CloseAt, ID: t.ID,
	})
}

// ------------------------------------------------------------------------------- price check

// Check is the 5-minute price check: sells first (frees capital), then buys, then marks the open
// rungs to market. Each rung is independent: one failure does not stop the others.
func (s *Service) Check(ctx context.Context) (string, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return "", err
	}
	if !cfg.Enabled {
		return "", scheduler.Skipf("DCA is disabled")
	}
	q := dbq.New(s.DB.Read)
	settings, err := q.ListDcaSettings(ctx, SettingActive)
	if err != nil {
		return "", err
	}
	sold, bought := 0, 0
	var failures []string
	for _, r := range settings {
		v := settingView{r.DcaSetting, r.PairName, r.CurrentPrice, r.StepSize, r.MinNotional, r.IsSpot, r.IsIsolated}
		if v.CurrentPrice == nil {
			continue
		}
		price := *v.CurrentPrice
		trades, err := q.ListDcaTrades(ctx, v.ID)
		if err != nil {
			return "", err
		}
		for _, t := range trades {
			if t.Status == TradeOpen && t.SellPrice.IsPositive() && price.GreaterOrEqual(t.SellPrice) {
				if err := s.sell(ctx, v, &t); err != nil {
					failures = append(failures, s.recordFailure(ctx, v, &t, err))
				} else {
					sold++
				}
			}
		}
		for _, t := range trades {
			if t.Status == TradeOpening && price.LessOrEqual(t.BuyPrice) {
				if err := s.buy(ctx, v, &t, cfg); err != nil {
					failures = append(failures, s.recordFailure(ctx, v, &t, err))
				} else {
					bought++
				}
			}
		}
		if err := s.markToMarket(ctx, v.ID, price); err != nil {
			return "", err
		}
	}
	if sold+bought > 0 || len(failures) > 0 {
		s.changed()
	}
	msg := fmt.Sprintf("%d ladder(s), %d bought, %d sold", len(settings), bought, sold)
	if len(failures) > 0 {
		return msg, fmt.Errorf("%s; %d failed: %s", msg, len(failures), strings.Join(failures, "; "))
	}
	return msg, nil
}

func (s *Service) recordFailure(ctx context.Context, v settingView, t *dbq.DcaTrade, err error) string {
	t.LastError = err.Error()
	_ = saveTrade(ctx, dbq.New(s.DB.Write), *t)
	text := fmt.Sprintf("DCA %s rung #%d: %v", v.PairName, t.ID, err)
	slog.ErrorContext(ctx, "dca", "setting", v.ID, "trade", t.ID, "err", err)
	if s.Alerter != nil {
		// Throttled: a buy refused for lack of balance would otherwise alert every 5 minutes.
		s.Alerter.Report(ctx, fmt.Sprintf("dca trade %d", t.ID), err)
	}
	return text
}

func (s *Service) markToMarket(ctx context.Context, settingID int64, price money.Dec) error {
	q := dbq.New(s.DB.Write)
	trades, err := q.ListDcaTrades(ctx, settingID)
	if err != nil {
		return err
	}
	for _, t := range trades {
		if t.Status != TradeOpen {
			continue
		}
		t.UnrealizedProfit = t.HeldAmount.Mul(price).Sub(t.BuyQuote).Sub(t.ExtraFeeUsdt).Round(8)
		if err := saveTrade(ctx, q, t); err != nil {
			return err
		}
	}
	return nil
}

// openCapital is the USDT held in open rungs across every ladder.
func (s *Service) openCapital(ctx context.Context) (money.Dec, error) {
	costs, err := dbq.New(s.DB.Read).ListOpenDcaTradeCosts(ctx)
	if err != nil {
		return money.Zero, err
	}
	return money.Sum(costs...), nil
}

// ------------------------------------------------------------------------------- orders

type fill struct {
	order    binance.Order
	avg      money.Dec
	netQty   money.Dec // base asset received (buy) after a base-asset fee
	feeUSDT  money.Dec // every fee valued in USDT
	extraFee money.Dec // fees not taken from the base asset (USDT, BNB)
}

func (s *Service) describe(ctx context.Context, account binance.Account, o binance.Order, base string) fill {
	f := fill{order: o, avg: o.AvgPrice(), netQty: o.ExecutedQty}
	fills := o.Fills
	if len(fills) == 0 {
		if trades, err := s.Binance.OrderTrades(ctx, account, o.Symbol, o.OrderID); err == nil {
			for _, t := range trades {
				fills = append(fills, binance.Fill{Price: t.Price, Qty: t.Qty, Commission: t.Commission, CommissionAsset: t.CommissionAsset})
			}
		}
	}
	if len(fills) == 0 {
		est := o.CummulativeQuoteQty.Mul(money.MustParse("0.001"))
		f.feeUSDT, f.extraFee = est, est
		return f
	}
	for _, fl := range fills {
		switch strings.ToUpper(fl.CommissionAsset) {
		case strings.ToUpper(base):
			f.netQty = f.netQty.Sub(fl.Commission)
			f.feeUSDT = f.feeUSDT.Add(fl.Commission.Mul(fl.Price))
		case "USDT":
			f.feeUSDT = f.feeUSDT.Add(fl.Commission)
			f.extraFee = f.extraFee.Add(fl.Commission)
		default: // BNB and others: valued at the taker rate of the fill
			v := fl.Qty.Mul(fl.Price).Mul(money.MustParse("0.001"))
			if strings.ToUpper(fl.CommissionAsset) == "BNB" {
				if bnb, err := s.Binance.TickerPrice(ctx, "BNBUSDT"); err == nil {
					v = fl.Commission.Mul(bnb)
				}
			}
			f.feeUSDT = f.feeUSDT.Add(v)
			f.extraFee = f.extraFee.Add(v)
		}
	}
	f.feeUSDT, f.extraFee = f.feeUSDT.Round(8), f.extraFee.Round(8)
	return f
}

// order places a MARKET order, reusing the one an earlier attempt placed under the same client id.
func (s *Service) order(ctx context.Context, v settingView, side binance.Side, qty money.Dec, clientID string) (binance.Order, error) {
	acc := v.account()
	if o, found, err := s.Binance.OrderByClientID(ctx, acc, v.PairName, clientID); err != nil {
		return o, err
	} else if found {
		if !o.Filled() {
			return o, fmt.Errorf("order %s exists on Binance but status=%s", clientID, o.Status)
		}
		return o, nil
	}
	o, err := s.Binance.PlaceMarketOrder(ctx, binance.MarketOrder{Account: acc, Symbol: v.PairName, Side: side, Quantity: qty, ClientOrderID: clientID})
	if err != nil {
		return o, err
	}
	if !o.Filled() {
		return o, fmt.Errorf("%s %s %s: order status %s", side, qty, v.PairName, o.Status)
	}
	return o, nil
}

func step(v settingView) money.Dec {
	if v.StepSize == nil {
		return money.Zero
	}
	return *v.StepSize
}

func (s *Service) buy(ctx context.Context, v settingView, t *dbq.DcaTrade, cfg Config) error {
	qty := money.FloorToStep(t.Amount, step(v))
	if !qty.IsPositive() {
		return fmt.Errorf("amount %s rounds to 0 with step %s", t.Amount, step(v))
	}
	if cfg.MaxCapitalUSDT.IsPositive() && v.CurrentPrice != nil {
		open, err := s.openCapital(ctx)
		if err != nil {
			return err
		}
		need := qty.Mul(*v.CurrentPrice)
		if open.Add(need).GreaterThan(cfg.MaxCapitalUSDT) {
			return fmt.Errorf("capital cap: %s USDT open + %s > cap %s", open.Round(2), need.Round(2), cfg.MaxCapitalUSDT)
		}
	}
	o, err := s.order(ctx, v, binance.Buy, qty, t.OpenOrderID)
	if err != nil {
		return err
	}
	f := s.describe(ctx, v.account(), o, v.baseAsset())
	s.record(ctx, v, t.ID, f)
	t.Status, t.OpenPrice, t.HeldAmount, t.BuyQuote = TradeOpen, &f.avg, f.netQty.Round(8), o.CummulativeQuoteQty
	t.FeeUsdt, t.ExtraFeeUsdt, t.LastError, t.OpenAt = f.feeUSDT, f.extraFee, "", ptr(dbtype.NewTime(s.now()))
	if err := saveTrade(ctx, dbq.New(s.DB.Write), *t); err != nil {
		return err
	}
	s.notify(ctx, fmt.Sprintf("DCA %s rung #%d BOUGHT %s @ %s (trigger %s)", v.PairName, t.ID, f.netQty, f.avg.Round(8), t.BuyPrice))
	return nil
}

func (s *Service) sell(ctx context.Context, v settingView, t *dbq.DcaTrade) error {
	// Floor, never round: the held amount is the exact coin received after fees.
	qty := money.FloorToStep(t.HeldAmount, step(v))
	if !qty.IsPositive() {
		return fmt.Errorf("held amount %s rounds to 0 with step %s", t.HeldAmount, step(v))
	}
	o, err := s.order(ctx, v, binance.Sell, qty, t.CloseOrderID)
	if err != nil {
		return err
	}
	f := s.describe(ctx, v.account(), o, v.baseAsset())
	s.record(ctx, v, t.ID, f)
	t.Status, t.ClosePrice, t.SellQuote = TradeClosed, &f.avg, o.CummulativeQuoteQty
	t.FeeUsdt, t.ExtraFeeUsdt = t.FeeUsdt.Add(f.feeUSDT), t.ExtraFeeUsdt.Add(f.extraFee)
	// The coin left over by rounding to the step size stays in the account and is not counted.
	t.RealizedProfit = t.SellQuote.Sub(t.BuyQuote.Mul(qty).Div(t.HeldAmount)).Sub(t.ExtraFeeUsdt).Round(8)
	t.UnrealizedProfit, t.LastError, t.CloseAt = money.Zero, "", ptr(dbtype.NewTime(s.now()))
	if err := saveTrade(ctx, dbq.New(s.DB.Write), *t); err != nil {
		return err
	}
	s.notify(ctx, fmt.Sprintf("DCA %s rung #%d SOLD %s @ %s, PnL %s USDT", v.PairName, t.ID, qty, f.avg.Round(8), t.RealizedProfit.Round(2)))
	return nil
}

func (s *Service) record(ctx context.Context, v settingView, tradeID int64, f fill) {
	raw := dbtype.JSON(fmt.Sprintf(`{"orderId":%d,"status":%q}`, f.order.OrderID, f.order.Status))
	_ = dbq.New(s.DB.Write).UpsertExchangeOrder(ctx, dbq.UpsertExchangeOrderParams{
		ClientOrderID: f.order.ClientOrderID, Strategy: StrategyKey, RefID: &tradeID, Symbol: v.PairName,
		Account: string(v.account()), Side: string(f.order.Side), ExchangeOrderID: &f.order.OrderID, Status: f.order.Status,
		ExecutedQty: f.order.ExecutedQty, QuoteQty: f.order.CummulativeQuoteQty, AvgPrice: f.avg, NetQty: f.netQty,
		FeeUsdt: f.feeUSDT, Raw: raw, ExecutedAt: dbtype.NewTime(f.order.ExecutedAt()),
	})
}

func (s *Service) notify(ctx context.Context, msg string) {
	if err := s.Notifier.Send(ctx, notify.Orders, msg); err != nil {
		slog.ErrorContext(ctx, "dca notify", "err", err)
	}
}

func ptr[T any](v T) *T { return &v }

// Jobs returns the DCA price check.
func (s *Service) Jobs() []scheduler.Job {
	return []scheduler.Job{{
		Name: "dca.check", Description: "DCA: mua khi giá ≤ giá mua, bán khi giá ≥ giá bán, cập nhật lãi/lỗ",
		Cron: "0 */5 * * * *", Timeout: 5 * time.Minute, Run: s.Check,
	}}
}
