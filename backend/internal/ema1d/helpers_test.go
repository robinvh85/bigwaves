package ema1d_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/testkit"
)

type env struct {
	t   *testing.T
	k   *testkit.Kit
	s   *ema1d.Service
	ex  *binancefake.Exchange
	now time.Time
	ctx context.Context
}

func newEnv(t *testing.T, now time.Time, opts ...testkit.Option) *env {
	t.Helper()
	k := testkit.New(t, opts...)
	e := &env{t: t, k: k, s: k.App.EMA1D, ex: k.Exchange, now: now, ctx: context.Background()}
	e.s.Now = func() time.Time { return e.now }
	e.ex.Now = func() time.Time { return e.now }
	return e
}

func (e *env) travel(d time.Duration) { e.now = e.now.Add(d) }

func (e *env) q() *dbq.Queries { return dbq.New(e.k.App.DB.Write) }

func (e *env) config(mut func(*ema1d.Config)) {
	e.t.Helper()
	c, v, err := e.s.Config(e.ctx)
	require.NoError(e.t, err)
	mut(&c)
	_, err = e.s.SaveConfig(e.ctx, v, c, "test")
	require.NoError(e.t, err)
}

type pairOpt func(*dbq.CreateEma1dPairParams)

func sensitive(p *dbq.CreateEma1dPairParams) { p.PairGroup = ema1d.GroupSensitive }
func longOnly(p *dbq.CreateEma1dPairParams) {
	p.AllowShort, p.ShortSupported = false, false
}

// pair creates the Binance symbol, the pair and the basket row (added 2026-10-01).
func (e *env) pair(coin, price, step string, opts ...pairOpt) dbq.Ema1dPair {
	e.t.Helper()
	e.ex.AddSymbol(binancefake.Symbol{Symbol: coin + "USDT", StepSize: step, MinNotional: "5", Margin: true, Borrowable: true}, price)
	p, err := e.k.App.Pairs.Create(e.ctx, marketdata.CreateInput{Name: coin + "USDT"})
	require.NoError(e.t, err)
	rule := "XO"
	params := dbq.CreateEma1dPairParams{
		PairID: p.ID, PairGroup: ema1d.GroupCore, AllowLong: true, AllowShort: true, ShortSupported: true,
		Status: ema1d.PairActive, AddedOn: "2026-10-01", LongExitRule: &rule,
	}
	for _, o := range opts {
		o(&params)
	}
	row, err := e.q().CreateEma1dPair(e.ctx, params)
	require.NoError(e.t, err)
	return row
}

func (e *env) setPair(id int64, mut func(*dbq.Ema1dPair)) {
	e.t.Helper()
	r, err := e.q().GetEma1dPair(e.ctx, id)
	require.NoError(e.t, err)
	p := r.Ema1dPair
	mut(&p)
	require.NoError(e.t, e.q().UpdateEma1dPair(e.ctx, dbq.UpdateEma1dPairParams{
		PairGroup: p.PairGroup, Status: p.Status, AllowLong: p.AllowLong, AllowShort: p.AllowShort, AddedOn: p.AddedOn,
		RemovedOn: p.RemovedOn, LongExitRule: p.LongExitRule, SignalFrom: p.SignalFrom, ID: p.ID,
	}))
	require.NoError(e.t, e.q().SetEma1dPairCheck(e.ctx, dbq.SetEma1dPairCheckParams{ShortSupported: p.ShortSupported, CheckNote: p.CheckNote, ID: p.ID}))
}

// position inserts an OPEN position as if it had been opened earlier.
func (e *env) position(p dbq.Ema1dPair, side, signalDate string, amount, openPrice string) dbq.Ema1dPosition {
	e.t.Helper()
	pos, err := e.q().CreatePosition(e.ctx, dbq.CreatePositionParams{
		Ema1dPairID: p.ID, PairID: p.PairID, Side: side, SignalDate: signalDate, Amount: money.MustParse(amount),
		SizePercent: money.FromInt(7), EquityAtOpen: money.FromInt(6000), NotionalUsdt: money.MustParse(amount).Mul(money.MustParse(openPrice)),
		OpenOrderID: fmt.Sprintf("seed_open_%d_%s", p.ID, signalDate), CloseOrderID: fmt.Sprintf("seed_close_%d_%s", p.ID, signalDate),
	})
	require.NoError(e.t, err)
	op := money.MustParse(openPrice)
	pos.OpenPrice, pos.Status, pos.OpenAt = &op, ema1d.PosOpen, ptr(dbtype.NewTime(e.now.Add(-48*time.Hour)))
	require.NoError(e.t, e.q().SavePosition(e.ctx, dbq.SavePositionParams{
		Status: pos.Status, Amount: pos.Amount, OpenPrice: pos.OpenPrice, FeeUsdt: pos.FeeUsdt, InterestPaid: pos.InterestPaid,
		InterestUsdt: pos.InterestUsdt, RealizedProfit: pos.RealizedProfit, UnrealizedProfit: pos.UnrealizedProfit,
		OpenAt: pos.OpenAt, ID: pos.ID,
	}))
	return pos
}

func (e *env) todos() []dbq.Ema1dTodo {
	e.t.Helper()
	rows, err := e.k.App.DB.Read.QueryContext(e.ctx, "SELECT id FROM ema1d_todos ORDER BY id")
	require.NoError(e.t, err)
	defer func() { _ = rows.Close() }()
	var out []dbq.Ema1dTodo
	for rows.Next() {
		var id int64
		require.NoError(e.t, rows.Scan(&id))
		t, err := e.q().GetTodo(e.ctx, id)
		require.NoError(e.t, err)
		out = append(out, t)
	}
	require.NoError(e.t, rows.Err())
	return out
}

func (e *env) todo(id int64) dbq.Ema1dTodo {
	e.t.Helper()
	t, err := e.q().GetTodo(e.ctx, id)
	require.NoError(e.t, err)
	return t
}

func (e *env) pos(id int64) dbq.Ema1dPosition {
	e.t.Helper()
	p, err := e.q().GetPosition(e.ctx, id)
	require.NoError(e.t, err)
	return p
}

func (e *env) exec(sqlText string, args ...any) {
	e.t.Helper()
	_, err := e.k.App.DB.Write.ExecContext(e.ctx, sqlText, args...)
	require.NoError(e.t, err)
}

func (e *env) count(table string) int {
	e.t.Helper()
	var n int
	require.NoError(e.t, e.k.App.DB.Read.QueryRowContext(e.ctx, "SELECT COUNT(*) FROM "+table).Scan(&n))
	return n
}

func ptr[T any](v T) *T { return &v }

func dec(s string) money.Dec { return money.MustParse(s) }

var _ = sql.ErrNoRows
