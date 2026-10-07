package user

import (
	"net/http"

	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the user module over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the user endpoints onto mux. Every route is
// behind requireUser (public endpoints live in the auth module).
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("POST /api/v1/users", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/users/{id}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("GET /api/v1/users", requireUser(http.HandlerFunc(h.list)))
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
