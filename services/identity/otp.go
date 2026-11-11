package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"time"

	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/events"
	"nongnooch/platform/pkg/chassis/outbox"
)

const (
	otpTTL         = 5 * time.Minute
	maxAttempts    = 5
)

// OTPLimits are the resend/hourly OTP request limits (env-configurable, see LoadOTPLimits).
type OTPLimits struct {
	Cooldown       time.Duration
	PerContactHour int
	PerIPHour      int
}

// LoadOTPLimits reads OTP_RESEND_COOLDOWN_SECONDS (60), OTP_MAX_PER_CONTACT_PER_HOUR (5) and
// OTP_MAX_PER_IP_PER_HOUR (20); invalid or non-positive values fall back to the defaults.
func LoadOTPLimits(getenv func(string) string) OTPLimits {
	n := func(k string, def int) int {
		if v, err := strconv.Atoi(getenv(k)); err == nil && v > 0 {
			return v
		}
		return def
	}
	return OTPLimits{
		Cooldown:       time.Duration(n("OTP_RESEND_COOLDOWN_SECONDS", 60)) * time.Second,
		PerContactHour: n("OTP_MAX_PER_CONTACT_PER_HOUR", 5),
		PerIPHour:      n("OTP_MAX_PER_IP_PER_HOUR", 20),
	}
}

func (l OTPLimits) withDefaults() OTPLimits {
	d := LoadOTPLimits(func(string) string { return "" })
	if l.Cooldown <= 0 {
		l.Cooldown = d.Cooldown
	}
	if l.PerContactHour <= 0 {
		l.PerContactHour = d.PerContactHour
	}
	if l.PerIPHour <= 0 {
		l.PerIPHour = d.PerIPHour
	}
	return l
}

var (
	ErrOTPInvalid  = errors.New("otp_invalid")
	ErrOTPExpired  = errors.New("otp_expired")
	ErrOTPLocked   = errors.New("otp_locked")
	ErrOTPUsed     = errors.New("otp_used")
	ErrRateLimited = errors.New("rate_limited")
)

// RateLimitError carries Retry-After seconds; Is(ErrRateLimited) is true.
type RateLimitError struct{ RetryAfter int }

func (e *RateLimitError) Error() string        { return "rate_limited" }
func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// OTPError wraps ErrOTPInvalid with the remaining attempts.
type OTPError struct {
	Err          error
	AttemptsLeft int
}

func (e *OTPError) Error() string { return e.Err.Error() }
func (e *OTPError) Unwrap() error { return e.Err }

var langs = map[string]bool{"th": true, "en": true, "zh": true, "ja": true, "ko": true, "de": true, "fr": true}

type OTPService struct {
	DB       *sql.DB
	Verifier Verifier
	Pepper   []byte
	Limits   OTPLimits // zero fields use the defaults
	Now      func() time.Time
}

func (s *OTPService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *OTPService) mac(requestID, code string) []byte {
	h := hmac.New(sha256.New, s.Pepper)
	h.Write([]byte(requestID + ":" + code))
	return h.Sum(nil)
}

func ipBytes(ip string) []byte {
	p := net.ParseIP(ip)
	if p == nil {
		return make([]byte, 4)
	}
	if v4 := p.To4(); v4 != nil {
		return v4
	}
	return p.To16()
}

