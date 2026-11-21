package engine

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLogReturnStdevOfAlternatingReturns(t *testing.T) {
	// 30 returns of +10% / -10% in log terms: mean 0, stdev exactly 0.1.
	closes := []float64{100}
	for i := 0; i < 30; i++ {
		step := 0.1
		if i%2 == 1 {
			step = -0.1
		}
		closes = append(closes, closes[len(closes)-1]*math.Exp(step))
	}
	v, ok := LogReturnStdev(closes, 30)
	require.True(t, ok)
	require.InDelta(t, 0.1, v, 1e-12)

	// Only the last n+1 closes count.
	v2, ok := LogReturnStdev(append([]float64{1, 500, 3}, closes...), 30)
	require.True(t, ok)
	require.InDelta(t, v, v2, 1e-12)
}

func TestLogReturnStdevNeedsEnoughPositiveCloses(t *testing.T) {
	_, ok := LogReturnStdev(make([]float64, 30), 30)
	require.False(t, ok, "30 closes give only 29 returns")
	closes := make([]float64, 31)
	for i := range closes {
		closes[i] = 1
	}
	v, ok := LogReturnStdev(closes, 30)
	require.True(t, ok)
	require.Zero(t, v)
	closes[10] = 0
	_, ok = LogReturnStdev(closes, 30)
	require.False(t, ok)
}

// The research computed it with numpy: np.std(np.diff(np.log(close[-31:]))) on the ETH candles
// before 2026-10-24.
func TestLogReturnStdevMatchesNumpy(t *testing.T) {
	before := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC).Unix()
	var closes []float64
	for _, c := range loadCSV(t, "eth", "1D") {
		if c.Time < before {
			closes = append(closes, c.Close)
		}
	}
	v, ok := LogReturnStdev(closes, 30)
	require.True(t, ok)
	require.InDelta(t, 0.016955900266762403, v, 1e-15)
}
