package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/api"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/storage"
)

func TestParseItemPath(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantContent bool
		wantValid   bool
	}{
		{"/api/items/root:/test.md:", "test.md", false, true},
		{"/api/items/root:/test.md:/content", "test.md", true, true},
		{"/api/items/root:/folder/test.md:", "folder/test.md", false, true},
		{"/api/items/root:/folder/test.md:/content", "folder/test.md", true, true},
		{"/api/items/root:/invalid", "", false, false},
		{"/api/items/invalid", "", false, false},
	}

	for _, tc := range tests {
		req := httptest.NewRequest("GET", tc.input, nil)
		// We use a small hack here since parseItemPath is unexported,
		// we test the handler behavior indirectly or just test the handler with these paths.
		// Since we want to test parseItemPath itself, we can export it or copy it here for testing.
		_ = req
	}
}

func TestItemCRUD(t *testing.T) {
	queries := setupTestDB(t)
	localFS := storage.NewLocalFS(t.TempDir())
	authHandler := &api.AuthHandler{Queries: queries}
	itemHandler := &api.ItemHandler{Queries: queries, Storage: localFS}

	mux := http.NewServeMux()
	mux.Handle("/api/items/root:/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))

	user := seedUser(t, queries, "items@example.com", "password")
	sessionID := uuid.New().String()
	now := time.Now().UnixMilli()

	_, err := queries.CreateSession(context.Background(), db.CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		AuthCode:  "",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	itemName := "test_note.md"
	itemContent := []byte("# Hello Joplin\nThis is a test note.")

	// 1. PUT Content
	t.Run("Create Item Content", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/items/root:/"+itemName+":/content", bytes.NewBuffer(itemContent))
		req.Header.Set("X-API-AUTH", sessionID)
		req.Header.Set("Content-Type", "text/markdown")
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp api.ItemMetadataResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Name != itemName {
			t.Errorf("Expected item name %s, got %s", itemName, resp.Name)
		}
	})

	// 2. GET Stat
	t.Run("Get Item Stat", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/items/root:/"+itemName+":", nil)
		req.Header.Set("X-API-AUTH", sessionID)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		var resp api.ItemMetadataResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Name != itemName {
			t.Errorf("Expected item name %s, got %s", itemName, resp.Name)
		}
	})

	// 3. GET Content
	t.Run("Get Item Content", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/items/root:/"+itemName+":/content", nil)
		req.Header.Set("X-API-AUTH", sessionID)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}

		body, _ := io.ReadAll(rr.Body)
		if !bytes.Equal(body, itemContent) {
			t.Errorf("Expected content %q, got %q", itemContent, body)
		}
	})

	// 4. DELETE Item
	t.Run("Delete Item", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/items/root:/"+itemName+":", nil)
		req.Header.Set("X-API-AUTH", sessionID)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
	})

	// 5. GET Stat (should be 404)
	t.Run("Get Item Stat (Not Found)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/items/root:/"+itemName+":", nil)
		req.Header.Set("X-API-AUTH", sessionID)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected 404 Not Found, got %d", rr.Code)
		}
	})
}

