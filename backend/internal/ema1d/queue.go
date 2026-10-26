package ema1d

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/scheduler"
)

// Queue rules (Ema1d::ProcessTodosService):
//
//	transient error (network, rate limit, Binance busy, nothing to lend yet)
//	  → back to PENDING after 1, 2, 5, 10, 30, 60, 60… minutes
//	permanent error, or maxAttempts / retryWindow reached → FAILED
//	  (an OPEN short that already borrowed gives the coin back first)
//	nothing left to do → SKIPPED
//
// Only FAILED/SKIPPED and DONE reach Discord: retries stay quiet.
var backoffMinutes = []int{1, 2, 5, 10, 30, 60}

const (
	maxAttempts    = 10
	retryWindow    = 12 * time.Hour
	staleAfter     = 24 * time.Hour
	stuckAfter     = 15 * time.Minute // a RUNNING todo older than this belongs to a dead worker
	maxTodosPerRun = 20
)

// ProcessTodos works the queue: one todo per pair at a time, CLOSE before OPEN.
func (s *Service) ProcessTodos(ctx context.Context) (string, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return "", err
	}
	if !cfg.Enabled {
		return "", scheduler.Skipf("EMA1D is disabled")
	}
	if !s.Binance.TradingEnabled() {
		return "", scheduler.Skipf("TRADING_ENABLED=false: the queue is left untouched")
	}
	now := s.now()
	q := dbq.New(s.DB.Write)
	if _, err := q.RecoverStuckTodos(ctx, dbtype.NewTime(now.Add(-stuckAfter))); err != nil {
		return "", err
	}
	if err := s.skipBrokenDependencies(ctx); err != nil {
		return "", err
	}
	if err := s.skipStaleOpens(ctx); err != nil {
		return "", err
	}

	done := 0
	for i := 0; i < maxTodosPerRun; i++ {
		var todo dbq.Ema1dTodo
		err := s.DB.Tx(ctx, func(tx *sql.Tx) error {
			var err error
			todo, err = dbq.New(tx).ClaimNextTodo(ctx, dbtype.NewTime(s.now()))
			return err
		})
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return "", err
		}
		s.run(ctx, &todo, cfg)
		done++
		s.changed()
	}
	if done == 0 {
		return "", scheduler.Skipf("nothing to do")
	}
	return fmt.Sprintf("%d todo(s) processed", done), nil
}

func (s *Service) run(ctx context.Context, todo *dbq.Ema1dTodo, cfg Config) {
	pos, err := s.execute(ctx, todo, cfg)
	if err != nil {
		s.handleFailure(ctx, todo, err)
		return
	}
	todo.Status, todo.LastError = TodoDone, ""
	if err := saveTodo(ctx, dbq.New(s.DB.Write), *todo); err != nil {
		s.handleFailure(ctx, todo, err)
		return
	}
	s.notify(ctx, s.doneMessage(ctx, todo, pos))
}

func (s *Service) handleFailure(ctx context.Context, todo *dbq.Ema1dTodo, err error) {
	todo.Attempts++
	todo.LastError = truncate(err.Error(), 2000)
	now := s.now()
	var skip *SkipError
	switch {
	case errors.As(err, &skip):
		todo.Status = TodoSkipped
		s.saveOrDefer(ctx, todo)
		s.notify(ctx, fmt.Sprintf("EMA1D %s SKIPPED: %s", s.label(ctx, todo), skip.msg))
	case isPermanent(err) || todo.Attempts >= maxAttempts || (todo.FirstAttemptedAt != nil && now.Sub(todo.FirstAttemptedAt.Time) >= retryWindow):
		s.fail(ctx, todo)
	default:
		delay := backoffMinutes[len(backoffMinutes)-1]
		if int(todo.Attempts)-1 < len(backoffMinutes) {
			delay = backoffMinutes[todo.Attempts-1]
		}
		todo.Status = TodoPending
		todo.RunAfter = tptr(now.Add(time.Duration(delay) * time.Minute))
		s.saveOrDefer(ctx, todo)
	}
}

// saveOrDefer never leaves a todo RUNNING because bookkeeping failed; stuck recovery would retry
// it blindly much later.
func (s *Service) saveOrDefer(ctx context.Context, todo *dbq.Ema1dTodo) {
	if err := saveTodo(ctx, dbq.New(s.DB.Write), *todo); err != nil {
		todo.Status, todo.RunAfter = TodoPending, tptr(s.now().Add(5*time.Minute))
		_ = saveTodo(ctx, dbq.New(s.DB.Write), *todo)
	}
}

func (s *Service) fail(ctx context.Context, todo *dbq.Ema1dTodo) {
	var note string
	if todo.Action == ActionOpen {
		note = s.cleanupFailedOpen(ctx, todo)
	}
	todo.Status = TodoFailed
	s.saveOrDefer(ctx, todo)
	msg := fmt.Sprintf("EMA1D %s FAILED after %d attempt(s): %s", s.label(ctx, todo), todo.Attempts, todo.LastError)
	if note != "" {
		msg += "\n" + note
	}
	s.notify(ctx, msg)
}

