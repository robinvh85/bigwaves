package ema1d

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
)

type Capital struct {
	BudgetUsdt           money.Dec `json:"budgetUsdt"`
	RealizedProfitUsdt   money.Dec `json:"realizedProfitUsdt"`
	EquityUsdt           money.Dec `json:"equityUsdt"`
	UnrealizedProfitUsdt money.Dec `json:"unrealizedProfitUsdt"`
	InterestPaidUsdt     money.Dec `json:"interestPaidUsdt"`
	FeePaidUsdt          money.Dec `json:"feePaidUsdt"`
	ClosedPositions      int       `json:"closedPositions"`
}

type Exposure struct {
	LongUsdt     money.Dec  `json:"longUsdt"`
	ShortUsdt    money.Dec  `json:"shortUsdt"`
	LongPercent  *money.Dec `json:"longPercent,omitempty"`
	ShortPercent *money.Dec `json:"shortPercent,omitempty"`
	LongCount    int        `json:"longCount"`
	ShortCount   int        `json:"shortCount"`
}

type MarginStatus struct {
	Value *money.Dec `json:"value,omitempty"`
	At    *time.Time `json:"at,omitempty"`
	Warn  money.Dec  `json:"warn"`
	Alert money.Dec  `json:"alert"`
}

type PositionRow struct {
	ID                   int64      `json:"id"`
	Coin                 string     `json:"coin"`
	PairName             string     `json:"pairName"`
	Side                 string     `json:"side"`
	Status               string     `json:"status"`
	SignalDate           string     `json:"signalDate"`
	ExitRule             *string    `json:"exitRule,omitempty"`
	Amount               money.Dec  `json:"amount"`
	OpenPrice            *money.Dec `json:"openPrice,omitempty"`
	RefOpenPrice         *money.Dec `json:"refOpenPrice,omitempty"`
	OpenSlippagePercent  *money.Dec `json:"openSlippagePercent,omitempty"`
	ClosePrice           *money.Dec `json:"closePrice,omitempty"`
	RefClosePrice        *money.Dec `json:"refClosePrice,omitempty"`
	CloseSlippagePercent *money.Dec `json:"closeSlippagePercent,omitempty"`
	CurrentPrice         *money.Dec `json:"currentPrice,omitempty"`
	PriceUpdatedAt       *time.Time `json:"priceUpdatedAt,omitempty"`
	NotionalUsdt         money.Dec  `json:"notionalUsdt"`
	SizePercent          money.Dec  `json:"sizePercent"`
	EquityPercent        *money.Dec `json:"equityPercent,omitempty"`
	ProfitUsdt           money.Dec  `json:"profitUsdt"`
	ProfitPercent        *money.Dec `json:"profitPercent,omitempty"`
	FeeUsdt              money.Dec  `json:"feeUsdt"`
	InterestUsdt         money.Dec  `json:"interestUsdt"`
	OpenAt               *time.Time `json:"openAt,omitempty"`
	CloseAt              *time.Time `json:"closeAt,omitempty"`
	DaysHeld             *int       `json:"daysHeld,omitempty"`
}

