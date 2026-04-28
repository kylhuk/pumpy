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

func runWhois(wallet string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	now := time.Now().UTC()

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 80
	}

	dsn := envOrDefault("DATABASE_URL", "postgres://pumpy:pumpy@localhost:5432/pumpy?sslmode=disable")
	st, err := store.New(ctx, dsn)
	if err != nil {
		log.Fatalf("whois: postgres connect: %v", err)
	}
	defer st.Pool().Close()
	pool := st.Pool()

	var g *graph.Writer
	{
		connCtx, connCancel := context.WithTimeout(ctx, 3*time.Second)
		g, err = graph.NewFromEnv(connCtx, 3*time.Second)
		connCancel()
		if err != nil {
			log.Printf("whois: neo4j unavailable: %v", err)
		}
	}
	if g != nil {
		defer g.Close(context.Background())
	}

	var d dashboard.WhoisData
	d.Wallet = wallet
	d.Now = now

	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	// Helper to fill one WindowedStats from Postgres.
	fillWindowStats := func(ws *dashboard.WindowedStats, window string) {
		run(func() {
			v, e := store.WalletRealizedPnLSOL(ctx, pool, wallet, window)
			ws.PnLSOL = dashboard.Result[float64]{Val: v, Err: e}
		})
		run(func() {
			v, e := store.WalletTradeCount(ctx, pool, wallet, window)
			ws.TradeCount = dashboard.Result[int64]{Val: v, Err: e}
		})
		run(func() {
			v, e := store.WalletDistinctPumpTokens(ctx, pool, wallet, window)
			ws.DistinctTokens = dashboard.Result[int64]{Val: v, Err: e}
		})
		run(func() {
			v, e := store.WalletTopTokensByVolume(ctx, pool, wallet, window, 5)
			ws.TopTokens = dashboard.Result[[]store.WalletTopToken]{Val: v, Err: e}
		})
	}

	fillWindowStats(&d.Window24h, "24 hours")
	fillWindowStats(&d.Window7d, "7 days")
	fillWindowStats(&d.Window14d, "14 days")

	sentinelErr := fmt.Errorf("neo4j unavailable")
	if g != nil {
		// Top 5 counterparties per window
		windows := []struct {
			dur time.Duration
			ws  *dashboard.WindowedStats
		}{
			{24 * time.Hour, &d.Window24h},
			{7 * 24 * time.Hour, &d.Window7d},
			{14 * 24 * time.Hour, &d.Window14d},
		}
		for _, w := range windows {
			since := now.Add(-w.dur)
			ws := w.ws
			run(func() {
				v, e := g.WalletOutboundCounterparties(ctx, wallet, since, 5)
				ws.TopCounterparties = dashboard.Result[[]graph.Counterparty]{Val: v, Err: e}
			})
		}
		// All-time counterparties (no limit)
		run(func() {
			v, e := g.WalletOutboundCounterparties(ctx, wallet, time.Time{}, 0)
			d.AllCounterparties = dashboard.Result[[]graph.Counterparty]{Val: v, Err: e}
		})
	} else {
		d.Window24h.TopCounterparties = dashboard.Result[[]graph.Counterparty]{Err: sentinelErr}
		d.Window7d.TopCounterparties = dashboard.Result[[]graph.Counterparty]{Err: sentinelErr}
		d.Window14d.TopCounterparties = dashboard.Result[[]graph.Counterparty]{Err: sentinelErr}
		d.AllCounterparties = dashboard.Result[[]graph.Counterparty]{Err: sentinelErr}
	}

	wg.Wait()

	d.Neo4jOK = g != nil

	fmt.Print(dashboard.RenderWhois(d, now, width))
}
