package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"nongnooch/platform/pkg/chassis/authn"
)

// API serves the commerce HTTP endpoints (behind the gateway at /api/commerce).
type API struct {
	Svc      *Service
	DemoMode bool
}

func (a *API) Register(mux *http.ServeMux) {
	user := authn.Require("user")
	staff := authn.Require("staff")
	mux.HandleFunc("GET /products", a.products)
	mux.HandleFunc("GET /products/availability", a.availability)
	mux.Handle("POST /orders", user(http.HandlerFunc(a.createOrder)))
	mux.Handle("GET /orders", user(http.HandlerFunc(a.listOrders)))
	mux.Handle("GET /orders/{id}", user(http.HandlerFunc(a.getOrder)))
	mux.Handle("POST /orders/{id}/payments", user(http.HandlerFunc(a.createPayment)))
	mux.HandleFunc("POST /webhooks/kbank", a.webhook)
	mux.Handle("GET /admin/stats/today", staff(http.HandlerFunc(a.stats)))
	mux.HandleFunc("POST /internal/seed", a.seed)
}

func write(w http.ResponseWriter, r result) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.Status)
	_, _ = w.Write(r.Body)
}

func writeJSON(w http.ResponseWriter, status int, v any) { write(w, ok(status, v)) }

func (a *API) internal(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "err", err)
	write(w, fail(http.StatusInternalServerError, "internal"))
}

func (a *API) products(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if len(lang) > 2 {
		lang = lang[:2]
	}
	items, err := a.Svc.Products(r.Context(), lang)
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) availability(w http.ResponseWriter, r *http.Request) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	_, e1 := time.Parse(dateLayout, from)
	_, e2 := time.Parse(dateLayout, to)
	if e1 != nil || e2 != nil {
		write(w, fail(http.StatusBadRequest, "bad_request"))
		return
	}
	items, err := a.Svc.Availability(r.Context(), from, to)
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func memberOf(w http.ResponseWriter, r *http.Request) (authn.Principal, bool) {
	p := authn.From(r.Context())
	p.MemberID = p.BuyerID() // guests (OTP-verified, no membership) may buy at the general price
	if p.MemberID == "" {
		write(w, fail(http.StatusForbidden, "member_required"))
		return p, false
	}
	return p, true
}

func (a *API) createOrder(w http.ResponseWriter, r *http.Request) {
	p, good := memberOf(w, r)
	if !good {
		return
	}
	var req orderReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		write(w, fail(http.StatusBadRequest, "bad_request"))
		return
	}
	lang := p.Lang
	if lang == "" {
		lang = "th"
	}
	res, err := a.Svc.runIdem(r.Context(), p.MemberID, "POST /orders", r.Header.Get("Idempotency-Key"), func(tx *sql.Tx) (result, error) {
		return a.Svc.CreateOrder(r.Context(), tx, p.MemberID, lang, req)
	})
	if err != nil {
		a.internal(w, r, err)
		return
	}
	write(w, res)
}

func (a *API) createPayment(w http.ResponseWriter, r *http.Request) {
	p, good := memberOf(w, r)
	if !good {
		return
	}
	var req struct {
		Method    string `json:"method"`
		ReturnURL string `json:"returnUrl"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&req); err != nil || req.ReturnURL == "" {
		write(w, fail(http.StatusBadRequest, "bad_request"))
		return
	}
	id := r.PathValue("id")
	res, err := a.Svc.runIdem(r.Context(), p.MemberID, "POST /orders/"+id+"/payments", r.Header.Get("Idempotency-Key"), func(tx *sql.Tx) (result, error) {
		return a.Svc.CreatePayment(r.Context(), tx, p.MemberID, id, req.Method, req.ReturnURL)
	})
	if err != nil {
		a.internal(w, r, err)
		return
	}
	write(w, res)
}

func (a *API) listOrders(w http.ResponseWriter, r *http.Request) {
	p, good := memberOf(w, r)
	if !good {
		return
	}
	orders, err := a.Svc.Orders(r.Context(), p.MemberID, "")
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": orders})
}

func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	p, good := memberOf(w, r)
	if !good {
		return
	}
	orders, err := a.Svc.Orders(r.Context(), p.MemberID, r.PathValue("id"))
	if err != nil {
		a.internal(w, r, err)
		return
	}
	if len(orders) == 0 {
		write(w, fail(http.StatusNotFound, "order_not_found"))
		return
	}
	writeJSON(w, http.StatusOK, orders[0])
}

func (a *API) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		write(w, fail(http.StatusBadRequest, "bad_request"))
		return
	}
	if !a.Svc.validSignature(body, strings.TrimSpace(r.Header.Get("X-Mock-Signature"))) {
		write(w, fail(http.StatusUnauthorized, "invalid_signature"))
		return
	}
	var wb webhookBody
	if err := json.Unmarshal(body, &wb); err != nil || wb.EventID == "" || wb.Ref == "" {
		write(w, fail(http.StatusBadRequest, "bad_request"))
		return
	}
	if err := a.Svc.HandleWebhook(r.Context(), wb); err != nil {
		a.internal(w, r, err) // 500 makes the provider retry
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	st, err := a.Svc.Stats(r.Context())
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) seed(w http.ResponseWriter, r *http.Request) {
	if !a.DemoMode {
		http.NotFound(w, r)
		return
	}
	if err := a.Svc.Seed(r.Context()); err != nil {
		a.internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
