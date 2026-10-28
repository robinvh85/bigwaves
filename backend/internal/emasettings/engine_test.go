package emasettings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/platform/money"
)

func dp(s string) *money.Dec { v := money.MustParse(s); return &v }

func TestEMAWindow(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 10, 28, h, m, 0, 0, time.UTC) }
	cases := []struct {
		check, period string
		now           time.Time
		want          bool
	}{
		{CheckAtCurrent, PeriodD1, at(13, 40), true},
		{CheckAtOpen, PeriodD1, at(0, 0), true},
		{CheckAtOpen, PeriodD1, at(0, 14), true},
		{CheckAtOpen, PeriodD1, at(0, 15), false},
		{CheckAtClose, PeriodD1, at(4, 5), false},
		{CheckAtClose, PeriodH4, at(4, 5), true},
		{CheckAtOpen, PeriodH4, at(20, 10), true},
		{CheckAtOpen, PeriodH4, at(21, 10), false},
	}
	for _, c := range cases {
		require.Equal(t, c.want, EMAWindow(c.check, c.period, c.now), "%s %s %s", c.check, c.period, c.now.Format("15:04"))
	}
}

func TestCloseReason(t *testing.T) {
	up := Reading{EMA: 101, Smooth: 100, OK: true}  // EMA above Smooth by 1%
	down := Reading{EMA: 99, Smooth: 100, OK: true} // EMA below by ~1%
	cases := []struct {
		name       string
		side       string
		price      string
		sl, target *money.Dec
		r          Reading
		window     bool
		want       string
	}{
		{"long stop loss", SideLong, "91", dp("92"), nil, up, true, ReasonSL},
		{"long stop loss beats the cross", SideLong, "91", dp("92"), nil, down, true, ReasonSL},
		{"long target", SideLong, "120", dp("92"), dp("110"), up, true, ReasonTarget},
		{"long at the target price stays", SideLong, "110", nil, dp("110"), up, true, ""},
		{"long crossed back down", SideLong, "100", nil, nil, down, true, ReasonEMAReverse},
		{"long crossed back outside the window", SideLong, "100", nil, nil, down, false, ""},
		{"long still up", SideLong, "100", nil, nil, up, true, ""},
		{"short stop loss", SideShort, "109", dp("108"), nil, down, true, ReasonSL},
		{"short target", SideShort, "80", nil, dp("85"), down, true, ReasonTarget},
		{"short crossed back up", SideShort, "100", nil, nil, up, true, ReasonEMAReverse},
		{"no reading", SideShort, "100", nil, nil, Reading{}, true, ""},
	}
	for _, c := range cases {
		got := CloseReason(c.side, money.MustParse(c.price), c.sl, c.target, c.r, 0.03, c.window)
		require.Equal(t, c.want, got, c.name)
	}
}

func TestEntrySide(t *testing.T) {
	up := Reading{EMA: 100.5, Smooth: 100, OK: true}
	down := Reading{EMA: 99.5, Smooth: 100, OK: true}
	flat := Reading{EMA: 100.01, Smooth: 100, OK: true} // 0.01% < 0.03%
	cases := []struct {
		name, tradeType string
		price           float64
		r               Reading
		window          bool
		want            string
	}{
		{"both up", Both, 101, up, true, SideLong},
		{"both down", Both, 99, down, true, SideShort},
		{"only long refuses down", OnlyLong, 99, down, true, ""},
		{"only short refuses up", OnlyShort, 101, up, true, ""},
		{"below the threshold", Both, 100, flat, true, ""},
		{"price too far from EMA (4%)", Both, 104.6, up, true, ""},
		{"outside the window", Both, 101, up, false, ""},
	}
	for _, c := range cases {
		require.Equal(t, c.want, EntrySide(c.tradeType, c.price, c.r, 0.03, 4, c.window), c.name)
	}
}

func TestShouldOpen(t *testing.T) {
	up := Reading{EMA: 100.5, Smooth: 100, OK: true}
	require.True(t, ShouldOpen(SideLong, dp("50"), money.MustParse("49"), Reading{}, 0.03, 4, false), "long entry price passed")
	require.False(t, ShouldOpen(SideLong, dp("50"), money.MustParse("51"), up, 0.03, 4, true), "long entry price not reached")
	require.True(t, ShouldOpen(SideShort, dp("50"), money.MustParse("51"), Reading{}, 0.03, 4, false), "short entry price passed")
	require.True(t, ShouldOpen(SideLong, nil, money.MustParse("101"), up, 0.03, 4, true), "long on the cross")
	require.False(t, ShouldOpen(SideLong, nil, money.MustParse("105"), up, 0.03, 4, true), "long chasing")
	require.False(t, ShouldOpen(SideShort, nil, money.MustParse("99"), up, 0.03, 4, true), "short against the cross")
	require.False(t, ShouldOpen(SideLong, nil, money.MustParse("101"), up, 0.03, 4, false), "outside the window")
}

func TestSettingHitAndStopLoss(t *testing.T) {
	require.Equal(t, ReasonSettingTP, SettingHit(money.FromInt(10), money.Zero, dp("10")))
	require.Equal(t, ReasonSettingSL, SettingHit(money.Zero, money.FromInt(5), dp("-5.1")))
	require.Equal(t, "", SettingHit(money.Zero, money.Zero, dp("50")))
	require.Equal(t, "", SettingHit(money.FromInt(10), money.FromInt(5), nil))

	require.Equal(t, "92", StopLossPrice(SideLong, money.FromInt(100), money.FromInt(8)).String())
	require.Equal(t, "108", StopLossPrice(SideShort, money.FromInt(100), money.FromInt(8)).String())
	require.Equal(t, "-10.1", Unrealized(SideShort, money.FromInt(100), money.FromInt(110), money.FromInt(1), money.MustParse("0.1")).String())
}

func TestReadingFrom(t *testing.T) {
	closes := make([]float64, 60)
	for i := range closes {
		closes[i] = 100 + float64(i)
	}
	cur, cl := ReadingFrom(closes, CheckAtCurrent), ReadingFrom(closes, CheckAtClose)
	require.True(t, cur.OK && cl.OK)
	require.Greater(t, cur.EMA, cl.EMA, "the running candle moves EMA14 on a rising market")
	require.False(t, ReadingFrom(closes[:10], CheckAtOpen).OK)
}
