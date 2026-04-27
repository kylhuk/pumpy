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

// ApplySchema runs the init migration if not already applied, then ensures
// daily partitions exist for [today-1 .. today+7].
func ApplySchema(ctx context.Context, pool *pgxpool.Pool) error {
	var v int
	err := pool.QueryRow(ctx,
		"SELECT version FROM schema_version LIMIT 1",
	).Scan(&v)
	if err != nil || v < 1 {
		log.Println("store: applying schema migration 001")
		if _, err := pool.Exec(ctx, initSQL); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	return EnsurePartitions(ctx, pool)
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
