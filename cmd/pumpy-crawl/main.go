package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"pumpy/internal/crawler"
	"pumpy/internal/dune"
	"pumpy/internal/graph"
	"pumpy/internal/solana"
	"pumpy/internal/store"
)

func main() {
	cfg, err := crawler.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer st.Pool().Close()
	if err := st.ApplySchema(ctx); err != nil {
		log.Fatalf("apply schema: %v", err)
	}

	seed := make([]struct{ ID, Label string }, len(solana.PumpProgramSeed))
	for i, s := range solana.PumpProgramSeed {
		seed[i] = struct{ ID, Label string }{s.ID, s.Label}
	}
	if err := crawler.SeedPrograms(ctx, st.Pool(), seed); err != nil {
		log.Fatalf("seed programs: %v", err)
	}

	lookup, err := crawler.NewPgLookup(ctx, st.Pool())
	if err != nil {
		log.Fatalf("pg lookup: %v", err)
	}

	duneClient := dune.NewClient(cfg.DuneBaseURL, cfg.DuneAPIKey, cfg.MaxRPS, cfg.MaxRetries)

	g, err := graph.New(ctx, cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPassword)
	if err != nil {
		log.Fatalf("neo4j: %v", err)
	}
	defer g.Close(context.Background())
	if err := g.EnsureConstraints(ctx); err != nil {
		log.Fatalf("neo4j constraints: %v", err)
	}

	runner := crawler.NewRunner(cfg, st.Pool(), duneClient, lookup, g)

	go func() {
		tick := time.NewTicker(5 * time.Minute)
		defer tick.Stop()
		syncQueue(ctx, st, "initial")
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				syncQueue(ctx, st, "tick")
			}
		}
	}()

	log.Printf("pumpy-crawl: started (rps=%.1f, page_limit=%d)", cfg.MaxRPS, cfg.PageLimit)
	for {
		if ctx.Err() != nil {
			return
		}
		lease, release, err := crawler.PickNextWallet(ctx, st.Pool(), cfg.IncrementalAge)
		if err != nil {
			log.Printf("pick: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if lease == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}

		pages, runErr := runner.CrawlOnce(ctx, lease)
		if runErr != nil {
			log.Printf("crawl %s: pages=%d err=%v", lease.Wallet, pages, runErr)
			_ = crawler.RecordError(ctx, st.Pool(), lease.Wallet, runErr.Error())
		} else {
			log.Printf("crawl %s: pages=%d ok", lease.Wallet, pages)
		}
		if err := release(ctx); err != nil {
			log.Printf("release lease %s: %v", lease.Wallet, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.LoopSleep):
		}
	}
}

func syncQueue(ctx context.Context, st *store.Store, label string) {
	n, err := crawler.SyncWalletQueue(ctx, st.Pool())
	if err != nil {
		log.Printf("queue sync (%s): %v", label, err)
		return
	}
	if n > 0 || label == "initial" {
		log.Printf("queue sync (%s): +%d wallets", label, n)
	}
}
