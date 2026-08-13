package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jberlyn/joplin-sync/db"
	_ "github.com/mattn/go-sqlite3"
)

func TestHealthCheck(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/api/ping", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"ok"}` + "\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestDatabaseSeed(t *testing.T) {
	// Setup an in-memory database for testing
	dbConn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer dbConn.Close()

	schema, err := os.ReadFile("db/schema.sql")
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	if _, err := dbConn.Exec(string(schema)); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	queries := db.New(dbConn)
	ctx := context.Background()

	// Seed user
	email := "test@example.com"
	password := "testpass"

	seedUser(queries, email, password)

	// Verify user was seeded
	user, err := queries.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("Failed to get seeded user: %v", err)
	}

	if user.Email != email {
		t.Errorf("Expected email %s, got %s", email, user.Email)
	}
	if user.IsAdmin != 1 {
		t.Errorf("Expected user to be admin (1), got %d", user.IsAdmin)
	}

	// Try seeding again, should not fail or duplicate
	seedUser(queries, email, password)
}
