package crawler

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DuneAPIKey       string
	DuneBaseURL      string
	PageLimit        int
	MaxRPS           float64
	MaxRetries       int
	MaxPagesPerRun   int
	MinTransferLamps int64
	StopAfterSeen    int
	IncrementalAge   time.Duration
	Neo4jURI         string
	Neo4jUser        string
	Neo4jPassword    string
	DatabaseURL      string
	LoopSleep        time.Duration
}

func LoadConfig() (Config, error) {
	c := Config{
		DuneAPIKey:       os.Getenv("DUNE_SIM_API_KEY"),
		DuneBaseURL:      env("DUNE_SIM_BASE_URL", "https://api.sim.dune.com"),
		PageLimit:        envInt("CRAWL_PAGE_LIMIT", 1000),
		MaxRPS:           envFloat("CRAWL_MAX_RPS", 4.0),
		MaxRetries:       envInt("CRAWL_MAX_RETRIES", 5),
		MaxPagesPerRun:   envInt("CRAWL_MAX_PAGES_PER_RUN", 50),
		MinTransferLamps: int64(envInt("CRAWL_MIN_TRANSFER_LAMPORTS", 1_000_000)),
		StopAfterSeen:    envInt("CRAWL_STOP_AFTER_SEEN", 20),
		IncrementalAge:   time.Duration(envInt("CRAWL_INCREMENTAL_AGE_HOURS", 6)) * time.Hour,
		Neo4jURI:         env("NEO4J_URI", "bolt://neo4j:7687"),
		Neo4jUser:        env("NEO4J_USER", "neo4j"),
		Neo4jPassword:    env("NEO4J_PASSWORD", "pumpypumpy"),
		DatabaseURL:      env("DATABASE_URL", "postgres://pumpy:pumpy@postgres:5432/pumpy?sslmode=disable"),
		LoopSleep:        time.Duration(envInt("CRAWL_LOOP_SLEEP_MS", 250)) * time.Millisecond,
	}
	if c.DuneAPIKey == "" {
		return c, fmt.Errorf("DUNE_SIM_API_KEY is required")
	}
	if c.PageLimit < 1 || c.PageLimit > 1000 {
		return c, fmt.Errorf("CRAWL_PAGE_LIMIT must be 1..1000, got %d", c.PageLimit)
	}
	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(k string, def float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
