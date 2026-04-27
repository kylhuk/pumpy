package dune

import (
	"encoding/json"
	"testing"
)

func TestNormalize_AccountKeysAndPrograms(t *testing.T) {
	raw := `{
		"signature": "5xS1aaaa",
		"block_slot": 250000000,
		"block_time": 1735000000,
		"raw_transaction": {
			"meta": {"err": null, "fee": 5000, "preBalances": [200000000, 50000000, 0], "postBalances": [99995000, 150000000, 0]},
			"transaction": {
				"signatures": ["5xS1aaaa"],
				"message": {
					"accountKeys": ["FromWallet1111111111111111111111111111111111", "ToWallet2222222222222222222222222222222222", "11111111111111111111111111111111"],
					"instructions": [{"programIdIndex": 2, "accounts": [0, 1], "data": "3Bxs4ThwQbE3vNAo"}]
				}
			}
		}
	}`
	var dt DuneTransaction
	if err := json.Unmarshal([]byte(raw), &dt); err != nil {
		t.Fatal(err)
	}
	n, err := Normalize(dt)
	if err != nil {
		t.Fatal(err)
	}
	if n.Signature != "5xS1aaaa" {
		t.Errorf("want signature 5xS1aaaa, got %q", n.Signature)
	}
	if len(n.AccountKeys) != 3 {
		t.Errorf("want 3 account keys, got %d", len(n.AccountKeys))
	}
	if !n.ProgramIDs["11111111111111111111111111111111"] {
		t.Error("want System Program in ProgramIDs")
	}
	if len(n.Instructions) != 1 {
		t.Errorf("want 1 instruction, got %d", len(n.Instructions))
	}
	if n.Instructions[0].ProgramID != "11111111111111111111111111111111" {
		t.Errorf("want System Program instruction, got %s", n.Instructions[0].ProgramID)
	}
	if len(n.Instructions[0].Accounts) != 2 {
		t.Errorf("want 2 accounts in instruction, got %d", len(n.Instructions[0].Accounts))
	}
}

func TestNormalize_LoadedAddresses(t *testing.T) {
	raw := `{
		"signature": "sig1",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {
				"err": null,
				"loadedAddresses": {"writable": ["WriteAddr111"], "readonly": ["RoAddr222"]}
			},
			"transaction": {
				"message": {
					"accountKeys": ["Static111", "Static222"],
					"instructions": []
				}
			}
		}
	}`
	var dt DuneTransaction
	if err := json.Unmarshal([]byte(raw), &dt); err != nil {
		t.Fatal(err)
	}
	n, err := Normalize(dt)
	if err != nil {
		t.Fatal(err)
	}
	if len(n.AccountKeys) != 4 {
		t.Errorf("want 4 keys (2 static + 2 loaded), got %d: %v", len(n.AccountKeys), n.AccountKeys)
	}
	if n.AccountKeys[2] != "WriteAddr111" || n.AccountKeys[3] != "RoAddr222" {
		t.Errorf("loadedAddresses not in writable+readonly order: %v", n.AccountKeys)
	}
}

func TestNormalize_FailedTransaction(t *testing.T) {
	raw := `{
		"signature": "failsig",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": {"InstructionError": [0, "Custom"]}},
			"transaction": {"message": {"accountKeys": [], "instructions": []}}
		}
	}`
	var dt DuneTransaction
	if err := json.Unmarshal([]byte(raw), &dt); err != nil {
		t.Fatal(err)
	}
	n, err := Normalize(dt)
	if err != nil {
		t.Fatal(err)
	}
	if n.Err == "" {
		t.Error("want non-empty Err for failed transaction")
	}
}
