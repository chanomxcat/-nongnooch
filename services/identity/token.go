package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 30 * 24 * time.Hour
	issuer     = "nongnooch-identity"
	audience   = "nongnooch-api"
)

var (
	ErrRefreshInvalid = errors.New("refresh_invalid")
	ErrRefreshReused  = errors.New("refresh_reused")
)

// MemberLookup resolves the memberId for a contact ("" when not a member yet).
// Task 5 replaces the stub with a gRPC client to the member service.
type MemberLookup interface {
	GetMemberIDByContact(ctx context.Context, contact string) (string, error)
}

type noMember struct{}

func (noMember) GetMemberIDByContact(context.Context, string) (string, error) { return "", nil }

type Tokens struct {
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
	ExpiresIn int    `json:"expiresIn"`
}

type TokenIssuer struct {
	DB      *sql.DB
	Key     ed25519.PrivateKey
	Members MemberLookup
	Now     func() time.Time
}

func (t *TokenIssuer) now() time.Time {
	if t.Now != nil {
		return t.Now().UTC()
	}
	return time.Now().UTC()
}

// ParseKey reads an Ed25519 private key from PKCS#8 PEM.
func ParseKey(pemStr string) (ed25519.PrivateKey, error) {
	blk, _ := pem.Decode([]byte(strings.TrimSpace(pemStr)))
	if blk == nil {
		return nil, errors.New("JWT_ED25519_KEY: not PEM")
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return nil, err
	}
	ek, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("JWT_ED25519_KEY: not an Ed25519 key")
	}
	return ek, nil
}

func (t *TokenIssuer) pub() ed25519.PublicKey { return t.Key.Public().(ed25519.PublicKey) }

// KID is derived from the public key.
func (t *TokenIssuer) KID() string {
	h := sha256.Sum256(t.pub())
	return base64.RawURLEncoding.EncodeToString(h[:8])
}

func (t *TokenIssuer) JWKS() map[string]any {
	return map[string]any{"keys": []map[string]string{{
		"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig", "kid": t.KID(),
		"x": base64.RawURLEncoding.EncodeToString(t.pub()),
	}}}
}

func (t *TokenIssuer) access(ctx context.Context, c Contact) (string, error) {
	mid := ""
	if t.Members != nil {
		var err error
		if mid, err = t.Members.GetMemberIDByContact(ctx, c.Value); err != nil {
			return "", fmt.Errorf("member lookup: %w", err)
		}
	}
	lang := c.Lang
	if lang == "" {
		lang = "en"
	}
	now := t.now()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"typ": "user", "sub": c.Value, "ctype": c.Kind, "mid": mid, "lang": lang, "verified": true,
		"iss": issuer, "aud": audience, "iat": now.Unix(), "exp": now.Add(accessTTL).Unix(),
	})
	tok.Header["kid"] = t.KID()
	return tok.SignedString(t.Key)
}

func hashToken(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

// Issue creates an access token and a new refresh token for c.
func (t *TokenIssuer) Issue(ctx context.Context, c Contact) (Tokens, error) {
	return t.issue(ctx, t.DB, c)
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (t *TokenIssuer) issue(ctx context.Context, x execer, c Contact) (Tokens, error) {
	acc, err := t.access(ctx, c)
	if err != nil {
		return Tokens{}, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Tokens{}, err
	}
	refresh := base64.RawURLEncoding.EncodeToString(raw)
	now := t.now()
	lang := c.Lang
	if lang == "" {
		lang = "en"
	}
	if _, err := x.ExecContext(ctx,
		`INSERT INTO refresh_tokens(id, subject, lang, token_hash, expires_at, created_at) VALUES(?,?,?,?,?,?)`,
		uuid.NewString(), c.Value, lang, hashToken(refresh), now.Add(refreshTTL), now); err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: acc, Refresh: refresh, ExpiresIn: int(accessTTL.Seconds())}, nil
}

// Refresh rotates a refresh token. Reuse of a revoked token revokes the whole subject.
func (t *TokenIssuer) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	tx, err := t.DB.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	var (
		id, subject, lang string
		expires           time.Time
		revoked           sql.NullTime
	)
	err = tx.QueryRowContext(ctx,
		`SELECT id, subject, lang, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = ? FOR UPDATE`,
		hashToken(refresh)).Scan(&id, &subject, &lang, &expires, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, ErrRefreshInvalid
	}
	if err != nil {
		return Tokens{}, err
	}
	now := t.now()
	if revoked.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = ? WHERE subject = ? AND revoked_at IS NULL`, now, subject); err != nil {
			return Tokens{}, err
		}
		if err := tx.Commit(); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, ErrRefreshReused
	}
	if !now.Before(expires) {
		return Tokens{}, ErrRefreshInvalid
	}
	if _, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = ? WHERE id = ?`, now, id); err != nil {
		return Tokens{}, err
	}
	kind := "phone"
	if strings.Contains(subject, "@") {
		kind = "email"
	}
	out, err := t.issue(ctx, tx, Contact{Kind: kind, Value: subject, Lang: lang})
	if err != nil {
		return Tokens{}, err
	}
	return out, tx.Commit()
}

// Logout revokes one refresh token (unknown tokens are ignored).
func (t *TokenIssuer) Logout(ctx context.Context, refresh string) error {
	_, err := t.DB.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL`,
		t.now(), hashToken(refresh))
	return err
}
