package app_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/config"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/testkit"
)

func withToken(c *config.Config) { c.EMA1DAPIToken = "secret-token" }

func ema1dKit(t *testing.T) *testkit.Kit {
	k := testkit.New(t, withToken)
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", StepSize: "1", Margin: true, Borrowable: true}, "0.02")
	p, err := k.App.Pairs.Create(context.Background(), marketdata.CreateInput{Name: "SUNUSDT"})
	require.NoError(t, err)
	rule := "XO"
	_, err = dbq.New(k.App.DB.Write).CreateEma1dPair(context.Background(), dbq.CreateEma1dPairParams{
		PairID: p.ID, PairGroup: "CORE", AllowLong: true, AllowShort: true, ShortSupported: true, Status: "ACTIVE",
		AddedOn: "2020-01-01", LongExitRule: &rule,
	})
	require.NoError(t, err)
	return k
}

func TestEma1dStatusAndConfigEndpoints(t *testing.T) {
	k := ema1dKit(t)

	code, body := call(t, k, http.MethodGet, "/api/v1/ema1d/status", nil)
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "6000", body["capital"].(map[string]any)["equityUsdt"])

	code, body = call(t, k, http.MethodGet, "/api/v1/ema1d/pairs", nil)
	require.Equal(t, http.StatusOK, code, body)
	require.Len(t, body["items"], 1)

	code, env := call(t, k, http.MethodGet, "/api/v1/ema1d/config", nil)
	require.Equal(t, http.StatusOK, code, env)
	cfg := env["config"].(map[string]any)
	cfg["pauseOpen"] = true
	code, body = call(t, k, http.MethodPatch, "/api/v1/ema1d/config", map[string]any{"version": env["version"], "config": cfg})
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, true, body["config"].(map[string]any)["pauseOpen"])

	// The old version again: someone saved in between.
	code, body = call(t, k, http.MethodPatch, "/api/v1/ema1d/config", map[string]any{"version": env["version"], "config": cfg})
	require.Equal(t, http.StatusConflict, code, body)

	cfg["maxEntryAgeDays"] = 99
	code, body = call(t, k, http.MethodPatch, "/api/v1/ema1d/config", map[string]any{"version": 2, "config": cfg})
	require.Equal(t, http.StatusBadRequest, code, body)

	code, body = call(t, k, http.MethodGet, "/api/v1/ema1d/config/history", nil)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, body["items"], 1)
	require.Equal(t, "dev@example.com", body["items"].([]any)[0].(map[string]any)["userEmail"])
}

func TestEma1dManualTodoEndpoints(t *testing.T) {
	k := ema1dKit(t)
	code, body := call(t, k, http.MethodPost, "/api/v1/ema1d/todos", map[string]any{
		"action": "OPEN", "coin": "SUN", "side": "LONG", "signalDate": "2026-10-01", "force": true,
	})
	require.Equal(t, http.StatusOK, code, body)
	todo := body["todo"].(map[string]any)
	require.Equal(t, true, todo["cancellable"])
	id := int64(todo["id"].(float64))

	code, body = call(t, k, http.MethodPost, "/api/v1/ema1d/todos/"+itoa(id)+"/cancel", nil)
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "CANCELLED", body["status"])

	code, _ = call(t, k, http.MethodPost, "/api/v1/ema1d/todos/"+itoa(id)+"/retry", nil)
	require.Equal(t, http.StatusConflict, code)

	code, body = call(t, k, http.MethodGet, "/api/v1/ema1d/todos?history=true", nil)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, body["items"], 1)
}

func get(t *testing.T, k *testkit.Kit, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(k.Server.URL + path)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestLegacyTokenEndpoints(t *testing.T) {
	k := ema1dKit(t)

	code, _ := get(t, k, "/api/ema1d/status")
	require.Equal(t, http.StatusUnauthorized, code)
	code, _ = get(t, k, "/api/ema1d/status?token=wrong")
	require.Equal(t, http.StatusUnauthorized, code)

	code, body := get(t, k, "/api/ema1d/status?token=secret-token")
	require.Equal(t, http.StatusOK, code, body)
	require.Contains(t, body, `"equityUsdt":"6000"`)

	q := url.Values{"token": {"secret-token"}, "date": {"2026-10-10"}, "run_id": {"r1"}, "s": {"SUN:L:2026-10-10:0.02:XO"}, "dry_run": {"1"}}
	code, body = get(t, k, "/api/ema1d/sync?"+q.Encode())
	require.Equal(t, http.StatusOK, code, body)
	require.Contains(t, body, `"dry_run":true`)
	require.Contains(t, body, `"created_todos":[`)

	var raw string
	require.NoError(t, k.App.DB.Read.QueryRow("SELECT raw_query FROM ema1d_syncs LIMIT 1").Scan(&raw))
	require.NotContains(t, raw, "secret-token", "the token is never stored")

	q = url.Values{"token": {"secret-token"}, "webhook_url": {"https://evil.example/api/webhooks/1"}, "message": {"hi"}}
	code, _ = get(t, k, "/api/discord_messages/send_message?"+q.Encode())
	require.Equal(t, http.StatusBadRequest, code)
}

func TestTokenEndpointsOffWithoutToken(t *testing.T) {
	k := testkit.New(t) // no EMA1D_API_TOKEN configured
	code, _ := get(t, k, "/api/ema1d/status?token=")
	require.Equal(t, http.StatusUnauthorized, code)
}
