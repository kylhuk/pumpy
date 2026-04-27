package pruner

import (
	"context"
	"log"
	"time"

	"pumpy/internal/portal"
	"pumpy/internal/store"
)

// Pruner runs hourly to:
//  1. Drop mints from the active set that had no trades in the last 24 h.
//  2. Pre-create daily partitions for the next 7 days.
//  3. Drop partitions older than retainDays.
type Pruner struct {
	st         *store.Store
	client     *portal.Client
	retainDays int
}

func New(st *store.Store, client *portal.Client, retainDays int) *Pruner {
	return &Pruner{st: st, client: client, retainDays: retainDays}
}

// Run blocks, ticking every hour. Call from a goroutine.
func (p *Pruner) Run(ctx context.Context) {
	p.tick(ctx) // run once immediately on start
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.tick(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pruner) tick(ctx context.Context) {
	if err := store.EnsurePartitions(ctx, p.st.Pool()); err != nil {
		log.Printf("pruner: ensure partitions: %v", err)
	}

	if err := store.DropOldPartitions(ctx, p.st.Pool(), p.retainDays); err != nil {
		log.Printf("pruner: drop old partitions: %v", err)
	}

	p.pruneInactiveMints(ctx)
}

func (p *Pruner) pruneInactiveMints(ctx context.Context) {
	// Ask the DB which mints are still active (unmigrated + traded in last 24 h).
	active, err := p.st.ActiveMints(ctx)
	if err != nil {
		log.Printf("pruner: active mints query: %v", err)
		return
	}
	activeSet := make(map[string]struct{}, len(active))
	for _, m := range active {
		activeSet[m] = struct{}{}
	}

	// Find mints the WS client is still subscribed to that have gone inactive.
	current := p.client.ActiveMints()
	var stale []string
	for _, m := range current {
		if _, ok := activeSet[m]; !ok {
			stale = append(stale, m)
		}
	}

	if len(stale) > 0 {
		p.client.UnsubscribeToken(stale)
		log.Printf("pruner: unsubscribed %d stale mints", len(stale))
	}
}
