package emasettings_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/emasettings"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/testkit"
)

type env struct {
	t     *testing.T
	k     *testkit.Kit
	s     *emasettings.Service
	now   time.Time
	ctx   context.Context
	daily []binancefake.Kline
}

func d(s string) *money.Dec { v := money.MustParse(s); return &v }
func str(s string) *string  { return &s }

func newEnv(t *testing.T) *env {
	k := testkit.New(t, testkit.WithTrading())
	e := &env{t: t, k: k, s: k.App.EMASets, now: time.Date(2026, 10, 28, 9, 30, 0, 0, time.UTC), ctx: context.Background()}
	e.s.Now = func() time.Time { return e.now }
	k.Exchange.FundCross("USDT", "10000")
	k.Exchange.FundSpot("USDT", "10000")
	cfg, v, err := e.s.Config(e.ctx)
	require.NoError(t, err)
	cfg.Enabled = true
	_, err = e.s.SaveConfig(e.ctx, v, cfg, "test")
	require.NoError(t, err)
	return e
}

// pair creates SUNUSDT (cross margin unless spot) at price.
func (e *env) pair(spot bool, price string) int64 {
	e.t.Helper()
	e.k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", StepSize: "1", MinNotional: "5", Margin: true, Borrowable: true}, price)
	p, err := e.k.App.Pairs.Create(e.ctx, marketdata.CreateInput{Name: "SUNUSDT", IsSpot: spot})
	require.NoError(e.t, err)
	return p.ID
}

// trend serves 80 flat daily candles at 1 then 20 moving by step per candle (+ up, − down), the
// last one being the running candle, and sets the ticker to the last close. Returns that price.
func (e *env) trend(step float64) string {
	running := e.now.Truncate(24 * time.Hour).Unix()
	closes := make([]float64, 100)
	for i := range closes {
		closes[i] = 1
		if i >= 80 {
			closes[i] = closes[i-1] * (1 + step)
		}
	}
	rows := make([]binancefake.Kline, len(closes))
	for i, c := range closes {
		v := fmt.Sprintf("%.6f", c)
		rows[i] = binancefake.Kline{Time: running - int64(len(closes)-1-i)*86400, Open: v, High: v, Low: v, Close: v}
	}
	e.daily = rows
	e.k.Exchange.SetKlines("SUNUSDT", "1d", rows)
	last := fmt.Sprintf("%.6f", closes[len(closes)-1])
	e.k.Exchange.SetPrice("SUNUSDT", last)
	return last
}

// spike replaces the running candle with price, leaving the history as it is.
func (e *env) spike(price string) {
	e.t.Helper()
	rows := append([]binancefake.Kline(nil), e.daily...)
	last := rows[len(rows)-1]
	last.Open, last.High, last.Low, last.Close = price, price, price, price
	rows[len(rows)-1] = last
	e.k.Exchange.SetKlines("SUNUSDT", "1d", rows)
	e.k.Exchange.SetPrice("SUNUSDT", price)
}

func (e *env) setting(pairID int64, in emasettings.SettingInput) emasettings.SettingRow {
	e.t.Helper()
	in.PairID = pairID
	if in.Budget == nil {
		in.Budget = d("100")
	}
	if in.CheckType == nil {
		in.CheckType = str(emasettings.CheckAtCurrent)
	}
	row, err := e.s.CreateSetting(e.ctx, in)
	require.NoError(e.t, err)
	return row
}

func (e *env) get(id int64) emasettings.SettingRow {
	e.t.Helper()
	row, err := e.s.Get(e.ctx, id)
	require.NoError(e.t, err)
	return row
}

func (e *env) check() {
	e.t.Helper()
	_, err := e.s.Check(e.ctx)
	require.NoError(e.t, err)
}

func (e *env) orders(msgContains string) int {
	n := 0
	for _, m := range e.k.Recorder.Messages {
		if strings.Contains(m.Msg, msgContains) {
			n++
		}
	}
	return n
}

func TestLongOpensOnTheCrossAndStopsOut(t *testing.T) {
	e := newEnv(t)
	set := e.setting(e.pair(false, "1"), emasettings.SettingInput{})
	price := e.trend(0.003)

	e.check()
	row := e.get(set.ID)
	require.Len(t, row.Trades, 1)
	tr := row.Trades[0]
	require.Equal(t, emasettings.SideLong, tr.Side)
	require.Equal(t, emasettings.TradeOpen, tr.Status)
	require.True(t, tr.IsAuto)
	require.True(t, tr.Amount.IsPositive())
	require.Equal(t, price, tr.OpenPrice.Round(6).StringFixed(6))
	require.Equal(t, emasettings.StopLossPrice(emasettings.SideLong, *tr.OpenPrice, money.FromInt(8)).String(), tr.SlPrice.String(), "8% D1 stop loss")
	require.Equal(t, 1, e.orders("AUTO OPENED LONG"))

	// A second scan holds: one trade, no new order.
	e.check()
	require.Len(t, e.get(set.ID).Trades, 1)

	// The price falls through the stop loss.
	e.spike(tr.SlPrice.Mul(money.MustParse("0.99")).Round(6).String())
	e.check()
	tr = e.get(set.ID).Trades[len(e.get(set.ID).Trades)-1]
	require.Equal(t, emasettings.TradeClosed, tr.Status)
	require.Equal(t, emasettings.ReasonSL, tr.CloseReason)
	require.True(t, tr.RealizedProfit.IsNegative())
	require.True(t, tr.FeeUsdt.IsPositive(), "net PnL books both fees")
	require.Equal(t, 1, e.orders("AUTO CLOSED LONG"))
}

