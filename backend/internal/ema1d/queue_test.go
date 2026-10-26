package ema1d_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/testkit"
)

// Ported from test/services/ema1d/process_todos_service_test.rb.

type queueEnv struct {
	*env
	sun, eth dbq.Ema1dPair
	sleeps   []time.Duration
}

func newQueueEnv(t *testing.T) *queueEnv {
	// Real time: the DB stamps created_at/updated_at with its own clock.
	e := newEnv(t, time.Now().UTC(), testkit.WithTrading())
	q := &queueEnv{env: e}
	e.s.Sleep = func(d time.Duration) { q.sleeps = append(q.sleeps, d) }
	e.config(func(c *ema1d.Config) { c.Enabled = true })
	q.sun = e.pair("SUN", "0.02", "1")
	q.eth = e.pair("ETH", "2000", "0.0001")
	e.ex.FundCross("USDT", "10000")
	return q
}

type todoOpt func(*dbq.CreateTodoParams)

func dependsOn(id int64) todoOpt { return func(p *dbq.CreateTodoParams) { p.DependsOnID = &id } }
func withRef(ref string) todoOpt {
	return func(p *dbq.CreateTodoParams) { r := dec(ref); p.RefPrice = &r }
}
func signal(date string) todoOpt  { return func(p *dbq.CreateTodoParams) { p.SignalDate = date } }
func onPosition(id int64) todoOpt { return func(p *dbq.CreateTodoParams) { p.PositionID = &id } }

func (q *queueEnv) create(p dbq.Ema1dPair, action, side string, opts ...todoOpt) dbq.Ema1dTodo {
	q.t.Helper()
	ref, rule := dec("0.0213"), "XO"
	params := dbq.CreateTodoParams{Ema1dPairID: p.ID, Action: action, Side: side, SignalDate: "2026-10-10", RefPrice: &ref, ExitRule: &rule, Source: "SYNC"}
	for _, o := range opts {
		o(&params)
	}
	t, err := q.q().CreateTodo(q.ctx, params)
	require.NoError(q.t, err)
	return t
}

func (q *queueEnv) process() {
	q.t.Helper()
	_, _ = q.s.ProcessTodos(q.ctx)
}

func (q *queueEnv) messages() []string {
	var out []string
	for _, m := range q.k.Recorder.Messages {
		out = append(out, m.Msg)
	}
	return out
}

func (q *queueEnv) lastMessage() string {
	m := q.messages()
	if len(m) == 0 {
		return ""
	}
	return m[len(m)-1]
}

func TestOpenLongBuysSevenPercentAndRecordsNetAmountAndFee(t *testing.T) {
	q := newQueueEnv(t)
	todo := q.create(q.sun, "OPEN", "LONG")

	q.process()

	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoDone, got.Status)
	pos := q.pos(*got.PositionID)
	require.Equal(t, ema1d.PosOpen, pos.Status)
	require.Equal(t, "20979", pos.Amount.String(), "21000 bought (420 USDT), 0.1% fee taken in SUN")
	require.Equal(t, "0.02", pos.OpenPrice.String())
	require.Equal(t, "0.42", pos.FeeUsdt.String())
	require.Equal(t, "7", pos.SizePercent.String())
	require.Equal(t, "6000", pos.EquityAtOpen.String())
	require.Equal(t, "420", pos.NotionalUsdt.String())
	require.Equal(t, "2026-10-10", pos.SignalDate)
	require.Equal(t, "0.0213", pos.RefOpenPrice.String())
	require.Regexp(t, `OPEN LONG SUN .* DONE: 20979 @ 0.02 \(ref 0.0213, -6.1%\)`, q.lastMessage())
	require.Equal(t, []string{fmt.Sprintf("bw_e1_%d_open", pos.ID)}, q.ex.OrderLog())
}

func TestOpenShortBorrowsOnceAndSells(t *testing.T) {
	q := newQueueEnv(t)
	todo := q.create(q.sun, "OPEN", "SHORT")

	q.process()

	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoDone, got.Status)
	require.Equal(t, ema1d.StepBorrowed, *got.Step)
	require.Equal(t, "10500", q.pos(*got.PositionID).Amount.String())
	require.Equal(t, "10500", q.ex.Debt("SUN").String())
	borrows, _ := q.ex.Loans()
	require.Equal(t, 1, borrows)
}

