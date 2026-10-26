-- name: GetStrategyConfig :one
SELECT * FROM strategy_configs WHERE strategy = ?;

-- name: InsertStrategyConfig :exec
INSERT INTO strategy_configs (strategy, config) VALUES (?, ?) ON CONFLICT (strategy) DO NOTHING;

-- name: UpdateStrategyConfig :execrows
UPDATE strategy_configs SET config = ?, version = version + 1, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE strategy = ? AND version = ?;

-- name: InsertConfigChange :exec
INSERT INTO config_changes (strategy, before, after, user_email) VALUES (?, ?, ?, ?);

-- name: ListConfigChanges :many
SELECT * FROM config_changes WHERE strategy = ? ORDER BY id DESC LIMIT ?;
