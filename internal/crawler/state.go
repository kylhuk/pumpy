package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pumpy/internal/dune"
)

type PgLookup struct {
	pool     *pgxpool.Pool
	programs map[string]bool
	mu       sync.RWMutex
}

func NewPgLookup(ctx context.Context, pool *pgxpool.Pool) (*PgLookup, error) {
	p := &PgLookup{pool: pool, programs: make(map[string]bool)}
	if err := p.RefreshPrograms(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *PgLookup) RefreshPrograms(ctx context.Context) error {
	rows, err := p.pool.Query(ctx, `SELECT program_id FROM pump_program_id`)
	if err != nil {
		return fmt.Errorf("load programs: %w", err)
	}
	defer rows.Close()
	next := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		next[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	p.programs = next
	p.mu.Unlock()
	return nil
}

func (p *PgLookup) IsPumpProgram(programID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.programs[programID]
}

func (p *PgLookup) IsKnownPumpSignature(ctx context.Context, sig string) (bool, error) {
	var exists bool
	err := p.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM trades WHERE signature = $1)`, sig).Scan(&exists)
	return exists, err
}

func SeedPrograms(ctx context.Context, pool *pgxpool.Pool, seed []struct{ ID, Label string }) error {
	for _, s := range seed {
		_, err := pool.Exec(ctx,
			`INSERT INTO pump_program_id (program_id, label) VALUES ($1, $2)
             ON CONFLICT (program_id) DO NOTHING`, s.ID, s.Label)
		if err != nil {
			return fmt.Errorf("seed program %s: %w", s.ID, err)
		}
	}
	return nil
}

func SyncWalletQueue(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
        INSERT INTO wallet_crawl_state (wallet)
        SELECT DISTINCT trader FROM trades
        ON CONFLICT (wallet) DO NOTHING
    `)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

type WalletLease struct {
	Wallet     string
	NextOffset string
	IsBackfill bool
}

func PickNextWallet(ctx context.Context, pool *pgxpool.Pool, incrementalAge time.Duration) (*WalletLease, func(context.Context) error, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	var w WalletLease
	var nextOffset *string
	var done bool
	err = tx.QueryRow(ctx, `
        SELECT wallet, backfill_complete, next_offset
          FROM wallet_crawl_state
         WHERE backfill_complete = FALSE
            OR last_completed_at IS NULL
            OR last_completed_at < now() - make_interval(secs => $1)
         ORDER BY backfill_complete ASC, last_completed_at NULLS FIRST
         LIMIT 1
         FOR UPDATE SKIP LOCKED`,
		int(incrementalAge.Seconds())).Scan(&w.Wallet, &done, &nextOffset)
	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			log.Printf("rollback wallet_crawl_state lease: %v", rbErr)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	w.IsBackfill = !done
	if nextOffset != nil {
		w.NextOffset = *nextOffset
	}
	if _, err := tx.Exec(ctx,
		`UPDATE wallet_crawl_state SET last_started_at = now() WHERE wallet = $1`, w.Wallet); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			log.Printf("rollback wallet_crawl_state lease: %v", rbErr)
		}
		return nil, nil, err
	}
	return &w, func(ctx context.Context) error { return tx.Commit(ctx) }, nil
}

func MarkSeen(ctx context.Context, pool *pgxpool.Pool, wallet, sig string, slot, blockTime int64) (bool, error) {
	tag, err := pool.Exec(ctx, `
        INSERT INTO wallet_tx_seen (wallet, signature, block_slot, block_time)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT DO NOTHING`, wallet, sig, slot, blockTime)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func StoreTransactionMin(ctx context.Context, pool *pgxpool.Pool, n *dune.NormalizedTransaction, isPumpExcluded bool, exclusionReason string) error {
	accountKeys, err := json.Marshal(n.AccountKeys)
	if err != nil {
		return fmt.Errorf("marshal account_keys: %w", err)
	}
	progIDs := make([]string, 0, len(n.ProgramIDs))
	for k := range n.ProgramIDs {
		progIDs = append(progIDs, k)
	}
	pj, err := json.Marshal(progIDs)
	if err != nil {
		return fmt.Errorf("marshal program_ids: %w", err)
	}
	var errStr *string
	if n.Err != "" {
		errStr = &n.Err
	}
	var excReason *string
	if exclusionReason != "" {
		excReason = &exclusionReason
	}
	_, err = pool.Exec(ctx, `
        INSERT INTO transaction_min (signature, block_slot, block_time, fee_lamports, err, account_keys, program_ids, is_pump_excluded, exclusion_reason)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (signature) DO NOTHING`,
		n.Signature, n.BlockSlot, n.BlockTime, n.FeeLamports, errStr, accountKeys, pj, isPumpExcluded, excReason)
	return err
}

func UpdateCrawlState(ctx context.Context, pool *pgxpool.Pool, wallet string, nextOffset *string, pagesDelta, txDelta int) error {
	_, err := pool.Exec(ctx, `
        UPDATE wallet_crawl_state
           SET next_offset = $2,
               pages_fetched = pages_fetched + $3,
               tx_seen_count = tx_seen_count + $4
         WHERE wallet = $1`, wallet, nextOffset, pagesDelta, txDelta)
	return err
}

func CompleteWallet(ctx context.Context, pool *pgxpool.Pool, wallet string, backfillDone bool) error {
	_, err := pool.Exec(ctx, `
        UPDATE wallet_crawl_state
           SET backfill_complete = backfill_complete OR $2,
               next_offset = CASE WHEN $2 THEN NULL ELSE next_offset END,
               last_completed_at = now()
         WHERE wallet = $1`, wallet, backfillDone)
	return err
}

func RecordError(ctx context.Context, pool *pgxpool.Pool, wallet, errMsg string) error {
	_, err := pool.Exec(ctx, `
        UPDATE wallet_crawl_state
           SET error_count = error_count + 1, last_error = $2
         WHERE wallet = $1`, wallet, errMsg)
	return err
}
