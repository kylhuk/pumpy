CREATE TABLE IF NOT EXISTS tokens (
    mint          TEXT        PRIMARY KEY,
    creator       TEXT        NOT NULL,
    name          TEXT,
    symbol        TEXT,
    uri           TEXT,
    created_at    TIMESTAMPTZ NOT NULL,
    migrated_at   TIMESTAMPTZ,
    last_trade_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS tokens_active
    ON tokens (last_trade_at)
    WHERE migrated_at IS NULL;

-- Partitioned by day. Partitions are named trades_YYYYMMDD and managed by the pruner.
CREATE TABLE IF NOT EXISTS trades (
    signature         TEXT          NOT NULL,
    mint              TEXT          NOT NULL,
    trader            TEXT          NOT NULL,
    side              SMALLINT      NOT NULL,  -- 0=buy 1=sell 2=create
    sol_lamports      BIGINT        NOT NULL,
    token_amount      NUMERIC(40,0) NOT NULL,
    new_token_balance NUMERIC(40,0) NOT NULL,
    market_cap_sol    DOUBLE PRECISION,
    captured_at       TIMESTAMPTZ   NOT NULL,
    PRIMARY KEY (captured_at, signature)
) PARTITION BY RANGE (captured_at);

CREATE INDEX IF NOT EXISTS trades_trader_time ON trades (trader, captured_at DESC);
CREATE INDEX IF NOT EXISTS trades_mint_time   ON trades (mint, captured_at DESC);

-- Schema version sentinel used by schema.go to skip re-running.
CREATE TABLE IF NOT EXISTS schema_version (version INT PRIMARY KEY);
INSERT INTO schema_version VALUES (1) ON CONFLICT DO NOTHING;
