package emasettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
)

const (
	// buyBackFeeBuffer: buying back exactly the debt leaves the base-asset fee short of the repay.
	buyBackFeeBuffer = "1.002"
	settleReads      = 5
	settleWait       = time.Second
	// historyMargin: borrow records can carry a timestamp slightly before our clock.
	historyMargin = 5 * time.Minute

	stepBorrowing = "BORROWING"
	stepBorrowed  = "BORROWED"
)

// feeRate is Binance's taker fee, used only when an order's fills cannot be read.
var feeRate = money.MustParse("0.001")

// errPermanent marks a refusal retrying cannot fix (pair settings, amount below the minimum).
var errPermanent = errors.New("refused")

func refused(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errPermanent, fmt.Sprintf(format, args...))
}

// pairView is a setting's pair with what the orders need.
type pairView struct {
	ID          int64
	Name        string
	Coin        string
	Status      string
	Price       *money.Dec
	StepSize    *money.Dec
	MinNotional *money.Dec
	IsSpot      bool
	IsIsolated  bool
}

func (p pairView) step() money.Dec {
	if p.StepSize == nil {
		return money.Zero
	}
	return *p.StepSize
}

// account of a trade's orders: a long buys on spot for a spot pair, on cross margin otherwise; a
// short always borrows and sells on cross margin (as in Rails).
func (p pairView) account(side string) (binance.Account, error) {
	if p.IsIsolated {
		return "", refused("%s is on isolated margin, not supported", p.Name)
	}
	if side == SideLong && p.IsSpot {
		return binance.Spot, nil
	}
	return binance.Cross, nil
}

type filled struct {
	Order    binance.Order
	Account  binance.Account
	AvgPrice money.Dec
	NetQty   money.Dec // base asset received (buys) after base-asset commissions
	FeeUSDT  money.Dec
}

func (s *Service) describeFill(ctx context.Context, acc binance.Account, o binance.Order, base string) (filled, error) {
	if o.ExecutedQty.IsZero() {
		return filled{}, fmt.Errorf("order %s: executedQty is 0", o.ClientOrderID)
	}
	f := filled{Order: o, Account: acc, AvgPrice: o.AvgPrice(), NetQty: o.ExecutedQty}
	fills := o.Fills
	if len(fills) == 0 {
		if trades, err := s.Binance.OrderTrades(ctx, acc, o.Symbol, o.OrderID); err == nil {
			for _, t := range trades {
				fills = append(fills, binance.Fill{Price: t.Price, Qty: t.Qty, Commission: t.Commission, CommissionAsset: t.CommissionAsset})
			}
		}
	}
	if len(fills) == 0 {
		f.FeeUSDT = o.CummulativeQuoteQty.Mul(feeRate).Round(8)
		return f, nil
	}
	fee := money.Zero
	for _, fl := range fills {
		switch strings.ToUpper(fl.CommissionAsset) {
		case "USDT":
			fee = fee.Add(fl.Commission)
		case strings.ToUpper(base):
			fee = fee.Add(fl.Commission.Mul(fl.Price))
			f.NetQty = f.NetQty.Sub(fl.Commission)
		case "BNB":
			if bnb, err := s.Binance.TickerPrice(ctx, "BNBUSDT"); err == nil {
				fee = fee.Add(fl.Commission.Mul(bnb))
			} else {
				fee = fee.Add(fl.Qty.Mul(fl.Price).Mul(feeRate))
			}
		default:
			fee = fee.Add(fl.Qty.Mul(fl.Price).Mul(feeRate))
		}
	}
	f.FeeUSDT = fee.Round(8)
	return f, nil
}

// lookupOrder finds an order an earlier attempt placed under clientID. One that is not FILLED is
// an error: a MARKET order fills at once, so something unusual happened.
func (s *Service) lookupOrder(ctx context.Context, acc binance.Account, symbol, clientID string) (*binance.Order, error) {
	o, found, err := s.Binance.OrderByClientID(ctx, acc, symbol, clientID)
	if err != nil || !found {
		return nil, err
	}
	if !o.Filled() {
		return nil, fmt.Errorf("order %s exists on Binance but status=%s (expected FILLED)", clientID, o.Status)
	}
	return &o, nil
}

