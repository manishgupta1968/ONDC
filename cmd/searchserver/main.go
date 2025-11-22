package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", handleSearch)

	addr := ":8080"
	log.Printf("search receiver listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var payload interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
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
