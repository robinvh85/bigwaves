package ema1d_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/httpx"
)

// Ported from test/services/ema1d/sync_service_test.rb.

const syncDate = "2026-10-10"

type syncEnv struct {
	*env
	sun, eth, btc dbq.Ema1dPair
	run           int
}

func newSyncEnv(t *testing.T) *syncEnv {
	e := newEnv(t, time.Date(2026, 10, 10, 2, 5, 0, 0, time.UTC))
	s := &syncEnv{env: e}
	s.sun = e.pair("SUN", "0.02", "1")
	s.eth = e.pair("ETH", "2400", "0.0001")
	s.btc = e.pair("BTC", "120000", "0.00001", sensitive, longOnly)
	return s
}

func (s *syncEnv) sync(str string, opts ...string) ema1d.SyncResult {
	s.t.Helper()
	s.run++
	runID, dry := fmt.Sprintf("run-%d", s.run), ""
	if len(opts) > 0 {
		runID = opts[0]
	}
	if len(opts) > 1 {
		dry = opts[1]
	}
	res, err := s.s.Sync(s.ctx, ema1d.SyncRequest{Date: syncDate, RunID: runID, S: str, DryRun: dry}, "")
	require.NoError(s.t, err)
	return res
}

type tk struct{ coin, action, side string }

func (s *syncEnv) todoKeys() []tk {
	coins := map[int64]string{s.sun.ID: "SUN", s.eth.ID: "ETH", s.btc.ID: "BTC"}
	var out []tk
	for _, t := range s.todos() {
		out = append(out, tk{coins[t.Ema1dPairID], t.Action, t.Side})
	}
	return out
}

func reasons(res ema1d.SyncResult) map[string]string {
	out := map[string]string{}
	for _, sk := range res.Skipped {
		out[sk.Coin] = sk.Reason
	}
	return out
}

func TestSyncOpensFreshEntries(t *testing.T) {
	s := newSyncEnv(t)
	res := s.sync("SUN:S:2026-10-10:0.0213:XO,ETH:L:2026-10-09:2410.5:DIV4H,BTC:F:-:120500:-")

	require.Equal(t, []tk{{"SUN", "OPEN", "SHORT"}, {"ETH", "OPEN", "LONG"}}, s.todoKeys())
	open := s.todos()[0]
	require.Equal(t, "2026-10-10", open.SignalDate)
	require.Equal(t, "0.0213", open.RefPrice.String())
	require.Equal(t, "XO", *open.ExitRule)
	require.Equal(t, ema1d.TodoPending, open.Status)
	require.Nil(t, open.DependsOnID)
	require.Len(t, res.CreatedTodos, 2)
	require.Empty(t, res.Skipped)
}

func TestSyncReversalClosesFirstThenOpens(t *testing.T) {
	s := newSyncEnv(t)
	pos := s.position(s.sun, ema1d.SideLong, "2026-10-03", "1000", "0.02")

	s.sync("SUN:S:2026-10-10:0.0213:XO,ETH:F:-:2400:-,BTC:F:-:1:-")

	ts := s.todos()
	require.Len(t, ts, 2)
	cl, op := ts[0], ts[1]
	require.Equal(t, ema1d.ActionClose, cl.Action)
	require.Equal(t, ema1d.SideLong, cl.Side)
	require.Equal(t, pos.ID, *cl.PositionID)
	require.Equal(t, "2026-10-03", cl.SignalDate)
	require.Equal(t, "0.0213", cl.RefPrice.String(), "entry today: its open is the exit open too")
	require.Equal(t, ema1d.ActionOpen, op.Action)
	require.Equal(t, ema1d.SideShort, op.Side)
	require.Equal(t, cl.ID, *op.DependsOnID)
}

func TestSyncOldEntryNotOpenedButHeldIsClosed(t *testing.T) {
	s := newSyncEnv(t)
	s.position(s.eth, ema1d.SideShort, "2026-10-01", "1", "2400")

	res := s.sync("SUN:L:2026-10-08:0.02:XO,ETH:L:2026-10-07:2400:XO,BTC:F:-:1:-")

	require.Equal(t, []tk{{"ETH", "CLOSE", "SHORT"}}, s.todoKeys())
	r := reasons(res)
	require.Equal(t, "entry_too_old (before 2026-10-09)", r["SUN"])
	require.Equal(t, "entry_too_old (before 2026-10-09)", r["ETH"])
}

