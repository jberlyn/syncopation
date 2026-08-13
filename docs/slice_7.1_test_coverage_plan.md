# Slice 7.1: Test Coverage Improvement

## Overview
The goal of this slice is to improve the test coverage of the Joplin Sync Server to 90%+. 
We want to rely primarily on integration tests and introduce end-to-end (E2E) tests that cover all API endpoints without creating a heavy maintenance burden from overlapping tests. Good coverage ensures we can make changes confidently without breaking existing functionality.

## Current Coverage Analysis
Current overall statement coverage is roughly **62.7%** (when measuring across all packages).

### Key Gaps Identified:
1. **Uncovered Files (0% Coverage)**:
   - `config/config.go`: The `LoadConfig` function is not tested (environment variable parsing and defaults).
   - `api/middleware.go`: `LoggingMiddleware` and `WriteHeader` are completely untested.
   - `main.go`: The `main` function (server wiring, DB init, route setup) is untested.

2. **Partial Coverage (50-80%)**:
   - `api/items.go`: Edge cases and error handling in `handleGetContent` (46.2%), `handleDelete` (50%), and `HandleItems` (64%).
   - `api/batch_items.go`: Error paths in `handlePutBatch` (56.5%) and `HandleBatchItems` (62.5%).
   - `api/auth.go`: Error paths in `Logout` (55.6%).
   - `api/locks.go`: Error paths in `ReleaseLock` (60%).
   - `db/queries.sql.go`: Database edge cases such as `GetKeyValue` (0%), `GetChangesByUser` (73%), `ListItemsByUser` (73%), and `ListKeyValuesByType` (73%).
   - `storage/local_fs.go`: Storage write errors and missing file deletions.

## Execution Plan for New Session (Handoff Prompt)

Hello! Your task is to implement "Slice 7.1: Test Coverage Improvement" for this repository. 

**Requirements:**
1. **End-to-End (E2E) Test Suite**: 
   - Create a new `e2e_test.go` (or similar).
   - Spin up the application as closely to real as possible (e.g., using `httptest.Server` over the router from `main.go` or directly running the server), with a real test SQLite DB and a temporary file system for storage.
   - Write a cohesive E2E test that simulates a complete client flow: load config -> login to get a session token -> acquire locks -> sync items -> batch operations -> delete items -> release locks -> logout.
   - This single overarching test flow should organically hit `main.go` wiring, `config.go`, `middleware.go`, and the happy paths of all API endpoints without duplicating granular tests unnecessarily.

2. **Improve Existing Integration Tests**:
   - Go into the existing tests (`api/*_test.go`) and add test cases for the missing error states (e.g., bad JSON payloads, invalid paths, missing files, simulated DB errors if possible).
   - Ensure the query edge cases in `db/queries.sql.go` and `storage/local_fs.go` (e.g., pagination cursors, missing keys) are covered either in the `api` integration tests or dedicated integration tests for those packages.

3. **Goal**: 
   - Achieve 90%+ overall code coverage.
   - Run `go test -coverprofile=c.out -coverpkg=./... ./... && go tool cover -func=c.out` frequently to track your progress and identify the remaining gaps.
   - Do NOT write mock-based unit tests. We prefer exclusively integration/E2E tests using real database and storage instances.
   - Remember to automatically commit your changes as you complete logical chunks. Ensure that coverage reports are not comitted to source control.
