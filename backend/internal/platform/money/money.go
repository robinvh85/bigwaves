// Package money holds the decimal type used for every amount, price and quantity, and the
// rounding rules Binance imposes on order quantities.
package money

import (
	"database/sql/driver"
	"fmt"

	"github.com/shopspring/decimal"
)

// Dec is a decimal stored in SQLite as TEXT and serialized to JSON as a string.
type Dec struct {
	decimal.Decimal
}

var (
	Zero = Dec{decimal.Zero}
	One  = Dec{decimal.NewFromInt(1)}
)

func New(d decimal.Decimal) Dec { return Dec{d} }

func FromInt(i int64) Dec { return Dec{decimal.NewFromInt(i)} }

// Parse parses a decimal string such as "0.00012345" or "1e-8".
func Parse(s string) (Dec, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Zero, fmt.Errorf("parse decimal %q: %w", s, err)
	}
	return Dec{d}, nil
}

// MustParse is for constants and tests.
func MustParse(s string) Dec {
	d, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

// FromFloat converts a float that came from a numeric indicator (EMA, RSI). Never use it for money
// read from the exchange; parse the exchange's string instead.
func FromFloat(f float64) Dec { return Dec{decimal.NewFromFloat(f)} }

func (d Dec) Add(o Dec) Dec          { return Dec{d.Decimal.Add(o.Decimal)} }
func (d Dec) Sub(o Dec) Dec          { return Dec{d.Decimal.Sub(o.Decimal)} }
func (d Dec) Mul(o Dec) Dec          { return Dec{d.Decimal.Mul(o.Decimal)} }
func (d Dec) Neg() Dec               { return Dec{d.Decimal.Neg()} }
func (d Dec) Abs() Dec               { return Dec{d.Decimal.Abs()} }
func (d Dec) Round(places int32) Dec { return Dec{d.Decimal.Round(places)} }

// Div divides with 16 digits after the point, enough for prices like 0.00000123.
func (d Dec) Div(o Dec) Dec { return Dec{d.DivRound(o.Decimal, 16)} }

func (d Dec) Cmp(o Dec) int             { return d.Decimal.Cmp(o.Decimal) }
func (d Dec) Equal(o Dec) bool          { return d.Decimal.Equal(o.Decimal) }
func (d Dec) GreaterThan(o Dec) bool    { return d.Decimal.GreaterThan(o.Decimal) }
func (d Dec) GreaterOrEqual(o Dec) bool { return d.GreaterThanOrEqual(o.Decimal) }
func (d Dec) LessThan(o Dec) bool       { return d.Decimal.LessThan(o.Decimal) }
func (d Dec) LessOrEqual(o Dec) bool    { return d.LessThanOrEqual(o.Decimal) }

func Max(a, b Dec) Dec {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

func Min(a, b Dec) Dec {
	if a.LessThan(b) {
		return a
	}
	return b
}

func Sum(ds ...Dec) Dec {
	s := Zero
	for _, d := range ds {
		s = s.Add(d)
	}
	return s
}

// String renders without exponent and without trailing zeros: "0.0001", "12.5", "100".
func (d Dec) String() string { return d.Decimal.String() }

// Value stores the canonical string.
func (d Dec) Value() (driver.Value, error) { return d.String(), nil }

// Scan reads TEXT (the normal case) and tolerates INTEGER/REAL left by hand-written SQL.
func (d *Dec) Scan(src any) error {
	switch v := src.(type) {
	case string:
		p, err := Parse(v)
		*d = p
		return err
	case []byte:
		p, err := Parse(string(v))
		*d = p
		return err
	case int64:
		*d = FromInt(v)
		return nil
	case float64:
		*d = Dec{decimal.NewFromFloat(v)}
		return nil
	case nil:
		return fmt.Errorf("money.Dec: NULL into non-nullable column")
	default:
		return fmt.Errorf("money.Dec: unsupported type %T", src)
	}
}

// MarshalJSON writes a JSON string so no precision is lost in JavaScript.
func (d Dec) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }

func (d *Dec) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	p, err := Parse(s)
	if err != nil {
		return err
	}
	*d = p
	return nil
}

func (d Dec) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

func (d *Dec) UnmarshalText(b []byte) error {
	p, err := Parse(string(b))
	if err != nil {
		return err
	}
	*d = p
	return nil
}
