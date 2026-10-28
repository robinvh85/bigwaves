-- name: ListEmaSettings :many
SELECT sqlc.embed(s), p.name AS pair_name, c.name AS coin_name, p.status AS pair_status, p.current_price,
  p.step_size, p.min_notional, p.is_spot, p.is_isolated
FROM ema_settings s JOIN pairs p ON p.id = s.pair_id JOIN coins c ON c.id = p.coin_id
WHERE (sqlc.narg(status) IS NULL OR s.status = sqlc.narg(status))
ORDER BY s.status = 'DONE', s.created_at DESC, s.id DESC;

-- name: GetEmaSetting :one
SELECT sqlc.embed(s), p.name AS pair_name, c.name AS coin_name, p.status AS pair_status, p.current_price,
  p.step_size, p.min_notional, p.is_spot, p.is_isolated
FROM ema_settings s JOIN pairs p ON p.id = s.pair_id JOIN coins c ON c.id = p.coin_id
WHERE s.id = ?;

-- name: CreateEmaSetting :one
INSERT INTO ema_settings (pair_id, budget, trade_type, period, check_type, next_trade_type, tp_percent, sl_percent, status, note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateEmaSetting :exec
UPDATE ema_settings SET budget = ?, trade_type = ?, period = ?, check_type = ?, next_trade_type = ?, tp_percent = ?,
  sl_percent = ?, status = ?, note = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetEmaSettingStatus :exec
UPDATE ema_settings SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: DeleteEmaSetting :exec
DELETE FROM ema_settings WHERE id = ?;

-- name: ListEmaSettingTrades :many
SELECT * FROM ema_setting_trades WHERE setting_id = ? ORDER BY id DESC;

-- name: ListAllEmaSettingTrades :many
-- Every trade, for the totals of the settings list (money is summed in Go, never in SQL).
SELECT * FROM ema_setting_trades ORDER BY setting_id, id DESC;

-- name: GetEmaSettingTrade :one
SELECT * FROM ema_setting_trades WHERE id = ?;

-- name: CreateEmaSettingTrade :one
INSERT INTO ema_setting_trades (setting_id, pair_id, side, status, is_auto, amount, entry_price, target_price, sl_price,
  open_price, unrealized_profit, open_order_id, close_order_id, open_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: SetEmaSettingTradeOrderIDs :exec
UPDATE ema_setting_trades SET open_order_id = ?, close_order_id = ? WHERE id = ?;

-- name: SaveEmaSettingTrade :exec
UPDATE ema_setting_trades SET side = ?, status = ?, amount = ?, entry_price = ?, target_price = ?, sl_price = ?,
  open_price = ?, close_price = ?, fee_usdt = ?, interest_paid = ?, interest_usdt = ?, realized_profit = ?,
  unrealized_profit = ?, close_reason = ?, first_attempted_at = ?, last_error = ?, open_at = ?, close_at = ?,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetEmaSettingTradeBorrowStep :exec
UPDATE ema_setting_trades SET borrow_step = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: DeleteEmaSettingTrade :exec
DELETE FROM ema_setting_trades WHERE id = ?;

-- name: ActiveSettingShorts :many
-- Shorts of EMA Settings holding (or about to hold) the coin of a pair: the cross-margin debt is
-- per coin, shared with EMA1D. id <> ? leaves the trade itself out (0: none).
SELECT id FROM ema_setting_trades
WHERE pair_id = ? AND side = 'SHORT' AND status IN ('OPENING', 'OPEN') AND id <> ?
ORDER BY id;
