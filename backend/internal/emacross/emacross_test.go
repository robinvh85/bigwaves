package emacross_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/emacross"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/notify"
	"bigwaves/internal/platform/scheduler"
	"bigwaves/internal/testkit"
)

// rising returns n closes going up by 1 from 100, then the given running closes.
func rising(n int, running ...float64) []float64 {
	out := make([]float64, 0, n+len(running))
	for i := range n {
		out = append(out, 100+float64(i))
	}
	return append(out, running...)
}

func TestEvaluate(t *testing.T) {
	t.Run("uptrend holds", func(t *testing.T) {
		r, ok := emacross.Evaluate(rising(60, 160), 0.01)
		require.True(t, ok)
		require.Equal(t, emacross.TrendUp, r.Trend)
		require.Empty(t, r.Cross)
		require.Positive(t, r.GapPercent)
	})
	t.Run("sharp drop crosses down", func(t *testing.T) {
		r, _ := emacross.Evaluate(rising(60, 50), 0.01)
		require.Equal(t, emacross.TrendDown, r.Trend)
		require.Equal(t, emacross.TrendDown, r.Cross)
		require.Negative(t, r.GapPercent)
	})
	t.Run("threshold above the gap", func(t *testing.T) {
		r, _ := emacross.Evaluate(rising(60, 50), 50)
		require.Equal(t, emacross.TrendDown, r.Trend)
		require.Empty(t, r.Cross)
	})
	t.Run("downtrend then jump crosses up", func(t *testing.T) {
		closes := make([]float64, 0, 61)
		for i := range 60 {
			closes = append(closes, 200-float64(i))
		}
		r, _ := emacross.Evaluate(append(closes, 260), 0.01)
		require.Equal(t, emacross.TrendUp, r.Cross)
	})
	t.Run("already crossed on the closed candle", func(t *testing.T) {
		// The cross happened on the previous candle: the running one only continues the trend.
		r, _ := emacross.Evaluate(rising(60, 50, 45), 0.01)
		require.Equal(t, emacross.TrendDown, r.Trend)
		require.Empty(t, r.Cross)
	})
	t.Run("too short", func(t *testing.T) {
		_, ok := emacross.Evaluate([]float64{1}, 0)
		require.False(t, ok)
	})
}

type env struct {
	t   *testing.T
	k   *testkit.Kit
	s   *emacross.Service
	now time.Time
	ctx context.Context
}

func newEnv(t *testing.T) *env {
	k := testkit.New(t)
	e := &env{t: t, k: k, s: k.App.EMACross, now: time.Date(2026, 10, 27, 10, 15, 0, 0, time.UTC), ctx: context.Background()}
	e.s.Now = func() time.Time { return e.now }
	return e
}

func (e *env) pair(symbol string) int64 {
	e.t.Helper()
	e.k.Exchange.AddSymbol(binancefake.Symbol{Symbol: symbol, StepSize: "1", MinNotional: "5", Margin: true}, "100")
	p, err := e.k.App.Pairs.Create(e.ctx, marketdata.CreateInput{Name: symbol})
	require.NoError(e.t, err)
	return p.ID
}

// candles serves closes as hourly klines ending with the running candle of e.now.
func (e *env) candles(symbol string, closes []float64) {
	running := e.now.Truncate(time.Hour).Unix()
	rows := make([]binancefake.Kline, len(closes))
	for i, c := range closes {
		v := fmt.Sprint(c)
		rows[i] = binancefake.Kline{Time: running - int64(len(closes)-1-i)*3600, Open: v, High: v, Low: v, Close: v}
	}
	e.k.Exchange.SetKlines(symbol, "1h", rows)
}

func (e *env) crossed() []notify.Sent {
	var out []notify.Sent
	for _, m := range e.k.Recorder.Messages {
		if m.Channel == notify.Crossed {
			out = append(out, m)
		}
	}
	return out
}

