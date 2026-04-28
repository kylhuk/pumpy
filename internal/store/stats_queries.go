package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WalletCount returns the total number of distinct traders ever recorded.
// now is accepted for interface consistency but not used in the query.
func WalletCount(ctx context.Context, pool *pgxpool.Pool, now time.Time) (int64, error) {
	var n int64
	err := pool.QueryRow(ctx, `SELECT count(DISTINCT trader) FROM trades`).Scan(&n)
	return n, err
}

// Volume24h returns the total SOL volume traded in the last 24h (as float64 SOL).
func Volume24h(ctx context.Context, pool *pgxpool.Pool, now time.Time) (float64, error) {
	var lamports int64
	err := pool.QueryRow(ctx,
		`SELECT coalesce(sum(sol_lamports), 0) FROM trades WHERE captured_at >= $1`,
		now.Add(-24*time.Hour),
	).Scan(&lamports)
	if err != nil {
		return 0, err
	}
	return float64(lamports) / 1e9, nil
}

// NewTokens24h returns the number of tokens created in the last 24h.
func NewTokens24h(ctx context.Context, pool *pgxpool.Pool, now time.Time) (int64, error) {
	var n int64
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM tokens WHERE created_at >= $1`,
		now.Add(-24*time.Hour),
	).Scan(&n)
	return n, err
}

// MigratedTokens returns the total number of migrated tokens and the count migrated in the last 24h.
func MigratedTokens(ctx context.Context, pool *pgxpool.Pool, now time.Time) (total int64, last24h int64, err error) {
	err = pool.QueryRow(ctx, `
SELECT
    count(*) FILTER (WHERE migrated_at IS NOT NULL) AS total,
    count(*) FILTER (WHERE migrated_at >= $1)       AS last_24h
FROM tokens`,
		now.Add(-24*time.Hour),
	).Scan(&total, &last24h)
	return
}

// ActiveWallets24h returns the number of distinct traders active in the last 24h.
func ActiveWallets24h(ctx context.Context, pool *pgxpool.Pool, now time.Time) (int64, error) {
	var n int64
	err := pool.QueryRow(ctx,
		`SELECT count(DISTINCT trader) FROM trades WHERE captured_at >= $1`,
		now.Add(-24*time.Hour),
	).Scan(&n)
	return n, err
}

// TradesPerMinute returns the number of trades captured in the last minute.
func TradesPerMinute(ctx context.Context, pool *pgxpool.Pool, now time.Time) (int64, error) {
	var n int64
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM trades WHERE captured_at >= $1`,
		now.Add(-1*time.Minute),
	).Scan(&n)
	return n, err
}

// BuySellRatio24h returns the count of buys and sells in the last 24h.
func BuySellRatio24h(ctx context.Context, pool *pgxpool.Pool, now time.Time) (buys int64, sells int64, err error) {
	err = pool.QueryRow(ctx, `
SELECT
    count(*) FILTER (WHERE side = 0) AS buys,
    count(*) FILTER (WHERE side = 1) AS sells
FROM trades
WHERE captured_at >= $1`,
		now.Add(-24*time.Hour),
	).Scan(&buys, &sells)
	return
}

// HottestToken1h returns the symbol (or mint if no symbol) and trade count of the
// most-traded token in the last hour. Returns ("", 0, nil) if there are no trades.
func HottestToken1h(ctx context.Context, pool *pgxpool.Pool, now time.Time) (symbol string, tradeCount int64, err error) {
	err = pool.QueryRow(ctx, `
WITH top AS (
    SELECT mint, count(*) AS n
    FROM trades
    WHERE captured_at >= $1
    GROUP BY mint
    ORDER BY n DESC
    LIMIT 1
)
SELECT coalesce(t.symbol, top.mint), top.n
FROM top
LEFT JOIN tokens t ON t.mint = top.mint`,
		now.Add(-1*time.Hour),
	).Scan(&symbol, &tradeCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, nil
	}
	return
}

// CrawlerQueueStatus returns total wallets, completed wallets, and wallets with errors.
// now is accepted for interface consistency but not used in the query.
func CrawlerQueueStatus(ctx context.Context, pool *pgxpool.Pool, now time.Time) (total int64, complete int64, errorWallets int64, err error) {
	err = pool.QueryRow(ctx, `
SELECT
    count(*)                                  AS total,
    count(*) FILTER (WHERE backfill_complete) AS complete,
    count(*) FILTER (WHERE error_count > 0)   AS error_wallets
FROM wallet_crawl_state`,
	).Scan(&total, &complete, &errorWallets)
	return
}

// LastTradeAt returns the timestamp of the most recent trade, or 1970-01-01 if none.
// now is accepted for interface consistency but not used in the query.
func LastTradeAt(ctx context.Context, pool *pgxpool.Pool, now time.Time) (time.Time, error) {
	var t time.Time
	err := pool.QueryRow(ctx,
		`SELECT coalesce(max(captured_at), '1970-01-01'::timestamptz) FROM trades`,
	).Scan(&t)
	return t, err
}

// LastSuccessfulDuneCallAt returns the timestamp of the most recent successful Dune API call,
// or 1970-01-01 if none. now is accepted for interface consistency but not used in the query.
func LastSuccessfulDuneCallAt(ctx context.Context, pool *pgxpool.Pool, now time.Time) (time.Time, error) {
	var t time.Time
	err := pool.QueryRow(ctx, `
SELECT coalesce(max(completed_at), '1970-01-01'::timestamptz)
FROM api_request_log
WHERE error IS NULL AND status_code BETWEEN 200 AND 299`,
	).Scan(&t)
	return t, err
}
