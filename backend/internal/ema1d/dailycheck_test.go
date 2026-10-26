package ema1d_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/ema1d/engine"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/notify"
)

// Ported from test/services/ema1d/daily_check_service_test.rb. Candles: the ETH / BTC files of
// tradingview-ema up to the 2026-10-24 candle. On that day the strategy holds ETH SHORT (entered
// before added_on, so the bot skips it) and BTC is flat.

func loadKlines(t *testing.T, coin, interval string) []binancefake.Kline {
	t.Helper()
	f, err := os.Open(filepath.Join("engine", "testdata", coin+"_"+interval+".csv"))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	seen := map[string]bool{}
	var out []binancefake.Kline
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Split(strings.TrimSpace(sc.Text()), ",")
		if len(p) < 5 || p[0] == "" || seen[p[0]] {
			continue
		}
		seen[p[0]] = true
		var ts int64
		require.NoError(t, json.Unmarshal([]byte(p[0]), &ts))
		out = append(out, binancefake.Kline{Time: ts, Open: p[1], High: p[2], Low: p[3], Close: p[4]})
	}
	return out
}

func TestDailyCheckOffDoesNothing(t *testing.T) {
	e := setupCheck(t)
	e.config(func(cfg *ema1d.Config) { cfg.DailyCheckEnabled = false })
	res, err := e.s.DailyCheck(e.ctx, ema1d.DailyCheckOptions{})
	require.NoError(t, err)
	require.Equal(t, "disabled", res.Status)
	require.Equal(t, 0, e.count("ema1d_daily_checks"))
}

func setupCheck(t *testing.T) *env {
	e := newEnv(t, time.Date(2026, 10, 24, 0, 3, 0, 0, time.UTC)) // 07:03 VN
	e.config(func(cfg *ema1d.Config) { cfg.DailyCheckEnabled = true; cfg.MaxEntryAgeDays = 1 })
	divRule, xo := "DIV4H", "XO"
	eth := e.pair("ETH", "2500", "0.0001")
	btc := e.pair("BTC", "110000", "0.00001", sensitive, longOnly)
	e.setPair(eth.ID, func(p *dbq.Ema1dPair) { p.AddedOn = "2026-10-10"; p.LongExitRule = &divRule })
	e.setPair(btc.ID, func(p *dbq.Ema1dPair) { p.AddedOn = "2026-10-10"; p.LongExitRule = &xo })
	for _, coin := range []string{"eth", "btc"} {
		sym := strings.ToUpper(coin) + "USDT"
		e.ex.SetKlines(sym, "1d", loadKlines(t, coin, "1D"))
		e.ex.SetKlines(sym, "4h", loadKlines(t, coin, "4H"))
	}
	return e
}

func reports(e *env) []string {
	var out []string
	for _, m := range e.k.Recorder.Messages {
		if m.Channel == notify.Reports {
			out = append(out, m.Msg)
		}
	}
	return out
}

func alerts(e *env) []string {
	var out []string
	for _, m := range e.k.Recorder.Messages {
		if m.Channel == notify.Orders {
			out = append(out, m.Msg)
		}
	}
	return out
}

func TestDailyCheckComputesSyncsOnceAndSendsOnce(t *testing.T) {
	e := setupCheck(t)
	res, err := e.s.DailyCheck(e.ctx, ema1d.DailyCheckOptions{})
	require.NoError(t, err)
	require.Equal(t, "sent", res.Status, res.Error)

	check, err := e.q().GetDailyCheck(e.ctx, "2026-10-24")
	require.NoError(t, err)
	require.Equal(t, ema1d.CheckSent, check.Status)
	require.Equal(t, "auto-20261024", check.RunID)
	require.Equal(t, 1, e.count("ema1d_syncs WHERE dry_run = 0 AND run_id = 'auto-20261024' AND signal_date = '2026-10-24'"))

	var rows []engine.Row
	require.NoError(t, json.Unmarshal(check.Rows, &rows))
	byPair := map[string]engine.Row{}
	for _, r := range rows {
		byPair[r.Pair] = r
	}
	require.Equal(t, "SHORT", byPair["ETH"].Lenh)
	require.Equal(t, "ĐỨNG NGOÀI", byPair["BTC"].Lenh)
	var params map[string]any
	require.NoError(t, json.Unmarshal(check.SyncParams, &params))
	require.Regexp(t, `^ETH:S:\d{4}-\d{2}-\d{2}:[\d.e-]+:XO,BTC:F:-:[\d.]+:XO$`, params["s"])

	sent := reports(e)
	require.Len(t, sent, 1)
	report := sent[0]
	t.Log(report)
	require.Contains(t, report, "**EMA 1D L+S — 24/10/2026** (bigwaves)")
	require.Contains(t, report, "**SHORT đang mở (1)**")
	require.Contains(t, report, "entry_before_added_on ETH")
	require.Equal(t, 0, e.count("ema1d_todos"))

	again, err := e.s.DailyCheck(e.ctx, ema1d.DailyCheckOptions{})
	require.NoError(t, err)
	require.Equal(t, "already_sent", again.Status)
	require.Len(t, reports(e), 1)
}

