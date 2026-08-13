-- 1. User Accounts
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(32) PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin INTEGER DEFAULT 0 NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

-- 2. Sessions (API authentication tokens)
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    auth_code VARCHAR(32) DEFAULT '' NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 4. Item Metadata and Content
CREATE TABLE IF NOT EXISTS sync_items (
    id VARCHAR(32) PRIMARY KEY,
    file_name TEXT NOT NULL,
    mime_type VARCHAR(128) DEFAULT 'application/octet-stream' NOT NULL,
    joplin_id VARCHAR(32) DEFAULT '' NOT NULL,
    parent_id VARCHAR(32) DEFAULT '' NOT NULL,
    share_id VARCHAR(32) DEFAULT '' NOT NULL,
    item_type INTEGER DEFAULT 0 NOT NULL,
    is_encrypted INTEGER DEFAULT 0 NOT NULL,
    client_updated_at BIGINT DEFAULT 0 NOT NULL,
    owner_id VARCHAR(32) NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY(owner_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sync_items_joplin_id ON sync_items(joplin_id);
CREATE INDEX IF NOT EXISTS idx_sync_items_file_name ON sync_items(file_name);

-- 5. User-Item Ownership Mapping
CREATE TABLE IF NOT EXISTS user_sync_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id VARCHAR(32) NOT NULL,
    sync_item_id VARCHAR(32) NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE(user_id, sync_item_id),
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_sync_items_user_id ON user_sync_items(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sync_items_sync_item_id ON user_sync_items(sync_item_id);

-- 6. Delta Sync Log
CREATE TABLE IF NOT EXISTS delta_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_uuid VARCHAR(32) UNIQUE NOT NULL,
    joplin_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) DEFAULT '' NOT NULL,
    file_name TEXT DEFAULT '' NOT NULL,
    previous_share_id VARCHAR(32) DEFAULT '' NOT NULL,
    item_type INTEGER NOT NULL,
    event_type INTEGER NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE(user_id, id),
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_delta_events_joplin_id ON delta_events(joplin_id);

-- 7. Key-Value & Lock Storage
CREATE TABLE IF NOT EXISTS sync_locks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lock_key TEXT UNIQUE NOT NULL,
    lock_type INTEGER NOT NULL,
    lock_data TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sync_locks_lock_key ON sync_locks(lock_key);

-- 8. Sharing
CREATE TABLE IF NOT EXISTS shares (
    id VARCHAR(32) PRIMARY KEY,
    owner_id VARCHAR(32) NOT NULL,
    folder_id VARCHAR(32) NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    FOREIGN KEY(owner_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    share_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    status INTEGER DEFAULT 0 NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE(share_id, user_id),
    FOREIGN KEY(share_id) REFERENCES shares(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_shares_user_id ON user_shares(user_id);
CREATE INDEX IF NOT EXISTS idx_user_shares_share_id ON user_shares(share_id);
