package notification

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the notification endpoints over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the notification endpoints onto mux. Every
// notification belongs to its reader — there are no shared reads.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("GET /api/v1/notifications", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/v1/notifications/{id}/read", requireUser(http.HandlerFunc(h.markRead)))
}

func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	unreadOnly := r.URL.Query().Get("unread") == "true"
	items, total, err := h.svc.List(r.Context(), u, unreadOnly, page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(items, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	n, err := h.svc.MarkRead(r.Context(), u, r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, n)
}