func TestSyncSameSideNewEntryReenters(t *testing.T) {
	s := newSyncEnv(t)
	s.position(s.sun, ema1d.SideShort, "2026-10-02", "1000", "0.02")

	res := s.sync("SUN:S:2026-10-10:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-")

	require.Equal(t, []tk{{"SUN", "CLOSE", "SHORT"}, {"SUN", "OPEN", "SHORT"}}, s.todoKeys())
	require.Contains(t, res.Warnings, "reentered: SUN SHORT 2026-10-02 -> 2026-10-10")
}

func TestSyncLateEntryIsKept(t *testing.T) {
	s := newSyncEnv(t)
	s.setPair(s.sun.ID, func(p *dbq.Ema1dPair) { p.AddedOn = "2026-10-10" })
	s.position(s.sun, ema1d.SideShort, "2026-10-10", "1000", "0.02") // opened by hand a day after the signal

	res := s.sync("SUN:S:2026-10-09:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-")

	require.Equal(t, 0, s.count("ema1d_todos"))
	require.Contains(t, res.Warnings, "late_entry: SUN SHORT 2026-10-10 (strategy 2026-10-09), kept")
	for _, w := range res.Warnings {
		require.NotContains(t, w, "reentered")
	}
}

func TestSyncLateEntryStillClosedOnFlatOrReverse(t *testing.T) {
	s := newSyncEnv(t)
	s.position(s.sun, ema1d.SideShort, "2026-10-10", "1000", "0.02")
	s.position(s.eth, ema1d.SideShort, "2026-10-10", "1", "2400")

	s.sync("SUN:F:-:0.02:-,ETH:L:2026-10-10:2400:DIV4H,BTC:F:-:1:-")

	require.Equal(t, []tk{{"SUN", "CLOSE", "SHORT"}, {"ETH", "CLOSE", "SHORT"}, {"ETH", "OPEN", "LONG"}}, s.todoKeys())
}

func TestSyncInFlightOpenDatedAfterEntryIsKept(t *testing.T) {
	s := newSyncEnv(t)
	_, err := s.q().CreateTodo(s.ctx, dbq.CreateTodoParams{Ema1dPairID: s.sun.ID, Action: "OPEN", Side: "SHORT", SignalDate: "2026-10-10", Source: "SYNC"})
	require.NoError(t, err)

	res := s.sync("SUN:S:2026-10-09:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-")

	require.Equal(t, ema1d.TodoPending, s.todos()[0].Status)
	require.Empty(t, res.CancelledTodos)
}

func TestSyncMatchingPositionLeftAlone(t *testing.T) {
	s := newSyncEnv(t)
	s.position(s.sun, ema1d.SideShort, "2026-10-09", "1000", "0.02")
	s.sync("SUN:S:2026-10-09:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-")
	require.Equal(t, 0, s.count("ema1d_todos"))
}

func TestSyncExitOnlyPairsOnlyClose(t *testing.T) {
	s := newSyncEnv(t)
	s.setPair(s.sun.ID, func(p *dbq.Ema1dPair) { p.Status = ema1d.PairExitOnly })
	s.setPair(s.eth.ID, func(p *dbq.Ema1dPair) { p.Status = ema1d.PairExitOnly })
	s.position(s.eth, ema1d.SideLong, "2026-10-01", "1", "2400")

	res := s.sync("SUN:L:2026-10-10:0.02:XO,ETH:F:-:2400:-,BTC:F:-:1:-")

	require.Equal(t, []tk{{"ETH", "CLOSE", "LONG"}}, s.todoKeys())
	require.Equal(t, []ema1d.SkippedEntry{{Coin: "SUN", Action: "OPEN", Side: "LONG", Reason: "pair_exit_only"}}, res.Skipped)
}

