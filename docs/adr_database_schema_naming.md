# ADR: Database Schema Modernization & Naming Conventions

## Context
When initially building Syncopation, the database schema (tables and columns) was copied 1:1 from the original Node.js Joplin Server implementation (e.g., `changes_2`, `jop_id`). This ensured strict conceptual parity during the first phases of the project, making it easier to verify that the core mechanics worked exactly like the original.

However, since Syncopation is a clean Go rewrite, and Joplin clients only care about the JSON structure returned by the REST API (not the internal database schema), we have the opportunity to modernize our database naming conventions. 

This document serves as the discovery phase for every table and column, proposing a clean, idiomatic naming convention that breaks away from historical Joplin Server debt while maintaining full API compatibility.

## Discovery and Proposed Renaming

### 1. `users` -> `users` (No table name change)
*Purpose: Stores user account credentials, including email, hashed password, and administrator status.*
- `id` -> `id`
- `email` -> `email`
- `password` -> `password_hash` *(Clarifies that this stores a hash, not plaintext)*
- `is_admin` -> `is_admin`
- `created_time` -> `created_at` *(Standard SQL convention for timestamps)*
- `updated_time` -> `updated_at`

### 2. `sessions` -> `sessions` (No table name change)
*Purpose: Tracks active authenticated API sessions. When a user logs in, an API token is generated and stored here to authorize subsequent requests.*
- `id` -> `id`
- `user_id` -> `user_id`
- `auth_code` -> `auth_code`
- `created_time` -> `created_at`
- `updated_time` -> `updated_at`

### 3. `storages` -> **(DROPPED)**
*Purpose: In the original Joplin Server, this tracked whether payloads were in the DB, local disk, or S3. Since Syncopation is explicitly designed for SQLite + Local Disk only (as per the tech stack ADR), this abstraction is unnecessary legacy bloat. We will drop this table entirely.*

### 4. `items` -> `sync_items`
*(`items` is highly generic. `sync_items` clarifies this is the core synchronizable entity)*
*Purpose: The core table storing metadata for every synchronized Joplin entity (notes, resources, tags, folders). Note that actual binary content is stored in the `storage_backends` location.*
- `id` -> `id`
- `name` -> `file_name` *(More accurately describes the `root:/<path>:` name)*
- `mime_type` -> `mime_type`
- `jop_id` -> `joplin_id` *(Removes the abbreviated prefix)*
- `jop_parent_id` -> `parent_id`
- `jop_share_id` -> `share_id`
- `jop_type` -> `item_type`
- `jop_encryption_applied` -> `is_encrypted` *(Boolean-like fields should use `is_` prefix)*
- `jop_updated_time` -> `client_updated_at` *(Clarifies this is the timestamp from the client, not the server)*
- `owner_id` -> `owner_id`
- `content_storage_id` -> **(DROPPED)** *(We only use local disk, so we don't need to track which storage backend holds the file)*
- `created_time` -> `created_at`
- `updated_time` -> `updated_at`

### 5. `user_items` -> `user_sync_items`
*Purpose: A mapping table tracking which user owns or has access to which sync item, acting as an access control list (ACL).*
- `id` -> `id`
- `user_id` -> `user_id`
- `item_id` -> `sync_item_id`
- `created_time` -> `created_at`
- `updated_time` -> `updated_at`

### 6. `changes_2` -> `delta_events`
*(Removes the historical `_2` migration artifact and clarifies the table's purpose for delta sync)*
*Purpose: An append-only log recording every creation, update, or deletion of a sync item. It drives the incremental "delta sync" engine, allowing clients to quickly pull only the changes that occurred since their last sync cursor.*
- `counter` -> `id` *(Standardizes the primary key name)*
- `id` -> `event_uuid` *(The string UUID of the event, distinguishing it from the PK id)*
- `item_id` -> `joplin_id` *(Matches the renamed field in `sync_items`)*
- `user_id` -> `user_id`
- `item_name` -> `file_name`
- `previous_share_id` -> `previous_share_id`
- `item_type` -> `item_type`
- `type` -> `event_type` *(Clarifies this is an enum of Create/Update/Delete)*
- `created_time` -> `created_at`
- `updated_time` -> `updated_at`

### 7. `key_values` -> `sync_locks`
*Purpose: Used exclusively by the server to manage distributed concurrency locks. These locks prevent two clients from overwriting each other if they try to sync simultaneously.*
- `id` -> `id`
- `key` -> `lock_key`
- `type` -> `lock_type`
- `value` -> `lock_data` *(Stores the JSON blob with lock details)*

### 8. `shares` -> `shares` (No table name change)
*Purpose: Tracks shared folders that a user has published or shared with others, enabling the multi-user notebook sharing features.*
- `id` -> `id`
- `owner_id` -> `owner_id`
- `folder_id` -> `folder_id`
- `created_time` -> `created_at`
- `updated_time` -> `updated_at`

### 9. `user_shares` -> `user_shares` (No table name change)
*Purpose: Tracks the participants (users) who have been invited to or have accepted access to a shared folder.*
- `id` -> `id`
- `share_id` -> `share_id`
- `user_id` -> `user_id`
- `status` -> `status`
- `created_time` -> `created_at`
- `updated_time` -> `updated_at`

## Implementation Plan (Slice 8.5)
We will introduce **Slice 8.5: Database Schema Modernization** to implement this.
To execute this properly:
1. Write a new SQL migration to rename these tables and columns.
2. Refactor all queries in `db/queries.sql` to use the new idiomatic names.
3. Re-run `sqlc generate` to update the generated Go models and query functions.
4. Update the Go codebase (handlers and services) to handle any struct field name changes (e.g. `CreatedTime` to `CreatedAt`).
5. **Critical:** Ensure JSON responses in handlers remain 100% strictly compatible with Joplin clients by explicitly mapping struct properties back to their legacy JSON keys using struct tags (e.g., `json:"updated_time"`).
