package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/platform/configstore"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/testkit"
)

func call(t *testing.T, k *testkit.Kit, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, k.Server.URL+path, r)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := k.Client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestAPIRequiresSession(t *testing.T) {
	k := testkit.New(t)
	resp, err := http.Get(k.Server.URL + "/api/v1/me")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	code, me := call(t, k, http.MethodGet, "/api/v1/me", nil)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "dev@example.com", me["email"])
}

func TestWritesFromAnotherOriginAreRefused(t *testing.T) {
	k := testkit.New(t)
	req, _ := http.NewRequest(http.MethodPost, k.Server.URL+"/api/v1/pairs", bytes.NewReader([]byte(`{"name":"SUNUSDT"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	resp, err := k.Client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestLogout(t *testing.T) {
	k := testkit.New(t)
	code, _ := call(t, k, http.MethodPost, "/auth/logout", nil)
	require.Equal(t, http.StatusNoContent, code)
	code, _ = call(t, k, http.MethodGet, "/api/v1/me", nil)
	require.Equal(t, http.StatusUnauthorized, code)
}

func TestPairLifecycle(t *testing.T) {
	k := testkit.New(t)
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", StepSize: "1", Margin: true, Borrowable: true}, "0.0251")

	code, body := call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "sunusdt"})
	require.Equal(t, http.StatusCreated, code, body)
	require.Equal(t, "SUNUSDT", body["name"])
	require.Equal(t, "SUN", body["coin"].(map[string]any)["name"])
	require.Equal(t, "1", body["stepSize"])
	require.Equal(t, "0.0251", body["currentPrice"])
	id := int64(body["id"].(float64))

	code, body = call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "SUNUSDT"})
	require.Equal(t, http.StatusConflict, code, body)

	code, body = call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "NOPEUSDT"})
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, "validation_failed", body["error"].(map[string]any)["code"])

	code, body = call(t, k, http.MethodPatch, "/api/v1/pairs/1", map[string]any{"status": "DISABLED"})
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "DISABLED", body["status"])

	coinID := int64(body["coin"].(map[string]any)["id"].(float64))
	code, body = call(t, k, http.MethodPatch, "/api/v1/coins/"+itoa(coinID), map[string]any{"description": "# SUN\nnotes"})
	require.Equal(t, http.StatusOK, code, body)

	code, _ = call(t, k, http.MethodDelete, "/api/v1/pairs/"+itoa(id), nil)
	require.Equal(t, http.StatusNoContent, code)
	code, body = call(t, k, http.MethodGet, "/api/v1/pairs", nil)
	require.Equal(t, http.StatusOK, code)
	require.Empty(t, body["items"])

	// Re-adding a deleted pair restores it with its coin description.
	code, body = call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "SUNUSDT"})
	require.Equal(t, http.StatusCreated, code, body)
	require.Equal(t, id, int64(body["id"].(float64)))
	require.Equal(t, "# SUN\nnotes", body["coin"].(map[string]any)["description"])
}

func TestSpotOnlySymbolNeedsSpotFlag(t *testing.T) {
	k := testkit.New(t)
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "XUSDT", Margin: false}, "1")
	code, _ := call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "XUSDT"})
	require.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "XUSDT", "isSpot": true})
	require.Equal(t, http.StatusCreated, code)
}

func TestMarketJobs(t *testing.T) {
	k := testkit.New(t)
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", StepSize: "1", Margin: true}, "0.02")
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "ETHUSDT", StepSize: "0.0001", Margin: true}, "3000")
	for _, n := range []string{"SUNUSDT", "ETHUSDT"} {
		code, body := call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": n})
		require.Equal(t, http.StatusCreated, code, body)
	}
	k.Exchange.SetPrice("SUNUSDT", "0.0312")
	k.Exchange.DailyRates["SUN"] = money("0.0001")
	k.Exchange.Symbols["ETHUSDT"] = binancefake.Symbol{Symbol: "ETHUSDT", Base: "ETH", Quote: "USDT", StepSize: "0.001", MinNotional: "10", Margin: true}

	for _, job := range []string{"market.update_prices", "market.update_interest", "market.update_filters"} {
		require.NoError(t, k.App.Scheduler.RunSync(job), job)
	}

	_, body := call(t, k, http.MethodGet, "/api/v1/pairs", nil)
	byName := map[string]map[string]any{}
	for _, it := range body["items"].([]any) {
		p := it.(map[string]any)
		byName[p["name"].(string)] = p
	}
	require.Equal(t, "0.0312", byName["SUNUSDT"]["currentPrice"])
	require.Equal(t, "3.65", byName["SUNUSDT"]["interestRate"])
	require.Equal(t, "0.001", byName["ETHUSDT"]["stepSize"])
	require.Equal(t, "10", byName["ETHUSDT"]["minNotional"])

	code, body := call(t, k, http.MethodGet, "/api/v1/jobs", nil)
	require.Equal(t, http.StatusOK, code)
	found := 0
	for _, it := range body["items"].([]any) {
		j := it.(map[string]any)
		if lr, ok := j["lastRun"].(map[string]any); ok {
			require.Equal(t, "OK", lr["status"], j["name"])
			found++
		}
	}
	require.Equal(t, 3, found)
}

func TestFailedJobIsRecordedAndAlerted(t *testing.T) {
	k := testkit.New(t)
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", Margin: true}, "0.02")
	code, _ := call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "SUNUSDT"})
	require.Equal(t, http.StatusCreated, code)

	k.Exchange.Fail(binancefake.Failure{Path: "/api/v3/ticker/price", Times: 2, HTTP: 500, Code: -1000, Msg: "boom"})
	require.Error(t, k.App.Scheduler.RunSync("market.update_prices"))
	require.Error(t, k.App.Scheduler.RunSync("market.update_prices"))

	runs, err := dbq.New(k.App.DB.Read).ListJobRuns(context.Background(), dbq.ListJobRunsParams{Job: "market.update_prices", Limit: 5})
	require.NoError(t, err)
	require.Equal(t, "FAILED", runs[0].Status)
	require.Len(t, k.Recorder.Messages, 1, "same error twice within the window alerts once")
}

func TestAccountWritesBlockedWithoutTrading(t *testing.T) {
	k := testkit.New(t)
	k.Exchange.FundSpot("USDT", "100")
	code, body := call(t, k, http.MethodPost, "/api/v1/account/transfer", map[string]any{"direction": "SPOT_TO_CROSS", "amount": "10"})
	require.Equal(t, http.StatusConflict, code)
	require.Equal(t, "trading_disabled", body["error"].(map[string]any)["code"])
}

func TestAccountBalancesAndTransfer(t *testing.T) {
	k := testkit.New(t, testkit.WithTrading())
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", Margin: true, Borrowable: true}, "0.02")
	code, _ := call(t, k, http.MethodPost, "/api/v1/pairs", map[string]any{"name": "SUNUSDT"})
	require.Equal(t, http.StatusCreated, code)
	k.Exchange.FundSpot("USDT", "100")
	k.Exchange.FundCross("USDT", "50")
	k.Exchange.FundCross("SUN", "1000")

	code, body := call(t, k, http.MethodPost, "/api/v1/account/transfer", map[string]any{"direction": "SPOT_TO_CROSS", "amount": "10"})
	require.Equal(t, http.StatusOK, code, body)

	code, body = call(t, k, http.MethodGet, "/api/v1/account/balances", nil)
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "90", body["spotUsdtFree"])
	require.Equal(t, "60", body["crossUsdtFree"])
	require.Equal(t, "80", body["crossTotalUsdt"]) // 60 USDT + 1000 SUN * 0.02
	require.Equal(t, "170", body["netTotalUsdt"])
}

type cfg struct {
	Budget string `json:"budget"`
	Mode   string `json:"mode"`
}

func TestConfigStoreVersioning(t *testing.T) {
	k := testkit.New(t)
	ctx := context.Background()
	s := k.App.Configs

	c := cfg{Budget: "6000", Mode: "a"}
	v, err := s.Load(ctx, "test", &c)
	require.NoError(t, err)
	require.Equal(t, int64(1), v)

	v2, err := s.Save(ctx, "test", v, cfg{Budget: "7000", Mode: "a"}, "dev@example.com")
	require.NoError(t, err)
	require.Equal(t, int64(2), v2)

	_, err = s.Save(ctx, "test", v, cfg{Budget: "8000"}, "dev@example.com")
	require.ErrorIs(t, err, configstore.ErrVersionConflict)

	var got cfg
	_, err = s.Load(ctx, "test", &got)
	require.NoError(t, err)
	require.Equal(t, "7000", got.Budget)

	hist, err := s.History(ctx, "test", 10)
	require.NoError(t, err)
	require.Len(t, hist, 1)
	require.Equal(t, "dev@example.com", hist[0].UserEmail)
}
