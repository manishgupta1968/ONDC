# ONDC

This repository includes small Go utilities for experimenting with ONDC Beckn-compatible search calls.

## Generating Beckn /search payloads

The `internal/beckn` package creates Beckn-compliant `/search` requests with validated context fields and intent descriptors.

Example usage through the CLI generator:

```bash
# Start the toy receiver
GOPROXY=off GOSUMDB=off go run ./cmd/searchserver

# From another terminal, generate and post a search call
GOPROXY=off GOSUMDB=off go run ./cmd/searchclient \
  -item "apples" \
  -target "http://localhost:8080/search"
```

Both commands avoid module proxy lookups, which can be helpful in offline or firewalled environments. The server logs the incoming headers and the search payload, then returns a simple `{"status":"ack"}` response.
