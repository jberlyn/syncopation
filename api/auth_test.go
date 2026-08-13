package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/api"
	"github.com/jberlyn/syncopation/db"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func setupTestDBConn(t *testing.T) (*sql.DB, *db.Queries) {
	dbConn, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	t.Cleanup(func() { dbConn.Close() })

	schema, err := os.ReadFile("../db/schema.sql")
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	if _, err := dbConn.Exec(string(schema)); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return dbConn, db.New(dbConn)
}

func setupTestDB(t *testing.T) *db.Queries {
	_, queries := setupTestDBConn(t)
	return queries
}

func seedUser(t *testing.T, queries *db.Queries, email, password string) db.User {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	now := time.Now().UnixMilli()

	user, err := queries.CreateUser(context.Background(), db.CreateUserParams{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		IsAdmin:      0,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}
	return user
}

func TestAuthFlow(t *testing.T) {
	queries := setupTestDB(t)
	authHandler := &api.AuthHandler{Queries: queries}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/sessions", authHandler.Login)
	mux.HandleFunc("DELETE /api/sessions/{id}", authHandler.Logout)

	// Protected route for testing middleware
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(api.UserIDKey)
		if userID == nil {
			t.Errorf("Expected user_id in context, got nil")
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /api/protected", authHandler.RequireAuth(protected))

	user := seedUser(t, queries, "test@example.com", "password123")

	// 1. Test Login - Success
	t.Run("Login Success", func(t *testing.T) {
		reqBody := map[string]string{"email": "test@example.com", "password": "password123"}
		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", rr.Code)
		}

		var resp api.LoginResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.UserID != user.ID {
			t.Errorf("Expected user ID %s, got %s", user.ID, resp.UserID)
		}
		if len(resp.ID) != 32 {
			t.Errorf("Expected 32-character session ID, got %d chars: %s", len(resp.ID), resp.ID)
		}

		// 2. Test Protected Route - Success
		req2 := httptest.NewRequest("GET", "/api/protected", nil)
		req2.Header.Set("X-API-AUTH", resp.ID)
		rr2 := httptest.NewRecorder()

		mux.ServeHTTP(rr2, req2)
		if rr2.Code != http.StatusOK {
			t.Errorf("Expected 200 OK on protected route, got %d", rr2.Code)
		}

		// 3. Test Logout
		req3 := httptest.NewRequest("DELETE", "/api/sessions/"+resp.ID, nil)
		rr3 := httptest.NewRecorder()

		mux.ServeHTTP(rr3, req3)
		if rr3.Code != http.StatusOK {
			t.Errorf("Expected 200 OK on logout, got %d", rr3.Code)
		}

		// 4. Test Protected Route - Failure after logout
		req4 := httptest.NewRequest("GET", "/api/protected", nil)
		req4.Header.Set("X-API-AUTH", resp.ID)
		rr4 := httptest.NewRecorder()

		mux.ServeHTTP(rr4, req4)
		if rr4.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden after logout, got %d", rr4.Code)
		}
	})

	// 5. Test Login - Invalid Credentials
	t.Run("Login Failure", func(t *testing.T) {
		reqBody := map[string]string{"email": "test@example.com", "password": "wrongpassword"}
		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got %d", rr.Code)
		}
	})

	// 6. Test Protected Route - No Header
	t.Run("Protected Route No Header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/protected", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden without header, got %d", rr.Code)
		}
	})
}
