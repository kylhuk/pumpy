package solana

import (
	"encoding/binary"
	"testing"

	"github.com/mr-tron/base58"
)

// makeTransferData encodes a System Program Transfer instruction data field.
func makeTransferData(discriminator uint32, lamports uint64) string {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:4], discriminator)
	binary.LittleEndian.PutUint64(buf[4:12], lamports)
	return base58.Encode(buf)
}

func TestDecodeSystemTransfer_Valid(t *testing.T) {
	data := makeTransferData(2, 100_000_000)
	tx, err := DecodeSystemTransfer(SystemProgram, []string{"FromW", "ToW"}, data)
	if err != nil {
		t.Fatal(err)
	}
	if tx == nil {
		t.Fatal("want transfer, got nil")
	}
	if tx.From != "FromW" || tx.To != "ToW" {
		t.Errorf("wrong endpoints: %+v", tx)
	}
	if tx.Lamports != 100_000_000 {
		t.Errorf("want 100_000_000 lamports, got %d", tx.Lamports)
	}
}

func TestDecodeSystemTransfer_WrongProgram(t *testing.T) {
	data := makeTransferData(2, 100_000_000)
	tx, err := DecodeSystemTransfer(TokenProgram, []string{"a", "b"}, data)
	if err != nil {
		t.Fatal(err)
	}
	if tx != nil {
		t.Error("want nil for non-system program")
	}
}

func TestDecodeSystemTransfer_WrongDiscriminator(t *testing.T) {
	data := makeTransferData(3, 100_000_000) // discriminator 3 = not Transfer
	tx, err := DecodeSystemTransfer(SystemProgram, []string{"a", "b"}, data)
	if err != nil {
		t.Fatal(err)
	}
	if tx != nil {
		t.Error("want nil for non-transfer discriminator")
	}
}

func TestDecodeSystemTransfer_TooFewAccounts(t *testing.T) {
	data := makeTransferData(2, 100_000_000)
	tx, err := DecodeSystemTransfer(SystemProgram, []string{"only-one"}, data)
	if err != nil {
		t.Fatal(err)
	}
	if tx != nil {
		t.Error("want nil for < 2 accounts")
	}
}

func TestDecodeSystemTransfer_EmptyData(t *testing.T) {
	tx, err := DecodeSystemTransfer(SystemProgram, []string{"a", "b"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if tx != nil {
		t.Error("want nil for empty data")
	}
}

func TestDecodeSystemTransfer_TooShort(t *testing.T) {
	// Only 4 bytes — valid discriminator but no lamport field.
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, 2)
	data := base58.Encode(buf)
	tx, err := DecodeSystemTransfer(SystemProgram, []string{"a", "b"}, data)
	if err != nil {
		t.Fatal(err)
	}
	if tx != nil {
		t.Error("want nil for too-short data")
	}
}
