package engine

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func loadCSV(t *testing.T, coin, interval string) []Candle {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", coin+"_"+interval+".csv"))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	seen := map[string]bool{}
	var out []Candle
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(strings.TrimSpace(sc.Text()), ",")
		if len(parts) < 5 || parts[0] == "" || seen[parts[0]] {
			continue
		}
		seen[parts[0]] = true
		ts, err := strconv.ParseInt(parts[0], 10, 64)
		require.NoError(t, err)
		var px [4]float64
		for i := range px {
			px[i], err = strconv.ParseFloat(parts[i+1], 64)
			require.NoError(t, err)
		}
		out = append(out, Candle{Time: ts, Open: px[0], High: px[1], Low: px[2], Close: px[3]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out
}

func before(rows []Candle, ts int64) []Candle {
	var out []Candle
	for _, r := range rows {
		if r.Time < ts {
			out = append(out, r)
		}
	}
	return out
}

// The golden test: rows computed by status.py (Python) on the same CSV files, on days with and
// without actions, ETH with LONG DIV4H + SHORT XO and BTC long only on XO.
func TestStatusRowsMatchPython(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "expected_status.json"))
	require.NoError(t, err)
	var expected map[string][]map[string]any
	require.NoError(t, json.Unmarshal(raw, &expected))
	require.GreaterOrEqual(t, len(expected), 5)

	config := map[string]PairInput{
		"ETH": {Coin: "ETH", LongExit: RuleDIV4H},
		"BTC": {Coin: "BTC", LongExit: RuleXO, LongOnly: true},
	}
	candles := map[string][]Candle{}
	for _, coin := range []string{"eth", "btc"} {
		candles[coin+"1D"] = loadCSV(t, coin, "1D")
		candles[coin+"4H"] = loadCSV(t, coin, "4H")
	}

	checked := 0
	for day, rows := range expected {
		d, err := time.Parse("2006-01-02", day)
		require.NoError(t, err)
		cut := d.Unix() + Day
		for _, want := range rows {
			coin := want["pair"].(string)
			lc := strings.ToLower(coin)
			got, err := BuildRow(config[coin], before(candles[lc+"1D"], cut), before(candles[lc+"4H"], cut))
			require.NoError(t, err)
			b, _ := json.Marshal(got)
			var gotMap map[string]any
			require.NoError(t, json.Unmarshal(b, &gotMap))
			for _, col := range []string{"pair", "ngay", "gia", "ema_open", "cat_hom_nay", "lenh", "vao", "gia_vao", "lai", "thoat_theo", "hanh_dong_hom_nay", "nhom", "ghi_chu"} {
				require.Equal(t, want[col], gotMap[col], "%s %s %s", day, coin, col)
			}
			checked++
		}
	}
	require.Equal(t, 28, checked)
}

func TestEWMSeedsWithFirstValueAndSkipsLeadingNaN(t *testing.T) {
	out := EWM([]float64{math.NaN(), 1, 3}, 0.5)
	require.True(t, math.IsNaN(out[0]))
	require.Equal(t, []float64{1, 2}, out[1:])
}

func TestRSIIs100WhenPriceOnlyGoesUp(t *testing.T) {
	r := RSI([]float64{1, 2, 3, 4}, 14)
	require.InDelta(t, 100.0, r[len(r)-1], 1e-9)
}

func TestCrossUpEntersAndCrossDownExits(t *testing.T) {
	var opens []float64
	for i := 0; i < 60; i++ {
		opens = append(opens, 100-float64(i))
	}
	for i := 0; i < 20; i++ {
		opens = append(opens, 41+float64(i*5))
	}
	for i := 0; i < 30; i++ {
		opens = append(opens, 136-float64(i*6))
	}
	rows := make([]Candle, len(opens))
	for i, o := range opens {
		rows[i] = Candle{Time: int64(i) * Day, Open: o, High: o + 1, Low: o - 1, Close: o}
	}
	d1 := NewSeries(rows)
	tr := Trades(d1, nil, 1)[0]
	entry := int(tr.Entry / Day)
	require.True(t, d1.UpO[entry])
	require.False(t, d1.UpO[entry-1])
	require.Equal(t, opens[entry], tr.EntryPx)
	require.False(t, tr.XO.Open)
	require.False(t, d1.UpO[entry+tr.XO.Days])
	require.InDelta(t, opens[entry+tr.XO.Days]/opens[entry]-1-2*Fee, tr.XO.Ret, 1e-12)
	require.Nil(t, tr.DIV4H, "no 4H data")
}

func TestPyFormat(t *testing.T) {
	require.Equal(t, "4.56e-06", Repr(4.56e-06))
	require.Equal(t, "1e-05", Repr(1e-05))
	require.Equal(t, "0.0001", Repr(0.0001))
	require.Equal(t, "0.5876", Repr(0.5876))
	require.Equal(t, "29301.0", Repr(29301.0))
	require.Equal(t, "1e+16", Repr(1e16))
	require.Equal(t, "+12.6%", Pct1(0.12564))
	require.Equal(t, "-0.0%", Pct1(-0.0004))
	require.Equal(t, "2.521e-05", G(2.521e-05, 6))
	require.Equal(t, "111234.56", G(111234.56, 10))
	require.Equal(t, "106947", G(106947.06, 6))
	require.Equal(t, "1e+06", G(1e6, 6))
}

func TestSyncParams(t *testing.T) {
	short, flat := "2026-10-20", ""
	_ = flat
	rows := []Row{
		{Pair: "ETH", Ngay: "2026-10-24", Lenh: LenhShort, Vao: &short, ThoatTheo: RuleXO},
		{Pair: "BTC", Ngay: "2026-10-24", Lenh: LenhFlat, ThoatTheo: RuleXO},
	}
	opens := map[string]float64{"ETH|2026-10-20": 2410.5, "BTC|2026-10-24": 111234.56}
	openOn := func(coin, day string) (float64, bool) { v, ok := opens[coin+"|"+day]; return v, ok }

	got := BuildSyncParams(rows, "2026-10-24", openOn)
	require.Equal(t, "ETH:S:2026-10-20:2410.5:XO,BTC:F:-:111234.56:XO", got.S)
	require.Empty(t, got.Warnings)

	stale := BuildSyncParams(rows, "2026-10-25", func(string, string) (float64, bool) { return 1, true })
	require.Contains(t, stale.Warnings[0], "khác hôm nay")
}
