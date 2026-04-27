package solana

const (
	SystemProgram    = "11111111111111111111111111111111"
	TokenProgram     = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	Token2022Program = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
	WSOLMint         = "So11111111111111111111111111111111111111112"

	// SystemTransferDiscriminator is the little-endian u32 discriminator for
	// SystemInstruction::Transfer (value 2).
	SystemTransferDiscriminator = uint32(2)
)

// PumpProgramSeed is the bootstrap set of pump.fun program IDs seeded into
// pump_program_id on first daemon start. Additional IDs can be added at
// runtime by inserting directly into the table.
var PumpProgramSeed = []struct {
	ID    string
	Label string
}{
	{"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P", "pump.fun bonding curve"},
	{"pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA", "pumpswap AMM"},
}
