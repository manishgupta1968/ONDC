package beckn

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestBuildSignature(t *testing.T) {
	// Seed deterministically generated for test purposes.
	seed, _ := base64.StdEncoding.DecodeString("E+VvA+Y1jC8hdhXpVJPz9gY3Y/8+mWQPXgBflOHP+GM=")
	body := []byte(`{"hello":"world"}`)

	created := int64(1_700_000_000)
	expires := created + 600

	sig, err := BuildSignature(body, "example-bap|1|ed25519", base64.StdEncoding.EncodeToString(seed), created, expires)
	if err != nil {
		t.Fatalf("BuildSignature returned error: %v", err)
	}

	expectedDigest := "SHA-256=k6I5cakU5erL8KjSUVTNownDwccvu5kU1Hxg88toFYg="
	if sig.Digest != expectedDigest {
		t.Fatalf("unexpected digest: %s", sig.Digest)
	}

	expectedAuth := "Signature keyId=\"example-bap|1|ed25519\", algorithm=\"ed25519\", created=\"1700000000\", expires=\"1700000600\", headers=\"(created) (expires) digest\", signature=\"StzF1tlqpDNmXLRyzVTNS1Gxsix5BZiMDhDr6Ki57z7ZyhWifbI2b8WZgJhlERWzp6v3oEeUG3Be6/07GOhYBw==\""
	if sig.Authorization != expectedAuth {
		t.Fatalf("unexpected authorization: %s", sig.Authorization)
	}
}

func TestBuildSignatureRejectsBadTiming(t *testing.T) {
	_, err := BuildSignature([]byte("{}"), "id", "", 0, 0)
	if err == nil {
		t.Fatal("expected error for invalid timestamps")
	}
}

func TestVerifySignature(t *testing.T) {
	seed, _ := base64.StdEncoding.DecodeString("E+VvA+Y1jC8hdhXpVJPz9gY3Y/8+mWQPXgBflOHP+GM=")
	priv := base64.StdEncoding.EncodeToString(seed)

	body := []byte(`{"hello":"world"}`)
	created := int64(1_700_000_000)
	expires := created + 600

	sig, err := BuildSignature(body, "example-bap|1|ed25519", priv, created, expires)
	if err != nil {
		t.Fatalf("BuildSignature returned error: %v", err)
	}

	// Derive the public key from the private key for verification.
	if err := VerifySignature(body, sig.Digest, sig.Authorization, priv, VerificationOptions{Now: func() int64 { return created + 1 }}); err != nil {
		t.Fatalf("VerifySignature returned error: %v", err)
	}
}

func TestVerifySignatureRejectsMismatchedDigest(t *testing.T) {
	seed, _ := base64.StdEncoding.DecodeString("E+VvA+Y1jC8hdhXpVJPz9gY3Y/8+mWQPXgBflOHP+GM=")
	priv := base64.StdEncoding.EncodeToString(seed)

	body := []byte(`{"hello":"world"}`)
	created := time.Now().Unix()
	expires := created + 600

	sig, err := BuildSignature(body, "example-bap|1|ed25519", priv, created, expires)
	if err != nil {
		t.Fatalf("BuildSignature returned error: %v", err)
	}

	if err := VerifySignature([]byte(`{"hello":"tampered"}`), sig.Digest, sig.Authorization, priv, VerificationOptions{Now: func() int64 { return created + 1 }}); err == nil {
		t.Fatal("expected digest mismatch error")
	}
}
