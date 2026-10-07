package api

import (
	"net/http"
	"strconv"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Page carries list pagination parameters.
type Page struct {
	Limit  int
	Offset int
}

// Meta is the pagination summary attached to list responses.
type Meta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// ParsePage reads limit/offset from the query string. Invalid values
// fall back to defaults; limit is clamped to [1, maxLimit].
func ParsePage(r *http.Request) Page {
	q := r.URL.Query()

	limit := defaultLimit
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = v
	}
	if limit < 1 {
		limit = 1
	} else if limit > maxLimit {
		limit = maxLimit
	}

	offset := 0
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v > 0 {
		offset = v
	}

	return Page{Limit: limit, Offset: offset}
}

// ListResponse is the standard list envelope.
type ListResponse[T any] struct {
	Items []T  `json:"items"`
	Meta  Meta `json:"meta"`
}

// NewList builds a ListResponse, normalizing nil items to an empty slice.
func NewList[T any](items []T, meta Meta) ListResponse[T] {
	if items == nil {
		items = []T{}
	}
	return ListResponse[T]{Items: items, Meta: meta}
}