func TestSyncShortNotSupportedAndSideNotAllowed(t *testing.T) {
	s := newSyncEnv(t)
	s.setPair(s.sun.ID, func(p *dbq.Ema1dPair) { p.ShortSupported = false })

	res := s.sync("SUN:S:2026-10-10:0.02:XO,ETH:F:-:1:-,BTC:S:2026-10-10:120000:XO")

	require.Equal(t, 0, s.count("ema1d_todos"))
	r := reasons(res)
	require.Equal(t, "short_not_supported", r["SUN"])
	require.Equal(t, "side_not_allowed", r["BTC"])
}

func TestSyncEntriesBeforeAddedOnIgnored(t *testing.T) {
	s := newSyncEnv(t)
	s.setPair(s.sun.ID, func(p *dbq.Ema1dPair) { p.AddedOn = "2026-10-10" })
	res := s.sync("SUN:L:2026-10-09:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-")
	require.Equal(t, 0, s.count("ema1d_todos"))
	require.Equal(t, "entry_before_added_on (2026-10-10)", res.Skipped[0].Reason)
}

func TestSyncPauseOpenBlocksEntriesNotCloses(t *testing.T) {
	s := newSyncEnv(t)
	s.config(func(c *ema1d.Config) { c.PauseOpen = true })
	s.position(s.eth, ema1d.SideLong, "2026-10-01", "1", "2400")

	res := s.sync("SUN:L:2026-10-10:0.02:XO,ETH:F:-:2400:-,BTC:F:-:1:-")

	require.Equal(t, []tk{{"ETH", "CLOSE", "LONG"}}, s.todoKeys())
	require.Equal(t, "pause_open", res.Skipped[0].Reason)
}

func TestSyncSameRunIDReplays(t *testing.T) {
	s := newSyncEnv(t)
	q := "SUN:S:2026-10-10:0.0213:XO,ETH:F:-:1:-,BTC:F:-:1:-"
	first := s.sync(q, "20261010-0911")
	second := s.sync(q, "20261010-0911")

	require.Equal(t, 1, s.count("ema1d_todos"))
	require.Equal(t, 1, s.count("ema1d_syncs WHERE dry_run = 0"))
	require.True(t, second.Replayed)
	require.Equal(t, first.CreatedTodos, second.CreatedTodos)
}

func TestSyncLaterRunSameStateCreatesNothing(t *testing.T) {
	s := newSyncEnv(t)
	q := "SUN:S:2026-10-10:0.0213:XO,ETH:F:-:1:-,BTC:F:-:1:-"
	s.sync(q, "run-a")
	second := s.sync(q, "run-b")
	require.Equal(t, 1, s.count("ema1d_todos"))
	require.Empty(t, second.CreatedTodos)
}

func TestSyncCancelsUnstartedOpenWhenFlat(t *testing.T) {
	s := newSyncEnv(t)
	s.sync("SUN:L:2026-10-10:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-", "run-a")
	res := s.sync("SUN:F:-:0.02:-,ETH:F:-:1:-,BTC:F:-:1:-", "run-b")
	require.Equal(t, ema1d.TodoCancelled, s.todos()[0].Status)
	require.Len(t, res.CancelledTodos, 1)
}

func TestSyncAttemptedOpenNotCancelledCloseWaits(t *testing.T) {
	s := newSyncEnv(t)
	s.sync("SUN:L:2026-10-10:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-", "run-a")
	s.exec("UPDATE ema1d_todos SET attempts = 1, step = 'BORROWED'")

	s.sync("SUN:F:-:0.02:-,ETH:F:-:1:-,BTC:F:-:1:-", "run-b")

	ts := s.todos()
	require.Len(t, ts, 2)
	open, cl := ts[0], ts[1]
	require.Equal(t, ema1d.TodoPending, open.Status)
	require.Equal(t, ema1d.ActionClose, cl.Action)
	require.Equal(t, open.ID, *cl.DependsOnID)
	require.Equal(t, ema1d.SideLong, cl.Side)
}