func (s *OTPService) Request(ctx context.Context, phone, email, lang, turnstileToken, ip string) (string, error) {
	c, err := NormalizeContact(phone, email)
	if err != nil {
		return "", err
	}
	if err := s.Verifier.Verify(ctx, turnstileToken, ip); err != nil {
		if errors.Is(err, ErrTurnstileFailed) {
			return "", ErrTurnstileFailed
		}
		return "", ErrTurnstileUnavailable
	}
	if !langs[lang] {
		lang = "en"
	}
	now := s.now()
	ipb := ipBytes(ip)

	keys := []string{"c:" + c.Value, fmt.Sprintf("i:%x", ipb)}
	for _, k := range keys { // autocommit, so no locks are held while creating rows
		if _, err := s.DB.ExecContext(ctx, `INSERT IGNORE INTO otp_rate(k) VALUES(?)`, k); err != nil {
			return "", err
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// Serialize requests per contact and per IP (fixed order: "c:" < "i:") so the
	// limit checks and the insert are atomic.
	for _, k := range keys {
		var x string
		if err := tx.QueryRowContext(ctx, `SELECT k FROM otp_rate WHERE k = ? FOR UPDATE`, k).Scan(&x); err != nil {
			return "", err
		}
	}
	if err := s.checkLimits(ctx, tx, c.Value, ipb, now); err != nil {
		return "", err
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	id := uuid.NewString()
	ev, err := events.New(events.OTPRequested, map[string]string{
		"requestId": id, "kind": c.Kind, "contact": c.Value, "code": code, "lang": lang,
	})
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO otp_requests(id, contact, kind, code_hmac, lang, ip, expires_at, created_at) VALUES(?,?,?,?,?,?,?,?)`,
		id, c.Value, c.Kind, s.mac(id, code), lang, ipb, now.Add(otpTTL), now); err != nil {
		return "", err
	}
	if err := outbox.Write(ctx, tx, ev); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func ceilSecs(d time.Duration) int {
	n := int((d + time.Second - 1) / time.Second)
	if n < 1 {
		n = 1
	}
	return n
}

func (s *OTPService) checkLimits(ctx context.Context, q *sql.Tx, contact string, ip []byte, now time.Time) error {
	lim := s.Limits.withDefaults()
	hourAgo := now.Add(-time.Hour)
	var last sql.NullTime
	if err := q.QueryRowContext(ctx, `SELECT MAX(created_at) FROM otp_requests WHERE contact = ?`, contact).Scan(&last); err != nil {
		return err
	}
	if last.Valid {
		if wait := last.Time.Add(lim.Cooldown).Sub(now); wait > 0 {
			return &RateLimitError{RetryAfter: ceilSecs(wait)}
		}
	}
	var cnt int
	var oldest sql.NullTime
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*), MIN(created_at) FROM otp_requests WHERE contact = ? AND created_at > ?`,
		contact, hourAgo).Scan(&cnt, &oldest); err != nil {
		return err
	}
	if cnt >= lim.PerContactHour {
		return &RateLimitError{RetryAfter: ceilSecs(oldest.Time.Add(time.Hour).Sub(now))}
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*), MIN(created_at) FROM otp_requests WHERE ip = ? AND created_at > ?`,
		ip, hourAgo).Scan(&cnt, &oldest); err != nil {
		return err
	}
	if cnt >= lim.PerIPHour {
		return &RateLimitError{RetryAfter: ceilSecs(oldest.Time.Add(time.Hour).Sub(now))}
	}
	return nil
}

// Verify checks the code for requestID. A wrong code commits the attempt counter.
func (s *OTPService) Verify(ctx context.Context, requestID, code string) (Contact, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Contact{}, err
	}
	defer tx.Rollback()
	var (
		contact, kind, lang string
		hm                  []byte
		attempts            int
		expires             time.Time
		used                sql.NullTime
	)
	err = tx.QueryRowContext(ctx,
		`SELECT contact, kind, code_hmac, lang, attempts, expires_at, used_at FROM otp_requests WHERE id = ? FOR UPDATE`,
		requestID).Scan(&contact, &kind, &hm, &lang, &attempts, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return Contact{}, &OTPError{Err: ErrOTPInvalid}
	}
	if err != nil {
		return Contact{}, err
	}
	switch {
	case used.Valid:
		return Contact{}, ErrOTPUsed
	case attempts >= maxAttempts:
		return Contact{}, ErrOTPLocked
	case !s.now().Before(expires):
		return Contact{}, ErrOTPExpired
	}
	if !hmac.Equal(hm, s.mac(requestID, code)) {
		attempts++
		if _, err := tx.ExecContext(ctx, `UPDATE otp_requests SET attempts = ? WHERE id = ?`, attempts, requestID); err != nil {
			return Contact{}, err
		}
		if err := tx.Commit(); err != nil {
			return Contact{}, err
		}
		if attempts >= maxAttempts {
			return Contact{}, ErrOTPLocked
		}
		return Contact{}, &OTPError{Err: ErrOTPInvalid, AttemptsLeft: maxAttempts - attempts}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE otp_requests SET used_at = ? WHERE id = ?`, s.now(), requestID); err != nil {
		return Contact{}, err
	}
	if err := tx.Commit(); err != nil {
		return Contact{}, err
	}
	return Contact{Kind: kind, Value: contact, Lang: lang}, nil
}
