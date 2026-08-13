# Architectural Drift Report

## Overview
This report outlines the architectural drift identified in the custom Joplin Sync Server implementation compared to the original design documents and foundation plan. The audit compared the codebase against the expected outcomes of each "Slice" defined in the planning phase.

Overall, the application functions and correctly implements the Joplin Server API endpoints, but it has diverged from the structural and data storage guidelines defined in the Architectural Decision Records (ADRs).

---

## 1. Project Structure & Layered Architecture
**Slice:** Slice 0.5 (Architecture & Design Principles)

**Expected (Design Docs):** 
According to `adr_architecture_patterns.md`, the project was supposed to use a **Layered Architecture** with strict Separation of Concerns. The structure should have included:
- `cmd/server/main.go`: Application entry point and Dependency Injection root.
- `internal/repositories/`: Data access layer (abstracted via interfaces).
- `internal/services/`: Core business logic.
- `internal/http/`: HTTP transport layer (handlers & middleware).

**Actual Implementation:**
The codebase has a flat, simplified structure at the root:
- `main.go` is in the root directory.
- `api/` contains the HTTP handlers directly.
- `db/` contains the `sqlc` generated database access code.
- There are no `repositories` or `services` layers. The HTTP handlers (e.g., `api.ItemHandler`) inject the `sqlc` `db.Queries` struct directly and contain all the business logic, SQL orchestration, and HTTP formatting.

**Resolution Plan & Status:**
Based on a review of Go idioms, the current flat structure is accepted as the "working as intended" architecture. 
- *Status: Resolved (Accepted Drift).* The ADR `adr_architecture_patterns.md` has been updated to reflect the new "Idiomatic Go Flat Architecture", keeping the handlers working directly with `sqlc` models.

---

## 2. Storage Backend (Item Payload)
**Slice:** Slice 0 (Tech Stack) & Slice 4 (Item Storage Engine)

**Expected (Design Docs):**
According to `adr_tech_stack.md`, the Storage Backend for item payloads (Markdown text, images, PDFs) was explicitly decided to be the **Local Filesystem**. The SQLite database was intended only for structured metadata to keep it small, fast, and nimble.

**Actual Implementation:**
The application was originally storing raw file contents directly inside the SQLite database as BLOBs. 

**Resolution Plan & Status:**
1. Created a `storage` package (`storage/local_fs.go`) that handles reading, writing, and deleting files on the local disk.
2. Updated the HTTP handlers to save file payloads to the local filesystem using the new storage driver, and only save metadata to the database.
3. Modified the database schema to drop the `content` and `content_size` columns from the `items` table.
- *Status: Resolved.*

---

## 3. Testing Structure
**Slice:** Slice 0.5 (Architecture & Design Principles)

**Expected (Design Docs):**
The `adr_architecture_patterns.md` proposed a distinct `tests/` directory containing `unit/`, `integration/`, and `e2e/` testing suites.

**Actual Implementation:**
Tests are colocated with the packages they test (e.g., `api/items_test.go`). 

**Resolution Plan & Status:**
Like the project structure, colocated tests that utilize the real (in-memory) database rather than mock interfaces are idiomatic in Go, especially when using `sqlc`. We embrace this pattern for the project.
- *Status: Resolved (Accepted Drift).*
