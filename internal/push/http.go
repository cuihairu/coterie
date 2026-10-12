package push

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the push registration endpoints over HTTP.
type Handler struct {
	svc         *Service
	vapidPublic string
}

// RegisterRoutes wires the push endpoints onto mux; every route is
// behind requireUser, including the key discovery endpoint — the VAPID
// key is public by nature, but the browser only needs it while signed
// in, and this keeps the documented anonymous surface unchanged.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware, vapidPublic string) {
	h := &Handler{svc: svc, vapidPublic: vapidPublic}
	mux.Handle("POST /api/v1/push/subscriptions", requireUser(http.HandlerFunc(h.register)))
	mux.Handle("GET /api/v1/push/subscriptions", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("DELETE /api/v1/push/subscriptions/{id}", requireUser(http.HandlerFunc(h.delete)))
	mux.Handle("GET /api/v1/push/vapid-public-key", requireUser(http.HandlerFunc(h.vapidKey)))
}

// vapidKey serves the server's VAPID public key so browsers can build
// a push subscription (applicationServerKey); 404 when the push
// channel has no keys configured, so clients can hide the flow.
func (h *Handler) vapidKey(w http.ResponseWriter, _ *http.Request) {
	if h.vapidPublic == "" {
		api.WriteError(w, api.NotFound("push notifications are not configured"))
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"publicKey": h.vapidPublic})
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
