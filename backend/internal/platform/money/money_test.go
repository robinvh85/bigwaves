package money_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/platform/money"
)

func TestFloorAndCeilToStep(t *testing.T) {
	cases := []struct{ v, step, floor, ceil string }{
		{"1.23456", "0.01", "1.23", "1.24"},
		{"1234.9", "1", "1234", "1235"},
		{"0.000123456", "0.00001", "0.00012", "0.00013"},
		{"5", "0.1", "5", "5"},
	}
	for _, c := range cases {
		v, step := money.MustParse(c.v), money.MustParse(c.step)
		require.Equal(t, c.floor, money.FloorToStep(v, step).String(), "floor %s/%s", c.v, c.step)
		require.Equal(t, c.ceil, money.CeilToStep(v, step).String(), "ceil %s/%s", c.v, c.step)
	}
}

func TestFallbackTiersWithoutStep(t *testing.T) {
	require.Equal(t, "1234", money.FloorToStep(money.MustParse("1234.9"), money.Zero).String())
	require.Equal(t, "3.4", money.FloorToStep(money.MustParse("3.456"), money.Zero).String())
	require.Equal(t, "3.5", money.CeilToStep(money.MustParse("3.456"), money.Zero).String())
	require.Equal(t, "0.00123", money.FloorAmount(money.MustParse("0.00123456")).String())
}

func TestRoundPrice(t *testing.T) {
	require.Equal(t, "64123", money.RoundPrice(money.MustParse("64123.98")).String())
	require.Equal(t, "12.3", money.RoundPrice(money.MustParse("12.345")).String())
	require.Equal(t, "0.0000123", money.RoundPrice(money.MustParse("0.000012345")).String())
}

func TestJSONIsAString(t *testing.T) {
	b, err := json.Marshal(struct{ P money.Dec }{money.MustParse("0.10000000")})
	require.NoError(t, err)
	require.JSONEq(t, `{"P":"0.1"}`, string(b))

	var out struct{ P money.Dec }
	require.NoError(t, json.Unmarshal([]byte(`{"P":"12.5"}`), &out))
	require.Equal(t, "12.5", out.P.String())
	require.NoError(t, json.Unmarshal([]byte(`{"P":12.5}`), &out), "numbers are accepted too")
}

func TestScan(t *testing.T) {
	var d money.Dec
	require.NoError(t, d.Scan("0.00000001"))
	require.Equal(t, "0.00000001", d.String())
	require.NoError(t, d.Scan(int64(5)))
	require.Equal(t, "5", d.String())
	require.Error(t, d.Scan(nil))
}
