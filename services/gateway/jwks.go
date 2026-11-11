package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"nongnooch/platform/pkg/chassis/authn"
)

// JWKS caches identity's Ed25519 signing keys.
type JWKS struct {
	URL        string
	Client     *http.Client
	Refresh    time.Duration // periodic refresh (default 10m)
	MinRefetch time.Duration // min gap between refetches triggered by an unknown kid (default 30s)

	mu      sync.Mutex
	keys    map[string]ed25519.PublicKey
	fetched time.Time
}

func (k *JWKS) client() *http.Client {
	if k.Client != nil {
		return k.Client
	}
	return &http.Client{Timeout: 5 * time.Second}
}

func (k *JWKS) fetch(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", k.URL, nil)
	resp, err := k.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("jwks: status %d", resp.StatusCode)
	}
	var doc struct {
		Keys []struct{ Kty, Crv, Kid, X string } `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	keys := map[string]ed25519.PublicKey{}
	for _, j := range doc.Keys {
		if j.Kty != "OKP" || j.Crv != "Ed25519" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(j.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		keys[j.Kid] = ed25519.PublicKey(raw)
	}
	k.mu.Lock()
	k.keys, k.fetched = keys, time.Now()
	k.mu.Unlock()
	return nil
}

// Start loads keys (retrying every 2s until success) and refreshes them periodically. It does not block.
func (k *JWKS) Start(ctx context.Context) {
	go func() {
		for k.fetch(ctx) != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
		slog.Info("jwks loaded")
		every := k.Refresh
		if every == 0 {
			every = 10 * time.Minute
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(every):
				if err := k.fetch(ctx); err != nil {
					slog.Warn("jwks refresh failed, keeping old keys", "err", err)
				}
			}
		}
	}()
}

func (k *JWKS) Ready() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.keys) > 0
}

// Key returns the key for kid, refetching (at most once per MinRefetch) when kid is unknown.
func (k *JWKS) Key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if pk, ok := k.keys[kid]; ok {
		return pk, nil
	}
	gap := k.MinRefetch
	if gap == 0 {
		gap = 30 * time.Second
	}
	if time.Since(k.fetched) >= gap {
		k.fetched = time.Now() // claim the slot so concurrent unknown kids do not stampede
		k.mu.Unlock()
		_ = k.fetch(ctx)
		k.mu.Lock()
		if pk, ok := k.keys[kid]; ok {
			return pk, nil
		}
	}
	return nil, errors.New("unknown kid")
}

var staffRoles = map[string]bool{"admin": true, "editor": true, "approver": true, "finance": true, "gate": true, "agent_manager": true, "viewer": true}

// VerifyToken validates raw and maps its claims to a Principal.
func VerifyToken(ctx context.Context, k *JWKS, raw string, now time.Time) (authn.Principal, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return k.Key(ctx, kid)
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer("nongnooch-identity"), jwt.WithAudience("nongnooch-api"),
		jwt.WithLeeway(30*time.Second), jwt.WithExpirationRequired(), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil {
		return authn.Principal{}, err
	}
	str := func(k string) string { s, _ := claims[k].(string); return s }
	p := authn.Principal{Type: str("typ"), Sub: str("sub")}
	switch p.Type {
	case "user":
		p.MemberID, p.Lang = str("mid"), str("lang")
	case "staff":
		if rs, ok := claims["roles"].([]any); ok {
			for _, r := range rs {
				if s, _ := r.(string); staffRoles[s] {
					p.Roles = append(p.Roles, s)
				}
			}
		}
	default:
		return authn.Principal{}, errors.New("unknown typ")
	}
	if p.Sub == "" {
		return authn.Principal{}, errors.New("missing sub")
	}
	return p, nil
}
