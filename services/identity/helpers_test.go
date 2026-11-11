//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/testutil"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type fakeVerifier struct{ err error }

func (f *fakeVerifier) Verify(context.Context, string, string) error { return f.err }

type fakeMembers struct{ mid string }

func (f fakeMembers) GetMemberIDByContact(context.Context, string) (string, error) {
	return f.mid, nil
}

type env struct {
	d     *sql.DB
	clock *fakeClock
	ver   *fakeVerifier
	otp   *OTPService
	tk    *TokenIssuer
	pub   ed25519.PublicKey
	mux   *http.ServeMux
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d, _ := testutil.DB(t)
	sub, _ := fs.Sub(migrationsFS, "migrations")
	if err := db.Migrate(testutil.Ctx(t), d, sub); err != nil {
		t.Fatal(err)
	}
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	e := &env{d: d, clock: &fakeClock{t: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}, ver: &fakeVerifier{}, pub: pub}
	e.otp = &OTPService{DB: d, Verifier: e.ver, Pepper: []byte("test-pepper"), Now: e.clock.Now}
	e.tk = &TokenIssuer{DB: d, Key: priv, Members: noMember{}, Now: e.clock.Now}
	e.mux = http.NewServeMux()
	(&API{OTP: e.otp, Tokens: e.tk, DemoMode: true}).Register(e.mux)
	return e
}

func (e *env) do(method, path, ip string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if ip != "" {
		r.Header.Set("X-Client-IP", ip)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func (e *env) post(path, ip string, body any) *httptest.ResponseRecorder {
	return e.do("POST", path, ip, body)
}

// lastOTP returns requestId and plaintext code of the newest otp.requested outbox row.
func (e *env) lastOTP(t *testing.T) (id, code string) {
	t.Helper()
	var raw string
	if err := e.d.QueryRow(`SELECT payload FROM outbox WHERE type='otp.requested' ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var ev struct {
		Payload struct{ RequestID, Code string } `json:"payload"`
	}
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatal(err)
	}
	return ev.Payload.RequestID, ev.Payload.Code
}

// newOTP requests an OTP for phone and returns its id and plaintext code.
func (e *env) newOTP(t *testing.T, phone string) (id, code string) {
	t.Helper()
	w := e.post("/auth/otp", "10.0.0.1", map[string]string{"phone": phone, "lang": "th", "turnstileToken": "x"})
	if w.Code != 202 {
		t.Fatalf("otp request: %d %s", w.Code, w.Body)
	}
	return e.lastOTP(t)
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func parseUnverified(tok string, claims map[string]any) (*jwt.Token, error) {
	t, _, err := jwt.NewParser().ParseUnverified(tok, jwt.MapClaims(claims))
	return t, err
}
