# Architectural Decision Record (ADR): Technology Stack & Storage

## Status
Accepted

## Context
We are building a custom, lightweight Joplin Sync Server. The goal is to create an open-source project that is easy to self-host, primarily distributed as a Docker container. We need to decide on the core programming language, web framework, database engine, and storage backend for the item payloads.

## Decisions

### 1. Programming Language & Framework
- **Decision:** Go + Standard Library (`net/http`) + sqlc (Query Builder/Type Generator)
- **Rationale:** Go provides exceptional performance, minimal memory footprint, and compiles to a single static binary, making it perfect for a lightweight self-hosted sync server. By relying heavily on the standard library (`net/http` for routing in Go 1.22+) and `sqlc` for type-safe database access, we minimize external dependencies. *Note: As the author has a strong background in TypeScript/Bun, Agy will assist by relating Go concepts back to TS/Node idioms during development.*

### 2. Database Engine
- **Decision:** SQLite
- **Rationale:** To make self-hosting as frictionless as possible, a single-container deployment is optimal. SQLite eliminates the need for users to run and manage a separate database service (like PostgreSQL).
- **Concurrency Strategy:** We will enable Write-Ahead Logging (WAL mode) (`PRAGMA journal_mode=WAL;`) to allow concurrent reads and writes. This ensures that multi-user households (e.g., multiple clients syncing simultaneously) do not encounter database lock contention. Go's `mattn/go-sqlite3` (or equivalent cgo-free SQLite driver) is well-suited for this use case.

### 3. Storage Backend (Item Payload)
- **Decision:** Local Filesystem
- **Rationale:** Joplin syncs two types of data: structured metadata and raw content payloads (Markdown text, images, PDFs). Storing the raw content as plain files on the local disk keeps the SQLite database small, fast, and nimble. Users will simply bind-mount a data directory to their Docker container, and the OS filesystem will efficiently handle the reading and writing of large binary attachments.
