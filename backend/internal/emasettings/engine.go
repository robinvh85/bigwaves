package emasettings

import (
	"math"
	"time"

	"bigwaves/internal/emacross"
	"bigwaves/internal/platform/money"
)

// The decisions of one scan, free of I/O so each rule is tested on its own. They follow the Rails
// EmaSettings::CheckTradesService phase by phase; comments name the phase.

// minCandles below which a pair (newly listed) has no reading.
const minCandles = 3 * emacross.EMAPeriod

// Reading is EMA14 and Smooth10 as the setting's check type sees them. OK is false when there were
// too few candles.
type Reading struct {
	EMA, Smooth float64
	OK          bool
}

// ReadingFrom takes the closes of the period, oldest first, the last one being the running
// candle (its close is the current price). CHECK_AT_CLOSE reads the averages at the close of the
// last closed candle; CHECK_AT_OPEN and CHECK_AT_CURRENT put the current price on top of it.
func ReadingFrom(closes []float64, checkType string) Reading {
	n := len(closes)
	if n < minCandles {
		return Reading{}
	}
	e, s := emacross.Lines(closes)
	i := n - 1
	if checkType == CheckAtClose {
		i = n - 2
	}
	return Reading{EMA: e[i], Smooth: s[i], OK: true}
}

// over is how far a is above b, in percent of b.
func over(a, b float64) float64 {
	if b == 0 {
		return math.Inf(-1)
	}
	return (a - b) / b * 100
}

// EMAWindow reports whether the EMA rules may run now: always for CHECK_AT_CURRENT, otherwise only
// in the first 15 minutes of a new candle (D1: 00:00 UTC, H4: every 4 hours). Stop loss and
// target are checked on every scan.
func EMAWindow(checkType, period string, now time.Time) bool {
	if checkType == CheckAtCurrent {
		return true
	}
	now = now.UTC()
	if now.Minute() >= 15 {
		return false
	}
	if period == PeriodH4 {
		return now.Hour()%4 == 0
	}
	return now.Hour() == 0
}

// CloseReason is phase 1: why an OPEN trade must close now, "" to keep it. Stop loss first, then
// the trade's target, then EMA14 crossing back past Smooth10 by closePct (only inside the window).
func CloseReason(side string, price money.Dec, sl, target *money.Dec, r Reading, closePct float64, window bool) string {
	long := side == SideLong
	if sl != nil && sl.IsPositive() && ((long && price.LessThan(*sl)) || (!long && price.GreaterThan(*sl))) {
		return ReasonSL
	}
	if target != nil && target.IsPositive() && ((long && price.GreaterThan(*target)) || (!long && price.LessThan(*target))) {
		return ReasonTarget
	}
	if r.OK && window {
		back := over(r.EMA, r.Smooth) // short: EMA back above Smooth
		if long {
			back = over(r.Smooth, r.EMA) // long: EMA back below Smooth (in % of EMA, as Rails)
		}
		if back >= closePct {
			return ReasonEMAReverse
		}
	}
	return ""
}

// EntrySide is phase 2: the side of a new trade when the setting holds nothing, "" for none.
// EMA14 above Smooth10 by openPct means up (LONG), below means down (SHORT); the trade type
// filters it, and a price too far from EMA14 (ignorePct) is not chased.
func EntrySide(tradeType string, price float64, r Reading, openPct, ignorePct float64, window bool) string {
	if !r.OK || !window {
		return ""
	}
	side := ""
	switch {
	case over(r.EMA, r.Smooth) >= openPct:
		side = SideLong
	case over(r.Smooth, r.EMA) >= openPct:
		side = SideShort
	default:
		return ""
	}
	if math.Abs(price-r.EMA)/r.EMA*100 >= ignorePct {
		return ""
	}
	switch tradeType {
	case OnlyLong:
		if side != SideLong {
			return ""
		}
	case OnlyShort:
		if side != SideShort {
			return ""
		}
	}
	return side
}

// ShouldOpen is phase 3: whether an OPENING trade enters now. With an entry price it waits for the
// price to pass it (a long below, a short above, like a limit); without one it waits for EMA14 to
// be past Smooth10 on its side by openPct, the price not further than ignorePct beyond EMA14.
func ShouldOpen(side string, entry *money.Dec, price money.Dec, r Reading, openPct, ignorePct float64, window bool) bool {
	long := side == SideLong
	if entry != nil && entry.IsPositive() {
		if long {
			return price.LessThan(*entry)
		}
		return price.GreaterThan(*entry)
	}
	if !r.OK || !window {
		return false
	}
	p := toFloat(price)
	if long {
		return over(r.EMA, r.Smooth) >= openPct && over(p, r.EMA) < ignorePct
	}
	return over(r.Smooth, r.EMA) >= openPct && over(r.EMA, p) < ignorePct
}

// SettingHit is phase 5: SETTING_TP or SETTING_SL when the total PnL in % of the budget reached the
// setting's take profit or stop loss (0 = off), "" otherwise.
func SettingHit(tpPercent, slPercent money.Dec, totalPercent *money.Dec) string {
	if totalPercent == nil {
		return ""
	}
	if tpPercent.IsPositive() && totalPercent.GreaterOrEqual(tpPercent) {
		return ReasonSettingTP
	}
	if slPercent.IsPositive() && totalPercent.LessOrEqual(slPercent.Neg()) {
		return ReasonSettingSL
	}
	return ""
}

// StopLossPrice is the stop loss set on a trade at open: pct below the entry for a long, above
// for a short.
func StopLossPrice(side string, open, pct money.Dec) money.Dec {
	f := pct.Div(money.FromInt(100))
	if side == SideLong {
		return open.Mul(money.FromInt(1).Sub(f)).Round(8)
	}
	return open.Mul(money.FromInt(1).Add(f)).Round(8)
}

// Unrealized is the PnL of an OPEN trade at price, net of the open fee already paid.
func Unrealized(side string, open, price, amount, fee money.Dec) money.Dec {
	gross := price.Sub(open).Mul(amount)
	if side == SideShort {
		gross = gross.Neg()
	}
	return gross.Sub(fee).Round(8)
}

func toFloat(d money.Dec) float64 {
	f, _ := d.Float64()
	return f
}
