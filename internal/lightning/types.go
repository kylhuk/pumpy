package lightning

// TradeRequest holds the parameters for a Lightning trade.
type TradeRequest struct {
	Action          string  `json:"action"`           // "buy" or "sell"
	Mint            string  `json:"mint"`
	Amount          float64 `json:"amount"`
	Denomination    string  `json:"denominatedInSol"` // "true" for SOL, "false" for token units
	Slippage        int     `json:"slippage"`         // percent, e.g. 50
	PriorityFee     float64 `json:"priorityFee"`      // SOL
	Pool            string  `json:"pool"`             // "pump"
}

// TradeResult is the response from a successful Lightning trade.
type TradeResult struct {
	Signature string
}
