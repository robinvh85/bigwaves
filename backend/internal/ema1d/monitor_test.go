package ema1d_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/httpx"
)

// Ported from monitor_service_test.rb and status_and_manual_todo_test.rb.

type monEnv struct {
	*env
	sun, eth    dbq.Ema1dPair
	short, long dbq.Ema1dPosition
}

func newMonEnv(t *testing.T, now time.Time) *monEnv {
	e := newEnv(t, now)
	m := &monEnv{env: e}
	e.config(func(c *ema1d.Config) { c.Enabled = true })
	m.sun = e.pair("SUN", "0.02", "1")
	m.eth = e.pair("ETH", "2000", "0.0001")
	m.short = e.position(m.sun, ema1d.SideShort, "2026-10-05", "10500", "0.02")
	m.long = e.position(m.eth, ema1d.SideLong, "2026-10-05", "0.2", "2000")
	e.exec("UPDATE ema1d_positions SET fee_usdt = '0.21' WHERE id = ?", m.short.ID)
	e.exec("UPDATE ema1d_positions SET fee_usdt = '0.4' WHERE id = ?", m.long.ID)
	return m
}

func (m *monEnv) monitor() {
	m.t.Helper()
	_, err := m.s.Monitor(m.ctx)
	require.NoError(m.t, err)
}

func TestMonitorMarksPositionsToMarketWithShortInterest(t *testing.T) {
	m := newMonEnv(t, time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC)) // 09:00 VN
	m.ex.SetPrice("SUNUSDT", "0.018")
	m.ex.SetPrice("ETHUSDT", "2100")
	m.ex.MarginLevel = dec("3.2")
	m.ex.AccrueInterest("SUN", "5")

	m.monitor()

	// (0.02 - 0.018) * 10500 - 0.21 fee - 5 SUN * 0.018
	require.Equal(t, "20.7", m.pos(m.short.ID).UnrealizedProfit.String())
	// (2100 - 2000) * 0.2 - 0.4 fee
	require.Equal(t, "19.6", m.pos(m.long.ID).UnrealizedProfit.String())
	st, err := m.s.Status(m.ctx)
	require.NoError(t, err)
	require.Equal(t, "3.2", st.MarginLevel.Value.String())
	require.Empty(t, alerts(m.env))
}

func TestMonitorWarnsOnLowMarginLevelOnceAnHour(t *testing.T) {
	m := newMonEnv(t, time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC))
	m.ex.MarginLevel = dec("1.4")
	m.monitor()
	m.monitor()
	require.Equal(t, []string{"EMA1D warning: cross margin level 1.4 < 1.5"}, alerts(m.env))

	m.ex.MarginLevel = dec("1.2")
	m.monitor()
	require.Contains(t, alerts(m.env)[1], "EMA1D ALERT: cross margin level 1.2 < 1.3")

	m.ex.MarginLevel = dec("1.4")
	m.travel(61 * time.Minute)
	m.monitor()
	require.Len(t, alerts(m.env), 3, "the warning comes back after an hour")
}

func TestMonitorRemindsOnceWhenNoSyncByNoon(t *testing.T) {
	m := newMonEnv(t, time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC))
	m.monitor()
	require.Empty(t, alerts(m.env), "09:00 VN is not late yet")

	m.now = time.Date(2026, 10, 10, 5, 30, 0, 0, time.UTC) // 12:30 VN
	m.monitor()
	m.monitor()
	require.Len(t, alerts(m.env), 1)
	require.Contains(t, alerts(m.env)[0], "chưa có sync cho 2026-10-10")
}

func TestMonitorSkipsWhenDisabledWithNoPosition(t *testing.T) {
	e := newEnv(t, time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC))
	_, err := e.s.Monitor(e.ctx)
	require.ErrorContains(t, err, "skipped")
	require.Zero(t, e.ex.CallCount("GET /sapi/v1/margin/account"))
}

func TestSnapshotRecordsEquityAndExposure(t *testing.T) {
	m := newMonEnv(t, time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC))
	m.ex.SetPrice("SUNUSDT", "0.018")
	m.ex.SetPrice("ETHUSDT", "2100")
	m.ex.MarginLevel = dec("3.2")
	m.monitor()

	_, err := m.s.Snapshot(m.ctx)
	require.NoError(t, err)
	curve, err := m.s.EquityCurve(m.ctx, 30)
	require.NoError(t, err)
	require.Len(t, curve, 1)
	require.Equal(t, "2026-10-10", curve[0].Date)
	require.Equal(t, dec("6000").Add(dec("20.79")).Add(dec("19.6")).String(), curve[0].EquityUsdt.String())
	require.Equal(t, "3.2", curve[0].MarginLevel.String())
}