// cleanupFailedOpen makes sure a failed OPEN leaves no position behind: return a borrowed coin,
// then drop the position if nothing is held, so the next sync does not see a phantom position.
func (s *Service) cleanupFailedOpen(ctx context.Context, todo *dbq.Ema1dTodo) string {
	pos, err := s.loadPosition(ctx, todo.PositionID)
	if err != nil || pos == nil || pos.Status != PosOpening {
		return ""
	}
	var note string
	if pos.Side == SideShort {
		pv, err := s.getPair(ctx, dbq.New(s.DB.Read), todo.Ema1dPairID)
		if err != nil {
			return "loading the pair failed: check the borrow by hand"
		}
		guard := &borrowGuard{s: s, todo: todo}
		note = guard.revert(ctx, pv.PairName, pv.CoinName, pos.OpenOrderID, pos.Amount)
	}
	if st := deref(todo.Step); st == StepBorrowed || st == StepBorrowing {
		return note
	}
	todo.PositionID = nil
	err = s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		if err := saveTodo(ctx, q, *todo); err != nil {
			return err
		}
		return q.DeletePosition(ctx, pos.ID)
	})
	if err != nil {
		todo.PositionID = &pos.ID
	}
	return note
}

// isPermanent classifies an execution error. Unknown errors are transient, like in Rails.
func isPermanent(err error) bool {
	var perm *PermanentError
	if errors.As(err, &perm) {
		return true
	}
	if errors.Is(err, errBalanceNotSettled) {
		return false
	}
	if errors.Is(err, binance.ErrTradingDisabled) || errors.Is(err, binance.ErrNotConfigured) {
		return true
	}
	var apiErr *binance.APIError
	if errors.As(err, &apiErr) {
		// HTML 5xx bodies come back without a code.
		return apiErr.Code != 0 && !binance.IsTransient(err)
	}
	return false
}

func (s *Service) skipBrokenDependencies(ctx context.Context) error {
	q := dbq.New(s.DB.Write)
	rows, err := q.BrokenDependents(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		t, err := q.GetTodo(ctx, r.ID)
		if err != nil {
			return err
		}
		t.Status = TodoSkipped
		t.LastError = fmt.Sprintf("depends on #%d which is %s", r.DepID, r.DepStatus)
		if err := saveTodo(ctx, q, t); err != nil {
			return err
		}
		s.notify(ctx, fmt.Sprintf("EMA1D %s SKIPPED: %s", s.label(ctx, &t), t.LastError))
	}
	return nil
}

// skipStaleOpens: an OPEN that never got its turn (EMA1D was off, a CLOSE kept retrying…) is too
// late now.
func (s *Service) skipStaleOpens(ctx context.Context) error {
	q := dbq.New(s.DB.Write)
	ids, err := q.StaleOpenTodos(ctx, dbtype.NewTime(s.now().Add(-staleAfter)))
	if err != nil {
		return err
	}
	for _, id := range ids {
		t, err := q.GetTodo(ctx, id)
		if err != nil {
			return err
		}
		t.Status = TodoSkipped
		t.LastError = "stale: not run within 24h"
		if err := saveTodo(ctx, q, t); err != nil {
			return err
		}
		s.notify(ctx, fmt.Sprintf("EMA1D %s SKIPPED: %s", s.label(ctx, &t), t.LastError))
	}
	return nil
}

func (s *Service) label(ctx context.Context, t *dbq.Ema1dTodo) string {
	coin := "?"
	if pv, err := s.getPair(ctx, dbq.New(s.DB.Read), t.Ema1dPairID); err == nil {
		coin = pv.CoinName
	}
	return fmt.Sprintf("%s %s %s (%s, todo #%d)", t.Action, t.Side, coin, t.SignalDate, t.ID)
}

func (s *Service) doneMessage(ctx context.Context, t *dbq.Ema1dTodo, pos *dbq.Ema1dPosition) string {
	price := deref(pos.OpenPrice)
	if t.Action == ActionClose {
		price = deref(pos.ClosePrice)
	}
	msg := fmt.Sprintf("EMA1D %s DONE: %s @ %s", s.label(ctx, t), pos.Amount, price)
	if t.RefPrice != nil && t.RefPrice.IsPositive() && price.IsPositive() {
		change := price.Div(*t.RefPrice).Sub(money.One).Mul(money.FromInt(100)).Round(2)
		msg += fmt.Sprintf(" (ref %s, %s%%)", t.RefPrice, change)
	}
	if t.Action == ActionClose {
		msg += fmt.Sprintf(", PnL %s USDT", pos.RealizedProfit.Round(2))
	}
	return msg
}
