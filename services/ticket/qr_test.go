package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// vectorSeed is the fixed test-only key behind testdata/qr_vectors.json.
func vectorSeed() []byte { h := sha256.Sum256([]byte("nongnooch-qr-test-vectors")); return h[:] }

func vectorSigner(t *testing.T) *Signer {
	s, err := NewSigner(base64.StdEncoding.EncodeToString(vectorSeed()), nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type vcase struct {
	Name   string `json:"name"`
	QR     string `json:"qr"`
	Expect string `json:"expect"` // valid | invalid
}

type vfile struct {
	PublicKeys []string `json:"publicKeys"`
	Cases      []vcase  `json:"cases"`
}

// TestGenerateVectors rewrites the vectors file when UPDATE_VECTORS=1.
func TestGenerateVectors(t *testing.T) {
	if os.Getenv("UPDATE_VECTORS") != "1" {
		t.Skip("set UPDATE_VECTORS=1 to regenerate")
	}
	s := vectorSigner(t)
	d := "2027-03-01"
	day := s.Sign(QRPayload{T: "0192f3a0-0000-7000-8000-000000000001", K: "day", D: &d, H: "0708"})
	annual := s.Sign(QRPayload{T: "0192f3a0-0000-7000-8000-000000000002", K: "annual", D: nil, H: "5678"})
	parts := strings.Split(day, ".")
	other := ed25519.NewKeyFromSeed(sha256Sum("other-key"))
	forged := parts[0] + "." + parts[1]
	forged += "." + b64.EncodeToString(ed25519.Sign(other, []byte(forged)))
	tamper, _ := json.Marshal(QRPayload{T: "0192f3a0-0000-7000-8000-000000000001", K: "day", D: &d, H: "9999"})
	f := vfile{PublicKeys: s.PublicKeys(), Cases: []vcase{
		{"valid day ticket", day, "valid"},
		{"valid annual ticket (d null)", annual, "valid"},
		{"signed by an unknown key", forged, "invalid"},
		{"payload tampered after signing", parts[0] + "." + b64.EncodeToString(tamper) + "." + parts[2], "invalid"},
		{"truncated signature", day[:len(day)-4], "invalid"},
		{"wrong prefix", "NN2." + strings.TrimPrefix(day, "NN1."), "invalid"},
		{"missing signature", parts[0] + "." + parts[1], "invalid"},
		{"garbage", "hello", "invalid"},
	}}
	b, _ := json.MarshalIndent(f, "", "  ")
	if err := os.WriteFile("testdata/qr_vectors.json", append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha256Sum(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func TestQRVectorsRoundTrip(t *testing.T) {
	raw, err := os.ReadFile("testdata/qr_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var f vfile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.PublicKeys) == 0 || len(f.Cases) < 5 {
		t.Fatalf("vectors file too thin: %d keys %d cases", len(f.PublicKeys), len(f.Cases))
	}
	var pubs []ed25519.PublicKey
	for _, k := range f.PublicKeys {
		b, err := b64.DecodeString(k)
		if err != nil {
			t.Fatal(err)
		}
		pubs = append(pubs, ed25519.PublicKey(b))
	}
	for _, c := range f.Cases {
		_, ok := VerifyQR(c.QR, pubs)
		if ok != (c.Expect == "valid") {
			t.Errorf("%s: verify=%v want %s", c.Name, ok, c.Expect)
		}
	}
	// our own signer still produces the committed valid vectors' key
	if got := vectorSigner(t).PublicKeys()[0]; got != f.PublicKeys[0] {
		t.Errorf("vector key drifted: %s vs %s", got, f.PublicKeys[0])
	}
}

func TestPreviousKeyStillVerifies(t *testing.T) {
	old := vectorSigner(t)
	d := "2027-03-01"
	qr := old.Sign(QRPayload{T: "t1", K: "day", D: &d, H: "0708"})
	cur, err := NewSigner(base64.StdEncoding.EncodeToString(sha256Sum("new")), old.PublicKeys())
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := cur.Verify(qr); !ok || p.T != "t1" {
		t.Fatal("rotated signer must verify old tickets")
	}
	if len(cur.PublicKeys()) != 2 {
		t.Fatal("snapshot must list both keys")
	}
}
