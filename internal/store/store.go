package store

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	flushSize     = 500
	flushInterval = 200 * time.Millisecond
)

// TradeRow is a single row destined for the trades table.
type TradeRow struct {
	Signature       string
	Mint            string
	Trader          string
	Side            int16 // 0=buy 1=sell 2=create
	SolLamports     int64
	TokenAmount     string // NUMERIC(40,0) as string
	NewTokenBalance string
	MarketCapSol    *float64
	CapturedAt      time.Time
}

// TokenRow is a row for the tokens table.
type TokenRow struct {
	Mint      string
	Creator   string
	Name      string
	Symbol    string
	URI       string
	CreatedAt time.Time
}

// Store manages the pgx pool and batched writes.
type Store struct {
	pool  *pgxpool.Pool
	queue chan TradeRow
}

func New(ctx context.Context, connStr string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("store: parse config: %w", err)
	}
	cfg.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	s := &Store{
		pool:  pool,
		queue: make(chan TradeRow, 8192),
	}
	go s.writer(ctx)
	return s, nil
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// EnqueueTrade adds a trade to the async flush queue.
func (s *Store) EnqueueTrade(r TradeRow) {
	s.queue <- r
}

// UpsertToken inserts or updates a token record.
func (s *Store) UpsertToken(ctx context.Context, t TokenRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tokens (mint, creator, name, symbol, uri, created_at, last_trade_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (mint) DO UPDATE SET last_trade_at = EXCLUDED.created_at`,
		t.Mint, t.Creator, t.Name, t.Symbol, t.URI, t.CreatedAt,
	)
	return err
}

// MarkMigrated sets migrated_at on a token.
func (s *Store) MarkMigrated(ctx context.Context, mint string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE tokens SET migrated_at = now() WHERE mint = $1`, mint)
	return err
}

// ActiveMints returns mints that are unmigrated and had a trade within the last 24 h.
func (s *Store) ActiveMints(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT mint FROM tokens
		WHERE migrated_at IS NULL
		  AND last_trade_at >= now() - interval '24 hours'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var mints []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		mints = append(mints, m)
	}
	return mints, rows.Err()
}

// UpdateLastTrade bumps last_trade_at on a token.
func (s *Store) UpdateLastTrade(ctx context.Context, mint string, at time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE tokens SET last_trade_at = $2 WHERE mint = $1 AND (last_trade_at IS NULL OR last_trade_at < $2)`,
		mint, at)
	return err
}

// writer is the background goroutine that flushes trade batches.
func (s *Store) writer(ctx context.Context) {
	batch := make([]TradeRow, 0, flushSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.copyTrades(ctx, batch); err != nil {
			log.Printf("store: flush error: %v", err)
		} else {
			log.Printf("store: flushed %d trades", len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case row := <-s.queue:
			batch = append(batch, row)
			if len(batch) >= flushSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}

func (s *Store) copyTrades(ctx context.Context, rows []TradeRow) error {
	_, err := s.pool.CopyFrom(ctx,
		pgx.Identifier{"trades"},
		[]string{"signature", "mint", "trader", "side", "sol_lamports",
			"token_amount", "new_token_balance", "market_cap_sol", "captured_at"},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			r := rows[i]
			return []any{
				r.Signature, r.Mint, r.Trader, r.Side, r.SolLamports,
				r.TokenAmount, r.NewTokenBalance, r.MarketCapSol, r.CapturedAt,
			}, nil
		}),
	)
	return err
}

// SolToLamports converts a SOL float to integer lamports.
func SolToLamports(sol float64) int64 {
	return int64(math.Round(sol * 1e9))
}

// FormatTokenAmount converts a float token amount to a string suitable for NUMERIC(40,0).
func FormatTokenAmount(f float64) string {
	return fmt.Sprintf("%.0f", f)
}
