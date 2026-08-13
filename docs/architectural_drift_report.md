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

**Resolution Plan:**
To realign with the design, refactor the project structure:
1. Move `main.go` to `cmd/server/main.go`.
2. Move all business logic out of `api/` handlers into a new `internal/services/` layer.
3. Define interfaces in `internal/repositories/` and wrap the `sqlc` database layer so that services are decoupled from the database implementation.
4. Move the HTTP transport logic into `internal/http/`.

---

## 2. Storage Backend (Item Payload)
**Slice:** Slice 0 (Tech Stack) & Slice 4 (Item Storage Engine)

**Expected (Design Docs):**
According to `adr_tech_stack.md`, the Storage Backend for item payloads (Markdown text, images, PDFs) was explicitly decided to be the **Local Filesystem**. The SQLite database was intended only for structured metadata to keep it small, fast, and nimble.

**Actual Implementation:**
The application stores the raw file contents directly inside the SQLite database as BLOBs. 
- `db/schema.sql` defines the `items` table with a `content BLOB` column.
- `api/items.go` (e.g., in `handlePutContent`) reads the raw HTTP request body and passes the entire payload into the SQLite `UpsertItem` query, saving it to the `content` BLOB column. 

**Resolution Plan:**
1. Create a `storage` package (e.g., `internal/storage/local_fs.go`) that handles reading, writing, and deleting files on the local disk.
2. Update the `ItemService` (once extracted) to save file payloads to the local filesystem using the new storage driver, and only save the metadata (`name`, `updated_time`, etc.) to the database.
3. Create a database migration to drop the `content` and `content_size` columns from the `items` table, and migrate any existing BLOB data to the filesystem.

---

## 3. Testing Structure
**Slice:** Slice 0.5 (Architecture & Design Principles)

**Expected (Design Docs):**
The `adr_architecture_patterns.md` proposed a distinct `tests/` directory containing `unit/`, `integration/`, and `e2e/` testing suites. Unit tests would isolate business logic using mocked repositories.

**Actual Implementation:**
Tests are colocated with the packages they test (e.g., `main_test.go`, `api/items_test.go`, `api/auth_test.go`). Because there is no Service/Repository separation or interfaces, the tests are likely integration tests that touch the database, rather than isolated unit tests with mocks.

**Resolution Plan:**
1. After extracting interfaces for repositories and services (as part of Drift #1), write isolated unit tests using mocks for the business logic.
2. Reorganize the test files into the proposed `tests/unit`, `tests/integration`, and `tests/e2e` directories to cleanly separate fast unit tests from slower database/HTTP integration tests.
