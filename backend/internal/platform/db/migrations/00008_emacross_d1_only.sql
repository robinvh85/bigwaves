-- +goose Up

-- EMA cross alerts run on D1 only: no pair watched H1 / H4, their jobs only skipped. A row in
-- emacross_pairs now means the pair is watched; the threshold becomes one number.
DELETE FROM emacross_states WHERE timeframe <> 'D1';
DELETE FROM emacross_alerts WHERE timeframe <> 'D1';
DELETE FROM emacross_pairs WHERE d1 = 0;
DELETE FROM emacross_states WHERE pair_id NOT IN (SELECT pair_id FROM emacross_pairs);
ALTER TABLE emacross_pairs DROP COLUMN h1;
ALTER TABLE emacross_pairs DROP COLUMN h4;
ALTER TABLE emacross_pairs DROP COLUMN d1;
UPDATE strategy_configs
SET config = json_set(config, '$.thresholdPercent', json_extract(config, '$.thresholdPercent.d1'))
WHERE strategy = 'emacross' AND json_type(config, '$.thresholdPercent') = 'object';

-- +goose Down
ALTER TABLE emacross_pairs ADD COLUMN h1 BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE emacross_pairs ADD COLUMN h4 BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE emacross_pairs ADD COLUMN d1 BOOLEAN NOT NULL DEFAULT 1;
UPDATE strategy_configs
SET config = json_set(config, '$.thresholdPercent', json_object(
  'h1', json_extract(config, '$.thresholdPercent'),
  'h4', json_extract(config, '$.thresholdPercent'),
  'd1', json_extract(config, '$.thresholdPercent')))
WHERE strategy = 'emacross' AND json_type(config, '$.thresholdPercent') = 'text';