func TestStatusSumsCapitalExposurePositionsAndTodos(t *testing.T) {
	e := newEnv(t, time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC))
	sun := e.pair("SUN", "0.018", "1")
	short := e.position(sun, ema1d.SideShort, "2026-10-05", "10500", "0.02")
	e.exec(`UPDATE ema1d_positions SET ref_open_price = '0.0213', unrealized_profit = '20', size_percent = '3.5',
	        notional_usdt = '210', open_at = '2026-10-05T02:00:00.000Z' WHERE id = ?`, short.ID)
	closed := e.position(sun, ema1d.SideLong, "2026-09-01", "1", "0.01")
	e.exec(`UPDATE ema1d_positions SET status = 'CLOSED', realized_profit = '150', fee_usdt = '1' WHERE id = ?`, closed.ID)
	failed, err := e.q().CreateTodo(e.ctx, dbq.CreateTodoParams{Ema1dPairID: sun.ID, Action: "CLOSE", Side: "SHORT", SignalDate: "2026-10-05", Source: "SYNC", PositionID: &short.ID})
	require.NoError(t, err)
	e.exec("UPDATE ema1d_todos SET status = 'FAILED', last_error = 'timeout' WHERE id = ?", failed.ID)
	_, err = e.q().CreateSync(e.ctx, dbq.CreateSyncParams{RunID: "20261010-0911", SignalDate: "2026-10-10", Parsed: dbtype.JSON("{}"),
		Result: dbtype.JSON(`{"created_todos":[{}],"skipped":[],"warnings":["missing: ETH"]}`)})
	require.NoError(t, err)

	st, err := e.s.Status(e.ctx)
	require.NoError(t, err)
	require.Equal(t, "6150", st.Capital.EquityUsdt.String())
	require.Equal(t, "150", st.Capital.RealizedProfitUsdt.String())
	require.Equal(t, "20", st.Capital.UnrealizedProfitUsdt.String())
	require.Equal(t, "189", st.Exposure.ShortUsdt.String())
	require.Equal(t, "3.07", st.Exposure.ShortPercent.String())
	require.Len(t, st.OpenPositions, 1)
	p := st.OpenPositions[0]
	require.Equal(t, "SUN", p.Coin)
	require.Equal(t, "6.103", p.OpenSlippagePercent.String(), "sold 6.1% below the strategy price: worse")
	require.Equal(t, 5, *p.DaysHeld)
	require.Equal(t, "9.52", p.ProfitPercent.String())
	require.Len(t, st.Todos, 1)
	require.Equal(t, "timeout", st.Todos[0].LastError)
	require.False(t, st.SyncMissingToday)
	require.Equal(t, []string{"missing: ETH"}, st.LastSync.Warnings)
	require.Equal(t, 1, st.LastSync.CreatedTodos)
	require.Equal(t, "coreLong", st.Sizes[0].Key)
}

func TestManualCloseTargetsTheOpenPositionAndIsIdempotent(t *testing.T) {
	e := newEnv(t, time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC))
	sun := e.pair("SUN", "0.018", "1")
	short := e.position(sun, ema1d.SideShort, "2026-10-05", "10500", "0.02")

	first, err := e.s.CreateManualTodo(e.ctx, ema1d.ManualTodoRequest{Action: "close", Coin: "sun", RefPrice: "0.018"})
	require.NoError(t, err)
	second, err := e.s.CreateManualTodo(e.ctx, ema1d.ManualTodoRequest{Action: "CLOSE", Coin: "SUN"})
	require.NoError(t, err)
	require.True(t, first.Created)
	require.False(t, second.Created)
	ts := e.todos()
	require.Len(t, ts, 1)
	require.Equal(t, ema1d.SourceManual, ts[0].Source)
	require.Equal(t, short.ID, *ts[0].PositionID)
	require.Equal(t, "SHORT", ts[0].Side)
	require.Equal(t, "2026-10-05", ts[0].SignalDate)
}

