package emasettings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
)

// TradeRow is one trade for the API.
type TradeRow struct {
	ID               int64      `json:"id"`
	SettingID        int64      `json:"settingId"`
	Side             string     `json:"side"`
	Status           string     `json:"status"`
	IsAuto           bool       `json:"isAuto"`
	Amount           money.Dec  `json:"amount"`
	EntryPrice       *money.Dec `json:"entryPrice,omitempty"`
	TargetPrice      *money.Dec `json:"targetPrice,omitempty"`
	SlPrice          *money.Dec `json:"slPrice,omitempty"`
	OpenPrice        *money.Dec `json:"openPrice,omitempty"`
	ClosePrice       *money.Dec `json:"closePrice,omitempty"`
	FeeUsdt          money.Dec  `json:"feeUsdt"`
	InterestUsdt     money.Dec  `json:"interestUsdt"`
	RealizedProfit   money.Dec  `json:"realizedProfit"`
	UnrealizedProfit money.Dec  `json:"unrealizedProfit"`
	CloseReason      string     `json:"closeReason"`
	LastError        string     `json:"lastError"`
	BorrowStep       string     `json:"borrowStep"`
	Legacy           bool       `json:"legacy"`
	OpenAt           *time.Time `json:"openAt,omitempty"`
	CloseAt          *time.Time `json:"closeAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// SettingRow is a setting with its totals.
type SettingRow struct {
	ID               int64      `json:"id"`
	PairID           int64      `json:"pairId"`
	PairName         string     `json:"pairName"`
	CurrentPrice     *money.Dec `json:"currentPrice,omitempty"`
	Account          string     `json:"account"`
	Budget           money.Dec  `json:"budget"`
	TradeType        string     `json:"tradeType"`
	Period           string     `json:"period"`
	CheckType        string     `json:"checkType"`
	NextTradeType    string     `json:"nextTradeType"`
	TpPercent        money.Dec  `json:"tpPercent"`
	SlPercent        money.Dec  `json:"slPercent"`
	Status           string     `json:"status"`
	Note             string     `json:"note"`
	RealizedProfit   money.Dec  `json:"realizedProfit"`
	UnrealizedProfit money.Dec  `json:"unrealizedProfit"`
	TotalProfit      money.Dec  `json:"totalProfit"`
	TotalPercent     *money.Dec `json:"totalPercent,omitempty"`
	TradeCount       int        `json:"tradeCount"`
	ActiveTrade      *TradeRow  `json:"activeTrade,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	Trades           []TradeRow `json:"trades,omitempty"`
}

func timeOf(t *dbtype.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.Time
	return &v
}

func tradeRow(t dbq.EmaSettingTrade) TradeRow {
	return TradeRow{
		ID: t.ID, SettingID: t.SettingID, Side: t.Side, Status: t.Status, IsAuto: t.IsAuto, Amount: t.Amount,
		EntryPrice: t.EntryPrice, TargetPrice: t.TargetPrice, SlPrice: t.SlPrice, OpenPrice: t.OpenPrice,
		ClosePrice: t.ClosePrice, FeeUsdt: t.FeeUsdt, InterestUsdt: t.InterestUsdt, RealizedProfit: t.RealizedProfit,
		UnrealizedProfit: t.UnrealizedProfit, CloseReason: t.CloseReason, LastError: t.LastError, BorrowStep: t.BorrowStep,
		Legacy: t.Legacy, OpenAt: timeOf(t.OpenAt), CloseAt: timeOf(t.CloseAt), CreatedAt: t.CreatedAt.Time,
	}
}

func settingRow(r dbq.GetEmaSettingRow, trades []dbq.EmaSettingTrade) SettingRow {
	s := r.EmaSetting
	sum := Summarize(s.Budget, trades)
	row := SettingRow{
		ID: s.ID, PairID: s.PairID, PairName: r.PairName, CurrentPrice: r.CurrentPrice, Account: "CROSS",
		Budget: s.Budget, TradeType: s.TradeType, Period: s.Period, CheckType: s.CheckType, NextTradeType: s.NextTradeType,
		TpPercent: s.TpPercent, SlPercent: s.SlPercent, Status: s.Status, Note: s.Note,
		RealizedProfit: sum.Realized, UnrealizedProfit: sum.Unrealized, TotalProfit: sum.Total, TotalPercent: sum.TotalPercent,
		TradeCount: len(trades), CreatedAt: s.CreatedAt.Time, UpdatedAt: s.UpdatedAt.Time,
	}
	switch {
	case r.IsSpot:
		row.Account = "SPOT"
	case r.IsIsolated:
		row.Account = "ISOLATED"
	}
	for _, t := range trades { // newest first: the latest active trade wins
		if t.Status == TradeOpening || t.Status == TradeOpen {
			tr := tradeRow(t)
			row.ActiveTrade = &tr
			break
		}
	}
	return row
}

