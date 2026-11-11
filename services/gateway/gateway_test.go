package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type env_ struct {
	g        *Gateway
	gw       *httptest.Server
	priv     ed25519.PrivateKey
	last     atomic.Pointer[http.Request]
	jwksHits atomic.Int32
	now      time.Time
}

func jwkDoc(kid string, pub ed25519.PublicKey) map[string]any {
	return map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "kid": kid, "x": base64.RawURLEncoding.EncodeToString(pub)}}}
}

func setup(t *testing.T) *env_ {
	t.Helper()
	e := &env_{now: time.Now()}
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	e.priv = priv
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.jwksHits.Add(1)
		_ = json.NewEncoder(w).Encode(jwkDoc("k1", pub))
	}))
	t.Cleanup(idp.Close)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.last.Store(r.Clone(r.Context()))
		w.Header().Set("X-Seen-Path", r.URL.Path)
		w.WriteHeader(200)
	}))
	t.Cleanup(up.Close)
	u, _ := url.Parse(up.URL)
	jw := &JWKS{URL: idp.URL}
	if err := jw.fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.g = &Gateway{
		JWKS:     jw,
		Services: set("identity,member,commerce"),
		Upstream: func(string) *url.URL { return u },
		Trusted:  &Trusted{Entries: []string{"10.0.0.0/8"}},
		Origins:  set("http://localhost:3000"),
		Now:      func() time.Time { return e.now },
	}
	e.gw = httptest.NewServer(e.g)
	t.Cleanup(e.gw.Close)
	return e
}

func (e *env_) token(typ string, mut func(jwt.MapClaims)) string {
	c := jwt.MapClaims{"typ": typ, "sub": "+66812345678", "mid": "m-1", "lang": "zh", "iss": "nongnooch-identity", "aud": "nongnooch-api",
		"exp": e.now.Add(15 * time.Minute).Unix(), "iat": e.now.Unix()}
	if typ == "staff" {
		c["sub"], c["roles"] = "s-1", []string{"editor", "approver", "root"}
	}
	if mut != nil {
		mut(c)
	}
	tk := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c)
	tk.Header["kid"] = "k1"
	s, _ := tk.SignedString(e.priv)
	return s
}

func (e *env_) do(t *testing.T, method, path string, h map[string]string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, e.gw.URL+path, nil)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func (e *env_) errCode(t *testing.T, path string, h map[string]string) (int, string) {
	req, _ := http.NewRequest("GET", e.gw.URL+path, nil)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var b map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&b)
	return resp.StatusCode, b["error"]
}

func TestValidUserTokenHeaders(t *testing.T) {
	e := setup(t)
	resp := e.do(t, "GET", "/api/member/me?x=1", map[string]string{"Authorization": "Bearer " + e.token("user", nil), "X-Device-Id": "dev-1"})
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	r := e.last.Load()
	want := map[string]string{"X-User-Type": "user", "X-User-Sub": "+66812345678", "X-Member-Id": "m-1", "X-Lang": "zh", "X-Device-Id": "dev-1"}
	for k, v := range want {
		if r.Header.Get(k) != v {
			t.Errorf("%s = %q want %q", k, r.Header.Get(k), v)
		}
	}
	if r.Header.Get("X-Request-Id") == "" || r.Header.Get("X-Client-IP") != "127.0.0.1" {
		t.Errorf("request id %q client ip %q", r.Header.Get("X-Request-Id"), r.Header.Get("X-Client-IP"))
	}
	if resp.Header.Get("X-Request-Id") != r.Header.Get("X-Request-Id") {
		t.Error("response request id mismatch")
	}
	if resp.Header.Get("X-Seen-Path") != "/me" || r.URL.RawQuery != "x=1" {
		t.Errorf("path %q query %q", resp.Header.Get("X-Seen-Path"), r.URL.RawQuery)
	}
}

func TestValidStaffTokenFiltersRoles(t *testing.T) {
	e := setup(t)
	e.do(t, "GET", "/api/member/x", map[string]string{"Authorization": "Bearer " + e.token("staff", nil)})
	r := e.last.Load()
	if r.Header.Get("X-User-Type") != "staff" || r.Header.Get("X-Roles") != "editor,approver" || r.Header.Get("X-User-Sub") != "s-1" {
		t.Fatalf("%v", r.Header)
	}
}

