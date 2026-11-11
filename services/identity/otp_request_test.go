//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func count(t *testing.T, e *env, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.d.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func otpBody(phone string) map[string]string {
	return map[string]string{"phone": phone, "lang": "th", "turnstileToken": "x"}
}

func TestRequestWritesRowAndOutboxInOneTx(t *testing.T) {
	e := newEnv(t)
	w := e.post("/auth/otp", "10.0.0.1", otpBody("0812345678"))
	if w.Code != 202 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if count(t, e, `SELECT COUNT(*) FROM otp_requests`) != 1 || count(t, e, `SELECT COUNT(*) FROM outbox WHERE type='otp.requested'`) != 1 {
		t.Fatal("expected 1 otp row and 1 outbox row")
	}
	id, code := e.lastOTP(t)
	var resp struct{ RequestID string }
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.RequestID != id || len(code) != 6 {
		t.Fatalf("id %q/%q code %q", resp.RequestID, id, code)
	}
	var contact string
	var hm []byte
	if err := e.d.QueryRow(`SELECT contact, code_hmac FROM otp_requests`).Scan(&contact, &hm); err != nil {
		t.Fatal(err)
	}
	if contact != "+66812345678" || string(hm) == code || len(hm) != 32 {
		t.Fatalf("contact %q hmac %x", contact, hm)
	}
}

func TestInvalidPhoneIs400(t *testing.T) {
	e := newEnv(t)
	w := e.post("/auth/otp", "10.0.0.1", otpBody("12345"))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_phone") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestTurnstileRejected(t *testing.T) {
	e := newEnv(t)
	e.ver.err = ErrTurnstileFailed
	w := e.post("/auth/otp", "10.0.0.1", otpBody("0812345678"))
	if w.Code != 403 || !strings.Contains(w.Body.String(), "turnstile_failed") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if count(t, e, `SELECT COUNT(*) FROM otp_requests`) != 0 {
		t.Fatal("row created")
	}
}

func TestTurnstileTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer slow.Close()
	e := newEnv(t)
	e.otp.Verifier = TurnstileVerifier{URL: slow.URL, Secret: "s", Client: &http.Client{Timeout: 100 * time.Millisecond}}
	w := e.post("/auth/otp", "10.0.0.1", otpBody("0812345678"))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "turnstile_unavailable") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if count(t, e, `SELECT COUNT(*) FROM otp_requests`) != 0 {
		t.Fatal("row created")
	}
}

func TestTurnstileVerifierSuccessAndFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		_, _ = w.Write([]byte(fmt.Sprintf(`{"success":%v}`, r.Form.Get("response") == "ok")))
	}))
	defer srv.Close()
	v := TurnstileVerifier{URL: srv.URL, Secret: "s"}
	if err := v.Verify(context.Background(), "ok", "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(context.Background(), "bad", "1.2.3.4"); err != ErrTurnstileFailed {
		t.Fatal(err)
	}
}

func TestResendCooldown60s(t *testing.T) {
	e := newEnv(t)
	if w := e.post("/auth/otp", "10.0.0.1", otpBody("0812345678")); w.Code != 202 {
		t.Fatal(w.Code)
	}
	w := e.post("/auth/otp", "10.0.0.1", otpBody("+66 81 234 5678"))
	ra, _ := strconv.Atoi(w.Header().Get("Retry-After"))
	if w.Code != 429 || ra < 1 || ra > 60 {
		t.Fatalf("%d retry-after %q", w.Code, w.Header().Get("Retry-After"))
	}
}

func TestPerContactLimit5PerHour(t *testing.T) {
	e := newEnv(t)
	for i := 1; i <= 5; i++ {
		if w := e.post("/auth/otp", "10.0.0.1", otpBody("0812345678")); w.Code != 202 {
			t.Fatalf("request %d: %d", i, w.Code)
		}
		e.clock.Advance(61 * time.Second)
	}
	if w := e.post("/auth/otp", "10.0.0.1", otpBody("0812345678")); w.Code != 429 {
		t.Fatalf("6th: %d", w.Code)
	}
}

func TestPerIPLimit20PerHour(t *testing.T) {
	e := newEnv(t)
	for i := 1; i <= 21; i++ {
		w := e.post("/auth/otp", "10.0.0.9", otpBody(fmt.Sprintf("08123%05d", i)))
		want := 202
		if i == 21 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("request %d: %d %s", i, w.Code, w.Body)
		}
	}
}

func TestUnknownLangFallsBackToEN(t *testing.T) {
	e := newEnv(t)
	w := e.post("/auth/otp", "10.0.0.1", map[string]string{"phone": "0812345678", "lang": "ru", "turnstileToken": "x"})
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	var raw string
	_ = e.d.QueryRow(`SELECT payload FROM outbox`).Scan(&raw)
	var ev struct{ Payload struct{ Lang string } }
	_ = json.Unmarshal([]byte(raw), &ev)
	if ev.Payload.Lang != "en" {
		t.Fatalf("lang %q", ev.Payload.Lang)
	}
}
