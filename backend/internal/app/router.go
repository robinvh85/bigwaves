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

// Router builds the full HTTP handler: health checks, the JSON API and the embedded SPA.
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

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.Timeout(60 * time.Second))
		httpapi.Mount(api, &httpapi.Server{Deps: httpapi.Deps{DB: a.DB}})
		api.NotFound(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, httpx.ErrNotFound) })
	})

	r.Handle("/*", web.Handler())
	return r
}
