# Architectural Decision Record: User Data Lifecycle & Deletion

## Context and Problem Statement

In the initial implementation of the admin dashboard, user deletion simply removes a row from the `users` table. However, since a user is tied to numerous entities (sessions, items, shares, changes) and physical BLOB data on disk via the Storage Driver, a standard SQL `DELETE` or `ON DELETE CASCADE` is insufficient. 

SQL-level cascading would delete database rows but orphan physical files in the local filesystem storage driver. Additionally, for accounts with thousands of items, a synchronous hard delete in an HTTP request would block the request, potentially time out, and cause massive database locks (which is highly problematic for SQLite).

We need a 100% Joplin-compatible and performant way to handle user deletion and data lifecycle cleanup.

## Findings from Official Joplin Server Codebase

Analysis of the official `joplin/joplin` repository (specifically `packages/server/src/services/UserDeletionService.ts` and `models/UserDeletionModel.ts`) reveals the following behavior:

1. **No synchronous hard deletes**: When a user account is deleted via the admin panel, the official server disables the account and queues a job in a `user_deletions` table.
2. **Background Garbage Collection**: A `UserDeletionService` polls the `user_deletions` table for pending jobs. 
3. **Account Freezing**: While deletion is in progress, a `UserDeletionInProgress` flag is applied to prevent any concurrent sync operations.
4. **Batched Deletion**: The worker deletes shares, then paginates through the user's items in batches of 1,000, calling the item delete mechanism (which cleans up disk/storage) and sleeping briefly between batches to avoid overloading the system.
5. **Final Cleanup**: Once all data is purged, it removes sessions, applications, notifications, and finally the user row itself.

## Proposed Technical Approach

To achieve 100% compatibility and avoid orphaned disk files, our Go sync server will implement a similar asynchronous background worker model.

### 1. Schema Changes
- Modify the `users` table to include an `enabled INTEGER DEFAULT 1` or `disabled_time BIGINT DEFAULT 0` column.
- Create a new `user_deletions` table:
  ```sql
  CREATE TABLE IF NOT EXISTS user_deletions (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      user_id VARCHAR(32) NOT NULL,
      scheduled_time BIGINT NOT NULL,
      start_time BIGINT DEFAULT 0 NOT NULL,
      end_time BIGINT DEFAULT 0 NOT NULL,
      success INTEGER DEFAULT 0 NOT NULL,
      error TEXT DEFAULT '' NOT NULL,
      created_time BIGINT NOT NULL,
      updated_time BIGINT NOT NULL
  );
  ```

### 2. Admin Deletion API (`DELETE /admin/users/:id`)
Instead of deleting the user row, the Admin API will:
1. Update `users.enabled = 0`.
2. Insert a record into `user_deletions` scheduled for immediate execution (or after a configurable TTL if soft-deletes are desired).
3. Terminate all active `sessions` for the user immediately to force logout.

### 3. Background Garbage Collection Worker (Goroutine)
A background worker (started in `main.go`) will poll the `user_deletions` table every minute. For each pending job:
1. **Mark Start**: Update `start_time` to claim the job.
2. **Share Cleanup**: Delete from `user_shares` and `shares` associated with the user.
3. **Item & File Cleanup**: Loop over `items` owned by `user_id` with `LIMIT 1000`.
    - For each item, call `storageDriver.Delete(item.ID)` to remove the physical file on disk.
    - Delete the `items`, `user_items`, and `changes_2` rows.
    - Sleep briefly between batches.
4. **Final User Deletion**: Delete the `users` row.
5. **Mark End**: Update `end_time` and `success = 1`.

### 4. Middleware & Sync Protection
The `AuthMiddleware` must be updated to reject any API requests for users where `enabled = 0`, ensuring clients cannot sync data while a background deletion is in progress.

## Consequences

- **Pros**: Prevents orphaned files on disk; avoids long-running synchronous HTTP requests; prevents SQLite lock starvation; perfectly mirrors the official server.
- **Cons**: Requires managing a background goroutine and introduces slightly more state complexity (job queue).
