// Package authn exposes the caller identity that the gateway injects as headers.
// Services trust these headers because they are reachable only through the gateway.
package authn

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
)

const (
	HeaderUserType  = "X-User-Type"
	HeaderUserSub   = "X-User-Sub"
	HeaderMemberID  = "X-Member-Id"
	HeaderRoles     = "X-Roles"
	HeaderLang      = "X-Lang"
	HeaderRequestID = "X-Request-Id"
	HeaderClientIP  = "X-Client-IP"
	HeaderDeviceID  = "X-Device-Id"
)

// IdentityHeaders are every header the gateway owns.
var IdentityHeaders = []string{HeaderUserType, HeaderUserSub, HeaderMemberID, HeaderRoles, HeaderLang, HeaderRequestID, HeaderClientIP, HeaderDeviceID}

// Principal is the caller. Type is "user", "staff" or "" (anonymous).
type Principal struct {
	Type, Sub, MemberID string
	Roles               []string
	Lang, ClientIP, DeviceID, RequestID string
}

func (p Principal) HasAnyRole(roles ...string) bool {
	for _, r := range roles {
		if slices.Contains(p.Roles, r) {
			return true
		}
	}
	return false
}

// FromHeaders reads a Principal from gateway headers.
func FromHeaders(h http.Header) Principal {
	p := Principal{
		Type: h.Get(HeaderUserType), Sub: h.Get(HeaderUserSub), MemberID: h.Get(HeaderMemberID),
		Lang: h.Get(HeaderLang), ClientIP: h.Get(HeaderClientIP), DeviceID: h.Get(HeaderDeviceID), RequestID: h.Get(HeaderRequestID),
	}
	for _, r := range strings.Split(h.Get(HeaderRoles), ",") {
		if r = strings.TrimSpace(r); r != "" {
			p.Roles = append(p.Roles, r)
		}
	}
	return p
}

// Headers renders the Principal as headers (empty values omitted).
func (p Principal) Headers() http.Header {
	h := http.Header{}
	set := func(k, v string) {
		if v != "" {
			h.Set(k, v)
		}
	}
	set(HeaderUserType, p.Type)
	set(HeaderUserSub, p.Sub)
	set(HeaderMemberID, p.MemberID)
	set(HeaderRoles, strings.Join(p.Roles, ","))
	set(HeaderLang, p.Lang)
	set(HeaderClientIP, p.ClientIP)
	set(HeaderDeviceID, p.DeviceID)
	set(HeaderRequestID, p.RequestID)
	return h
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the Principal in ctx (zero value when absent).
func From(ctx context.Context) Principal {
	p, _ := ctx.Value(ctxKey{}).(Principal)
	return p
}

// Middleware reads the identity headers into the request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), FromHeaders(r.Header))))
	})
}

func principal(r *http.Request) (Principal, *http.Request) {
	if p, ok := r.Context().Value(ctxKey{}).(Principal); ok {
		return p, r
	}
	p := FromHeaders(r.Header)
	return p, r.WithContext(WithPrincipal(r.Context(), p))
}

func fail(w http.ResponseWriter, code int, e, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": e, "message": msg})
}

// Require allows only callers whose Type is in types (anonymous -> 401, other -> 403).
func Require(types ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, r := principal(r)
			if p.Type == "" {
				fail(w, 401, "unauthenticated", "login required")
				return
			}
			if !slices.Contains(types, p.Type) {
				fail(w, 403, "forbidden", "caller type not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole allows only staff holding at least one of roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, r := principal(r)
			if p.Type != "staff" {
				fail(w, 401, "unauthenticated", "staff login required")
				return
			}
			if !p.HasAnyRole(roles...) {
				fail(w, 403, "forbidden", "role not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
