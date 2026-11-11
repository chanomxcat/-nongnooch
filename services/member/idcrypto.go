package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// IDCipher encrypts ID numbers (AES-256-GCM) and hashes them (HMAC-SHA256) for duplicate checks.
type IDCipher struct {
	gcm     cipher.AEAD
	kid     byte
	hmacKey []byte
}

func NewIDCipher(encKeyB64 string, kid byte, hmacKeyB64 string) (*IDCipher, error) {
	k, err := base64.StdEncoding.DecodeString(encKeyB64)
	if err != nil || len(k) != 32 {
		return nil, errors.New("MEMBER_ID_ENC_KEY must be 32 bytes base64")
	}
	h, err := base64.StdEncoding.DecodeString(hmacKeyB64)
	if err != nil || len(h) < 16 {
		return nil, errors.New("MEMBER_ID_HMAC_KEY must be >=16 bytes base64")
	}
	blk, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	return &IDCipher{gcm: g, kid: kid, hmacKey: h}, nil
}

// Encrypt returns kid(1) || nonce(12) || ciphertext. aad binds the blob to its owner.
func (c *IDCipher) Encrypt(plain, aad string) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{c.kid}, nonce...)
	return c.gcm.Seal(out, nonce, []byte(plain), []byte(aad)), nil
}

func (c *IDCipher) Decrypt(blob []byte, aad string) (string, error) {
	ns := c.gcm.NonceSize()
	if len(blob) < 1+ns {
		return "", errors.New("id blob too short")
	}
	p, err := c.gcm.Open(nil, blob[1:1+ns], blob[1+ns:], []byte(aad))
	if err != nil {
		return "", fmt.Errorf("decrypt id: %w", err)
	}
	return string(p), nil
}

// Hash is stable across input formats (it works on the parsed Identity).
func (c *IDCipher) Hash(id Identity) []byte {
	m := hmac.New(sha256.New, c.hmacKey)
	if id.Kind == "thai_id" {
		m.Write([]byte("thai_id:" + id.Number))
	} else {
		m.Write([]byte("passport:" + id.Country + ":" + id.Number))
	}
	return m.Sum(nil)
}
