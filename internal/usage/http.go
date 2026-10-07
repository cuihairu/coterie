package usage

import (
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the usage ledger over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the usage endpoints onto mux. Reads are open to
// authenticated users; recording is owner-only in the service.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("POST /api/v1/subscriptions/{subID}/usage-records", requireUser(http.HandlerFunc(h.create)))
	mux.Handle("GET /api/v1/subscriptions/{subID}/usage-records", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/usage-records/{id}", requireUser(http.HandlerFunc(h.get)))
}

func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req CreateRecordRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	record, err := h.svc.Create(r.Context(), u, r.PathValue("subID"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, record)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters, err := ParseFilters(q.Get("member_id"), q.Get("unit"), q.Get("from"), q.Get("to"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	page := api.ParsePage(r)
	records, total, err := h.svc.List(r.Context(), r.PathValue("subID"), filters, page)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, api.NewList(records, api.Meta{
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	record, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, record)
}
