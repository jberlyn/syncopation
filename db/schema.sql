-- 1. User Accounts
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(32) PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    is_admin INTEGER DEFAULT 0 NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL
);

-- 2. Sessions (API authentication tokens)
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    auth_code VARCHAR(32) DEFAULT '' NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id)
);

-- 3. Storage Driver Registry
CREATE TABLE IF NOT EXISTS storages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_string TEXT UNIQUE NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL
);

-- 4. Item Metadata and Content
CREATE TABLE IF NOT EXISTS items (
    id VARCHAR(32) PRIMARY KEY,
    name TEXT NOT NULL,
    mime_type VARCHAR(128) DEFAULT 'application/octet-stream' NOT NULL,
    jop_id VARCHAR(32) DEFAULT '' NOT NULL,
    jop_parent_id VARCHAR(32) DEFAULT '' NOT NULL,
    jop_share_id VARCHAR(32) DEFAULT '' NOT NULL,
    jop_type INTEGER DEFAULT 0 NOT NULL,
    jop_encryption_applied INTEGER DEFAULT 0 NOT NULL,
    jop_updated_time BIGINT DEFAULT 0 NOT NULL,
    owner_id VARCHAR(32) NOT NULL,
    content_storage_id INTEGER DEFAULT 1 NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_items_jop_id ON items(jop_id);
CREATE INDEX IF NOT EXISTS idx_items_name ON items(name);

-- 5. User-Item Ownership Mapping
CREATE TABLE IF NOT EXISTS user_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id VARCHAR(32) NOT NULL,
    item_id VARCHAR(32) NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL,
    UNIQUE(user_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_user_items_user_id ON user_items(user_id);
CREATE INDEX IF NOT EXISTS idx_user_items_item_id ON user_items(item_id);

-- 6. Delta Sync Log
CREATE TABLE IF NOT EXISTS changes_2 (
    counter INTEGER PRIMARY KEY AUTOINCREMENT,
    id VARCHAR(32) UNIQUE NOT NULL,
    item_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) DEFAULT '' NOT NULL,
    item_name TEXT DEFAULT '' NOT NULL,
    previous_share_id VARCHAR(32) DEFAULT '' NOT NULL,
    item_type INTEGER NOT NULL,
    type INTEGER NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL,
    UNIQUE(user_id, counter)
);
CREATE INDEX IF NOT EXISTS idx_changes2_item_id ON changes_2(item_id);

-- 7. Key-Value & Lock Storage
CREATE TABLE IF NOT EXISTS key_values (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    type INTEGER NOT NULL,
    value TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_key_values_key ON key_values(key);

-- 8. Sharing
CREATE TABLE IF NOT EXISTS shares (
    id VARCHAR(32) PRIMARY KEY,
    owner_id VARCHAR(32) NOT NULL,
    folder_id VARCHAR(32) NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL,
    FOREIGN KEY(owner_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS user_shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    share_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    status INTEGER DEFAULT 0 NOT NULL,
    created_time BIGINT NOT NULL,
    updated_time BIGINT NOT NULL,
    UNIQUE(share_id, user_id),
    FOREIGN KEY(share_id) REFERENCES shares(id),
    FOREIGN KEY(user_id) REFERENCES users(id)
);
CREATE INDEX IF NOT EXISTS idx_user_shares_user_id ON user_shares(user_id);
CREATE INDEX IF NOT EXISTS idx_user_shares_share_id ON user_shares(share_id);
