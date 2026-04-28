package graph

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// newReadSession opens a read-only session for snapshot queries.
func (w *Writer) newReadSession(ctx context.Context) neo4j.SessionWithContext {
	return w.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
}

// TopExternal holds results for TopPumpWalletsByExternalTrades.
type TopExternal struct {
	Wallet string
	Trades int64
}

// DiscoveredWalletCount returns the number of Wallet nodes with source='discovered'.
func (w *Writer) DiscoveredWalletCount(ctx context.Context) (int64, error) {
	sess := w.newReadSession(ctx)
	defer sess.Close(ctx)
	result, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		r, err := tx.Run(ctx, `MATCH (w:Wallet {source: 'discovered'}) RETURN count(w) AS n`, nil)
		if err != nil {
			return int64(0), err
		}
		rec, err := r.Single(ctx)
		if err != nil {
			return int64(0), err
		}
		return rec.Values[0].(int64), nil
	})
	if err != nil {
		return 0, err
	}
	return result.(int64), nil
}

// EdgeCount returns the number of TRANSFER relationships in the graph.
// It tries the fast APOC path first and falls back to a full scan.
func (w *Writer) EdgeCount(ctx context.Context) (int64, error) {
	sess := w.newReadSession(ctx)
	defer sess.Close(ctx)
	result, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Try fast APOC path first
		r, err := tx.Run(ctx, `CALL apoc.meta.stats() YIELD relTypesCount RETURN relTypesCount`, nil)
		if err == nil {
			rec, recErr := r.Single(ctx)
			if recErr == nil {
				if m, ok := rec.Values[0].(map[string]any); ok {
					if n, ok := m["()-[:TRANSFER]->()"]; ok {
						return toInt64(n), nil
					}
				}
			}
		}
		// Fall back to full relationship scan
		r2, err2 := tx.Run(ctx, `MATCH ()-[r:TRANSFER]->() RETURN count(r) AS n`, nil)
		if err2 != nil {
			return int64(0), err2
		}
		rec2, err2 := r2.Single(ctx)
		if err2 != nil {
			return int64(0), err2
		}
		return rec2.Values[0].(int64), nil
	})
	if err != nil {
		return 0, err
	}
	return result.(int64), nil
}

// TopPumpWalletsByExternalTrades returns the top pump_seed wallets ranked by
// outbound TRANSFER count to discovered wallets.
func (w *Writer) TopPumpWalletsByExternalTrades(ctx context.Context, limit int) ([]TopExternal, error) {
	if limit <= 0 {
		limit = 5
	}
	sess := w.newReadSession(ctx)
	defer sess.Close(ctx)
	results, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		r, err := tx.Run(ctx,
			`MATCH (a:Wallet {source: 'pump_seed'})-[t:TRANSFER]->(b:Wallet {source: 'discovered'})
             RETURN a.address AS wallet, count(t) AS n
             ORDER BY n DESC
             LIMIT $limit`,
			map[string]any{"limit": limit},
		)
		if err != nil {
			return nil, err
		}
		var rows []TopExternal
		for r.Next(ctx) {
			rec := r.Record()
			rows = append(rows, TopExternal{
				Wallet: rec.Values[0].(string),
				Trades: rec.Values[1].(int64),
			})
		}
		return rows, r.Err()
	})
	if err != nil {
		return nil, err
	}
	if results == nil {
		return nil, nil
	}
	return results.([]TopExternal), nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}
