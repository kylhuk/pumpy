package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Order controls result ordering for ranked queries.
type Order int

const (
	OrderDesc Order = iota
	OrderAsc
)

type PnLEntry struct {
	Trader           string
	RealizedPnLSOL   float64
	RealizedPnLLamps int64
}

// RealizedPnL returns wallets ranked by realized PnL (closed positions only).
// window must be a valid Postgres interval string (e.g. "24 hours").
// order controls whether results are sorted DESC (top earners) or ASC (worst performers).
func RealizedPnL(ctx context.Context, pool *pgxpool.Pool, window string, limit int, order Order) ([]PnLEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	dir := "DESC"
	if order == OrderAsc {
		dir = "ASC"
	}
	sql := fmt.Sprintf(`
WITH last_bal AS (
    SELECT DISTINCT ON (trader, mint)
           trader, mint, new_token_balance
    FROM   trades
    WHERE  captured_at >= now() - interval '%s'
    ORDER  BY trader, mint, captured_at DESC
),
closed AS (
    SELECT trader, mint FROM last_bal WHERE new_token_balance = 0
),
flows AS (
    SELECT  t.trader,
            SUM(CASE WHEN t.side = 1 THEN t.sol_lamports ELSE 0 END) -
            SUM(CASE WHEN t.side = 0 THEN t.sol_lamports ELSE 0 END) AS pnl
    FROM    trades t
    JOIN    closed c ON c.trader = t.trader AND c.mint = t.mint
    WHERE   t.captured_at >= now() - interval '%s'
    GROUP   BY t.trader
)
SELECT trader, pnl
FROM   flows
ORDER  BY pnl %s
LIMIT  $1`, window, window, dir)

	rows, err := pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PnLEntry
	for rows.Next() {
		var e PnLEntry
		if err := rows.Scan(&e.Trader, &e.RealizedPnLLamps); err != nil {
			return nil, err
		}
		e.RealizedPnLSOL = float64(e.RealizedPnLLamps) / 1e9
		out = append(out, e)
	}
	return out, rows.Err()
}
