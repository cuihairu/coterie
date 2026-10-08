package push

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the push registration endpoints over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the push endpoints onto mux; every route is
// behind requireUser.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("POST /api/v1/push/subscriptions", requireUser(http.HandlerFunc(h.register)))
	mux.Handle("GET /api/v1/push/subscriptions", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("DELETE /api/v1/push/subscriptions/{id}", requireUser(http.HandlerFunc(h.delete)))
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	actor, _ := auth.UserFrom(r.Context())
	sub, err := h.svc.Register(r.Context(), actor, req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, sub)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFrom(r.Context())
	subs, err := h.svc.List(r.Context(), actor)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{"items": subs})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.UserFrom(r.Context())
	if err := h.svc.Delete(r.Context(), actor, r.PathValue("id")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
