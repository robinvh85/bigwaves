package ema1d_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
)

// One LONG and one SHORT size for every coin, with the same total as the earlier core 7 / 3.5
// and sensitive 2.5 / 1.25 over 15 + 5 coins.
func TestSuggestedSizesKeepTheEarlierTotal(t *testing.T) {
	s := ema1d.SuggestedSizes
	require.Equal(t, "5.875", s.Long.String())
	require.Equal(t, "2.9375", s.Short.String())
	require.Equal(t, "117.5", s.Long.Mul(dec("20")).String(), "15 × 7 + 5 × 2.5")
	require.Equal(t, "58.75", s.Short.Mul(dec("20")).String(), "15 × 3.5 + 5 × 1.25")
	require.Equal(t, s, ema1d.DefaultConfig().Sizes)

	long, suggested := ema1d.DefaultConfig().SizeFor(ema1d.SideLong)
	require.Equal(t, "5.875", long.String())
	require.True(t, suggested.Equal(long))
	short, _ := ema1d.DefaultConfig().SizeFor(ema1d.SideShort)
	require.Equal(t, "2.9375", short.String())
}

func TestSizesReadTheSettingsSavedPerGroup(t *testing.T) {
	load := func(js string) ema1d.Sizes {
		t.Helper()
		c := ema1d.DefaultConfig()
		require.NoError(t, json.Unmarshal([]byte(js), &c))
		return c.Sizes
	}
	// The settings production ran with: averaged over the 15 + 5 coins of that basket.
	s := load(`{"sizes":{"coreLong":"7","coreShort":"3.5","sensitiveLong":"2.5","sensitiveShort":"1.25"}}`)
	require.Equal(t, "5.875", s.Long.String())
	require.Equal(t, "2.9375", s.Short.String())

	// A small test-run size stays small.
	s = load(`{"sizes":{"coreLong":"0.4","coreShort":"0.2","sensitiveLong":"0.4","sensitiveShort":"0.2"}}`)
	require.Equal(t, "0.4", s.Long.String())
	require.Equal(t, "0.2", s.Short.String())

	// Only the core size known: that one.
	s = load(`{"sizes":{"coreLong":"5"}}`)
	require.Equal(t, "5", s.Long.String())
	require.Equal(t, "2.9375", s.Short.String(), "missing: the default stays")

	// The new keys win over the old ones.
	s = load(`{"sizes":{"long":"4","short":"2","coreLong":"7","sensitiveLong":"2.5"}}`)
	require.Equal(t, "4", s.Long.String())
	require.Equal(t, "2", s.Short.String())

	// Saved again, only the new keys remain.
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	require.JSONEq(t, `{"long":"4","short":"2"}`, string(raw))
}
