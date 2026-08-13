-- name: CreateUser :one
INSERT INTO users (
  id, email, password, is_admin, created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?, ?
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
  id, name, mime_type, jop_id, jop_parent_id, jop_share_id,
  jop_type, jop_encryption_applied, jop_updated_time, owner_id, content_storage_id,
  created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  mime_type = excluded.mime_type,
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

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: GetUser :one
SELECT * FROM users
WHERE id = ? LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_time ASC;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: GetInstanceStats :one
SELECT 
  (SELECT COUNT(*) FROM users) as total_users,
  (SELECT COUNT(*) FROM items) as total_items;

-- name: GetUserStats :many
SELECT 
  users.id as user_id, 
  users.email,
  users.is_admin,
  users.created_time,
  COUNT(user_items.item_id) as total_items
FROM users
LEFT JOIN user_items ON users.id = user_items.user_id
GROUP BY users.id, users.email, users.is_admin, users.created_time
ORDER BY users.created_time ASC;

-- name: InsertShareTombstonesForDeletedUser :exec
INSERT INTO changes_2 (
  id, item_id, user_id, item_name, previous_share_id, item_type, type, created_time, updated_time
)
SELECT 
  lower(hex(randomblob(16))),
  items.id,
  user_shares.user_id,
  items.name,
  items.jop_share_id,
  items.jop_type,
  3,
  ?,
  ?
FROM items
JOIN shares ON items.jop_share_id = shares.id
JOIN user_shares ON shares.id = user_shares.share_id
WHERE shares.owner_id = ?;

-- name: CreateShare :one
INSERT INTO shares (
  id, owner_id, folder_id, created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?
)
RETURNING *;

-- name: CreateUserShare :one
INSERT INTO user_shares (
  share_id, user_id, status, created_time, updated_time
) VALUES (
  ?, ?, ?, ?, ?
)
RETURNING *;
