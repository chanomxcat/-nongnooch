package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func siteverify(a *App, token string) string {
	mux := http.NewServeMux()
	a.Routes(mux)
	req := httptest.NewRequest("POST", "/turnstile/v0/siteverify", strings.NewReader(url.Values{"secret": {"s"}, "response": {token}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return strings.TrimSpace(rec.Body.String())
}

func TestTurnstile(t *testing.T) {
	a := newTestApp()
	if got := siteverify(a, "mock-ok"); got != `{"success":true}` {
		t.Fatal(got)
	}
	if got := siteverify(a, "mock-fail"); got != `{"error-codes":["invalid-input-response"],"success":false}` {
		t.Fatal(got)
	}
	var slept time.Duration
	a.Sleep = func(d time.Duration) { slept = d }
	if got := siteverify(a, "mock-timeout"); got != `{"success":true}` || slept != 5*time.Second {
		t.Fatal(got, slept)
	}
}
