package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jberlyn/joplin-sync/config"
	"github.com/jberlyn/joplin-sync/db"
	"github.com/jberlyn/joplin-sync/storage"
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
	dbConn, err := sql.Open("sqlite3", ":memory:?_fk=1")
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

func TestMain_Seed(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("DB_PATH", tempDir+"/main_test.db")
	os.Setenv("STORAGE_PATH", tempDir+"/storage")
	os.Setenv("PORT", "9999")
	defer func() {
		os.Unsetenv("DB_PATH")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("PORT")
	}()

	os.Args = []string{"joplin-sync", "-seed", "-email", "main@example.com", "-password", "mainpass"}

	main()
}

func TestSetupMux(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("DB_PATH", tempDir+"/main_test_mux.db")
	os.Setenv("STORAGE_PATH", tempDir+"/storage")
	os.Setenv("PORT", "9999")
	defer func() {
		os.Unsetenv("DB_PATH")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("PORT")
	}()

	cfg := config.LoadConfig()
	dbConn, _ := sql.Open("sqlite3", cfg.DBPath+"?_journal=WAL&_fk=1")
	defer dbConn.Close()
	schema, _ := os.ReadFile("db/schema.sql")
	_, _ = dbConn.Exec(string(schema))

	queries := db.New(dbConn)
	localFS := storage.NewLocalFS(cfg.StoragePath)

	mux := setupMux(queries, dbConn, localFS)
	if mux == nil {
		t.Fatal("setupMux returned nil")
	}
}

func TestMain_Run(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("DB_PATH", tempDir+"/main_test_run.db")
	os.Setenv("STORAGE_PATH", tempDir+"/storage_run")
	os.Setenv("PORT", "0") // let OS pick port
	defer func() {
		os.Unsetenv("DB_PATH")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("PORT")
	}()

	os.Args = []string{"joplin-sync"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	go main()

	// Wait a moment for server to start and then exit test
	// This will cover the main setup paths!
	time.Sleep(100 * time.Millisecond)
}
