package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// WriteJSON renders v as a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON strictly decodes the request body into dst: unknown fields
// are rejected, the body size is capped, and trailing content is an
// error. On failure it returns an APIError ready for WriteError.
func DecodeJSON(r *http.Request, dst any) *APIError {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return BadRequest("request body too large")
		}
		return BadRequest("invalid JSON body: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BadRequest("body must contain a single JSON value")
	}
	return nil
}
