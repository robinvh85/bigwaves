package dca_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/dca"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
	"bigwaves/internal/testkit"
)

type env struct {
	t   *testing.T
	k   *testkit.Kit
	s   *dca.Service
	ctx context.Context
}

func d(s string) *money.Dec { v := money.MustParse(s); return &v }

func newEnv(t *testing.T) *env {
	k := testkit.New(t, testkit.WithTrading())
	return &env{t: t, k: k, s: k.App.DCA, ctx: context.Background()}
}

// ladder creates SUNUSDT (cross margin unless spot) at price and a DCA setting on it.
func (e *env) ladder(spot bool, price string) dca.SettingRow {
	e.t.Helper()
	e.k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", StepSize: "1", MinNotional: "5", Margin: true}, price)
	p, err := e.k.App.Pairs.Create(e.ctx, marketdata.CreateInput{Name: "SUNUSDT", IsSpot: spot})
	require.NoError(e.t, err)
	row, err := e.s.CreateSetting(e.ctx, p.ID, "")
	require.NoError(e.t, err)
	return row
}

func (e *env) price(p string) {
	e.k.Exchange.SetPrice("SUNUSDT", p)
	require.NoError(e.t, e.k.App.Scheduler.RunSync("market.update_prices"))
}

func (e *env) check() error {
	_, err := e.s.Check(e.ctx)
	return err
}

func (e *env) get(id int64) dca.SettingRow {
	row, err := e.s.Get(e.ctx, id)
	require.NoError(e.t, err)
	return row
}

func TestRungValidation(t *testing.T) {
	e := newEnv(t)
	l := e.ladder(false, "0.02")
	cases := []dca.TradeInput{
		{BuyPrice: d("0.02"), SellPrice: d("0.019"), Amount: d("1000")}, // sell below buy
		{BuyPrice: d("0"), Amount: d("1000")},
		{BuyPrice: d("0.02"), Amount: d("100")}, // 2 USDT < min notional 5
		{BuyPrice: d("0.02"), Amount: d("0.5")}, // below the step size
	}
	for _, in := range cases {
		_, err := e.s.CreateTrade(e.ctx, l.ID, in)
		var he *httpx.Error
		require.ErrorAs(t, err, &he)
		require.Equal(t, 400, he.Status)
	}
	_, err := e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), SellPrice: d("0"), Amount: d("1000")})
	require.NoError(t, err, "sell 0 means hold")
}

func TestBuysAtTriggerSellsAtTargetAndChargesFees(t *testing.T) {
	e := newEnv(t)
	e.k.Exchange.FundCross("USDT", "1000")
	l := e.ladder(false, "0.025")
	rung, err := e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), SellPrice: d("0.03"), Amount: d("1000")})
	require.NoError(t, err)

	require.NoError(t, e.check())
	require.Equal(t, dca.TradeOpening, e.get(l.ID).Trades[0].Status, "0.025 is above the buy trigger")

	e.price("0.019")
	require.NoError(t, e.check())
	tr := e.get(l.ID).Trades[0]
	require.Equal(t, dca.TradeOpen, tr.Status)
	require.Equal(t, "999", tr.HeldAmount.String(), "0.1% fee taken in SUN")
	require.Equal(t, "0.019", tr.OpenPrice.String())
	require.Equal(t, "19", tr.CostUsdt.String())
	require.Equal(t, "0.02", tr.BuyPrice.String(), "the trigger is kept apart from the fill")

	e.price("0.025")
	require.NoError(t, e.check())
	require.Equal(t, "5.975", e.get(l.ID).Trades[0].UnrealizedProfit.String(), "999 * 0.025 - 19")

	e.price("0.031")
	require.NoError(t, e.check())
	tr = e.get(l.ID).Trades[0]
	require.Equal(t, dca.TradeClosed, tr.Status)
	// sold 999 at 0.031 = 30.969, sell fee 0.030969 USDT, bought for 19
	require.Equal(t, "11.938031", tr.RealizedProfit.String())
	require.Equal(t, "0", tr.UnrealizedProfit.String())
	require.Equal(t, []string{"bw_dca_" + itoa(rung.ID) + "_open", "bw_dca_" + itoa(rung.ID) + "_close"}, e.k.Exchange.OrderLog())
	require.True(t, e.k.Exchange.CrossBalance("SUN").Free.IsZero(), "sold exactly what was held")
}

func TestSpotLadderUsesTheSpotWallet(t *testing.T) {
	e := newEnv(t)
	e.k.Exchange.FundSpot("USDT", "100")
	l := e.ladder(true, "0.02")
	_, err := e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), Amount: d("1000")})
	require.NoError(t, err)
	require.NoError(t, e.check())
	require.Equal(t, dca.TradeOpen, e.get(l.ID).Trades[0].Status)
	require.Equal(t, 1, e.k.Exchange.CallCount("POST /api/v3/order"))

	e.price("10")
	require.NoError(t, e.check())
	require.Equal(t, dca.TradeOpen, e.get(l.ID).Trades[0].Status, "sell price 0: held, never sold")
}

