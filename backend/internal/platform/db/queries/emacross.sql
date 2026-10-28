-- name: ListEmacrossPairs :many
SELECT sqlc.embed(w), p.name AS pair_name, p.status AS pair_status, p.current_price, p.price_updated_at
FROM emacross_pairs w JOIN pairs p ON p.id = w.pair_id
WHERE p.deleted_at IS NULL
ORDER BY p.name;

-- name: GetEmacrossPair :one
SELECT sqlc.embed(w), p.name AS pair_name, p.status AS pair_status, p.current_price, p.price_updated_at
FROM emacross_pairs w JOIN pairs p ON p.id = w.pair_id
WHERE w.pair_id = ?;

-- name: UpsertEmacrossPair :exec
INSERT INTO emacross_pairs (pair_id, h1, h4, d1) VALUES (?, ?, ?, ?)
ON CONFLICT (pair_id) DO UPDATE SET h1 = excluded.h1, h4 = excluded.h4, d1 = excluded.d1,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: DeleteEmacrossPair :exec
DELETE FROM emacross_pairs WHERE pair_id = ?;

-- name: ListEmacrossStates :many
SELECT * FROM emacross_states;

-- name: UpsertEmacrossState :exec
INSERT INTO emacross_states (pair_id, timeframe, candle_time, trend, gap_percent, price, checked_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (pair_id, timeframe) DO UPDATE SET candle_time = excluded.candle_time, trend = excluded.trend,
  gap_percent = excluded.gap_percent, price = excluded.price, checked_at = excluded.checked_at;

-- name: DeleteEmacrossStates :exec
DELETE FROM emacross_states WHERE pair_id = ?;

-- name: InsertEmacrossAlert :one
-- Returns no row when this candle already has its alert.
INSERT INTO emacross_alerts (pair_id, timeframe, candle_time, direction, price, gap_percent)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (pair_id, timeframe, candle_time) DO NOTHING
RETURNING id;

-- name: FinishEmacrossAlert :exec
UPDATE emacross_alerts SET sent_at = ?, error = ? WHERE id = ?;

-- name: ListEmacrossAlerts :many
SELECT sqlc.embed(a), p.name AS pair_name
FROM emacross_alerts a JOIN pairs p ON p.id = a.pair_id
WHERE (sqlc.narg('pair_id') IS NULL OR a.pair_id = sqlc.narg('pair_id'))
  AND (sqlc.narg('timeframe') IS NULL OR a.timeframe = sqlc.narg('timeframe'))
ORDER BY a.id DESC
LIMIT sqlc.arg('limit');

-- name: LastEmacrossAlerts :many
-- The latest alert of each watched pair and timeframe.
SELECT sqlc.embed(a), p.name AS pair_name
FROM emacross_alerts a JOIN pairs p ON p.id = a.pair_id
WHERE a.id IN (SELECT MAX(id) FROM emacross_alerts GROUP BY pair_id, timeframe);

-- name: DeleteEmacrossAlertsBefore :execrows
DELETE FROM emacross_alerts WHERE created_at < ?;
