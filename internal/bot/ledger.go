package bot

import (
	"sync"
	"sync/atomic"
	"time"
)

// TxRecord is one entry in the recent-transactions ring buffer.
type TxRecord struct {
	At        time.Time
	Mint      string
	Side      string  // "buy" or "sell"
	SolAmount float64
	McapSol   float64
	Signature string
}

// Ledger tracks session-level counters and recent transactions.
// All methods are safe for concurrent use.
type Ledger struct {
	// fixed-point lamports (×1e9) for atomic ops
	solSpentFixed  atomic.Uint64
	solSoldFixed   atomic.Uint64
	buyCount       atomic.Int64
	sellCount      atomic.Int64
	openPositions  atomic.Int64
	closedPositions atomic.Int64
	winCount       atomic.Int64

	startBalanceLamports  uint64 // set once at startup, immutable
	liveBalanceLamports   atomic.Uint64

	mu      sync.RWMutex
	history []TxRecord
	maxHistory int
}

func NewLedger(maxHistory int) *Ledger {
	return &Ledger{maxHistory: maxHistory, history: make([]TxRecord, 0, maxHistory)}
}

func (l *Ledger) SetStartBalance(lamports uint64) {
	l.startBalanceLamports = lamports
	l.liveBalanceLamports.Store(lamports)
}

func (l *Ledger) SetLiveBalance(lamports uint64) { l.liveBalanceLamports.Store(lamports) }

func (l *Ledger) StartBalanceLamports() uint64 { return l.startBalanceLamports }
func (l *Ledger) LiveBalanceLamports() uint64  { return l.liveBalanceLamports.Load() }

func (l *Ledger) StartBalanceSol() float64 { return float64(l.startBalanceLamports) / 1e9 }
func (l *Ledger) LiveBalanceSol() float64  { return float64(l.liveBalanceLamports.Load()) / 1e9 }

func (l *Ledger) BalancePct() float64 {
	if l.startBalanceLamports == 0 {
		return 0
	}
	return float64(l.liveBalanceLamports.Load()) / float64(l.startBalanceLamports) * 100.0
}

func (l *Ledger) RecordBuy(sol float64, rec TxRecord) {
	l.solSpentFixed.Add(uint64(sol * 1e9))
	l.buyCount.Add(1)
	l.openPositions.Add(1)
	l.addHistory(rec)
}

func (l *Ledger) RecordSell(sol float64, rec TxRecord) {
	l.solSoldFixed.Add(uint64(sol * 1e9))
	l.sellCount.Add(1)
	l.addHistory(rec)
}

func (l *Ledger) RecordPositionClosed(win bool) {
	l.openPositions.Add(-1)
	l.closedPositions.Add(1)
	if win {
		l.winCount.Add(1)
	}
}

func (l *Ledger) SolSpent() float64   { return float64(l.solSpentFixed.Load()) / 1e9 }
func (l *Ledger) SolSold() float64    { return float64(l.solSoldFixed.Load()) / 1e9 }
func (l *Ledger) PnL() float64        { return l.SolSold() - l.SolSpent() }
func (l *Ledger) OpenPositions() int64  { return l.openPositions.Load() }
func (l *Ledger) ClosedPositions() int64 { return l.closedPositions.Load() }

func (l *Ledger) WinRate() float64 {
	closed := l.closedPositions.Load()
	if closed == 0 {
		return 0
	}
	return float64(l.winCount.Load()) / float64(closed) * 100.0
}

func (l *Ledger) addHistory(rec TxRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.history = append(l.history, rec)
	if len(l.history) > l.maxHistory {
		l.history = l.history[len(l.history)-l.maxHistory:]
	}
}

// History returns a snapshot of recent transactions, newest first.
func (l *Ledger) History() []TxRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]TxRecord, len(l.history))
	for i, r := range l.history {
		out[len(l.history)-1-i] = r
	}
	return out
}