// List returns the settings (all, or of one status) with their totals.
func (s *Service) List(ctx context.Context, status string) ([]SettingRow, error) {
	q := dbq.New(s.DB.Read)
	var filter any
	if status != "" {
		filter = status
	}
	rows, err := q.ListEmaSettings(ctx, filter)
	if err != nil {
		return nil, err
	}
	all, err := q.ListAllEmaSettingTrades(ctx)
	if err != nil {
		return nil, err
	}
	bySetting := map[int64][]dbq.EmaSettingTrade{}
	for _, t := range all {
		bySetting[t.SettingID] = append(bySetting[t.SettingID], t)
	}
	out := make([]SettingRow, len(rows))
	for i, r := range rows {
		out[i] = settingRow(dbq.GetEmaSettingRow(r), bySetting[r.EmaSetting.ID])
	}
	return out, nil
}

// Get returns one setting with its trades, newest first.
func (s *Service) Get(ctx context.Context, id int64) (SettingRow, error) {
	r, err := s.getRow(ctx, id)
	if err != nil {
		return SettingRow{}, err
	}
	trades, err := s.trades(ctx, id)
	if err != nil {
		return SettingRow{}, err
	}
	row := settingRow(r, trades)
	row.Trades = make([]TradeRow, len(trades))
	for i, t := range trades {
		row.Trades[i] = tradeRow(t)
	}
	return row, nil
}

func (s *Service) getRow(ctx context.Context, id int64) (dbq.GetEmaSettingRow, error) {
	r, err := dbq.New(s.DB.Read).GetEmaSetting(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return r, httpx.NewError(404, "ema_setting_not_found", "EMA setting not found")
	}
	return r, err
}

func (s *Service) getTrade(ctx context.Context, id int64) (dbq.EmaSettingTrade, dbq.GetEmaSettingRow, error) {
	t, err := dbq.New(s.DB.Read).GetEmaSettingTrade(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return t, dbq.GetEmaSettingRow{}, httpx.NewError(404, "ema_setting_trade_not_found", "EMA setting trade not found")
	}
	if err != nil {
		return t, dbq.GetEmaSettingRow{}, err
	}
	r, err := s.getRow(ctx, t.SettingID)
	return t, r, err
}

// ----------------------------------------------------------------------------------- settings

// SettingInput creates (PairID and Budget required) or patches a setting: nil fields stay.
type SettingInput struct {
	PairID        int64
	Budget        *money.Dec
	TradeType     *string
	Period        *string
	CheckType     *string
	NextTradeType *string
	TpPercent     *money.Dec
	SlPercent     *money.Dec
	Status        *string
	Note          *string
}

func (in SettingInput) apply(set *dbq.EmaSetting) error {
	pick := func(v *string, dst *string, allowed ...string) error {
		if v == nil {
			return nil
		}
		if !slices.Contains(allowed, *v) {
			return httpx.Validation(fmt.Sprintf("%q is not one of %v", *v, allowed), nil)
		}
		*dst = *v
		return nil
	}
	nonNeg := func(name string, v *money.Dec, dst *money.Dec) error {
		if v == nil {
			return nil
		}
		if v.IsNegative() {
			return httpx.Validation(name+" must not be negative", nil)
		}
		*dst = *v
		return nil
	}
	return errors.Join(
		nonNeg("budget", in.Budget, &set.Budget),
		nonNeg("tpPercent", in.TpPercent, &set.TpPercent),
		nonNeg("slPercent", in.SlPercent, &set.SlPercent),
		pick(in.TradeType, &set.TradeType, OnlyLong, OnlyShort, Both),
		pick(in.Period, &set.Period, PeriodD1, PeriodH4),
		pick(in.CheckType, &set.CheckType, CheckAtOpen, CheckAtClose, CheckAtCurrent),
		pick(in.NextTradeType, &set.NextTradeType, NewTrade, NotNewTrade),
		pick(in.Status, &set.Status, StatusActive, StatusInactive, StatusDone),
	)
}

