package portal

import "encoding/json"

// RawEvent is decoded first to branch on txType.
type RawEvent struct {
	TxType    string          `json:"txType"`
	Signature string          `json:"signature"`
	Raw       json.RawMessage `json:"-"`
}

func (r *RawEvent) UnmarshalJSON(b []byte) error {
	type alias struct {
		TxType    string `json:"txType"`
		Signature string `json:"signature"`
	}
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	r.TxType = a.TxType
	r.Signature = a.Signature
	r.Raw = b
	return nil
}

// NewToken is the payload for txType=create events (subscribeNewToken).
type NewToken struct {
	Signature          string  `json:"signature"`
	Mint               string  `json:"mint"`
	TraderPublicKey    string  `json:"traderPublicKey"`
	Name               string  `json:"name"`
	Symbol             string  `json:"symbol"`
	URI                string  `json:"uri"`
	InitialBuy         float64 `json:"initialBuy"`
	SolAmount          float64 `json:"solAmount"`
	TokenAmount        float64 `json:"tokenAmount"`
	NewTokenBalance    float64 `json:"newTokenBalance"`
	BondingCurveKey    string  `json:"bondingCurveKey"`
	VTokensInBonding   float64 `json:"vTokensInBondingCurve"`
	VSolInBonding      float64 `json:"vSolInBondingCurve"`
	MarketCapSol       float64 `json:"marketCapSol"`
}

// Trade is the payload for txType=buy or txType=sell events (subscribeTokenTrade).
type Trade struct {
	Signature        string  `json:"signature"`
	Mint             string  `json:"mint"`
	TraderPublicKey  string  `json:"traderPublicKey"`
	TxType           string  `json:"txType"`
	SolAmount        float64 `json:"solAmount"`
	TokenAmount      float64 `json:"tokenAmount"`
	NewTokenBalance  float64 `json:"newTokenBalance"`
	BondingCurveKey  string  `json:"bondingCurveKey"`
	VTokensInBonding float64 `json:"vTokensInBondingCurve"`
	VSolInBonding    float64 `json:"vSolInBondingCurve"`
	MarketCapSol     float64 `json:"marketCapSol"`
}

// Migration is the payload for subscribeMigration events.
type Migration struct {
	Signature string `json:"signature"`
	Mint      string `json:"mint"`
}
