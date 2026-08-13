We are continuing the custom Joplin Sync Server project. Please refer to `docs/joplin_server_deep_dive.md` and our latest architectural decisions in `docs/adr_phase2_admin_sharing.md`.

In the previous sessions, we completed Slice 8.1, establishing the Go Server-Side Rendered (SSR) Admin UI shell, the Zero-User Onboarding Flow, and the foundational schema for Multi-User Sharing. We also simplified the database schema by completely removing the `full_name` column from the `users` table, standardizing on only requiring `email` and `password` for accounts.

Our goal for this session is to implement **Slice 8.2: Phase 2 Implementation - Admin Dashboard (User Management & Statistics)**:

1. **User Management**: Add a user list view to the admin dashboard (`/admin`). Implement functionality for the administrator to create new regular users (requiring only email and password) and delete existing ones.
2. **Instance Statistics**: Display basic statistics on the dashboard, such as the total number of users and total number of synchronized items. Use a SQL `JOIN` or similar to show how many items each individual user has.
3. **HTMX Integration**: Utilize HTMX to make the user creation and deletion operations dynamic, without requiring full page reloads. The base `layout.html` already includes the HTMX script. Keep the UI simple and clean.
4. **Backend Implementation**: Create the necessary handler functions in `api/admin.go`, add or modify templates in `api/templates/`, and write any necessary SQL queries in `db/queries.sql` to fetch statistics. Remember to run `sqlc generate` if you add new queries.

Please review the progress tracker (`docs/progress_tracker.md`), explore the current Go codebase (specifically `api/admin.go` and `db/queries.sql`), and begin implementing these features.

*Note: At the end of the slice, please remember to run tests, commit your work, and update the `docs/progress_tracker.md` to mark this slice as complete and queue up Slice 9.*
