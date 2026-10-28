package emacross

// Periods of the two averages, as in the Rails EMA trading: EMA14 of the close and Smooth10 =
// EMA10 of that EMA.
const (
	EMAPeriod    = 14
	SmoothPeriod = 10
)

const (
	TrendUp   = "UP"
	TrendDown = "DOWN"
)

// Reading is where EMA14 and Smooth10 stand on the running candle.
type Reading struct {
	EMA, Smooth float64
	// Trend is UP when EMA14 >= Smooth10.
	Trend string
	// GapPercent is (EMA14 - Smooth10) / Smooth10 * 100: positive above, negative below.
	GapPercent float64
	// Cross is UP or DOWN when EMA14 crossed Smooth10 since the last closed candle by at least the
	// threshold, "" otherwise.
	Cross string
}

// ema is the recursive EMA ema = (x - prev) * 2 / (n + 1) + prev, seeded with the first value.
// Klines give a few hundred candles, enough for the seed not to matter.
func ema(xs []float64, n int) []float64 {
	out := make([]float64, len(xs))
	k := 2 / float64(n+1)
	for i, x := range xs {
		if i == 0 {
			out[i] = x
			continue
		}
		out[i] = (x-out[i-1])*k + out[i-1]
	}
	return out
}

// Lines returns EMA14 of closes and Smooth10 (EMA10 of that EMA), one value per close.
func Lines(closes []float64) (emaLine, smooth []float64) {
	e := ema(closes, EMAPeriod)
	return e, ema(e, SmoothPeriod)
}

// Evaluate reads closes, oldest first, where the last one is the running candle (its close is the
// current price). The cross test is the Rails one (CheckCross{H1,H4,D1}EmasForPairsService): the
// trend of the last closed candle, then how far the current EMA14 has gone past Smooth10 the other
// way, in percent of the lower line. ok is false with fewer than 2 closes.
func Evaluate(closes []float64, thresholdPercent float64) (r Reading, ok bool) {
	n := len(closes)
	if n < 2 {
		return Reading{}, false
	}
	e, s := Lines(closes)
	prevUp := e[n-2] >= s[n-2]
	r = Reading{EMA: e[n-1], Smooth: s[n-1], Trend: TrendDown}
	if r.EMA >= r.Smooth {
		r.Trend = TrendUp
	}
	if r.Smooth != 0 {
		r.GapPercent = (r.EMA - r.Smooth) / r.Smooth * 100
	}
	var past float64
	if prevUp {
		if r.EMA != 0 {
			past = (r.Smooth - r.EMA) / r.EMA * 100
		}
	} else if r.Smooth != 0 {
		past = (r.EMA - r.Smooth) / r.Smooth * 100
	}
	if past > 0 && past >= thresholdPercent {
		r.Cross = TrendUp
		if prevUp {
			r.Cross = TrendDown
		}
	}
	return r, true
}