type TodoRow struct {
	ID          int64      `json:"id"`
	Coin        string     `json:"coin"`
	Action      string     `json:"action"`
	Side        string     `json:"side"`
	SignalDate  string     `json:"signalDate"`
	Status      string     `json:"status"`
	Source      string     `json:"source"`
	RefPrice    *money.Dec `json:"refPrice,omitempty"`
	ExitRule    *string    `json:"exitRule,omitempty"`
	Attempts    int64      `json:"attempts"`
	RunAfter    *time.Time `json:"runAfter,omitempty"`
	LastError   string     `json:"lastError"`
	Step        *string    `json:"step,omitempty"`
	DependsOnID *int64     `json:"dependsOnId,omitempty"`
	PositionID  *int64     `json:"positionId,omitempty"`
	Cancellable bool       `json:"cancellable"`
	Retryable   bool       `json:"retryable"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type SizeRow struct {
	Key              string    `json:"key"`
	Percent          money.Dec `json:"percent"`
	SuggestedPercent money.Dec `json:"suggestedPercent"`
}

type LastSync struct {
	RunID        string         `json:"runId"`
	Date         string         `json:"date"`
	At           time.Time      `json:"at"`
	CreatedTodos int            `json:"createdTodos"`
	Skipped      []SkippedEntry `json:"skipped"`
	Warnings     []string       `json:"warnings"`
}

type Status struct {
	GeneratedAt       time.Time     `json:"generatedAt"`
	Enabled           bool          `json:"enabled"`
	PauseOpen         bool          `json:"pauseOpen"`
	DailyCheckEnabled bool          `json:"dailyCheckEnabled"`
	TradingEnabled    bool          `json:"tradingEnabled"`
	Capital           Capital       `json:"capital"`
	Exposure          Exposure      `json:"exposure"`
	MarginLevel       MarginStatus  `json:"marginLevel"`
	Sizes             []SizeRow     `json:"sizes"`
	OpenPositions     []PositionRow `json:"openPositions"`
	Todos             []TodoRow     `json:"todos"`
	LastSync          *LastSync     `json:"lastSync,omitempty"`
	SyncMissingToday  bool          `json:"syncMissingToday"`
}

func percentOf(part, whole money.Dec) *money.Dec {
	if !whole.IsPositive() {
		return nil
	}
	p := part.Div(whole).Mul(money.FromInt(100)).Round(2)
	return &p
}

func timePtr(t *dbtype.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.Time
	return &v
}

// Status is the state of the strategy in one structure. It reads the database only (prices and the
// margin level come from the monitor), so it is cheap and never fails on Binance.
func (s *Service) Status(ctx context.Context) (Status, error) {
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return Status{}, err
	}
	now := s.now()
	q := dbq.New(s.DB.Read)
	equity, err := s.equity(ctx, q, cfg)
	if err != nil {
		return Status{}, err
	}
	active, err := q.ListActivePositions(ctx)
	if err != nil {
		return Status{}, err
	}
	closed, err := q.ListClosedPositionTotals(ctx)
	if err != nil {
		return Status{}, err
	}

	st := Status{
		GeneratedAt: now, Enabled: cfg.Enabled, PauseOpen: cfg.PauseOpen, DailyCheckEnabled: cfg.DailyCheckEnabled,
		TradingEnabled: s.Binance.TradingEnabled(), OpenPositions: []PositionRow{}, Todos: []TodoRow{},
	}
	st.Capital = Capital{BudgetUsdt: cfg.BudgetUSDT, RealizedProfitUsdt: equity.Sub(cfg.BudgetUSDT), EquityUsdt: equity, ClosedPositions: len(closed)}
	for _, c := range closed {
		st.Capital.InterestPaidUsdt = st.Capital.InterestPaidUsdt.Add(c.InterestUsdt)
		st.Capital.FeePaidUsdt = st.Capital.FeePaidUsdt.Add(c.FeeUsdt)
	}
	for _, r := range active {
		pos := r.Ema1dPosition
		st.Capital.UnrealizedProfitUsdt = st.Capital.UnrealizedProfitUsdt.Add(pos.UnrealizedProfit)
		st.Capital.FeePaidUsdt = st.Capital.FeePaidUsdt.Add(pos.FeeUsdt)
		value := pos.Amount.Mul(deref(r.CurrentPrice))
		if pos.Side == SideLong {
			st.Exposure.LongUsdt = st.Exposure.LongUsdt.Add(value)
			st.Exposure.LongCount++
		} else {
			st.Exposure.ShortUsdt = st.Exposure.ShortUsdt.Add(value)
			st.Exposure.ShortCount++
		}
		st.OpenPositions = append(st.OpenPositions, s.positionRow(pos, r.PairName, r.CoinName, r.CurrentPrice, r.PriceUpdatedAt, equity, now))
	}
	st.Exposure.LongUsdt, st.Exposure.ShortUsdt = st.Exposure.LongUsdt.Round(2), st.Exposure.ShortUsdt.Round(2)
	st.Exposure.LongPercent = percentOf(st.Exposure.LongUsdt, equity)
	st.Exposure.ShortPercent = percentOf(st.Exposure.ShortUsdt, equity)

	m := s.lastMarginLevel(ctx)
	st.MarginLevel = MarginStatus{Value: m.Value, At: m.At, Warn: cfg.MarginLevelWarn, Alert: cfg.MarginLevelAlert}
	st.Sizes = []SizeRow{
		{"coreLong", cfg.Sizes.CoreLong, SuggestedSizes.CoreLong},
		{"coreShort", cfg.Sizes.CoreShort, SuggestedSizes.CoreShort},
		{"sensitiveLong", cfg.Sizes.SensitiveLong, SuggestedSizes.SensitiveLong},
		{"sensitiveShort", cfg.Sizes.SensitiveShort, SuggestedSizes.SensitiveShort},
	}
	if st.Todos, err = s.Todos(ctx, false); err != nil {
		return Status{}, err
	}
	if st.LastSync, err = s.lastSync(ctx); err != nil {
		return Status{}, err
	}
	st.SyncMissingToday = s.syncMissingToday(ctx)
	return st, nil
}

func (s *Service) positionRow(pos dbq.Ema1dPosition, pairName, coin string, current *money.Dec, priceAt *dbtype.Time, equity money.Dec, now time.Time) PositionRow {
	done := pos.Status == PosClosed
	row := PositionRow{
		ID: pos.ID, Coin: coin, PairName: pairName, Side: pos.Side, Status: pos.Status, SignalDate: pos.SignalDate,
		ExitRule: pos.ExitRule, Amount: pos.Amount, OpenPrice: pos.OpenPrice, RefOpenPrice: pos.RefOpenPrice,
		OpenSlippagePercent: slippagePercent(pos.OpenPrice, pos.RefOpenPrice, pos.Side == SideShort),
		CurrentPrice:        current, PriceUpdatedAt: dbtype.TimePtr(priceAt), NotionalUsdt: pos.NotionalUsdt, SizePercent: pos.SizePercent,
		FeeUsdt: pos.FeeUsdt, InterestUsdt: pos.InterestUsdt, OpenAt: timePtr(pos.OpenAt), CloseAt: timePtr(pos.CloseAt),
		ProfitUsdt: pos.UnrealizedProfit,
	}
	price := deref(current)
	end := now
	if done {
		row.ClosePrice, row.RefClosePrice = pos.ClosePrice, pos.RefClosePrice
		row.CloseSlippagePercent = slippagePercent(pos.ClosePrice, pos.RefClosePrice, pos.Side == SideLong)
		row.ProfitUsdt = pos.RealizedProfit
		price = deref(pos.ClosePrice)
		if pos.CloseAt != nil {
			end = pos.CloseAt.Time
		}
	}
	row.EquityPercent = percentOf(price.Mul(pos.Amount), equity)
	row.ProfitPercent = percentOf(row.ProfitUsdt, cost(pos))
	if pos.OpenAt != nil {
		d := int(math.Floor(end.Sub(pos.OpenAt.Time).Hours() / 24))
		row.DaysHeld = &d
	}
	return row
}

func todoRow(t dbq.Ema1dTodo, coin string) TodoRow {
	return TodoRow{
		ID: t.ID, Coin: coin, Action: t.Action, Side: t.Side, SignalDate: t.SignalDate, Status: t.Status, Source: t.Source,
		RefPrice: t.RefPrice, ExitRule: t.ExitRule, Attempts: t.Attempts, RunAfter: timePtr(t.RunAfter), LastError: t.LastError,
		Step: t.Step, DependsOnID: t.DependsOnID, PositionID: t.PositionID, Cancellable: cancellable(t),
		Retryable: t.Status == TodoFailed || t.Status == TodoSkipped, CreatedAt: t.CreatedAt.Time, UpdatedAt: t.UpdatedAt.Time,
	}
}

// Todos lists the todos in flight plus what failed in the last 7 days or was skipped in the last
// day. With history, every todo of the last 30 days.
func (s *Service) Todos(ctx context.Context, history bool) ([]TodoRow, error) {
	now := s.now()
	rows, err := dbq.New(s.DB.Read).ListTodosWithCoin(ctx, dbq.ListTodosWithCoinParams{
		FailedSince: dbtype.NewTime(now.AddDate(0, 0, -7)), SkippedSince: dbtype.NewTime(now.AddDate(0, 0, -1)),
		All: history, AllSince: dbtype.NewTime(now.AddDate(0, 0, -30)),
	})
	if err != nil {
		return nil, err
	}
	out := make([]TodoRow, len(rows))
	for i, r := range rows {
		out[i] = todoRow(r.Ema1dTodo, r.CoinName)
	}
	return out, nil
}

func (s *Service) lastSync(ctx context.Context) (*LastSync, error) {
	row, err := dbq.New(s.DB.Read).LastRealSync(ctx)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r SyncResult
	_ = json.Unmarshal(row.Result, &r)
	return &LastSync{
		RunID: row.RunID, Date: row.SignalDate, At: row.CreatedAt.Time, CreatedTodos: len(r.CreatedTodos),
		Skipped: nonNil(r.Skipped), Warnings: nonNil(r.Warnings),
	}, nil
}

// ------------------------------------------------------------------------------------ dashboard

type Wanted struct {
	State     string  `json:"state"`
	EntryDate *string `json:"entryDate,omitempty"`
	Rule      *string `json:"rule,omitempty"`
	SyncDate  string  `json:"syncDate"`
}

type PairPosition struct {
	PositionID           int64      `json:"positionId"`
	Side                 string     `json:"side"`
	Status               string     `json:"status"`
	SignalDate           string     `json:"signalDate"`
	UnrealizedProfitUsdt money.Dec  `json:"unrealizedProfitUsdt"`
	UnrealizedPercent    *money.Dec `json:"unrealizedPercent,omitempty"`
}

type PairRow struct {
	ID             int64         `json:"id"`
	PairID         int64         `json:"pairId"`
	Coin           string        `json:"coin"`
	PairName       string        `json:"pairName"`
	PairGroup      string        `json:"pairGroup"`
	Status         string        `json:"status"`
	AllowLong      bool          `json:"allowLong"`
	AllowShort     bool          `json:"allowShort"`
	ShortSupported bool          `json:"shortSupported"`
	LongExitRule   *string       `json:"longExitRule,omitempty"`
	AddedOn        string        `json:"addedOn"`
	RemovedOn      *string       `json:"removedOn,omitempty"`
	SignalFrom     *string       `json:"signalFrom,omitempty"`
	CheckNote      string        `json:"checkNote"`
	CheckedAt      *time.Time    `json:"checkedAt,omitempty"`
	CurrentPrice   *money.Dec    `json:"currentPrice,omitempty"`
	PriceUpdatedAt *time.Time    `json:"priceUpdatedAt,omitempty"`
	Position       *PairPosition `json:"position,omitempty"`
	Wanted         *Wanted       `json:"wanted,omitempty"`
	PendingTodos   []string      `json:"pendingTodos"`
	Mismatch       bool          `json:"mismatch"`
}

type EquityPoint struct {
	Date        string     `json:"date"`
	EquityUsdt  money.Dec  `json:"equityUsdt"`
	MarginLevel *money.Dec `json:"marginLevel,omitempty"`
}

// Pairs is one row per basket pair: what it holds next to what the last sync wanted.
func (s *Service) PairRows(ctx context.Context) ([]PairRow, error) {
	q := dbq.New(s.DB.Read)
	pairs, err := s.listPairs(ctx, q)
	if err != nil {
		return nil, err
	}
	wanted, err := s.lastWanted(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PairRow, 0, len(pairs))
	for _, pv := range pairs {
		row := PairRow{
			ID: pv.ID, PairID: pv.PairID, Coin: pv.CoinName, PairName: pv.PairName, PairGroup: pv.PairGroup, Status: pv.Status,
			AllowLong: pv.AllowLong, AllowShort: pv.AllowShort, ShortSupported: pv.ShortSupported, LongExitRule: pv.LongExitRule,
			AddedOn: pv.AddedOn, RemovedOn: pv.RemovedOn, SignalFrom: pv.SignalFrom, CheckNote: pv.CheckNote,
			CheckedAt: timePtr(pv.CheckedAt), CurrentPrice: pv.CurrentPrice, PriceUpdatedAt: dbtype.TimePtr(pv.PriceUpdatedAt), PendingTodos: []string{},
		}
		pos, err := activePosition(ctx, q, pv.ID)
		if err != nil {
			return nil, err
		}
		if pos != nil {
			row.Position = &PairPosition{
				PositionID: pos.ID, Side: pos.Side, Status: pos.Status, SignalDate: pos.SignalDate,
				UnrealizedProfitUsdt: pos.UnrealizedProfit, UnrealizedPercent: percentOf(pos.UnrealizedProfit, cost(*pos)),
			}
		}
		todos, err := q.InFlightTodosForPair(ctx, pv.ID)
		if err != nil {
			return nil, err
		}
		for _, t := range todos {
			row.PendingTodos = append(row.PendingTodos, t.Action+" "+t.Side)
		}
		if w, ok := wanted[pv.CoinName]; ok {
			w := w
			row.Wanted = &w
		}
		row.Mismatch = mismatch(pv, pos, row.Wanted, len(todos) > 0)
		out = append(out, row)
	}
	return out, nil
}

func (s *Service) lastWanted(ctx context.Context) (map[string]Wanted, error) {
	row, err := dbq.New(s.DB.Read).LastRealSync(ctx)
	if isNoRows(err) {
		return map[string]Wanted{}, nil
	}
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Items []SyncItem `json:"items"`
	}
	_ = json.Unmarshal(row.Parsed, &parsed)
	out := map[string]Wanted{}
	for _, it := range parsed.Items {
		out[it.Coin] = Wanted{State: it.State, EntryDate: it.EntryDate, Rule: it.Rule, SyncDate: row.SignalDate}
	}
	return out, nil
}

// mismatch: the held position differs from the strategy and nothing is queued to fix it. Entries
// the sync deliberately skipped (too old, before added_on…) show too: the screen is where that
// difference should be seen.
func mismatch(pv pairView, pos *dbq.Ema1dPosition, w *Wanted, pending bool) bool {
	if w == nil || pending || pv.Status == PairDisabled {
		return false
	}
	held := ""
	if pos != nil {
		held = pos.Side[:1]
	}
	if w.State == "F" {
		return held != ""
	}
	return held != w.State || pos == nil || pos.SignalDate != deref(w.EntryDate)
}

// EquityCurve returns the daily snapshots of the last days.
func (s *Service) EquityCurve(ctx context.Context, days int) ([]EquityPoint, error) {
	rows, err := dbq.New(s.DB.Read).ListSnapshotsSince(ctx, dateOnly(s.now().AddDate(0, 0, -days)))
	if err != nil {
		return nil, err
	}
	out := make([]EquityPoint, len(rows))
	for i, r := range rows {
		out[i] = EquityPoint{Date: r.Date, EquityUsdt: r.EquityUsdt, MarginLevel: r.MarginLevel}
	}
	return out, nil
}

// ------------------------------------------------------------------------------------ trades

type TradesFilter struct {
	Closed bool
	Coin   string
	Side   string
	Month  string // YYYY-MM: open month for open positions, close month for closed ones
}

type TradesSummary struct {
	Count           int        `json:"count"`
	TotalProfitUsdt money.Dec  `json:"totalProfitUsdt"`
	WinRatePercent  *money.Dec `json:"winRatePercent,omitempty"`
	AvgWinUsdt      *money.Dec `json:"avgWinUsdt,omitempty"`
	AvgLossUsdt     *money.Dec `json:"avgLossUsdt,omitempty"`
	InterestUsdt    money.Dec  `json:"interestUsdt"`
	FeeUsdt         money.Dec  `json:"feeUsdt"`
}

var monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// Trades lists positions for the trade table, filtered, with a summary.
func (s *Service) Trades(ctx context.Context, f TradesFilter) ([]PositionRow, TradesSummary, error) {
	params := dbq.ListPositionsParams{Closed: f.Closed}
	if f.Coin != "" {
		params.Coin = f.Coin
	}
	if f.Side != "" {
		params.Side = f.Side
	}
	if f.Month != "" {
		if !monthRe.MatchString(f.Month) {
			return nil, TradesSummary{}, httpx.Validation("month must be YYYY-MM", nil)
		}
		start, _ := time.Parse("2006-01", f.Month)
		params.From, params.To = dbtype.NewTime(start), dbtype.NewTime(start.AddDate(0, 1, 0))
	}
	cfg, _, err := s.Config(ctx)
	if err != nil {
		return nil, TradesSummary{}, err
	}
	q := dbq.New(s.DB.Read)
	equity, err := s.equity(ctx, q, cfg)
	if err != nil {
		return nil, TradesSummary{}, err
	}
	rows, err := q.ListPositions(ctx, params)
	if err != nil {
		return nil, TradesSummary{}, err
	}
	now := s.now()
	out := make([]PositionRow, len(rows))
	sum := TradesSummary{Count: len(rows)}
	var wins, losses []money.Dec
	for i, r := range rows {
		out[i] = s.positionRow(r.Ema1dPosition, r.PairName, r.CoinName, r.CurrentPrice, r.PriceUpdatedAt, equity, now)
		p := out[i].ProfitUsdt
		sum.TotalProfitUsdt = sum.TotalProfitUsdt.Add(p)
		sum.InterestUsdt = sum.InterestUsdt.Add(r.Ema1dPosition.InterestUsdt)
		sum.FeeUsdt = sum.FeeUsdt.Add(r.Ema1dPosition.FeeUsdt)
		if p.IsPositive() {
			wins = append(wins, p)
		} else {
			losses = append(losses, p)
		}
	}
	if len(rows) > 0 {
		wr := money.FromInt(int64(len(wins) * 100)).Div(money.FromInt(int64(len(rows)))).Round(1)
		sum.WinRatePercent = &wr
	}
	avg := func(xs []money.Dec) *money.Dec {
		if len(xs) == 0 {
			return nil
		}
		a := money.Sum(xs...).Div(money.FromInt(int64(len(xs)))).Round(2)
		return &a
	}
	sum.AvgWinUsdt, sum.AvgLossUsdt = avg(wins), avg(losses)
	return out, sum, nil
}

// DailyCheckRow is one day of the daily check, for the screen.
type DailyCheckRow struct {
	SignalDate string     `json:"signalDate"`
	Status     string     `json:"status"`
	RunID      string     `json:"runId"`
	Attempts   int64      `json:"attempts"`
	Report     string     `json:"report"`
	LastError  string     `json:"lastError"`
	SyncedAt   *time.Time `json:"syncedAt,omitempty"`
	SentAt     *time.Time `json:"sentAt,omitempty"`
}

func (s *Service) DailyChecks(ctx context.Context, limit int64) ([]DailyCheckRow, error) {
	rows, err := dbq.New(s.DB.Read).ListDailyChecks(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]DailyCheckRow, len(rows))
	for i, r := range rows {
		out[i] = DailyCheckRow{
			SignalDate: r.SignalDate, Status: r.Status, RunID: r.RunID, Attempts: r.Attempts, Report: r.Report,
			LastError: r.LastError, SyncedAt: timePtr(r.SyncedAt), SentAt: timePtr(r.SentAt),
		}
	}
	return out, nil
}