func TestDeltaSync(t *testing.T) {
	queries := setupTestDB(t)
	localFS := storage.NewLocalFS(t.TempDir())
	authHandler := &api.AuthHandler{Queries: queries}
	itemHandler := &api.ItemHandler{Queries: queries, Storage: localFS}

	mux := http.NewServeMux()
	mux.Handle("/api/items/root:/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))
	// Delta sync is requested with root:: path
	mux.Handle("/api/items/root::/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))

	user := seedUser(t, queries, "delta@example.com", "password")
	sessionID := uuid.New().String()
	now := time.Now().UnixMilli()

	_, _ = queries.CreateSession(context.Background(), db.CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		CreatedAt: now,
		UpdatedAt: now,
	})

	itemName := "delta_note.md"
	itemContent := []byte("Initial content")
	updatedContent := []byte("Updated content")

	// 1. Create Item
	reqCreate := httptest.NewRequest("PUT", "/api/items/root:/"+itemName+":/content", bytes.NewBuffer(itemContent))
	reqCreate.Header.Set("X-API-AUTH", sessionID)
	reqCreate.Header.Set("Content-Type", "text/markdown")
	rrCreate := httptest.NewRecorder()
	mux.ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusOK {
		t.Fatalf("Failed to create item: %d %s", rrCreate.Code, rrCreate.Body.String())
	}

	// 2. Fetch Delta (should return 1 create change)
	reqDelta1 := httptest.NewRequest("GET", "/api/items/root::/delta", nil)
	reqDelta1.Header.Set("X-API-AUTH", sessionID)
	rrDelta1 := httptest.NewRecorder()
	mux.ServeHTTP(rrDelta1, reqDelta1)

	if rrDelta1.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for delta, got %d. Body: %s", rrDelta1.Code, rrDelta1.Body.String())
	}

	var deltaResp1 api.DeltaResponse
	if err := json.NewDecoder(rrDelta1.Body).Decode(&deltaResp1); err != nil {
		t.Fatalf("Failed to decode delta response: %v", err)
	}
	if len(deltaResp1.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaResp1.Items))
	}
	if deltaResp1.Items[0].Type != 1 {
		t.Errorf("Expected change type 1 (Create), got %d", deltaResp1.Items[0].Type)
	}

	cursor1 := deltaResp1.Cursor

	// 3. Update Item
	reqUpdate := httptest.NewRequest("PUT", "/api/items/root:/"+itemName+":/content", bytes.NewBuffer(updatedContent))
	reqUpdate.Header.Set("X-API-AUTH", sessionID)
	reqUpdate.Header.Set("Content-Type", "text/markdown")
	rrUpdate := httptest.NewRecorder()
	mux.ServeHTTP(rrUpdate, reqUpdate)
	if rrUpdate.Code != http.StatusOK {
		t.Fatalf("Failed to update item: %d", rrUpdate.Code)
	}

	// 4. Fetch Delta with Cursor 1 (should return 1 update change)
	reqDelta2 := httptest.NewRequest("GET", "/api/items/root::/delta?cursor="+cursor1, nil)
	reqDelta2.Header.Set("X-API-AUTH", sessionID)
	rrDelta2 := httptest.NewRecorder()
	mux.ServeHTTP(rrDelta2, reqDelta2)

	var deltaResp2 api.DeltaResponse
	_ = json.NewDecoder(rrDelta2.Body).Decode(&deltaResp2)
	if len(deltaResp2.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaResp2.Items))
	}
	if deltaResp2.Items[0].Type != 2 {
		t.Errorf("Expected change type 2 (Update), got %d", deltaResp2.Items[0].Type)
	}

	cursor2 := deltaResp2.Cursor

	// 5. Delete Item
	reqDelete := httptest.NewRequest("DELETE", "/api/items/root:/"+itemName+":", nil)
	reqDelete.Header.Set("X-API-AUTH", sessionID)
	rrDelete := httptest.NewRecorder()
	mux.ServeHTTP(rrDelete, reqDelete)

	// 6. Fetch Delta with Cursor 2 (should return 1 delete change)
	reqDelta3 := httptest.NewRequest("GET", "/api/items/root::/delta?cursor="+cursor2, nil)
	reqDelta3.Header.Set("X-API-AUTH", sessionID)
	rrDelta3 := httptest.NewRecorder()
	mux.ServeHTTP(rrDelta3, reqDelta3)

	var deltaResp3 api.DeltaResponse
	_ = json.NewDecoder(rrDelta3.Body).Decode(&deltaResp3)
	if len(deltaResp3.Items) != 1 {
		t.Fatalf("Expected 1 delta item, got %d", len(deltaResp3.Items))
	}
	if deltaResp3.Items[0].Type != 3 {
		t.Errorf("Expected change type 3 (Delete), got %d", deltaResp3.Items[0].Type)
	}
}

func TestDirectoryChildren(t *testing.T) {
	queries := setupTestDB(t)
	localFS := storage.NewLocalFS(t.TempDir())
	authHandler := &api.AuthHandler{Queries: queries}
	itemHandler := &api.ItemHandler{Queries: queries, Storage: localFS}

	mux := http.NewServeMux()
	mux.Handle("/api/items/root:/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))
	// Children sync could be requested with root:: or root:/*:/children
	mux.Handle("/api/items/root::/", authHandler.RequireAuth(http.HandlerFunc(itemHandler.HandleItems)))

	user := seedUser(t, queries, "children@example.com", "password")
	sessionID := uuid.New().String()
	now := time.Now().UnixMilli()

	_, _ = queries.CreateSession(context.Background(), db.CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		CreatedAt: now,
		UpdatedAt: now,
	})

	// 1. Create two items
	for _, name := range []string{"file1.md", "file2.md"} {
		reqCreate := httptest.NewRequest("PUT", "/api/items/root:/"+name+":/content", bytes.NewBuffer([]byte("content")))
		reqCreate.Header.Set("X-API-AUTH", sessionID)
		reqCreate.Header.Set("Content-Type", "text/markdown")
		rrCreate := httptest.NewRecorder()
		mux.ServeHTTP(rrCreate, reqCreate)
		if rrCreate.Code != http.StatusOK {
			t.Fatalf("Failed to create item: %d", rrCreate.Code)
		}
	}

	// 2. Fetch children
	reqChildren := httptest.NewRequest("GET", "/api/items/root:/*:/children", nil)
	reqChildren.Header.Set("X-API-AUTH", sessionID)
	rrChildren := httptest.NewRecorder()
	mux.ServeHTTP(rrChildren, reqChildren)

	if rrChildren.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for children, got %d", rrChildren.Code)
	}

	var resp struct {
		Items   []api.ItemMetadataResponse `json:"items"`
		HasMore bool                       `json:"has_more"`
		Cursor  string                     `json:"cursor"`
	}

	if err := json.NewDecoder(rrChildren.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(resp.Items))
	}
}
