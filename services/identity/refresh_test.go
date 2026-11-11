//go:build integration

package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func login(t *testing.T, e *env) Tokens {
	t.Helper()
	id, code := e.newOTP(t, "0812345678")
	w := e.post("/auth/verify", "", verifyBody(id, code))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var tk Tokens
	_ = json.Unmarshal(w.Body.Bytes(), &tk)
	return tk
}

func refreshReq(e *env, rt string) *httptest.ResponseRecorder {
	return e.post("/auth/refresh", "", map[string]string{"refreshToken": rt})
}

func TestRefreshRotatesAndReuseRevokesAll(t *testing.T) {
	e := newEnv(t)
	t1 := login(t, e)
	w := refreshReq(e, t1.Refresh)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var t2 Tokens
	_ = json.Unmarshal(w.Body.Bytes(), &t2)
	if t2.Refresh == "" || t2.Refresh == t1.Refresh {
		t.Fatal("not rotated")
	}
	if w := refreshReq(e, t1.Refresh); w.Code != 401 || !contains(w.Body.String(), "refresh_reused") {
		t.Fatalf("reuse: %d %s", w.Code, w.Body)
	}
	if w := refreshReq(e, t2.Refresh); w.Code != 401 {
		t.Fatalf("new token after reuse: %d", w.Code)
	}
}

func TestRefreshExpiresAfter30Days(t *testing.T) {
	e := newEnv(t)
	t1 := login(t, e)
	e.clock.Advance(30*24*time.Hour + time.Minute)
	if w := refreshReq(e, t1.Refresh); w.Code != 401 {
		t.Fatalf("%d", w.Code)
	}
}

func TestLogoutThenRefreshFails(t *testing.T) {
	e := newEnv(t)
	t1 := login(t, e)
	if w := e.post("/auth/logout", "", map[string]string{"refreshToken": t1.Refresh}); w.Code != 204 {
		t.Fatalf("%d", w.Code)
	}
	if w := refreshReq(e, t1.Refresh); w.Code != 401 {
		t.Fatalf("%d", w.Code)
	}
}

func TestRefreshUnknownToken(t *testing.T) {
	e := newEnv(t)
	if w := refreshReq(e, "garbage"); w.Code != 401 {
		t.Fatalf("%d", w.Code)
	}
}

func TestRefreshFillsMid(t *testing.T) {
	e := newEnv(t)
	t1 := login(t, e)
	e.tk.Members = fakeMembers{mid: "m-7"}
	w := refreshReq(e, t1.Refresh)
	var t2 Tokens
	_ = json.Unmarshal(w.Body.Bytes(), &t2)
	claims := map[string]any{}
	if _, err := parseUnverified(t2.Access, claims); err != nil || claims["mid"] != "m-7" || claims["lang"] != "th" {
		t.Fatalf("%v %v", err, claims)
	}
}

func TestJWKSMatchesKid(t *testing.T) {
	e := newEnv(t)
	t1 := login(t, e)
	w := e.do("GET", "/.well-known/jwks.json", "", nil)
	var jw struct{ Keys []map[string]string }
	_ = json.Unmarshal(w.Body.Bytes(), &jw)
	claims := map[string]any{}
	tok, err := parseUnverified(t1.Access, claims)
	if err != nil || len(jw.Keys) != 1 || jw.Keys[0]["kid"] != tok.Header["kid"] || jw.Keys[0]["kty"] != "OKP" || jw.Keys[0]["crv"] != "Ed25519" {
		t.Fatalf("%v %+v", err, jw)
	}
}

func TestSeedStub(t *testing.T) {
	e := newEnv(t)
	if w := e.post("/internal/seed", "", nil); w.Code != 204 {
		t.Fatalf("%d", w.Code)
	}
}
