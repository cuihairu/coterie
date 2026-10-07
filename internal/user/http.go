package user

import (
	"net/http"

	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the user module over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the user endpoints onto mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	h := &Handler{svc: svc}
	mux.HandleFunc("POST /api/v1/users", h.create)
	mux.HandleFunc("GET /api/v1/users/{id}", h.get)
	mux.HandleFunc("GET /api/v1/users", h.list)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	u, err := h.svc.Create(r.Context(), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, u)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	users, total, err := h.svc.List(r.Context(), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(users, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}
