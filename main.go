package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/api"
	"github.com/jberlyn/syncopation/config"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/storage"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var Version = "v1.0.2"

func main() {
	seedFlag := flag.Bool("seed", false, "Seed the database with an admin user")
	emailFlag := flag.String("email", "admin@localhost", "Email for the seeded user")
	passwordFlag := flag.String("password", "admin", "Password for the seeded user")
	flag.Parse()

	cfg := config.LoadConfig()

	// Configure global slog to use JSON, similar to Node's pino
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Ensure the parent directory for the database exists
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		slog.Error("Failed to create database directory", "error", err)
		os.Exit(1)
	}

	// Connect to SQLite DB
	dbConn, err := sql.Open("sqlite", cfg.DBPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	// Run migrations (In a real app, use golang-migrate, here we run schema.sql)
	schema, err := os.ReadFile("db/schema.sql")
	if err != nil {
		slog.Error("Failed to read schema.sql", "error", err)
		os.Exit(1)
	}
	if _, err := dbConn.Exec(string(schema)); err != nil {
		// Ignore "already exists" errors or ensure schema.sql uses IF NOT EXISTS
		slog.Info("Migration output (ignoring errors if tables exist)", "error", err)
	}

	queries := db.New(dbConn)
	localFS := storage.NewLocalFS(cfg.StoragePath)

	if *seedFlag {
		seedUser(queries, *emailFlag, *passwordFlag)
		return
	}

	mux := setupMux(queries, dbConn, localFS)

	slog.Info("Server listening", "port", cfg.Port)

	// Wrap mux with the LoggingMiddleware
	loggedMux := api.LoggingMiddleware(mux)

	if err := http.ListenAndServe(":"+cfg.Port, loggedMux); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func setupMux(queries *db.Queries, dbConn *sql.DB, localFS *storage.LocalFS) *http.ServeMux {
	mux := http.NewServeMux()

	authHandler := &api.AuthHandler{Queries: queries}

	// GET /api/ping
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/sessions", authHandler.Login)
	mux.HandleFunc("DELETE /api/sessions/{id}", authHandler.Logout)

	lockHandler := &api.LockHandler{Queries: queries}
	mux.Handle("POST /api/locks", authHandler.RequireAuth(http.HandlerFunc(lockHandler.AcquireLock)))
	mux.Handle("DELETE /api/locks/{id}", authHandler.RequireAuth(http.HandlerFunc(lockHandler.ReleaseLock)))
	mux.Handle("GET /api/locks", authHandler.RequireAuth(http.HandlerFunc(lockHandler.ListLocks)))

	itemHandler := &api.ItemHandler{Queries: queries, Storage: localFS}
	mux.Handle("/api/items/root:/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))

	batchItemHandler := &api.BatchItemHandler{Queries: queries, DB: dbConn, Storage: localFS}
	mux.Handle("/api/batch_items", authHandler.RequireAuth(http.HandlerFunc(batchItemHandler.HandleBatchItems)))

	// Admin UI Routes
	adminHandler := &api.AdminHandler{Queries: queries, Storage: localFS, Version: Version}

	// Create a sub-router for admin routes so we can apply the middleware to all of them easily.
	// Since Go 1.22 mux doesn't easily let us apply middleware to a prefix without stripping or matching exactly,
	// we will wrap each handler.
	mux.Handle("GET /admin/setup", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleSetupGet)))
	mux.Handle("POST /admin/setup", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleSetupPost)))
	mux.Handle("GET /admin/login", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleLoginGet)))
	mux.Handle("POST /admin/login", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleLoginPost)))
	mux.Handle("GET /admin/logout", http.HandlerFunc(adminHandler.HandleLogout))
	// Exact match for /admin
	mux.Handle("GET /admin", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleDashboard)))
	mux.Handle("GET /admin/", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleDashboard)))

	mux.Handle("POST /admin/users", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleUsersPost)))
	mux.Handle("DELETE /admin/users/{id}", adminHandler.AdminMiddleware(http.HandlerFunc(adminHandler.HandleUsersDelete)))

	return mux
}

func seedUser(queries *db.Queries, email, password string) {
	ctx := context.Background()

	// Check if user exists
	existingUser, err := queries.GetUserByEmail(ctx, email)
	if err == nil && existingUser.ID != "" {
		fmt.Printf("User %s already exists with ID %s\n", email, existingUser.ID)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		os.Exit(1)
	}

	id := uuid.New().String()
	now := time.Now().UnixMilli()

	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:           id,
		Email:        email,
		PasswordHash: string(hashedPassword),
		IsAdmin:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		slog.Error("Failed to create user", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created admin user: %s (ID: %s)\n", user.Email, user.ID)
}
