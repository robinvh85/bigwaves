-- name: ListDcaSettings :many
SELECT sqlc.embed(s), p.name AS pair_name, p.current_price, p.step_size, p.min_notional, p.is_spot, p.is_isolated
FROM dca_settings s JOIN pairs p ON p.id = s.pair_id
WHERE (sqlc.narg('status') IS NULL OR s.status = sqlc.narg('status'))
ORDER BY CASE s.status WHEN 'ACTIVE' THEN 0 WHEN 'INACTIVE' THEN 1 ELSE 2 END, s.id DESC;

-- name: GetDcaSetting :one
SELECT sqlc.embed(s), p.name AS pair_name, p.current_price, p.step_size, p.min_notional, p.is_spot, p.is_isolated
FROM dca_settings s JOIN pairs p ON p.id = s.pair_id
WHERE s.id = ?;

-- name: CreateDcaSetting :one
INSERT INTO dca_settings (pair_id, note) VALUES (?, ?) RETURNING *;

-- name: UpdateDcaSetting :exec
UPDATE dca_settings SET status = ?, note = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: DeleteDcaSetting :exec
DELETE FROM dca_settings WHERE id = ?;

-- name: ListDcaTrades :many
SELECT * FROM dca_trades WHERE dca_setting_id = ? ORDER BY buy_price DESC, id;

-- name: ListActiveDcaTrades :many
SELECT * FROM dca_trades WHERE status IN ('OPENING', 'OPEN') ORDER BY id;

-- name: ListOpenDcaTradeCosts :many
SELECT buy_quote FROM dca_trades WHERE status = 'OPEN';

-- name: GetDcaTrade :one
SELECT * FROM dca_trades WHERE id = ?;

-- name: CreateDcaTrade :one
INSERT INTO dca_trades (dca_setting_id, buy_price, sell_price, amount, open_order_id, close_order_id)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: SetDcaTradeOrderIDs :exec
UPDATE dca_trades SET open_order_id = ?, close_order_id = ? WHERE id = ?;

-- name: SaveDcaTrade :exec
UPDATE dca_trades SET status = ?, buy_price = ?, sell_price = ?, amount = ?, held_amount = ?, open_price = ?,
  close_price = ?, buy_quote = ?, sell_quote = ?, fee_usdt = ?, extra_fee_usdt = ?, realized_profit = ?,
  unrealized_profit = ?, last_error = ?, open_at = ?, close_at = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: DeleteDcaTrade :exec
DELETE FROM dca_trades WHERE id = ?;
