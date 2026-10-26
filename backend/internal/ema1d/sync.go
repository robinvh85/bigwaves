package ema1d

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
)

// TodoSummary is how a todo appears in a sync result (JSON keys as in Rails).
type TodoSummary struct {
	ID          *int64 `json:"id,omitempty"`
	Coin        string `json:"coin"`
	Action      string `json:"action"`
	Side        string `json:"side"`
	SignalDate  string `json:"signal_date"`
	Status      string `json:"status"`
	DependsOnID *int64 `json:"depends_on_id,omitempty"`
}

type SkippedEntry struct {
	Coin   string `json:"coin"`
	Action string `json:"action"`
	Side   string `json:"side"`
	Reason string `json:"reason"`
}

type PositionSummary struct {
	Coin            string   `json:"coin"`
	PairStatus      string   `json:"pair_status"`
	Wanted          string   `json:"wanted"`
	WantedEntryDate *string  `json:"wanted_entry_date"`
	Actual          *string  `json:"actual"`
	ActualEntryDate *string  `json:"actual_entry_date"`
	Actions         []string `json:"actions"`
}

// SyncResult is returned by the sync API and stored in ema1d_syncs.result.
type SyncResult struct {
	Success          bool              `json:"success"`
	Date             string            `json:"date"`
	RunID            string            `json:"run_id"`
	DryRun           bool              `json:"dry_run"`
	Replayed         bool              `json:"replayed,omitempty"`
	CreatedTodos     []TodoSummary     `json:"created_todos"`
	CancelledTodos   []TodoSummary     `json:"cancelled_todos"`
	RequeuedTodos    []TodoSummary     `json:"requeued_todos"`
	Skipped          []SkippedEntry    `json:"skipped"`
	Warnings         []string          `json:"warnings"`
	PositionsSummary []PositionSummary `json:"positions_summary"`
}

// Sync turns the wanted state of the basket into OPEN/CLOSE todos.
//
// For each pair it compares the wanted state with the effective position:
//   - an in-flight OPEN todo (what we are about to hold), else
//   - the active position, unless an in-flight CLOSE todo is already taking it down, else
//   - flat.
//
// Same side and an entry date on or after the strategy's means in sync: a later date is a late
// entry for the same signal (e.g. opened by hand), kept with a late_entry warning. Same side with
// an older entry date means the strategy closed and re-entered while we were not syncing
// (reentered). Anything not in sync closes what is held (a not yet started OPEN is cancelled
// instead) and opens the wanted side when the entry is still valid. An OPEN that follows a CLOSE
// on the same pair depends on it, so a reversal runs in order.
//
// Calling twice with the same run_id + date returns the stored result; dry_run computes the same
// diff and rolls it back.
func (s *Service) Sync(ctx context.Context, req SyncRequest, rawQuery string) (SyncResult, error) {
	p, err := parseSync(req)
	if err != nil {
		return SyncResult{}, err
	}
	if !p.DryRun {
		if prev, ok, err := s.storedSync(ctx, p.RunID, p.Date); err != nil || ok {
			return prev, err
		}
	}
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return SyncResult{}, err
	}

	parsedJSON, _ := json.Marshal(map[string]any{"items": p.Items, "warnings": nonNil(p.Warnings)})
	var result SyncResult
	errRollback := errors.New("dry run rollback")
	err = s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		row, err := q.CreateSync(ctx, dbq.CreateSyncParams{
			RunID: p.RunID, SignalDate: p.Date, RawQuery: rawQuery, Parsed: dbtype.JSON(parsedJSON),
			Result: dbtype.JSON("{}"), DryRun: p.DryRun,
		})
		if err != nil {
			return err
		}
		r := &reconciler{s: s, q: q, ctx: ctx, cfg: cfg, sync: row, date: p.Date, dryRun: p.DryRun}
		result, err = r.run(p)
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(result)
		if err := q.SetSyncResult(ctx, dbq.SetSyncResultParams{Result: dbtype.JSON(raw), ID: row.ID}); err != nil {
			return err
		}
		if p.DryRun {
			return errRollback
		}
		return nil
	})
	if errors.Is(err, errRollback) {
		raw, _ := json.Marshal(result)
		_, err = dbq.New(s.DB.Write).CreateSync(ctx, dbq.CreateSyncParams{
			RunID: p.RunID, SignalDate: p.Date, RawQuery: rawQuery, Parsed: dbtype.JSON(parsedJSON),
			Result: dbtype.JSON(raw), DryRun: true,
		})
		return result, err
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			// A concurrent call with the same run_id committed first.
			prev, _, err := s.storedSync(ctx, p.RunID, p.Date)
			return prev, err
		}
		return SyncResult{}, err
	}
	s.changed()
	return result, nil
}

