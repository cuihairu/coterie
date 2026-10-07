package seat

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the seat endpoints over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the seat endpoints onto mux. Reads are open to
// any authenticated user; mutations require the subscription owner and
// are enforced in the service layer.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("GET /api/v1/subscriptions/{subID}/seats", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/v1/subscriptions/{subID}/seats", requireUser(http.HandlerFunc(h.provision)))
	mux.Handle("GET /api/v1/seats/{id}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /api/v1/seats/{id}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("POST /api/v1/seats/{id}/assign", requireUser(http.HandlerFunc(h.assign)))
	mux.Handle("POST /api/v1/seats/{id}/release", requireUser(http.HandlerFunc(h.release)))
}

func (h *Handler) provision(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFrom(r.Context())
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req ProvisionRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	seats, err := h.svc.Provision(r.Context(), actor, r.PathValue("subID"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, api.NewList(seats, api.Meta{
		Total:  int64(len(seats)),
		Limit:  len(seats),
		Offset: 0,
	}))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	seats, total, err := h.svc.List(r.Context(), r.PathValue("subID"), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(seats, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	seat, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, seat)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFrom(r.Context())
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req UpdateSeatRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	seat, err := h.svc.Update(r.Context(), actor, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, seat)
}

func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFrom(r.Context())
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req AssignRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	seat, err := h.svc.Assign(r.Context(), actor, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, seat)
}

func (h *Handler) release(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.UserFrom(r.Context())
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	seat, err := h.svc.Release(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, seat)
}
