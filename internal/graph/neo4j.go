package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Writer struct {
	driver neo4j.DriverWithContext
}

type Edge struct {
	Signature      string
	BlockSlot      int64
	BlockTime      int64
	From           string
	To             string
	Asset          string  // "SOL" for v1
	AmountLamports int64
	Kind           string  // "system_transfer"
	Confidence     float64
	SourceProgram  string
}

func New(ctx context.Context, uri, user, password string) (*Writer, error) {
	drv, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, password, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j driver: %w", err)
	}
	if err := drv.VerifyConnectivity(ctx); err != nil {
		drv.Close(ctx)
		return nil, fmt.Errorf("neo4j connectivity: %w", err)
	}
	return &Writer{driver: drv}, nil
}

func (w *Writer) Close(ctx context.Context) error { return w.driver.Close(ctx) }

func (w *Writer) EnsureConstraints(ctx context.Context) error {
	sess := w.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer sess.Close(ctx)
	_, err := sess.Run(ctx,
		`CREATE CONSTRAINT wallet_address IF NOT EXISTS FOR (w:Wallet) REQUIRE w.address IS UNIQUE`,
		nil)
	return err
}

func (w *Writer) UpsertEdges(ctx context.Context, seedSource string, edges []Edge) error {
	if len(edges) == 0 {
		return nil
	}
	sess := w.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer sess.Close(ctx)

	params := map[string]any{
		"edges":  toEdgeParams(edges),
		"source": seedSource,
	}
	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			UNWIND $edges AS e
			MERGE (a:Wallet {address: e.from})
			  ON CREATE SET a.first_seen_at = datetime(), a.source = $source
			MERGE (b:Wallet {address: e.to})
			  ON CREATE SET b.first_seen_at = datetime(), b.source = 'discovered'
			MERGE (a)-[t:TRANSFER {
				signature:       e.signature,
				from:            e.from,
				to:              e.to,
				asset:           e.asset,
				amount_lamports: e.amount_lamports,
				kind:            e.kind
			}]->(b)
			ON CREATE SET
				t.block_slot     = e.block_slot,
				t.block_time     = e.block_time,
				t.confidence     = e.confidence,
				t.source_program = e.source_program,
				t.inserted_at    = datetime()
		`, params)
		return nil, err
	})
	return err
}

func (w *Writer) UpsertWallet(ctx context.Context, address, source string) error {
	sess := w.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer sess.Close(ctx)
	_, err := sess.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx,
			`MERGE (a:Wallet {address: $address})
			   ON CREATE SET a.first_seen_at = datetime(), a.source = $source`,
			map[string]any{"address": address, "source": source})
		return nil, err
	})
	return err
}

func toEdgeParams(edges []Edge) []map[string]any {
	out := make([]map[string]any, len(edges))
	for i, e := range edges {
		out[i] = map[string]any{
			"signature":       e.Signature,
			"block_slot":      e.BlockSlot,
			"block_time":      e.BlockTime,
			"from":            e.From,
			"to":              e.To,
			"asset":           e.Asset,
			"amount_lamports": e.AmountLamports,
			"kind":            e.Kind,
			"confidence":      e.Confidence,
			"source_program":  e.SourceProgram,
		}
	}
	return out
}
