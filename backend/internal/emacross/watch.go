package emacross

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
)

// State is the last reading of one pair and timeframe.
type State struct {
	Timeframe  string    `json:"timeframe"`
	Trend      string    `json:"trend"`
	GapPercent float64   `json:"gapPercent"`
	Price      money.Dec `json:"price"`
	CandleTime time.Time `json:"candleTime"`
	CheckedAt  time.Time `json:"checkedAt"`
}

// Alert is one cross sent (or not, see Error) to Discord.
type Alert struct {
	ID         int64      `json:"id"`
	PairID     int64      `json:"pairId"`
	PairName   string     `json:"pairName"`
	Timeframe  string     `json:"timeframe"`
	Direction  string     `json:"direction"`
	Price      money.Dec  `json:"price"`
	GapPercent float64    `json:"gapPercent"`
	CandleTime time.Time  `json:"candleTime"`
	SentAt     *time.Time `json:"sentAt,omitempty"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Pair is a watched pair with its switches, its last readings and its last alerts.
type Pair struct {
	PairID         int64      `json:"pairId"`
	PairName       string     `json:"pairName"`
	PairStatus     string     `json:"pairStatus"`
	CurrentPrice   *money.Dec `json:"currentPrice,omitempty"`
	PriceUpdatedAt *time.Time `json:"priceUpdatedAt,omitempty"`
	H1             bool       `json:"h1"`
	H4             bool       `json:"h4"`
	D1             bool       `json:"d1"`
	// States and LastAlerts only hold the switched-on timeframes, in H1, H4, D1 order.
	States     []State `json:"states"`
	LastAlerts []Alert `json:"lastAlerts"`
}

func (p Pair) watches(tf string) bool {
	return (tf == H1.Name && p.H1) || (tf == H4.Name && p.H4) || (tf == D1.Name && p.D1)
}

// List returns the watched pairs by name.
func (s *Service) List(ctx context.Context) ([]Pair, error) {
	q := dbq.New(s.DB.Read)
	rows, err := q.ListEmacrossPairs(ctx)
	if err != nil {
		return nil, err
	}
	states, err := q.ListEmacrossStates(ctx)
	if err != nil {
		return nil, err
	}
	last, err := q.LastEmacrossAlerts(ctx)
	if err != nil {
		return nil, err
	}
	byPair := map[int64]*Pair{}
	out := make([]Pair, len(rows))
	for i, r := range rows {
		out[i] = pairView(r.EmacrossPair, r.PairName, r.PairStatus, r.CurrentPrice, r.PriceUpdatedAt)
		byPair[r.EmacrossPair.PairID] = &out[i]
	}
	for _, st := range states {
		if p := byPair[st.PairID]; p != nil && p.watches(st.Timeframe) {
			p.States = append(p.States, stateView(st))
		}
	}
	for _, a := range last {
		if p := byPair[a.EmacrossAlert.PairID]; p != nil && p.watches(a.EmacrossAlert.Timeframe) {
			p.LastAlerts = append(p.LastAlerts, alertView(a.EmacrossAlert, a.PairName))
		}
	}
	for i := range out {
		sortByTimeframe(out[i].States, func(s State) string { return s.Timeframe })
		sortByTimeframe(out[i].LastAlerts, func(a Alert) string { return a.Timeframe })
	}
	return out, nil
}

// Watch sets the timeframes watched on a pair. All off removes the pair from the list.
func (s *Service) Watch(ctx context.Context, pairID int64, h1, h4, d1 bool) (Pair, error) {
	q := dbq.New(s.DB.Write)
	row, err := q.GetPair(ctx, pairID)
	p := row.Pair
	if errors.Is(err, sql.ErrNoRows) || (err == nil && p.DeletedAt != nil) {
		return Pair{}, httpx.NewError(404, "pair_not_found", "pair not found")
	}
	if err != nil {
		return Pair{}, err
	}
	if !h1 && !h4 && !d1 {
		if err := q.DeleteEmacrossPair(ctx, pairID); err != nil {
			return Pair{}, err
		}
		if err := q.DeleteEmacrossStates(ctx, pairID); err != nil {
			return Pair{}, err
		}
		s.changed()
		return Pair{PairID: p.ID, PairName: p.Name, PairStatus: p.Status, CurrentPrice: p.CurrentPrice,
			PriceUpdatedAt: dbtype.TimePtr(p.PriceUpdatedAt), States: []State{}, LastAlerts: []Alert{}}, nil
	}
	if err := q.UpsertEmacrossPair(ctx, dbq.UpsertEmacrossPairParams{PairID: pairID, H1: h1, H4: h4, D1: d1}); err != nil {
		return Pair{}, err
	}
	s.changed()
	all, err := s.List(ctx)
	if err != nil {
		return Pair{}, err
	}
	for _, w := range all {
		if w.PairID == pairID {
			return w, nil
		}
	}
	return Pair{}, httpx.ErrNotFound
}

// AlertFilter narrows Alerts; zero values mean all.
type AlertFilter struct {
	PairID    int64
	Timeframe string
	Limit     int64
}

// Alerts returns the latest alerts, newest first.
func (s *Service) Alerts(ctx context.Context, f AlertFilter) ([]Alert, error) {
	params := dbq.ListEmacrossAlertsParams{Limit: f.Limit}
	if params.Limit <= 0 || params.Limit > 500 {
		params.Limit = 100
	}
	if f.PairID > 0 {
		params.PairID = f.PairID
	}
	if f.Timeframe != "" {
		params.Timeframe = f.Timeframe
	}
	rows, err := dbq.New(s.DB.Read).ListEmacrossAlerts(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]Alert, len(rows))
	for i, r := range rows {
		out[i] = alertView(r.EmacrossAlert, r.PairName)
	}
	return out, nil
}

func pairView(w dbq.EmacrossPair, name, status string, price *money.Dec, priceAt *dbtype.Time) Pair {
	return Pair{
		PairID: w.PairID, PairName: name, PairStatus: status, CurrentPrice: price, PriceUpdatedAt: dbtype.TimePtr(priceAt),
		H1: w.H1, H4: w.H4, D1: w.D1, States: []State{}, LastAlerts: []Alert{},
	}
}

func stateView(st dbq.EmacrossState) State {
	return State{
		Timeframe: st.Timeframe, Trend: st.Trend, GapPercent: st.GapPercent, Price: st.Price,
		CandleTime: time.Unix(st.CandleTime, 0).UTC(), CheckedAt: st.CheckedAt.Time,
	}
}

func alertView(a dbq.EmacrossAlert, pairName string) Alert {
	v := Alert{
		ID: a.ID, PairID: a.PairID, PairName: pairName, Timeframe: a.Timeframe, Direction: a.Direction,
		Price: a.Price, GapPercent: a.GapPercent, CandleTime: time.Unix(a.CandleTime, 0).UTC(),
		Error: a.Error, CreatedAt: a.CreatedAt.Time,
	}
	if a.SentAt != nil {
		t := a.SentAt.Time
		v.SentAt = &t
	}
	return v
}

var timeframeRank = map[string]int{H1.Name: 0, H4.Name: 1, D1.Name: 2}

func sortByTimeframe[T any](xs []T, tf func(T) string) {
	slices.SortFunc(xs, func(a, b T) int { return timeframeRank[tf(a)] - timeframeRank[tf(b)] })
}
