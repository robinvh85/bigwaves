package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/testkit"
)

func TestEmaCrossEndpoints(t *testing.T) {
	k := testkit.New(t)
	k.Exchange.AddSymbol(binancefake.Symbol{Symbol: "SUNUSDT", StepSize: "1", Margin: true}, "0.02")
	p, err := k.App.Pairs.Create(context.Background(), marketdata.CreateInput{Name: "SUNUSDT"})
	require.NoError(t, err)

	code, body := call(t, k, http.MethodPut, "/api/v1/ema-cross/pairs/"+itoa(p.ID), map[string]any{"h1": false, "h4": true, "d1": true})
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "SUNUSDT", body["pairName"])

	code, body = call(t, k, http.MethodGet, "/api/v1/ema-cross/pairs", nil)
	require.Equal(t, http.StatusOK, code, body)
	require.Len(t, body["items"], 1)

	code, body = call(t, k, http.MethodGet, "/api/v1/ema-cross/alerts?timeframe=H4", nil)
	require.Equal(t, http.StatusOK, code, body)
	require.Empty(t, body["items"])

	code, env := call(t, k, http.MethodGet, "/api/v1/ema-cross/config", nil)
	require.Equal(t, http.StatusOK, code, env)
	cfg := env["config"].(map[string]any)
	require.Equal(t, "0.01", cfg["thresholdPercent"].(map[string]any)["h4"])
	cfg["thresholdPercent"].(map[string]any)["h4"] = "0.5"
	code, body = call(t, k, http.MethodPatch, "/api/v1/ema-cross/config", map[string]any{"version": env["version"], "config": cfg})
	require.Equal(t, http.StatusOK, code, body)
	require.Equal(t, "0.5", body["config"].(map[string]any)["thresholdPercent"].(map[string]any)["h4"])

	cfg["thresholdPercent"].(map[string]any)["h4"] = "-1"
	code, body = call(t, k, http.MethodPatch, "/api/v1/ema-cross/config", map[string]any{"version": body["version"], "config": cfg})
	require.Equal(t, http.StatusBadRequest, code, body)

	code, body = call(t, k, http.MethodGet, "/api/v1/ema-cross/config/history", nil)
	require.Equal(t, http.StatusOK, code, body)
	require.Len(t, body["items"], 1)
}
