package beckn

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Signature encapsulates the headers required for Beckn-compatible HTTP signatures.
type Signature struct {
	Authorization string
	Digest        string
}

// VerificationOptions controls how incoming Beckn signatures are checked.
type VerificationOptions struct {
	// ExpectedKeyID, when set, must match the keyId field in the Authorization
	// header. If empty, any keyId is accepted.
	ExpectedKeyID string
	// Now overrides the timestamp used for created/expires validation. If
	// nil, time.Now is used.
	Now func() int64
}

// BuildSignature creates Beckn/ONDC HTTP signature headers for a request body.
//
// The Beckn protocol signs the SHA-256 digest of the request payload using
// Ed25519 and composes an Authorization header formatted per ONDC guidance:
//
//	Signature keyId="<subscriber_id>|<ukid>|ed25519", algorithm="ed25519", created="<ts>", expires="<ts>", headers="(created) (expires) digest", signature="<base64>"
//
// The function returns the Authorization value and the computed Digest header
// (`SHA-256=<base64>`). The private key must be supplied as a base64-encoded
// Ed25519 seed (32 bytes) or full private key (64 bytes).
func BuildSignature(body []byte, keyID, privateKeyBase64 string, created, expires int64) (Signature, error) {
	if keyID == "" {
		return Signature{}, errors.New("keyID is required for signature")
	}
	if created <= 0 || expires <= 0 || expires <= created {
		return Signature{}, errors.New("created and expires must be positive with expires > created")
	}

	digestBytes := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(digestBytes[:])

	pkBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return Signature{}, fmt.Errorf("decode private key: %w", err)
	}

	var privateKey ed25519.PrivateKey
	switch len(pkBytes) {
	case ed25519.SeedSize:
		privateKey = ed25519.NewKeyFromSeed(pkBytes)
	case ed25519.PrivateKeySize:
		privateKey = ed25519.PrivateKey(pkBytes)
	default:
		return Signature{}, fmt.Errorf("unexpected private key length: %d", len(pkBytes))
	}

	signingString := fmt.Sprintf("(created): %d\n(expires): %d\ndigest: %s", created, expires, digest)
	signature := ed25519.Sign(privateKey, []byte(signingString))
	encodedSig := base64.StdEncoding.EncodeToString(signature)

	authorization := fmt.Sprintf(
		"Signature keyId=\"%s\", algorithm=\"ed25519\", created=\"%d\", expires=\"%d\", headers=\"(created) (expires) digest\", signature=\"%s\"",
		keyID, created, expires, encodedSig,
	)

	return Signature{Authorization: authorization, Digest: digest}, nil
}

// VerifySignature checks an incoming Beckn/ONDC HTTP signature and digest
// against the provided request body and Ed25519 public key. The public key must
// be supplied as a base64-encoded 32-byte Ed25519 public key or a 64-byte
// private key (from which the public portion is derived).
func VerifySignature(body []byte, digestHeader, authorizationHeader, publicKeyBase64 string, opts VerificationOptions) error {
	if authorizationHeader == "" {
		return errors.New("missing Authorization header")
	}
	if digestHeader == "" {
		return errors.New("missing Digest header")
	}
	if publicKeyBase64 == "" {
		return errors.New("public key is required for verification")
	}

	params, err := parseAuthorization(authorizationHeader)
	if err != nil {
		return err
	}
	if opts.ExpectedKeyID != "" && params["keyId"] != opts.ExpectedKeyID {
		return fmt.Errorf("unexpected keyId: %s", params["keyId"])
	}
	if params["algorithm"] != "ed25519" {
		return fmt.Errorf("unsupported algorithm: %s", params["algorithm"])
	}
	if params["headers"] != "(created) (expires) digest" {
		return fmt.Errorf("unexpected headers list: %s", params["headers"])
	}

	digestBytes := sha256.Sum256(body)
	expectedDigest := "SHA-256=" + base64.StdEncoding.EncodeToString(digestBytes[:])
	if digestHeader != expectedDigest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", expectedDigest, digestHeader)
	}

	created, err := strconv.ParseInt(params["created"], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid created timestamp: %w", err)
	}
	expires, err := strconv.ParseInt(params["expires"], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expires timestamp: %w", err)
	}
	now := time.Now().Unix()
	if opts.Now != nil {
		now = opts.Now()
	}
	if now < created {
		return fmt.Errorf("signature not yet valid: created %d > now %d", created, now)
	}
	if now > expires {
		return fmt.Errorf("signature expired: expires %d < now %d", expires, now)
	}

	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}

	var publicKey ed25519.PublicKey
	keyLen := len(publicKeyBytes)
	switch keyLen {
	case ed25519.PublicKeySize:
		publicKey = ed25519.PublicKey(publicKeyBytes)
	case ed25519.PrivateKeySize:
		pk := ed25519.PrivateKey(publicKeyBytes)
		publicKey = pk.Public().(ed25519.PublicKey)
	default:
		return fmt.Errorf("unexpected public key length: %d", keyLen)
	}

	signingString := fmt.Sprintf("(created): %d\n(expires): %d\ndigest: %s", created, expires, digestHeader)
	signatureBytes, err := base64.StdEncoding.DecodeString(params["signature"])
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	if ed25519.Verify(publicKey, []byte(signingString), signatureBytes) {
		return nil
	}

	if keyLen == ed25519.PublicKeySize {
		// Retry with the input treated as a seed-derived key in case the 32
		// bytes represent a seed instead of a raw public key.
		pk := ed25519.NewKeyFromSeed(publicKeyBytes)
		if ed25519.Verify(pk.Public().(ed25519.PublicKey), []byte(signingString), signatureBytes) {
			return nil
		}
	}

	return errors.New("signature verification failed")
}

func parseAuthorization(auth string) (map[string]string, error) {
	if !strings.HasPrefix(auth, "Signature ") {
		return nil, fmt.Errorf("unexpected scheme in authorization: %s", auth)
	}

	params := strings.Split(strings.TrimPrefix(auth, "Signature "), ",")
	result := make(map[string]string, len(params))
	for _, param := range params {
		pieces := strings.SplitN(strings.TrimSpace(param), "=", 2)
		if len(pieces) != 2 {
			return nil, fmt.Errorf("invalid param: %s", param)
		}
		result[pieces[0]] = strings.Trim(pieces[1], "\"")
	}
	required := []string{"keyId", "algorithm", "created", "expires", "headers", "signature"}
	for _, key := range required {
		if result[key] == "" {
			return nil, fmt.Errorf("missing %s in authorization", key)
		}
	}
	return result, nil
}
