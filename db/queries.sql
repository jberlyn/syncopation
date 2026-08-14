-- name: CreateUser :one
INSERT INTO users (
  id, email, password_hash, is_admin, created_at, updated_at
) VALUES (
  ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ? LIMIT 1;

-- name: CreateSession :one
INSERT INTO sessions (
  id, user_id, auth_code, created_at, updated_at
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

-- name: SetSyncLock :one
INSERT INTO sync_locks (
  lock_key, lock_type, lock_data
) VALUES (
  ?, ?, ?
)
ON CONFLICT(lock_key) DO UPDATE SET
  lock_type = excluded.lock_type,
  lock_data = excluded.lock_data
RETURNING *;

-- name: GetSyncLock :one
SELECT * FROM sync_locks
WHERE lock_key = ? LIMIT 1;

-- name: DeleteSyncLock :exec
DELETE FROM sync_locks
WHERE lock_key = ?;

-- name: ListSyncLocksByType :many
SELECT * FROM sync_locks
WHERE lock_type = ?;

-- name: UpsertSyncItem :one
INSERT INTO sync_items (
  id, file_name, mime_type, joplin_id, parent_id, share_id,
  item_type, is_encrypted, client_updated_at, owner_id,
  created_at, updated_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT(id) DO UPDATE SET
  file_name = excluded.file_name,
  mime_type = excluded.mime_type,
  joplin_id = excluded.joplin_id,
  parent_id = excluded.parent_id,
  share_id = excluded.share_id,
  item_type = excluded.item_type,
  is_encrypted = excluded.is_encrypted,
  client_updated_at = excluded.client_updated_at,
  owner_id = excluded.owner_id,
  updated_at = excluded.updated_at
RETURNING *;

-- name: UpsertUserSyncItem :one
INSERT INTO user_sync_items (
  user_id, sync_item_id, created_at, updated_at
) VALUES (
  ?, ?, ?, ?
)
ON CONFLICT(user_id, sync_item_id) DO UPDATE SET
  updated_at = excluded.updated_at
RETURNING *;

-- name: GetSyncItemByFileNameAndUser :one
SELECT sync_items.* FROM sync_items
JOIN user_sync_items ON sync_items.id = user_sync_items.sync_item_id
WHERE sync_items.file_name = ? AND user_sync_items.user_id = ?
LIMIT 1;

-- name: DeleteSyncItemByFileNameAndUser :exec
DELETE FROM sync_items
WHERE file_name = ? AND id IN (
  SELECT sync_item_id FROM user_sync_items WHERE user_id = ?
);

-- name: DeleteUserSyncItem :exec
DELETE FROM user_sync_items
WHERE user_id = ? AND sync_item_id = ?;

-- name: InsertDeltaEvent :one
INSERT INTO delta_events (
  event_uuid, joplin_id, user_id, file_name, previous_share_id, item_type, event_type, created_at, updated_at
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetDeltaEventsByUser :many
SELECT * FROM delta_events
WHERE user_id = ? AND id > ?
ORDER BY id ASC
LIMIT ?;

-- name: ListSyncItemsByUser :many
SELECT sync_items.* FROM sync_items
JOIN user_sync_items ON sync_items.id = user_sync_items.sync_item_id
WHERE user_sync_items.user_id = ?
ORDER BY sync_items.updated_at ASC
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: GetUser :one
SELECT * FROM users
WHERE id = ? LIMIT 1;


-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: GetInstanceStats :one
SELECT 
  (SELECT COUNT(*) FROM users) as total_users,
  (SELECT COUNT(*) FROM sync_items) as total_items;

-- name: GetUserStats :many
SELECT 
  users.id as user_id, 
  users.email,
  users.is_admin,
  users.created_at,
  COUNT(user_sync_items.sync_item_id) as total_items
FROM users
LEFT JOIN user_sync_items ON users.id = user_sync_items.user_id
GROUP BY users.id, users.email, users.is_admin, users.created_at
ORDER BY users.created_at ASC;

-- name: InsertShareTombstonesForDeletedUser :exec
INSERT INTO delta_events (
  event_uuid, joplin_id, user_id, file_name, previous_share_id, item_type, event_type, created_at, updated_at
)
SELECT 
  lower(hex(randomblob(16))),
  sync_items.id,
  user_shares.user_id,
  sync_items.file_name,
  sync_items.share_id,
  sync_items.item_type,
  3,
  ?,
  ?
FROM sync_items
JOIN shares ON sync_items.share_id = shares.id
JOIN user_shares ON shares.id = user_shares.share_id
WHERE shares.owner_id = ?;

-- name: CreateShare :one
INSERT INTO shares (
  id, owner_id, folder_id, created_at, updated_at
) VALUES (
  ?, ?, ?, ?, ?
)
RETURNING *;

-- name: CreateUserShare :one
INSERT INTO user_shares (
  share_id, user_id, status, created_at, updated_at
) VALUES (
  ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteSessionsByUserId :exec
DELETE FROM sessions
WHERE user_id = ?;