func TestSyncFailedCloseIsRequeued(t *testing.T) {
	s := newSyncEnv(t)
	pos := s.position(s.eth, ema1d.SideShort, "2026-10-01", "1", "2400")
	failed, err := s.q().CreateTodo(s.ctx, dbq.CreateTodoParams{Ema1dPairID: s.eth.ID, Action: "CLOSE", Side: "SHORT", SignalDate: "2026-10-01", Source: "SYNC", PositionID: &pos.ID})
	require.NoError(t, err)
	s.exec("UPDATE ema1d_todos SET status = 'FAILED', attempts = 10, last_error = 'timeout' WHERE id = ?", failed.ID)

	res := s.sync("SUN:F:-:1:-,ETH:F:-:2400:-,BTC:F:-:1:-")

	got := s.todo(failed.ID)
	require.Equal(t, ema1d.TodoPending, got.Status)
	require.Zero(t, got.Attempts)
	require.Equal(t, 1, s.count("ema1d_todos"))
	require.Contains(t, res.Warnings, "close_requeued: ETH SHORT 2026-10-01 (was FAILED)")
}

func TestSyncFailedOpenNotRecreated(t *testing.T) {
	s := newSyncEnv(t)
	o, err := s.q().CreateTodo(s.ctx, dbq.CreateTodoParams{Ema1dPairID: s.sun.ID, Action: "OPEN", Side: "LONG", SignalDate: "2026-10-10", Source: "SYNC"})
	require.NoError(t, err)
	s.exec("UPDATE ema1d_todos SET status = 'FAILED' WHERE id = ?", o.ID)

	res := s.sync("SUN:L:2026-10-10:0.02:XO,ETH:F:-:1:-,BTC:F:-:1:-")

	require.Equal(t, 1, s.count("ema1d_todos"))
	require.Contains(t, res.Skipped[0].Reason, "open_already_failed")
}

func TestSyncWarnings(t *testing.T) {
	s := newSyncEnv(t)
	s.setPair(s.btc.ID, func(p *dbq.Ema1dPair) { p.Status = ema1d.PairDisabled })
	s.position(s.btc, ema1d.SideLong, "2026-10-01", "0.01", "120000")

	res := s.sync("SUN:X:-:1:-,DOGE:L:2026-10-10:0.1:XO,BTC:F:-:1:-")

	require.Equal(t, 0, s.count("ema1d_todos"))
	for _, w := range []string{"invalid_item: SUN:X:-:1:- (state must be L, S or F)", "unknown_coin: DOGE", "missing: SUN", "missing: ETH", "disabled_with_open_trade: BTC"} {
		require.Contains(t, res.Warnings, w)
	}
}

func TestSyncDryRun(t *testing.T) {
	s := newSyncEnv(t)
	s.position(s.sun, ema1d.SideLong, "2026-10-03", "1000", "0.02")
	q := "SUN:S:2026-10-10:0.0213:XO,ETH:F:-:1:-,BTC:F:-:1:-"

	dry := s.sync(q, "run-x", "1")
	require.True(t, dry.DryRun)
	require.Len(t, dry.CreatedTodos, 2)
	require.Equal(t, "CLOSE", dry.CreatedTodos[0].Action)
	require.Equal(t, "LONG", dry.CreatedTodos[0].Side)
	require.Equal(t, "OPEN", dry.CreatedTodos[1].Action)
	require.Nil(t, dry.CreatedTodos[0].ID, "dry run hides ids")
	require.Equal(t, 0, s.count("ema1d_todos"))
	require.Equal(t, 1, s.count("ema1d_syncs WHERE dry_run = 1"))

	// The real call right after, with the same run_id, is not mistaken for a replay.
	real := s.sync(q, "run-x")
	require.False(t, real.Replayed)
	require.Equal(t, 2, s.count("ema1d_todos"))
}

func TestSyncRejectsBadRequests(t *testing.T) {
	s := newSyncEnv(t)
	for _, req := range []ema1d.SyncRequest{
		{RunID: "a", S: "SUN:F:-:1:-"},
		{Date: syncDate, S: "SUN:F:-:1:-"},
		{Date: syncDate, RunID: "a"},
		{Date: "10/10/2026", RunID: "a", S: "x"},
	} {
		_, err := s.s.Sync(s.ctx, req, "")
		var he *httpx.Error
		require.ErrorAs(t, err, &he)
		require.Equal(t, 400, he.Status)
	}
}
