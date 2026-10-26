-- +goose Up

-- A DCA ladder on one pair: the user sets each rung (buy at, sell at, amount); the bot only
-- watches the price and fills MARKET orders.
CREATE TABLE dca_settings (
  id         INTEGER PRIMARY KEY,
  pair_id    INTEGER NOT NULL REFERENCES pairs(id),
  status     TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'INACTIVE', 'DONE')),
  note       TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_dca_settings_pair ON dca_settings(pair_id, status);

-- One rung. OPENING: waiting for price <= buy_price · OPEN: held, waiting for price >= sell_price
-- (0 = hold, no automatic sell) · CLOSED: sold.
CREATE TABLE dca_trades (
  id                INTEGER PRIMARY KEY,
  dca_setting_id    INTEGER NOT NULL REFERENCES dca_settings(id) ON DELETE CASCADE,
  status            TEXT NOT NULL DEFAULT 'OPENING' CHECK (status IN ('OPENING', 'OPEN', 'CLOSED')),
  buy_price         DECIMAL_TEXT NOT NULL,         -- trigger set by the user
  sell_price        DECIMAL_TEXT NOT NULL,         -- trigger; 0 = hold
  amount            DECIMAL_TEXT NOT NULL,         -- base asset to buy
  held_amount       DECIMAL_TEXT NOT NULL DEFAULT '0', -- base asset received (after a base-asset fee)
  open_price        DECIMAL_TEXT,                  -- average fill of the buy
  close_price       DECIMAL_TEXT,                  -- average fill of the sell
  buy_quote         DECIMAL_TEXT NOT NULL DEFAULT '0', -- USDT spent
  sell_quote        DECIMAL_TEXT NOT NULL DEFAULT '0', -- USDT received (before the sell fee)
  fee_usdt          DECIMAL_TEXT NOT NULL DEFAULT '0', -- every fee, valued in USDT (display)
  extra_fee_usdt    DECIMAL_TEXT NOT NULL DEFAULT '0', -- fees not taken from the coin (USDT, BNB): charged to PnL
  realized_profit   DECIMAL_TEXT NOT NULL DEFAULT '0',
  unrealized_profit DECIMAL_TEXT NOT NULL DEFAULT '0',
  open_order_id     TEXT NOT NULL UNIQUE,
  close_order_id    TEXT NOT NULL UNIQUE,
  last_error        TEXT NOT NULL DEFAULT '',
  legacy_trade_id   INTEGER,
  open_at           TIMESTAMP_TEXT,
  close_at          TIMESTAMP_TEXT,
  created_at        TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at        TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_dca_trades_setting ON dca_trades(dca_setting_id, status);
CREATE INDEX idx_dca_trades_status ON dca_trades(status);

-- +goose Down
DROP TABLE dca_trades;
DROP TABLE dca_settings;
