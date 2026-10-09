package report

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the report flow over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the report endpoints onto mux. Reporting sits
// behind requireUser; the admin inbox and decisions sit behind
// requireAdmin, which wraps requireUser to keep the session check in
// one place (design D26).
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser, requireAdmin api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("POST /api/v1/coteries/{id}/report", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/admin/reports", requireAdmin(http.HandlerFunc(h.list)))
	mux.Handle("POST /api/v1/admin/reports/{id}/resolve", requireAdmin(http.HandlerFunc(h.resolve)))
	mux.Handle("POST /api/v1/admin/reports/{id}/dismiss", requireAdmin(http.HandlerFunc(h.dismiss)))
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
	var req CreateReportRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	view, err := h.svc.Create(r.Context(), u, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, view)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	items, total, err := h.svc.List(r.Context(), r.URL.Query().Get("status"), page)
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

// decide closes the report with the given status — resolve and dismiss
// share everything but the target state.
func (h *Handler) decide(w http.ResponseWriter, r *http.Request, status string) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req DecideReportRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	view, err := h.svc.Decide(r.Context(), u, r.PathValue("id"), status, req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, view)
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, StatusResolved)
}

func (h *Handler) dismiss(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, StatusDismissed)
}
