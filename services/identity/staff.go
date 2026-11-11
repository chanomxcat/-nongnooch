package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	staffMaxFails   = 10
	staffFailWindow = 15 * time.Minute
	demoPassword    = "Demo1234!"
)

var (
	ErrInvalidCredentials = errors.New("invalid_credentials")
	ErrStaffInactive      = errors.New("staff_inactive")
)

// dummyHash is compared when the email is unknown so both failures cost the same.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type StaffService struct {
	DB     *sql.DB
	Tokens *TokenIssuer
	Now    func() time.Time
}

func (s *StaffService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// Login verifies credentials; failed attempts are limited per email (10 per 15 min).
func (s *StaffService) Login(ctx context.Context, email, password string) (string, int, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	now := s.now()
	var fails int
	var oldest sql.NullTime
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*), MIN(created_at) FROM staff_login_fails WHERE email = ? AND created_at > ?`,
		email, now.Add(-staffFailWindow)).Scan(&fails, &oldest); err != nil {
		return "", 0, err
	}
	if fails >= staffMaxFails {
		return "", 0, &RateLimitError{RetryAfter: ceilSecs(oldest.Time.Add(staffFailWindow).Sub(now))}
	}
	var id, name, hash, roles string
	var active bool
	err := s.DB.QueryRowContext(ctx, `SELECT id, name, password, roles, active FROM staff_users WHERE email = ?`, email).
		Scan(&id, &name, &hash, &roles, &active)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return "", 0, s.fail(ctx, email, now)
	}
	if err != nil {
		return "", 0, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", 0, s.fail(ctx, email, now)
	}
	if !active {
		return "", 0, ErrStaffInactive
	}
	return s.Tokens.StaffAccess(id, name, strings.Split(roles, ","))
}

func (s *StaffService) fail(ctx context.Context, email string, now time.Time) error {
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO staff_login_fails(email, created_at) VALUES(?,?)`, email, now); err != nil {
		return err
	}
	return ErrInvalidCredentials
}

// Seed inserts one demo staff per role (+ approver2); idempotent.
func (s *StaffService) Seed(ctx context.Context) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	users := []struct{ local, role string }{
		{"admin", "admin"}, {"editor", "editor"}, {"approver", "approver"}, {"finance", "finance"},
		{"gate", "gate"}, {"agent_manager", "agent_manager"}, {"viewer", "viewer"}, {"approver2", "approver"},
	}
	for _, u := range users {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if _, err := s.DB.ExecContext(ctx,
			`INSERT IGNORE INTO staff_users(id, email, name, password, roles, active, created_at) VALUES(?,?,?,?,?,TRUE,?)`,
			id.String(), u.local+"@demo.nongnooch.local", "Demo "+u.local, string(hash), u.role, s.now()); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) staffLogin(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if !decode(w, r, &in) {
		return
	}
	acc, exp, err := a.Staff.Login(r.Context(), in.Email, in.Password)
	var rl *RateLimitError
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"access": acc, "expiresIn": exp})
	case errors.Is(err, ErrInvalidCredentials):
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", nil)
	case errors.Is(err, ErrStaffInactive):
		writeErr(w, http.StatusForbidden, "staff_inactive", nil)
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfter))
		writeErr(w, http.StatusTooManyRequests, "rate_limited", nil)
	default:
		slog.ErrorContext(r.Context(), "staff login failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal", nil)
	}
}
