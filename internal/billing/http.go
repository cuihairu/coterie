package billing

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes billing periods and contributions over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the billing endpoints onto mux. Reads are open
// to authenticated users; every mutation is owner-only in the service.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("POST /api/v1/subscriptions/{subID}/billing-periods", requireUser(http.HandlerFunc(h.createPeriod)))
	mux.Handle("GET /api/v1/subscriptions/{subID}/billing-periods", requireUser(http.HandlerFunc(h.listPeriods)))
	mux.Handle("GET /api/v1/billing-periods/{id}", requireUser(http.HandlerFunc(h.getPeriod)))
	mux.Handle("POST /api/v1/billing-periods/{id}/close", requireUser(http.HandlerFunc(h.close)))
	mux.Handle("POST /api/v1/billing-periods/{id}/contributions/generate", requireUser(http.HandlerFunc(h.generate)))
	mux.Handle("GET /api/v1/billing-periods/{id}/contributions", requireUser(http.HandlerFunc(h.listContributions)))
	mux.Handle("PATCH /api/v1/contributions/{id}", requireUser(http.HandlerFunc(h.updateContribution)))
}

func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

func (h *Handler) createPeriod(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req CreatePeriodRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	period, err := h.svc.CreatePeriod(r.Context(), u, r.PathValue("subID"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, period)
}

func (h *Handler) listPeriods(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	periods, total, err := h.svc.ListPeriods(r.Context(), r.PathValue("subID"), page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(periods, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) getPeriod(w http.ResponseWriter, r *http.Request) {
	period, err := h.svc.GetPeriod(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, period)
}

func (h *Handler) close(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	period, err := h.svc.Close(r.Context(), u, r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, period)
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req GenerateRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	items, err := h.svc.Generate(r.Context(), u, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, api.NewList(items, api.Meta{
		Total:  int64(len(items)),
		Limit:  len(items),
		Offset: 0,
	}))
}

func (h *Handler) listContributions(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	items, total, err := h.svc.ListContributions(r.Context(), r.PathValue("id"), page)
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

func (h *Handler) updateContribution(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req UpdateContributionRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	item, err := h.svc.UpdateContribution(r.Context(), u, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, item)
}
