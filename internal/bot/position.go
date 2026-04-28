package bot

import (
	"sync/atomic"
	"time"
)

// Tier constants for position state.
const (
	TierS   int32 = 0
	TierQ   int32 = 1
	TierAll int32 = 2
	TierDone int32 = 3
)

// Position tracks one open mint. All fields safe for concurrent access.
type Position struct {
	Mint         string
	CreatedAt    time.Time
	CreationMcap float64 // immutable after open

	// atomic fixed-point: mcap * 1e6 (uint64 for lock-free loads/stores)
	lastMcapFixed atomic.Uint64

	remainingTokens atomic.Uint64 // token units held (see TokenToFixed)
	boughtSol       float64       // SOL spent on buy
	sellsExecuted   atomic.Int32
	inFlight        atomic.Bool
	closed          atomic.Bool
	currentTier     atomic.Int32 // tracks highest tier reached for sequence integrity
}

func newPosition(mint string, creationMcap, boughtSol float64) *Position {
	p := &Position{
		Mint:         mint,
		CreatedAt:    time.Now(),
		CreationMcap: creationMcap,
		boughtSol:    boughtSol,
	}
	p.SetLastMcap(creationMcap)
	return p
}

func (p *Position) SetLastMcap(mcap float64) {
	p.lastMcapFixed.Store(uint64(mcap * 1e6))
}

func (p *Position) LastMcap() float64 {
	return float64(p.lastMcapFixed.Load()) / 1e6
}

func (p *Position) SetRemainingTokens(tokens uint64) {
	p.remainingTokens.Store(tokens)
}

func (p *Position) RemainingTokens() uint64 {
	return p.remainingTokens.Load()
}

func (p *Position) BoughtSol() float64 { return p.boughtSol }

func (p *Position) SellsExecuted() int32 { return p.sellsExecuted.Load() }

func (p *Position) IsClosed() bool { return p.closed.Load() }

func (p *Position) TryLockInFlight() bool { return p.inFlight.CompareAndSwap(false, true) }
func (p *Position) UnlockInFlight()       { p.inFlight.Store(false) }

func (p *Position) MarkClosed() { p.closed.Store(true) }

// Age returns how long this position has been open.
func (p *Position) Age() time.Duration { return time.Since(p.CreatedAt) }

// PriceRatio returns current_mcap / creation_mcap.
func (p *Position) PriceRatio() float64 {
	if p.CreationMcap == 0 {
		return 0
	}
	return p.LastMcap() / p.CreationMcap
}

// TierName returns a short string for display.
func (p *Position) TierName() string {
	switch p.currentTier.Load() {
	case TierS:
		return "S"
	case TierQ:
		return "Q"
	case TierAll, TierDone:
		return "ALL"
	}
	return "?"
}

// EvaluateTier returns the tier the bot should sell at given the price ratio and config.
// Returns TierDone if position is already closed.
func EvaluateTier(priceRatio float64, cfg Config) int32 {
	switch {
	case priceRatio >= cfg.TierSRatio:
		return TierS
	case priceRatio >= cfg.TierQRatio:
		return TierQ
	default:
		return TierAll
	}
}

// SellPct returns the fraction of remaining balance to sell for the given tier.
func SellPct(tier int32, cfg Config) float64 {
	switch tier {
	case TierS:
		return cfg.TierSPct / 100.0
	case TierQ:
		return cfg.TierQPct / 100.0
	default:
		return cfg.TierAllPct / 100.0
	}
}
