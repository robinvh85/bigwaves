-- +goose Up

-- Every order sent to an exchange, by any strategy: the audit trail and the source for
-- reconciling with Binance (and for rolling back a cutover).
CREATE TABLE exchange_orders (
  id                INTEGER PRIMARY KEY,
  client_order_id   TEXT NOT NULL UNIQUE,
  strategy          TEXT NOT NULL,                 -- ema1d, dca
  ref_id            INTEGER,                       -- position / trade id in that strategy
  symbol            TEXT NOT NULL,
  account           TEXT NOT NULL CHECK (account IN ('SPOT', 'CROSS', 'ISOLATED')),
  side              TEXT NOT NULL CHECK (side IN ('BUY', 'SELL')),
  exchange_order_id INTEGER,
  status            TEXT NOT NULL,
  executed_qty      DECIMAL_TEXT NOT NULL,
  quote_qty         DECIMAL_TEXT NOT NULL,
  avg_price         DECIMAL_TEXT NOT NULL,
  net_qty           DECIMAL_TEXT NOT NULL,         -- base asset received (buy) after base-asset fees
  fee_usdt          DECIMAL_TEXT NOT NULL,
  raw               JSON_TEXT NOT NULL CHECK (json_valid(raw)),
  executed_at       TIMESTAMP_TEXT NOT NULL,
  created_at        TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_exchange_orders_ref ON exchange_orders(strategy, ref_id);

-- Small key/value state that Rails kept in Redis: last margin level reading, alert throttles.
CREATE TABLE app_state (
  key        TEXT PRIMARY KEY,
  value      JSON_TEXT NOT NULL CHECK (json_valid(value)),
  expires_at TIMESTAMP_TEXT,
  updated_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE ema1d_pairs (
  id              INTEGER PRIMARY KEY,
  pair_id         INTEGER NOT NULL UNIQUE REFERENCES pairs(id),
  pair_group      TEXT NOT NULL DEFAULT 'CORE' CHECK (pair_group IN ('CORE', 'SENSITIVE')),
  allow_long      BOOLEAN NOT NULL DEFAULT 1,
  allow_short     BOOLEAN NOT NULL DEFAULT 1,
  short_supported BOOLEAN NOT NULL DEFAULT 0,     -- set by the pair check against Binance
  status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'EXIT_ONLY', 'DISABLED')),
  added_on        TEXT NOT NULL,                  -- YYYY-MM-DD: entries dated before it are ignored
  removed_on      TEXT,
  long_exit_rule  TEXT CHECK (long_exit_rule IN ('DIV4H', 'XO')),
  signal_from     TEXT,                           -- YYYY-MM-DD: strategy entries before it are ignored by the signal
  checked_at      TIMESTAMP_TEXT,
  check_note      TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at      TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- A real EMA1D position (Rails: ema_trades with source = EMA1D_EXIT).
--   OPENING: created, order not confirmed yet · OPEN: held · CLOSED: sold/bought back
CREATE TABLE ema1d_positions (
  id                INTEGER PRIMARY KEY,
  ema1d_pair_id     INTEGER NOT NULL REFERENCES ema1d_pairs(id),
  pair_id           INTEGER NOT NULL REFERENCES pairs(id),
  side              TEXT NOT NULL CHECK (side IN ('LONG', 'SHORT')),
  status            TEXT NOT NULL CHECK (status IN ('OPENING', 'OPEN', 'CLOSED')),
  signal_date       TEXT NOT NULL,                -- entry candle date of the strategy
  exit_rule         TEXT CHECK (exit_rule IN ('DIV4H', 'XO')),
  amount            DECIMAL_TEXT NOT NULL,        -- coin held (long: net of the buy fee) or owed (short)
  open_price        DECIMAL_TEXT,                 -- average fill price
  close_price       DECIMAL_TEXT,
  ref_open_price    DECIMAL_TEXT,                 -- strategy price: open of the entry candle
  ref_close_price   DECIMAL_TEXT,
  size_percent      DECIMAL_TEXT NOT NULL,        -- % of equity actually used
  equity_at_open    DECIMAL_TEXT NOT NULL,
  notional_usdt     DECIMAL_TEXT NOT NULL,
  fee_usdt          DECIMAL_TEXT NOT NULL DEFAULT '0',
  interest_paid     DECIMAL_TEXT NOT NULL DEFAULT '0', -- coin
  interest_usdt     DECIMAL_TEXT NOT NULL DEFAULT '0',
  realized_profit   DECIMAL_TEXT NOT NULL DEFAULT '0',
  unrealized_profit DECIMAL_TEXT NOT NULL DEFAULT '0',
  open_order_id     TEXT NOT NULL UNIQUE,         -- clientOrderId of the opening order
  close_order_id    TEXT NOT NULL UNIQUE,
  legacy_trade_id   INTEGER,                      -- ema_trades.id when imported from Rails
  open_at           TIMESTAMP_TEXT,
  close_at          TIMESTAMP_TEXT,
  created_at        TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at        TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_ema1d_positions_pair ON ema1d_positions(ema1d_pair_id, status);
CREATE INDEX idx_ema1d_positions_status ON ema1d_positions(status);

CREATE TABLE ema1d_syncs (
  id          INTEGER PRIMARY KEY,
  run_id      TEXT NOT NULL,
  signal_date TEXT NOT NULL,
  raw_query   TEXT NOT NULL DEFAULT '',
  parsed      JSON_TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(parsed)),
  result      JSON_TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(result)),
  dry_run     BOOLEAN NOT NULL DEFAULT 0,
  created_at  TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
-- A dry run is usually followed by the real call with the same run_id: only real runs are unique.
CREATE UNIQUE INDEX idx_ema1d_syncs_run ON ema1d_syncs(run_id, signal_date) WHERE dry_run = 0;
CREATE INDEX idx_ema1d_syncs_date ON ema1d_syncs(signal_date);

CREATE TABLE ema1d_todos (
  id                 INTEGER PRIMARY KEY,
  ema1d_pair_id      INTEGER NOT NULL REFERENCES ema1d_pairs(id),
  action             TEXT NOT NULL CHECK (action IN ('OPEN', 'CLOSE')),
  side               TEXT NOT NULL CHECK (side IN ('LONG', 'SHORT')),
  signal_date        TEXT NOT NULL,               -- entry candle date of the position (OPEN and CLOSE alike)
  ref_date           TEXT,                        -- candle date of the sync that asked for it
  ref_price          DECIMAL_TEXT,                -- OPEN: entry-day open, CLOSE: exit-day open
  exit_rule          TEXT CHECK (exit_rule IN ('DIV4H', 'XO')),
  source             TEXT NOT NULL DEFAULT 'SYNC' CHECK (source IN ('SYNC', 'MANUAL')),
  ema1d_sync_id      INTEGER REFERENCES ema1d_syncs(id),
  depends_on_id      INTEGER REFERENCES ema1d_todos(id),
  position_id        INTEGER REFERENCES ema1d_positions(id),
  status             TEXT NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'RUNNING', 'DONE', 'FAILED', 'SKIPPED', 'CANCELLED')),
  attempts           INTEGER NOT NULL DEFAULT 0,
  first_attempted_at TIMESTAMP_TEXT,              -- start of the retry window; borrow history is searched from here
  run_after          TIMESTAMP_TEXT,
  last_error         TEXT NOT NULL DEFAULT '',
  step               TEXT CHECK (step IN ('BORROWING', 'BORROWED', 'BORROW_REVERTED')),
  created_at         TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at         TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE UNIQUE INDEX idx_ema1d_todos_unique ON ema1d_todos(ema1d_pair_id, action, side, signal_date) WHERE status <> 'CANCELLED';
