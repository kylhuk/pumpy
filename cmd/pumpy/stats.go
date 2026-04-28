package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/term"
	"pumpy/internal/dashboard"
	"pumpy/internal/graph"
	"pumpy/internal/store"
)

func runStats() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	now := time.Now().UTC()

	// Detect terminal width; fall back to 80 for non-TTY.
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 80
	}

	// --- Postgres connection ---
	dsn := envOrDefault("DATABASE_URL", "postgres://pumpy:pumpy@localhost:5432/pumpy?sslmode=disable")
	st, err := store.New(ctx, dsn)
	if err != nil {
		log.Fatalf("stats: postgres connect: %v", err)
	}
	defer st.Pool().Close()
	pool := st.Pool()

	// --- Neo4j connection (best-effort, 3s timeout) ---
	var g *graph.Writer
	{
		connCtx, connCancel := context.WithTimeout(ctx, 3*time.Second)
		g, err = graph.NewFromEnv(connCtx, 3*time.Second)
		connCancel()
		if err != nil {
			log.Printf("stats: neo4j unavailable: %v", err)
			// g stays nil — handled below
		}
	}
	if g != nil {
		defer g.Close(context.Background())
	}

	// --- Fan-out: collect all metrics concurrently ---
	var d dashboard.DashboardData
	var wg sync.WaitGroup

	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	// Postgres metrics
	run(func() {
		v, e := store.WalletCount(ctx, pool, now)
		d.TotalWallets = dashboard.Result[int64]{Val: v, Err: e}
	})
	run(func() {
		v, e := store.ActiveWallets24h(ctx, pool, now)
		d.ActiveWallets24h = dashboard.Result[int64]{Val: v, Err: e}
	})
	run(func() {
		v, e := store.Volume24h(ctx, pool, now)
		d.Volume24hSOL = dashboard.Result[float64]{Val: v, Err: e}
	})
	run(func() {
		v, e := store.NewTokens24h(ctx, pool, now)
		d.NewTokens24h = dashboard.Result[int64]{Val: v, Err: e}
	})
	run(func() {
		total, last24h, e := store.MigratedTokens(ctx, pool, now)
		d.MigratedTotal = dashboard.Result[int64]{Val: total, Err: e}
		d.MigratedLast24h = dashboard.Result[int64]{Val: last24h, Err: e}
	})
	run(func() {
		v, e := store.TradesPerMinute(ctx, pool, now)
		d.TradesPerMinute = dashboard.Result[int64]{Val: v, Err: e}
	})
	run(func() {
		buys, sells, e := store.BuySellRatio24h(ctx, pool, now)
		d.BuysLast24h = dashboard.Result[int64]{Val: buys, Err: e}
		d.SellsLast24h = dashboard.Result[int64]{Val: sells, Err: e}
	})
	run(func() {
		sym, cnt, e := store.HottestToken1h(ctx, pool, now)
		d.HottestTokenSymbol = dashboard.Result[string]{Val: sym, Err: e}
		d.HottestTokenTrades = dashboard.Result[int64]{Val: cnt, Err: e}
	})
	run(func() {
		total, complete, errs, e := store.CrawlerQueueStatus(ctx, pool, now)
		d.CrawlerTotal = dashboard.Result[int64]{Val: total, Err: e}
		d.CrawlerComplete = dashboard.Result[int64]{Val: complete, Err: e}
		d.CrawlerErrors = dashboard.Result[int64]{Val: errs, Err: e}
	})
	run(func() {
		v, e := store.LastTradeAt(ctx, pool, now)
		d.LastTradeAt = dashboard.Result[time.Time]{Val: v, Err: e}
	})
	run(func() {
		v, e := store.LastSuccessfulDuneCallAt(ctx, pool, now)
		d.LastDuneCallAt = dashboard.Result[time.Time]{Val: v, Err: e}
	})
	// PnL (uses RealizedPnL from queries.go)
	run(func() {
		rows, e := store.RealizedPnL(ctx, pool, "24 hours", 5, store.OrderDesc)
		entries := make([]dashboard.PnLRow, len(rows))
		for i, r := range rows {
			entries[i] = dashboard.PnLRow{Rank: i + 1, Wallet: r.Trader, PnLSOL: r.RealizedPnLSOL}
		}
		d.Top5Traders = dashboard.Result[[]dashboard.PnLRow]{Val: entries, Err: e}
	})
	run(func() {
		rows, e := store.RealizedPnL(ctx, pool, "24 hours", 5, store.OrderAsc)
		entries := make([]dashboard.PnLRow, len(rows))
		for i, r := range rows {
			entries[i] = dashboard.PnLRow{Rank: i + 1, Wallet: r.Trader, PnLSOL: r.RealizedPnLSOL}
		}
		d.Flop5Traders = dashboard.Result[[]dashboard.PnLRow]{Val: entries, Err: e}
	})

	// Neo4j metrics (only if connected)
	if g != nil {
		run(func() {
			v, e := g.DiscoveredWalletCount(ctx)
			d.DiscoveredWallets = dashboard.Result[int64]{Val: v, Err: e}
		})
		run(func() {
			v, e := g.EdgeCount(ctx)
			d.GraphEdges = dashboard.Result[int64]{Val: v, Err: e}
		})
		run(func() {
			rows, e := g.TopPumpWalletsByExternalTrades(ctx, 5)
			extRows := make([]dashboard.ExternalRow, len(rows))
			for i, r := range rows {
				extRows[i] = dashboard.ExternalRow{Rank: i + 1, Wallet: r.Wallet, Trades: r.Trades}
			}
			d.TopExternalWallets = dashboard.Result[[]dashboard.ExternalRow]{Val: extRows, Err: e}
		})
	} else {
		// Mark Neo4j fields with a sentinel error so renderer shows n/a
		sentinelErr := fmt.Errorf("neo4j unavailable")
		d.DiscoveredWallets = dashboard.Result[int64]{Err: sentinelErr}
		d.GraphEdges = dashboard.Result[int64]{Err: sentinelErr}
		d.TopExternalWallets = dashboard.Result[[]dashboard.ExternalRow]{Err: sentinelErr}
	}

	wg.Wait()

	neo4jOK := g != nil && d.DiscoveredWallets.OK()
	d.Neo4jOK = dashboard.Result[bool]{Val: neo4jOK}

	fmt.Print(dashboard.Render(d, now, width))
}
