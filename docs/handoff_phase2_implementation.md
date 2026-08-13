We are continuing the custom Joplin Sync Server project. Please refer to @joplin_server_deep_dive.md and our latest architectural decisions in @adr_phase2_admin_sharing.md.

Our goal for this session is to begin the implementation of Phase 2 (Admin Management & Multi-User Sharing), executing against the decisions we locked in during the discovery phase:

1. Implement the Go Server-Side Rendered (SSR) Admin UI using `html/template` and HTMX. Serve this UI under the `/admin` route using `go:embed`.
2. Implement the "Zero-User Onboarding Flow" for the `/admin` route: if `SELECT COUNT(*) FROM users` is 0, display a setup screen to create the initial admin account.
3. Implement an `AdminMiddleware` utilizing the `is_admin` boolean flag on the `users` table to restrict access to the `/admin` routes. Regular users should not have access to these routes.
4. Begin the foundational schema changes to support the Multi-User Sharing Fan-out write strategy for `changes_2` (e.g., adding a `shares` and `user_shares` table so we know who to fan out to when an item in a shared folder is edited).
Let's tackle this step-by-step. Please review the ADR, explore the current Go codebase, and present a plan for the first step.

*Note: At the end of the slice, please remember to update the `@progress_tracker.md` to mark this slice as complete and queue up the next one.*
