package ema1d

import (
	"context"
	"fmt"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/money"
)

// borrowGuard makes the short borrow of an OPEN todo happen at most once across retries. The
// step is written (and committed) around the borrow request:
//
//	none      → never sent: borrow
//	BORROWING → sent, outcome unknown (lost response, crash): look for it in the borrow history
//	BORROWED  → done: go straight to the sell, itself idempotent (client order id)
type borrowGuard struct {
	s    *Service
	todo *dbq.Ema1dTodo
}

// historyMargin: borrow records can carry a timestamp slightly before our clock.
const historyMargin = 5 * time.Minute

func (g *borrowGuard) step() string { return deref(g.todo.Step) }

func (g *borrowGuard) mark(ctx context.Context, step string) error {
	g.todo.Step = &step
	return dbq.New(g.s.DB.Write).SetTodoStep(ctx, dbq.SetTodoStepParams{Step: &step, ID: g.todo.ID})
}

func (g *borrowGuard) borrow(ctx context.Context, coin string, amount money.Dec) error {
	switch g.step() {
	case StepBorrowed:
		return nil
	case StepBorrowing:
		ok, err := g.borrowed(ctx, coin, amount)
		if err != nil {
			return err
		}
		if ok {
			return g.mark(ctx, StepBorrowed)
		}
	}
	if err := g.mark(ctx, StepBorrowing); err != nil {
		return err
	}
	if _, err := g.s.Binance.BorrowRepay(ctx, binance.Borrow, coin, amount, false, ""); err != nil {
		return err
	}
	return g.mark(ctx, StepBorrowed)
}

func (g *borrowGuard) borrowed(ctx context.Context, coin string, amount money.Dec) (bool, error) {
	since := g.todo.CreatedAt.Time
	if g.todo.FirstAttemptedAt != nil {
		since = g.todo.FirstAttemptedAt.Time
	}
	rows, err := g.s.Binance.LoanRecords(ctx, binance.Borrow, coin, since.Add(-historyMargin))
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

// revert gives the coin back when the short gave up before selling it. Returns a sentence for
// the alert.
func (g *borrowGuard) revert(ctx context.Context, symbol, coin, sellClientID string, amount money.Dec) string {
	switch g.step() {
	case StepBorrowed:
	case StepBorrowing:
		ok, err := g.borrowed(ctx, coin, amount)
		if err != nil {
			return fmt.Sprintf("checking the borrow of %s %s failed (%v): check by hand", amount, coin, err)
		}
		if !ok {
			return ""
		}
	default:
		return ""
	}
	o, found, err := g.s.Binance.OrderByClientID(ctx, binance.Cross, symbol, sellClientID)
	if err != nil {
		return fmt.Sprintf("repaying the borrowed %s %s failed (%v): repay by hand", amount, coin, err)
	}
	if found {
		_ = o
		return fmt.Sprintf("sell %s exists on Binance: the short is open, check position by hand", sellClientID)
	}
	acc, err := g.s.Binance.CrossMarginAccount(ctx)
	if err != nil {
		return fmt.Sprintf("repaying the borrowed %s %s failed (%v): repay by hand", amount, coin, err)
	}
	free := acc.Asset(coin).Free
	if free.LessThan(amount) {
		return fmt.Sprintf("%s %s borrowed but only %s free: NOT repaid, repay by hand", amount, coin, free)
	}
	if _, err := g.s.Binance.BorrowRepay(ctx, binance.Repay, coin, amount, false, ""); err != nil {
		return fmt.Sprintf("repaying the borrowed %s %s failed (%v): repay by hand", amount, coin, err)
	}
	if err := g.mark(ctx, StepBorrowReverted); err != nil {
		return fmt.Sprintf("borrowed %s %s repaid (step not saved: %v)", amount, coin, err)
	}
	return fmt.Sprintf("borrowed %s %s repaid", amount, coin)
}