func TestSellFailingAfterBorrowRetriesWithoutBorrowingAgain(t *testing.T) {
	q := newQueueEnv(t)
	q.ex.Fail(binancefake.Failure{Path: "/sapi/v1/margin/order", Method: "POST", HTTP: 503})
	todo := q.create(q.sun, "OPEN", "SHORT")

	q.process()
	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoPending, got.Status)
	require.EqualValues(t, 1, got.Attempts)
	require.Equal(t, ema1d.StepBorrowed, *got.Step)
	require.WithinDuration(t, q.now.Add(time.Minute), got.RunAfter.Time, time.Second)
	require.Empty(t, q.messages(), "retries stay quiet")

	q.travel(2 * time.Minute)
	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(todo.ID).Status)
	borrows, _ := q.ex.Loans()
	require.Equal(t, 1, borrows)
	require.Equal(t, "10500", q.ex.Debt("SUN").String())
}

func TestLostBorrowResponseIsFoundInHistory(t *testing.T) {
	q := newQueueEnv(t)
	q.ex.Fail(binancefake.Failure{Path: "/sapi/v1/margin/borrow-repay", Method: "POST", HTTP: 503, AfterApply: true})
	todo := q.create(q.sun, "OPEN", "SHORT")

	q.process()
	require.Equal(t, ema1d.StepBorrowing, *q.todo(todo.ID).Step)

	q.travel(2 * time.Minute)
	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(todo.ID).Status)
	borrows, _ := q.ex.Loans()
	require.Equal(t, 1, borrows)
	require.Equal(t, "10500", q.ex.Debt("SUN").String())
}

func TestBorrowThatNeverReachedBinanceIsSentAgain(t *testing.T) {
	q := newQueueEnv(t)
	q.ex.Fail(binancefake.Failure{Path: "/sapi/v1/margin/borrow-repay", Method: "POST", HTTP: 503})
	todo := q.create(q.sun, "OPEN", "SHORT")

	q.process()
	q.travel(2 * time.Minute)
	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(todo.ID).Status)
	require.Equal(t, "10500", q.ex.Debt("SUN").String())
}

func TestSellRefusedForGoodRepaysTheBorrowAndLeavesNoPosition(t *testing.T) {
	q := newQueueEnv(t)
	q.ex.Fail(binancefake.Failure{Path: "/sapi/v1/margin/order", Method: "POST", Code: -1013, Msg: "Filter failure: LOT_SIZE"})
	todo := q.create(q.sun, "OPEN", "SHORT")

	q.process()

	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoFailed, got.Status)
	require.Equal(t, ema1d.StepBorrowReverted, *got.Step)
	require.Nil(t, got.PositionID)
	require.Equal(t, 0, q.count("ema1d_positions"))
	require.True(t, q.ex.Debt("SUN").IsZero())
	require.Regexp(t, `(?s)FAILED after 1 attempt.*LOT_SIZE.*\nborrowed 10500 SUN repaid`, q.lastMessage())
}

func TestTransientErrorsGiveUpAfterTenAttempts(t *testing.T) {
	q := newQueueEnv(t)
	todo := q.create(q.sun, "OPEN", "LONG")
	q.exec("UPDATE ema1d_todos SET attempts = 9, first_attempted_at = ? WHERE id = ?", dbtype.NewTime(q.now.Add(-time.Hour)), todo.ID)
	q.ex.Fail(binancefake.Failure{Path: "/sapi/v1/margin/order", Method: "POST", Code: -1003, Msg: "Too many requests"})

	q.process()

	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoFailed, got.Status)
	require.EqualValues(t, 10, got.Attempts)
	require.Contains(t, q.lastMessage(), "FAILED after 10 attempt")
}

func TestCloseShortBuysBackPrincipalAndInterestAndChargesBothInPnL(t *testing.T) {
	q := newQueueEnv(t)
	open := q.create(q.sun, "OPEN", "SHORT")
	q.process()
	posID := *q.todo(open.ID).PositionID

	q.ex.AccrueInterest("SUN", "5")
	q.ex.SetPrice("SUNUSDT", "0.018")
	cl := q.create(q.sun, "CLOSE", "SHORT", withRef("0.0181"), dependsOn(open.ID))
	q.travel(time.Minute)
	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(cl.ID).Status)
	pos := q.pos(posID)
	require.Equal(t, ema1d.PosClosed, pos.Status)
	require.True(t, q.ex.Debt("SUN").IsZero())
	require.Equal(t, "5", pos.InterestPaid.String())
	require.Equal(t, "0.09", pos.InterestUsdt.String())
	require.Equal(t, "0.0181", pos.RefClosePrice.String())
	// bought ceil((10500 + 5) * 1.002) = 10527 SUN at 0.018
	openFee := dec("10500").Mul(dec("0.02")).Mul(dec("0.001"))
	closeFee := dec("10527").Mul(dec("0.001")).Mul(dec("0.018"))
	require.Equal(t, openFee.Add(closeFee).String(), pos.FeeUsdt.String())
	want := dec("0.002").Mul(dec("10500")).Sub(openFee).Sub(closeFee).Sub(dec("0.09"))
	require.Equal(t, want.Round(8).String(), pos.RealizedProfit.String())
}

