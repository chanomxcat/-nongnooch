package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestParseKeyAndJWKS(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	k, err := ParseKey(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})))
	if err != nil || !k.Equal(priv) {
		t.Fatalf("parse: %v", err)
	}
	ti := &TokenIssuer{Key: k}
	keys := ti.JWKS()["keys"].([]map[string]string)
	if len(keys) != 1 || keys[0]["kid"] != ti.KID() || keys[0]["alg"] != "EdDSA" {
		t.Fatalf("%v", keys)
	}
	if _, err := ParseKey("junk"); err == nil {
		t.Fatal("want error")
	}
}
