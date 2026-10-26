package binance

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
)

// APIError is an error response from Binance: {"code": -1121, "msg": "Invalid symbol."}.
type APIError struct {
	HTTPStatus int
	Code       int
	Msg        string
	// Request describes the call without signature/timestamp, for logs and alerts.
	Request string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("binance %d (http %d): %s [%s]", e.Code, e.HTTPStatus, e.Msg, e.Request)
}

// Binance error codes we branch on.
const (
	CodeUnknown            = -1000
	CodeDisconnected       = -1001
	CodeTooManyRequests    = -1003
	CodeUnexpectedResponse = -1006
	CodeTimeout            = -1007
	CodeServerBusy         = -1008
	CodeTooManyOrders      = -1015
	CodeServiceShutdown    = -1016
	CodeInvalidTimestamp   = -1021
	CodeBadSymbol          = -1121
	CodeNoSuchOrder        = -2013
	CodeMarginBusy         = -3045
)

// transientCodes are worth retrying later; the list matches the Rails todo queue.
var transientCodes = []int{
	CodeUnknown, CodeDisconnected, CodeTooManyRequests, CodeUnexpectedResponse, CodeTimeout,
	CodeServerBusy, CodeTooManyOrders, CodeServiceShutdown, CodeInvalidTimestamp, CodeMarginBusy,
}

// ErrTradingDisabled is returned by every write call while TRADING_ENABLED is false.
var ErrTradingDisabled = errors.New("binance: trading disabled (TRADING_ENABLED=false)")

// ErrBanned is returned while Binance has banned this IP (HTTP 418) or asked us to back off (429).
var ErrBanned = errors.New("binance: rate limited, backing off")

// IsTransient reports whether err is worth retrying: network failures, timeouts, 5xx and the
// Binance codes that mean "busy, try again".
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrBanned) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatus >= 500 || slices.Contains(transientCodes, apiErr.Code)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// HasCode reports whether err is a Binance error with the given code.
func HasCode(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}
