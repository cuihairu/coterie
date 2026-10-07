package marketplace

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the marketplace directory and join-request lifecycle.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the marketplace endpoints onto mux. The
// directory is public (no auth); everything else requires a session.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("GET /api/v1/marketplace/coteries", http.HandlerFunc(h.directory))
	mux.Handle("POST /api/v1/coteries/{coterieID}/join-requests", requireUser(http.HandlerFunc(h.createRequest)))
	mux.Handle("GET /api/v1/coteries/{coterieID}/join-requests", requireUser(http.HandlerFunc(h.listRequests)))
	mux.Handle("POST /api/v1/join-requests/{id}/accept", requireUser(http.HandlerFunc(h.accept)))
	mux.Handle("POST /api/v1/join-requests/{id}/decline", requireUser(http.HandlerFunc(h.decline)))
	mux.Handle("DELETE /api/v1/join-requests/{id}", requireUser(http.HandlerFunc(h.cancel)))
}

func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

// directory is intentionally public — browsing listed coteries needs
// no account.
func (h *Handler) directory(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	entries, total, err := h.svc.Directory(r.Context(), r.URL.Query().Get("product_id"), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(entries, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) createRequest(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req CreateJoinRequestRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	reqr, err := h.svc.CreateJoinRequest(r.Context(), u, r.PathValue("coterieID"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, reqr)
}

func (h *Handler) listRequests(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	views, total, err := h.svc.ListJoinRequests(r.Context(), u, r.PathValue("coterieID"), r.URL.Query().Get("status"), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(views, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	reqr, err := h.svc.AcceptJoinRequest(r.Context(), u, r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, reqr)
}

func (h *Handler) decline(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	reqr, err := h.svc.DeclineJoinRequest(r.Context(), u, r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, reqr)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	if err := h.svc.CancelJoinRequest(r.Context(), u, r.PathValue("id")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
