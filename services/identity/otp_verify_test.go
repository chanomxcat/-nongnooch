//go:build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func verifyBody(id, code string) map[string]string {
	return map[string]string{"requestId": id, "otp": code}
}

func wrong(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func TestVerifyCorrectCode(t *testing.T) {
	e := newEnv(t)
	e.tk.Members = fakeMembers{mid: "m-42"}
	id, code := e.newOTP(t, "0812345678")
	w := e.post("/auth/verify", "", verifyBody(id, code))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var tk Tokens
	_ = json.Unmarshal(w.Body.Bytes(), &tk)
	if tk.Refresh == "" || tk.ExpiresIn != 900 {
		t.Fatalf("%+v", tk)
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tk.Access, claims, func(*jwt.Token) (any, error) { return e.pub, nil },
		jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithAudience("nongnooch-api"), jwt.WithIssuer("nongnooch-identity"),
		jwt.WithTimeFunc(e.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header["kid"] != e.tk.KID() {
		t.Fatalf("kid %v", parsed.Header["kid"])
	}
	if claims["sub"] != "+66812345678" || claims["typ"] != "user" || claims["ctype"] != "phone" ||
		claims["mid"] != "m-42" || claims["lang"] != "th" || claims["verified"] != true {
		t.Fatalf("claims %v", claims)
	}
}

func TestVerifyMidEmptyWithStubLookup(t *testing.T) {
	e := newEnv(t)
	id, code := e.newOTP(t, "0812345678")
	w := e.post("/auth/verify", "", verifyBody(id, code))
	var tk Tokens
	_ = json.Unmarshal(w.Body.Bytes(), &tk)
	claims := jwt.MapClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(tk.Access, claims)
	if err != nil || claims["mid"] != "" {
		t.Fatalf("%v %v", err, claims)
	}
}

func TestVerifyWrongCodeDecrementsAttempts(t *testing.T) {
	e := newEnv(t)
	id, code := e.newOTP(t, "0812345678")
	for _, left := range []float64{4, 3, 2, 1} {
		w := e.post("/auth/verify", "", verifyBody(id, wrong(code)))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		if w.Code != 400 || out["error"] != "otp_invalid" || out["attemptsLeft"] != left {
			t.Fatalf("want left %v: %d %s", left, w.Code, w.Body)
		}
	}
	if w := e.post("/auth/verify", "", verifyBody(id, wrong(code))); w.Code != 423 {
		t.Fatalf("5th: %d", w.Code)
	}
}

func TestLockedEvenWithRightCode(t *testing.T) {
	e := newEnv(t)
	id, code := e.newOTP(t, "0812345678")
	for i := 0; i < 5; i++ {
		e.post("/auth/verify", "", verifyBody(id, wrong(code)))
	}
	if w := e.post("/auth/verify", "", verifyBody(id, code)); w.Code != 423 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestExpiredAfter5Min(t *testing.T) {
	e := newEnv(t)
	id, code := e.newOTP(t, "0812345678")
	e.clock.Advance(5*time.Minute + time.Second)
	if w := e.post("/auth/verify", "", verifyBody(id, code)); w.Code != 410 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestCodeSingleUse(t *testing.T) {
	e := newEnv(t)
	id, code := e.newOTP(t, "0812345678")
	if w := e.post("/auth/verify", "", verifyBody(id, code)); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := e.post("/auth/verify", "", verifyBody(id, code)); w.Code != 409 {
		t.Fatalf("%d", w.Code)
	}
}

func TestUnknownRequestIDInvalid(t *testing.T) {
	e := newEnv(t)
	if w := e.post("/auth/verify", "", verifyBody("nope", "123456")); w.Code != 400 {
		t.Fatalf("%d", w.Code)
	}
}

func TestConcurrentVerifyOnlyOneWins(t *testing.T) {
	e := newEnv(t)
	id, code := e.newOTP(t, "0812345678")
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, used := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.otp.Verify(context.Background(), id, code)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrOTPUsed):
				used++
			default:
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || used != 9 {
		t.Fatalf("ok=%d used=%d", ok, used)
	}
}
