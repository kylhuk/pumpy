package dune

import "encoding/json"

// TransactionsResponse mirrors GET /beta/svm/transactions/{address}.
type TransactionsResponse struct {
	Transactions []DuneTransaction `json:"transactions"`
	NextOffset   *string           `json:"next_offset"`
}

// DuneTransaction holds the per-transaction envelope returned by Dune Sim.
// RawTransaction is kept as json.RawMessage so the normalizer decodes it
// lazily, and test fixtures can inject arbitrary JSON.
type DuneTransaction struct {
	Signature      string          `json:"signature"`
	BlockSlot      int64           `json:"block_slot"`
	BlockTime      int64           `json:"block_time"`
	RawTransaction json.RawMessage `json:"raw_transaction"`
}
