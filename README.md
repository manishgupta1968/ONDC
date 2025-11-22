# ONDC

This repository includes small Go utilities for experimenting with ONDC Beckn-compatible search calls.

## Generating Beckn /search payloads

The `internal/beckn` package creates Beckn-compliant `/search` requests with validated context fields and intent descriptors.

Example usage through the CLI generator:

```bash
# Start the toy receiver with the public key that matches your client key pair
GOPROXY=off GOSUMDB=off go run ./cmd/searchserver \
  -public-key "<base64-ed25519-public-or-private-key>" \
  -key-id "example-bap|1|ed25519"

# From another terminal, generate and post a signed search call.
# The private key must be an Ed25519 key encoded in base64.
GOPROXY=off GOSUMDB=off go run ./cmd/searchclient \
  -item "apples" \
  -private-key "<base64-ed25519-private-key>" \
  -target "http://localhost:8080/search"
```

Both commands avoid module proxy lookups, which can be helpful in offline or firewalled environments. The server logs the incoming headers and the search payload, verifies the Ed25519 digest signature, and then returns a simple `{"status":"ack"}` response when verification succeeds.
