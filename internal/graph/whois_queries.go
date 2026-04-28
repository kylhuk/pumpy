package graph

import (
	"context"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Counterparty is a wallet that received outbound SOL transfers from the queried wallet.
type Counterparty struct {
	Address       string
	SOLVolume     float64 // amount_lamports / 1e9
	TransferCount int64
}

// WalletOutboundCounterparties returns outbound :TRANSFER targets for wallet, ordered by
// total SOL sent descending. If since is non-zero, only edges with block_time >= since.Unix()
// are included. limit <= 0 returns all rows.
func (w *Writer) WalletOutboundCounterparties(ctx context.Context, wallet string, since time.Time, limit int) ([]Counterparty, error) {
	sinceUnix := int64(0)
	if !since.IsZero() {
		sinceUnix = since.Unix()
	}

	cypher := `
MATCH (a:Wallet {address: $wallet})-[t:TRANSFER]->(b:Wallet)
WHERE $since = 0 OR t.block_time >= $since
RETURN b.address    AS wallet,
       sum(t.amount_lamports) AS total_lamports,
       count(t)     AS tx_count
ORDER BY total_lamports DESC`

	params := map[string]any{
		"wallet": wallet,
		"since":  sinceUnix,
	}
	if limit > 0 {
		cypher += "\nLIMIT $limit"
		params["limit"] = int64(limit)
	}

	sess := w.newReadSession(ctx)
	defer sess.Close(ctx)

	results, err := sess.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		r, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		var rows []Counterparty
		for r.Next(ctx) {
			rec := r.Record()
			rows = append(rows, Counterparty{
				Address:       rec.Values[0].(string),
				SOLVolume:     float64(toInt64(rec.Values[1])) / 1e9,
				TransferCount: toInt64(rec.Values[2]),
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
	return results.([]Counterparty), nil
}
