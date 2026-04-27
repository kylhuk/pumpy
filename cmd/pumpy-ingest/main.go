package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"pumpy/internal/portal"
	"pumpy/internal/pruner"
	"pumpy/internal/store"
)

func main() {
	connStr := env("DATABASE_URL", "postgres://pumpy:pumpy@localhost:5432/pumpy?sslmode=disable")
	retainDays := envInt("RETAIN_DAYS", 60)

	ctx := context.Background()

	st, err := store.New(ctx, connStr)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	if err := st.ApplySchema(ctx); err != nil {
		log.Fatalf("schema: %v", err)
	}

	// Hydrate active mints from DB before connecting WS.
	activeMints, err := st.ActiveMints(ctx)
	if err != nil {
		log.Fatalf("active mints: %v", err)
	}
	log.Printf("ingester: loaded %d active mints from DB", len(activeMints))

	h := &handler{st: st}
	client := portal.NewClient(h)
	h.client = client
	client.SetActiveMints(activeMints)

	p := pruner.New(st, client, retainDays)
	go p.Run(ctx)

	// Run blocks and reconnects forever.
	client.Run()
}

type handler struct {
	st     *store.Store
	client *portal.Client
}

func (h *handler) OnNewToken(t portal.NewToken) {
	ctx := context.Background()
	now := time.Now().UTC()

	if err := h.st.UpsertToken(ctx, store.TokenRow{
		Mint:      t.Mint,
		Creator:   t.TraderPublicKey,
		Name:      t.Name,
		Symbol:    t.Symbol,
		URI:       t.URI,
		CreatedAt: now,
	}); err != nil {
		log.Printf("upsert token %s: %v", t.Mint, err)
	}

	h.st.EnqueueTrade(store.TradeRow{
		Signature:       t.Signature,
		Mint:            t.Mint,
		Trader:          t.TraderPublicKey,
		Side:            2,
		SolLamports:     store.SolToLamports(t.SolAmount),
		TokenAmount:     store.FormatTokenAmount(t.TokenAmount),
		NewTokenBalance: store.FormatTokenAmount(t.NewTokenBalance),
		MarketCapSol:    &t.MarketCapSol,
		CapturedAt:      now,
	})

	h.client.SubscribeToken(t.Mint)
}

func (h *handler) OnTrade(t portal.Trade) {
	now := time.Now().UTC()

	side := int16(0)
	if t.TxType == "sell" {
		side = 1
	}

	mc := t.MarketCapSol
	h.st.EnqueueTrade(store.TradeRow{
		Signature:       t.Signature,
		Mint:            t.Mint,
		Trader:          t.TraderPublicKey,
		Side:            side,
		SolLamports:     store.SolToLamports(t.SolAmount),
		TokenAmount:     store.FormatTokenAmount(t.TokenAmount),
		NewTokenBalance: store.FormatTokenAmount(t.NewTokenBalance),
		MarketCapSol:    &mc,
		CapturedAt:      now,
	})

	go func() {
		ctx := context.Background()
		if err := h.st.UpdateLastTrade(ctx, t.Mint, now); err != nil {
			log.Printf("update last_trade %s: %v", t.Mint, err)
		}
	}()
}

func (h *handler) OnMigration(m portal.Migration) {
	ctx := context.Background()
	if err := h.st.MarkMigrated(ctx, m.Mint); err != nil {
		log.Printf("mark migrated %s: %v", m.Mint, err)
	}
	h.client.UnsubscribeToken([]string{m.Mint})
	log.Printf("ingester: migration — unsubscribed %s", m.Mint)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
