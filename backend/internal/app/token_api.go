package app

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"

	"bigwaves/internal/ema1d"
	"bigwaves/internal/platform/httpx"
)

// tokenRoutes keeps the endpoints the Rails app exposed to external tasks (Claude's scheduled
// task called them from a browser, hence GET even for state changes), at the same paths. They
// are not behind the session login: EMA1D_API_TOKEN guards them, with rate limiting, and every
// state change is idempotent (run_id) so a replayed URL does nothing new.
//
//	GET /api/ema1d/sync?token=&date=&run_id=&s=&dry_run=
//	GET /api/ema1d/status?token=
//	GET /api/ema1d/todos/create?token=&todo_action=OPEN|CLOSE&coin=&side=&signal_date=&ref_price=&rule=&force=
//	GET /api/discord_messages/send_message?token=&webhook_url=&message=
func (a *App) tokenRoutes(r chi.Router) {
	ema1dLimit := rate.NewLimiter(rate.Every(2*time.Second), 30) // 30 per minute
	relayLimit := rate.NewLimiter(rate.Every(6*time.Second), 10) // 10 per minute

	r.Group(func(g chi.Router) {
		g.Use(a.requireToken, limit(ema1dLimit))
		g.Get("/api/ema1d/sync", a.legacySync)
		g.Get("/api/ema1d/status", a.legacyStatus)
		g.Get("/api/ema1d/todos/create", a.legacyCreateTodo)
	})
	r.With(a.requireToken, limit(relayLimit)).Get("/api/discord_messages/send_message", a.discordRelay)
}

func tokenFrom(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func (a *App) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := a.Cfg.EMA1DAPIToken
		got := tokenFrom(r)
		if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			httpx.WriteError(w, r, httpx.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func limit(l *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow() {
				httpx.WriteError(w, r, httpx.NewError(http.StatusTooManyRequests, "too_many_requests", "too many requests"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// redactedQuery is the query without the token, for the sync log.
func redactedQuery(r *http.Request) string {
	q := r.URL.Query()
	q.Del("token")
	return q.Encode()
}

func (a *App) legacySync(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := a.EMA1D.Sync(r.Context(), ema1d.SyncRequest{
		Date: q.Get("date"), RunID: q.Get("run_id"), S: q.Get("s"), DryRun: q.Get("dry_run"),
	}, redactedQuery(r))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (a *App) legacyStatus(w http.ResponseWriter, r *http.Request) {
	st, err := a.EMA1D.Status(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (a *App) legacyCreateTodo(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := a.EMA1D.CreateManualTodo(r.Context(), ema1d.ManualTodoRequest{
		Action: q.Get("todo_action"), Coin: q.Get("coin"), Side: q.Get("side"), SignalDate: q.Get("signal_date"),
		RefPrice: q.Get("ref_price"), Rule: q.Get("rule"), Force: q.Get("force") == "1" || q.Get("force") == "true",
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// discordRelay forwards a message to a Discord webhook given by the caller. Only Discord webhook
// URLs are accepted, so the bot cannot be used to call arbitrary hosts.
func (a *App) discordRelay(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target, err := url.Parse(q.Get("webhook_url"))
	msg := q.Get("message")
	if err != nil || target.Scheme != "https" || !allowedDiscordHost(target.Host) || !strings.HasPrefix(target.Path, "/api/webhooks/") {
		httpx.WriteError(w, r, httpx.Validation("webhook_url must be a Discord webhook URL", nil))
		return
	}
	if strings.TrimSpace(msg) == "" || len(msg) > 2000 {
		httpx.WriteError(w, r, httpx.Validation("message must be 1-2000 characters", nil))
		return
	}
	if err := postDiscord(r.Context(), target.String(), msg); err != nil {
		httpx.WriteError(w, r, &httpx.Error{Status: http.StatusBadGateway, Code: "discord_error", Message: err.Error(), Err: err})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func allowedDiscordHost(h string) bool {
	switch h {
	case "discord.com", "discordapp.com", "ptb.discord.com", "canary.discord.com":
		return true
	}
	return false
}

var relayClient = &http.Client{Timeout: 15 * time.Second}

func postDiscord(ctx context.Context, webhook, msg string) error {
	body, _ := json.Marshal(map[string]string{"content": msg})
	// webhook was checked by discordRelay: https, a Discord host, /api/webhooks/ path.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body)) //nolint:gosec // see above
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := relayClient.Do(req) //nolint:gosec // Discord hosts only, checked by discordRelay
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("discord http %d: %s", resp.StatusCode, b)
	}
	return nil
}
