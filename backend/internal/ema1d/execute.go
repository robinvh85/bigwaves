package ema1d

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/money"
)

var (
	minNotionalMargin = money.MustParse("1.1")
	usdtFeeMargin     = money.MustParse("1.01")
	// buyBackFeeBuffer: Binance charges the buy fee in the base asset, so buying back exactly the
	// debt leaves us a fee short of the repay. Buy this much extra (taker fee 0.1%).
	buyBackFeeBuffer = money.MustParse("1.002")
)

const (
	settleReads = 5
	settleWait  = time.Second
)

func openOrderID(id int64) string  { return fmt.Sprintf("bw_e1_%d_open", id) }
func closeOrderID(id int64) string { return fmt.Sprintf("bw_e1_%d_close", id) }

// execute runs one OPEN/CLOSE todo against Binance. It is safe to call again after any failure:
// the position (with its client order ids) is created before the first order, and the short
// borrow goes through the borrow guard. It returns an error; the queue decides between retry,
// FAILED and SKIPPED.
func (s *Service) execute(ctx context.Context, todo *dbq.Ema1dTodo, cfg Config) (*dbq.Ema1dPosition, error) {
	pv, err := s.getPair(ctx, dbq.New(s.DB.Read), todo.Ema1dPairID)
	if err != nil {
		return nil, err
	}
	if todo.Action == ActionOpen {
		return s.executeOpen(ctx, todo, pv, cfg)
	}
	return s.executeClose(ctx, todo, pv)
}

