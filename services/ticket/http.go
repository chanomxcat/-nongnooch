package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"

	"nongnooch/platform/pkg/chassis/authn"
)

// API serves the ticket HTTP endpoints (behind the gateway at /api/ticket).
type API struct {
	Svc      *Service
	DemoMode bool
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func (a *API) internal(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "request failed", "path", r.URL.Path, "err", err)
	errJSON(w, http.StatusInternalServerError, "internal")
}

func (a *API) Register(mux *http.ServeMux) {
	user := authn.Require("user")
	gate := authn.RequireRole("gate", "admin")
	staff := authn.Require("staff")
	mux.Handle("GET /tickets/me", user(http.HandlerFunc(a.mine)))
	mux.Handle("GET /gate/snapshot", gate(http.HandlerFunc(a.snapshot)))
	mux.Handle("POST /gate/scans", gate(http.HandlerFunc(a.scan)))
	mux.Handle("GET /admin/stats/today", staff(http.HandlerFunc(a.stats)))
	mux.HandleFunc("POST /internal/seed", a.seed)
}

func (a *API) mine(w http.ResponseWriter, r *http.Request) {
	p := authn.From(r.Context())
	buyer := p.BuyerID()
	if buyer == "" {
		errJSON(w, http.StatusForbidden, "member_required")
		return
	}
	items, err := a.Svc.ByMember(r.Context(), buyer)
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) snapshot(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = a.Svc.today()
	}
	if !dateRe.MatchString(date) {
		errJSON(w, http.StatusBadRequest, "bad_date")
		return
	}
	snap, err := a.Svc.Snapshot(r.Context(), date)
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (a *API) scan(w http.ResponseWriter, r *http.Request) {
	var in ScanInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in); err != nil || in.GateID == "" {
		errJSON(w, http.StatusBadRequest, "bad_request")
		return
	}
	res, err := a.Svc.Scan(r.Context(), in)
	if err == errBadScan {
		errJSON(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	st, err := a.Svc.Stats(r.Context())
	if err != nil {
		a.internal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// seed is a no-op for now (Task 10 adds demo ticket history).
func (a *API) seed(w http.ResponseWriter, r *http.Request) {
	if !a.DemoMode {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
