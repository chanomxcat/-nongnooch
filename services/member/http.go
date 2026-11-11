package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"nongnooch/platform/pkg/chassis/authn"
)

// API serves the member HTTP endpoints (behind the gateway at /api/member).
type API struct {
	Svc      *MemberService
	DemoMode bool
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code, "message": code})
}

func (a *API) Register(mux *http.ServeMux) {
	user := authn.Require("user")
	mux.Handle("POST /members", user(http.HandlerFunc(a.register)))
	mux.Handle("GET /members/me", user(http.HandlerFunc(a.me)))
	mux.Handle("GET /members/me/points", user(http.HandlerFunc(a.points)))
	mux.HandleFunc("POST /internal/seed", a.seed)
}

type registerBody struct {
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	Nationality     string `json:"nationality"`
	IDNumber        string `json:"idNumber"`
	PassportCountry string `json:"passportCountry"`
	InviteCode      string `json:"inviteCode"`
	Lang            string `json:"lang"`
	Channel         string `json:"channel"`
	Consents        struct {
		Terms     bool `json:"terms"`
		PDPA      bool `json:"pdpa"`
		Marketing bool `json:"marketing"`
	} `json:"consents"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var b registerBody
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	p := authn.From(r.Context())
	res, err := a.Svc.Register(r.Context(), RegisterInput{
		Contact: p.Sub, FirstName: b.FirstName, LastName: b.LastName, Nationality: b.Nationality,
		IDNumber: b.IDNumber, PassportCountry: b.PassportCountry, InviteCode: b.InviteCode,
		Lang: orDefault(b.Lang, p.Lang), Channel: b.Channel, IP: p.ClientIP, DeviceID: p.DeviceID,
		Consents: ConsentInput{Terms: b.Consents.Terms, PDPA: b.Consents.PDPA, Marketing: b.Consents.Marketing},
	})
	switch {
	case err == nil && res.Existing:
		writeJSON(w, http.StatusOK, map[string]any{"member": res.Member, "existing": true})
	case err == nil:
		writeJSON(w, http.StatusCreated, map[string]any{"member": res.Member, "inviteStatus": res.InviteStatus})
	case errors.Is(err, ErrConsentRequired), errors.Is(err, ErrNameRequired),
		errors.Is(err, ErrInvalidNationality), errors.Is(err, ErrInvalidCountry):
		writeErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrIDNumberInvalid):
		writeErr(w, http.StatusUnprocessableEntity, "id_number_invalid")
	case errors.Is(err, ErrInviteInvalid):
		writeErr(w, http.StatusUnprocessableEntity, "invite_invalid")
	case errors.Is(err, ErrIDNumberTaken):
		writeErr(w, http.StatusConflict, "id_number_taken")
	case errors.Is(err, ErrInviteDown):
		writeErr(w, http.StatusServiceUnavailable, "invite_unavailable")
	default:
		slog.ErrorContext(r.Context(), "register failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal")
	}
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	m, err := a.Svc.ByContact(r.Context(), authn.From(r.Context()).Sub)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "member_not_found")
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "get member failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (a *API) points(w http.ResponseWriter, r *http.Request) {
	m, err := a.Svc.ByContact(r.Context(), authn.From(r.Context()).Sub)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "member_not_found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	avail, pending, err := Balance(r.Context(), a.Svc.DB, m.MemberID)
	if err == nil {
		var items []Entry
		if items, err = History(r.Context(), a.Svc.DB, m.MemberID, limit); err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"available": avail, "pending": pending, "redeemEnabled": false, "items": items,
			})
			return
		}
	}
	slog.ErrorContext(r.Context(), "points failed", "err", err)
	writeErr(w, http.StatusInternalServerError, "internal")
}
