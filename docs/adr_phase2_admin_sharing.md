# Architectural Decision Record (ADR): Phase 2 - Admin Management & Multi-User Sharing

## Status
Accepted

## Context
We are starting Phase 2 of the custom Joplin Sync Server, which focuses on adding server administration capabilities and multi-user collaboration (notebook sharing). We need to determine the architectural patterns for the Admin UI, how to secure those admin routes, and how to model data changes when multiple users are modifying shared notebooks. The overarching goal is to maintain the lightweight, performant, single-container philosophy established in Phase 1.

## Decisions

### 1. Admin UI Technical Approach
- **Decision:** Go Server-Side Rendering (SSR) (`html/template`) + HTMX.
- **Rationale:** Building a separate Single Page Application (React/Vue) would require introducing Node.js and a build step into the Dockerfile, contradicting our goal of a simple, fast-compiling Go application. By using Go's built-in `html/template` and HTMX, we can deliver a dynamic, interactive UI that compiles directly into the single Go binary using `go:embed`.
- **Key Features:**
  - The UI will be served under the `/admin` route.
  - It will provide basic functionality: login, user management (CRUD), and basic instance statistics.
  - **Zero-User Onboarding Flow:** If the server starts and the `users` table is completely empty, visiting `/admin` will present a special onboarding screen to create the initial administrative account, bypassing the need for a CLI seed script.

### 2. Role-Based Access Control (RBAC)
- **Decision:** Simple Boolean Flag (`is_admin`).
- **Rationale:** We only require two distinct access levels: Server Administrators (who can access `/admin`) and Sync Users (who can only access the Joplin client API for syncing). A full Roles table/enum is unnecessary overhead. The existing `is_admin` column in the `users` table will be utilized by a new `AdminMiddleware` to secure the `/admin` routes. Regular users will not have access to the web UI.

### 3. Multi-User Notebook Sharing Architecture
- **Decision:** "Fan-out" write strategy for the Delta Sync Log (`changes_2`).
- **Rationale:** When an item inside a shared notebook is modified, the server needs to notify all users who have access to that notebook during their next sync. Instead of running complex, dynamic permission-checking `JOIN` queries during every `GET /delta` sync request (which happens constantly), we will "fan-out" the writes. 
  - When an edit occurs in a shared folder, the server will look up all participating users and write a distinct change event row into `changes_2` for *every* user.
  - This slightly increases database storage usage on writes but ensures that the highly-frequent Delta Sync read operations remain lightning-fast and computationally inexpensive, matching our core goal of maximum performance.
