//go:build integration

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type staffTok struct {
	Access    string `json:"access"`
	ExpiresIn int    `json:"expiresIn"`
}

func seedStaff(t *testing.T, e *env) {
	t.Helper()
	if w := e.post("/internal/seed", "", nil); w.Code != 204 {
		t.Fatalf("seed: %d %s", w.Code, w.Body)
	}
}

func TestStaffLoginIssuesStaffToken(t *testing.T) {
	e := newEnv(t)
	seedStaff(t, e)
	w := e.post("/auth/staff/login", "", map[string]string{"email": "Gate@Demo.Nongnooch.local", "password": "Demo1234!"})
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var tk staffTok
	_ = json.Unmarshal(w.Body.Bytes(), &tk)
	if tk.ExpiresIn != 8*3600 {
		t.Fatalf("expiresIn %d", tk.ExpiresIn)
	}
	claims := map[string]any{}
	if _, err := parseUnverified(tk.Access, claims); err != nil {
		t.Fatal(err)
	}
	roles, _ := claims["roles"].([]any)
	if claims["typ"] != "staff" || len(roles) != 1 || roles[0] != "gate" || claims["name"] == "" || claims["sub"] == "" {
		t.Fatalf("claims %v", claims)
	}
	if exp := int64(claims["exp"].(float64)) - int64(claims["iat"].(float64)); exp != 8*3600 {
		t.Fatalf("exp-iat %d", exp)
	}
	if claims["iss"] != issuer || claims["aud"] != audience {
		t.Fatalf("iss/aud %v", claims)
	}
}

func TestStaffLoginBadCredentialsIndistinguishable(t *testing.T) {
	e := newEnv(t)
	seedStaff(t, e)
	a := e.post("/auth/staff/login", "", map[string]string{"email": "gate@demo.nongnooch.local", "password": "wrong"})
	b := e.post("/auth/staff/login", "", map[string]string{"email": "nobody@demo.nongnooch.local", "password": "Demo1234!"})
	if a.Code != 401 || b.Code != 401 || a.Body.String() != b.Body.String() || !contains(a.Body.String(), "invalid_credentials") {
		t.Fatalf("%d %s / %d %s", a.Code, a.Body, b.Code, b.Body)
	}
}

func TestStaffLoginInactive(t *testing.T) {
	e := newEnv(t)
	seedStaff(t, e)
	e.d.Exec(`UPDATE staff_users SET active = FALSE WHERE email = 'viewer@demo.nongnooch.local'`)
	w := e.post("/auth/staff/login", "", map[string]string{"email": "viewer@demo.nongnooch.local", "password": "Demo1234!"})
	if w.Code != 403 || !contains(w.Body.String(), "staff_inactive") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestStaffLoginRateLimitedPerEmail(t *testing.T) {
	e := newEnv(t)
	seedStaff(t, e)
	bad := map[string]string{"email": "admin@demo.nongnooch.local", "password": "nope"}
	for i := 0; i < 10; i++ {
		if w := e.post("/auth/staff/login", "", bad); w.Code != 401 {
			t.Fatalf("try %d: %d", i, w.Code)
		}
	}
	if w := e.post("/auth/staff/login", "", bad); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("11th: %d", w.Code)
	}
	// even the right password is blocked while locked; other emails are unaffected
	if w := e.post("/auth/staff/login", "", map[string]string{"email": "admin@demo.nongnooch.local", "password": "Demo1234!"}); w.Code != 429 {
		t.Fatalf("correct while locked: %d", w.Code)
	}
	if w := e.post("/auth/staff/login", "", map[string]string{"email": "editor@demo.nongnooch.local", "password": "Demo1234!"}); w.Code != 200 {
		t.Fatalf("other email: %d", w.Code)
	}
	e.clock.Advance(16 * time.Minute)
	if w := e.post("/auth/staff/login", "", map[string]string{"email": "admin@demo.nongnooch.local", "password": "Demo1234!"}); w.Code != 200 {
		t.Fatalf("after window: %d", w.Code)
	}
}

func TestStaffSeedIdempotentAllRoles(t *testing.T) {
	e := newEnv(t)
	seedStaff(t, e)
	seedStaff(t, e)
	var n int
	e.d.QueryRow(`SELECT COUNT(*) FROM staff_users`).Scan(&n)
	if n != 8 {
		t.Fatalf("staff rows %d", n)
	}
	for _, r := range []string{"admin", "editor", "approver", "finance", "gate", "agent_manager", "viewer", "approver2"} {
		w := e.post("/auth/staff/login", "", map[string]string{"email": r + "@demo.nongnooch.local", "password": "Demo1234!"})
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", r, w.Code, w.Body)
		}
	}
}

func TestOTPLimitsFromEnv(t *testing.T) {
	d := LoadOTPLimits(func(string) string { return "" })
	if d.Cooldown != 60*time.Second || d.PerContactHour != 5 || d.PerIPHour != 20 {
		t.Fatalf("defaults %+v", d)
	}
	env := map[string]string{"OTP_RESEND_COOLDOWN_SECONDS": "10", "OTP_MAX_PER_CONTACT_PER_HOUR": "100", "OTP_MAX_PER_IP_PER_HOUR": "1000"}
	l := LoadOTPLimits(func(k string) string { return env[k] })
	if l.Cooldown != 10*time.Second || l.PerContactHour != 100 || l.PerIPHour != 1000 {
		t.Fatalf("env %+v", l)
	}
}

func TestOTPRelaxedLimitsAllowRapidRequests(t *testing.T) {
	e := newEnv(t)
	e.otp.Limits = OTPLimits{Cooldown: 10 * time.Second, PerContactHour: 100, PerIPHour: 1000}
	e.newOTP(t, "0812345678")
	if w := e.post("/auth/otp", "10.0.0.1", map[string]string{"phone": "0812345678", "lang": "th", "turnstileToken": "x"}); w.Code != 429 {
		t.Fatalf("inside cooldown: %d", w.Code)
	}
	for i := 0; i < 10; i++ {
		e.clock.Advance(11 * time.Second)
		if w := e.post("/auth/otp", "10.0.0.1", map[string]string{"phone": "0812345678", "lang": "th", "turnstileToken": "x"}); w.Code != 202 {
			t.Fatalf("req %d: %d", i, w.Code)
		}
	}
}

func TestStaffLoginOversizedEmailIs401(t *testing.T) {
	e := newEnv(t)
	long := strings.Repeat("a", 300) + "@demo.nongnooch.local"
	if w := e.post("/auth/staff/login", "", map[string]string{"email": long, "password": "x"}); w.Code != 401 || !contains(w.Body.String(), "invalid_credentials") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := e.post("/auth/staff/login", "", map[string]string{"email": "not-an-email", "password": "x"}); w.Code != 401 {
		t.Fatalf("malformed: %d", w.Code)
	}
}

func TestStaffLoginFailsPurgedAfterWindow(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 5; i++ {
		e.post("/auth/staff/login", "", map[string]string{"email": "u" + string(rune('a'+i)) + "@x.io", "password": "x"})
	}
	var n int
	e.d.QueryRow(`SELECT COUNT(*) FROM staff_login_fails`).Scan(&n)
	if n != 5 {
		t.Fatalf("rows %d", n)
	}
	e.clock.Advance(16 * time.Minute)
	e.post("/auth/staff/login", "", map[string]string{"email": "z@x.io", "password": "x"})
	e.d.QueryRow(`SELECT COUNT(*) FROM staff_login_fails`).Scan(&n)
	if n != 1 {
		t.Fatalf("rows after window %d", n)
	}
}
