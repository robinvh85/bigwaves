-- +goose Up
-- Type names: see internal/platform/dbtype. DECIMAL_TEXT and TIMESTAMP_TEXT keep TEXT affinity.

CREATE TABLE markets (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
INSERT INTO markets (id, name) VALUES (1, 'BINANCE');

CREATE TABLE coins (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at  TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE pairs (
  id               INTEGER PRIMARY KEY,
  coin_id          INTEGER NOT NULL REFERENCES coins(id),
  market_id        INTEGER NOT NULL REFERENCES markets(id),
  name             TEXT NOT NULL UNIQUE,                    -- Binance symbol, e.g. SUNUSDT
  quote_asset      TEXT NOT NULL,                           -- USDT
  status           TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DISABLED')),
  current_price    DECIMAL_TEXT,
  price_updated_at TIMESTAMP_TEXT,
  interest_rate    DECIMAL_TEXT,                            -- yearly borrow rate in %, from dailyInterestRate * 365 * 100
  step_size        DECIMAL_TEXT,                            -- LOT_SIZE stepSize
  min_notional     DECIMAL_TEXT,                            -- NOTIONAL / MIN_NOTIONAL filter
  is_spot          BOOLEAN NOT NULL DEFAULT 0,              -- trade on spot instead of margin
  is_isolated      BOOLEAN NOT NULL DEFAULT 0,              -- isolated margin instead of cross
  deleted_at       TIMESTAMP_TEXT,
  created_at       TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at       TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_pairs_coin ON pairs(coin_id);
CREATE INDEX idx_pairs_status ON pairs(status) WHERE deleted_at IS NULL;

-- One row per strategy ("ema1d", "dca"); the JSON shape is owned by that strategy's Config type.
CREATE TABLE strategy_configs (
  strategy   TEXT PRIMARY KEY,
  config     JSON_TEXT NOT NULL CHECK (json_valid(config)),
  version    INTEGER NOT NULL DEFAULT 1,
  updated_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE config_changes (
  id         INTEGER PRIMARY KEY,
  strategy   TEXT NOT NULL,
  before     JSON_TEXT NOT NULL CHECK (json_valid(before)),
  after      JSON_TEXT NOT NULL CHECK (json_valid(after)),
  user_email TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_config_changes_strategy ON config_changes(strategy, id);

CREATE TABLE job_runs (
  id          INTEGER PRIMARY KEY,
  job         TEXT NOT NULL,
  trigger     TEXT NOT NULL DEFAULT 'schedule' CHECK (trigger IN ('schedule', 'manual')),
  status      TEXT NOT NULL CHECK (status IN ('RUNNING', 'OK', 'FAILED', 'SKIPPED')),
  message     TEXT NOT NULL DEFAULT '',
  started_at  TIMESTAMP_TEXT NOT NULL,
  finished_at TIMESTAMP_TEXT
);
CREATE INDEX idx_job_runs_job ON job_runs(job, id);
CREATE INDEX idx_job_runs_started ON job_runs(started_at);

-- Throttles repeated alerts: one row per fingerprint (job/operation + error kind + normalized message).
CREATE TABLE error_events (
  fingerprint   TEXT PRIMARY KEY,
  source        TEXT NOT NULL,
  message       TEXT NOT NULL,
  count         INTEGER NOT NULL DEFAULT 1,
  first_seen_at TIMESTAMP_TEXT NOT NULL,
  last_seen_at  TIMESTAMP_TEXT NOT NULL,
  notified_at   TIMESTAMP_TEXT
);

-- +goose Down
DROP TABLE error_events;
DROP TABLE job_runs;
DROP TABLE config_changes;
DROP TABLE strategy_configs;
DROP TABLE pairs;
DROP TABLE coins;
DROP TABLE markets;
