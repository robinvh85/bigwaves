package engine

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Number formats of the Python scripts, so the report and the sync string read exactly as before.

// Repr is Python repr(float): shortest round-trip digits, exponent form below 1e-4 or from 1e16,
// "29301.0" for integers.
func Repr(x float64) string {
	if math.IsNaN(x) {
		return ""
	}
	if math.IsInf(x, 0) {
		if x > 0 {
			return "inf"
		}
		return "-inf"
	}
	if x == 0 {
		if math.Signbit(x) {
			return "-0.0"
		}
		return "0.0"
	}
	// "d.dddde±XX" with the shortest digits.
	e := strconv.FormatFloat(math.Abs(x), 'e', -1, 64)
	mant, expStr, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expStr)
	digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	if digits == "" {
		digits = "0"
	}
	var s string
	switch {
	case exp < -4 || exp >= 16:
		m := digits
		if len(digits) > 1 {
			m = digits[:1] + "." + digits[1:]
		}
		sign := "+"
		if exp < 0 {
			sign = "-"
		}
		s = fmt.Sprintf("%se%s%02d", m, sign, abs(exp))
	case exp < 0:
		s = "0." + strings.Repeat("0", -exp-1) + digits
	case len(digits) <= exp+1:
		s = digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
	default:
		s = digits[:exp+1] + "." + digits[exp+1:]
	}
	if x < 0 {
		return "-" + s
	}
	return s
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// Pct1 is Python f"{x:+.1%}".
func Pct1(x float64) string { return fmt.Sprintf("%+.1f%%", x*100) }

// G is Python/C "%.{precision}g".
func G(x float64, precision int) string { return fmt.Sprintf("%.*g", precision, x) }