func (s *Service) loadPosition(ctx context.Context, id *int64) (*dbq.Ema1dPosition, error) {
	if id == nil {
		return nil, nil
	}
	p, err := dbq.New(s.DB.Read).GetPosition(ctx, *id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ---------------------------------------------------------------------------------------- open

func (s *Service) executeOpen(ctx context.Context, todo *dbq.Ema1dTodo, pv pairView, cfg Config) (*dbq.Ema1dPosition, error) {
	pos, err := s.loadPosition(ctx, todo.PositionID)
	if err != nil {
		return nil, err
	}
	if pos == nil {
		if pos, err = s.createPosition(ctx, todo, pv, cfg); err != nil {
			return nil, err
		}
	}
	if pos.Status != PosOpening { // opened on an earlier attempt
		return pos, nil
	}
	if err := s.openPosition(ctx, todo, pos, pv); err != nil {
		return nil, err
	}
	return pos, nil
}

// createPosition checks everything that can refuse the trade before the position exists, so a
// refused OPEN leaves nothing behind.
func (s *Service) createPosition(ctx context.Context, todo *dbq.Ema1dTodo, pv pairView, cfg Config) (*dbq.Ema1dPosition, error) {
	side := todo.Side
	switch {
	case cfg.PauseOpen:
		return nil, permanent("pause_open")
	case pv.Status != PairActive:
		return nil, permanent("pair is %s", pv.Status)
	case !pv.sideAllowed(side):
		return nil, permanent("%s not allowed on %s", side, pv.PairName)
	case pv.IsSpot || pv.IsIsolated:
		return nil, permanent("pair is not on cross margin")
	}
	q := dbq.New(s.DB.Read)
	if side == SideShort {
		others, err := q.OtherActiveShorts(ctx, dbq.OtherActiveShortsParams{PairID: pv.PairID, ID: 0})
		if err != nil {
			return nil, err
		}
		if len(others) > 0 {
			return nil, permanent("conflict: position(s) #%s already short %s", joinIDs(others), pv.CoinName)
		}
	}
	equity, err := s.equity(ctx, q, cfg)
	if err != nil {
		return nil, err
	}
	if !equity.IsPositive() {
		return nil, permanent("EMA1D equity is %s USDT", equity.Round(2))
	}
	sizePct, notional, err := s.size(ctx, pv, side, equity, cfg)
	if err != nil {
		return nil, err
	}
	price, err := s.refreshPrice(ctx, pv)
	if err != nil {
		return nil, err
	}
	amount := money.FloorToStep(notional.Div(price), pv.step())
	if !amount.IsPositive() {
		return nil, permanent("amount rounds to 0 (notional %s USDT)", notional.Round(2))
	}
	notional = amount.Mul(price).Round(8)
	if side == SideLong {
		if err := s.checkLongBudget(ctx, q, notional, equity); err != nil {
			return nil, err
		}
	}

	var pos dbq.Ema1dPosition
	err = s.DB.Tx(ctx, func(tx *sql.Tx) error {
		tq := dbq.New(tx)
		// Unique placeholders, replaced by the id-based client order ids right after the insert.
		tmp := fmt.Sprintf("tmp_%d_%d", todo.ID, s.now().UnixNano())
		pos, err = tq.CreatePosition(ctx, dbq.CreatePositionParams{
			Ema1dPairID: pv.ID, PairID: pv.PairID, Side: side, SignalDate: todo.SignalDate, ExitRule: todo.ExitRule,
			Amount: amount, RefOpenPrice: todo.RefPrice, SizePercent: sizePct, EquityAtOpen: equity.Round(8),
			NotionalUsdt: notional, OpenOrderID: tmp + "_o", CloseOrderID: tmp + "_c",
		})
		if err != nil {
			return err
		}
		pos.OpenOrderID, pos.CloseOrderID = openOrderID(pos.ID), closeOrderID(pos.ID)
		if err := tq.SetPositionOrderIDs(ctx, dbq.SetPositionOrderIDsParams{OpenOrderID: pos.OpenOrderID, CloseOrderID: pos.CloseOrderID, ID: pos.ID}); err != nil {
			return err
		}
		todo.PositionID = &pos.ID
		return saveTodo(ctx, tq, *todo)
	})
	if err != nil {
		return nil, err
	}
	return &pos, nil
}

func joinIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return strings.Join(parts, ", #")
}

// size returns the effective % of equity and the notional in USDT.
func (s *Service) size(ctx context.Context, pv pairView, side string, equity money.Dec, cfg Config) (money.Dec, money.Dec, error) {
	pct, suggested := cfg.SizeFor(pv.PairGroup, side)
	if !pct.IsPositive() {
		return money.Zero, money.Zero, permanent("size %s %s is %s", pv.PairGroup, side, pct)
	}
	notional := equity.Mul(pct).Div(money.FromInt(100))
	minNotional, err := s.minNotional(ctx, pv)
	if err != nil {
		return money.Zero, money.Zero, err
	}
	if minNotional.IsPositive() && notional.LessThan(minNotional) {
		// A size set below the suggested one is a test run: trade the smallest size Binance takes
		// rather than nothing. At the real size, a too small order means the settings are wrong.
		if pct.GreaterOrEqual(suggested) {
			return money.Zero, money.Zero, permanent("notional %s below MIN_NOTIONAL %s", notional.Round(2), minNotional)
		}
		notional = minNotional.Mul(minNotionalMargin)
	}
	return notional.Div(equity).Mul(money.FromInt(100)).Round(4), notional, nil
}

func (s *Service) minNotional(ctx context.Context, pv pairView) (money.Dec, error) {
	if pv.MinNotional != nil {
		return *pv.MinNotional, nil
	}
	infos, err := s.Binance.ExchangeInfo(ctx, pv.PairName)
	if err != nil || len(infos) == 0 {
		return money.Zero, err
	}
	return infos[0].MinNotional, nil
}

// refreshPrice reads the ticker and stores it on the pair.
func (s *Service) refreshPrice(ctx context.Context, pv pairView) (money.Dec, error) {
	price, err := s.Binance.TickerPrice(ctx, pv.PairName)
	if err != nil {
		return money.Zero, err
	}
	if !price.IsPositive() {
		return money.Zero, fmt.Errorf("no current price for %s", pv.PairName)
	}
	now := s.now()
	_ = dbq.New(s.DB.Write).SetPairPrice(ctx, dbq.SetPairPriceParams{CurrentPrice: &price, PriceUpdatedAt: tptr(now), ID: pv.PairID})
	return price, nil
}

func (s *Service) checkLongBudget(ctx context.Context, q *dbq.Queries, notional, equity money.Dec) error {
	open, err := s.openLongNotional(ctx, q)
	if err != nil {
		return err
	}
	if open.Add(notional).GreaterThan(equity) {
		return permanent("insufficient_budget: open LONG %s + %s > equity %s USDT", open.Round(2), notional.Round(2), equity.Round(2))
	}
	acc, err := s.Binance.CrossMarginAccount(ctx)
	if err != nil {
		return err
	}
	free := acc.Asset("USDT").Free
	if free.LessThan(notional.Mul(usdtFeeMargin)) {
		return permanent("insufficient_budget: %s USDT free, %s needed", free.Round(2), notional.Round(2))
	}
	return nil
}

func (s *Service) openPosition(ctx context.Context, todo *dbq.Ema1dTodo, pos *dbq.Ema1dPosition, pv pairView) error {
	existing, err := s.lookupOrder(ctx, pv.PairName, pos.OpenOrderID)
	if err != nil {
		return err
	}
	order := existing
	if order == nil {
		var o binance.Order
		if pos.Side == SideLong {
			o, err = s.marketOrder(ctx, pv.PairName, binance.Buy, pos.Amount, pos.OpenOrderID)
		} else {
			guard := &borrowGuard{s: s, todo: todo}
			if err := guard.borrow(ctx, pv.CoinName, pos.Amount); err != nil {
				return err
			}
			s.waitForFree(ctx, pv.CoinName, pos.Amount)
			o, err = s.marketOrder(ctx, pv.PairName, binance.Sell, pos.Amount, pos.OpenOrderID)
		}
		if err != nil {
			return err
		}
		order = &o
	}
	f, err := s.describeFill(ctx, *order, pv.CoinName)
	if err != nil {
		return err
	}
	s.recordOrder(ctx, pos.ID, f)

	// Long: the fee is taken out of the coin bought, so the amount really held is the net filled
	// quantity. Short: we owe exactly what was borrowed, so the requested amount stays.
	if pos.Side == SideLong && f.NetQty.IsPositive() {
		pos.Amount = f.NetQty.Round(8)
	}
	pos.OpenPrice = &f.AvgPrice
	pos.FeeUsdt = f.FeeUSDT
	if pos.OpenAt == nil {
		pos.OpenAt = tptr(s.now())
	}
	pos.Status = PosOpen
	return savePosition(ctx, dbq.New(s.DB.Write), *pos)
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

// ---------------------------------------------------------------------------------------- close

func (s *Service) executeClose(ctx context.Context, todo *dbq.Ema1dTodo, pv pairView) (*dbq.Ema1dPosition, error) {
	pos, err := s.loadPosition(ctx, todo.PositionID)
	if err != nil {
		return nil, err
	}
	if pos == nil && todo.DependsOnID != nil {
		dep, err := dbq.New(s.DB.Read).GetTodo(ctx, *todo.DependsOnID)
		if err == nil {
			if pos, err = s.loadPosition(ctx, dep.PositionID); err != nil {
				return nil, err
			}
		}
	}
	if pos == nil {
		p, err := dbq.New(s.DB.Read).ActivePositionBySignal(ctx, dbq.ActivePositionBySignalParams{PairID: pv.PairID, Side: todo.Side, SignalDate: todo.SignalDate})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			pos = &p
		}
	}
	if pos == nil {
		return nil, &SkipError{msg: fmt.Sprintf("nothing to close: no EMA1D %s position from %s", todo.Side, todo.SignalDate)}
	}
	if pos.Status == PosClosed {
		return pos, nil
	}
	if pos.Status == PosOpening {
		return nil, permanent("position #%d was never opened", pos.ID)
	}
	if todo.PositionID == nil {
		todo.PositionID = &pos.ID
		if err := saveTodo(ctx, dbq.New(s.DB.Write), *todo); err != nil {
			return nil, err
		}
	}
	if err := s.closePosition(ctx, pos, pv); err != nil {
		return nil, err
	}
	if todo.RefPrice != nil {
		pos.RefClosePrice = todo.RefPrice
		if err := savePosition(ctx, dbq.New(s.DB.Write), *pos); err != nil {
			return nil, err
		}
	}
	return pos, nil
}

func (s *Service) closePosition(ctx context.Context, pos *dbq.Ema1dPosition, pv pairView) error {
	existing, err := s.lookupOrder(ctx, pv.PairName, pos.CloseOrderID)
	if err != nil {
		return err
	}
	var order binance.Order
	switch pos.Side {
	case SideLong:
		if existing != nil {
			order = *existing
		} else {
			// Floor, never round: amount is the exact coin held after fees.
			order, err = s.marketOrder(ctx, pv.PairName, binance.Sell, money.FloorToStep(pos.Amount, pv.step()), pos.CloseOrderID)
		}
	default:
		others, oErr := dbq.New(s.DB.Read).OtherActiveShorts(ctx, dbq.OtherActiveShortsParams{PairID: pos.PairID, ID: pos.ID})
		if oErr != nil {
			return oErr
		}
		if len(others) == 0 {
			order, err = s.closeShortWithInterest(ctx, pos, pv, existing)
		} else {
			s.notify(ctx, fmt.Sprintf("EMA1D: %s position #%d shares the coin debt with position(s) #%s: repaying the principal only, interest left on the account",
				pv.PairName, pos.ID, joinIDs(others)))
			order, err = s.closeShortPrincipal(ctx, pos, pv, existing)
		}
	}
	if err != nil {
		return err
	}
	f, err := s.describeFill(ctx, order, pv.CoinName)
	if err != nil {
		return err
	}
	s.recordOrder(ctx, pos.ID, f)

	fill := f.AvgPrice
	pos.FeeUsdt = pos.FeeUsdt.Add(f.FeeUSDT)
	pos.InterestUsdt = pos.InterestPaid.Mul(fill).Round(8)
	open := deref(pos.OpenPrice)
	gross := fill.Sub(open).Mul(pos.Amount)
	if pos.Side == SideShort {
		gross = gross.Neg()
	}
	pos.RealizedProfit = gross.Sub(pos.FeeUsdt).Sub(pos.InterestUsdt).Round(8)
	pos.UnrealizedProfit = money.Zero
	pos.ClosePrice = &fill
	pos.CloseAt = tptr(s.now())
	pos.Status = PosClosed
	return savePosition(ctx, dbq.New(s.DB.Write), *pos)
}

// closeShortWithInterest buys back the coin's whole cross-margin debt (principal and interest)
// and repays it: EMA1D shorts are held for weeks, so the interest is real money.
func (s *Service) closeShortWithInterest(ctx context.Context, pos *dbq.Ema1dPosition, pv pairView, existing *binance.Order) (binance.Order, error) {
	coin := pv.CoinName
	var order binance.Order
	if existing != nil {
		order = *existing
	} else {
		debt, err := s.coinDebt(ctx, coin)
		if err != nil {
			return order, err
		}
		if !debt.IsPositive() {
			return order, fmt.Errorf("position #%d: no %s debt on the cross account to close", pos.ID, coin)
		}
		if order, err = s.marketOrder(ctx, pv.PairName, binance.Buy, money.CeilToStep(debt.Mul(buyBackFeeBuffer), pv.step()), pos.CloseOrderID); err != nil {
			return order, err
		}
	}
	// Read again: on a retry the buy may be done and the repay too (then nothing is left to repay).
	asset, err := s.settledAsset(ctx, coin)
	if err != nil {
		return order, err
	}
	debt := asset.Debt()
	if debt.IsPositive() {
		// Floor, never round: rounding up could ask for more than the account holds.
		repay := money.New(money.Min(debt, asset.Free).RoundFloor(8))
		if !repay.IsPositive() {
			return order, fmt.Errorf("%w: position #%d: bought %s not in the cross account yet (free %s, owed %s), repay deferred",
				errBalanceNotSettled, pos.ID, coin, asset.Free, debt)
		}
		if _, err := s.Binance.BorrowRepay(ctx, binance.Repay, coin, repay, false, ""); err != nil {
			return order, err
		}
		pos.InterestPaid = asset.Interest
	}
	if remaining, err := s.coinDebt(ctx, coin); err == nil && remaining.IsPositive() {
		s.notify(ctx, fmt.Sprintf("EMA1D: %s position #%d closed but %s %s is still owed", pv.PairName, pos.ID, remaining, coin))
	}
	return order, nil
}

// closeShortPrincipal buys back the position's amount plus the fee buffer and repays what was
// really bought (used only when another position shorts the same coin).
func (s *Service) closeShortPrincipal(ctx context.Context, pos *dbq.Ema1dPosition, pv pairView, existing *binance.Order) (binance.Order, error) {
	var order binance.Order
	var err error
	if existing != nil {
		order = *existing
	} else if order, err = s.marketOrder(ctx, pv.PairName, binance.Buy, money.CeilToStep(pos.Amount.Mul(buyBackFeeBuffer), pv.step()), pos.CloseOrderID); err != nil {
		return order, err
	}
	f, err := s.describeFill(ctx, order, pv.CoinName)
	if err != nil {
		return order, err
	}
	repay := pos.Amount
	if f.NetQty.LessThan(pos.Amount) {
		repay = money.New(f.NetQty.RoundFloor(8))
	}
	_, err = s.Binance.BorrowRepay(ctx, binance.Repay, pv.CoinName, repay, false, "")
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

func savePosition(ctx context.Context, q *dbq.Queries, p dbq.Ema1dPosition) error {
	return q.SavePosition(ctx, dbq.SavePositionParams{
		Status: p.Status, Amount: p.Amount, OpenPrice: p.OpenPrice, ClosePrice: p.ClosePrice, RefClosePrice: p.RefClosePrice,
		FeeUsdt: p.FeeUsdt, InterestPaid: p.InterestPaid, InterestUsdt: p.InterestUsdt, RealizedProfit: p.RealizedProfit,
		UnrealizedProfit: p.UnrealizedProfit, OpenAt: p.OpenAt, CloseAt: p.CloseAt, ID: p.ID,
	})
}
