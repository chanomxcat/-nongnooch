package authn_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"nongnooch/platform/pkg/chassis/authn"
)

func TestFromHeadersRoundTrip(t *testing.T) {
	p := authn.Principal{Type: "staff", Sub: "s1", Roles: []string{"editor", "viewer"}, Lang: "th", ClientIP: "1.2.3.4", DeviceID: "d1", RequestID: "r1"}
	h := p.Headers()
	if h.Get("X-Roles") != "editor,viewer" {
		t.Fatalf("X-Roles = %q", h.Get("X-Roles"))
	}
	if got := authn.FromHeaders(h); !reflect.DeepEqual(got, p) {
		t.Fatalf("got %+v want %+v", got, p)
	}
}

func TestRolesTrimAndDropEmpty(t *testing.T) {
	h := http.Header{}
	h.Set("X-Roles", "editor, ,viewer,")
	if got := authn.FromHeaders(h).Roles; !reflect.DeepEqual(got, []string{"editor", "viewer"}) {
		t.Fatalf("roles = %v", got)
	}
}

func do(mw func(http.Handler) http.Handler, h http.Header) *httptest.ResponseRecorder {
	var sub string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sub = authn.From(r.Context()).Sub; w.Header().Set("X-Sub", sub) })
	r := httptest.NewRequest("GET", "/", nil)
	for k, v := range h {
		r.Header[k] = v
	}
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)
	return w
}

func TestRequireAnonymous401(t *testing.T) {
	if w := do(authn.Require("user"), nil); w.Code != 401 || !strings.Contains(w.Body.String(), `"unauthenticated"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestRequireWrongType403(t *testing.T) {
	h := authn.Principal{Type: "staff", Sub: "s"}.Headers()
	if w := do(authn.Require("user"), h); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := do(authn.Require("user", "staff"), h); w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestRequireRoleUserType401(t *testing.T) {
	h := authn.Principal{Type: "user", Sub: "u", Roles: []string{"admin"}}.Headers()
	if w := do(authn.RequireRole("admin"), h); w.Code != 401 {
		t.Fatal(w.Code)
	}
}

func TestRequireRoleMissing403(t *testing.T) {
	h := authn.Principal{Type: "staff", Sub: "s", Roles: []string{"viewer"}}.Headers()
	if w := do(authn.RequireRole("approver", "admin"), h); w.Code != 403 {
		t.Fatal(w.Code)
	}
}

func TestRequireRoleOK(t *testing.T) {
	h := authn.Principal{Type: "staff", Sub: "s9", Roles: []string{"admin"}}.Headers()
	w := do(authn.RequireRole("approver", "admin"), h)
	if w.Code != 200 || w.Header().Get("X-Sub") != "s9" {
		t.Fatal(w.Code, w.Header())
	}
}

func TestErrorBodyJSON(t *testing.T) {
	w := do(authn.Require("user"), authn.Principal{Type: "staff"}.Headers())
	if w.Header().Get("Content-Type") != "application/json" || !strings.Contains(w.Body.String(), `"error":"forbidden"`) || !strings.Contains(w.Body.String(), `"message"`) {
		t.Fatalf("%s %s", w.Header(), w.Body)
	}
}
