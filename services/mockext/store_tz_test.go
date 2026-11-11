package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConsoleShowsBangkokTime(t *testing.T) {
	a := newTestApp()
	at := time.Date(2026, 11, 12, 4, 5, 6, 0, time.UTC)
	a.Store.Add("sms", Entry{At: at, Title: "+66811111111", Body: "OTP"})
	if got := a.Store.List("sms")[0].At; !got.Equal(at) || got.Location() != time.UTC {
		t.Fatalf("stored time must stay UTC: %v", got)
	}
	mux := http.NewServeMux()
	a.Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if body := rec.Body.String(); !strings.Contains(body, "11:05:06") || strings.Contains(body, "04:05:06") {
		t.Fatal(body)
	}
}
