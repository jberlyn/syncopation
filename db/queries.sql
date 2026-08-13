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

-- name: UpsertItem :one
INSERT INTO items (
  id, name, mime_type, content, content_size, jop_id, jop_parent_id, jop_share_id,
  jop_type, jop_encryption_applied, jop_updated_time, owner_id, content_storage_id,
  created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  mime_type = excluded.mime_type,
  content = excluded.content,
  content_size = excluded.content_size,
  jop_id = excluded.jop_id,
  jop_parent_id = excluded.jop_parent_id,
  jop_share_id = excluded.jop_share_id,
  jop_type = excluded.jop_type,
  jop_encryption_applied = excluded.jop_encryption_applied,
  jop_updated_time = excluded.jop_updated_time,
  owner_id = excluded.owner_id,
  content_storage_id = excluded.content_storage_id,
  updated_time = excluded.updated_time
RETURNING *;

-- name: UpsertUserItem :one
INSERT INTO user_items (
  user_id, item_id, created_time, updated_time
) VALUES (
  ?, ?, ?, ?
)
ON CONFLICT(user_id, item_id) DO UPDATE SET
  updated_time = excluded.updated_time
RETURNING *;

-- name: GetItemByNameAndUser :one
SELECT items.* FROM items
JOIN user_items ON items.id = user_items.item_id
WHERE items.name = ? AND user_items.user_id = ?
LIMIT 1;

-- name: DeleteItemByNameAndUser :exec
DELETE FROM items
WHERE name = ? AND id IN (
  SELECT item_id FROM user_items WHERE user_id = ?
);

-- name: DeleteUserItem :exec
DELETE FROM user_items
WHERE user_id = ? AND item_id = ?;

-- name: InsertChange :one
INSERT INTO changes_2 (
  id, item_id, user_id, item_name, previous_share_id, item_type, type, created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetChangesByUser :many
SELECT * FROM changes_2
WHERE user_id = ? AND counter > ?
ORDER BY counter ASC
LIMIT ?;

-- name: ListItemsByUser :many
SELECT items.* FROM items
JOIN user_items ON items.id = user_items.item_id
WHERE user_items.user_id = ?
ORDER BY items.updated_time ASC
LIMIT ? OFFSET ?;
