package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

const qrPrefix = "NN1."

var b64 = base64.RawURLEncoding

// QRPayload is the signed content of a ticket QR (contracts section 6).
type QRPayload struct {
	T string  `json:"t"`
	K string  `json:"k"`
	D *string `json:"d"`
	H string  `json:"h"`
}

// Signer signs tickets and verifies against the current plus previous public keys.
type Signer struct {
	priv ed25519.PrivateKey
	pubs []ed25519.PublicKey
}

// NewSigner takes a base64 Ed25519 seed (32 bytes) or private key (64 bytes) and
// previous public keys (base64url or std base64).
func NewSigner(keyB64 string, previous []string) (*Signer, error) {
	raw, err := decodeB64(keyB64)
	if err != nil {
		return nil, errors.New("TICKET_SIGNING_KEY: bad base64")
	}
	s := &Signer{}
	switch len(raw) {
	case ed25519.SeedSize:
		s.priv = ed25519.NewKeyFromSeed(raw)
	case ed25519.PrivateKeySize:
		s.priv = ed25519.PrivateKey(raw)
	default:
		return nil, errors.New("TICKET_SIGNING_KEY: must be a 32-byte seed or 64-byte key")
	}
	s.pubs = []ed25519.PublicKey{s.priv.Public().(ed25519.PublicKey)}
	for _, p := range previous {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		pb, err := decodeB64(p)
		if err != nil || len(pb) != ed25519.PublicKeySize {
			return nil, errors.New("TICKET_PREVIOUS_PUBLIC_KEYS: bad key")
		}
		s.pubs = append(s.pubs, ed25519.PublicKey(pb))
	}
	return s, nil
}

func decodeB64(s string) ([]byte, error) {
	s = strings.TrimRight(strings.TrimSpace(s), "=")
	s = strings.NewReplacer("+", "-", "/", "_").Replace(s)
	return b64.DecodeString(s)
}

// PublicKeys returns base64url public keys, current first.
func (s *Signer) PublicKeys() []string {
	out := make([]string, len(s.pubs))
	for i, p := range s.pubs {
		out[i] = b64.EncodeToString(p)
	}
	return out
}

func (s *Signer) Sign(p QRPayload) string {
	j, _ := json.Marshal(p)
	body := qrPrefix + b64.EncodeToString(j)
	return body + "." + b64.EncodeToString(ed25519.Sign(s.priv, []byte(body)))
}

// Verify checks the signature against every known key and returns the payload.
func (s *Signer) Verify(qr string) (QRPayload, bool) {
	return VerifyQR(qr, s.pubs)
}

func VerifyQR(qr string, pubs []ed25519.PublicKey) (QRPayload, bool) {
	var p QRPayload
	parts := strings.Split(strings.TrimSpace(qr), ".")
	if len(parts) != 3 || parts[0]+"." != qrPrefix {
		return p, false
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return p, false
	}
	body := []byte(parts[0] + "." + parts[1])
	good := false
	for _, k := range pubs {
		if ed25519.Verify(k, body, sig) {
			good = true
			break
		}
	}
	if !good {
		return p, false
	}
	j, err := b64.DecodeString(parts[1])
	if err != nil || json.Unmarshal(j, &p) != nil || p.T == "" {
		return p, false
	}
	return p, true
}
