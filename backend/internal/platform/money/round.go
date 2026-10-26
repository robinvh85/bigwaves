package money

import "github.com/shopspring/decimal"

// FloorToStep rounds a quantity down to a multiple of the pair's LOT_SIZE step: Binance rejects any
// other quantity, and rounding down never asks for more than we hold. Without a step it falls back
// to FloorAmount.
func FloorToStep(v, step Dec) Dec {
	if !step.IsPositive() {
		return FloorAmount(v)
	}
	return Dec{v.Decimal.Div(step.Decimal).Floor().Mul(step.Decimal)}
}

// CeilToStep rounds up to a multiple of step, for quantities that must not fall below the source
// value (buying back enough coin to repay a debt).
func CeilToStep(v, step Dec) Dec {
	if !step.IsPositive() {
		return CeilAmount(v)
	}
	return Dec{v.Decimal.Div(step.Decimal).Ceil().Mul(step.Decimal)}
}

// amountPlaces is the precision tier used when a pair has no cached step size yet (ported from
// Utils::Number in the Rails app).
func amountPlaces(v Dec) int32 {
	tiers := []struct {
		above  string
		places int32
	}{
		{"30", 0}, {"3", 1}, {"0.3", 2}, {"0.03", 3}, {"0.01", 4},
		{"0.001", 5}, {"0.0001", 6}, {"0.00001", 7},
	}
	a := v.Abs()
	for _, t := range tiers {
		if a.GreaterThan(MustParse(t.above)) {
			return t.places
		}
	}
	return 8
}

func FloorAmount(v Dec) Dec {
	p := amountPlaces(v)
	return Dec{v.RoundFloor(p)}
}

func CeilAmount(v Dec) Dec {
	p := amountPlaces(v)
	return Dec{v.RoundCeil(p)}
}

// RoundPrice keeps a sensible number of decimals for display and storage of ticker prices.
func RoundPrice(v Dec) Dec {
	tiers := []struct {
		above  string
		places int32
	}{
		{"300", 0}, {"10", 1}, {"1", 2}, {"0.1", 3}, {"0.01", 4},
		{"0.001", 5}, {"0.0001", 6}, {"0.00001", 7},
	}
	a := v.Abs()
	for _, t := range tiers {
		if a.GreaterThan(MustParse(t.above)) {
			if t.places == 0 {
				return Dec{v.Truncate(0)}
			}
			return Dec{v.Decimal.Round(t.places)}
		}
	}
	return Dec{v.Decimal.Round(8)}
}

// Percent returns part/whole*100, or zero when whole is zero.
func Percent(part, whole Dec) Dec {
	if whole.IsZero() {
		return Zero
	}
	return Dec{part.Decimal.Mul(decimal.NewFromInt(100)).DivRound(whole.Decimal, 8)}
}
