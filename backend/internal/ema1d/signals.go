package ema1d

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"bigwaves/internal/ema1d/engine"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/money"
)

// Candle intervals stored for the strategy. TradingView, where the strategy was researched, only
// had 4H candles from 01/2024.
var intervals = []struct {
	Name    string
	API     string
	Seconds int64
	First   time.Time
}{
	{"1D", "1d", engine.Day, time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)},
	{"4H", "4h", engine.H4, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
}

const (
	klineLimit    = 1000
	candleOverlap = 3
)

// CandleReport tells what UpdateCandles did.
type CandleReport struct {
	Fetched []string `json:"fetched"`
	Errors  []string `json:"errors"`
	Gaps    []string `json:"gaps"`
}

// UpdateCandles brings the stored 1D/4H candles of the tracked pairs (ACTIVE + EXIT_ONLY) up to
// now: from 3 candles before the last stored one (the running candle is stored too and rewritten
// next time), or the whole history for a pair without candles yet. Errors of one pair are
// reported, never raised.
func (s *Service) UpdateCandles(ctx context.Context) (CandleReport, error) {
	rep := CandleReport{Fetched: []string{}, Errors: []string{}, Gaps: []string{}}
	pairs, err := s.trackedPairs(ctx)
	if err != nil {
		return rep, err
	}
	for _, pv := range pairs {
		for _, iv := range intervals {
			if err := s.updateCandles(ctx, pv, iv.Name, iv.API, iv.Seconds, iv.First, &rep); err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("%s %s: %s", pv.PairName, iv.Name, truncate(err.Error(), 200)))
			}
		}
	}
	return rep, nil
}

func (s *Service) trackedPairs(ctx context.Context) ([]pairView, error) {
	all, err := s.listPairs(ctx, dbq.New(s.DB.Read))
	if err != nil {
		return nil, err
	}
	var out []pairView
	for _, pv := range all {
		if pv.Status == PairActive || pv.Status == PairExitOnly {
			out = append(out, pv)
		}
	}
	slices.SortFunc(out, func(a, b pairView) int { return int(a.ID - b.ID) })
	return out, nil
}

func (s *Service) updateCandles(ctx context.Context, pv pairView, interval, api string, step int64, first time.Time, rep *CandleReport) error {
	q := dbq.New(s.DB.Write)
	last, err := q.MaxCandleTime(ctx, dbq.MaxCandleTimeParams{PairID: pv.PairID, Interval: interval})
	if err != nil {
		return err
	}
	start := first
	if last > 0 {
		start = time.Unix(last-candleOverlap*step, 0).UTC()
	}
	rows, err := s.fetchCandles(ctx, pv.PairName, api, start)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("no candles returned")
	}
	prev, err := q.MaxCandleTimeBefore(ctx, dbq.MaxCandleTimeBeforeParams{PairID: pv.PairID, Interval: interval, OpenTime: rows[0].Time})
	if err != nil {
		return err
	}
	times := make([]int64, 0, len(rows)+1)
	if prev > 0 {
		times = append(times, prev)
	}
	for _, r := range rows {
		times = append(times, r.Time)
	}
	for i := 1; i < len(times); i++ {
		if times[i]-times[i-1] != step {
			rep.Gaps = append(rep.Gaps, fmt.Sprintf("%s %s %s → %s", pv.CoinName, interval, fmtTS(times[i-1]), fmtTS(times[i])))
		}
	}
	for _, r := range rows {
		if err := q.UpsertCandle(ctx, dbq.UpsertCandleParams{
			PairID: pv.PairID, Interval: interval, OpenTime: r.Time, Open: r.Open, High: r.High, Low: r.Low, Close: r.Close,
		}); err != nil {
			return err
		}
	}
	rep.Fetched = append(rep.Fetched, fmt.Sprintf("%s %s: %d, last %s UTC", pv.PairName, interval, len(rows), fmtTS(rows[len(rows)-1].Time)))
	return nil
}

func fmtTS(sec int64) string { return time.Unix(sec, 0).UTC().Format("2006-01-02 15:04") }

