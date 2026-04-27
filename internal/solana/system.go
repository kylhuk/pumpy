package solana

import (
	"encoding/binary"

	"github.com/mr-tron/base58"
)

// SystemTransfer represents a decoded System Program Transfer instruction.
type SystemTransfer struct {
	From     string
	To       string
	Lamports uint64
}

func DecodeSystemTransfer(programID string, accounts []string, dataB58 string) (*SystemTransfer, error) {
	if programID != SystemProgram {
		return nil, nil
	}
	if len(accounts) < 2 {
		return nil, nil
	}
	if dataB58 == "" {
		return nil, nil
	}
	raw, err := base58.Decode(dataB58)
	if err != nil {
		return nil, err
	}
	if len(raw) < 12 {
		return nil, nil
	}
	if binary.LittleEndian.Uint32(raw[0:4]) != SystemTransferDiscriminator {
		return nil, nil
	}
	lamports := binary.LittleEndian.Uint64(raw[4:12])
	return &SystemTransfer{From: accounts[0], To: accounts[1], Lamports: lamports}, nil
}
