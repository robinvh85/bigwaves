// Package httpx holds HTTP plumbing shared by every handler: error responses and middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Error is an error with an HTTP status and a stable machine-readable code. Domain code returns
// it (or wraps it) and the HTTP layer turns it into {"error":{...}}.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

var (
	ErrUnauthorized = NewError(http.StatusUnauthorized, "unauthorized", "sign in required")
	ErrForbidden    = NewError(http.StatusForbidden, "forbidden", "forbidden")
	ErrNotFound     = NewError(http.StatusNotFound, "not_found", "not found")
)

func Validation(message string, details map[string]any) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "validation_failed", Message: message, Details: details}
}

func Conflict(code, message string) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Message: message}
}

func Unprocessable(code, message string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: code, Message: message}
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// WriteError writes err as a JSON error response. Unknown errors become a 500 without leaking
// their message.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var he *Error
	if !errors.As(err, &he) {
		slog.ErrorContext(r.Context(), "unhandled error", "err", err, "path", r.URL.Path)
		he = NewError(http.StatusInternalServerError, "internal", "internal error")
	} else if he.Status >= 500 {
		slog.ErrorContext(r.Context(), "server error", "err", err, "path", r.URL.Path)
	}
	WriteJSON(w, he.Status, errorBody{Error: errorPayload{Code: he.Code, Message: he.Message, Details: he.Details}})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
