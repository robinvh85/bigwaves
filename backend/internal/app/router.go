// Package app wires configuration, storage, services and HTTP routes together.
package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"bigwaves/internal/httpapi"
	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/web"
)

// Router builds the full HTTP handler: health checks, auth, the JSON API, the SSE stream and
// the embedded SPA.
func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, httpx.Logger, httpx.Recover)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.Ping(r.Context()); err != nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_unavailable"})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	a.Auth.Routes(r)
	// Registered before /api/v1 so the legacy /api/ema1d paths are not caught by the SPA.
	a.tokenRoutes(r)

	r.Route("/api/v1", func(api chi.Router) {
		api.NotFound(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, httpx.ErrNotFound) })
		api.Group(func(authed chi.Router) {
			authed.Use(a.Auth.RequireSession)
			// The event stream is long-lived: no request timeout.
			authed.Get("/events", a.Events.ServeSSE)
			authed.Group(func(json chi.Router) {
				json.Use(middleware.Timeout(60 * time.Second))
				httpapi.Mount(json, &httpapi.Server{Deps: httpapi.Deps{
					Cfg: a.Cfg, DB: a.DB, Pairs: a.Pairs, Account: a.Account, Scheduler: a.Scheduler,
					EMA1D: a.EMA1D, DCA: a.DCA, Configs: a.Configs,
				}})
			})
		})
	})

	r.Handle("/*", web.Handler())
	return r
}
