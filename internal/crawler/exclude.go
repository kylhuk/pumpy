package crawler

import (
	"context"

	"pumpy/internal/dune"
)

type PumpLookup interface {
	IsKnownPumpSignature(ctx context.Context, signature string) (bool, error)
	IsPumpProgram(programID string) bool
}

type Classification struct {
	Excluded bool
	Reason   string
}

func Classify(ctx context.Context, n *dune.NormalizedTransaction, lk PumpLookup) (Classification, error) {
	known, err := lk.IsKnownPumpSignature(ctx, n.Signature)
	if err != nil {
		return Classification{}, err
	}
	if known {
		return Classification{Excluded: true, Reason: "known_pump_signature"}, nil
	}
	for pid := range n.ProgramIDs {
		if lk.IsPumpProgram(pid) {
			return Classification{Excluded: true, Reason: "pump_program_instruction"}, nil
		}
	}
	for _, key := range n.AccountKeys {
		if lk.IsPumpProgram(key) {
			return Classification{Excluded: true, Reason: "pump_program_account_key"}, nil
		}
	}
	return Classification{Excluded: false}, nil
}