func TestCloseShortRereadsTheAccountUntilTheCoinShows(t *testing.T) {
	q := newQueueEnv(t)
	open := q.create(q.sun, "OPEN", "SHORT")
	q.process()
	posID := *q.todo(open.ID).PositionID
	cl := q.create(q.sun, "CLOSE", "SHORT", dependsOn(open.ID))
	q.ex.LagNextBuy("SUN", 2)
	q.sleeps = nil
	q.travel(time.Minute)
	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(cl.ID).Status)
	require.Equal(t, ema1d.PosClosed, q.pos(posID).Status)
	require.True(t, q.ex.Debt("SUN").IsZero())
	require.Equal(t, []time.Duration{time.Second, time.Second}, q.sleeps)
}

func TestCloseShortWhoseCoinNeverShowsRetriesTheRepayWithoutBuyingAgain(t *testing.T) {
	q := newQueueEnv(t)
	open := q.create(q.sun, "OPEN", "SHORT")
	q.process()
	posID := *q.todo(open.ID).PositionID
	cl := q.create(q.sun, "CLOSE", "SHORT", dependsOn(open.ID))
	q.ex.LagNextBuy("SUN", 5)
	q.travel(time.Minute)
	q.process()

	got := q.todo(cl.ID)
	require.Equal(t, ema1d.TodoPending, got.Status)
	require.EqualValues(t, 1, got.Attempts)
	require.Contains(t, got.LastError, "not in the cross account yet")
	_, repays := q.ex.Loans()
	require.Zero(t, repays)
	require.Equal(t, ema1d.PosOpen, q.pos(posID).Status)

	q.travel(2 * time.Minute)
	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(cl.ID).Status)
	require.True(t, q.ex.Debt("SUN").IsZero())
	closes := 0
	for _, id := range q.ex.OrderLog() {
		if id == fmt.Sprintf("bw_e1_%d_close", posID) {
			closes++
		}
	}
	require.Equal(t, 1, closes)
}

func TestBinanceErrorKeepsTheRequestInLastError(t *testing.T) {
	q := newQueueEnv(t)
	q.ex.Fail(binancefake.Failure{Path: "/sapi/v1/margin/order", Method: "POST", Code: -1100, Msg: "Illegal characters found in a parameter."})
	todo := q.create(q.sun, "OPEN", "LONG")
	q.process()

	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoFailed, got.Status)
	require.Contains(t, got.LastError, "-1100")
	require.Contains(t, got.LastError, "Illegal characters")
	require.Contains(t, got.LastError, "POST /sapi/v1/margin/order")
	require.Contains(t, got.LastError, "symbol=SUNUSDT")
}

func TestReversalRunsCloseBeforeTheOpenThatDependsOnIt(t *testing.T) {
	q := newQueueEnv(t)
	long := q.create(q.sun, "OPEN", "LONG", signal("2026-10-09"))
	q.process()
	longPos := *q.todo(long.ID).PositionID
	cl := q.create(q.sun, "CLOSE", "LONG", signal("2026-10-09"), onPosition(longPos))
	short := q.create(q.sun, "OPEN", "SHORT", dependsOn(cl.ID))

	q.travel(time.Minute)
	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(cl.ID).Status)
	sh := q.todo(short.ID)
	require.Equal(t, ema1d.TodoDone, sh.Status)
	require.Equal(t, []string{
		fmt.Sprintf("bw_e1_%d_open", longPos), fmt.Sprintf("bw_e1_%d_close", longPos), fmt.Sprintf("bw_e1_%d_open", *sh.PositionID),
	}, q.ex.OrderLog())
}

func TestLongBeyondBudgetFailsAtOnceAndPlacesNothing(t *testing.T) {
	q := newQueueEnv(t)
	p := q.position(q.eth, ema1d.SideLong, "2026-10-01", "2.9", "2000") // 5800 USDT notional
	_ = p
	todo := q.create(q.sun, "OPEN", "LONG")

	q.process()

	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoFailed, got.Status)
	require.Contains(t, got.LastError, "insufficient_budget: open LONG 5800 + 420 > equity 6000 USDT")
	require.Empty(t, q.ex.OrderLog())
	require.Equal(t, 1, q.count("ema1d_positions"))
}

func TestShortOnACoinAlreadyShortFailsAsConflictWithoutBorrowing(t *testing.T) {
	q := newQueueEnv(t)
	other := q.position(q.sun, ema1d.SideShort, "2026-10-01", "100", "0.02")
	todo := q.create(q.sun, "OPEN", "SHORT")

	q.process()

	got := q.todo(todo.ID)
	require.Equal(t, ema1d.TodoFailed, got.Status)
	require.Contains(t, got.LastError, fmt.Sprintf("conflict: position(s) #%d already short SUN", other.ID))
	borrows, _ := q.ex.Loans()
	require.Zero(t, borrows)
}