func (s *Service) CreateSetting(ctx context.Context, in SettingInput) (SettingRow, error) {
	if in.Budget == nil || !in.Budget.IsPositive() {
		return SettingRow{}, httpx.Validation("budget must be greater than 0", nil)
	}
	pr, err := dbq.New(s.DB.Read).GetPair(ctx, in.PairID)
	p := pr.Pair
	if errors.Is(err, sql.ErrNoRows) || (err == nil && p.DeletedAt != nil) {
		return SettingRow{}, httpx.Validation("pair not found", nil)
	}
	if err != nil {
		return SettingRow{}, err
	}
	if p.IsIsolated {
		return SettingRow{}, httpx.Validation(p.Name+" is on isolated margin, not supported", nil)
	}
	set := dbq.EmaSetting{TradeType: Both, Period: PeriodD1, CheckType: CheckAtOpen, NextTradeType: NewTrade,
		TpPercent: money.Zero, SlPercent: money.Zero, Status: StatusActive}
	if in.Note != nil {
		set.Note = *in.Note
	}
	if err := in.apply(&set); err != nil {
		return SettingRow{}, err
	}
	if set.Status == StatusDone {
		return SettingRow{}, httpx.Validation("a new setting cannot be DONE", nil)
	}
	created, err := dbq.New(s.DB.Write).CreateEmaSetting(ctx, dbq.CreateEmaSettingParams{
		PairID: in.PairID, Budget: set.Budget, TradeType: set.TradeType, Period: set.Period, CheckType: set.CheckType,
		NextTradeType: set.NextTradeType, TpPercent: set.TpPercent, SlPercent: set.SlPercent, Status: set.Status, Note: set.Note,
	})
	if err != nil {
		return SettingRow{}, err
	}
	s.changed()
	return s.Get(ctx, created.ID)
}

func (s *Service) UpdateSetting(ctx context.Context, id int64, in SettingInput) (SettingRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getRow(ctx, id)
	if err != nil {
		return SettingRow{}, err
	}
	set := r.EmaSetting
	if in.Note != nil {
		set.Note = *in.Note
	}
	if err := in.apply(&set); err != nil {
		return SettingRow{}, err
	}
	if set.Status == StatusDone && r.EmaSetting.Status != StatusDone {
		trades, err := s.trades(ctx, id)
		if err != nil {
			return SettingRow{}, err
		}
		if hasActive(trades) {
			return SettingRow{}, httpx.Unprocessable("ema_setting_active", "close the setting to end it: it still has trades waiting or held")
		}
	}
	if err := dbq.New(s.DB.Write).UpdateEmaSetting(ctx, dbq.UpdateEmaSettingParams{
		Budget: set.Budget, TradeType: set.TradeType, Period: set.Period, CheckType: set.CheckType, NextTradeType: set.NextTradeType,
		TpPercent: set.TpPercent, SlPercent: set.SlPercent, Status: set.Status, Note: set.Note, ID: id,
	}); err != nil {
		return SettingRow{}, err
	}
	s.changed()
	return s.Get(ctx, id)
}

func (s *Service) DeleteSetting(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.getRow(ctx, id); err != nil {
		return err
	}
	trades, err := s.trades(ctx, id)
	if err != nil {
		return err
	}
	if hasActive(trades) {
		return httpx.Unprocessable("ema_setting_active", "the setting has trades waiting or held: close it first")
	}
	if err := dbq.New(s.DB.Write).DeleteEmaSetting(ctx, id); err != nil {
		return err
	}
	s.changed()
	return nil
}

// CloseSetting closes a setting by hand: waiting trades cancelled, held ones closed at market.
func (s *Service) CloseSetting(ctx context.Context, id int64) (SettingRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getRow(ctx, id)
	if err != nil {
		return SettingRow{}, err
	}
	if r.EmaSetting.Status == StatusDone {
		return SettingRow{}, httpx.Unprocessable("ema_setting_done", "the setting is already DONE")
	}
	err = s.closeSetting(ctx, r.EmaSetting, viewOf(r), ReasonManual)
	s.changed()
	if err != nil {
		return SettingRow{}, actionError(err)
	}
	return s.Get(ctx, id)
}

// actionError turns a refusal into a 422 so the screen shows the reason.
func actionError(err error) error {
	var he *httpx.Error
	if errors.As(err, &he) {
		return err
	}
	if errors.Is(err, errPermanent) {
		return httpx.Unprocessable("refused", err.Error())
	}
	return httpx.Unprocessable("exchange_failed", err.Error())
}

