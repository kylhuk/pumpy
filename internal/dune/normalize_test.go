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

func TestNormalize_InlineProgramID(t *testing.T) {
	// Some Dune responses supply programId as a direct string rather than
	// programIdIndex. Verify resolveIx honours the string field.
	raw := `{
		"signature": "sigProg",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {
				"message": {
					"accountKeys": ["AcctA", "AcctB"],
					"instructions": [{"programId": "DirectProgram111", "accounts": [0, 1], "data": ""}]
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
	if len(n.Instructions) != 1 {
		t.Fatalf("want 1 instruction, got %d", len(n.Instructions))
	}
	if n.Instructions[0].ProgramID != "DirectProgram111" {
		t.Errorf("want ProgramID DirectProgram111, got %q", n.Instructions[0].ProgramID)
	}
}

func TestNormalize_StringAccounts(t *testing.T) {
	// Instructions can have accounts as []string (direct addresses) instead of
	// []int indices. Verify resolveAccounts passes them through unchanged.
	raw := `{
		"signature": "sigStr",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {
				"message": {
					"accountKeys": [],
					"instructions": [{"programId": "SomeProg", "accounts": ["AddrOne", "AddrTwo"], "data": ""}]
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
	if len(n.Instructions) != 1 {
		t.Fatalf("want 1 instruction, got %d", len(n.Instructions))
	}
	got := n.Instructions[0].Accounts
	if len(got) != 2 || got[0] != "AddrOne" || got[1] != "AddrTwo" {
		t.Errorf("want [AddrOne AddrTwo], got %v", got)
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

func TestNormalize_BlockTimePrecision(t *testing.T) {
	cases := []struct {
		name      string
		blockTime int64
		want      int64
	}{
		{name: "seconds", blockTime: 1_735_000_000, want: 1_735_000_000},
		{name: "milliseconds", blockTime: 1_735_000_000_000, want: 1_735_000_000},
		{name: "microseconds", blockTime: 1_735_000_000_000_000, want: 1_735_000_000},
		{name: "nanoseconds", blockTime: 1_735_000_000_000_000_000, want: 1_735_000_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"meta":{"err":null},"transaction":{"message":{"accountKeys":[],"instructions":[]}}}`
			n, err := Normalize(DuneTransaction{BlockTime: tc.blockTime, RawTransaction: json.RawMessage(raw)})
			if err != nil {
				t.Fatal(err)
			}
			if n.BlockTime != tc.want {
				t.Fatalf("want block_time %d, got %d", tc.want, n.BlockTime)
			}
		})
	}
}

func TestNormalizeUnixSeconds_Boundaries(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want int64
	}{
		{name: "millisecond lower bound", in: 1_000_000_000_000, want: 1_000_000_000},
		{name: "microsecond lower bound", in: 1_000_000_000_000_000, want: 1_000_000_000},
		{name: "nanosecond lower bound", in: 1_000_000_000_000_000_000, want: 1_000_000_000},
		{name: "negative seconds passthrough", in: -1_000, want: -1_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeUnixSeconds(tc.in); got != tc.want {
				t.Fatalf("normalizeUnixSeconds(%d): want %d, got %d", tc.in, tc.want, got)
			}
		})
	}
}

func TestNormalize_AccountKeysObjectEncoding(t *testing.T) {
	raw := `{
		"signature": "sigObj",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {
				"message": {
					"accountKeys": [{"pubkey": "ObjAcct1"}, {"pubkey": "ObjAcct2"}],
					"instructions": [{"programId": "Prog", "accounts": [0, 1], "data": ""}]
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
	if len(n.AccountKeys) != 2 || n.AccountKeys[0] != "ObjAcct1" || n.AccountKeys[1] != "ObjAcct2" {
		t.Fatalf("unexpected accountKeys: %v", n.AccountKeys)
	}
}

func TestNormalize_InvalidAccountKeysType(t *testing.T) {
	raw := `{
		"signature": "sigBad",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {"message": {"accountKeys": [123], "instructions": []}}
		}
	}`
	var dt DuneTransaction
	if err := json.Unmarshal([]byte(raw), &dt); err != nil {
		t.Fatal(err)
	}
	if _, err := Normalize(dt); err == nil {
		t.Fatal("expected error for invalid accountKeys encoding")
	}
}

func TestNormalize_InvalidInstructionAccountsType(t *testing.T) {
	raw := `{
		"signature": "sigBadIx",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {
				"message": {
					"accountKeys": ["A", "B"],
					"instructions": [{"programId": "Prog", "accounts": [true], "data": ""}]
				}
			}
		}
	}`
	var dt DuneTransaction
	if err := json.Unmarshal([]byte(raw), &dt); err != nil {
		t.Fatal(err)
	}
	if _, err := Normalize(dt); err == nil {
		t.Fatal("expected error for invalid instruction accounts encoding")
	}
}

func TestNormalize_SignatureFallbackFromRawTransaction(t *testing.T) {
	raw := `{
		"signature": "",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {
				"signatures": ["sigFromMessage"],
				"message": {"accountKeys": [], "instructions": []}
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
	if n.Signature != "sigFromMessage" {
		t.Fatalf("want fallback signature sigFromMessage, got %q", n.Signature)
	}
}

func TestNormalize_InnerInstructionsAreCollected(t *testing.T) {
	raw := `{
		"signature": "sigInner",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {
				"err": null,
				"innerInstructions": [
					{"instructions": [{"programId": "InnerProgA", "accounts": [], "data": ""}]},
					{"instructions": [{"programId": "InnerProgB", "accounts": [], "data": ""}]}
				]
			},
			"transaction": {
				"message": {
					"accountKeys": [],
					"instructions": [{"programId": "OuterProg", "accounts": [], "data": ""}]
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
	if len(n.InnerInstructions) != 2 {
		t.Fatalf("want 2 inner instructions, got %d", len(n.InnerInstructions))
	}
	if !n.ProgramIDs["OuterProg"] || !n.ProgramIDs["InnerProgA"] || !n.ProgramIDs["InnerProgB"] {
		t.Fatalf("expected program IDs from outer and inner instructions, got %v", n.ProgramIDs)
	}
}

func TestNormalize_InstructionAccountIndexOutOfRange(t *testing.T) {
	raw := `{
		"signature": "sigBadIndex",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {
				"message": {
					"accountKeys": ["OnlyOne"],
					"instructions": [{"programId": "Prog", "accounts": [1], "data": ""}]
				}
			}
		}
	}`
	var dt DuneTransaction
	if err := json.Unmarshal([]byte(raw), &dt); err != nil {
		t.Fatal(err)
	}
	if _, err := Normalize(dt); err == nil {
		t.Fatal("expected out-of-range account index error")
	}
}

func TestNormalize_AccountKeysObjectMissingPubkey(t *testing.T) {
	raw := `{
		"signature": "sigObjMissing",
		"block_slot": 1,
		"block_time": 1,
		"raw_transaction": {
			"meta": {"err": null},
			"transaction": {
				"message": {
					"accountKeys": [{"pubkey": "Present"}, {}],
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
	if len(n.AccountKeys) != 2 || n.AccountKeys[0] != "Present" || n.AccountKeys[1] != "" {
		t.Fatalf("unexpected accountKeys for missing pubkey field: %v", n.AccountKeys)
	}
}

func BenchmarkResolveAccounts_IntIndexes(b *testing.B) {
	raw := json.RawMessage(`[0,1,2,3,4,5,6,7,8,9]`)
	keys := []string{"k0", "k1", "k2", "k3", "k4", "k5", "k6", "k7", "k8", "k9"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := resolveAccounts(raw, keys); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormalize_BasicTransaction(b *testing.B) {
	raw := `{
		"signature": "benchsig",
		"block_slot": 250000000,
		"block_time": 1735000000,
		"raw_transaction": {
			"meta": {
				"err": null,
				"loadedAddresses": {"writable": ["WriteAddr111"], "readonly": ["RoAddr222"]},
				"innerInstructions": [{"instructions": [{"programId": "InnerProg", "accounts": [0], "data": ""}]}]
			},
			"transaction": {
				"signatures": ["benchsig"],
				"message": {
					"accountKeys": ["Acct0", "Prog111"],
					"instructions": [{"programIdIndex": 1, "accounts": [0], "data": ""}]
				}
			}
		}
	}`
	var dt DuneTransaction
	if err := json.Unmarshal([]byte(raw), &dt); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Normalize(dt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDetectJSONArrayType(b *testing.B) {
	raw := []byte(`[{"pubkey":"ObjAcct1"},{"pubkey":"ObjAcct2"}]`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := detectJSONArrayType(raw); got != jsonArrayObjects {
			b.Fatalf("unexpected kind: %v", got)
		}
	}
}

func TestDetectJSONArrayType(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want jsonArrayKind
	}{
		{name: "empty", raw: `[]`, want: jsonArrayEmpty},
		{name: "strings", raw: `["a"]`, want: jsonArrayStrings},
		{name: "objects", raw: `[{"pubkey":"x"}]`, want: jsonArrayObjects},
		{name: "ints", raw: `[1,2]`, want: jsonArrayInts},
		{name: "ints with spaces", raw: " [ \n 2 ] ", want: jsonArrayInts},
		{name: "bool unsupported", raw: `[true]`, want: jsonArrayUnknown},
		{name: "non-array", raw: `{"k":"v"}`, want: jsonArrayUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectJSONArrayType([]byte(tc.raw)); got != tc.want {
				t.Fatalf("detectJSONArrayType(%q): want %v, got %v", tc.raw, tc.want, got)
			}
		})
	}
}
