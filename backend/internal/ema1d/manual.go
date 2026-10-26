package ema1d

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
)

// ManualTodoRequest asks for one OPEN or CLOSE by hand, when a sync cannot fix things.
type ManualTodoRequest struct {
	Action     string // OPEN | CLOSE
	Coin       string
	Side       string // required for OPEN; checked against the position for CLOSE
	SignalDate string // OPEN: YYYY-MM-DD
	RefPrice   string
	Rule       string
	// Force lifts the entry age limit (only).
	Force bool
}

type ManualTodoResult struct {
	Created bool    `json:"created"`
	Todo    TodoRow `json:"todo"`
}

// CreateManualTodo runs the same checks as the sync. Asking again for a todo already queued
// returns it.
func (s *Service) CreateManualTodo(ctx context.Context, req ManualTodoRequest) (ManualTodoResult, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return ManualTodoResult{}, err
	}
	coin := strings.ToUpper(strings.TrimSpace(req.Coin))
	if coin == "" {
		return ManualTodoResult{}, invalidRequest("coin is required")
	}
	q := dbq.New(s.DB.Read)
	r, err := q.GetEma1dPairByCoin(ctx, coin)
	if isNoRows(err) {
		return ManualTodoResult{}, invalidRequest(coin + " is not an EMA1D pair")
	}
	if err != nil {
		return ManualTodoResult{}, err
	}
	pv := pairView{r.Ema1dPair, r.PairName, r.CoinName, r.CurrentPrice, r.StepSize, r.MinNotional, r.IsSpot, r.IsIsolated}

	side := strings.ToUpper(strings.TrimSpace(req.Side))
	if side != "" && side != SideLong && side != SideShort {
		return ManualTodoResult{}, invalidRequest("side must be LONG or SHORT")
	}
	var ref *money.Dec
	if strings.TrimSpace(req.RefPrice) != "" {
		px, err := money.Parse(req.RefPrice)
		if err != nil {
			return ManualTodoResult{}, invalidRequest("ref_price must be a number")
		}
		if !px.IsPositive() {
			return ManualTodoResult{}, invalidRequest("ref_price must be positive")
		}
		ref = &px
	}
	var rule *string
	if r := strings.ToUpper(strings.TrimSpace(req.Rule)); r != "" && r != "-" {
		if r != "DIV4H" && r != "XO" {
			return ManualTodoResult{}, invalidRequest("rule must be DIV4H or XO")
		}
		rule = &r
	}

	switch strings.ToUpper(req.Action) {
	case ActionOpen:
		if side == "" {
			return ManualTodoResult{}, invalidRequest("side is required (LONG or SHORT)")
		}
		if req.SignalDate == "" {
			return ManualTodoResult{}, invalidRequest("signal_date is required (YYYY-MM-DD)")
		}
		if _, err := parseDate(req.SignalDate); err != nil {
			return ManualTodoResult{}, invalidRequest("signal_date must be YYYY-MM-DD")
		}
		return s.manualOpen(ctx, pv, cfg, side, req.SignalDate, ref, rule, req.Force)
	case ActionClose:
		return s.manualClose(ctx, pv, side, ref, rule)
	}
	return ManualTodoResult{}, invalidRequest("action must be OPEN or CLOSE")
}

func (s *Service) manualOpen(ctx context.Context, pv pairView, cfg Config, side, signalDate string, ref *money.Dec, rule *string, force bool) (ManualTodoResult, error) {
	q := dbq.New(s.DB.Read)
	todos, err := q.InFlightTodosForPair(ctx, pv.ID)
	if err != nil {
		return ManualTodoResult{}, err
	}
	for _, t := range todos {
		if t.Action == ActionOpen && t.Side == side && t.SignalDate == signalDate {
			return ManualTodoResult{Created: false, Todo: todoRow(t, pv.CoinName)}, nil
		}
	}
	pos, err := activePosition(ctx, q, pv.ID)
	if err != nil {
		return ManualTodoResult{}, err
	}
	maxAge := cfg.MaxEntryAgeDays
	var reason string
	switch {
	case pv.Status != PairActive:
		reason = "pair is " + pv.Status
	case !pv.sideAllowed(side):
		reason = side + " not allowed or not supported"
	case cfg.PauseOpen:
		reason = "pause open is on"
	case pos != nil:
		reason = fmt.Sprintf("already holding position #%d", pos.ID)
	case hasAction(todos, ActionOpen):
		reason = "another OPEN is queued"
	case signalDate < pv.AddedOn:
		reason = "entry before added_on " + pv.AddedOn
	case !force && signalDate < addDays(dateOnly(s.now()), -maxAge):
		reason = fmt.Sprintf("entry older than %d day(s), add force", maxAge)
	}
	if reason != "" {
		return ManualTodoResult{}, invalidRequest(fmt.Sprintf("cannot open %s %s: %s", side, pv.CoinName, reason))
	}
	var blocker *int64
	for _, t := range todos {
		if t.Action == ActionClose {
			id := t.ID
			blocker = &id
			break
		}
	}
	return s.createManual(ctx, pv, ActionOpen, side, signalDate, ref, rule, nil, blocker)
}

