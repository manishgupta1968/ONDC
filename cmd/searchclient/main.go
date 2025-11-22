package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"ondc/internal/beckn"
)

func main() {
	var (
		domain  = flag.String("domain", "nic2004:52110", "ONDC domain code")
		country = flag.String("country", "IND", "ISO 3166 country code")
		city    = flag.String("city", "std:080", "Beckn city code")
		core    = flag.String("core", "1.2.0", "Beckn core version")
		bapID   = flag.String("bap", "example-bap", "BAP ID")
		bapURI  = flag.String("bap-uri", "https://buyer-app.ondc.org/protocol/v1", "BAP callback URI")
		ttl     = flag.Duration("ttl", 30*time.Second, "context TTL (e.g. 30s)")
		item    = flag.String("item", "apples", "item descriptor name")
		target  = flag.String("target", "http://localhost:8080/search", "URL to POST the search request")
	)
	flag.Parse()

	req, err := beckn.NewSearchRequest(beckn.SearchOptions{
		Domain:      *domain,
		Country:     *country,
		City:        *city,
		CoreVersion: *core,
		BAPID:       *bapID,
		BAPURI:      *bapURI,
		TTL:         *ttl,
		ItemName:    *item,
	})
	if err != nil {
		log.Fatalf("unable to build search request: %v", err)
	}

	body, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		log.Fatalf("cannot marshal request: %v", err)
	}

	log.Printf("POST %s", *target)
	resp, err := http.Post(*target, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("post failed: %v", err)
	}
	defer resp.Body.Close()

	var ack map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		log.Fatalf("cannot read response: %v", err)
	}

	log.Printf("server responded with %s", resp.Status)
	log.Printf("ack payload: %v", ack)
	fmt.Println(string(body))
}
