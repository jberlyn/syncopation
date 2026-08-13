package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/joplin-sync/api"
	"github.com/jberlyn/joplin-sync/db"
	"github.com/jberlyn/joplin-sync/storage"
)

func TestBatchOperations(t *testing.T) {
	dbConn, queries := setupTestDBConn(t)
	localFS := storage.NewLocalFS(t.TempDir())
	authHandler := &api.AuthHandler{Queries: queries}
	batchItemHandler := &api.BatchItemHandler{Queries: queries, DB: dbConn, Storage: localFS}

	mux := http.NewServeMux()
	mux.Handle("/api/batch_items", authHandler.RequireAuth(http.HandlerFunc(batchItemHandler.HandleBatchItems)))

	user := seedUser(t, queries, "batch@example.com", "password")
	sessionID := uuid.New().String()
	now := time.Now().UnixMilli()

	_, _ = queries.CreateSession(context.Background(), db.CreateSessionParams{
		ID:          sessionID,
		UserID:      user.ID,
		CreatedTime: now,
		UpdatedTime: now,
	})

	// 1. Batch Upload Items
	t.Run("Batch Upload", func(t *testing.T) {
		reqBody := api.BatchPutRequest{
			Items: []api.BatchPutItem{
				{Name: "note1.md", Body: "content1"},
				{Name: "note2.md", Body: "content2"},
			},
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("PUT", "/api/batch_items", bytes.NewBuffer(bodyBytes))
		req.Header.Set("X-API-AUTH", sessionID)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp api.BatchPutResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(resp.Items) != 2 {
			t.Fatalf("Expected 2 items, got %d", len(resp.Items))
		}

		if resp.Items["note1.md"].Item == nil || resp.Items["note1.md"].Item.Name != "note1.md" {
			t.Errorf("Expected item note1.md in response")
		}
	})

	// 2. Batch Delete Items
	t.Run("Batch Delete", func(t *testing.T) {
		reqBody := api.BatchDeleteRequest{
			Items: []string{"note1.md", "note2.md", "nonexistent.md"},
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("DELETE", "/api/batch_items", bytes.NewBuffer(bodyBytes))
		req.Header.Set("X-API-AUTH", sessionID)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp api.BatchDeleteResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(resp.Items) != 3 {
			t.Fatalf("Expected 3 items in response, got %d", len(resp.Items))
		}
	})
}
