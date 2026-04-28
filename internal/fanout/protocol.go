package fanout

import "encoding/json"

// Envelope is the wire format for a single fanout event.
// One JSON object per line over a TCP connection.
type Envelope struct {
	Type string          `json:"type"` // "create", "trade", or "migration"
	Data json.RawMessage `json:"data"`
}
