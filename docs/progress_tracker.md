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

---

## 🚀 Current Focus

### [ ] Slice 7: E2E Client Verification, Encryption & Deployment
- [ ] Configure and test official Joplin client against the custom sync server.
- [ ] Verify initial sync, delta sync, and E2EE note sync.
- [ ] Create Dockerfile and `docker-compose.yml`.
- [ ] Finalize user documentation and server administration instructions.

---

## ⏳ Upcoming Slices

### [ ] Slice 8: Phase 2 Discovery - Admin Management & Multi-User Support
- [ ] Interactively design technical approach for Admin UI (SSR vs SPA vs Microservice).
- [ ] Design Role-Based Access Control (RBAC) strategy.
- [ ] Architect data model extensions for multi-user notebook sharing.
- [ ] Create Architectural Decision Record (`adr_phase2_admin_sharing.md`).

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
