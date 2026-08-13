package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/joplin-sync/api"
	"github.com/jberlyn/joplin-sync/config"
	"github.com/jberlyn/joplin-sync/db"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	seedFlag := flag.Bool("seed", false, "Seed the database with an admin user")
	emailFlag := flag.String("email", "admin@localhost", "Email for the seeded user")
	passwordFlag := flag.String("password", "admin", "Password for the seeded user")
	flag.Parse()

	cfg := config.LoadConfig()

	// Connect to SQLite DB
	dbConn, err := sql.Open("sqlite3", cfg.DBPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer dbConn.Close()

	// Run migrations (In a real app, use golang-migrate, here we run schema.sql)
	schema, err := os.ReadFile("db/schema.sql")
	if err != nil {
		log.Fatalf("Failed to read schema.sql: %v", err)
	}
	if _, err := dbConn.Exec(string(schema)); err != nil {
		// Ignore "already exists" errors or ensure schema.sql uses IF NOT EXISTS
		log.Printf("Migration output (ignoring errors if tables exist): %v", err)
	}

	queries := db.New(dbConn)

	if *seedFlag {
		seedUser(queries, *emailFlag, *passwordFlag)
		return
	}

	// Setup HTTP server
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

	itemHandler := &api.ItemHandler{Queries: queries}
	mux.Handle("/api/items/root:/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))

	log.Printf("Server listening on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
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
		log.Fatalf("Failed to hash password: %v", err)
	}

	id := uuid.New().String()
	now := time.Now().UnixMilli()

	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:          id,
		Email:       email,
		Password:    string(hashedPassword),
		FullName:    "Admin User",
		IsAdmin:     1,
		CreatedTime: now,
		UpdatedTime: now,
	})
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("Successfully created admin user: %s (ID: %s)\n", user.Email, user.ID)
}