func hasAction(todos []dbq.Ema1dTodo, action string) bool {
	for _, t := range todos {
		if t.Action == action {
			return true
		}
	}
	return false
}

func (s *Service) manualClose(ctx context.Context, pv pairView, side string, ref *money.Dec, rule *string) (ManualTodoResult, error) {
	q := dbq.New(s.DB.Read)
	pos, err := activePosition(ctx, q, pv.ID)
	if err != nil {
		return ManualTodoResult{}, err
	}
	if pos == nil {
		return ManualTodoResult{}, invalidRequest("no open EMA1D position on " + pv.CoinName + " to close")
	}
	if side != "" && side != pos.Side {
		return ManualTodoResult{}, invalidRequest(fmt.Sprintf("the open position on %s is %s", pv.CoinName, pos.Side))
	}
	todos, err := q.InFlightTodosForPair(ctx, pv.ID)
	if err != nil {
		return ManualTodoResult{}, err
	}
	for _, t := range todos {
		if t.Action == ActionClose && t.Side == pos.Side && t.SignalDate == pos.SignalDate {
			return ManualTodoResult{Created: false, Todo: todoRow(t, pv.CoinName)}, nil
		}
	}
	id := pos.ID
	return s.createManual(ctx, pv, ActionClose, pos.Side, pos.SignalDate, ref, rule, &id, nil)
}

func (s *Service) createManual(ctx context.Context, pv pairView, action, side, signalDate string, ref *money.Dec, rule *string, posID, dependsOn *int64) (ManualTodoResult, error) {
	q := dbq.New(s.DB.Write)
	prev, err := q.FindTodo(ctx, dbq.FindTodoParams{Ema1dPairID: pv.ID, Action: action, Side: side, SignalDate: signalDate})
	if err == nil {
		return ManualTodoResult{}, invalidRequest(fmt.Sprintf("todo #%d for this already exists (%s): retry it instead", prev.ID, prev.Status))
	}
	if !isNoRows(err) {
		return ManualTodoResult{}, err
	}
	today := dateOnly(s.now())
	t, err := q.CreateTodo(ctx, dbq.CreateTodoParams{
		Ema1dPairID: pv.ID, Action: action, Side: side, SignalDate: signalDate, RefDate: &today, RefPrice: ref,
		ExitRule: rule, Source: SourceManual, DependsOnID: dependsOn, PositionID: posID,
	})
	if err != nil {
		return ManualTodoResult{}, err
	}
	s.changed()
	return ManualTodoResult{Created: true, Todo: todoRow(t, pv.CoinName)}, nil
}

func (s *Service) todoWithCoin(ctx context.Context, id int64) (dbq.Ema1dTodo, string, error) {
	q := dbq.New(s.DB.Read)
	t, err := q.GetTodo(ctx, id)
	if isNoRows(err) {
		return t, "", httpx.NewError(404, "todo_not_found", "không tìm thấy todo")
	}
	if err != nil {
		return t, "", err
	}
	pv, err := s.getPair(ctx, q, t.Ema1dPairID)
	return t, pv.CoinName, err
}

// RetryTodo puts a FAILED/SKIPPED todo back in the queue with a fresh retry budget.
func (s *Service) RetryTodo(ctx context.Context, id int64) (TodoRow, error) {
	t, coin, err := s.todoWithCoin(ctx, id)
	if err != nil {
		return TodoRow{}, err
	}
	if t.Status != TodoFailed && t.Status != TodoSkipped {
		return TodoRow{}, httpx.Conflict("not_retryable", "chỉ retry được todo FAILED hoặc SKIPPED (todo này đang "+t.Status+")")
	}
	t.Status, t.Attempts, t.RunAfter = TodoPending, 0, nil
	// Keep first_attempted_at while a borrow may be in flight: the guard searches history from it.
	if t.Step == nil {
		t.FirstAttemptedAt = nil
	}
	if err := saveTodo(ctx, dbq.New(s.DB.Write), t); err != nil {
		return TodoRow{}, err
	}
	s.changed()
	return todoRow(t, coin), nil
}