func TestShortBorrowsSellsAndRepaysTheWholeDebtOnTheCrossBack(t *testing.T) {
	e := newEnv(t)
	// No stop loss, so the turn of the market closes the short on the cross back, not on the stop.
	cfg, v, _ := e.s.Config(e.ctx)
	cfg.D1.StopLossPercent = money.Zero
	_, err := e.s.SaveConfig(e.ctx, v, cfg, "test")
	require.NoError(t, err)
	set := e.setting(e.pair(false, "1"), emasettings.SettingInput{})
	e.trend(-0.003)

	e.check()
	tr := e.get(set.ID).Trades[0]
	require.Equal(t, emasettings.SideShort, tr.Side)
	require.Equal(t, emasettings.TradeOpen, tr.Status)
	require.Equal(t, tr.Amount.String(), e.k.Exchange.Debt("SUN").String())
	require.Equal(t, "BORROWED", tr.BorrowStep)

	// Interest accrues; then the market turns up hard: EMA14 back above Smooth10.
	e.k.Exchange.AccrueInterest("SUN", "0.5")
	e.trend(0.003)
	e.check()
	trades := e.get(set.ID).Trades
	closed := trades[len(trades)-1]
	require.Equal(t, tr.ID, closed.ID)
	require.Equal(t, emasettings.TradeClosed, closed.Status)
	require.Equal(t, emasettings.ReasonEMAReverse, closed.CloseReason)
	require.True(t, closed.InterestUsdt.IsPositive(), "the interest is repaid and booked")
	require.True(t, e.k.Exchange.Debt("SUN").IsZero(), "nothing is owed after the close")
	// BOTH flips: a LONG follows in the same scan.
	require.Equal(t, emasettings.SideLong, trades[0].Side)
}

func TestSharedCoinDebtRepaysThePrincipalOnly(t *testing.T) {
	e := newEnv(t)
	pairID := e.pair(false, "2")
	e.trend(0) // flat: no automatic trade
	a := e.setting(pairID, emasettings.SettingInput{NextTradeType: str(emasettings.NotNewTrade)})
	b := e.setting(pairID, emasettings.SettingInput{NextTradeType: str(emasettings.NotNewTrade)})
	ta, err := e.s.CreateTrade(e.ctx, a.ID, emasettings.TradeInput{Side: emasettings.SideShort, Amount: d("40")})
	require.NoError(t, err)
	tb, err := e.s.CreateTrade(e.ctx, b.ID, emasettings.TradeInput{Side: emasettings.SideShort, Amount: d("60")})
	require.NoError(t, err)
	_, err = e.s.ForceOpen(e.ctx, ta.ID)
	require.NoError(t, err)
	_, err = e.s.ForceOpen(e.ctx, tb.ID)
	require.NoError(t, err)
	require.Equal(t, "100", e.k.Exchange.Debt("SUN").String())
	e.k.Exchange.AccrueInterest("SUN", "1")

	_, err = e.s.ForceClose(e.ctx, ta.ID)
	require.NoError(t, err)
	require.Equal(t, "61", e.k.Exchange.Debt("SUN").String(), "trade b's 60 and the shared interest stay owed")
	require.Equal(t, 1, e.orders("shares the SUN debt with EMA Settings #"))

	_, err = e.s.ForceClose(e.ctx, tb.ID)
	require.NoError(t, err)
	require.True(t, e.k.Exchange.Debt("SUN").IsZero(), "the last short repays everything")
}

