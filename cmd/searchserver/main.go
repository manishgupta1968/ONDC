package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"ondc/internal/beckn"
)

func main() {
	var publicKey string
	var expectedKeyID string
	flag.StringVar(&publicKey, "public-key", "", "base64-encoded Ed25519 public key or private key")
	flag.StringVar(&expectedKeyID, "key-id", "", "expected keyId value in the Authorization header (optional)")
	flag.Parse()

	if publicKey == "" {
		log.Fatal("-public-key is required for signature verification")
	}
	// Ensure the public key decodes before starting the server to fail fast on configuration issues.
	if _, err := base64.StdEncoding.DecodeString(publicKey); err != nil {
		log.Fatalf("invalid public key: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		handleSearch(w, r, publicKey, expectedKeyID)
	})

	addr := ":8080"
	log.Printf("search receiver listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request, publicKey, expectedKeyID string) {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusInternalServerError)
		return
	}

	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if err := beckn.VerifySignature(body, r.Header.Get("Digest"), r.Header.Get("Authorization"), publicKey, beckn.VerificationOptions{
		ExpectedKeyID: expectedKeyID,
		Now:           func() int64 { return time.Now().Unix() },
	}); err != nil {
		http.Error(w, fmt.Sprintf("signature verification failed: %v", err), http.StatusUnauthorized)
		return
	}

	log.Printf("received /search request from %s", r.RemoteAddr)
	for name, values := range r.Header {
		log.Printf("header %s: %v", name, values)
	}

	pretty, _ := json.MarshalIndent(payload, "", "  ")
	log.Printf("payload:\n%s", string(pretty))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ack"}`))
}
