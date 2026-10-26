// Package engine is the EMA1D signal engine, ported from tradingview-ema/ema-1d-exit (engine.py,
// status.py, build_sync.py) through the Rails port (Ema1d::Strategy, Ema1d::SignalRows). It is pure
// float64 math with no I/O, checked against expected_status.json produced by the Python script.
//
// Keep the order of every floating-point operation as in Python: the EMA has no warm-up cut, so a
// different rounding on one candle can move a cross by a day. Products that are added to something
// are wrapped in float64(...) on purpose: Go may otherwise fuse a*b+c into one FMA instruction
// (it does on arm64), which rounds differently from pandas.
//
// Rules (09/10/2026):
//   - EMA14 of the OPEN price and Smooth10 = EMA10 of that EMA, 1D candles at 00:00 UTC.
//   - LONG enters at the open of the candle where EMA14(open) crosses above Smooth10; SHORT on the
//     cross below.
//   - XO exit: the opposite cross, at the open of that candle.
//   - DIV4H exit (long): the 2nd 1D fractal high (2 left / 1 right) whose 4H window has a bearish
//     RSI14 divergence (4H fractal 2/2 higher than the previous 4H fractal high within 30 candles,
//     RSI lower), confirmed within the last 18 4H candles: sell at the open of the next 1D candle.
//     Falls back to XO.
//   - Fee + slippage 0.06% per side. One trade per pair at a time. No stop loss.
package engine

import (
	"fmt"
	"math"
	"sort"
)

const (
	Fee  = 0.0006
	Day  = 86_400
	H4   = 4 * 3600
	Warm = 40
)

// Exit rules.
const (
	RuleXO    = "XO"
	RuleDIV4H = "DIV4H"
)

// Candle is one row: open time in unix seconds and OHLC prices.
type Candle struct {
	Time                   int64
	Open, High, Low, Close float64
}

// Series is one timeframe with its indicators.
type Series struct {
	Time  []int64
	Open  []float64
	High  []float64
	Low   []float64
	Close []float64
	UpO   []bool // EMA14(open) > Smooth10
	RSI   []float64
}

func NewSeries(rows []Candle) *Series {
	n := len(rows)
	s := &Series{
		Time: make([]int64, n), Open: make([]float64, n), High: make([]float64, n),
		Low: make([]float64, n), Close: make([]float64, n), UpO: make([]bool, n),
	}
	for i, r := range rows {
		s.Time[i], s.Open[i], s.High[i], s.Low[i], s.Close[i] = r.Time, r.Open, r.High, r.Low, r.Close
	}
	e := EMA(s.Open, 14)
	sm := EMA(e, 10)
	for i := range e {
		s.UpO[i] = e[i] > sm[i]
	}
	s.RSI = RSI(s.Close, 14)
	return s
}

func (s *Series) Len() int { return len(s.Time) }

// EWM is pandas ewm(alpha, adjust=False): leading NaN stay NaN, the first value seeds the average.
func EWM(x []float64, alpha float64) []float64 {
	out := make([]float64, len(x))
	prev := math.NaN()
	for i, v := range x {
		if math.IsNaN(v) {
			out[i] = prev
			continue
		}
		if math.IsNaN(prev) {
			prev = v
		} else {
			prev = float64((1-alpha)*prev) + float64(alpha*v)
		}
		out[i] = prev
	}
	return out
}

func EMA(x []float64, n int) []float64 { return EWM(x, 2.0/float64(n+1)) }

func RSI(close []float64, n int) []float64 {
	d := make([]float64, len(close))
	for i := range close {
		if i == 0 {
			d[i] = math.NaN()
		} else {
			d[i] = close[i] - close[i-1] //nolint:gosec // i >= 1 here
		}
	}
	upIn := make([]float64, len(d))
	dnIn := make([]float64, len(d))
	for i, v := range d {
		if math.IsNaN(v) {
			upIn[i], dnIn[i] = v, v
			continue
		}
		upIn[i] = math.Max(v, 0)
		dnIn[i] = -math.Min(v, 0)
	}
	alpha := 1.0 / float64(n)
	up := EWM(upIn, alpha)
	dn := EWM(dnIn, alpha)
	out := make([]float64, len(d))
	for i := range up {
		out[i] = 100 - 100/(1+safeDiv(up[i], dn[i]))
	}
	return out
}

// safeDiv is numpy float division: x/0 is ±inf (nan for 0/0) instead of an error.
func safeDiv(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	if b != 0 {
		return a / b
	}
	if a == 0 {
		return math.NaN()
	}
	if a > 0 {
		return math.Inf(1)
	}
	return math.Inf(-1)
}

// pyMax is Python's builtin max(a, b): keeps a unless b > a (so a NaN on the left wins).
func pyMax(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

func maxOf(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func fractalHighs(h []float64, left, right int) []bool {
	n := len(h)
	out := make([]bool, n)
	for p := left; p < n-right; p++ {
		if h[p] > maxOf(h[p-left:p]) && h[p] >= maxOf(h[p+1:p+right+1]) {
			out[p] = true
		}
	}
	return out
}

func neg(xs []float64) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = -x
	}
	return out
}

