//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"nongnooch/platform/pkg/chassis/db"
	"nongnooch/platform/pkg/chassis/testutil"
)

type fakeInvites struct{ err error }

func (f fakeInvites) ValidateInvite(context.Context, string) (InviteResult, error) {
	return InviteResult{}, f.err
}

type env struct {
	d   *sql.DB
	svc *MemberService
	mux *http.ServeMux
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d, _ := testutil.DB(t)
	sub, _ := fs.Sub(migrationsFS, "migrations")
	if err := db.Migrate(testutil.Ctx(t), d, sub); err != nil {
		t.Fatal(err)
	}
	k := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	h := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("h", 32)))
	c, err := NewIDCipher(k, 1, h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO demo_invite_codes VALUES('DEMOGUIDE1','g-1','c-1')`); err != nil {
		t.Fatal(err)
	}
	svc := &MemberService{DB: d, Cipher: c, Invites: DBInviteValidator{DB: d}, WelcomePoints: 100, InvitePoints: 50}
	mux := http.NewServeMux()
	(&API{Svc: svc, DemoMode: true}).Register(mux)
	return &env{d: d, svc: svc, mux: mux}
}

// as sends a request as a logged-in user with the given contact ("" = anonymous).
func (e *env) as(contact, method, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if contact != "" {
		r.Header.Set("X-User-Type", "user")
		r.Header.Set("X-User-Sub", contact)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func reg(over map[string]any) map[string]any {
	m := map[string]any{
		"firstName": "Somchai", "lastName": "Jaidee", "nationality": "TH", "idNumber": "1101700230708",
		"lang": "th", "channel": "web", "consents": map[string]bool{"terms": true, "pdpa": true, "marketing": false},
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

func (e *env) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string               { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func captureLogs(t *testing.T) *syncBuf {
	buf := &syncBuf{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return buf
}
