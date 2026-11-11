package main

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nongnooch/platform/pkg/chassis/authn"
)

var langs = map[string]bool{"th": true, "en": true, "zh": true, "ja": true, "ko": true, "de": true, "fr": true}
var deviceRe = regexp.MustCompile(`[^A-Za-z0-9-]`)

// Gateway routes /api/<svc>/* to http://<svc>:8080/* after verifying the JWT.
type Gateway struct {
	JWKS     *JWKS
	Services map[string]bool
	Upstream func(svc string) *url.URL // default http://<svc>:8080
	Trusted  *Trusted
	Origins  map[string]bool
	Now      func() time.Time

	mu      sync.Mutex
	proxies map[string]*httputil.ReverseProxy
}

func writeErr(w http.ResponseWriter, code int, e, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": e, "message": msg})
}

func (g *Gateway) proxy(svc string) *httputil.ReverseProxy {
	g.mu.Lock()
	defer g.mu.Unlock()
	if p, ok := g.proxies[svc]; ok {
		return p
	}
	target := &url.URL{Scheme: "http", Host: svc + ":8080"}
	if g.Upstream != nil {
		target = g.Upstream(svc)
	}
	p := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) { pr.SetURL(target); pr.Out.Host = target.Host },
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			writeErr(w, 502, "bad_gateway", "upstream unavailable")
		},
	}
	if g.proxies == nil {
		g.proxies = map[string]*httputil.ReverseProxy{}
	}
	g.proxies[svc] = p
	return p
}

// cors sets CORS headers for allowed origins and answers preflight; true means the request is done.
func (g *Gateway) cors(w http.ResponseWriter, r *http.Request) bool {
	if o := r.Header.Get("Origin"); o != "" && g.Origins[o] {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", o)
		h.Add("Vary", "Origin")
		h.Set("Access-Control-Expose-Headers", "X-Request-Id")
		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key,X-Device-Id,X-Queue-Token,Accept-Language")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return true
		}
	}
	return false
}

// publicLenient: identity login/JWKS routes ignore a stale or broken Authorization header
// instead of answering 401, so a client holding an expired token can still log in again.
func publicLenient(rest string) bool {
	return strings.HasPrefix(rest, "/auth/") || strings.HasPrefix(rest, "/.well-known/")
}

func lang(claim, accept string) string {
	if langs[claim] {
		return claim
	}
	for _, part := range strings.Split(accept, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		tag = strings.SplitN(tag, "-", 2)[0]
		if langs[tag] {
			return tag
		}
	}
	return "en"
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.cors(w, r) {
		return
	}
	clean := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if !strings.HasPrefix(clean, "/api/") {
		writeErr(w, 404, "not_found", "unknown route")
		return
	}
	seg := strings.SplitN(strings.TrimPrefix(clean, "/api/"), "/", 2)
	svc := seg[0]
	if !g.Services[svc] {
		writeErr(w, 404, "not_found", "unknown route")
		return
	}
	rest := "/"
	if len(seg) == 2 {
		rest += seg[1]
	}
	for _, s := range strings.Split(rest, "/") {
		if strings.EqualFold(s, "internal") {
			writeErr(w, 404, "not_found", "unknown route")
			return
		}
	}

	// Identity headers are owned by the gateway: drop whatever the client sent (X-Device-Id is sanitized).
	device := deviceRe.ReplaceAllString(r.Header.Get(authn.HeaderDeviceID), "")
	if len(device) > 64 {
		device = device[:64]
	}
	for _, h := range authn.IdentityHeaders {
		r.Header.Del(h)
	}

	var p authn.Principal
	if auth := r.Header.Get("Authorization"); auth != "" {
		now := time.Now()
		if g.Now != nil {
			now = g.Now()
		}
		tok, ok := strings.CutPrefix(auth, "Bearer ")
		var err error
		if !ok {
			err = errors.New("not a bearer token")
		} else {
			p, err = VerifyToken(r.Context(), g.JWKS, strings.TrimSpace(tok), now)
		}
		if err != nil {
			if !(svc == "identity" && publicLenient(rest)) {
				writeErr(w, 401, "invalid_token", "token invalid or expired")
				return
			}
			p = authn.Principal{}
			r.Header.Del("Authorization")
		}
	}
	p.Lang = lang(p.Lang, r.Header.Get("Accept-Language"))
	p.DeviceID = device
	p.RequestID = uuid.NewString()
	p.ClientIP = g.Trusted.ClientIP(r)
	for k, v := range p.Headers() {
		r.Header[k] = v
	}
	w.Header().Set(authn.HeaderRequestID, p.RequestID)

	r.URL.Path, r.URL.RawPath = rest, ""
	g.proxy(svc).ServeHTTP(w, r)
}

// Trusted decides which peers may supply X-Forwarded-For. Entries are IPs, CIDRs or host names;
// host names are re-resolved at most every 30s (compose container IPs change across restarts).
type Trusted struct {
	Entries []string
	Lookup  func(string) ([]string, error) // default net.LookupHost

	mu    sync.Mutex
	at    time.Time
	nets  []*net.IPNet
	addrs map[string]bool
}

func (t *Trusted) refresh() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.addrs != nil && time.Since(t.at) < 30*time.Second {
		return
	}
	lookup := t.Lookup
	if lookup == nil {
		lookup = net.LookupHost
	}
	t.at, t.addrs, t.nets = time.Now(), map[string]bool{}, nil
	for _, e := range t.Entries {
		e = strings.TrimSpace(e)
		if _, n, err := net.ParseCIDR(e); err == nil {
			t.nets = append(t.nets, n)
		} else if ip := net.ParseIP(e); ip != nil {
			t.addrs[ip.String()] = true
		} else if e != "" {
			if ips, err := lookup(e); err == nil {
				for _, a := range ips {
					t.addrs[a] = true
				}
			}
		}
	}
}

func (t *Trusted) is(ip string) bool {
	t.refresh()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.addrs[ip] {
		return true
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		for _, n := range t.nets {
			if n.Contains(parsed) {
				return true
			}
		}
	}
	return false
}

// ClientIP is the peer address or, when the peer is trusted, the right-most X-Forwarded-For entry that is not trusted.
func (t *Trusted) ClientIP(r *http.Request) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if t == nil || !t.is(peer) {
		return peer
	}
	parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if net.ParseIP(ip) != nil && !t.is(ip) {
			return ip
		}
	}
	return peer
}
