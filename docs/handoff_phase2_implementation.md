We are continuing the custom Joplin Sync Server project. Please refer to @joplin_server_deep_dive.md and our latest architectural decisions in @adr_phase2_admin_sharing.md.

In the previous session, we completed Slice 8.1, which established the Go Server-Side Rendered (SSR) Admin UI shell, the Zero-User Onboarding Flow, and the foundational schema for Multi-User Sharing. 

Our goal for this session is to implement **Slice 8.2: Phase 2 Implementation - Admin Dashboard (User Management & Statistics)**:

1. **User Management**: Add a user list view to the admin dashboard (`/admin`). Implement functionality for the administrator to create new regular users and delete existing ones.
2. **Instance Statistics**: Display basic statistics on the dashboard, such as the total number of users, total number of synchronized items, and the total number of active sessions.
3. **HTMX Integration**: Utilize HTMX to make the user creation and deletion operations dynamic, without requiring full page reloads. The base `layout.html` already includes the HTMX script.
4. **Backend Implementation**: Create the necessary handler functions in `api/admin.go`, add or modify templates in `api/templates/`, and write any necessary SQL queries in `db/queries.sql` to fetch statistics.

Please review the progress tracker (`@progress_tracker.md`), explore the current Go codebase (specifically `api/admin.go` and `db/queries.sql`), and present a plan to implement these features.

*Note: At the end of the slice, please remember to run tests, commit your work, and update the `@progress_tracker.md` to mark this slice as complete and queue up Slice 9.*
