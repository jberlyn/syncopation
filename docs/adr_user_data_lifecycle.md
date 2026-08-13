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

After reviewing the requirements and our specific architecture, we've decided to diverge from the official server's heavy-duty background queue approach in favor of a simpler, synchronous deletion strategy. 

Because our custom `StorageDriver` (specifically `local_fs`) already groups physical files into a root directory based on `userID`, and because SQLite handles cascading deletes very efficiently, we can accomplish user deletion in a single, fast operation.

### 1. Schema Changes (Foreign Keys)
We will update our database schema and migrations to include strict `ON DELETE CASCADE` foreign keys linking back to `users(id)`. This will be applied to:
- `sessions.user_id`
- `items.owner_id`
- `user_items.user_id`
- `changes_2.user_id`
- `shares.owner_id`
- `user_shares.user_id`

### 2. Admin Deletion API (`DELETE /admin/users/:id`)
When the Admin deletes a user, the API handler will simply do two things sequentially:

1. **Delete from Database**: Execute `DELETE FROM users WHERE id = ?`. Due to the cascading foreign keys, SQLite will instantly purge all associated items, changes, shares, and sessions in a single transaction.
2. **Delete from Disk**: Call a new `DeleteUser(userID)` method on the `StorageDriver` interface. For our `local_fs` driver, this will execute an `os.RemoveAll()` on the user's root folder, completely removing all physical files instantly.

## Consequences

- **Pros**: Drastically simpler architecture. No need for background goroutines, polling, state management flags, or a `user_deletions` table. Deletion is instantaneous and clean.
- **Cons**: A synchronous deletion might block the HTTP thread for a few milliseconds longer than a soft-delete, but for the scale of personal/small-team servers, this is completely negligible. It technically diverges from the official implementation's internal flow, but achieves the exact same functional outcome.
