package engine

import "math"

// LogReturnStdev is the population standard deviation of the last n daily log returns of closes,
// e.g. 0.06 = the close moved about ±6% a day. It needs n+1 closes; ok is false with fewer or
// with a price that is not positive.
func LogReturnStdev(closes []float64, n int) (v float64, ok bool) {
	if n < 2 || len(closes) < n+1 {
		return 0, false
	}
	c := closes[len(closes)-n-1:]
	rets := make([]float64, n)
	sum := 0.0
	for i := 1; i <= n; i++ {
		if c[i] <= 0 || c[i-1] <= 0 {
			return 0, false
		}
		rets[i-1] = math.Log(c[i] / c[i-1])
		sum += rets[i-1]
	}
	mean := sum / float64(n)
	ss := 0.0
	for _, x := range rets {
		ss += (x - mean) * (x - mean)
	}
	return math.Sqrt(ss / float64(n)), true
}