func (s *Service) fetchCandles(ctx context.Context, symbol, api string, start time.Time) ([]engine.Candle, error) {
	var out []engine.Candle
	now := s.now().Unix()
	for {
		ks, err := s.Binance.Klines(ctx, symbol, api, start, klineLimit)
		if err != nil {
			return nil, err
		}
		for _, k := range ks {
			c := engine.Candle{Time: k.OpenTime.Unix(), Open: f64(k.Open), High: f64(k.High), Low: f64(k.Low), Close: f64(k.Close)}
			if c.Time <= now {
				out = append(out, c)
			}
		}
		if len(ks) < klineLimit {
			break
		}
		start = ks[len(ks)-1].OpenTime.Add(time.Millisecond)
		s.sleep(100 * time.Millisecond)
	}
	return out, nil
}

// f64 parses the decimal string like Ruby Float(): correctly rounded, the same as the research.
func f64(d money.Dec) float64 {
	v, _ := strconv.ParseFloat(d.String(), 64)
	return v
}

func (s *Service) candles(ctx context.Context, pairID int64, interval string, before int64) ([]engine.Candle, error) {
	rows, err := dbq.New(s.DB.Read).ListCandlesBefore(ctx, dbq.ListCandlesBeforeParams{PairID: pairID, Interval: interval, OpenTime: before})
	if err != nil {
		return nil, err
	}
	out := make([]engine.Candle, len(rows))
	for i, r := range rows {
		out[i] = engine.Candle{Time: r.OpenTime, Open: r.Open, High: r.High, Low: r.Low, Close: r.Close}
	}
	return out, nil
}

// SignalResult is where the strategy stands for the tracked pairs, from the stored candles.
type SignalResult struct {
	Date     string       `json:"date"`
	Rows     []engine.Row `json:"rows"`
	S        string       `json:"s"`
	Warnings []string     `json:"warnings"`
}

// Signals computes the status rows (status.py) and the sync string (build_sync.py). The running
// candle of today is included.
func (s *Service) Signals(ctx context.Context) (SignalResult, error) {
	now := s.now()
	today := dateOnly(now)
	t, _ := parseDate(today)
	before := t.AddDate(0, 0, 1).Unix()

	pairs, err := s.trackedPairs(ctx)
	if err != nil {
		return SignalResult{}, err
	}
	opens := map[string]map[string]float64{}
	var rows []engine.Row
	var errs []string
	for _, pv := range pairs {
		row, err := s.signalRow(ctx, pv, before, opens)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", pv.CoinName, err))
			continue
		}
		rows = append(rows, row)
	}
	if len(errs) > 0 {
		return SignalResult{}, fmt.Errorf("signals: %s", strings.Join(errs, "; "))
	}
	// Basket first, then the old basket (status.py order).
	slices.SortStableFunc(rows, func(a, b engine.Row) int {
		return boolInt(a.Nhom == engine.NhomOldBask) - boolInt(b.Nhom == engine.NhomOldBask)
	})
	params := engine.BuildSyncParams(rows, today, func(coin, day string) (float64, bool) {
		v, ok := opens[coin][day]
		return v, ok
	})
	return SignalResult{Date: params.Date, Rows: rows, S: params.S, Warnings: nonNil(params.Warnings)}, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Service) signalRow(ctx context.Context, pv pairView, before int64, opens map[string]map[string]float64) (engine.Row, error) {
	d1, err := s.candles(ctx, pv.PairID, "1D", before)
	if err != nil {
		return engine.Row{}, err
	}
	h4, err := s.candles(ctx, pv.PairID, "4H", before)
	if err != nil {
		return engine.Row{}, err
	}
	opens[pv.CoinName] = map[string]float64{}
	for _, c := range d1 {
		opens[pv.CoinName][engine.ISODate(c.Time)] = c.Open
	}
	if pv.LongExitRule == nil {
		return engine.Row{}, errors.New("long exit rule is not set")
	}
	in := engine.PairInput{Coin: pv.CoinName, LongExit: *pv.LongExitRule, LongOnly: !pv.AllowShort, ExitOnly: pv.Status == PairExitOnly}
	if pv.SignalFrom != nil {
		in.SignalFrom = dateUnix(*pv.SignalFrom)
	}
	if pv.RemovedOn != nil {
		in.RemovedOn = dateUnix(*pv.RemovedOn)
	}
	return engine.BuildRow(in, d1, h4)
}

func dateUnix(d string) *int64 {
	t, err := parseDate(d)
	if err != nil {
		return nil
	}
	u := t.Unix()
	return &u
}