func (s *Service) storedSync(ctx context.Context, runID, date string) (SyncResult, bool, error) {
	row, err := dbq.New(s.DB.Read).GetRealSync(ctx, dbq.GetRealSyncParams{RunID: runID, SignalDate: date})
	if errors.Is(err, sql.ErrNoRows) {
		return SyncResult{}, false, nil
	}
	if err != nil {
		return SyncResult{}, false, err
	}
	var r SyncResult
	if err := json.Unmarshal(row.Result, &r); err != nil {
		return SyncResult{}, false, err
	}
	r.Replayed = true
	return r, true, nil
}

func nonNil[T any](xs []T) []T {
	if xs == nil {
		return []T{}
	}
	return xs
}

type reconciler struct {
	s      *Service
	q      *dbq.Queries
	ctx    context.Context
	cfg    Config
	sync   dbq.Ema1dSync
	date   string
	dryRun bool
	cutoff string

	created, cancelled, requeued []dbq.Ema1dTodo
	skipped                      []SkippedEntry
	warnings                     []string
	summary                      []PositionSummary
	coins                        map[int64]string // ema1d pair id → coin
}

func (r *reconciler) run(p parsedSync) (SyncResult, error) {
	r.warnings = append(r.warnings, p.Warnings...)
	today := dateOnly(r.s.now())
	if r.date != today {
		r.warnings = append(r.warnings, fmt.Sprintf("date_not_today: %s (UTC today %s)", r.date, today))
	}
	if !r.cfg.Enabled {
		r.warnings = append(r.warnings, "ema1d_disabled: todos are created but not executed (EMA1D enabled is off)")
	}
	// Entry age is measured from the later of the candle date and today, so replaying an old date
	// cannot open a stale entry.
	r.cutoff = addDays(max(r.date, today), -r.cfg.MaxEntryAgeDays)

	pairs, err := r.s.listPairs(r.ctx, r.q)
	if err != nil {
		return SyncResult{}, err
	}
	slices.SortFunc(pairs, func(a, b pairView) int { return int(a.ID - b.ID) })
	r.coins = map[int64]string{}
	known := map[string]bool{}
	for _, pv := range pairs {
		r.coins[pv.ID] = pv.CoinName
		known[pv.CoinName] = true
	}
	items := map[string]SyncItem{}
	for _, it := range p.Items {
		items[it.Coin] = it
		if !known[it.Coin] {
			r.warnings = append(r.warnings, "unknown_coin: "+it.Coin)
		}
	}

	for _, pv := range pairs {
		item, ok := items[pv.CoinName]
		if pv.Status == PairDisabled {
			pos, err := activePosition(r.ctx, r.q, pv.ID)
			if err != nil {
				return SyncResult{}, err
			}
			if pos != nil {
				r.warnings = append(r.warnings, "disabled_with_open_trade: "+pv.CoinName)
			}
			continue
		}
		if !ok {
			r.warnings = append(r.warnings, "missing: "+pv.CoinName)
			continue
		}
		if err := r.reconcile(pv, item); err != nil {
			return SyncResult{}, err
		}
	}

	return SyncResult{
		Success: true, Date: r.date, RunID: p.RunID, DryRun: r.dryRun,
		CreatedTodos:     r.summaries(r.created),
		CancelledTodos:   r.summaries(r.cancelled),
		RequeuedTodos:    r.summaries(r.requeued),
		Skipped:          nonNil(r.skipped),
		Warnings:         nonNil(r.warnings),
		PositionsSummary: nonNil(r.summary),
	}, nil
}

type effective struct {
	side       string
	signalDate string
	todo       *dbq.Ema1dTodo
	position   *dbq.Ema1dPosition
}

