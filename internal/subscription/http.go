package subscription

import (
	"net/http"

	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the subscription module over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the subscription endpoints onto mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	h := &Handler{svc: svc}
	mux.HandleFunc("POST /api/v1/subscriptions", h.create)
	mux.HandleFunc("GET /api/v1/subscriptions/{id}", h.get)
	mux.HandleFunc("GET /api/v1/subscriptions", h.list)
	mux.HandleFunc("PATCH /api/v1/subscriptions/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/subscriptions/{id}", h.remove)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	sub, err := h.svc.Create(r.Context(), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, sub)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	sub, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, sub)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	subs, total, err := h.svc.List(
		r.Context(),
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
	var req UpdateSubscriptionRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	sub, err := h.svc.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, sub)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