func (s *Service) marketOrder(ctx context.Context, acc binance.Account, symbol string, side binance.Side, qty money.Dec, clientID string) (binance.Order, error) {
	o, err := s.Binance.PlaceMarketOrder(ctx, binance.MarketOrder{Account: acc, Symbol: symbol, Side: side, Quantity: qty, ClientOrderID: clientID})
	if err != nil {
		return o, err
	}
	if !o.Filled() {
		return o, fmt.Errorf("%s %s %s: order status %s", side, qty, symbol, o.Status)
	}
	return o, nil
}

// recordOrder keeps the order in exchange_orders (audit, cutover rollback). Best effort: the trade
// row is the source of truth.
func (s *Service) recordOrder(ctx context.Context, tradeID int64, f filled) {
	raw, _ := json.Marshal(f.Order)
	_ = dbq.New(s.DB.Write).UpsertExchangeOrder(ctx, dbq.UpsertExchangeOrderParams{
		ClientOrderID: f.Order.ClientOrderID, Strategy: StrategyKey, RefID: &tradeID, Symbol: f.Order.Symbol,
		Account: string(f.Account), Side: string(f.Order.Side), ExchangeOrderID: &f.Order.OrderID,
		Status: f.Order.Status, ExecutedQty: f.Order.ExecutedQty, QuoteQty: f.Order.CummulativeQuoteQty,
		AvgPrice: f.AvgPrice, NetQty: f.NetQty, FeeUsdt: f.FeeUSDT, Raw: dbtype.JSON(raw),
		ExecutedAt: dbtype.NewTime(f.Order.ExecutedAt()),
	})
}

// ------------------------------------------------------------------------------------- open

// openTrade enters an OPENING trade at market: a long buys, a short borrows the coin (once across
// retries) and sells it. Safe to call again after any failure.
func (s *Service) openTrade(ctx context.Context, set dbq.EmaSetting, t *dbq.EmaSettingTrade, pv pairView, cfg Config) error {
	if t.Status != TradeOpening {
		return refused("trade #%d is %s, not OPENING", t.ID, t.Status)
	}
	acc, err := pv.account(t.Side)
	if err != nil {
		return err
	}
	if t.FirstAttemptedAt == nil {
		t.FirstAttemptedAt = ptr(dbtype.NewTime(s.now()))
		if err := saveTrade(ctx, dbq.New(s.DB.Write), *t); err != nil {
			return err
		}
	}
	order, err := s.lookupOrder(ctx, acc, pv.Name, t.OpenOrderID)
	if err != nil {
		return err
	}
	if order == nil {
		qty, err := s.openAmount(ctx, set, t, pv)
		if err != nil {
			return err
		}
		// Saved before any order: a retry must borrow and sell this same quantity, not one sized
		// again at a new price.
		if !t.Amount.Equal(qty) {
			t.Amount = qty
			if err := saveTrade(ctx, dbq.New(s.DB.Write), *t); err != nil {
				return err
			}
		}
		var o binance.Order
		if t.Side == SideLong {
			o, err = s.marketOrder(ctx, acc, pv.Name, binance.Buy, qty, t.OpenOrderID)
		} else {
			if err := s.borrowOnce(ctx, t, pv.Coin, qty); err != nil {
				return err
			}
			s.waitForFree(ctx, pv.Coin, qty)
			o, err = s.marketOrder(ctx, acc, pv.Name, binance.Sell, qty, t.OpenOrderID)
		}
		if err != nil {
			return err
		}
		order = &o
	}
	f, err := s.describeFill(ctx, acc, *order, pv.Coin)
	if err != nil {
		return err
	}
	s.recordOrder(ctx, t.ID, f)

	// Long: the fee comes out of the coin bought, so the amount held is the net quantity. Short:
	// we owe exactly what was borrowed.
	if t.Side == SideLong && f.NetQty.IsPositive() {
		t.Amount = f.NetQty.Round(8)
	} else if t.Amount.IsZero() {
		t.Amount = order.ExecutedQty
	}
	t.OpenPrice = &f.AvgPrice
	t.FeeUsdt = f.FeeUSDT
	if t.SlPrice == nil || !t.SlPrice.IsPositive() {
		if p := cfg.Params(set.Period).StopLossPercent; p.IsPositive() {
			t.SlPrice = ptr(StopLossPrice(t.Side, f.AvgPrice, p))
		}
	}
	t.UnrealizedProfit = Unrealized(t.Side, f.AvgPrice, f.AvgPrice, t.Amount, t.FeeUsdt)
	t.OpenAt = ptr(dbtype.NewTime(s.now()))
	t.Status, t.LastError = TradeOpen, ""
	return saveTrade(ctx, dbq.New(s.DB.Write), *t)
}

