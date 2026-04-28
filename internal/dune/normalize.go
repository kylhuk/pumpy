package dune

import (
	"encoding/json"
	"fmt"
)

// NormalizedTransaction is the parsed form of a DuneTransaction's raw_transaction.
type NormalizedTransaction struct {
	Signature         string
	BlockSlot         int64
	BlockTime         int64
	FeeLamports       int64
	Err               string // non-empty if the on-chain transaction failed
	AccountKeys       []string
	ProgramIDs        map[string]bool
	Instructions      []NormalizedInstruction
	InnerInstructions []NormalizedInstruction
	PreBalances       []int64
	PostBalances      []int64
	LogMessages       []string
}

// NormalizedInstruction holds a decoded instruction with resolved addresses.
type NormalizedInstruction struct {
	ProgramID string
	Accounts  []string
	Data      string // base58-encoded instruction data
}

type rawTx struct {
	Meta struct {
		Err              json.RawMessage `json:"err"`
		Fee              int64           `json:"fee"`
		PreBalances      []int64         `json:"preBalances"`
		PostBalances     []int64         `json:"postBalances"`
		LogMessages      []string        `json:"logMessages"`
		InnerInstructions []struct {
			Instructions []rawIx `json:"instructions"`
		} `json:"innerInstructions"`
		LoadedAddresses struct {
			Writable []string `json:"writable"`
			Readonly []string `json:"readonly"`
		} `json:"loadedAddresses"`
	} `json:"meta"`
	Transaction struct {
		Signatures []string `json:"signatures"`
		Message    struct {
			AccountKeys  json.RawMessage `json:"accountKeys"`
			Instructions []rawIx         `json:"instructions"`
		} `json:"message"`
	} `json:"transaction"`
}

type rawIx struct {
	ProgramIDIndex int             `json:"programIdIndex"`
	ProgramID      string          `json:"programId"`
	Accounts       json.RawMessage `json:"accounts"` // []int or []string depending on encoding
	Data           string          `json:"data"`
}

// Normalize converts a raw DuneTransaction into a NormalizedTransaction.
func Normalize(dt DuneTransaction) (*NormalizedTransaction, error) {
	var rt rawTx
	if err := json.Unmarshal(dt.RawTransaction, &rt); err != nil {
		return nil, fmt.Errorf("decode raw_transaction: %w", err)
	}

	keys, err := decodeAccountKeys(rt.Transaction.Message.AccountKeys)
	if err != nil {
		return nil, err
	}
	// Append versioned-transaction loaded addresses after static keys.
	keys = append(keys, rt.Meta.LoadedAddresses.Writable...)
	keys = append(keys, rt.Meta.LoadedAddresses.Readonly...)

	sig := dt.Signature
	if sig == "" && len(rt.Transaction.Signatures) > 0 {
		sig = rt.Transaction.Signatures[0]
	}

	// API returns block_time in microseconds; convert to seconds.
	blockTime := dt.BlockTime
	if blockTime > 1e12 {
		blockTime /= 1_000_000
	}

	n := &NormalizedTransaction{
		Signature:    sig,
		BlockSlot:    dt.BlockSlot,
		BlockTime:    blockTime,
		FeeLamports:  rt.Meta.Fee,
		AccountKeys:  keys,
		ProgramIDs:   make(map[string]bool),
		PreBalances:  rt.Meta.PreBalances,
		PostBalances: rt.Meta.PostBalances,
		LogMessages:  rt.Meta.LogMessages,
	}
	if len(rt.Meta.Err) > 0 && string(rt.Meta.Err) != "null" {
		n.Err = string(rt.Meta.Err)
	}

	convertIxs := func(in []rawIx) ([]NormalizedInstruction, error) {
		out := make([]NormalizedInstruction, 0, len(in))
		for _, ix := range in {
			ni, err := resolveIx(ix, keys)
			if err != nil {
				return nil, err
			}
			n.ProgramIDs[ni.ProgramID] = true
			out = append(out, ni)
		}
		return out, nil
	}

	n.Instructions, err = convertIxs(rt.Transaction.Message.Instructions)
	if err != nil {
		return nil, err
	}
	for _, inner := range rt.Meta.InnerInstructions {
		ixs, err := convertIxs(inner.Instructions)
		if err != nil {
			return nil, err
		}
		n.InnerInstructions = append(n.InnerInstructions, ixs...)
	}
	return n, nil
}

func decodeAccountKeys(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	// Keys are either []string (legacy) or []{"pubkey":...} (newer encoding).
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err == nil {
		return asStrings, nil
	}
	var asObjects []struct {
		Pubkey string `json:"pubkey"`
	}
	if err := json.Unmarshal(raw, &asObjects); err != nil {
		return nil, fmt.Errorf("accountKeys: not []string or []object: %w", err)
	}
	out := make([]string, len(asObjects))
	for i, o := range asObjects {
		out[i] = o.Pubkey
	}
	return out, nil
}

func resolveIx(ix rawIx, keys []string) (NormalizedInstruction, error) {
	pid := ix.ProgramID
	if pid == "" {
		if ix.ProgramIDIndex < 0 || ix.ProgramIDIndex >= len(keys) {
			return NormalizedInstruction{}, fmt.Errorf("programIdIndex %d out of range (keys len %d)", ix.ProgramIDIndex, len(keys))
		}
		pid = keys[ix.ProgramIDIndex]
	}
	accounts, err := resolveAccounts(ix.Accounts, keys)
	if err != nil {
		return NormalizedInstruction{}, err
	}
	return NormalizedInstruction{ProgramID: pid, Accounts: accounts, Data: ix.Data}, nil
}

func resolveAccounts(raw json.RawMessage, keys []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	// Accounts are either []int (index into keys) or []string (already addresses).
	var asInts []int
	if err := json.Unmarshal(raw, &asInts); err == nil {
		out := make([]string, len(asInts))
		for i, idx := range asInts {
			if idx < 0 || idx >= len(keys) {
				return nil, fmt.Errorf("account index %d out of range (keys len %d)", idx, len(keys))
			}
			out[i] = keys[idx]
		}
		return out, nil
	}
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err != nil {
		return nil, fmt.Errorf("instruction accounts: not []int or []string: %w", err)
	}
	return asStrings, nil
}
