package beckn

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Context captures the Beckn context object for search requests.
type Context struct {
	Domain        string    `json:"domain"`
	Country       string    `json:"country"`
	City          string    `json:"city"`
	Action        string    `json:"action"`
	CoreVersion   string    `json:"core_version"`
	BAPID         string    `json:"bap_id"`
	BAPURI        string    `json:"bap_uri"`
	TransactionID string    `json:"transaction_id"`
	MessageID     string    `json:"message_id"`
	Timestamp     time.Time `json:"timestamp"`
	TTL           string    `json:"ttl"`
}

// Descriptor names a search target in the intent.
type Descriptor struct {
	Name string `json:"name,omitempty"`
}

// ItemIntent captures the item descriptor in a search intent.
type ItemIntent struct {
	Descriptor Descriptor `json:"descriptor"`
}

// Intent describes the desired item or service to be discovered.
type Intent struct {
	Item *ItemIntent `json:"item,omitempty"`
}

// SearchMessage wraps the intent field as per Beckn schema.
type SearchMessage struct {
	Intent Intent `json:"intent"`
}

// SearchRequest models the Beckn search request body.
type SearchRequest struct {
	Context Context       `json:"context"`
	Message SearchMessage `json:"message"`
}

// SearchOptions configures how a search request is generated.
type SearchOptions struct {
	Domain      string
	Country     string
	City        string
	CoreVersion string
	BAPID       string
	BAPURI      string
	TTL         time.Duration
	ItemName    string
	Timestamp   time.Time
}

// NewSearchRequest creates a Beckn-compliant search request with generated IDs.
func NewSearchRequest(opts SearchOptions) (SearchRequest, error) {
	if opts.Timestamp.IsZero() {
		opts.Timestamp = time.Now().UTC()
	}

	req := SearchRequest{
		Context: Context{
			Domain:        opts.Domain,
			Country:       opts.Country,
			City:          opts.City,
			Action:        "search",
			CoreVersion:   opts.CoreVersion,
			BAPID:         opts.BAPID,
			BAPURI:        opts.BAPURI,
			TransactionID: newID(),
			MessageID:     newID(),
			Timestamp:     opts.Timestamp,
			TTL:           formatTTL(opts.TTL),
		},
		Message: SearchMessage{
			Intent: Intent{
				Item: &ItemIntent{Descriptor: Descriptor{Name: opts.ItemName}},
			},
		},
	}

	if err := req.Validate(); err != nil {
		return SearchRequest{}, err
	}

	return req, nil
}

// Validate enforces the mandatory Beckn fields required for a search call.
func (r SearchRequest) Validate() error {
	ctx := r.Context
	switch {
	case ctx.Domain == "":
		return errors.New("context.domain is required")
	case ctx.Country == "":
		return errors.New("context.country is required")
	case ctx.City == "":
		return errors.New("context.city is required")
	case ctx.Action != "search":
		return errors.New("context.action must be 'search'")
	case ctx.CoreVersion == "":
		return errors.New("context.core_version is required")
	case ctx.BAPID == "":
		return errors.New("context.bap_id is required")
	case ctx.BAPURI == "":
		return errors.New("context.bap_uri is required")
	case ctx.TransactionID == "":
		return errors.New("context.transaction_id is required")
	case ctx.MessageID == "":
		return errors.New("context.message_id is required")
	case ctx.TTL == "":
		return errors.New("context.ttl is required")
	case r.Message.Intent.Item == nil || r.Message.Intent.Item.Descriptor.Name == "":
		return errors.New("message.intent.item.descriptor.name is required")
	}

	if !ctx.Timestamp.IsZero() && ctx.Timestamp.Location() != time.UTC {
		return errors.New("context.timestamp must be in UTC")
	}

	return nil
}

// MarshalJSON ensures timestamp renders in RFC3339 with Z suffix.
func (r SearchRequest) MarshalJSON() ([]byte, error) {
	type alias SearchRequest
	return json.Marshal(&struct {
		Context aliasContext  `json:"context"`
		Message SearchMessage `json:"message"`
	}{
		Context: aliasContext(r.Context),
		Message: r.Message,
	})
}

type aliasContext Context

func (c aliasContext) MarshalJSON() ([]byte, error) {
	type alias aliasContext
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		alias
	}{
		Timestamp: c.Timestamp.UTC().Format(time.RFC3339),
		alias:     alias(c),
	})
}

func formatTTL(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return fmt.Sprintf("PT%vS", int(d.Seconds()))
}

func newID() string {
	// Generate 16 random bytes and hex encode to produce a 32-character ID.
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("unable to generate id: %v", err))
	}
	return hex.EncodeToString(b[:])
}
