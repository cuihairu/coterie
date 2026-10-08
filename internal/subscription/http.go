package subscription

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the subscription module over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the subscription endpoints onto mux. Every
// route is behind requireUser (public endpoints live in the auth module).
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("POST /api/v1/subscriptions", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/subscriptions/{id}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("GET /api/v1/subscriptions", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("PATCH /api/v1/subscriptions/{id}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("DELETE /api/v1/subscriptions/{id}", requireUser(http.HandlerFunc(h.remove)))
}

func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req CreateSubscriptionRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	sub, err := h.svc.Create(r.Context(), u, req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, sub)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	sub, err := h.svc.Get(r.Context(), u, r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, sub)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	subs, total, err := h.svc.List(
		r.Context(),
		u,
		r.URL.Query().Get("owner_user_id"),
		r.URL.Query().Get("product_id"),
		page,
	)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(subs, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req UpdateSubscriptionRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	sub, err := h.svc.Update(r.Context(), u, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, sub)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	if err := h.svc.Delete(r.Context(), u, r.PathValue("id")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