func TestCheckSendsEachCrossOnce(t *testing.T) {
	e := newEnv(t)
	sun := e.pair("SUNUSDT")
	e.pair("ADAUSDT") // not watched
	_, err := e.s.Watch(e.ctx, sun, true, false, false)
	require.NoError(t, err)

	e.candles("SUNUSDT", rising(60, 160))
	msg, err := e.s.Check(e.ctx, emacross.H1)
	require.NoError(t, err)
	require.Contains(t, msg, "1 pairs checked, 0 crossed")
	require.Empty(t, e.crossed())

	e.candles("SUNUSDT", rising(60, 50))
	_, err = e.s.Check(e.ctx, emacross.H1)
	require.NoError(t, err)
	require.Len(t, e.crossed(), 1)
	require.Contains(t, e.crossed()[0].Msg, "EMA H1: SUNUSDT is DOWN CROSSED")

	// Same candle, still crossed: no second message.
	e.now = e.now.Add(15 * time.Minute)
	_, err = e.s.Check(e.ctx, emacross.H1)
	require.NoError(t, err)
	require.Len(t, e.crossed(), 1)

	alerts, err := e.s.Alerts(e.ctx, emacross.AlertFilter{})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, "DOWN", alerts[0].Direction)
	require.NotNil(t, alerts[0].SentAt)
	require.Equal(t, "50", alerts[0].Price.String())

	list, err := e.s.List(e.ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Len(t, list[0].States, 1)
	require.Equal(t, "DOWN", list[0].States[0].Trend)
	require.Len(t, list[0].LastAlerts, 1)

	// The next candle crosses again (back up): a new alert.
	e.now = e.now.Add(time.Hour)
	e.candles("SUNUSDT", append(rising(60, 50), 300))
	_, err = e.s.Check(e.ctx, emacross.H1)
	require.NoError(t, err)
	require.Len(t, e.crossed(), 2)
	require.Contains(t, e.crossed()[1].Msg, "is UP CROSSED")
}

func TestCheckSkips(t *testing.T) {
	e := newEnv(t)
	_, err := e.s.Check(e.ctx, emacross.H4)
	require.ErrorIs(t, err, scheduler.ErrSkip)

	sun := e.pair("SUNUSDT")
	_, err = e.s.Watch(e.ctx, sun, true, false, false)
	require.NoError(t, err)
	_, err = e.s.Check(e.ctx, emacross.H4) // H1 only
	require.ErrorIs(t, err, scheduler.ErrSkip)

	c, v, err := e.s.Config(e.ctx)
	require.NoError(t, err)
	c.Enabled = false
	_, err = e.s.SaveConfig(e.ctx, v, c, "test")
	require.NoError(t, err)
	_, err = e.s.Check(e.ctx, emacross.H1)
	require.ErrorIs(t, err, scheduler.ErrSkip)
}

func TestCheckReportsPairErrors(t *testing.T) {
	e := newEnv(t)
	sun, ada := e.pair("SUNUSDT"), e.pair("ADAUSDT")
	for _, id := range []int64{sun, ada} {
		_, err := e.s.Watch(e.ctx, id, true, false, false)
		require.NoError(t, err)
	}
	e.candles("SUNUSDT", rising(60, 50))
	e.candles("ADAUSDT", rising(10)) // newly listed: too few candles

	_, err := e.s.Check(e.ctx, emacross.H1)
	require.ErrorContains(t, err, "ADAUSDT: only")
	require.Len(t, e.crossed(), 1, "the other pair is still checked")
}

func TestWatchAllOffRemovesThePair(t *testing.T) {
	e := newEnv(t)
	sun := e.pair("SUNUSDT")
	p, err := e.s.Watch(e.ctx, sun, true, true, false)
	require.NoError(t, err)
	require.True(t, p.H1)
	require.True(t, p.H4)

	_, err = e.s.Watch(e.ctx, sun, false, false, false)
	require.NoError(t, err)
	list, err := e.s.List(e.ctx)
	require.NoError(t, err)
	require.Empty(t, list)

	_, err = e.s.Watch(e.ctx, 999, true, false, false)
	require.ErrorContains(t, err, "pair not found")
}

func TestConfigValidation(t *testing.T) {
	c := emacross.DefaultConfig()
	require.NoError(t, c.Validate())
	c.ThresholdPercent.H4 = c.ThresholdPercent.H4.Neg()
	require.ErrorContains(t, c.Validate(), "thresholdPercent.h4")
}