func TestShortRetryBorrowsOnce(t *testing.T) {
	e := newEnv(t)
	pairID := e.pair(false, "2")
	e.trend(0)
	set := e.setting(pairID, emasettings.SettingInput{NextTradeType: str(emasettings.NotNewTrade)})
	tr, err := e.s.CreateTrade(e.ctx, set.ID, emasettings.TradeInput{Side: emasettings.SideShort, Amount: d("50")})
	require.NoError(t, err)

	e.k.Exchange.Fail(binancefake.Failure{Path: "/sapi/v1/margin/order", Method: "POST", Times: 1, HTTP: 500})
	_, err = e.s.ForceOpen(e.ctx, tr.ID)
	require.Error(t, err)
	got := e.get(set.ID).Trades[0]
	require.Equal(t, emasettings.TradeOpening, got.Status)
	require.Equal(t, "BORROWED", got.BorrowStep)
	require.NotEmpty(t, got.LastError)
	require.Error(t, e.s.DeleteTrade(e.ctx, tr.ID), "a borrowed trade cannot be dropped")

	_, err = e.s.ForceOpen(e.ctx, tr.ID)
	require.NoError(t, err)
	borrows, _ := e.k.Exchange.Loans()
	require.Equal(t, 1, borrows)
	require.Equal(t, "50", e.k.Exchange.Debt("SUN").String())
}

func TestManualTradesSplitPartialCloseAndCloseSetting(t *testing.T) {
	e := newEnv(t)
	pairID := e.pair(true, "2") // spot pair: longs trade on spot
	e.trend(0)
	e.k.Exchange.FundSpot("SUN", "100")
	set := e.setting(pairID, emasettings.SettingInput{NextTradeType: str(emasettings.NotNewTrade)})

	// A position already held is recorded without any order.
	held, err := e.s.CreateTrade(e.ctx, set.ID, emasettings.TradeInput{Side: emasettings.SideLong, Status: emasettings.TradeOpen,
		Amount: d("100"), OpenPrice: d("1.5")})
	require.NoError(t, err)
	require.Zero(t, e.k.Exchange.OrderCount())

	part, err := e.s.Split(e.ctx, held.ID, money.FromInt(30))
	require.NoError(t, err)
	require.Equal(t, "30", part.Amount.String())
	require.Zero(t, e.k.Exchange.OrderCount())

	closed, err := e.s.PartialClose(e.ctx, held.ID, money.FromInt(20))
	require.NoError(t, err)
	require.Equal(t, emasettings.TradeClosed, closed.Status)
	require.Equal(t, "20", closed.Amount.String())
	require.Equal(t, 1, e.k.Exchange.OrderCount())

	waiting, err := e.s.CreateTrade(e.ctx, set.ID, emasettings.TradeInput{Side: emasettings.SideLong, EntryPrice: d("1")})
	require.NoError(t, err)
	require.Equal(t, emasettings.TradeOpening, waiting.Status)

	row, err := e.s.CloseSetting(e.ctx, set.ID)
	require.NoError(t, err)
	require.Equal(t, emasettings.StatusDone, row.Status)
	reasons := map[string]int{}
	for _, tr := range row.Trades {
		require.Equal(t, emasettings.TradeClosed, tr.Status)
		reasons[tr.CloseReason]++
	}
	require.Equal(t, map[string]int{emasettings.ReasonManual: 3, emasettings.ReasonCancelled: 1}, reasons)
	require.Equal(t, 3, e.k.Exchange.OrderCount(), "the two held trades sold, the waiting one cancelled")
}

func TestSettingTakeProfitClosesTheCampaign(t *testing.T) {
	e := newEnv(t)
	pairID := e.pair(false, "2")
	e.trend(0)
	set := e.setting(pairID, emasettings.SettingInput{Budget: d("100"), TpPercent: d("10"), NextTradeType: str(emasettings.NotNewTrade)})
	e.k.Exchange.FundCross("SUN", "50")
	_, err := e.s.CreateTrade(e.ctx, set.ID, emasettings.TradeInput{Side: emasettings.SideLong, Status: emasettings.TradeOpen,
		Amount: d("50"), OpenPrice: d("0.5")})
	require.NoError(t, err)

	e.check() // (1 - 0.5) * 50 = 25 USDT = 25% of the budget
	row := e.get(set.ID)
	require.Equal(t, emasettings.StatusDone, row.Status)
	require.Equal(t, emasettings.ReasonSettingTP, row.Trades[0].CloseReason)
	require.Equal(t, 1, e.orders("setting #"))
}

func TestScanOffAndNotActiveDoNothing(t *testing.T) {
	e := newEnv(t)
	set := e.setting(e.pair(false, "1"), emasettings.SettingInput{Status: str(emasettings.StatusInactive)})
	e.trend(0.003)
	_, err := e.s.Check(e.ctx)
	require.Error(t, err) // skipped: no ACTIVE setting
	require.Empty(t, e.get(set.ID).Trades)

	cfg, v, _ := e.s.Config(e.ctx)
	cfg.Enabled = false
	_, err = e.s.SaveConfig(e.ctx, v, cfg, "test")
	require.NoError(t, err)
	_, err = e.s.UpdateSetting(e.ctx, set.ID, emasettings.SettingInput{Status: str(emasettings.StatusActive)})
	require.NoError(t, err)
	_, err = e.s.Check(e.ctx)
	require.ErrorContains(t, err, "EMA Settings is off")
	require.Zero(t, e.k.Exchange.OrderCount())
}
