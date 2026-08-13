# AI Assistant Context & Guidelines

## Tech Stack & Storage
- **Language & Framework**: Go 1.21+ using Standard Library (`net/http`) and `sqlc` for type-safe database queries.
- **Database Engine**: SQLite with Write-Ahead Logging (WAL) enabled (`PRAGMA journal_mode=WAL;`).
- **Storage Backend**: Local Filesystem for item payloads. Metadata in SQLite, raw content as files.
- **Admin UI**: Go Server-Side Rendering (`html/template`) + HTMX, served under `/admin`.

## Architecture Patterns
- **Service/Repository Pattern**: Strict separation between data access (Repositories) and business logic (Services).
- **Dependency Injection**: Manual Constructor Injection at the application root (`main.go`). No magic IoC containers.
- **Project Structure**: Flat, domain-oriented structure at the root level (e.g., `api/`, `db/`, `models/`, `storage/`).

## Testing Strategy
- **Unit Testing**: Services tested in isolation using mocked repository interfaces.
- **Integration Testing**: Repositories tested against an in-memory SQLite database (`:memory:`).
- **E2E Testing**: HTTP endpoints tested directly via `net/http/httptest`.

## Database Schema & Naming Conventions
The database schema has been modernized from the legacy Joplin Server implementation:
- **Tables**: `users`, `sessions`, `sync_items`, `user_sync_items`, `delta_events` (formerly `changes_2`), `sync_locks` (formerly `key_values`), `shares`, `user_shares`.
- **Columns**: Uses idiomatic names like `id`, `created_at`, `updated_at`, `joplin_id`.
- **Compatibility**: The Go codebase maps these modernized internal names back to the original JSON structures expected by Joplin clients using explicit struct tags (e.g., `json:"updated_time"`).

## Key Design Decisions
- **Role-Based Access Control**: Simple `is_admin` boolean flag on the `users` table.
- **Multi-User Sharing**: "Fan-out" write strategy for the Delta Sync Log (`delta_events`). When a shared notebook item is modified, a distinct event is written for every participating user, keeping `GET /delta` reads fast.
- **User Deletion**: Synchronous deletion strategy. Relies on SQLite `ON DELETE CASCADE` to wipe metadata and a synchronous `os.RemoveAll()` on the `local_fs` storage driver to wipe files, avoiding complex background job queues.
