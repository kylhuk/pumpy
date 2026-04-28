package dashboard

import (
	"time"

	"pumpy/internal/graph"
	"pumpy/internal/store"
)

// Result holds either a value or an error from one metric query.
// The renderer uses Err to decide whether to show "n/a" (with red indicator).
type Result[T any] struct {
	Val T
	Err error
}

// OK returns true when the result has a valid value.
func (r Result[T]) OK() bool { return r.Err == nil }

// DashboardData holds one Result field per metric, populated by the
// metrics-fetching layer and consumed by the renderer.
type DashboardData struct {
	// Operational health (derive status lights from these)
	LastTradeAt    Result[time.Time] // MAX(captured_at) from trades
	LastDuneCallAt Result[time.Time] // MAX(completed_at) from api_request_log (success only)
	Neo4jOK        Result[bool]      // true if Neo4j connected and queries succeeded

	// Headline counts
	TotalWallets    Result[int64]   // DISTINCT traders ever
	ActiveWallets24h Result[int64]  // DISTINCT traders in last 24h
	Volume24hSOL    Result[float64] // SUM(sol_lamports)/1e9 last 24h
	NewTokens24h    Result[int64]   // tokens.created_at > now-24h
	MigratedTotal   Result[int64]   // tokens.migrated_at IS NOT NULL (all-time)
	MigratedLast24h Result[int64]   // migrated_at > now-24h

	// Live pulse
	TradesPerMinute Result[int64] // trades in the last 60 seconds
	BuysLast24h     Result[int64] // side=0 count
	SellsLast24h    Result[int64] // side=1 count

	// Hottest token
	HottestTokenSymbol Result[string] // symbol of most-traded mint last hour
	HottestTokenTrades Result[int64]  // trade count for that mint last hour

	// Crawler state
	CrawlerTotal    Result[int64] // total rows in wallet_crawl_state
	CrawlerComplete Result[int64] // backfill_complete = true
	CrawlerErrors   Result[int64] // sum(error_count) > 0

	// PnL tables (24h, closed positions only)
	Top5Traders  Result[[]PnLRow]
	Flop5Traders Result[[]PnLRow]

	// Graph stats (Neo4j)
	DiscoveredWallets   Result[int64]          // Wallet{source:"discovered"} count
	GraphEdges          Result[int64]          // total TRANSFER edges
	TopExternalWallets  Result[[]ExternalRow]  // pump wallets with most non-pump trades
}

// PnLRow represents a single row in the realized PnL leaderboard.
type PnLRow struct {
	Rank   int
	Wallet string
	PnLSOL float64
}

// ExternalRow represents a wallet with notable external (non-pump) trading activity.
type ExternalRow struct {
	Rank   int
	Wallet string
	Trades int64
}

// WindowedStats holds all per-time-window metrics for a single wallet.
type WindowedStats struct {
	PnLSOL            Result[float64]
	TradeCount        Result[int64]
	DistinctTokens    Result[int64]
	TopTokens         Result[[]store.WalletTopToken]
	TopCounterparties Result[[]graph.Counterparty]
}

// WhoisData holds all metrics for a single wallet, consumed by RenderWhois.
type WhoisData struct {
	Wallet            string
	Now               time.Time
	Neo4jOK           bool
	Window24h         WindowedStats
	Window7d          WindowedStats
	Window14d         WindowedStats
	AllCounterparties Result[[]graph.Counterparty]
}
