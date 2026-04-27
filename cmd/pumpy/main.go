package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"pumpy/internal/store"
)

func main() {
	top := flag.NewFlagSet("top", flag.ExitOnError)
	window := top.String("window", "24h", "time window: 24h, 7d, or 14d")
	limit := top.Int("limit", 100, "number of wallets to show")

	if len(os.Args) < 2 {
		fmt.Println("usage: pumpy top [--window 24h|7d|14d] [--limit N]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "top":
		_ = top.Parse(os.Args[2:])
		runTop(*window, *limit)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runTop(window string, limit int) {
	pg := interval(window)
	if pg == "" {
		log.Fatalf("window must be 24h, 7d, or 14d — got %q", window)
	}

	connStr := envOrDefault("DATABASE_URL", "postgres://pumpy:pumpy@localhost:5432/pumpy?sslmode=disable")
	ctx := context.Background()

	st, err := store.New(ctx, connStr)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}

	earners, err := store.TopEarners(ctx, st.Pool(), pg, limit)
	if err != nil {
		log.Fatalf("query: %v", err)
	}

	if len(earners) == 0 {
		fmt.Printf("No closed positions found in the last %s.\n", window)
		fmt.Println("(The 14-day window needs ~14 days of data to warm up.)")
		return
	}

	fmt.Printf("%-4s  %-44s  %12s\n", "Rank", "Wallet", "Realized SOL")
	fmt.Printf("%-4s  %-44s  %12s\n", "----", "--------------------------------------------", "------------")
	for i, e := range earners {
		fmt.Printf("%-4d  %-44s  %+12.4f\n", i+1, e.Trader, e.RealizedPnLSOL)
	}
}

// interval maps the user-facing window flag to a Postgres interval string.
func interval(w string) string {
	switch w {
	case "24h":
		return "24 hours"
	case "7d":
		return "7 days"
	case "14d":
		return "14 days"
	default:
		return ""
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
