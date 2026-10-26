-- ---------------------------------------------------------------- pairs

-- name: ListEma1dPairs :many
SELECT sqlc.embed(e), p.name AS pair_name, c.name AS coin_name, p.current_price, p.step_size, p.min_notional,
       p.is_spot, p.is_isolated
FROM ema1d_pairs e JOIN pairs p ON p.id = e.pair_id JOIN coins c ON c.id = p.coin_id
ORDER BY CASE e.status WHEN 'ACTIVE' THEN 0 WHEN 'EXIT_ONLY' THEN 1 ELSE 2 END, e.id;

-- name: GetEma1dPair :one
SELECT sqlc.embed(e), p.name AS pair_name, c.name AS coin_name, p.current_price, p.step_size, p.min_notional,
       p.is_spot, p.is_isolated
FROM ema1d_pairs e JOIN pairs p ON p.id = e.pair_id JOIN coins c ON c.id = p.coin_id
WHERE e.id = ?;

-- name: GetEma1dPairByCoin :one
SELECT sqlc.embed(e), p.name AS pair_name, c.name AS coin_name, p.current_price, p.step_size, p.min_notional,
       p.is_spot, p.is_isolated
FROM ema1d_pairs e JOIN pairs p ON p.id = e.pair_id JOIN coins c ON c.id = p.coin_id
WHERE c.name = ?;

-- name: GetEma1dPairByPairID :one
SELECT * FROM ema1d_pairs WHERE pair_id = ?;

-- name: CreateEma1dPair :one
INSERT INTO ema1d_pairs (pair_id, pair_group, allow_long, allow_short, short_supported, status, added_on,
                         long_exit_rule, signal_from, check_note)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateEma1dPair :exec
UPDATE ema1d_pairs SET pair_group = ?, status = ?, allow_long = ?, allow_short = ?, added_on = ?, removed_on = ?,
  long_exit_rule = ?, signal_from = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetEma1dPairCheck :exec
UPDATE ema1d_pairs SET short_supported = ?, checked_at = ?, check_note = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- ---------------------------------------------------------------- positions

-- name: CreatePosition :one
INSERT INTO ema1d_positions (ema1d_pair_id, pair_id, side, status, signal_date, exit_rule, amount, ref_open_price,
  size_percent, equity_at_open, notional_usdt, open_order_id, close_order_id)
