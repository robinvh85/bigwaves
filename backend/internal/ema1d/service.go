// Package ema1d runs the EMA1D-EXIT strategy: a fixed basket of ~20 coins traded on Binance cross
// margin from 1D signals (see engine). LONG buys with USDT; SHORT borrows the coin and sells it.
// No stop loss, no take profit: a position only closes on an exit signal.
//
// Flow: daily check (07:03 VN) → candles → signals → sync (wanted state vs positions) → todos →
// queue worker (every minute) → executor → Binance. Ported from app/services/ema1d of the Rails app.
package ema1d

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"bigwaves/internal/exchange/binance"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/events"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/platform/notify"
)

// Enumerations stored as text.
const (
	GroupCore      = "CORE"
	GroupSensitive = "SENSITIVE"

	PairActive   = "ACTIVE"
	PairExitOnly = "EXIT_ONLY"
	PairDisabled = "DISABLED"

	SideLong  = "LONG"
	SideShort = "SHORT"

	ActionOpen  = "OPEN"
	ActionClose = "CLOSE"

	TodoPending   = "PENDING"
	TodoRunning   = "RUNNING"
	TodoDone      = "DONE"
	TodoFailed    = "FAILED"
	TodoSkipped   = "SKIPPED"
	TodoCancelled = "CANCELLED"

	SourceSync   = "SYNC"
	SourceManual = "MANUAL"

	PosOpening = "OPENING"
	PosOpen    = "OPEN"
	PosClosed  = "CLOSED"

	StepBorrowing      = "BORROWING"
	StepBorrowed       = "BORROWED"
	StepBorrowReverted = "BORROW_REVERTED"
)

type Service struct {
	DB       *db.DB
	Binance  *binance.Client
	Configs  *configstore.Store
	Notifier notify.Notifier
	Events   *events.Broker
	Pairs    *marketdata.Service
	// ReportConfigured tells whether the Discord report webhook is set (the daily check fails
	// without it, like the Rails EMA1D_REPORT_WEBHOOK_URL check).
	ReportConfigured bool

	// Now and Sleep are replaced in tests.
	Now   func() time.Time
	Sleep func(time.Duration)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) sleep(d time.Duration) {
	if s.Sleep != nil {
		s.Sleep(d)
		return
	}
	time.Sleep(d)
}

// Config returns the current config and its version.
func (s *Service) Config(ctx context.Context) (Config, int64, error) {
	c := DefaultConfig()
	v, err := s.Configs.Load(ctx, StrategyKey, &c)
	return c, v, err
}

func (s *Service) notify(ctx context.Context, msg string) {
	if err := s.Notifier.Send(ctx, notify.Orders, msg); err != nil {
		slog.ErrorContext(ctx, "ema1d notify", "err", err)
	}
}

func (s *Service) changed() { s.Events.Publish("ema1d.updated", nil) }

// PermanentError: retrying cannot help (budget, conflict, pair settings); the todo fails at once.
type PermanentError struct{ msg string }

func (e *PermanentError) Error() string { return e.msg }

func permanent(format string, args ...any) error {
	return &PermanentError{msg: fmt.Sprintf(format, args...)}
}

// SkipError: nothing to do any more (e.g. nothing left to close); the todo goes SKIPPED.
type SkipError struct{ msg string }

func (e *SkipError) Error() string { return e.msg }

// errBalanceNotSettled: the coin bought to close a short is not in the account yet. Transient:
// the retry finds the buy by its client order id and only repays.
var errBalanceNotSettled = errors.New("balance not settled")

// pairView is an EMA1D pair with what the strategy needs from its pair and coin.
type pairView struct {
	dbq.Ema1dPair
	PairName     string
	CoinName     string
	CurrentPrice *money.Dec
	StepSize     *money.Dec
	MinNotional  *money.Dec
	IsSpot       bool
	IsIsolated   bool
}

func (p pairView) sideAllowed(side string) bool {
	switch side {
	case SideLong:
		return p.AllowLong
	case SideShort:
		return p.AllowShort && p.ShortSupported
	}
	return false
}

func (p pairView) step() money.Dec {
	if p.StepSize == nil {
		return money.Zero
	}
	return *p.StepSize
}

func viewFromList(r dbq.ListEma1dPairsRow) pairView {
	return pairView{r.Ema1dPair, r.PairName, r.CoinName, r.CurrentPrice, r.StepSize, r.MinNotional, r.IsSpot, r.IsIsolated}
}

func (s *Service) listPairs(ctx context.Context, q *dbq.Queries) ([]pairView, error) {
	rows, err := q.ListEma1dPairs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]pairView, len(rows))
	for i, r := range rows {
		out[i] = viewFromList(r)
	}
	return out, nil
}

func (s *Service) getPair(ctx context.Context, q *dbq.Queries, id int64) (pairView, error) {
	r, err := q.GetEma1dPair(ctx, id)
	if err != nil {
		return pairView{}, err
	}
	return pairView{r.Ema1dPair, r.PairName, r.CoinName, r.CurrentPrice, r.StepSize, r.MinNotional, r.IsSpot, r.IsIsolated}, nil
}

// activePosition returns the open (or opening) position of a pair, nil when flat.
func activePosition(ctx context.Context, q *dbq.Queries, pairID int64) (*dbq.Ema1dPosition, error) {
	p, err := q.ActivePositionForPair(ctx, pairID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Equity is the strategy capital: the budget plus every realized PnL so far.
func (s *Service) equity(ctx context.Context, q *dbq.Queries, cfg Config) (money.Dec, error) {
	rows, err := q.ListClosedPositionTotals(ctx)
	if err != nil {
		return money.Zero, err
	}
	e := cfg.BudgetUSDT
	for _, r := range rows {
		e = e.Add(r.RealizedProfit)
	}
	return e, nil
}

func (s *Service) openLongNotional(ctx context.Context, q *dbq.Queries) (money.Dec, error) {
	rows, err := q.ListActivePositions(ctx)
	if err != nil {
		return money.Zero, err
	}
	sum := money.Zero
	for _, r := range rows {
		if r.Ema1dPosition.Side == SideLong {
			sum = sum.Add(r.Ema1dPosition.NotionalUsdt)
		}
	}
	return sum, nil
}

func dateOnly(t time.Time) string { return t.UTC().Format("2006-01-02") }

func parseDate(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }

func addDays(date string, days int) string {
	t, err := parseDate(date)
	if err != nil {
		return date
	}
	return dateOnly(t.AddDate(0, 0, days))
}

func ptr[T any](v T) *T { return &v }

func tptr(t time.Time) *dbtype.Time { return ptr(dbtype.NewTime(t)) }

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
