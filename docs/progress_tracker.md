# Joplin Sync Server: Progress Tracker

This document tracks the progress of the custom Joplin Sync Server implementation based on the `foundation_plan.md`.

## ✅ Completed Slices

### [x] Slice 0: Tech Stack & Storage Strategy Selection
- [x] Compare language options, web frameworks, and storage driver options.
- [x] Evaluate Database Engine (SQLite vs PostgreSQL).
- [x] Create Architectural Decision Record (`adr_tech_stack.md`).

### [x] Slice 0.5: Architecture & Design Principles
- [x] Establish Service/Repository pattern.
- [x] Define Dependency Injection strategies.
- [x] Outline testing strategy and TDD workflow.
- [x] Ensure forward-compatibility for Phase 2 (Admin UI, RBAC, Multi-User Sharing).
- [x] Create Architectural Decision Record (`adr_architecture_patterns.md`).

### [x] Slice 1: Project Setup, DB Schema & Environment
- [x] Initialize project and dependency management.
- [x] Implement configuration loader (env vars / YAML) and logging.
- [x] Setup DB connection pool & migration runner for the 7 essential tables (`users`, `sessions`, `storages`, `items`, `user_items`, `changes_2`, `key_values`).
- [x] Create seed CLI command/utility to initialize an admin user.
- [x] Implement `GET /api/ping` health check endpoint.
- [x] Write unit/integration tests for setup and health check.

### [x] Slice 2: Authentication & Session Management
- [x] Design auth service and session token generator.
- [x] Implement `POST /api/sessions` (Login).
- [x] Implement `DELETE /api/sessions/:id` (Logout).
- [x] Implement `AuthMiddleware` to validate `X-API-AUTH` header.
- [x] Write automated tests for login, logout, and token authorization.

### [x] Slice 3: Concurrency Lock Management Engine (`/api/locks`)
- [x] Design lock manager using `key_values` table / memory fallback.
- [x] Implement `POST /api/locks` (Acquire Sync and Exclusive locks).
- [x] Implement `DELETE /api/locks/:id` (Release lock).
- [x] Implement `GET /api/locks` (List active locks).
- [x] Implement lock expiration / TTL check.
- [x] Write integration tests for acquiring and releasing locks.

### [x] Slice 4: Item Storage Engine & Core CRUD API (`/api/items`)
- [x] Implement Joplin URL path parser for `root:/<path>:` syntax.
- [x] Implement Storage Driver (DB BLOB or Local Disk based on ADR).
- [x] Implement `GET /api/items/root:/<path>:` (Get item stat metadata).
- [x] Implement `GET /api/items/root:/<path>:/content` (Get raw item content).
- [x] Implement `PUT /api/items/root:/<path>:/content` (Create/Update item).
- [x] Implement `DELETE /api/items/root:/<path>:` (Delete item).
- [x] Write tests verifying CRUD operations.

### [x] Slice 5: Change Event Log & Delta Sync Engine (`changes_2`)
- [x] Implement change tracking hook on item create, update, and delete.
- [x] Log events (Create=1, Update=2, Delete=3) in `changes_2` with counter.
- [x] Implement `GET /api/items/root:/<path>:/delta` with cursor pagination.
- [x] Write integration tests for delta sync and cursor progression.

---

### [x] Slice 6: Batch Operations & Directory Listing (`/api/batch_items`)
- [x] Implement `PUT /api/batch_items` (Batch insert/update in single transaction).
- [x] Implement `DELETE /api/batch_items` (Batch delete in single transaction).
- [x] Integrate batch operations with `changes_2` event logging.
- [x] Implement `GET /api/items/root:/<path>/*:/children` for directory listing.
- [x] Write tests verifying batch operations performance and correctness.

### [x] Slice 6.5: Observability & Structured Logging
- [x] Setup Go 1.21+ `log/slog` with a JSON handler globally.
- [x] Implement an HTTP request logger middleware.
- [x] Redact sensitive headers such as `Authorization` / `X-API-AUTH`.
- [x] Replace all standard library `log` calls with `slog` calls.

### [x] Slice 7: E2E Client Verification, Encryption & Deployment
- [x] Create a production Dockerfile and `docker-compose.yml` for lightweight hosting.
- [x] Configure and test an official Joplin client against our running custom sync server.
- [x] Verify initial sync, delta sync, and E2EE note sync.
- [x] Fix any edge-case protocol mismatches or header issues discovered during client testing.
- [x] Finalize user documentation and server administration instructions.

### [x] Slice 7.1: Test Coverage Improvement
- [x] Implement E2E tests to cover `main.go`, `config.go`, and `middleware.go` via a complete client flow.
- [x] Add integration test edge-cases to cover error paths in API handlers (e.g. `handleGetContent`, `Logout`, `handlePutBatch`).
- [x] Improve database and storage driver coverage to handle edge cases like pagination and missing keys.
- [x] Reach high overall code coverage (~80%) using exclusively integration/E2E tests (90%+ was found impractical without mock databases for SQLite transaction/driver-level failure paths).

---

### [x] Slice 8: Phase 2 Discovery - Admin Management & Multi-User Support
- [x] Interactively design technical approach for Admin UI (SSR vs SPA vs Microservice).
- [x] Design Role-Based Access Control (RBAC) strategy.
- [x] Architect data model extensions for multi-user notebook sharing.
- [x] Create Architectural Decision Record (`adr_phase2_admin_sharing.md`).

### [x] Slice 8.1: Phase 2 Implementation - Admin & RBAC
- [x] Implement Go Server-Side Rendered (SSR) Admin UI (`html/template` + HTMX).
- [x] Implement "Zero-User Onboarding Flow".
- [x] Implement `AdminMiddleware` utilizing `is_admin` flag.
- [x] Implement foundational schema changes for Multi-User Sharing (Fan-out).

---

## 🚀 Current Focus

### [ ] Slice 8.2: Phase 2 Implementation - Admin Dashboard (User Management & Statistics)
- [ ] Implement user list view in the admin dashboard.
- [ ] Implement user creation and deletion functionality for the admin.
- [ ] Implement basic instance statistics on the admin dashboard (e.g., total items, total active sessions).
- [ ] Ensure HTMX is utilized for dynamic interactions without full page reloads.

---

## ⏳ Upcoming Slices

### [ ] Slice 9: CI/CD & Image Building
- [ ] Implement a `.github/workflows/docker-publish.yml` file.
- [ ] Trigger workflow on Git version tags (e.g., `v*.*.*`).
- [ ] Build and push Docker image with both the version tag and `:latest` tag.
- [ ] Verify pushing a version tag successfully triggers the image build.

### [ ] Slice 10: Documentation Cleanup & Self-Hosted Guide
- [ ] Migrate all planning and ADR documents into a slimmed `README.md` and `CLAUDE.md`.
- [ ] Rewrite `README.md` to focus on stack, how to run/dev/test, and 100% Joplin compatibility (skip motivations).
- [ ] Create a self-hosted guide covering Docker setup and requirements.
- [ ] Include setup for bind mounts or volumes.
- [ ] Document recommendations for reverse proxy vs exposing ports.
- [ ] Document recommendations for backing up the database and files.