func TestSizeBelowSuggestedIsRaisedToMinNotional(t *testing.T) {
	q := newQueueEnv(t)
	q.config(func(c *ema1d.Config) { c.Sizes.CoreLong = dec("0.007") }) // 0.42 USDT, a test size
	small := q.create(q.sun, "OPEN", "LONG")
	q.process()
	got := q.todo(small.ID)
	require.Equal(t, ema1d.TodoDone, got.Status)
	pos := q.pos(*got.PositionID)
	require.Equal(t, "5.5", pos.NotionalUsdt.String())
	require.Equal(t, "0.0917", pos.SizePercent.String())

	q.config(func(c *ema1d.Config) { c.Sizes.CoreLong = dec("7") })
	q.exec("UPDATE pairs SET min_notional = '1000' WHERE id = ?", q.eth.PairID)
	full := q.create(q.eth, "OPEN", "LONG")
	q.process()
	require.Equal(t, ema1d.TodoFailed, q.todo(full.ID).Status)
	require.Contains(t, q.todo(full.ID).LastError, "below MIN_NOTIONAL 1000")
}

func TestSizesFollowTheConfig(t *testing.T) {
	q := newQueueEnv(t)
	q.config(func(c *ema1d.Config) { c.Sizes.CoreShort = dec("0.35") })
	todo := q.create(q.sun, "OPEN", "SHORT")
	q.process()
	pos := q.pos(*q.todo(todo.ID).PositionID)
	require.Equal(t, "1050", pos.Amount.String(), "6000 * 0.35% = 21 USDT at 0.02")
	require.Equal(t, "0.35", pos.SizePercent.String())
}

func TestDisabledLeavesTheQueueAlone(t *testing.T) {
	q := newQueueEnv(t)
	q.config(func(c *ema1d.Config) { c.Enabled = false })
	todo := q.create(q.sun, "OPEN", "LONG")
	q.process()
	require.Equal(t, ema1d.TodoPending, q.todo(todo.ID).Status)
	require.Empty(t, q.ex.OrderLog())
}

func TestBrokenDependencyAndStaleOpenAreSkipped(t *testing.T) {
	q := newQueueEnv(t)
	failedClose := q.create(q.sun, "CLOSE", "LONG", signal("2026-10-01"))
	q.exec("UPDATE ema1d_todos SET status = 'FAILED' WHERE id = ?", failedClose.ID)
	reopen := q.create(q.sun, "OPEN", "SHORT", dependsOn(failedClose.ID))
	stale := q.create(q.eth, "OPEN", "LONG", signal("2026-10-08"))
	q.exec("UPDATE ema1d_todos SET created_at = ? WHERE id = ?", dbtype.NewTime(q.now.Add(-25*time.Hour)), stale.ID)

	q.process()

	require.Equal(t, ema1d.TodoSkipped, q.todo(reopen.ID).Status)
	require.Equal(t, ema1d.TodoSkipped, q.todo(stale.ID).Status)
	require.Empty(t, q.ex.OrderLog())
	n := 0
	for _, m := range q.messages() {
		if strings.Contains(m, "SKIPPED") {
			n++
		}
	}
	require.Equal(t, 2, n)
}

func TestStuckRunningTodoIsPickedUpAgain(t *testing.T) {
	q := newQueueEnv(t)
	todo := q.create(q.sun, "OPEN", "LONG")
	q.exec("UPDATE ema1d_todos SET status = 'RUNNING', updated_at = ? WHERE id = ?", dbtype.NewTime(q.now.Add(-20*time.Minute)), todo.ID)

	q.process()

	require.Equal(t, ema1d.TodoDone, q.todo(todo.ID).Status)
}

func TestQueueIsLeftAloneWhileTradingIsDisabled(t *testing.T) {
	e := newEnv(t, time.Now().UTC())
	e.config(func(c *ema1d.Config) { c.Enabled = true })
	sun := e.pair("SUN", "0.02", "1")
	ref := money.MustParse("0.02")
	todo, err := e.q().CreateTodo(e.ctx, dbq.CreateTodoParams{Ema1dPairID: sun.ID, Action: "OPEN", Side: "LONG", SignalDate: "2026-10-10", RefPrice: &ref, Source: "SYNC"})
	require.NoError(t, err)

	_, err = e.s.ProcessTodos(e.ctx)
	require.ErrorContains(t, err, "TRADING_ENABLED=false")
	require.Equal(t, ema1d.TodoPending, e.todo(todo.ID).Status)
	require.Zero(t, e.todo(todo.ID).Attempts)
}