// ------------------------------------------------------------------------------------- trades

type newTrade struct {
	Side        string
	Status      string
	Amount      money.Dec
	EntryPrice  *money.Dec
	TargetPrice *money.Dec
	SlPrice     *money.Dec
	OpenPrice   *money.Dec
	OpenAt      *time.Time
	Auto        bool
}

// createTrade inserts a trade and gives it its id-based client order ids in the same transaction.
func (s *Service) createTrade(ctx context.Context, set dbq.EmaSetting, in newTrade) (dbq.EmaSettingTrade, error) {
	var t dbq.EmaSettingTrade
	var openAt *dbtype.Time
	if in.OpenAt != nil {
		openAt = ptr(dbtype.NewTime(*in.OpenAt))
	}
	err := s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		tmp := fmt.Sprintf("tmp_%d_%d", set.ID, s.now().UnixNano())
		var err error
		t, err = q.CreateEmaSettingTrade(ctx, dbq.CreateEmaSettingTradeParams{
			SettingID: set.ID, PairID: set.PairID, Side: in.Side, Status: in.Status, IsAuto: in.Auto, Amount: in.Amount,
			EntryPrice: in.EntryPrice, TargetPrice: in.TargetPrice, SlPrice: in.SlPrice, OpenPrice: in.OpenPrice,
			UnrealizedProfit: money.Zero, OpenOrderID: tmp + "_o", CloseOrderID: tmp + "_c", OpenAt: openAt,
		})
		if err != nil {
			return err
		}
		t.OpenOrderID, t.CloseOrderID = openOrderID(t.ID), closeOrderID(t.ID)
		return q.SetEmaSettingTradeOrderIDs(ctx, dbq.SetEmaSettingTradeOrderIDsParams{OpenOrderID: t.OpenOrderID, CloseOrderID: t.CloseOrderID, ID: t.ID})
	})
	return t, err
}

// positiveOrNil turns 0 into "not set".
func positiveOrNil(d *money.Dec) *money.Dec {
	if d == nil || !d.IsPositive() {
		return nil
	}
	return d
}

// TradeInput adds a trade by hand: OPENING waits like an auto trade (entry price or the cross),
// OPEN records a position already held (no order is placed).
type TradeInput struct {
	Side        string
	Status      string
	Amount      *money.Dec
	EntryPrice  *money.Dec
	TargetPrice *money.Dec
	SlPrice     *money.Dec
	OpenPrice   *money.Dec
	OpenAt      *time.Time
}

func (s *Service) CreateTrade(ctx context.Context, settingID int64, in TradeInput) (TradeRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.getRow(ctx, settingID)
	if err != nil {
		return TradeRow{}, err
	}
	if r.EmaSetting.Status == StatusDone {
		return TradeRow{}, httpx.Unprocessable("ema_setting_done", "the setting is DONE")
	}
	if in.Side != SideLong && in.Side != SideShort {
		return TradeRow{}, httpx.Validation("side must be LONG or SHORT", nil)
	}
	nt := newTrade{Side: in.Side, Status: in.Status, EntryPrice: positiveOrNil(in.EntryPrice),
		TargetPrice: positiveOrNil(in.TargetPrice), SlPrice: positiveOrNil(in.SlPrice), Amount: money.Zero}
	if in.Amount != nil {
		if in.Amount.IsNegative() {
			return TradeRow{}, httpx.Validation("amount must not be negative", nil)
		}
		nt.Amount = *in.Amount
	}
	switch in.Status {
	case "", TradeOpening:
		nt.Status = TradeOpening
	case TradeOpen:
		if !nt.Amount.IsPositive() || positiveOrNil(in.OpenPrice) == nil {
			return TradeRow{}, httpx.Validation("a held trade needs its amount and open price", nil)
		}
		nt.OpenPrice, nt.EntryPrice = in.OpenPrice, nil
		at := s.now()
		if in.OpenAt != nil {
			at = *in.OpenAt
		}
		nt.OpenAt = &at
	default:
		return TradeRow{}, httpx.Validation("status must be OPENING or OPEN", nil)
	}
	t, err := s.createTrade(ctx, r.EmaSetting, nt)
	if err != nil {
		return TradeRow{}, err
	}
	s.changed()
	return tradeRow(t), nil
}

