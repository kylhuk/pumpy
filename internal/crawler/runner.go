package crawler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pumpy/internal/dune"
	"pumpy/internal/graph"
	"pumpy/internal/solana"
)

type Runner struct {
	cfg    Config
	pool   *pgxpool.Pool
	dune   *dune.Client
	lookup *PgLookup
	g      *graph.Writer
}

func NewRunner(cfg Config, pool *pgxpool.Pool, d *dune.Client, lookup *PgLookup, g *graph.Writer) *Runner {
	return &Runner{cfg: cfg, pool: pool, dune: d, lookup: lookup, g: g}
}

func (r *Runner) CrawlOnce(ctx context.Context, lease *WalletLease) (int, error) {
	offset := lease.NextOffset
	seenOldRun := 0
	pages := 0

	for pages < r.cfg.MaxPagesPerRun {
		started := time.Now()
		page, httpResp, err := r.dune.GetTransactions(ctx, lease.Wallet, r.cfg.PageLimit, offset)
		status := 0
		if httpResp != nil {
			status = httpResp.StatusCode
		}
		if err != nil {
			r.logRequest(ctx, lease.Wallet, status, 0, false, started, err.Error())
			return pages, err
		}
		r.logRequest(ctx, lease.Wallet, status, len(page.Transactions), page.NextOffset != nil, started, "")

		var edges []graph.Edge
		for _, dt := range page.Transactions {
			n, err := dune.Normalize(dt)
			if err != nil {
				log.Printf("crawl %s: normalize %s: %v", lease.Wallet, dt.Signature, err)
				_, _ = MarkSeen(ctx, r.pool, lease.Wallet, dt.Signature, dt.BlockSlot, dt.BlockTime)
				continue
			}

			inserted, err := MarkSeen(ctx, r.pool, lease.Wallet, n.Signature, n.BlockSlot, n.BlockTime)
			if err != nil {
				return pages, err
			}
			if !inserted {
				if !lease.IsBackfill {
					seenOldRun++
					if seenOldRun >= r.cfg.StopAfterSeen {
						if err := UpdateCrawlState(ctx, r.pool, lease.Wallet, nil, 0, 0); err != nil {
							return pages, err
						}
						if err := CompleteWallet(ctx, r.pool, lease.Wallet, false); err != nil {
							return pages, err
						}
						return pages, nil
					}
				}
				continue
			}
			seenOldRun = 0

			if n.Err != "" {
				if err := StoreTransactionMin(ctx, r.pool, n, false, "failed_tx"); err != nil {
					return pages, err
				}
				continue
			}

			cls, err := Classify(ctx, n, r.lookup)
			if err != nil {
				return pages, err
			}
			if err := StoreTransactionMin(ctx, r.pool, n, cls.Excluded, cls.Reason); err != nil {
				return pages, err
			}
			if cls.Excluded {
				continue
			}

			edges = append(edges, r.extractSOLEdges(n, lease.Wallet)...)
		}

		if len(edges) > 0 {
			if err := r.g.UpsertEdges(ctx, "pump_seed", edges); err != nil {
				return pages, fmt.Errorf("neo4j upsert: %w", err)
			}
		}

		pages++
		if err := UpdateCrawlState(ctx, r.pool, lease.Wallet, page.NextOffset, 1, len(page.Transactions)); err != nil {
			return pages, err
		}
		if page.NextOffset == nil {
			return pages, CompleteWallet(ctx, r.pool, lease.Wallet, true)
		}
		offset = *page.NextOffset
	}

	return pages, CompleteWallet(ctx, r.pool, lease.Wallet, false)
}

func (r *Runner) extractSOLEdges(n *dune.NormalizedTransaction, currentWallet string) []graph.Edge {
	var edges []graph.Edge
	consider := func(ix dune.NormalizedInstruction) {
		st, err := solana.DecodeSystemTransfer(ix.ProgramID, ix.Accounts, ix.Data)
		if err != nil || st == nil {
			return
		}
		if int64(st.Lamports) < r.cfg.MinTransferLamps {
			return
		}
		if st.From == st.To {
			return
		}
		if st.From != currentWallet && st.To != currentWallet {
			return
		}
		edges = append(edges, graph.Edge{
			Signature:      n.Signature,
			BlockSlot:      n.BlockSlot,
			BlockTime:      n.BlockTime,
			From:           st.From,
			To:             st.To,
			Asset:          "SOL",
			AmountLamports: int64(st.Lamports),
			Kind:           "system_transfer",
			Confidence:     1.0,
			SourceProgram:  solana.SystemProgram,
		})
	}
	for _, ix := range n.Instructions {
		consider(ix)
	}
	for _, ix := range n.InnerInstructions {
		consider(ix)
	}
	return edges
}

func (r *Runner) logRequest(ctx context.Context, wallet string, status, txCount int, hadNext bool, started time.Time, errStr string) {
	var errp *string
	if errStr != "" {
		errp = &errStr
	}
	_, err := r.pool.Exec(ctx, `
        INSERT INTO api_request_log (wallet, endpoint, status_code, response_tx_count, had_next_offset, started_at, completed_at, error)
        VALUES ($1, $2, $3, $4, $5, $6, now(), $7)
    `, wallet, "/beta/svm/transactions", status, txCount, hadNext, started, errp)
	if err != nil {
		log.Printf("log request: %v", err)
	}
}
