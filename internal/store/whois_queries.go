package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WalletTopToken is a pump.fun token ranked by the wallet's traded SOL volume.
type WalletTopToken struct {
	Mint       string
	Symbol     string
	Name       string
	SOLVolume  float64
	TradeCount int64
}

// WalletRealizedPnLSOL returns the sum of realized PnL (in SOL) for closed positions
// opened and closed within the given Postgres interval window (e.g. "24 hours").
func WalletRealizedPnLSOL(ctx context.Context, pool *pgxpool.Pool, wallet, window string) (float64, error) {
	sql := fmt.Sprintf(`
WITH last_bal AS (
    SELECT DISTINCT ON (mint) mint, new_token_balance
    FROM   trades
    WHERE  trader = $1
      AND  captured_at >= now() - interval '%s'
    ORDER  BY mint, captured_at DESC
),
closed AS (
    SELECT mint FROM last_bal WHERE new_token_balance = 0
),
flows AS (
    SELECT coalesce(
        SUM(CASE WHEN side = 1 THEN sol_lamports ELSE 0 END) -
        SUM(CASE WHEN side = 0 THEN sol_lamports ELSE 0 END), 0
    ) AS pnl_lamports
    FROM trades
    WHERE trader = $1
      AND captured_at >= now() - interval '%s'
      AND mint IN (SELECT mint FROM closed)
)
SELECT pnl_lamports::double precision / 1e9 FROM flows`, window, window)

	var pnl float64
	err := pool.QueryRow(ctx, sql, wallet).Scan(&pnl)
	return pnl, err
}

// WalletTradeCount returns the number of trades the wallet made within window.
func WalletTradeCount(ctx context.Context, pool *pgxpool.Pool, wallet, window string) (int64, error) {
	sql := fmt.Sprintf(`
SELECT count(*)
FROM   trades
WHERE  trader = $1
  AND  captured_at >= now() - interval '%s'`, window)

	var n int64
	err := pool.QueryRow(ctx, sql, wallet).Scan(&n)
	return n, err
}

// WalletDistinctPumpTokens returns the number of distinct pump.fun tokens the wallet
// traded within window. Every mint in trades is by definition a pump.fun token.
func WalletDistinctPumpTokens(ctx context.Context, pool *pgxpool.Pool, wallet, window string) (int64, error) {
	sql := fmt.Sprintf(`
SELECT count(DISTINCT mint)
FROM   trades
WHERE  trader = $1
  AND  captured_at >= now() - interval '%s'`, window)

	var n int64
	err := pool.QueryRow(ctx, sql, wallet).Scan(&n)
	return n, err
}

// WalletTopTokensByVolume returns up to limit pump.fun tokens ranked by SOL volume
// traded by the wallet within window.
func WalletTopTokensByVolume(ctx context.Context, pool *pgxpool.Pool, wallet, window string, limit int) ([]WalletTopToken, error) {
	if limit <= 0 {
		limit = 5
	}
	sql := fmt.Sprintf(`
SELECT t.mint,
       coalesce(tk.symbol, ''),
       coalesce(tk.name, ''),
       sum(t.sol_lamports)::double precision / 1e9 AS sol_volume,
       count(*)                                    AS trades
FROM   trades t
LEFT JOIN tokens tk ON tk.mint = t.mint
WHERE  t.trader = $1
  AND  t.captured_at >= now() - interval '%s'
GROUP  BY t.mint, tk.symbol, tk.name
ORDER  BY sol_volume DESC
LIMIT  $2`, window)

	rows, err := pool.Query(ctx, sql, wallet, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WalletTopToken
	for rows.Next() {
		var tok WalletTopToken
		if err := rows.Scan(&tok.Mint, &tok.Symbol, &tok.Name, &tok.SOLVolume, &tok.TradeCount); err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
	return out, rows.Err()
}
