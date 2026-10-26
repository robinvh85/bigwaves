// Package web serves the embedded frontend build. Any path that is not a file falls back to
// index.html so client-side routes work on reload.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves dist/. When the frontend has not been built (dev), it returns a short hint.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "frontend not built: run `make build-fe`, or use the Vite dev server on :5173", http.StatusNotFound)
		})
	}
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// SPA route: serve index.html, never cache it.
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
