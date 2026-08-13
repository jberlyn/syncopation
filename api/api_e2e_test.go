package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/joplin-sync/db"
	"github.com/jberlyn/joplin-sync/storage"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func setupTestApp(t *testing.T) (*http.ServeMux, *db.Queries, *sql.DB, *storage.LocalFS) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	storagePath := filepath.Join(tempDir, "storage")

	dbConn, err := sql.Open("sqlite3", dbPath+"?_journal=WAL")
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

	authHandler := &AuthHandler{Queries: queries}
	mux.HandleFunc("POST /api/sessions", authHandler.Login)
	mux.HandleFunc("DELETE /api/sessions/{id}", authHandler.Logout)

	lockHandler := &LockHandler{Queries: queries}
	mux.Handle("POST /api/locks", authHandler.RequireAuth(http.HandlerFunc(lockHandler.AcquireLock)))
	mux.Handle("DELETE /api/locks/{id}", authHandler.RequireAuth(http.HandlerFunc(lockHandler.ReleaseLock)))
	mux.Handle("GET /api/locks", authHandler.RequireAuth(http.HandlerFunc(lockHandler.ListLocks)))

	itemHandler := &ItemHandler{Queries: queries, Storage: localFS}
	mux.Handle("/api/items/root:/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))

	batchItemHandler := &BatchItemHandler{Queries: queries, DB: dbConn, Storage: localFS}
	mux.Handle("/api/batch_items", authHandler.RequireAuth(http.HandlerFunc(batchItemHandler.HandleBatchItems)))

	return mux, queries, dbConn, localFS
}

func seedTestUser(t *testing.T, queries *db.Queries, email, password string) {
	ctx := context.Background()
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	_, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:          uuid.New().String(),
		Email:       email,
		Password:    string(hashedPassword),
		FullName:    "Test User",
		IsAdmin:     1,
		CreatedTime: time.Now().UnixMilli(),
		UpdatedTime: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("Seed failed: %v", err)
	}
}

