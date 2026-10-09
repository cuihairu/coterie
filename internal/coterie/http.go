package coterie

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the coterie aggregate over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the coterie, member, and invitation endpoints
// onto mux. Every route is behind requireUser; per-coterie permissions
// are enforced in the service layer.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("POST /api/v1/coteries", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/coteries", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/coteries/{id}", requireUser(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /api/v1/coteries/{id}", requireUser(http.HandlerFunc(h.update)))
	mux.Handle("GET /api/v1/coteries/{id}/members", requireUser(http.HandlerFunc(h.listMembers)))
	mux.Handle("POST /api/v1/coteries/{id}/leave", requireUser(http.HandlerFunc(h.leave)))
	mux.Handle("DELETE /api/v1/members/{id}", requireUser(http.HandlerFunc(h.removeMember)))
	mux.Handle("POST /api/v1/coteries/{id}/invitations", requireUser(http.HandlerFunc(h.createInvitation)))
	mux.Handle("GET /api/v1/coteries/{id}/invitations", requireUser(http.HandlerFunc(h.listInvitations)))
	mux.Handle("POST /api/v1/invitations/accept", requireUser(http.HandlerFunc(h.acceptInvitation)))
	mux.Handle("PUT /api/v1/coteries/{id}/blocks/{userID}", requireUser(http.HandlerFunc(h.putBlock)))
	mux.Handle("DELETE /api/v1/coteries/{id}/blocks/{userID}", requireUser(http.HandlerFunc(h.deleteBlock)))
	mux.Handle("GET /api/v1/coteries/{id}/blocks", requireUser(http.HandlerFunc(h.listBlocks)))
}

// actor requires an authenticated user; the middleware guarantees one,
// so a miss is a defensive 401.
func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req CreateCoterieRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	view, err := h.svc.Create(r.Context(), u, req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, view)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, view)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	views, total, err := h.svc.List(r.Context(), actor, r.URL.Query().Get("subscription_id"), page)
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

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req UpdateCoterieRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	view, err := h.svc.Update(r.Context(), u, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, view)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	members, total, err := h.svc.ListMembers(r.Context(), r.PathValue("id"), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(members, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	if err := h.svc.Leave(r.Context(), u, r.PathValue("id")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	if err := h.svc.RemoveMember(r.Context(), u, r.PathValue("id")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req CreateInvitationRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	view, err := h.svc.CreateInvitation(r.Context(), u, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, view)
}

func (h *Handler) listInvitations(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	items, total, err := h.svc.ListInvitations(r.Context(), u, r.PathValue("id"), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(items, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req AcceptInvitationRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	member, err := h.svc.Join(r.Context(), u, req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, member)
}

// putBlock implements PUT /coteries/{id}/blocks/{userID} — 201 when the
// block is new, 200 for an idempotent replay.
func (h *Handler) putBlock(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	entry, created, err := h.svc.Block(r.Context(), u, r.PathValue("id"), r.PathValue("userID"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	api.WriteJSON(w, status, entry)
}

func (h *Handler) deleteBlock(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	if err := h.svc.Unblock(r.Context(), u, r.PathValue("id"), r.PathValue("userID")); err != nil {
		api.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listBlocks(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	page := api.ParsePage(r)
	entries, total, err := h.svc.ListBlocks(r.Context(), u, r.PathValue("id"), page)
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
