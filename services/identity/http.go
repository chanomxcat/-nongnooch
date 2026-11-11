package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
)

// API serves the identity HTTP endpoints.
type API struct {
	OTP      *OTPService
	Tokens   *TokenIssuer
	Members  MemberLookup
	Staff    *StaffService
	DemoMode bool
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code string, extra map[string]any) {
	m := map[string]any{"error": code}
	for k, v := range extra {
		m[k] = v
	}
	writeJSON(w, status, m)
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Client-IP"); v != "" {
		return v
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", nil)
		return false
	}
	return true
}

func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/otp", a.requestOTP)
	mux.HandleFunc("POST /auth/verify", a.verify)
	mux.HandleFunc("POST /auth/refresh", a.refresh)
	mux.HandleFunc("POST /auth/logout", a.logout)
	mux.HandleFunc("POST /auth/staff/login", a.staffLogin)
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, a.Tokens.JWKS())
	})
	mux.HandleFunc("POST /internal/seed", func(w http.ResponseWriter, r *http.Request) {
		if !a.DemoMode {
			http.NotFound(w, r)
			return
		}
		if err := a.Staff.Seed(r.Context()); err != nil {
			slog.ErrorContext(r.Context(), "seed failed", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (a *API) requestOTP(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone, Email, Lang, TurnstileToken string
	}
	if !decode(w, r, &in) {
		return
	}
	id, err := a.OTP.Request(r.Context(), in.Phone, in.Email, in.Lang, in.TurnstileToken, clientIP(r))
	var rl *RateLimitError
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, map[string]string{"requestId": id})
	case errors.Is(err, ErrInvalidPhone), errors.Is(err, ErrInvalidEmail), errors.Is(err, ErrContactRequired):
		writeErr(w, http.StatusBadRequest, err.Error(), nil)
	case errors.Is(err, ErrTurnstileFailed):
		writeErr(w, http.StatusForbidden, "turnstile_failed", nil)
	case errors.Is(err, ErrTurnstileUnavailable):
		writeErr(w, http.StatusServiceUnavailable, "turnstile_unavailable", nil)
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfter))
		writeErr(w, http.StatusTooManyRequests, "rate_limited", nil)
	default:
		slog.ErrorContext(r.Context(), "otp request failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal", nil)
	}
}

func (a *API) verify(w http.ResponseWriter, r *http.Request) {
	var in struct{ RequestID, Otp string }
	if !decode(w, r, &in) {
		return
	}
	c, err := a.OTP.Verify(r.Context(), in.RequestID, in.Otp)
	var oe *OTPError
	switch {
	case err == nil:
		tk, err := a.Tokens.Issue(r.Context(), c)
		if err != nil {
			slog.ErrorContext(r.Context(), "issue token failed", "err", err)
			writeErr(w, http.StatusInternalServerError, "internal", nil)
			return
		}
		writeJSON(w, http.StatusOK, tk)
	case errors.As(err, &oe):
		writeErr(w, http.StatusBadRequest, "otp_invalid", map[string]any{"attemptsLeft": oe.AttemptsLeft})
	case errors.Is(err, ErrOTPExpired):
		writeErr(w, http.StatusGone, "otp_expired", nil)
	case errors.Is(err, ErrOTPLocked):
		writeErr(w, http.StatusLocked, "otp_locked", nil)
	case errors.Is(err, ErrOTPUsed):
		writeErr(w, http.StatusConflict, "otp_used", nil)
	default:
		slog.ErrorContext(r.Context(), "otp verify failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal", nil)
	}
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var in struct{ RefreshToken string }
	if !decode(w, r, &in) {
		return
	}
	tk, err := a.Tokens.Refresh(r.Context(), in.RefreshToken)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, tk)
	case errors.Is(err, ErrRefreshReused):
		writeErr(w, http.StatusUnauthorized, "refresh_reused", nil)
	case errors.Is(err, ErrRefreshInvalid):
		writeErr(w, http.StatusUnauthorized, "refresh_invalid", nil)
	default:
		slog.ErrorContext(r.Context(), "refresh failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal", nil)
	}
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	var in struct{ RefreshToken string }
	if !decode(w, r, &in) {
		return
	}
	if err := a.Tokens.Logout(r.Context(), in.RefreshToken); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
