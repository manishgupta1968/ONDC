package tests

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ondc/internal/beckn"
)

func TestSearchEndToEndSignatureAndAck(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	base64Priv := base64.StdEncoding.EncodeToString(priv)

	created := int64(1_700_000_000)
	expires := created + 600

	req, err := beckn.NewSearchRequest(beckn.SearchOptions{
		Domain:      "nic2004:52110",
		Country:     "IND",
		City:        "std:080",
		CoreVersion: "1.2.0",
		BAPID:       "example-bap",
		BAPURI:      "https://buyer-app.ondc.org/protocol/v1",
		TTL:         30 * time.Second,
		ItemName:    "apples",
		Timestamp:   time.Unix(created, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("build search request: %v", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	signature, err := beckn.BuildSignature(body, "example-bap|1|ed25519", base64Priv, created, expires)
	if err != nil {
		t.Fatalf("build signature: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		defer r.Body.Close()

		if err := beckn.VerifySignature(body, r.Header.Get("Digest"), r.Header.Get("Authorization"), base64.StdEncoding.EncodeToString(pub), beckn.VerificationOptions{
			ExpectedKeyID: "example-bap|1|ed25519",
			Now:           func() int64 { return created + 1 },
		}); err != nil {
			t.Fatalf("verify signature: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ack"}`))
	}))
	defer server.Close()

	httpReq, err := http.NewRequest(http.MethodPost, server.URL+"/search", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Digest", signature.Digest)
	httpReq.Header.Set("Authorization", signature.Authorization)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("post search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", resp.Status)
	}

	var ack map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		t.Fatalf("decode ack: %v", err)
	}

	if ack["status"] != "ack" {
		t.Fatalf("unexpected ack payload: %v", ack)
	}
}