// openAmount is the quantity to buy or borrow: the trade's amount, or the setting's budget at the
// current price when it has none, floored to the step size and checked against the minimum.
func (s *Service) openAmount(ctx context.Context, set dbq.EmaSetting, t *dbq.EmaSettingTrade, pv pairView) (money.Dec, error) {
	if t.BorrowStep != "" && t.Amount.IsPositive() {
		return t.Amount, nil // the coin is (maybe) borrowed already: sell exactly that
	}
	price, err := s.Binance.TickerPrice(ctx, pv.Name)
	if err != nil {
		return money.Zero, err
	}
	qty := t.Amount
	if !qty.IsPositive() {
		if !set.Budget.IsPositive() {
			return money.Zero, refused("setting #%d has no budget and the trade no amount", set.ID)
		}
		qty = set.Budget.Div(price)
	}
	qty = money.FloorToStep(qty, pv.step())
	if !qty.IsPositive() {
		return money.Zero, refused("amount rounds to 0 with step %s", pv.step())
	}
	if pv.MinNotional != nil && qty.Mul(price).LessThan(*pv.MinNotional) {
		return money.Zero, refused("%s × %s = %s USDT is below the minimum %s", qty, price, qty.Mul(price).Round(2), pv.MinNotional)
	}
	return qty, nil
}

// borrowOnce borrows the short's coin at most once across retries: the step is saved around the
// request, and a BORROWING left by a lost response is settled from the borrow history.
func (s *Service) borrowOnce(ctx context.Context, t *dbq.EmaSettingTrade, coin string, amount money.Dec) error {
	switch t.BorrowStep {
	case stepBorrowed:
		return nil
	case stepBorrowing:
		ok, err := s.borrowed(ctx, t, coin, amount)
		if err != nil {
			return err
		}
		if ok {
			return s.setBorrowStep(ctx, t, stepBorrowed)
		}
	}
	if err := s.setBorrowStep(ctx, t, stepBorrowing); err != nil {
		return err
	}
	if _, err := s.Binance.BorrowRepay(ctx, binance.Borrow, coin, amount, false, ""); err != nil {
		return err
	}
	return s.setBorrowStep(ctx, t, stepBorrowed)
}

func (s *Service) setBorrowStep(ctx context.Context, t *dbq.EmaSettingTrade, step string) error {
	t.BorrowStep = step
	return dbq.New(s.DB.Write).SetEmaSettingTradeBorrowStep(ctx, dbq.SetEmaSettingTradeBorrowStepParams{BorrowStep: step, ID: t.ID})
}

func (s *Service) borrowed(ctx context.Context, t *dbq.EmaSettingTrade, coin string, amount money.Dec) (bool, error) {
	since := t.CreatedAt.Time
	if t.FirstAttemptedAt != nil {
		since = t.FirstAttemptedAt.Time
	}
	rows, err := s.Binance.LoanRecords(ctx, binance.Borrow, coin, since.Add(-historyMargin))
	if err != nil {
		return false, err
	}
	for _, r := range rows {
		got := r.Principal
		if got.IsZero() {
			got = r.Amount
		}
		if (r.Status == "CONFIRMED" || r.Status == "PENDING") && got.Equal(amount) {
			return true, nil
		}
	}
	return false, nil
}

// waitForFree gives a just-borrowed coin a moment to show up as free before selling it.
func (s *Service) waitForFree(ctx context.Context, coin string, amount money.Dec) {
	for i := 0; i < settleReads; i++ {
		if i > 0 {
			s.sleep(settleWait)
		}
		acc, err := s.Binance.CrossMarginAccount(ctx)
		if err == nil && acc.Asset(coin).Free.GreaterOrEqual(amount) {
			return
		}
	}
}

