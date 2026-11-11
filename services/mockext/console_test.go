package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestApp() *App {
	return &App{
		Store: NewStore(), PublicURL: "http://localhost:8090", APIKey: "demo-key", Secret: "demo-secret",
		Client: http.DefaultClient, Sleep: func(time.Duration) {}, Sync: true,
		payments: map[string]*Payment{},
	}
}

func TestConsoleShowsOTP(t *testing.T) {
	a := newTestApp()
	a.Store.Add("sms", Entry{Title: "+66811111111", Body: "รหัส OTP 123456"})
	mux := http.NewServeMux()
	a.Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatal(rec.Code, rec.Header())
	}
	if !strings.Contains(rec.Body.String(), "รหัส OTP 123456") {
		t.Fatal(rec.Body.String())
	}
}
