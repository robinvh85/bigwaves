-- name: GetCoinByName :one
SELECT * FROM coins WHERE name = ?;

-- name: GetCoin :one
SELECT * FROM coins WHERE id = ?;

-- name: CreateCoin :one
INSERT INTO coins (name) VALUES (?)
ON CONFLICT (name) DO UPDATE SET name = excluded.name
RETURNING *;

-- name: UpdateCoinDescription :one
UPDATE coins SET description = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?
RETURNING *;

-- name: ListPairs :many
SELECT sqlc.embed(p), c.name AS coin_name, c.description AS coin_description
FROM pairs p JOIN coins c ON c.id = p.coin_id
WHERE (sqlc.narg('include_deleted') IS NOT NULL OR p.deleted_at IS NULL)
  AND (sqlc.narg('status') IS NULL OR p.status = sqlc.narg('status'))
ORDER BY p.name;

-- name: GetPair :one
SELECT sqlc.embed(p), c.name AS coin_name, c.description AS coin_description
FROM pairs p JOIN coins c ON c.id = p.coin_id
WHERE p.id = ?;

-- name: GetPairByName :one
SELECT * FROM pairs WHERE name = ?;

-- name: CreatePair :one
INSERT INTO pairs (coin_id, market_id, name, quote_asset, status, step_size, min_notional, is_spot, is_isolated)
VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: RestorePair :one
UPDATE pairs SET deleted_at = NULL, status = ?, step_size = ?, min_notional = ?, is_spot = ?, is_isolated = ?,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?
RETURNING *;

-- name: UpdatePair :one
UPDATE pairs SET status = ?, is_spot = ?, is_isolated = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeletePair :execrows
UPDATE pairs SET deleted_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND deleted_at IS NULL;

-- name: ListLivePairNames :many
SELECT id, name, status FROM pairs WHERE deleted_at IS NULL ORDER BY name;

-- name: SetPairPrice :exec
UPDATE pairs SET current_price = ?, price_updated_at = ? WHERE id = ?;

-- name: SetPairInterestRate :exec
UPDATE pairs SET interest_rate = ? WHERE id = ?;

-- name: SetPairFilters :exec
UPDATE pairs SET step_size = ?, min_notional = ? WHERE id = ?;

-- name: PricesByNames :many
SELECT name, current_price FROM pairs WHERE deleted_at IS NULL AND current_price IS NOT NULL;
