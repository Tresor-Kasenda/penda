package errors

import (
	"net/http"

	fwctx "penda/framework/context"
)

// HTTPError is the framework HTTP error type.
type HTTPError = fwctx.HTTPError

// New creates an HTTPError with status/message/cause.
func New(code int, message string, err error) *HTTPError {
	return fwctx.NewHTTPError(code, message, err)
}

// BadRequest returns a 400 HTTP error.
func BadRequest(message string, err error) *HTTPError {
	return fwctx.NewHTTPError(http.StatusBadRequest, message, err)
}

// Unauthorized returns a 401 HTTP error.
func Unauthorized(message string, err error) *HTTPError {
	return fwctx.NewHTTPError(http.StatusUnauthorized, message, err)
}

// Forbidden returns a 403 HTTP error.
func Forbidden(message string, err error) *HTTPError {
	return fwctx.NewHTTPError(http.StatusForbidden, message, err)
}

// NotFound returns a 404 HTTP error.
func NotFound(message string, err error) *HTTPError {
	return fwctx.NewHTTPError(http.StatusNotFound, message, err)
}

// Conflict returns a 409 HTTP error.
func Conflict(message string, err error) *HTTPError {
	return fwctx.NewHTTPError(http.StatusConflict, message, err)
}

// TooManyRequests returns a 429 HTTP error.
func TooManyRequests(message string, err error) *HTTPError {
	return fwctx.NewHTTPError(http.StatusTooManyRequests, message, err)
}

// Internal returns a 500 HTTP error.
func Internal(message string, err error) *HTTPError {
	return fwctx.NewHTTPError(http.StatusInternalServerError, message, err)
}