// TradeUpdate edits a trade: OPENING its side, amount and prices; OPEN its amount, target and stop
// loss. Status CLOSED cancels an OPENING trade, or records an OPEN one closed by hand on the
// exchange at ClosePrice (no order is placed). A price of 0 clears it.
type TradeUpdate struct {
	Side        *string
	Amount      *money.Dec
	EntryPrice  *money.Dec
	TargetPrice *money.Dec
	SlPrice     *money.Dec
	Status      *string
	ClosePrice  *money.Dec
}

func (s *Service) UpdateTrade(ctx context.Context, id int64, in TradeUpdate) (TradeRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, _, err := s.getTrade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	if t.Status == TradeClosed {
		return TradeRow{}, httpx.Unprocessable("ema_setting_trade_closed", "the trade is closed")
	}
	if in.Amount != nil {
		if !in.Amount.IsPositive() {
			return TradeRow{}, httpx.Validation("amount must be greater than 0", nil)
		}
		t.Amount = *in.Amount
	}
	if in.TargetPrice != nil {
		t.TargetPrice = positiveOrNil(in.TargetPrice)
	}
	if in.SlPrice != nil {
		t.SlPrice = positiveOrNil(in.SlPrice)
	}
	if t.Status == TradeOpening {
		if in.Side != nil {
			if *in.Side != SideLong && *in.Side != SideShort {
				return TradeRow{}, httpx.Validation("side must be LONG or SHORT", nil)
			}
			if t.BorrowStep != "" && *in.Side != t.Side {
				return TradeRow{}, httpx.Unprocessable("ema_setting_trade_borrowed", "the short borrow was sent: the side cannot change")
			}
			t.Side = *in.Side
		}
		if in.EntryPrice != nil {
			t.EntryPrice = positiveOrNil(in.EntryPrice)
		}
	} else if in.Side != nil && *in.Side != t.Side {
		return TradeRow{}, httpx.Unprocessable("ema_setting_trade_open", "a held trade cannot change side")
	}
	q := dbq.New(s.DB.Write)
	if in.Status != nil && *in.Status == TradeClosed {
		if t.Status == TradeOpening {
			if err := cancelTrade(ctx, q, &t, s.now()); err != nil {
				return TradeRow{}, actionError(err)
			}
			s.changed()
			return tradeRow(t), nil
		}
		cp := positiveOrNil(in.ClosePrice)
		if cp == nil {
			return TradeRow{}, httpx.Validation("closePrice is required to record a trade closed by hand", nil)
		}
		gross := cp.Sub(deref(t.OpenPrice)).Mul(t.Amount)
		if t.Side == SideShort {
			gross = gross.Neg()
		}
		t.ClosePrice, t.RealizedProfit, t.UnrealizedProfit = cp, gross.Sub(t.FeeUsdt).Round(8), money.Zero
		t.Status, t.CloseReason, t.CloseAt = TradeClosed, ReasonManual, ptr(dbtype.NewTime(s.now()))
	} else if in.Status != nil && *in.Status != t.Status {
		return TradeRow{}, httpx.Validation("status can only be set to CLOSED", nil)
	}
	if err := saveTrade(ctx, q, t); err != nil {
		return TradeRow{}, err
	}
	s.changed()
	return tradeRow(t), nil
}

func (s *Service) DeleteTrade(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, _, err := s.getTrade(ctx, id)
	if err != nil {
		return err
	}
	if t.Status != TradeOpening {
		return httpx.Unprocessable("ema_setting_trade_not_opening", "only a waiting (OPENING) trade can be deleted")
	}
	if t.BorrowStep != "" {
		return httpx.Unprocessable("ema_setting_trade_borrowed", "the short borrow was sent: open the trade or repay by hand first")
	}
	if err := dbq.New(s.DB.Write).DeleteEmaSettingTrade(ctx, id); err != nil {
		return err
	}
	s.changed()
	return nil
}

// ForceOpen enters an OPENING trade at market now, whatever the EMA says.
func (s *Service) ForceOpen(ctx context.Context, id int64) (TradeRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, r, err := s.getTrade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return TradeRow{}, err
	}
	err = s.openTrade(ctx, r.EmaSetting, &t, viewOf(r), cfg)
	s.changed()
	if err != nil {
		_ = s.failed(ctx, &t, "open", err)
		return TradeRow{}, actionError(err)
	}
	s.notify(ctx, fmt.Sprintf("EMA %s: %s MANUAL OPENED %s #%d @ %s · amount %s",
		r.EmaSetting.Period, r.PairName, t.Side, t.ID, deref(t.OpenPrice).Round(8), t.Amount))
	return tradeRow(t), nil
}