func TestCapitalCapBlocksBuys(t *testing.T) {
	e := newEnv(t)
	e.k.Exchange.FundCross("USDT", "1000")
	c, v, err := e.s.Config(e.ctx)
	require.NoError(t, err)
	c.MaxCapitalUSDT = money.MustParse("30")
	_, err = e.s.SaveConfig(e.ctx, v, c, "test")
	require.NoError(t, err)
	l := e.ladder(false, "0.02")
	for range 2 {
		_, err := e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), Amount: d("1000")})
		require.NoError(t, err)
	}
	err = e.check()
	require.ErrorContains(t, err, "capital cap")
	row := e.get(l.ID)
	require.Equal(t, 1, row.HoldingCount, "the second 20 USDT rung would exceed the 30 USDT cap")
	require.Equal(t, 1, row.WaitingCount)
	for _, tr := range row.Trades {
		if tr.Status == dca.TradeOpening {
			require.Contains(t, tr.LastError, "capital cap")
		}
	}
}

func TestLostBuyResponseIsNotBoughtTwice(t *testing.T) {
	e := newEnv(t)
	e.k.Exchange.FundCross("USDT", "1000")
	l := e.ladder(false, "0.02")
	_, err := e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), Amount: d("1000")})
	require.NoError(t, err)
	e.k.Exchange.Fail(binancefake.Failure{Path: "/sapi/v1/margin/order", Method: "POST", HTTP: 503, AfterApply: true})

	require.Error(t, e.check())
	require.Equal(t, dca.TradeOpening, e.get(l.ID).Trades[0].Status)
	require.NoError(t, e.check())
	require.Equal(t, dca.TradeOpen, e.get(l.ID).Trades[0].Status)
	require.Equal(t, 1, e.k.Exchange.OrderCount())
}

func TestEditAndDeleteRules(t *testing.T) {
	e := newEnv(t)
	e.k.Exchange.FundCross("USDT", "1000")
	l := e.ladder(false, "0.02")
	rung, err := e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), Amount: d("1000")})
	require.NoError(t, err)

	_, err = e.s.UpdateTrade(e.ctx, rung.ID, dca.TradeInput{Amount: d("2000")})
	require.NoError(t, err)

	_, err = e.s.ForceBuy(e.ctx, rung.ID)
	require.NoError(t, err)
	_, err = e.s.UpdateTrade(e.ctx, rung.ID, dca.TradeInput{Amount: d("10")})
	require.ErrorContains(t, err, "only the sell price can change")
	_, err = e.s.UpdateTrade(e.ctx, rung.ID, dca.TradeInput{SellPrice: d("0.01")})
	require.ErrorContains(t, err, "above the bought price")
	require.ErrorContains(t, e.s.DeleteTrade(e.ctx, rung.ID), "not yet bought")
	require.ErrorContains(t, e.s.DeleteSetting(e.ctx, l.ID), "still hold coins")

	sold, err := e.s.ForceSell(e.ctx, rung.ID)
	require.NoError(t, err)
	require.Equal(t, dca.TradeClosed, sold.Status)
	require.NoError(t, e.s.DeleteSetting(e.ctx, l.ID))
}

func TestDisabledDoesNothing(t *testing.T) {
	e := newEnv(t)
	c, v, _ := e.s.Config(e.ctx)
	c.Enabled = false
	_, err := e.s.SaveConfig(e.ctx, v, c, "test")
	require.NoError(t, err)
	l := e.ladder(false, "0.01")
	_, err = e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), Amount: d("1000")})
	require.NoError(t, err)
	require.ErrorContains(t, e.check(), "disabled")
	require.Zero(t, e.k.Exchange.OrderCount())
}

func itoa(i int64) string { return money.FromInt(i).String() }

func TestPairAccountLockedWhileRungHeld(t *testing.T) {
	e := newEnv(t)
	e.k.Exchange.FundCross("USDT", "1000")
	l := e.ladder(false, "0.025")
	_, err := e.s.CreateTrade(e.ctx, l.ID, dca.TradeInput{BuyPrice: d("0.02"), SellPrice: d("0.03"), Amount: d("1000")})
	require.NoError(t, err)
	spot := true

	// A rung waiting to buy holds nothing: switching is fine, the buy will use the new account.
	_, err = e.k.App.Pairs.Update(e.ctx, l.PairID, marketdata.UpdateInput{IsSpot: &spot})
	require.NoError(t, err)
	cross := false
	_, err = e.k.App.Pairs.Update(e.ctx, l.PairID, marketdata.UpdateInput{IsSpot: &cross})
	require.NoError(t, err)

	e.price("0.019")
	require.NoError(t, e.check())
	require.Equal(t, dca.TradeOpen, e.get(l.ID).Trades[0].Status)

	// Bought on cross: selling must happen on cross too.
	_, err = e.k.App.Pairs.Update(e.ctx, l.PairID, marketdata.UpdateInput{IsSpot: &spot})
	var he *httpx.Error
	require.ErrorAs(t, err, &he)
	require.Equal(t, "pair_has_holdings", he.Code)

	// Status changes stay allowed.
	disabled := "DISABLED"
	_, err = e.k.App.Pairs.Update(e.ctx, l.PairID, marketdata.UpdateInput{Status: &disabled})
	require.NoError(t, err)
}

func TestLadderShowsWhenItsPriceWasUpdated(t *testing.T) {
	e := newEnv(t)
	l := e.ladder(false, "0.02")
	e.price("0.021")
	got := e.get(l.ID)
	require.Equal(t, "0.021", got.CurrentPrice.String())
	require.NotNil(t, got.PriceUpdatedAt)
	require.WithinDuration(t, time.Now(), *got.PriceUpdatedAt, time.Minute)

	list, err := e.s.List(e.ctx, "")
	require.NoError(t, err)
	require.NotNil(t, list[0].PriceUpdatedAt)
}
