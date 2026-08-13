package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/joplin-sync/api"
	"github.com/jberlyn/joplin-sync/db"
	"golang.org/x/crypto/bcrypt"
)

func TestAdminUI(t *testing.T) {
	queries := setupTestDB(t)

	adminHandler := &api.AdminHandler{Queries: queries}

	// Setup mux
	mux := http.NewServeMux()
	mux.Handle("GET /admin/setup", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleSetupGet)))
	mux.Handle("POST /admin/setup", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleSetupPost)))
	mux.Handle("GET /admin/login", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleLoginGet)))
	mux.Handle("POST /admin/login", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleLoginPost)))
	mux.Handle("GET /admin/logout", http.HandlerFunc(adminHandler.HandleLogout))
	mux.Handle("GET /admin", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleDashboard)))

	// 1. Zero users - access /admin should redirect to /admin/setup
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/admin/setup" {
		t.Fatalf("Expected redirect to /admin/setup, got %d %s", w.Code, w.Header().Get("Location"))
	}

	// 2. Submit setup form
	form := url.Values{}
	form.Add("email", "admin@example.com")
	form.Add("password", "password123")

	req = httptest.NewRequest(http.MethodPost, "/admin/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound || w.Header().Get("Location") != "/admin" {
		t.Fatalf("Expected redirect to /admin after setup, got %d %s", w.Code, w.Header().Get("Location"))
	}

	// 3. User now exists, /admin/setup should redirect to /admin
	req = httptest.NewRequest(http.MethodGet, "/admin/setup", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/admin" {
		t.Fatalf("Expected redirect to /admin when setup already complete, got %d %s", w.Code, w.Header().Get("Location"))
	}

	// 4. Regular user login attempt should fail
	// First create a regular user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
		ID:          uuid.New().String(),
		Email:       "regular@example.com",
		Password:    string(hashedPassword),
		IsAdmin:     0,
		CreatedTime: time.Now().UnixMilli(),
		UpdatedTime: time.Now().UnixMilli(),
	})

	form = url.Values{}
	form.Add("email", "regular@example.com")
	form.Add("password", "password")
	req = httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Since they are not an admin, it should just re-render login with error, returning 200 (HTMX standard practice or typical SSR)
	// We currently return 200 with the template for errors.
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK with error template for non-admin login, got %d", w.Code)
	}

	// 5. Admin login success
	form = url.Values{}
	form.Add("email", "admin@example.com")
	form.Add("password", "password123")
	req = httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound || w.Header().Get("Location") != "/admin" {
		t.Fatalf("Expected redirect to /admin after successful login, got %d", w.Code)
	}

	// Get the session cookie
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "admin_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("Expected admin_session cookie to be set")
	}

	// 6. Access /admin with cookie
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK when accessing /admin with valid session, got %d", w.Code)
	}
}
