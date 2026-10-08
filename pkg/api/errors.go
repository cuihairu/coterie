package api

import (
	"errors"
	"fmt"
	"net/http"
)

// Detail points at a single invalid request field.
type Detail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError is the canonical error, rendered as {"error": {...}}.
type APIError struct {
	Status  int      `json:"-"`
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []Detail `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string { return e.Message }

// BadRequest reports a malformed request (400).
func BadRequest(format string, args ...any) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: fmt.Sprintf(format, args...)}
}

// Validation reports semantically invalid input (422).
func Validation(message string, details ...Detail) *APIError {
	return &APIError{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Message: message, Details: details}
}

// Unauthorized reports missing or invalid credentials (401).
func Unauthorized(format string, args ...any) *APIError {
	return &APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: fmt.Sprintf(format, args...)}
}

// Forbidden reports an authenticated actor lacking the required role (403).
func Forbidden(format string, args ...any) *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "forbidden", Message: fmt.Sprintf(format, args...)}
}

// NotFound reports a missing resource (404).
func NotFound(format string, args ...any) *APIError {
	return &APIError{Status: http.StatusNotFound, Code: "not_found", Message: fmt.Sprintf(format, args...)}
}

// Conflict reports a uniqueness or referential-integrity clash (409).
func Conflict(format string, args ...any) *APIError {
	return &APIError{Status: http.StatusConflict, Code: "conflict", Message: fmt.Sprintf(format, args...)}
}

// TooManyRequests reports a request rejected by local rate limiting
// (429). Handlers pair it with a Retry-After header.
func TooManyRequests(format string, args ...any) *APIError {
	return &APIError{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: fmt.Sprintf(format, args...)}
}

// Internal reports an unexpected failure (500) without leaking details.
func Internal() *APIError {
	return &APIError{Status: http.StatusInternalServerError, Code: "internal", Message: "internal server error"}
}

// WriteError renders err as the error envelope; non-APIError values map
// to a generic 500.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		apiErr = Internal()
	}
	WriteJSON(w, apiErr.Status, struct {
		Error *APIError `json:"error"`
	}{apiErr})
}