func (r *reconciler) reconcile(pv pairView, item SyncItem) error {
	pos, err := activePosition(r.ctx, r.q, pv.ID)
	if err != nil {
		return err
	}
	todos, err := r.q.InFlightTodosForPair(r.ctx, pv.ID)
	if err != nil {
		return err
	}
	var openTodo, closeTodo *dbq.Ema1dTodo
	for i := range todos {
		t := &todos[i]
		if t.Action == ActionOpen && openTodo == nil {
			openTodo = t
		}
	}
	if pos != nil {
		for i := range todos {
			t := &todos[i]
			if t.Action == ActionClose && closeTodo == nil && r.closesPosition(t, pos) {
				closeTodo = t
			}
		}
	}
	var eff *effective
	switch {
	case openTodo != nil:
		eff = &effective{side: openTodo.Side, signalDate: openTodo.SignalDate, todo: openTodo}
	case pos != nil && closeTodo == nil:
		eff = &effective{side: pos.Side, signalDate: pos.SignalDate, position: pos}
	}

	sum := PositionSummary{Coin: pv.CoinName, PairStatus: pv.Status, Wanted: item.State, WantedEntryDate: item.EntryDate, Actions: []string{}}
	if eff != nil {
		sum.Actual, sum.ActualEntryDate = &eff.side, &eff.signalDate
	}
	idx := len(r.summary)
	r.summary = append(r.summary, sum)
	addAction := func(a string) { r.summary[idx].Actions = append(r.summary[idx].Actions, a) }

	if inSync(eff, item) {
		if eff != nil && eff.signalDate > *item.EntryDate {
			r.warnings = append(r.warnings, fmt.Sprintf("late_entry: %s %s %s (strategy %s), kept", pv.CoinName, item.Side(), eff.signalDate, *item.EntryDate))
		}
		return nil
	}
	if eff != nil && eff.side == item.Side() {
		r.warnings = append(r.warnings, fmt.Sprintf("reentered: %s %s %s -> %s", pv.CoinName, item.Side(), eff.signalDate, deref(item.EntryDate)))
	}

	// 1. Get flat: cancel an OPEN that has not touched Binance yet, otherwise close what is held.
	blocker := closeTodo
	if eff != nil {
		if eff.todo != nil && cancellable(*eff.todo) {
			if err := r.cancel(eff.todo); err != nil {
				return err
			}
			addAction("cancel_open")
		} else {
			b, err := r.ensureClose(pv, eff, item)
			if err != nil {
				return err
			}
			blocker = b
			if b != nil {
				addAction("close")
			}
		}
	}
	if item.Flat() {
		return nil
	}

	// 2. Open the wanted side when the entry is still valid.
	if reason := r.openSkipReason(pv, item); reason != "" {
		r.skipped = append(r.skipped, SkippedEntry{Coin: pv.CoinName, Action: ActionOpen, Side: item.Side(), Reason: reason})
		return nil
	}
	opened, err := r.createTodo(pv, ActionOpen, item.Side(), *item.EntryDate, item.RefPrice, item.Rule, nil, blocker)
	if err != nil {
		return err
	}
	if opened != nil {
		addAction("open")
	}
	return nil
}

func inSync(eff *effective, item SyncItem) bool {
	if item.Flat() {
		return eff == nil
	}
	return eff != nil && eff.side == item.Side() && eff.signalDate >= *item.EntryDate
}

func (r *reconciler) closesPosition(t *dbq.Ema1dTodo, pos *dbq.Ema1dPosition) bool {
	if t.PositionID != nil && *t.PositionID == pos.ID {
		return true
	}
	if t.DependsOnID != nil {
		dep, err := r.q.GetTodo(r.ctx, *t.DependsOnID)
		if err == nil && dep.PositionID != nil && *dep.PositionID == pos.ID {
			return true
		}
	}
	return false
}

// cancellable: only a todo that never touched Binance. Once attempted it may own a position, a
// borrow or a filled order whose response was lost.
func cancellable(t dbq.Ema1dTodo) bool {
	return t.Status == TodoPending && t.Attempts == 0 && t.Step == nil && (t.Action == ActionClose || t.PositionID == nil)
}

func (r *reconciler) cancel(t *dbq.Ema1dTodo) error {
	t.Status = TodoCancelled
	t.LastError = "cancelled by sync " + r.sync.RunID
	if err := saveTodo(r.ctx, r.q, *t); err != nil {
		return err
	}
	r.cancelled = append(r.cancelled, *t)
	return nil
}

// ensureClose queues a CLOSE for the effective position: the active one, or the one an in-flight
// OPEN will create.
func (r *reconciler) ensureClose(pv pairView, eff *effective, item SyncItem) (*dbq.Ema1dTodo, error) {
	var ref = item.RefPrice
	if !item.Flat() && deref(item.EntryDate) != r.date {
		ref = nil
	}
	var rule *string
	if !item.Flat() {
		rule = item.Rule
	}
	var posID *int64
	if eff.position != nil {
		posID = &eff.position.ID
	}
	return r.createTodo(pv, ActionClose, eff.side, eff.signalDate, ref, rule, posID, eff.todo)
}

