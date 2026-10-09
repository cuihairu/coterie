package payment

import (
	"io"
	"net/http"

	"github.com/cuihairu/coterie/internal/auth"
	"github.com/cuihairu/coterie/internal/user"
	"github.com/cuihairu/coterie/pkg/api"
)

// Handler exposes the payments ledger over HTTP.
type Handler struct {
	svc *Service
}

// RegisterRoutes wires the payment endpoints onto mux. Reads are open
// to authenticated users (billing read baseline); recording is owner or
// paying member in the service. The channel webhook (D24) is public —
// the channel can't hold a session, so the adapter's signature check is
// the only gate.
func RegisterRoutes(mux *http.ServeMux, svc *Service, requireUser api.Middleware) {
	h := &Handler{svc: svc}
	mux.Handle("GET /api/v1/payments/methods", requireUser(http.HandlerFunc(h.methods)))
	mux.Handle("POST /api/v1/contributions/{id}/payments", requireUser(http.HandlerFunc(h.record)))
	mux.Handle("GET /api/v1/contributions/{id}/payments", requireUser(http.HandlerFunc(h.list)))
	mux.Handle("GET /api/v1/payments/{id}", requireUser(http.HandlerFunc(h.get)))
	mux.HandleFunc("POST /api/v1/payments/webhooks/{method}", h.webhook)
}

// webhook receives a channel's outbound confirmation (D24). The body is
// read raw — the signature covers the exact bytes — and handed to the
// adapter; an uninteresting event is still a 200 so the channel stops
// retrying.
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		api.WriteError(w, api.Validation("invalid webhook",
			api.Detail{Field: "body", Message: "payload too large or unreadable"}))
		return
	}
	p, err := h.svc.ConfirmWebhook(r.Context(), r.PathValue("method"), r.Header, body)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	if p == nil {
		api.WriteJSON(w, http.StatusOK, map[string]any{"received": true})
		return
	}
	api.WriteJSON(w, http.StatusOK, p)
}

func actor(r *http.Request) (*user.User, bool) {
	return auth.UserFrom(r.Context())
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	u, ok := actor(r)
	if !ok {
		api.WriteError(w, api.Unauthorized("missing bearer token"))
		return
	}
	var req RecordPaymentRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	p, err := h.svc.Record(r.Context(), u, r.PathValue("id"), req)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, p)
}

// methods lists the registered payment channels.
func (h *Handler) methods(w http.ResponseWriter, _ *http.Request) {
	api.WriteJSON(w, http.StatusOK, api.NewList(h.svc.Methods(), api.Meta{
		Total: int64(len(h.svc.Methods())),
	}))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePage(r)
	items, total, err := h.svc.List(r.Context(), r.PathValue("id"), page)
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

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, p)
}
