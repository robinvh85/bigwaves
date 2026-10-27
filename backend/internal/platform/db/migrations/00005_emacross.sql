-- +goose Up

-- Pairs watched by the EMA cross alert, with a switch per timeframe (replaces the Rails flags
-- pairs.is_auto_ema_{h1,h4,d1}_trade).
CREATE TABLE emacross_pairs (
  pair_id    INTEGER PRIMARY KEY REFERENCES pairs(id),
  h1         BOOLEAN NOT NULL DEFAULT 0,
  h4         BOOLEAN NOT NULL DEFAULT 0,
  d1         BOOLEAN NOT NULL DEFAULT 0,
  created_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- Where EMA14 / Smooth10 stood at the last check of a pair and timeframe, for the screen.
CREATE TABLE emacross_states (
  pair_id     INTEGER NOT NULL REFERENCES pairs(id),
  timeframe   TEXT NOT NULL CHECK (timeframe IN ('H1', 'H4', 'D1')),
  candle_time INTEGER NOT NULL,                   -- open time of the running candle, unix seconds
  trend       TEXT NOT NULL CHECK (trend IN ('UP', 'DOWN')),
  gap_percent REAL NOT NULL,                      -- (EMA14 - Smooth10) / Smooth10 * 100
  price       DECIMAL_TEXT NOT NULL,
  checked_at  TIMESTAMP_TEXT NOT NULL,
  PRIMARY KEY (pair_id, timeframe)
) WITHOUT ROWID;

-- One row per cross: at most one alert per pair, timeframe and candle (the Rails job sent the same
-- alert again on every run while the cross held).
CREATE TABLE emacross_alerts (
  id          INTEGER PRIMARY KEY,
  pair_id     INTEGER NOT NULL REFERENCES pairs(id),
  timeframe   TEXT NOT NULL CHECK (timeframe IN ('H1', 'H4', 'D1')),
  candle_time INTEGER NOT NULL,                   -- open time of the candle where EMA14 crossed Smooth10
  direction   TEXT NOT NULL CHECK (direction IN ('UP', 'DOWN')),
  price       DECIMAL_TEXT NOT NULL,
  gap_percent REAL NOT NULL,
  sent_at     TIMESTAMP_TEXT,                     -- NULL: the Discord message failed (see error)
  error       TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMP_TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  UNIQUE (pair_id, timeframe, candle_time)
);
CREATE INDEX idx_emacross_alerts_created ON emacross_alerts(created_at);

-- +goose Down
DROP TABLE emacross_alerts;
DROP TABLE emacross_states;
DROP TABLE emacross_pairs;
