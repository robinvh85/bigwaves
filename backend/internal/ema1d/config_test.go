package ema1d_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
)

// Core and sensitive coins get the same size, with the same total as the earlier 7/3.5 and
// 2.5/1.25 over 15 + 5 coins.
func TestSuggestedSizesAreTheSameForBothGroups(t *testing.T) {
	s := ema1d.SuggestedSizes
	require.Equal(t, "5.875", s.CoreLong.String())
	require.Equal(t, "2.9375", s.CoreShort.String())
	require.True(t, s.SensitiveLong.Equal(s.CoreLong))
	require.True(t, s.SensitiveShort.Equal(s.CoreShort))
	require.Equal(t, "117.5", s.CoreLong.Mul(dec("20")).String(), "15 × 7 + 5 × 2.5")
	require.Equal(t, s, ema1d.DefaultConfig().Sizes)

	c := ema1d.DefaultConfig()
	for _, group := range []string{ema1d.GroupCore, ema1d.GroupSensitive} {
		long, suggested := c.SizeFor(group, ema1d.SideLong)
		require.Equal(t, "5.875", long.String(), group)
		require.True(t, suggested.Equal(long), group)
	}
}
