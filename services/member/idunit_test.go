package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testCipher(t *testing.T) *IDCipher {
	t.Helper()
	k := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	h := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("h", 32)))
	c, err := NewIDCipher(k, 1, h)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseThai(t *testing.T) {
	id, err := ParseIdentity("TH", "1-1017-00230-70-8", "")
	if err != nil || id.Kind != "thai_id" || id.Number != "1101700230708" || id.Last4 != "0708" || !id.ThaiVerified {
		t.Fatalf("%+v %v", id, err)
	}
}

func TestParseThaiBadCheckDigit(t *testing.T) {
	if _, err := ParseIdentity("TH", "1101700230705", ""); !errors.Is(err, ErrIDNumberInvalid) {
		t.Fatal(err)
	}
}

func TestParsePassport(t *testing.T) {
	id, err := ParseIdentity("CN", " e 1234567 ", "CN")
	if err != nil || id.Number != "E1234567" || id.Country != "CN" || id.ThaiVerified || id.Last4 != "4567" {
		t.Fatalf("%+v %v", id, err)
	}
}

func TestThaiMustUseThaiID(t *testing.T) {
	if _, err := ParseIdentity("TH", "E1234567", "TH"); !errors.Is(err, ErrIDNumberInvalid) {
		t.Fatal(err)
	}
}

func TestUnknownCountry(t *testing.T) {
	if _, err := ParseIdentity("XX", "E1234567", ""); !errors.Is(err, ErrInvalidNationality) {
		t.Fatal(err)
	}
	if _, err := ParseIdentity("CN", "E1234567", "ZZ"); !errors.Is(err, ErrInvalidCountry) {
		t.Fatal(err)
	}
}

func TestEncryptRoundTrip(t *testing.T) {
	c := testCipher(t)
	b, err := c.Encrypt("1101700230708", "member:1")
	if err != nil || b[0] != 1 {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("1101700230708")) {
		t.Fatal("plaintext in blob")
	}
	got, err := c.Decrypt(b, "member:1")
	if err != nil || got != "1101700230708" {
		t.Fatal(got, err)
	}
	if _, err := c.Decrypt(b, "member:2"); err == nil {
		t.Fatal("wrong aad must fail")
	}
	b2, _ := c.Encrypt("1101700230708", "member:1")
	if bytes.Equal(b, b2) {
		t.Fatal("ciphertext must not be deterministic")
	}
}

func TestHashStableAcrossFormats(t *testing.T) {
	c := testCipher(t)
	a, _ := ParseIdentity("TH", "1-1017-00230-70-8", "")
	b, _ := ParseIdentity("TH", "1101700230708", "")
	if !bytes.Equal(c.Hash(a), c.Hash(b)) {
		t.Fatal("hash differs")
	}
	p, _ := ParseIdentity("CN", "E12345678", "CN")
	if bytes.Equal(c.Hash(a), c.Hash(p)) {
		t.Fatal("collision")
	}
}