func TestSpoofedHeadersStripped(t *testing.T) {
	e := setup(t)
	e.do(t, "GET", "/api/member/x", map[string]string{"X-Roles": "admin", "X-User-Type": "staff", "X-Member-Id": "m1", "X-User-Sub": "evil", "X-Client-IP": "6.6.6.6", "X-Request-Id": "spoof"})
	r := e.last.Load()
	for _, h := range []string{"X-Roles", "X-User-Type", "X-Member-Id", "X-User-Sub"} {
		if v := r.Header.Get(h); v != "" {
			t.Errorf("%s leaked: %q", h, v)
		}
	}
	if r.Header.Get("X-Client-IP") == "6.6.6.6" || r.Header.Get("X-Request-Id") == "spoof" {
		t.Error("client ip / request id not overwritten")
	}
}

func TestDeviceIDSanitized(t *testing.T) {
	e := setup(t)
	for in, want := range map[string]string{"dev-123": "dev-123", "dev_1.2": "dev12", strings.Repeat("a", 500): strings.Repeat("a", 64), "!!!": ""} {
		e.do(t, "GET", "/api/member/x", map[string]string{"X-Device-Id": in})
		if got := e.last.Load().Header.Get("X-Device-Id"); got != want {
			t.Errorf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestInternalBlocked(t *testing.T) {
	e := setup(t)
	e.last.Store(nil)
	for _, p := range []string{"/api/member/internal/seed", "/api/member/internal", "/api/member/a/../internal/x", "/api/member/%69nternal/x", "/api/member/Internal/x"} {
		if c := e.do(t, "POST", p, nil).StatusCode; c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
	if e.last.Load() != nil {
		t.Error("request reached upstream")
	}
}

func TestUnknownServiceAndPath(t *testing.T) {
	e := setup(t)
	for _, p := range []string{"/api/nope/x", "/api/", "/other"} {
		if c := e.do(t, "GET", p, nil).StatusCode; c != 404 {
			t.Errorf("%s -> %d", p, c)
		}
	}
}

func TestInvalidTokens401(t *testing.T) {
	e := setup(t)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	forged := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"typ": "staff", "sub": "x", "iss": "nongnooch-identity", "aud": "nongnooch-api", "exp": e.now.Add(time.Hour).Unix()})
	forged.Header["kid"] = "k1"
	forgedS, _ := forged.SignedString(otherPriv)
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"typ": "staff", "sub": "x", "exp": e.now.Add(time.Hour).Unix()}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	cases := map[string]string{
		"expired":    e.token("user", func(c jwt.MapClaims) { c["exp"] = e.now.Add(-time.Minute).Unix() }),
		"badaud":     e.token("user", func(c jwt.MapClaims) { c["aud"] = "other" }),
		"badtyp":     e.token("service", nil),
		"noexp":      e.token("user", func(c jwt.MapClaims) { delete(c, "exp") }),
		"forged":     forgedS,
		"alg none":   none,
		"garbage":    "not-a-jwt",
		"unknownkid": strings.Replace(e.token("user", nil), "", "", 1),
	}
	// unknown kid: re-sign with a kid the JWKS does not have
	tk := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"typ": "user", "sub": "x", "iss": "nongnooch-identity", "aud": "nongnooch-api", "exp": e.now.Add(time.Hour).Unix()})
	tk.Header["kid"] = "zzz"
	cases["unknownkid"], _ = tk.SignedString(e.priv)
	for name, tok := range cases {
		if c, code := e.errCode(t, "/api/member/x", map[string]string{"Authorization": "Bearer " + tok}); c != 401 || code != "invalid_token" {
			t.Errorf("%s -> %d %s", name, c, code)
		}
	}
	if c, code := e.errCode(t, "/api/member/x", map[string]string{"Authorization": "Basic abc"}); c != 401 || code != "invalid_token" {
		t.Errorf("basic -> %d %s", c, code)
	}
	// within leeway still valid
	if c := e.do(t, "GET", "/api/member/x", map[string]string{"Authorization": "Bearer " + e.token("user", func(c jwt.MapClaims) { c["exp"] = e.now.Add(-29 * time.Second).Unix() })}).StatusCode; c != 200 {
		t.Errorf("leeway -> %d", c)
	}
}