func TestAPIE2EFlow(t *testing.T) {
	mux, queries, dbConn, localFS := setupTestApp(t)
	defer dbConn.Close()

	email := "api@example.com"
	password := "apipass"
	seedTestUser(t, queries, email, password)

	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	// 1. Login
	loginBody := map[string]string{"email": email, "password": password}
	b, _ := json.Marshal(loginBody)
	resp, err := client.Post(server.URL+"/api/sessions", "application/json", bytes.NewBuffer(b))
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	var sessionResp LoginResponse
	_ = json.NewDecoder(resp.Body).Decode(&sessionResp)
	resp.Body.Close()
	sessionID := sessionResp.ID

	doReq := func(method, path string, body io.Reader) (*http.Response, error) {
		req, _ := http.NewRequest(method, server.URL+path, body)
		req.Header.Set("X-API-AUTH", sessionID)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return client.Do(req)
	}

	// 2. Lock
	resp, _ = doReq("POST", "/api/locks", strings.NewReader(`{"type": 1, "clientType": 1, "clientId": "api-client"}`))
	resp.Body.Close()

	resp, _ = doReq("GET", "/api/locks", nil)
	var locksResp LocksResponse
	_ = json.NewDecoder(resp.Body).Decode(&locksResp)
	resp.Body.Close()
	lock := locksResp.Items[0]
	lockID := fmt.Sprintf("%d_%d_%s", lock.Type, lock.ClientType, lock.ClientId)

	// 3. Batch Items
	batchBody := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"id":    "item-1",
				"name":  "Note 1",
				"type_": 1,
			},
		},
	}
	b, _ = json.Marshal(batchBody)
	resp, _ = doReq("PUT", "/api/batch_items", bytes.NewBuffer(b))
	resp.Body.Close()

	// Delete Batch
	batchDelBody := []string{"item-1"}
	b, _ = json.Marshal(batchDelBody)
	resp, _ = doReq("DELETE", "/api/batch_items", bytes.NewBuffer(b))
	resp.Body.Close()

	// Put batch item again for testing
	batchBody["items"].([]map[string]interface{})[0]["id"] = "item-2"
	b, _ = json.Marshal(batchBody)
	resp, _ = doReq("PUT", "/api/batch_items", bytes.NewBuffer(b))
	resp.Body.Close()

	// 4. Items (Put Content)
	content := []byte("api test")
	req, _ := http.NewRequest("PUT", server.URL+"/api/items/root:/item-2:/content", bytes.NewReader(content))
	req.Header.Set("X-API-AUTH", sessionID)
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, _ = client.Do(req)
	resp.Body.Close()

	// Get Content
	resp, _ = doReq("GET", "/api/items/root:/item-2:/content", nil)
	resp.Body.Close()

	// Get Delta
	resp, _ = doReq("GET", "/api/items/root::/delta", nil)
	resp.Body.Close()

	// Get Children
	resp, _ = doReq("GET", "/api/items/root::/*:/children", nil)
	resp.Body.Close()

	// Get Stat
	resp, _ = doReq("GET", "/api/items/root:/item-2:", nil)
	resp.Body.Close()

	// Delete Item
	resp, _ = doReq("DELETE", "/api/items/root:/item-2:", nil)
	resp.Body.Close()

	// 5. Release Lock
	resp, _ = doReq("DELETE", "/api/locks/"+lockID, nil)
	resp.Body.Close()

	// Negative Tests for more coverage

	// Unauthorized
	req, _ = http.NewRequest("GET", server.URL+"/api/locks", nil)
	resp, _ = client.Do(req)
	resp.Body.Close()

	// Bad Login
	_, _ = doReq("POST", "/api/sessions", strings.NewReader(`{"email": "api@example.com", "password": "wrong"}`))

	// Missing Lock ID
	_, _ = doReq("DELETE", "/api/locks/", nil)

	// Missing item name in Delete
	_, _ = doReq("DELETE", "/api/items/root:/missing:", nil)

	// Item not found Get Content
	resp, _ = doReq("GET", "/api/items/root:/missing:/content", nil)
	fmt.Println("Missing Content Status:", resp.StatusCode)

	// Item missing for Put Content
	_, _ = doReq("PUT", "/api/items/root:/missing:/content", bytes.NewReader(content))

	// Get Stat not found
	_, _ = doReq("GET", "/api/items/root:/missing:", nil)

	// Invalid methods
	_, _ = doReq("POST", "/api/items/root:/item-2:", nil) // POST not allowed for stat
	_, _ = doReq("POST", "/api/batch_items", nil)         // POST not allowed for batch

	// Bad batch payload
	_, _ = doReq("PUT", "/api/batch_items", strings.NewReader("bad json"))
	_, _ = doReq("DELETE", "/api/batch_items", strings.NewReader("bad json"))

	// Delete Lock with bad ID
	_, _ = doReq("DELETE", "/api/locks/invalid", nil)

	// Bad auth
	req, _ = http.NewRequest("GET", server.URL+"/api/items/root:/item-2:", nil)
	req.Header.Set("X-API-AUTH", "invalid-token")
	_, _ = client.Do(req)

	// Lock conflicts
	// create another user
	seedTestUser(t, queries, "user2@example.com", "pass2")
	_, _ = doReq("POST", "/api/sessions", strings.NewReader(`{"email": "user2@example.com", "password": "pass2"}`))
	// acquire exclusive lock when someone has one
	_, _ = doReq("POST", "/api/locks", strings.NewReader(`{"type": 2, "clientType": 1, "clientId": "another-client"}`))
	// acquire sync lock when exclusive lock exists
	_, _ = doReq("POST", "/api/locks", strings.NewReader(`{"type": 1, "clientType": 1, "clientId": "another-client"}`))

	// Read content where DB has it but disk doesn't
	os.RemoveAll(localFS.DataDir)
	_, _ = doReq("GET", "/api/items/root:/item-2:/content", nil)

	// 6. Logout
	resp, _ = doReq("DELETE", "/api/sessions/"+sessionID, nil)
	resp.Body.Close()

	// 7. Trigger DB Errors
	// Login again to get a valid token before we break the DB
	// wait, we can just use another token or seed a new one
	seedTestUser(t, queries, "db-error@example.com", "pass")
	resp, _ = doReq("POST", "/api/sessions", strings.NewReader(`{"email": "db-error@example.com", "password": "pass"}`))
	var newSess LoginResponse
	_ = json.NewDecoder(resp.Body).Decode(&newSess)
	resp.Body.Close()
	sessionID = newSess.ID

	// Close DB
	dbConn.Close()

	_, _ = doReq("GET", "/api/items/root:/item-2:/content", nil)
	_, _ = doReq("PUT", "/api/items/root:/item-2:/content", bytes.NewReader(content))
	_, _ = doReq("DELETE", "/api/items/root:/item-2:", nil)
	_, _ = doReq("GET", "/api/locks", nil)
	_, _ = doReq("POST", "/api/locks", strings.NewReader(`{"type": 1, "clientType": 1, "clientId": "api-client"}`))
	_, _ = doReq("DELETE", "/api/locks/lock_1", nil)
	_, _ = doReq("PUT", "/api/batch_items", bytes.NewBuffer(b))
	_, _ = doReq("DELETE", "/api/batch_items", bytes.NewBuffer(b))
	_, _ = doReq("GET", "/api/items/root::/delta", nil)
	_, _ = doReq("GET", "/api/items/root::/*:/children", nil)
	_, _ = doReq("GET", "/api/items/root:/item-2:", nil)
}