CREATE INDEX idx_ema1d_todos_status ON ema1d_todos(status, run_after);
CREATE INDEX idx_ema1d_todos_position ON ema1d_todos(position_id);
CREATE INDEX idx_ema1d_todos_depends ON ema1d_todos(depends_on_id);

CREATE TABLE ema1d_equity_snapshots (
  date                TEXT PRIMARY KEY,
  equity_usdt         DECIMAL_TEXT NOT NULL,
  margin_level        DECIMAL_TEXT,
  open_long_notional  DECIMAL_TEXT NOT NULL,
  open_short_notional DECIMAL_TEXT NOT NULL,
  created_at          TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- 1D and 4H candles, kept so the daily check only fetches the last few: the EMA has no warm-up
-- cut, its value depends on the whole history. REAL like the Python research.
CREATE TABLE ema1d_candles (
  pair_id   INTEGER NOT NULL REFERENCES pairs(id),
  interval  TEXT NOT NULL CHECK (interval IN ('1D', '4H')),
  open_time INTEGER NOT NULL,                     -- unix seconds, UTC
  open      REAL NOT NULL,
  high      REAL NOT NULL,
  low       REAL NOT NULL,
  close     REAL NOT NULL,
  PRIMARY KEY (pair_id, interval, open_time)
) WITHOUT ROWID;

CREATE TABLE ema1d_daily_checks (
  signal_date TEXT PRIMARY KEY,
  status      TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SYNCED', 'SENT', 'FAILED')),
  run_id      TEXT NOT NULL DEFAULT '',
  attempts    INTEGER NOT NULL DEFAULT 0,
  rows        JSON_TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(rows)),
  sync_params JSON_TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(sync_params)),
  sync_result JSON_TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(sync_result)),
  report      TEXT NOT NULL DEFAULT '',
  last_error  TEXT NOT NULL DEFAULT '',
  synced_at   TIMESTAMP_TEXT,
  sent_at     TIMESTAMP_TEXT,
  created_at  TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at  TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- +goose Down
DROP TABLE ema1d_daily_checks;
DROP TABLE ema1d_candles;
DROP TABLE ema1d_equity_snapshots;
DROP TABLE ema1d_todos;
DROP TABLE ema1d_syncs;
DROP TABLE ema1d_positions;
DROP TABLE ema1d_pairs;
DROP TABLE app_state;
DROP TABLE exchange_orders;
