-- +goose Up

-- EMA Settings: a budgeted EMA14 / Smooth10 cross campaign on one pair (Rails ema_settings).
CREATE TABLE ema_settings (
  id              INTEGER PRIMARY KEY,
  pair_id         INTEGER NOT NULL REFERENCES pairs(id),
  budget          DECIMAL_TEXT NOT NULL,                -- USDT per trade
  trade_type      TEXT NOT NULL DEFAULT 'BOTH' CHECK (trade_type IN ('ONLY_LONG', 'ONLY_SHORT', 'BOTH')),
  period          TEXT NOT NULL DEFAULT 'D1' CHECK (period IN ('D1', 'H4')),
  check_type      TEXT NOT NULL DEFAULT 'CHECK_AT_OPEN' CHECK (check_type IN ('CHECK_AT_OPEN', 'CHECK_AT_CLOSE', 'CHECK_AT_CURRENT')),
  next_trade_type TEXT NOT NULL DEFAULT 'NEW_TRADE' CHECK (next_trade_type IN ('NEW_TRADE', 'NOT_NEW_TRADE')),
  tp_percent      DECIMAL_TEXT NOT NULL DEFAULT '0',     -- of the budget, on the total PnL; 0 = off
  sl_percent      DECIMAL_TEXT NOT NULL DEFAULT '0',
  status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'INACTIVE', 'DONE')),
  note            TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at      TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_ema_settings_status ON ema_settings(status);
CREATE INDEX idx_ema_settings_pair ON ema_settings(pair_id);

-- Trades of a setting. Rails kept them in ema_trades and used open_price / close_price both as the
-- trigger (before the order) and as the fill (after): here the triggers have their own columns.
CREATE TABLE ema_setting_trades (
  id                 INTEGER PRIMARY KEY,              -- the Rails ema_trades id for imported trades
  setting_id         INTEGER NOT NULL REFERENCES ema_settings(id) ON DELETE CASCADE,
  pair_id            INTEGER NOT NULL REFERENCES pairs(id),
  side               TEXT NOT NULL CHECK (side IN ('LONG', 'SHORT')),
  status             TEXT NOT NULL DEFAULT 'OPENING' CHECK (status IN ('OPENING', 'OPEN', 'CLOSED')),
  is_auto            BOOLEAN NOT NULL DEFAULT 0,       -- created by the scan, not by hand
  amount             DECIMAL_TEXT NOT NULL,            -- base asset held (long: net of the buy fee) or owed (short)
  entry_price        DECIMAL_TEXT,                     -- OPENING: open when the price passes it; NULL = on the cross
  target_price       DECIMAL_TEXT,                     -- take profit of this trade; NULL = none
  sl_price           DECIMAL_TEXT,                     -- stop loss; set from the stop-loss % at open when NULL
  open_price         DECIMAL_TEXT,                     -- average fill of the open order
  close_price        DECIMAL_TEXT,                     -- average fill of the close order
  fee_usdt           DECIMAL_TEXT NOT NULL DEFAULT '0',
  interest_paid      DECIMAL_TEXT NOT NULL DEFAULT '0', -- coin interest repaid at close (short)
  interest_usdt      DECIMAL_TEXT NOT NULL DEFAULT '0',
  realized_profit    DECIMAL_TEXT NOT NULL DEFAULT '0', -- net of fees and interest (imported trades: Rails gross)
  unrealized_profit  DECIMAL_TEXT NOT NULL DEFAULT '0', -- net of the open fee
  close_reason       TEXT NOT NULL DEFAULT '',          -- SL, TARGET, EMA_REVERSE, SETTING_TP, SETTING_SL, MANUAL
  open_order_id      TEXT NOT NULL UNIQUE,
  close_order_id     TEXT NOT NULL UNIQUE,
  borrow_step        TEXT NOT NULL DEFAULT '',          -- short open: '' → BORROWING → BORROWED (one borrow across retries)
  first_attempted_at TIMESTAMP_TEXT,
  last_error         TEXT NOT NULL DEFAULT '',
  legacy             BOOLEAN NOT NULL DEFAULT 0,        -- imported from Rails (gross PnL, ema_<id>_* order ids)
  open_at            TIMESTAMP_TEXT,
  close_at           TIMESTAMP_TEXT,
  created_at         TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at         TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_ema_setting_trades_setting ON ema_setting_trades(setting_id, status);
CREATE INDEX idx_ema_setting_trades_pair ON ema_setting_trades(pair_id, side, status);

-- +goose Down
DROP TABLE ema_setting_trades;
DROP TABLE ema_settings;
