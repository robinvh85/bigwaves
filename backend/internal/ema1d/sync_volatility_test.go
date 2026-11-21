package ema1d_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/platform/db/dbq"
)

// The LONG volatility filter of the sync (Config.MaxLongVolPct, 6% by default).

// candles stores the 31 closed 1D candles before entry so that the 30 daily log returns alternate
// +vol / -vol: their stdev is exactly vol. skipLast leaves out that many of the newest candles.
func (s *syncEnv) candles(p dbq.Ema1dPair, entry string, vol float64, skipLast int) {
	s.t.Helper()
	t0, err := time.Parse("2006-01-02", entry)
	require.NoError(s.t, err)
	px := 100.0
	for i := 31; i >= 1+skipLast; i-- {
		require.NoError(s.t, s.q().UpsertCandle(s.ctx, dbq.UpsertCandleParams{
			PairID: p.PairID, Interval: "1D", OpenTime: t0.AddDate(0, 0, -i).Unix(), Open: px, High: px, Low: px, Close: px,
		}))
		if i%2 == 0 {
			px *= math.Exp(vol)
		} else {
			px *= math.Exp(-vol)
		}
	}
}

// volWarnings keeps the warnings of the volatility filter (a sync in tests always warns that
// EMA1D is off).
func volWarnings(res ema1d.SyncResult) []string {
	out := []string{}
	for _, w := range res.Warnings {
		if strings.HasPrefix(w, "long_vol") {
			out = append(out, w)
		}
	}
	return out
}

func TestSyncSkipsLongWhenThirtyDayVolatilityIsHigh(t *testing.T) {
	s := newSyncEnv(t)
	s.candles(s.eth, "2026-10-10", 0.07, 0)
	s.candles(s.sun, "2026-10-10", 0.05, 0)

	res := s.sync("SUN:L:2026-10-10:0.02:XO,ETH:L:2026-10-10:2400:XO,BTC:F:-:1:-")

	require.Equal(t, []tk{{"SUN", "OPEN", "LONG"}}, s.todoKeys(), "5% is below the 6% default, 7% is not")
	require.Equal(t, map[string]string{"ETH": "long_vol_too_high (30d 7.0% >= 6%)"}, reasons(res))
	require.Empty(t, volWarnings(res))

	// The same signal a day later is still skipped: the volatility is the one at the entry candle.
	s.now = s.now.Add(24 * time.Hour)
	res, err := s.s.Sync(s.ctx, ema1d.SyncRequest{Date: "2026-10-11", RunID: "next-day", S: "SUN:L:2026-10-10:0.02:XO,ETH:L:2026-10-10:2400:XO,BTC:F:-:1:-"}, "")
	require.NoError(t, err)
	require.Equal(t, "long_vol_too_high (30d 7.0% >= 6%)", reasons(res)["ETH"])
	require.Len(t, s.todos(), 1)
}

func TestSyncVolatilityFilterDoesNotTouchShortsOrCloses(t *testing.T) {
	s := newSyncEnv(t)
	s.candles(s.sun, "2026-10-10", 0.09, 0)
	s.candles(s.eth, "2026-10-10", 0.09, 0)
	s.position(s.eth, ema1d.SideShort, "2026-10-03", "1", "2400")

	res := s.sync("SUN:S:2026-10-10:0.02:XO,ETH:L:2026-10-10:2400:XO,BTC:F:-:1:-")

	require.Equal(t, []tk{{"SUN", "OPEN", "SHORT"}, {"ETH", "CLOSE", "SHORT"}}, s.todoKeys(),
		"the short opens and the reversal still closes; only the LONG entry is skipped")
	require.Equal(t, map[string]string{"ETH": "long_vol_too_high (30d 9.0% >= 6%)"}, reasons(res))
}

func TestSyncVolatilityFilterFollowsTheSetting(t *testing.T) {
	s := newSyncEnv(t)
	s.candles(s.eth, "2026-10-10", 0.07, 0)
	s.candles(s.sun, "2026-10-10", 0.07, 0)

	s.config(func(c *ema1d.Config) { c.MaxLongVolPct = dec("7.5") })
	res := s.sync("SUN:F:-:1:-,ETH:L:2026-10-10:2400:XO,BTC:F:-:1:-")
	require.Empty(t, res.Skipped, "7% is below 7.5%")

	s.config(func(c *ema1d.Config) { c.MaxLongVolPct = dec("0") })
	res = s.sync("SUN:L:2026-10-10:0.02:XO,ETH:L:2026-10-10:2400:XO,BTC:F:-:1:-")
	require.Empty(t, res.Skipped, "0 turns the filter off")
	require.Empty(t, volWarnings(res))
	require.Equal(t, []tk{{"ETH", "OPEN", "LONG"}, {"SUN", "OPEN", "LONG"}}, s.todoKeys())
}

func TestSyncVolatilityUnknownOpensWithWarning(t *testing.T) {
	s := newSyncEnv(t)
	s.candles(s.eth, "2026-10-10", 0.09, 1) // yesterday's candle is missing: the candles are stale

	res := s.sync("SUN:L:2026-10-10:0.02:XO,ETH:L:2026-10-10:2400:XO,BTC:F:-:1:-")

	require.Equal(t, []tk{{"SUN", "OPEN", "LONG"}, {"ETH", "OPEN", "LONG"}}, s.todoKeys())
	require.Empty(t, res.Skipped)
	require.Equal(t, []string{
		"long_vol_unknown: SUN 2026-10-10 (1D candles before the entry are missing, filter not applied)",
		"long_vol_unknown: ETH 2026-10-10 (1D candles before the entry are missing, filter not applied)",
	}, volWarnings(res))
}

func TestConfigRejectsVolatilityLimitOutOfRange(t *testing.T) {
	c := ema1d.DefaultConfig()
	require.Equal(t, "6", c.MaxLongVolPct.String())
	require.NoError(t, c.Validate())
	c.MaxLongVolPct = dec("-1")
	require.ErrorContains(t, c.Validate(), "maxLongVolPct")
	c.MaxLongVolPct = dec("101")
	require.ErrorContains(t, c.Validate(), "maxLongVolPct")
}