func TestDailyCheckFailedSendKeepsTheSyncAndRerunOnlyResends(t *testing.T) {
	e := setupCheck(t)
	e.s.ReportConfigured = false
	res, err := e.s.DailyCheck(e.ctx, ema1d.DailyCheckOptions{})
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	check, _ := e.q().GetDailyCheck(e.ctx, "2026-10-24")
	require.Equal(t, ema1d.CheckFailed, check.Status)
	require.NotNil(t, check.SyncedAt)
	require.Contains(t, check.LastError, "DISCORD_REPORT_WEBHOOK")
	require.Len(t, alerts(e), 1)
	require.Contains(t, alerts(e)[0], "đã sync, chưa gửi báo cáo")

	e.travel(time.Hour)
	e.s.ReportConfigured = true
	klinesBefore := e.ex.CallCount("GET /api/v3/klines")
	res, err = e.s.DailyCheck(e.ctx, ema1d.DailyCheckOptions{})
	require.NoError(t, err)
	require.Equal(t, "sent", res.Status)
	require.Equal(t, klinesBefore, e.ex.CallCount("GET /api/v3/klines"), "candles must not be fetched again")
	check, _ = e.q().GetDailyCheck(e.ctx, "2026-10-24")
	require.EqualValues(t, 2, check.Attempts)
	require.Equal(t, 1, e.count("ema1d_syncs WHERE dry_run = 0"))
}

func TestDailyCheckTodaysCandleMissing(t *testing.T) {
	e := setupCheck(t)
	e.now = time.Date(2026, 10, 25, 0, 3, 0, 0, time.UTC) // no 25/10 candle in the files

	res, err := e.s.DailyCheck(e.ctx, ema1d.DailyCheckOptions{})
	require.NoError(t, err)
	require.Equal(t, "failed", res.Status)
	check, _ := e.q().GetDailyCheck(e.ctx, "2026-10-25")
	require.Contains(t, check.LastError, "chưa sẵn sàng")
	require.Equal(t, 0, e.count("ema1d_syncs"))
	require.Empty(t, reports(e))
	require.Len(t, alerts(e), 1)
	require.Contains(t, alerts(e)[0], "chưa sync")
}

func TestDailyCheckDryRun(t *testing.T) {
	e := setupCheck(t)
	res, err := e.s.DailyCheck(e.ctx, ema1d.DailyCheckOptions{DryRun: true})
	require.NoError(t, err)
	require.Equal(t, "synced", res.Status)
	require.Contains(t, res.Report, "(DRY RUN)")
	require.Equal(t, 0, e.count("ema1d_daily_checks"))
	require.Equal(t, 0, e.count("ema1d_syncs WHERE dry_run = 0"))
	require.Empty(t, reports(e))
}

func TestSignalsMatchTheGoldenRows(t *testing.T) {
	e := setupCheck(t)
	_, err := e.s.UpdateCandles(e.ctx)
	require.NoError(t, err)
	sig, err := e.s.Signals(e.ctx)
	require.NoError(t, err)
	require.Empty(t, sig.Warnings)
	require.Equal(t, "2026-10-24", sig.Date)

	raw, err := os.ReadFile(filepath.Join("engine", "testdata", "expected_status.json"))
	require.NoError(t, err)
	var expected map[string][]map[string]any
	require.NoError(t, json.Unmarshal(raw, &expected))
	for i, want := range expected["2026-10-24"] {
		require.Equal(t, want["lenh"], sig.Rows[i].Lenh)
		require.Equal(t, want["lai"], sig.Rows[i].Lai)
		require.Equal(t, want["gia"], sig.Rows[i].Gia)
	}
}
