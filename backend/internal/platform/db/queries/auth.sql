-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? LIMIT 1;

-- name: GetUser :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: UpsertUser :one
INSERT INTO users (email, uid) VALUES (?, ?)
ON CONFLICT (email) DO UPDATE SET uid = excluded.uid, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
RETURNING *;

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, ip, user_agent, expires_at) VALUES (?, ?, ?, ?, ?);

-- name: GetSessionUser :one
SELECT u.id, u.email, s.expires_at
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.id = ? AND s.expires_at > ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= ?;
