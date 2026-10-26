// Package testkit builds a full App on a temporary database and a fake Binance for tests.
package testkit

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"bigwaves/internal/app"
	"bigwaves/internal/config"
	"bigwaves/internal/exchange/binancefake"
	"bigwaves/internal/platform/notify"
)

type Kit struct {
	App      *app.App
	Exchange *binancefake.Exchange
	Server   *httptest.Server
	// Client is signed in through the dev login.
	Client   *http.Client
	Recorder *notify.Recorder
}

type Option func(*config.Config)

func WithTrading() Option { return func(c *config.Config) { c.TradingEnabled = true } }

func New(t *testing.T, opts ...Option) *Kit {
	t.Helper()
	ex := binancefake.New(t)
	cfg := config.Config{
		Env:              "test",
		DBPath:           filepath.Join(t.TempDir(), "test.db"),
		AppURL:           "http://app.test",
		SchedulerEnabled: false,
		Binance:          config.Binance{BaseURL: ex.URL(), APIKey: "k", SecretKey: "s"},
		Session:          config.Session{TTL: time.Hour, DevUserEmail: "dev@example.com"},
	}
	for _, o := range opts {
		o(&cfg)
	}
	a, err := app.New(context.Background(), cfg)
	require.NoError(t, err)
	rec := &notify.Recorder{}
	a.Notifier = rec
	a.Alerter.Notifier = rec
	a.EMA1D.Notifier = rec
	a.EMA1D.ReportConfigured = true
	a.EMA1D.Sleep = func(time.Duration) {}
	t.Cleanup(func() { _ = a.Close() })

	srv := httptest.NewServer(a.Router())
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(srv.URL + "/auth/login")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)

	return &Kit{App: a, Exchange: ex, Server: srv, Client: c, Recorder: rec}
}
