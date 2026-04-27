-- Pump.fun program IDs to exclude. Bootstrapped from a constant; can be
-- amended at runtime by INSERTing additional rows.
CREATE TABLE IF NOT EXISTS pump_program_id (
  program_id TEXT PRIMARY KEY,
  label      TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-wallet crawl progress. Acts as both the queue and the durable state.
CREATE TABLE IF NOT EXISTS wallet_crawl_state (
  wallet                 TEXT PRIMARY KEY,
  backfill_complete      BOOLEAN NOT NULL DEFAULT FALSE,
  next_offset            TEXT,
  newest_seen_signature  TEXT,
  newest_seen_slot       BIGINT,
  newest_seen_block_time BIGINT,
  last_started_at        TIMESTAMPTZ,
  last_completed_at      TIMESTAMPTZ,
  pages_fetched          INTEGER NOT NULL DEFAULT 0,
  tx_seen_count          INTEGER NOT NULL DEFAULT 0,
  error_count            INTEGER NOT NULL DEFAULT 0,
  last_error             TEXT
);

-- Index supports the picker query: backfill-first, then stalest incremental.
CREATE INDEX IF NOT EXISTS wallet_crawl_state_due
  ON wallet_crawl_state (backfill_complete, last_completed_at NULLS FIRST);

-- Idempotency: which signatures we've already processed for which wallet.
-- Drives incremental-mode early-stop and pagination resume safety.
CREATE TABLE IF NOT EXISTS wallet_tx_seen (
  wallet     TEXT      NOT NULL,
  signature  TEXT      NOT NULL,
  block_slot BIGINT,
  block_time BIGINT,
  seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (wallet, signature)
);

-- Minimal per-tx metadata. Edges live in Neo4j; this table is for diagnostics
-- and pump-exclusion auditing only. Raw JSON is never persisted by default.
CREATE TABLE IF NOT EXISTS transaction_min (
  signature        TEXT PRIMARY KEY,
  block_slot       BIGINT,
  block_time       BIGINT,
  fee_lamports     BIGINT,
  err              TEXT,
  account_keys     JSONB,
  program_ids      JSONB,
  is_pump_excluded BOOLEAN NOT NULL DEFAULT FALSE,
  exclusion_reason TEXT,
  inserted_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every Dune request, for budget tracking and debugging.
CREATE TABLE IF NOT EXISTS api_request_log (
  id                BIGSERIAL PRIMARY KEY,
  wallet            TEXT,
  endpoint          TEXT NOT NULL,
  status_code       INTEGER,
  cu_cost           INTEGER NOT NULL DEFAULT 1,
  response_tx_count INTEGER,
  had_next_offset   BOOLEAN,
  started_at        TIMESTAMPTZ NOT NULL,
  completed_at      TIMESTAMPTZ,
  error             TEXT
);

CREATE INDEX IF NOT EXISTS api_request_log_started
  ON api_request_log (started_at DESC);

INSERT INTO schema_version (version) VALUES (2) ON CONFLICT DO NOTHING;