// ForceClose exits an OPEN trade at market now.
func (s *Service) ForceClose(ctx context.Context, id int64) (TradeRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, r, err := s.getTrade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	err = s.closeTrade(ctx, &t, viewOf(r), ReasonManual)
	s.changed()
	if err != nil {
		_ = s.failed(ctx, &t, "close", err)
		return TradeRow{}, actionError(err)
	}
	s.notify(ctx, fmt.Sprintf("EMA %s: %s MANUAL CLOSED %s #%d @ %s · PnL %s USDT",
		r.EmaSetting.Period, r.PairName, t.Side, t.ID, deref(t.ClosePrice).Round(8), t.RealizedProfit.Round(2)))
	return tradeRow(t), nil
}

// splitOff moves amount of t into a new trade with the same prices; the open fee and unrealized
// PnL are shared out pro rata. Both rows are written in one transaction.
func (s *Service) splitOff(ctx context.Context, set dbq.EmaSetting, t *dbq.EmaSettingTrade, amount money.Dec) (dbq.EmaSettingTrade, error) {
	if !amount.IsPositive() || amount.GreaterOrEqual(t.Amount) {
		return dbq.EmaSettingTrade{}, httpx.Validation(fmt.Sprintf("amount must be between 0 and %s", t.Amount), nil)
	}
	if t.BorrowStep != "" && t.Status == TradeOpening {
		return dbq.EmaSettingTrade{}, httpx.Unprocessable("ema_setting_trade_borrowed", "the short borrow was sent: open the trade first")
	}
	share := amount.Div(t.Amount)
	fee, unrealized := t.FeeUsdt.Mul(share).Round(8), t.UnrealizedProfit.Mul(share).Round(8)
	part, err := s.createTrade(ctx, set, newTrade{Side: t.Side, Status: t.Status, Amount: amount, EntryPrice: t.EntryPrice,
		TargetPrice: t.TargetPrice, SlPrice: t.SlPrice, OpenPrice: t.OpenPrice, OpenAt: timeOf(t.OpenAt)})
	if err != nil {
		return part, err
	}
	part.FeeUsdt, part.UnrealizedProfit = fee, unrealized
	t.Amount, t.FeeUsdt, t.UnrealizedProfit = t.Amount.Sub(amount), t.FeeUsdt.Sub(fee), t.UnrealizedProfit.Sub(unrealized)
	err = s.DB.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		if err := saveTrade(ctx, q, part); err != nil {
			return err
		}
		return saveTrade(ctx, q, *t)
	})
	return part, err
}

// Split moves amount into a new trade without any order.
func (s *Service) Split(ctx context.Context, id int64, amount money.Dec) (TradeRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, r, err := s.getTrade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	if t.Status == TradeClosed {
		return TradeRow{}, httpx.Unprocessable("ema_setting_trade_closed", "the trade is closed")
	}
	part, err := s.splitOff(ctx, r.EmaSetting, &t, amount)
	if err != nil {
		return TradeRow{}, err
	}
	s.changed()
	return tradeRow(part), nil
}

// PartialClose splits amount off an OPEN trade and closes that part at market.
func (s *Service) PartialClose(ctx context.Context, id int64, amount money.Dec) (TradeRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, r, err := s.getTrade(ctx, id)
	if err != nil {
		return TradeRow{}, err
	}
	if t.Status != TradeOpen {
		return TradeRow{}, httpx.Unprocessable("ema_setting_trade_not_open", "only a held (OPEN) trade can be partly closed")
	}
	part, err := s.splitOff(ctx, r.EmaSetting, &t, amount)
	if err != nil {
		return TradeRow{}, err
	}
	err = s.closeTrade(ctx, &part, viewOf(r), ReasonManual)
	s.changed()
	if err != nil {
		_ = s.failed(ctx, &part, "close", err)
		return TradeRow{}, actionError(fmt.Errorf("split off as trade #%d but its close failed (close it again): %w", part.ID, err))
	}
	s.notify(ctx, fmt.Sprintf("EMA %s: %s PARTIAL CLOSE %s #%d → #%d %s @ %s · PnL %s USDT",
		r.EmaSetting.Period, r.PairName, t.Side, t.ID, part.ID, part.Amount, deref(part.ClosePrice).Round(8), part.RealizedProfit.Round(2)))
	return tradeRow(part), nil
}
