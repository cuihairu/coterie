package audit

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// RegisterRoutes wires the audit read endpoints onto mux. The two
// owner reads are behind RequireUser and enforce ownership in the
// service; the platform-wide read sits behind RequireAdmin (design
// D27), which wraps RequireUser to keep the session check in one
// place.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser, requireAdmin api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("GET /api/v1/subscriptions/{id}/audit-logs",
		requireUser(http.HandlerFunc(h.listForSubscription)))
	mux.Handle("GET /api/v1/coteries/{id}/audit-logs",
		requireUser(http.HandlerFunc(h.listForCoterie)))
	mux.Handle("GET /api/v1/admin/audit-logs",
		requireAdmin(http.HandlerFunc(h.listAll)))
}

// Handler serves the audit endpoints.
type Handler struct {
	svc *Service
}

func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

func (h *Handler) listForSubscription(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	items, total, err := h.svc.ListForSubscription(r.Context(), u, r.PathValue("id"),
		r.URL.Query().Get("action"), page)
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

func (h *Handler) listForCoterie(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	items, total, err := h.svc.ListForCoterie(r.Context(), u, r.PathValue("id"),
		r.URL.Query().Get("action"), page)
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

// listAll is the platform admin inbox of the ledger (design D27).
func (h *Handler) listAll(w http.ResponseWriter, r *http.Request) {
	if _, ok := actor(r); !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	q := r.URL.Query()
	page := api.ParsePage(r)
	items, total, err := h.svc.ListAll(r.Context(),
		q.Get("action"), q.Get("actor_id"), q.Get("coterie_id"), q.Get("subscription_id"), page)
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