func TestManualOpenRunsTheSyncChecks(t *testing.T) {
	e := newEnv(t, time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC))
	e.pair("SUN", "0.018", "1")
	e.pair("BTC", "100000", "0.00001", sensitive, longOnly)
	open := func(req ema1d.ManualTodoRequest) (ema1d.ManualTodoResult, error) {
		req.Action = "OPEN"
		if req.Coin == "" {
			req.Coin = "SUN"
		}
		if req.Side == "" {
			req.Side = "SHORT"
		}
		return e.s.CreateManualTodo(e.ctx, req)
	}
	_, err := open(ema1d.ManualTodoRequest{SignalDate: "2026-10-07"})
	require.ErrorContains(t, err, "older than 1 day(s), add force")
	var he *httpx.Error
	require.ErrorAs(t, err, &he)
	require.Equal(t, 400, he.Status)

	res, err := open(ema1d.ManualTodoRequest{SignalDate: "2026-10-07", Force: true, Rule: "XO"})
	require.NoError(t, err)
	require.True(t, res.Created)

	_, err = open(ema1d.ManualTodoRequest{Coin: "BTC", SignalDate: "2026-10-10"})
	require.ErrorContains(t, err, "SHORT not allowed")
	_, err = e.s.CreateManualTodo(e.ctx, ema1d.ManualTodoRequest{Action: "OPEN", Coin: "DOGE"})
	require.ErrorContains(t, err, "DOGE is not an EMA1D pair")
}

func TestManualOpenRefusedWhileHolding(t *testing.T) {
	e := newEnv(t, time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC))
	sun := e.pair("SUN", "0.018", "1")
	pos := e.position(sun, ema1d.SideShort, "2026-10-05", "10500", "0.02")
	_, err := e.s.CreateManualTodo(e.ctx, ema1d.ManualTodoRequest{Action: "OPEN", Coin: "SUN", Side: "LONG", SignalDate: "2026-10-10"})
	require.ErrorContains(t, err, "already holding position #")
	_ = pos
}

func TestRetryAndCancel(t *testing.T) {
	e := newEnv(t, time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC))
	sun := e.pair("SUN", "0.018", "1")
	t1, err := e.q().CreateTodo(e.ctx, dbq.CreateTodoParams{Ema1dPairID: sun.ID, Action: "OPEN", Side: "LONG", SignalDate: "2026-10-10", Source: "SYNC"})
	require.NoError(t, err)

	_, err = e.s.RetryTodo(e.ctx, t1.ID)
	require.ErrorContains(t, err, "FAILED hoặc SKIPPED")

	row, err := e.s.CancelTodo(e.ctx, t1.ID)
	require.NoError(t, err)
	require.Equal(t, ema1d.TodoCancelled, row.Status)

	t2, err := e.q().CreateTodo(e.ctx, dbq.CreateTodoParams{Ema1dPairID: sun.ID, Action: "OPEN", Side: "SHORT", SignalDate: "2026-10-10", Source: "SYNC"})
	require.NoError(t, err)
	e.exec("UPDATE ema1d_todos SET status = 'FAILED', attempts = 3, step = 'BORROWING', first_attempted_at = '2026-10-10T01:00:00.000Z' WHERE id = ?", t2.ID)
	_, err = e.s.CancelTodo(e.ctx, t2.ID)
	require.ErrorContains(t, err, "không huỷ được")
	row, err = e.s.RetryTodo(e.ctx, t2.ID)
	require.NoError(t, err)
	require.Equal(t, ema1d.TodoPending, row.Status)
	require.NotNil(t, e.todo(t2.ID).FirstAttemptedAt, "kept while a borrow may be in flight")
}

func TestUpdatePairExitOnlyDatesTheRemoval(t *testing.T) {
	e := newEnv(t, time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC))
	sun := e.pair("SUN", "0.018", "1")
	st := ema1d.PairExitOnly
	row, err := e.s.UpdatePair(e.ctx, sun.ID, ema1d.PairUpdate{Status: &st})
	require.NoError(t, err)
	require.Equal(t, ema1d.PairExitOnly, row.Status)
	require.Equal(t, "2026-10-10", *row.RemovedOn)

	active := ema1d.PairActive
	row, err = e.s.UpdatePair(e.ctx, sun.ID, ema1d.PairUpdate{Status: &active})
	require.NoError(t, err)
	require.Nil(t, row.RemovedOn)
}
