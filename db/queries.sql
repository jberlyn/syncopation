-- name: CreateUser :one
INSERT INTO users (
  id, email, password, full_name, is_admin, created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ? LIMIT 1;

-- name: CreateSession :one
INSERT INTO sessions (
  id, user_id, auth_code, created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions
WHERE id = ? LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = ?;

-- name: SetKeyValue :one
INSERT INTO key_values (
  key, type, value
) VALUES (
  ?, ?, ?
)
ON CONFLICT(key) DO UPDATE SET
  type = excluded.type,
  value = excluded.value
RETURNING *;

-- name: GetKeyValue :one
SELECT * FROM key_values
WHERE key = ? LIMIT 1;

-- name: DeleteKeyValue :exec
DELETE FROM key_values
WHERE key = ?;

-- name: ListKeyValuesByType :many
SELECT * FROM key_values
WHERE type = ?;
