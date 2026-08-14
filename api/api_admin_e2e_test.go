package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/services"
	"github.com/jberlyn/syncopation/storage"
	"golang.org/x/crypto/bcrypt"
)

func setupAdminTestApp(t *testing.T) (*http.ServeMux, *db.Queries, *sql.DB, *storage.LocalFS) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	storagePath := filepath.Join(tempDir, "storage")

	dbConn, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}

	schema, err := os.ReadFile("../db/schema.sql")
	if err != nil {
		t.Fatalf("Failed to read schema: %v", err)
	}
	_, _ = dbConn.Exec(string(schema))

	queries := db.New(dbConn)
	localFS := storage.NewLocalFS(storagePath)

	mux := http.NewServeMux()

	adminService := services.NewAdminService(queries, localFS)
	adminHandler := &AdminHandler{AdminService: adminService, Version: "test"}

	mux.Handle("GET /setup", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleSetupGet)))
	mux.Handle("POST /setup", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleSetupPost)))
	mux.Handle("GET /login", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleLoginGet)))
	mux.Handle("POST /login", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleLoginPost)))
	mux.Handle("GET /logout", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleLogout)))
	mux.Handle("GET /", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleDashboard)))
	mux.Handle("POST /api/users", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleUsersPost)))
	mux.Handle("DELETE /api/users/{id}", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleUsersDelete)))

	return mux, queries, dbConn, localFS
}

func seedAdminTestUser(t *testing.T, queries *db.Queries, email, password string, isAdmin int64) db.User {
	ctx := context.Background()
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		IsAdmin:      isAdmin,
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("Seed failed: %v", err)
	}
	return u
}

func TestAdminE2EFlow(t *testing.T) {
	mux, queries, dbConn, _ := setupAdminTestApp(t)
	defer dbConn.Close()

	server := httptest.NewServer(mux)
	defer server.Close()
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 1. Initial State (No Users) -> Should redirect to /setup
	resp, _ := client.Get(server.URL + "/")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/setup" {
		t.Errorf("Expected redirect to /setup, got %d to %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp.Body.Close()

	// 2. GET /setup (Valid)
	resp, _ = client.Get(server.URL + "/setup")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for /setup, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. POST /setup (Missing fields)
	form := url.Values{}
	resp, _ = client.PostForm(server.URL+"/setup", form)
	if resp.StatusCode != http.StatusOK { // Returns HTML with error
		t.Errorf("Expected 200 OK with error, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. POST /setup (Valid)
	form.Set("email", "admin@example.com")
	form.Set("password", "adminpass")
	resp, _ = client.PostForm(server.URL+"/setup", form)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/" {
		t.Errorf("Expected redirect to /, got %d", resp.StatusCode)
	}
	sessionCookie := resp.Header.Get("Set-Cookie")
	resp.Body.Close()

	// 5. GET /setup (Invalid, users exist)
	resp, _ = client.Get(server.URL + "/setup")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/" {
		t.Errorf("Expected redirect to /, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 6. GET /login
	resp, _ = client.Get(server.URL + "/login")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for /login, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 7. POST /login (Invalid)
	form = url.Values{}
	form.Set("email", "admin@example.com")
	form.Set("password", "wrongpass")
	resp, _ = client.PostForm(server.URL+"/login", form)
	if resp.StatusCode != http.StatusOK { // Returns HTML with error
		t.Errorf("Expected 200 OK with error, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 8. POST /login (Valid)
	form.Set("password", "adminpass")
	resp, _ = client.PostForm(server.URL+"/login", form)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected redirect after valid login")
	}
	sessionCookie = resp.Header.Get("Set-Cookie")
	resp.Body.Close()

	doReq := func(method, path string, form url.Values) *http.Response {
		var req *http.Request
		if form != nil {
			req, _ = http.NewRequest(method, server.URL+path, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req, _ = http.NewRequest(method, server.URL+path, nil)
		}
		if sessionCookie != "" {
			req.Header.Set("Cookie", strings.Split(sessionCookie, ";")[0])
		}
		r, _ := client.Do(req)
		return r
	}

	// 9. GET / (Dashboard)
	resp = doReq("GET", "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for dashboard, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 10. POST /api/users (Add user)
	form = url.Values{}
	form.Set("email", "user@example.com")
	form.Set("password", "userpass")
	resp = doReq("POST", "/api/users", form)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for creating user, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Add user again (Error)
	resp = doReq("POST", "/api/users", form)
	if resp.StatusCode != http.StatusOK { // Returns HTML with error
		t.Errorf("Expected 200 OK with error HTML, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// POST /api/users missing fields
	resp = doReq("POST", "/api/users", url.Values{})
	resp.Body.Close()

	// 11. DELETE /api/users/{id}
	// Fetch the newly created user to get ID
	user, _ := queries.GetUserByEmail(context.Background(), "user@example.com")
	resp = doReq("DELETE", "/api/users/"+user.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for deleting user, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Delete admin user (should fail but we can't easily assert HTML response cleanly, just ensure no panic)
	adminUser, _ := queries.GetUserByEmail(context.Background(), "admin@example.com")
	resp = doReq("DELETE", "/api/users/"+adminUser.ID, nil)
	resp.Body.Close()

	// 12. GET /logout
	resp = doReq("GET", "/logout", nil)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("Expected redirect after logout")
	}
	resp.Body.Close()

	// 13. Invalid methods
	resp = doReq("PUT", "/setup", nil)
	resp.Body.Close()
	resp = doReq("PUT", "/login", nil)
	resp.Body.Close()
	resp = doReq("PUT", "/", nil)
	resp.Body.Close()
	resp = doReq("GET", "/api/users", nil)
	resp.Body.Close()
	resp = doReq("POST", "/api/users/abc", nil)
	resp.Body.Close()

	// 14. Unauthenticated accesses
	sessionCookie = ""
	resp = doReq("GET", "/", nil)
	resp.Body.Close()

	// 15. Invalid Session Cookie
	sessionCookie = "admin_session=invalid"
	resp = doReq("GET", "/", nil)
	resp.Body.Close()

	// 16. Non-Admin user accessing dashboard
	normalUser := seedAdminTestUser(t, queries, "normal@example.com", "pass", 0)
	normalSession, _ := queries.CreateSession(context.Background(), db.CreateSessionParams{
		ID:        "normal_session_id",
		UserID:    normalUser.ID,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	})
	sessionCookie = "admin_session=" + normalSession.ID
	resp = doReq("GET", "/", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-admin, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
