package crawler

import (
	"context"
	"fmt"
	"testing"

	"pumpy/internal/dune"
)

type stubLookup struct {
	sigs     map[string]bool
	programs map[string]bool
	sigErr   error
}

func (s *stubLookup) IsKnownPumpSignature(_ context.Context, sig string) (bool, error) {
	return s.sigs[sig], s.sigErr
}
func (s *stubLookup) IsPumpProgram(programID string) bool { return s.programs[programID] }

func TestClassify_KnownSignature(t *testing.T) {
	lk := &stubLookup{sigs: map[string]bool{"sig-pump": true}, programs: map[string]bool{}}
	n := &dune.NormalizedTransaction{Signature: "sig-pump", AccountKeys: []string{"a"}, ProgramIDs: map[string]bool{"x": true}}
	res, err := Classify(context.Background(), n, lk)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Excluded || res.Reason != "known_pump_signature" {
		t.Errorf("want excluded by known_pump_signature, got %+v", res)
	}
}

func TestClassify_PumpProgramInstruction(t *testing.T) {
	lk := &stubLookup{sigs: map[string]bool{}, programs: map[string]bool{"PumpProg": true}}
	n := &dune.NormalizedTransaction{
		Signature:   "sig-other",
		AccountKeys: []string{"a", "b"},
		ProgramIDs:  map[string]bool{"PumpProg": true},
	}
	res, err := Classify(context.Background(), n, lk)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Excluded || res.Reason != "pump_program_instruction" {
		t.Errorf("want pump_program_instruction, got %+v", res)
	}
}

func TestClassify_PumpProgramAccountKey(t *testing.T) {
	lk := &stubLookup{sigs: map[string]bool{}, programs: map[string]bool{"PumpProg": true}}
	n := &dune.NormalizedTransaction{
		Signature:   "sig-other",
		AccountKeys: []string{"a", "PumpProg"},
		ProgramIDs:  map[string]bool{"OtherProg": true},
	}
	res, err := Classify(context.Background(), n, lk)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Excluded || res.Reason != "pump_program_account_key" {
		t.Errorf("want pump_program_account_key, got %+v", res)
	}
}

func TestClassify_NonPump(t *testing.T) {
	lk := &stubLookup{sigs: map[string]bool{}, programs: map[string]bool{}}
	n := &dune.NormalizedTransaction{
		Signature:   "sig-clean",
		AccountKeys: []string{"a", "b"},
		ProgramIDs:  map[string]bool{"11111111111111111111111111111111": true},
	}
	res, err := Classify(context.Background(), n, lk)
	if err != nil {
		t.Fatal(err)
	}
	if res.Excluded {
		t.Errorf("want not excluded, got %+v", res)
	}
}

func TestClassify_LookupError(t *testing.T) {
	lk := &stubLookup{sigErr: fmt.Errorf("db down")}
	n := &dune.NormalizedTransaction{
		Signature:   "any-sig",
		AccountKeys: []string{"a"},
		ProgramIDs:  map[string]bool{"x": true},
	}
	_, err := Classify(context.Background(), n, lk)
	if err == nil {
		t.Error("want error from Classify when lookup fails, got nil")
	}
}