func TestPublicRoutesPassAnonymously(t *testing.T) {
	e := setup(t)
	for _, p := range []string{"/api/identity/.well-known/jwks.json", "/api/identity/auth/otp", "/api/commerce/products"} {
		if c := e.do(t, "GET", p, map[string]string{"Accept-Language": "ja-JP,en;q=0.5"}).StatusCode; c != 200 {
			t.Errorf("%s -> %d", p, c)
		}
		r := e.last.Load()
		if r.Header.Get("X-User-Type") != "" || r.Header.Get("X-Lang") != "ja" {
			t.Errorf("%s: type %q lang %q", p, r.Header.Get("X-User-Type"), r.Header.Get("X-Lang"))
		}
	}
	e.do(t, "GET", "/api/member/x", map[string]string{"Accept-Language": "es,xx"})
	if e.last.Load().Header.Get("X-Lang") != "en" {
		t.Error("fallback lang")
	}
}

func TestExpiredTokenOnIdentityAuthIsAnonymous(t *testing.T) {
	e := setup(t)
	exp := e.token("user", func(c jwt.MapClaims) { c["exp"] = e.now.Add(-time.Hour).Unix() })
	if c := e.do(t, "POST", "/api/identity/auth/refresh", map[string]string{"Authorization": "Bearer " + exp}).StatusCode; c != 200 {
		t.Fatalf("auth with stale token -> %d", c)
	}
	r := e.last.Load()
	if r.Header.Get("X-User-Type") != "" || r.Header.Get("Authorization") != "" {
		t.Error("stale identity should be dropped")
	}
}

func TestClientIPTrustedProxy(t *testing.T) {
	tr := &Trusted{Entries: []string{"10.0.0.0/8", "web"}, Lookup: func(h string) ([]string, error) { return []string{"172.20.0.5"}, nil }}
	mk := func(remote, xff string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	for _, c := range []struct{ remote, xff, want string }{
		{"203.0.113.9:1234", "6.6.6.6", "203.0.113.9"},                // untrusted peer: XFF ignored
		{"10.1.2.3:80", "1.2.3.4", "1.2.3.4"},                         // trusted CIDR
		{"172.20.0.5:80", "9.9.9.9, 1.2.3.4", "1.2.3.4"},              // resolved host name, right-most
		{"10.1.2.3:80", "1.2.3.4, 10.9.9.9", "1.2.3.4"},               // skip trusted hop on right
		{"10.1.2.3:80", "", "10.1.2.3"},                               // no XFF
		{"10.1.2.3:80", "garbage", "10.1.2.3"},
	} {
		if got := tr.ClientIP(mk(c.remote, c.xff)); got != c.want {
			t.Errorf("%v -> %s", c, got)
		}
	}
}

func TestCORS(t *testing.T) {
	e := setup(t)
	e.last.Store(nil)
	resp := e.do(t, "OPTIONS", "/api/member/x", map[string]string{"Origin": "http://localhost:3000", "Access-Control-Request-Method": "POST"})
	if resp.StatusCode != 204 || resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" || !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("%d %v", resp.StatusCode, resp.Header)
	}
	if e.last.Load() != nil {
		t.Error("preflight reached upstream")
	}
	resp = e.do(t, "GET", "/api/member/x", map[string]string{"Origin": "http://evil.example"})
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Error("evil origin allowed")
	}
}

func TestJWKSUnknownKidRefetchRateLimited(t *testing.T) {
	e := setup(t)
	e.g.JWKS.MinRefetch = time.Hour
	e.g.JWKS.fetched = time.Now().Add(-2 * time.Hour)
	before := e.jwksHits.Load()
	for i := 0; i < 50; i++ {
		_, _ = e.g.JWKS.Key(context.Background(), "rand"+string(rune('a'+i%26)))
	}
	if n := e.jwksHits.Load() - before; n != 1 {
		t.Fatalf("jwks refetched %d times", n)
	}
}