// ------------------------------------------------------------------------------------ close

// closeTrade exits an OPEN trade at market and books its PnL, net of fees and interest. Safe to
// call again: the close order is found by its client id.
func (s *Service) closeTrade(ctx context.Context, t *dbq.EmaSettingTrade, pv pairView, reason string) error {
	if t.Status != TradeOpen {
		return refused("trade #%d is %s, not OPEN", t.ID, t.Status)
	}
	acc, err := pv.account(t.Side)
	if err != nil {
		return err
	}
	existing, err := s.lookupOrder(ctx, acc, pv.Name, t.CloseOrderID)
	if err != nil {
		return err
	}
	var order binance.Order
	if t.Side == SideLong {
		if existing != nil {
			order = *existing
		} else if order, err = s.marketOrder(ctx, acc, pv.Name, binance.Sell, money.FloorToStep(t.Amount, pv.step()), t.CloseOrderID); err != nil {
			return err
		}
	} else {
		others, err := s.otherShorts(ctx, pv.ID, t.ID)
		if err != nil {
			return err
		}
		if len(others) == 0 {
			order, err = s.closeShortWithInterest(ctx, t, pv, existing)
		} else {
			s.notify(ctx, fmt.Sprintf("EMA Settings: %s trade #%d shares the %s debt with %s: repaying the principal only, interest left on the account",
				pv.Name, t.ID, pv.Coin, strings.Join(others, ", ")))
			order, err = s.closeShortPrincipal(ctx, t, pv, existing)
		}
		if err != nil {
			return err
		}
	}
	f, err := s.describeFill(ctx, acc, order, pv.Coin)
	if err != nil {
		return err
	}
	s.recordOrder(ctx, t.ID, f)

	fill := f.AvgPrice
	t.FeeUsdt = t.FeeUsdt.Add(f.FeeUSDT)
	t.InterestUsdt = t.InterestPaid.Mul(fill).Round(8)
	gross := fill.Sub(deref(t.OpenPrice)).Mul(t.Amount)
	if t.Side == SideShort {
		gross = gross.Neg()
	}
	t.RealizedProfit = gross.Sub(t.FeeUsdt).Sub(t.InterestUsdt).Round(8)
	t.UnrealizedProfit = money.Zero
	t.ClosePrice = &fill
	t.CloseAt = ptr(dbtype.NewTime(s.now()))
	t.Status, t.CloseReason, t.LastError = TradeClosed, reason, ""
	return saveTrade(ctx, dbq.New(s.DB.Write), *t)
}

// otherShorts names the other trades short the same coin (EMA1D positions and EMA Settings
// trades): the cross-margin debt is per coin, so the interest cannot be told apart.
func (s *Service) otherShorts(ctx context.Context, pairID, selfID int64) ([]string, error) {
	q := dbq.New(s.DB.Read)
	e1, err := q.OtherActiveShorts(ctx, dbq.OtherActiveShortsParams{PairID: pairID, ID: 0})
	if err != nil {
		return nil, err
	}
	es, err := q.ActiveSettingShorts(ctx, dbq.ActiveSettingShortsParams{PairID: pairID, ID: selfID})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(e1)+len(es))
	for _, id := range e1 {
		out = append(out, fmt.Sprintf("EMA1D #%d", id))
	}
	for _, id := range es {
		out = append(out, fmt.Sprintf("EMA Settings #%d", id))
	}
	return out, nil
}

