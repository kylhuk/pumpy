package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const initSQL = `
CREATE TABLE IF NOT EXISTS tokens (
    mint          TEXT        PRIMARY KEY,
    creator       TEXT        NOT NULL,
    name          TEXT,
    symbol        TEXT,
    uri           TEXT,
    created_at    TIMESTAMPTZ NOT NULL,
    migrated_at   TIMESTAMPTZ,
    last_trade_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS tokens_active
    ON tokens (last_trade_at)
    WHERE migrated_at IS NULL;

CREATE TABLE IF NOT EXISTS trades (
    signature         TEXT          NOT NULL,
    mint              TEXT          NOT NULL,
    trader            TEXT          NOT NULL,
    side              SMALLINT      NOT NULL,
    sol_lamports      BIGINT        NOT NULL,
    token_amount      NUMERIC(40,0) NOT NULL,
    new_token_balance NUMERIC(40,0) NOT NULL,
    market_cap_sol    DOUBLE PRECISION,
    captured_at       TIMESTAMPTZ   NOT NULL,
    PRIMARY KEY (captured_at, signature)
) PARTITION BY RANGE (captured_at);

CREATE INDEX IF NOT EXISTS trades_trader_time ON trades (trader, captured_at DESC);
CREATE INDEX IF NOT EXISTS trades_mint_time   ON trades (mint, captured_at DESC);

CREATE TABLE IF NOT EXISTS schema_version (version INT PRIMARY KEY);
INSERT INTO schema_version VALUES (1) ON CONFLICT DO NOTHING;
`

// initSQLv2 mirrors migrations/002_wallet_graph.sql — keep both in sync.
const initSQLv2 = `
-- Pump.fun program IDs to exclude. Bootstrapped from a constant; can be
-- amended at runtime by INSERTing additional rows.
CREATE TABLE IF NOT EXISTS pump_program_id (
  program_id TEXT PRIMARY KEY,
  label      TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-wallet crawl progress. Acts as both the queue and the durable state.
CREATE TABLE IF NOT EXISTS wallet_crawl_state (
  wallet                 TEXT PRIMARY KEY,
  backfill_complete      BOOLEAN NOT NULL DEFAULT FALSE,
  next_offset            TEXT,
  newest_seen_signature  TEXT,
  newest_seen_slot       BIGINT,
  newest_seen_block_time BIGINT,
  last_started_at        TIMESTAMPTZ,
  last_completed_at      TIMESTAMPTZ,
  pages_fetched          INTEGER NOT NULL DEFAULT 0,
  tx_seen_count          INTEGER NOT NULL DEFAULT 0,
  error_count            INTEGER NOT NULL DEFAULT 0,
  last_error             TEXT
);

-- Index supports the picker query: backfill-first, then stalest incremental.
CREATE INDEX IF NOT EXISTS wallet_crawl_state_due
  ON wallet_crawl_state (backfill_complete, last_completed_at NULLS FIRST);

-- Idempotency: which signatures we've already processed for which wallet.
-- Drives incremental-mode early-stop and pagination resume safety.
CREATE TABLE IF NOT EXISTS wallet_tx_seen (
  wallet     TEXT      NOT NULL,
  signature  TEXT      NOT NULL,
  block_slot BIGINT,
  block_time BIGINT,
  seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (wallet, signature)
);

-- Minimal per-tx metadata. Edges live in Neo4j; this table is for diagnostics
-- and pump-exclusion auditing only. Raw JSON is never persisted by default.
CREATE TABLE IF NOT EXISTS transaction_min (
  signature        TEXT PRIMARY KEY,
  block_slot       BIGINT,
  block_time       BIGINT,
  fee_lamports     BIGINT,
  err              TEXT,
  account_keys     JSONB,
  program_ids      JSONB,
  is_pump_excluded BOOLEAN NOT NULL DEFAULT FALSE,
  exclusion_reason TEXT,
  inserted_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every Dune request, for budget tracking and debugging.
CREATE TABLE IF NOT EXISTS api_request_log (
  id                BIGSERIAL PRIMARY KEY,
  wallet            TEXT,
  endpoint          TEXT NOT NULL,
  status_code       INTEGER,
  cu_cost           INTEGER NOT NULL DEFAULT 1,
  response_tx_count INTEGER,
  had_next_offset   BOOLEAN,
  started_at        TIMESTAMPTZ NOT NULL,
  completed_at      TIMESTAMPTZ,
  error             TEXT
);

CREATE INDEX IF NOT EXISTS api_request_log_started
  ON api_request_log (started_at DESC);

INSERT INTO schema_version (version) VALUES (2) ON CONFLICT DO NOTHING;
`

// ApplySchema runs migrations sequentially by version number, then ensures
// daily partitions exist for [today-1 .. today+7].
func (s *Store) ApplySchema(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_version (version INT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("ensure schema_version: %w", err)
	}

	if _, err := s.pool.Exec(ctx, `SELECT pg_advisory_lock(8675309)`); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer s.pool.Exec(ctx, `SELECT pg_advisory_unlock(8675309)`) //nolint:errcheck

	var current int
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if current < 1 {
		if _, err := s.pool.Exec(ctx, initSQL); err != nil {
			return fmt.Errorf("apply schema v1: %w", err)
		}
		log.Printf("schema: applied v1")
	}
	if current < 2 {
		if _, err := s.pool.Exec(ctx, initSQLv2); err != nil {
			return fmt.Errorf("apply schema v2: %w", err)
		}
		log.Printf("schema: applied v2")
	}
	return EnsurePartitions(ctx, s.pool)
}

// EnsurePartitions creates daily partitions for the range [today-1 .. today+7].
func EnsurePartitions(ctx context.Context, pool *pgxpool.Pool) error {
	now := time.Now().UTC()
	for offset := -1; offset <= 7; offset++ {
		day := now.AddDate(0, 0, offset)
		if err := createPartition(ctx, pool, day); err != nil {
			return err
		}
	}
	return nil
}

// DropOldPartitions drops partitions older than retainDays.
func DropOldPartitions(ctx context.Context, pool *pgxpool.Pool, retainDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -retainDays)
	for d := cutoff.AddDate(0, 0, -30); d.Before(cutoff); d = d.AddDate(0, 0, 1) {
		name := partName(d)
		sql := fmt.Sprintf("DROP TABLE IF EXISTS %s", name)
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("drop partition %s: %w", name, err)
		}
	}
	return nil
}

func createPartition(ctx context.Context, pool *pgxpool.Pool, day time.Time) error {
	name := partName(day)
	from := day.Format("2006-01-02")
	to := day.AddDate(0, 0, 1).Format("2006-01-02")
	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF trades
		 FOR VALUES FROM ('%s') TO ('%s')`,
		name, from, to,
	)
	if _, err := pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("create partition %s: %w", name, err)
	}
	return nil
}

func partName(day time.Time) string {
	return "trades_" + day.Format("20060102")
}
