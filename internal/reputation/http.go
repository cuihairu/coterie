package reputation

import (
	"net/http"

	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the reputation endpoint. Reading someone's
// settlement reputation is on the billing read baseline: any
// authenticated user.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the reputation endpoints onto mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("GET /api/v1/users/{id}/reputation", requireUser(http.HandlerFunc(h.get)))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	rep, err := h.svc.For(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, rep)
}
