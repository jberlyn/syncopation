# Architectural Decision Record (ADR): Architecture Patterns & Design Principles

## Status
Accepted

## Context
As we begin implementing the custom Joplin Sync Server (Slice 0.5), we need to establish the core architectural patterns, project structure, and testing strategies. These decisions will ensure the codebase remains maintainable, testable, and loosely coupled as we progress through the subsequent development slices.

## Decisions

### 1. Service/Repository Pattern
- **Decision:** We will strictly separate concerns using the Service/Repository pattern.
  - **Repositories:** Responsible for all data access logic (SQLite via `database/sql` & `sqlc`, and Filesystem via storage drivers). They hide the underlying storage mechanisms from the rest of the application.
  - **Services:** Contain the core business and domain logic (e.g., delta sync calculations, authentication rules, lock management). Services will orchestrate data flow by calling repositories.
  - **HTTP Handlers:** Responsible only for HTTP-level concerns: parsing requests, validating input (e.g., Joplin's virtual paths), invoking the appropriate service, and formatting HTTP responses using `net/http`.
- **Rationale:** This separation ensures business logic is agnostic of the database and HTTP transport, making it highly testable and easier to modify.

### 2. Dependency Injection (DI)
- **Decision:** Manual Constructor Injection.
- **Rationale:** Given our stack (Go), a lightweight and explicit approach is heavily favored idiomatically. Services will accept their repository dependencies (via interfaces) in constructor functions (e.g., `NewItemService`). At application startup (the Composition Root in `cmd/server/main.go`), we will manually instantiate the repositories and inject them into the services, which are then passed to the HTTP route handlers. This avoids the overhead and "magic" of an IoC container while preserving full testability.

### 3. Testability and Test-Driven Development (TDD)
- **Decision:** We will use Go's built-in `testing` package.
- **Rationale:** The standard library `testing` package is zero-configuration and ubiquitous in the Go ecosystem.
  - **Unit Testing (Services):** We will test business logic in isolation by injecting mock implementations of our repository interfaces into our services.
  - **Integration Testing (Repositories):** We will test database queries against a real SQLite database running in memory (`:memory:`) to ensure SQL correctness without touching the disk.
  - **End-to-End Testing (E2E):** We will test our HTTP endpoints directly using Go's built-in `net/http/httptest` package to simulate HTTP requests against our router without starting a real TCP server.

### 4. Project Structure (Idiomatic Go Flat Architecture)
- **Decision:** The project will be organized using a flat, domain-oriented structure at the root, typical of small-to-medium Go applications.
- **Rationale:** While originally planned to use nested layered architecture (`internal/repositories`, `internal/services`, `internal/http`), this is often considered over-engineering in Go. Instead, we embrace simplicity: HTTP handlers directly utilize `sqlc` generated code (`db.Queries`) for database interaction. Interfaces are introduced only when polymorphism is strictly needed (e.g., swapping out the Storage Engine).

#### Directory Layout:
```text
joplin-sync/
├── api/             # HTTP route handlers (AuthHandler, ItemHandler)
├── cmd/             # Obsolete/Empty - we keep main.go at root
├── db/              # sqlc generated code, schema definitions, and queries
├── docs/            # Architecture decision records and plans
├── models/          # Shared domain models
├── storage/         # Storage driver implementations (Local FS)
├── main.go          # Application entry point and Dependency Injection root
└── ...
```