// closeShortWithInterest buys back the coin's whole cross-margin debt (principal and interest)
// and repays it. Only when no other trade is short the coin.
func (s *Service) closeShortWithInterest(ctx context.Context, t *dbq.EmaSettingTrade, pv pairView, existing *binance.Order) (binance.Order, error) {
	coin := pv.Coin
	var order binance.Order
	if existing != nil {
		order = *existing
	} else {
		debt, err := s.coinDebt(ctx, coin)
		if err != nil {
			return order, err
		}
		if !debt.IsPositive() {
			return order, fmt.Errorf("trade #%d: no %s debt on the cross account to close", t.ID, coin)
		}
		qty := money.CeilToStep(debt.Mul(money.MustParse(buyBackFeeBuffer)), pv.step())
		if order, err = s.marketOrder(ctx, binance.Cross, pv.Name, binance.Buy, qty, t.CloseOrderID); err != nil {
			return order, err
		}
	}
	// Read again: on a retry the buy may be done and the repay too.
	asset, err := s.settledAsset(ctx, coin)
	if err != nil {
		return order, err
	}
	if debt := asset.Debt(); debt.IsPositive() {
		repay := money.New(money.Min(debt, asset.Free).RoundFloor(8))
		if !repay.IsPositive() {
			return order, fmt.Errorf("trade #%d: bought %s not in the cross account yet (free %s, owed %s), repay deferred", t.ID, coin, asset.Free, debt)
		}
		if _, err := s.Binance.BorrowRepay(ctx, binance.Repay, coin, repay, false, ""); err != nil {
			return order, err
		}
		t.InterestPaid = asset.Interest
	}
	if remaining, err := s.coinDebt(ctx, coin); err == nil && remaining.IsPositive() {
		s.notify(ctx, fmt.Sprintf("EMA Settings: %s trade #%d closed but %s %s is still owed", pv.Name, t.ID, remaining, coin))
	}
	return order, nil
}

// closeShortPrincipal buys back the trade's amount plus the fee buffer and repays what was really
// bought, leaving the other trades' debt (and the shared interest) on the account.
func (s *Service) closeShortPrincipal(ctx context.Context, t *dbq.EmaSettingTrade, pv pairView, existing *binance.Order) (binance.Order, error) {
	var order binance.Order
	var err error
	if existing != nil {
		order = *existing
	} else {
		qty := money.CeilToStep(t.Amount.Mul(money.MustParse(buyBackFeeBuffer)), pv.step())
		if order, err = s.marketOrder(ctx, binance.Cross, pv.Name, binance.Buy, qty, t.CloseOrderID); err != nil {
			return order, err
		}
	}
	f, err := s.describeFill(ctx, binance.Cross, order, pv.Coin)
	if err != nil {
		return order, err
	}
	repay := t.Amount
	if f.NetQty.LessThan(t.Amount) {
		repay = money.New(f.NetQty.RoundFloor(8))
	}
	_, err = s.Binance.BorrowRepay(ctx, binance.Repay, pv.Coin, repay, false, "")
	return order, err
}

func (s *Service) coinDebt(ctx context.Context, coin string) (money.Dec, error) {
	acc, err := s.Binance.CrossMarginAccount(ctx)
	if err != nil {
		return money.Zero, err
	}
	return acc.Asset(coin).Debt(), nil
}

// settledAsset reads the coin until the account shows the bought coin (free >= debt), a few times.
func (s *Service) settledAsset(ctx context.Context, coin string) (binance.MarginAsset, error) {
	var asset binance.MarginAsset
	for i := 0; i < settleReads; i++ {
		if i > 0 {
			s.sleep(settleWait)
		}
		acc, err := s.Binance.CrossMarginAccount(ctx)
		if err != nil {
			return asset, err
		}
		asset = acc.Asset(coin)
		if asset.Free.GreaterOrEqual(asset.Debt()) {
			break
		}
	}
	return asset, nil
}

func saveTrade(ctx context.Context, q *dbq.Queries, t dbq.EmaSettingTrade) error {
	return q.SaveEmaSettingTrade(ctx, dbq.SaveEmaSettingTradeParams{
		Side: t.Side, Status: t.Status, Amount: t.Amount, EntryPrice: t.EntryPrice, TargetPrice: t.TargetPrice,
		SlPrice: t.SlPrice, OpenPrice: t.OpenPrice, ClosePrice: t.ClosePrice, FeeUsdt: t.FeeUsdt,
		InterestPaid: t.InterestPaid, InterestUsdt: t.InterestUsdt, RealizedProfit: t.RealizedProfit,
		UnrealizedProfit: t.UnrealizedProfit, CloseReason: t.CloseReason, FirstAttemptedAt: t.FirstAttemptedAt,
		LastError: t.LastError, OpenAt: t.OpenAt, CloseAt: t.CloseAt, ID: t.ID,
	})
}

func ptr[T any](v T) *T { return &v }

func deref(d *money.Dec) money.Dec {
	if d == nil {
		return money.Zero
	}
	return *d
}
