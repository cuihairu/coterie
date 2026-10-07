package provider

import (
	"net/http"

	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the provider module over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the provider endpoints onto mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	h := &Handler{svc: svc}
	mux.HandleFunc("POST /api/v1/providers", h.create)
	mux.HandleFunc("GET /api/v1/providers/{id}", h.get)
	mux.HandleFunc("GET /api/v1/providers", h.list)
	mux.HandleFunc("PATCH /api/v1/providers/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/providers/{id}", h.remove)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	p, err := h.svc.Create(r.Context(), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, p)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	providers, total, err := h.svc.List(r.Context(), r.URL.Query().Get("category"), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(providers, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req UpdateProviderRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	p, err := h.svc.Update(r.Context(), r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), r.PathValue("id")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
