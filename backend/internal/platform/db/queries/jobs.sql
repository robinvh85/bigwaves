-- name: StartJobRun :one
INSERT INTO job_runs (job, trigger, status, started_at) VALUES (?, ?, 'RUNNING', ?) RETURNING id;

-- name: FinishJobRun :exec
UPDATE job_runs SET status = ?, message = ?, finished_at = ? WHERE id = ?;

-- name: ListJobRuns :many
SELECT * FROM job_runs WHERE job = ? ORDER BY id DESC LIMIT ?;

-- name: LatestJobRuns :many
SELECT r.* FROM job_runs r
JOIN (SELECT job, MAX(id) AS id FROM job_runs GROUP BY job) last ON last.id = r.id;

-- name: LatestFinishedJobRuns :many
SELECT r.* FROM job_runs r
JOIN (SELECT job, MAX(id) AS id FROM job_runs WHERE status <> 'RUNNING' GROUP BY job) last ON last.id = r.id;

-- name: MarkStaleJobRuns :exec
UPDATE job_runs SET status = 'FAILED', message = 'interrupted (process restarted)', finished_at = ?
WHERE status = 'RUNNING';

-- name: DeleteJobRunsBefore :execrows
DELETE FROM job_runs WHERE started_at < ?;

-- name: GetErrorEvent :one
SELECT * FROM error_events WHERE fingerprint = ?;

-- name: UpsertErrorEvent :one
INSERT INTO error_events (fingerprint, source, message, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (fingerprint) DO UPDATE SET
  count = count + 1, message = excluded.message, last_seen_at = excluded.last_seen_at
RETURNING *;

-- name: MarkErrorNotified :exec
UPDATE error_events SET notified_at = ? WHERE fingerprint = ?;
