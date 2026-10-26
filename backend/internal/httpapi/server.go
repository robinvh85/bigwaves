// Package httpapi implements the generated StrictServerInterface from api/openapi.yaml.
// Handlers return domain errors (usually *httpx.Error); the error handler renders them.
package httpapi

//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../../../api/openapi.yaml

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"bigwaves/internal/platform/httpx"
)

// Server aggregates the per-domain handlers. Each domain adds its methods in its own file.
type Server struct {
	Deps
}

var _ StrictServerInterface = (*Server)(nil)

// Mount registers every /api/v1 operation on r.
func Mount(r chi.Router, s *Server, mws ...StrictMiddlewareFunc) {
	strict := NewStrictHandlerWithOptions(s, mws, StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteError(w, r, httpx.Validation(err.Error(), nil))
		},
		ResponseErrorHandlerFunc: httpx.WriteError,
	})
	HandlerWithOptions(strict, ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			httpx.WriteError(w, r, httpx.Validation(err.Error(), nil))
		},
	})
}
