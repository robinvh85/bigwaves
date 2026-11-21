-- +goose Up

-- EMA1D sizes no longer depend on a core / sensitive group (one LONG and one SHORT size for every
-- coin): the split had no out-of-sample value (tradingview-ema review2611).
ALTER TABLE ema1d_pairs DROP COLUMN pair_group;

-- +goose Down
ALTER TABLE ema1d_pairs ADD COLUMN pair_group TEXT NOT NULL DEFAULT 'CORE' CHECK (pair_group IN ('CORE', 'SENSITIVE'));