// CancelTodo cancels a todo that never reached Binance.
func (s *Service) CancelTodo(ctx context.Context, id int64) (TodoRow, error) {
	t, coin, err := s.todoWithCoin(ctx, id)
	if err != nil {
		return TodoRow{}, err
	}
	if !cancellable(t) {
		return TodoRow{}, httpx.Conflict("not_cancellable", fmt.Sprintf("todo #%d có thể đã gửi lên Binance nên không huỷ được", t.ID))
	}
	t.Status, t.LastError = TodoCancelled, "cancelled from the screen"
	if err := saveTodo(ctx, dbq.New(s.DB.Write), t); err != nil {
		return TodoRow{}, err
	}
	s.changed()
	return todoRow(t, coin), nil
}

// PairUpdate edits a basket pair. Nil fields are kept.
type PairUpdate struct {
	PairGroup    *string
	Status       *string
	AllowLong    *bool
	AllowShort   *bool
	AddedOn      *string
	RemovedOn    *string // "" clears
	LongExitRule *string // "" clears
	SignalFrom   *string // "" clears
}

// UpdatePair edits a basket pair. Taking a coin out of the basket: EXIT_ONLY keeps closing its
// position and removed_on dates it.
func (s *Service) UpdatePair(ctx context.Context, id int64, in PairUpdate) (PairRow, error) {
	var out PairRow
	err := s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		cur, err := q.GetEma1dPair(ctx, id)
		if isNoRows(err) {
			return httpx.NewError(404, "pair_not_found", "không tìm thấy pair EMA1D")
		}
		if err != nil {
			return err
		}
		p := cur.Ema1dPair
		if in.PairGroup != nil {
			if *in.PairGroup != GroupCore && *in.PairGroup != GroupSensitive {
				return httpx.Validation("pairGroup must be CORE or SENSITIVE", nil)
			}
			p.PairGroup = *in.PairGroup
		}
		if in.Status != nil {
			st := *in.Status
			if st != PairActive && st != PairExitOnly && st != PairDisabled {
				return httpx.Validation("status must be ACTIVE, EXIT_ONLY or DISABLED", nil)
			}
			if st == PairExitOnly && p.RemovedOn == nil && in.RemovedOn == nil {
				today := dateOnly(s.now())
				p.RemovedOn = &today
			}
			if st == PairActive {
				p.RemovedOn = nil
			}
			p.Status = st
		}
		if in.AllowLong != nil {
			p.AllowLong = *in.AllowLong
		}
		if in.AllowShort != nil {
			p.AllowShort = *in.AllowShort
		}
		for _, d := range []struct {
			in  *string
			dst **string
			req bool
		}{{in.AddedOn, nil, true}, {in.RemovedOn, &p.RemovedOn, false}, {in.SignalFrom, &p.SignalFrom, false}} {
			if d.in == nil {
				continue
			}
			if *d.in == "" && !d.req {
				*d.dst = nil
				continue
			}
			if _, err := parseDate(*d.in); err != nil {
				return httpx.Validation("dates must be YYYY-MM-DD", nil)
			}
			if d.req {
				p.AddedOn = *d.in
			} else {
				v := *d.in
				*d.dst = &v
			}
		}
		if in.LongExitRule != nil {
			switch *in.LongExitRule {
			case "":
				p.LongExitRule = nil
			case "DIV4H", "XO":
				v := *in.LongExitRule
				p.LongExitRule = &v
			default:
				return httpx.Validation("longExitRule must be DIV4H or XO", nil)
			}
		}
		return q.UpdateEma1dPair(ctx, dbq.UpdateEma1dPairParams{
			PairGroup: p.PairGroup, Status: p.Status, AllowLong: p.AllowLong, AllowShort: p.AllowShort,
			AddedOn: p.AddedOn, RemovedOn: p.RemovedOn, LongExitRule: p.LongExitRule, SignalFrom: p.SignalFrom, ID: p.ID,
		})
	})
	if err != nil {
		return out, err
	}
	rows, err := s.PairRows(ctx)
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		if r.ID == id {
			out = r
		}
	}
	s.changed()
	return out, nil
}

// SaveConfig validates and stores a new config version.
func (s *Service) SaveConfig(ctx context.Context, version int64, c Config, user string) (int64, error) {
	if err := c.Validate(); err != nil {
		return 0, httpx.Validation(err.Error(), nil)
	}
	v, err := s.Configs.Save(ctx, StrategyKey, version, c, user)
	if err != nil {
		return 0, err
	}
	s.changed()
	return v, nil
}