VALUES (?, ?, ?, 'OPENING', ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: SetPositionOrderIDs :exec
UPDATE ema1d_positions SET open_order_id = ?, close_order_id = ? WHERE id = ?;

-- name: GetPosition :one
SELECT * FROM ema1d_positions WHERE id = ?;

-- name: SavePosition :exec
UPDATE ema1d_positions SET status = ?, amount = ?, open_price = ?, close_price = ?, ref_close_price = ?, fee_usdt = ?,
  interest_paid = ?, interest_usdt = ?, realized_profit = ?, unrealized_profit = ?, open_at = ?, close_at = ?,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: DeletePosition :exec
DELETE FROM ema1d_positions WHERE id = ?;

-- name: ActivePositionForPair :one
SELECT * FROM ema1d_positions WHERE ema1d_pair_id = ? AND status IN ('OPENING', 'OPEN') ORDER BY id DESC LIMIT 1;

-- name: ActivePositionBySignal :one
SELECT * FROM ema1d_positions
WHERE pair_id = ? AND side = ? AND signal_date = ? AND status IN ('OPENING', 'OPEN')
ORDER BY id DESC LIMIT 1;

-- name: OtherActiveShorts :many
SELECT id FROM ema1d_positions WHERE pair_id = ? AND side = 'SHORT' AND status IN ('OPENING', 'OPEN') AND id <> ?;

-- name: ListActivePositions :many
SELECT sqlc.embed(x), p.name AS pair_name, c.name AS coin_name, p.current_price
FROM ema1d_positions x JOIN pairs p ON p.id = x.pair_id JOIN coins c ON c.id = p.coin_id
WHERE x.status IN ('OPENING', 'OPEN')
ORDER BY x.id;

-- name: ListClosedPositionTotals :many
SELECT realized_profit, interest_usdt, fee_usdt FROM ema1d_positions WHERE status = 'CLOSED';

-- name: ListPositions :many
SELECT sqlc.embed(x), p.name AS pair_name, c.name AS coin_name, p.current_price
FROM ema1d_positions x JOIN pairs p ON p.id = x.pair_id JOIN coins c ON c.id = p.coin_id
WHERE (CASE WHEN sqlc.arg('closed') THEN x.status = 'CLOSED' ELSE x.status IN ('OPENING', 'OPEN') END)
  AND (sqlc.narg('coin') IS NULL OR c.name = sqlc.narg('coin'))
  AND (sqlc.narg('side') IS NULL OR x.side = sqlc.narg('side'))
  AND (sqlc.narg('from') IS NULL OR (CASE WHEN sqlc.arg('closed') THEN x.close_at ELSE x.open_at END) >= sqlc.narg('from'))
  AND (sqlc.narg('to') IS NULL OR (CASE WHEN sqlc.arg('closed') THEN x.close_at ELSE x.open_at END) < sqlc.narg('to'))
ORDER BY COALESCE(x.close_at, x.open_at) DESC, x.id DESC
LIMIT 500;

-- ---------------------------------------------------------------- todos

-- name: CreateTodo :one
INSERT INTO ema1d_todos (ema1d_pair_id, action, side, signal_date, ref_date, ref_price, exit_rule, source,
  ema1d_sync_id, depends_on_id, position_id, status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING')
RETURNING *;

-- name: GetTodo :one
SELECT * FROM ema1d_todos WHERE id = ?;

-- name: SaveTodo :exec
UPDATE ema1d_todos SET status = ?, attempts = ?, first_attempted_at = ?, run_after = ?, last_error = ?, step = ?,
  position_id = ?, depends_on_id = ?, ema1d_sync_id = ?, ref_date = ?, ref_price = ?,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

-- name: SetTodoStep :exec
UPDATE ema1d_todos SET step = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?;

-- name: InFlightTodosForPair :many
SELECT * FROM ema1d_todos WHERE ema1d_pair_id = ? AND status IN ('PENDING', 'RUNNING') ORDER BY id;

-- name: FindTodo :one
SELECT * FROM ema1d_todos
WHERE ema1d_pair_id = ? AND action = ? AND side = ? AND signal_date = ? AND status <> 'CANCELLED'
LIMIT 1;

-- name: ClaimNextTodo :one
UPDATE ema1d_todos
SET status = 'RUNNING', first_attempted_at = COALESCE(first_attempted_at, sqlc.arg('now')), updated_at = sqlc.arg('now')
WHERE id = (
  SELECT t.id FROM ema1d_todos t
  WHERE t.status = 'PENDING'
    AND (t.run_after IS NULL OR t.run_after <= sqlc.arg('now'))
    AND t.ema1d_pair_id NOT IN (SELECT r.ema1d_pair_id FROM ema1d_todos r WHERE r.status = 'RUNNING')
    AND (t.depends_on_id IS NULL OR t.depends_on_id IN (SELECT d.id FROM ema1d_todos d WHERE d.status = 'DONE'))
  ORDER BY CASE t.action WHEN 'CLOSE' THEN 0 ELSE 1 END, t.id
  LIMIT 1
)
RETURNING *;

-- name: RecoverStuckTodos :execrows
UPDATE ema1d_todos SET status = 'PENDING', last_error = 'recovered from a stuck RUNNING state',
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE status = 'RUNNING' AND updated_at < ?;

-- name: BrokenDependents :many
SELECT t.id, d.id AS dep_id, d.status AS dep_status FROM ema1d_todos t JOIN ema1d_todos d ON d.id = t.depends_on_id
WHERE t.status = 'PENDING' AND d.status IN ('FAILED', 'SKIPPED', 'CANCELLED');

-- name: StaleOpenTodos :many
SELECT id FROM ema1d_todos WHERE status = 'PENDING' AND action = 'OPEN' AND attempts = 0 AND created_at < ?;

-- name: ListTodosWithCoin :many
SELECT sqlc.embed(t), c.name AS coin_name
FROM ema1d_todos t JOIN ema1d_pairs e ON e.id = t.ema1d_pair_id JOIN pairs p ON p.id = e.pair_id JOIN coins c ON c.id = p.coin_id
WHERE t.status IN ('PENDING', 'RUNNING')
   OR (t.status = 'FAILED' AND t.updated_at >= sqlc.arg('failed_since'))
   OR (t.status = 'SKIPPED' AND t.updated_at >= sqlc.arg('skipped_since'))
   OR (sqlc.arg('all') AND t.updated_at >= sqlc.arg('all_since'))
ORDER BY t.id DESC
LIMIT 500;

-- ---------------------------------------------------------------- syncs

-- name: GetRealSync :one
SELECT * FROM ema1d_syncs WHERE run_id = ? AND signal_date = ? AND dry_run = 0;

-- name: CreateSync :one
INSERT INTO ema1d_syncs (run_id, signal_date, raw_query, parsed, result, dry_run) VALUES (?, ?, ?, ?, ?, ?) RETURNING *;

-- name: SetSyncResult :exec
UPDATE ema1d_syncs SET result = ? WHERE id = ?;

-- name: LastRealSync :one
SELECT * FROM ema1d_syncs WHERE dry_run = 0 ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: CountRealSyncsForDate :one
SELECT COUNT(*) FROM ema1d_syncs WHERE dry_run = 0 AND signal_date = ?;

-- name: DeleteDryRunSyncsBefore :execrows
DELETE FROM ema1d_syncs WHERE dry_run = 1 AND created_at < ?;

-- ---------------------------------------------------------------- candles

-- name: UpsertCandle :exec
INSERT INTO ema1d_candles (pair_id, interval, open_time, open, high, low, close) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (pair_id, interval, open_time) DO UPDATE SET open = excluded.open, high = excluded.high,
  low = excluded.low, close = excluded.close;

-- name: MaxCandleTime :one
SELECT CAST(COALESCE(MAX(open_time), 0) AS INTEGER) FROM ema1d_candles WHERE pair_id = ? AND interval = ?;

-- name: MaxCandleTimeBefore :one
SELECT CAST(COALESCE(MAX(open_time), 0) AS INTEGER) FROM ema1d_candles WHERE pair_id = ? AND interval = ? AND open_time < ?;

-- name: ListCandlesBefore :many
SELECT open_time, open, high, low, close FROM ema1d_candles
WHERE pair_id = ? AND interval = ? AND open_time < ?
ORDER BY open_time;

-- ---------------------------------------------------------------- snapshots

-- name: UpsertSnapshot :exec
INSERT INTO ema1d_equity_snapshots (date, equity_usdt, margin_level, open_long_notional, open_short_notional)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (date) DO UPDATE SET equity_usdt = excluded.equity_usdt, margin_level = excluded.margin_level,
  open_long_notional = excluded.open_long_notional, open_short_notional = excluded.open_short_notional;

-- name: ListSnapshotsSince :many
SELECT * FROM ema1d_equity_snapshots WHERE date >= ? ORDER BY date;

-- ---------------------------------------------------------------- daily checks

-- name: GetDailyCheck :one
SELECT * FROM ema1d_daily_checks WHERE signal_date = ?;

-- name: EnsureDailyCheck :exec
INSERT INTO ema1d_daily_checks (signal_date) VALUES (?) ON CONFLICT (signal_date) DO NOTHING;

-- name: SaveDailyCheck :exec
UPDATE ema1d_daily_checks SET status = ?, run_id = ?, attempts = ?, rows = ?, sync_params = ?, sync_result = ?,
  report = ?, last_error = ?, synced_at = ?, sent_at = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE signal_date = ?;

-- name: ListDailyChecks :many
SELECT * FROM ema1d_daily_checks ORDER BY signal_date DESC LIMIT ?;

-- ---------------------------------------------------------------- app state & orders

-- name: GetState :one
SELECT * FROM app_state WHERE key = ? AND (expires_at IS NULL OR expires_at > ?);

-- name: SetState :exec
INSERT INTO app_state (key, value, expires_at) VALUES (?, ?, ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at,
  updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now');

-- name: UpsertExchangeOrder :exec
INSERT INTO exchange_orders (client_order_id, strategy, ref_id, symbol, account, side, exchange_order_id, status,
  executed_qty, quote_qty, avg_price, net_qty, fee_usdt, raw, executed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (client_order_id) DO UPDATE SET status = excluded.status, executed_qty = excluded.executed_qty,
  quote_qty = excluded.quote_qty, avg_price = excluded.avg_price, net_qty = excluded.net_qty,
  fee_usdt = excluded.fee_usdt, raw = excluded.raw;

-- name: ListExchangeOrdersSince :many
SELECT * FROM exchange_orders WHERE created_at >= ? ORDER BY id;