// h4DivFlags marks, per 4H candle, whether an RSI divergence was confirmed within the last window
// candles. side 1: bearish (higher high, lower RSI); side -1 mirrors on -low and 100 - RSI.
func h4DivFlags(f4 *Series, side, window int) []bool {
	h := f4.High
	r := f4.RSI
	if side != 1 {
		h = neg(f4.Low)
		r = make([]float64, len(f4.RSI))
		for i, v := range f4.RSI {
			r[i] = 100 - v
		}
	}
	fh := fractalHighs(h, 2, 2)
	n := f4.Len()
	flag := make([]bool, n)
	type peak struct {
		p    int
		h, r float64
	}
	var last *peak
	for p := 1; p < n; p++ {
		if !fh[p] {
			continue
		}
		rp := pyMax(r[p-1], r[p])
		if last != nil && p-last.p <= 30 && h[p] > last.h && rp < last.r && p+2 < n {
			flag[p+2] = true
		}
		last = &peak{p, h[p], rp}
	}
	out := make([]bool, n)
	lastTrue := -1
	for i := range flag {
		if flag[i] {
			lastTrue = i
		}
		out[i] = lastTrue >= 0 && i-lastTrue < window
	}
	return out
}

// Outcome is the result of one exit rule on a trade.
type Outcome struct {
	Ret  float64 // return after fees, e.g. 0.12 = +12%
	Days int     // candles from entry to exit (or to the last candle when still open)
	Open bool    // still open at the last candle
}

// Trade is one strategy trade of one side with the outcome of each exit rule.
type Trade struct {
	Entry   int64   // entry candle open time, unix seconds
	EntryPx float64 // open of the entry candle
	Side    string  // "L" or "S"
	XO      Outcome
	DIV4H   *Outcome // nil when the entry is before the 4H data (+10 days)
}

type exitAt struct {
	idx int
	px  float64
}

// Trades returns every trade of one side (engine.py trades()).
func Trades(d1, f4 *Series, side int) []Trade {
	const right = 1
	o, c, n := d1.Open, d1.Close, d1.Len()
	h := d1.High
	uo := d1.UpO
	if side != 1 {
		h = neg(d1.Low)
		uo = make([]bool, n)
		for i, v := range d1.UpO {
			uo[i] = !v
		}
	}
	fh := fractalHighs(h, 2, right)
	var ct4 []int64
	var dv4 []bool
	var start4 int64
	if f4 != nil {
		ct4 = make([]int64, f4.Len())
		for i, t := range f4.Time {
			ct4[i] = t + H4
		}
		dv4 = h4DivFlags(f4, side, 18)
		start4 = f4.Time[0] + 10*Day
	}

	var out []Trade
	for i := Warm; i < n; {
		if !uo[i] || uo[i-1] { // not a cross into the side

			i++
			continue
		}
		e := i
		ent := o[e]
		j := e + 1
		for j < n && uo[j] {
			j++
		}
		var xo *exitAt
		lastXO := n
		if j < n {
			xo = &exitAt{j, o[j]}
			lastXO = j
		}

		var div *exitAt
		divNA := f4 == nil || d1.Time[e] < start4
		if !divNA {
			k4 := 0
			for t := e + right; t < min(lastXO, n-1); t++ {
				p := t - right
				if p >= e && fh[p] {
					jj := upperBound(ct4, d1.Time[t]+Day) - 1
					if jj >= 0 && dv4[jj] {
						k4++
						if k4 == 2 {
							div = &exitAt{t + 1, o[t+1]}
							break
						}
					}
				}
			}
			if div == nil {
				div = xo
			}
		}

		sideName := "L"
		if side != 1 {
			sideName = "S"
		}
		tr := Trade{Entry: d1.Time[e], EntryPx: ent, Side: sideName, XO: outcome(xo, side, ent, c, n, e)}
		if !divNA {
			oc := outcome(div, side, ent, c, n, e)
			tr.DIV4H = &oc
		}
		out = append(out, tr)
		if xo != nil {
			i = xo.idx
		} else {
			i = n
		}
	}
	return out
}

func outcome(exit *exitAt, side int, ent float64, close []float64, n, e int) Outcome {
	s := float64(side)
	if exit == nil {
		return Outcome{Ret: float64(s*(close[n-1]/ent-1)) - 2*Fee, Days: n - 1 - e, Open: true}
	}
	return Outcome{Ret: float64(s*(exit.px/ent-1)) - 2*Fee, Days: exit.idx - e}
}

// upperBound is the number of elements <= v in a sorted slice (numpy searchsorted side="right").
func upperBound(arr []int64, v int64) int {
	return sort.Search(len(arr), func(i int) bool { return arr[i] > v })
}

// PairTrade is a trade with the configured exit rule applied.
type PairTrade struct {
	Entry   int64
	EntryPx float64
	Side    string // L or S
	Rule    string
	Ret     float64
	Days    int
	Open    bool
}

// ExitTime is the open time of the exit candle (or of the last candle when still open).
func (t PairTrade) ExitTime() int64 { return t.Entry + int64(t.Days)*Day }

// PairTrades returns both sides of a pair with the configured rule applied (status.py
// pair_trades), sorted by entry.
func PairTrades(d1, f4 *Series, longExit string, longOnly bool) ([]PairTrade, error) {
	type sideRule struct {
		side int
		rule string
	}
	sides := []sideRule{{1, longExit}}
	if !longOnly {
		sides = append(sides, sideRule{-1, RuleXO})
	}
	var all []PairTrade
	for _, sr := range sides {
		if sr.rule != RuleXO && sr.rule != RuleDIV4H {
			return nil, fmt.Errorf("unknown exit rule %q", sr.rule)
		}
		for _, t := range Trades(d1, f4, sr.side) {
			oc := t.XO
			if sr.rule == RuleDIV4H && t.DIV4H != nil { // DIV4H before 4H data: XO
				oc = *t.DIV4H
			}
			all = append(all, PairTrade{Entry: t.Entry, EntryPx: t.EntryPx, Side: t.Side, Rule: sr.rule, Ret: oc.Ret, Days: oc.Days, Open: oc.Open})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Entry < all[j].Entry })
	return all, nil
}