func (r *reconciler) openSkipReason(pv pairView, item SyncItem) string {
	side := item.Side()
	entry := *item.EntryDate
	switch {
	case pv.Status != PairActive:
		return "pair_" + strings.ToLower(pv.Status)
	case !pv.sideAllowed(side):
		if side == SideShort && pv.AllowShort {
			return "short_not_supported"
		}
		return "side_not_allowed"
	case entry < pv.AddedOn:
		return fmt.Sprintf("entry_before_added_on (%s)", pv.AddedOn)
	case entry < r.cutoff:
		return fmt.Sprintf("entry_too_old (before %s)", r.cutoff)
	case r.cfg.PauseOpen:
		return "pause_open"
	}
	return ""
}

// createTodo returns the todo later steps should wait for, or nil when nothing is left to wait for.
func (r *reconciler) createTodo(pv pairView, action, side, signalDate string, ref *money.Dec, rule *string, posID *int64, dependsOn *dbq.Ema1dTodo) (*dbq.Ema1dTodo, error) {
	var depID *int64
	if dependsOn != nil {
		depID = &dependsOn.ID
	}
	existing, err := r.q.FindTodo(r.ctx, dbq.FindTodoParams{Ema1dPairID: pv.ID, Action: action, Side: side, SignalDate: signalDate})
	if errors.Is(err, sql.ErrNoRows) {
		t, err := r.q.CreateTodo(r.ctx, dbq.CreateTodoParams{
			Ema1dPairID: pv.ID, Action: action, Side: side, SignalDate: signalDate, RefDate: &r.date,
			RefPrice: ref, ExitRule: rule, Source: SourceSync, Ema1dSyncID: &r.sync.ID, DependsOnID: depID, PositionID: posID,
		})
		if err != nil {
			return nil, err
		}
		r.created = append(r.created, t)
		return &t, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.Status == TodoPending || existing.Status == TodoRunning {
		return &existing, nil
	}
	if action == ActionClose && (existing.Status == TodoFailed || existing.Status == TodoSkipped) {
		// A close is always wanted: put it back in the queue.
		prev := existing.Status
		existing.Status, existing.Attempts, existing.RunAfter = TodoPending, 0, nil
		existing.Ema1dSyncID, existing.DependsOnID, existing.RefDate = &r.sync.ID, depID, &r.date
		if ref != nil {
			existing.RefPrice = ref
		}
		if err := saveTodo(r.ctx, r.q, existing); err != nil {
			return nil, err
		}
		r.requeued = append(r.requeued, existing)
		r.warnings = append(r.warnings, fmt.Sprintf("close_requeued: %s %s %s (was %s)", pv.CoinName, side, signalDate, prev))
		return &existing, nil
	}
	reason := "open_already_" + strings.ToLower(existing.Status)
	if action == ActionClose {
		reason = "close_already_" + strings.ToLower(existing.Status) + "_but_trade_open"
	}
	r.skipped = append(r.skipped, SkippedEntry{Coin: pv.CoinName, Action: action, Side: side, Reason: fmt.Sprintf("%s (todo #%d)", reason, existing.ID)})
	return nil, nil
}

func (r *reconciler) summaries(ts []dbq.Ema1dTodo) []TodoSummary {
	out := make([]TodoSummary, 0, len(ts))
	for _, t := range ts {
		sum := TodoSummary{Coin: r.coins[t.Ema1dPairID], Action: t.Action, Side: t.Side, SignalDate: t.SignalDate, Status: t.Status}
		if !r.dryRun {
			id := t.ID
			sum.ID, sum.DependsOnID = &id, t.DependsOnID
		}
		out = append(out, sum)
	}
	return out
}

func saveTodo(ctx context.Context, q *dbq.Queries, t dbq.Ema1dTodo) error {
	return q.SaveTodo(ctx, dbq.SaveTodoParams{
		Status: t.Status, Attempts: t.Attempts, FirstAttemptedAt: t.FirstAttemptedAt, RunAfter: t.RunAfter,
		LastError: t.LastError, Step: t.Step, PositionID: t.PositionID, DependsOnID: t.DependsOnID,
		Ema1dSyncID: t.Ema1dSyncID, RefDate: t.RefDate, RefPrice: t.RefPrice, ID: t.ID,
	})
}
