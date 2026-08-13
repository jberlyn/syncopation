package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/api"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/storage"
)

func TestUserDeletionCascade(t *testing.T) {
	queries := setupTestDB(t)
	tmpDir := t.TempDir()
	store := storage.NewLocalFS(tmpDir)

	adminHandler := &api.AdminHandler{Queries: queries, Storage: store}

	ctx := context.Background()
	now := time.Now().UnixMilli()

	// 1. Create Admin
	adminID := uuid.New().String()
	admin, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:           adminID,
		Email:        "admin@example.com",
		PasswordHash: "hashed",
		IsAdmin:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	// 2. Create User A
	userAID := uuid.New().String()
	_, err = queries.CreateUser(ctx, db.CreateUserParams{
		ID:           userAID,
		Email:        "usera@example.com",
		PasswordHash: "hashed",
		IsAdmin:      0,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Failed to create User A: %v", err)
	}

	// 3. Create User B
	userBID := uuid.New().String()
	_, err = queries.CreateUser(ctx, db.CreateUserParams{
		ID:           userBID,
		Email:        "userb@example.com",
		PasswordHash: "hashed",
		IsAdmin:      0,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Failed to create User B: %v", err)
	}

	// Create session for User A
	_, err = queries.CreateSession(ctx, db.CreateSessionParams{
		ID:        uuid.New().String(),
		UserID:    userAID,
		AuthCode:  "",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Failed to create session for User A: %v", err)
	}

	// Create item for User A
	itemID := uuid.New().String()
	_, err = queries.UpsertSyncItem(ctx, db.UpsertSyncItemParams{
		ID:              itemID,
		FileName:        "test_item.md",
		MimeType:        "text/markdown",
		JoplinID:        itemID,
		ParentID:        "",
		ShareID:         "share1",
		ItemType:        1,
		IsEncrypted:     0,
		ClientUpdatedAt: now,
		OwnerID:         userAID,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatalf("Failed to create item for User A: %v", err)
	}
	_, err = queries.UpsertUserSyncItem(ctx, db.UpsertUserSyncItemParams{
		UserID:     userAID,
		SyncItemID: itemID,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("Failed to create user item for User A: %v", err)
	}

	// Write file for User A
	err = store.WriteItem(ctx, userAID, "test_item.md", []byte("hello"))
	if err != nil {
		t.Fatalf("Failed to write item: %v", err)
	}

	// Create a share owned by User A
	shareID := "share1"
	_, err = queries.CreateShare(ctx, db.CreateShareParams{
		ID:        shareID,
		OwnerID:   userAID,
		FolderID:  "folder1",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	// Add User B to the share
	_, err = queries.CreateUserShare(ctx, db.CreateUserShareParams{
		ShareID:   shareID,
		UserID:    userBID,
		Status:    1,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Failed to create user_share: %v", err)
	}

	// Make HTTP request to delete User A
	req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+userAID, nil)
	// Add admin to context
	req = req.WithContext(context.WithValue(req.Context(), api.AdminUserKey, admin))
	w := httptest.NewRecorder()

	adminHandler.HandleUsersDelete(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Result().StatusCode)
	}

	// Verify User A is deleted
	_, err = queries.GetUser(ctx, userAID)
	if err == nil {
		t.Fatalf("Expected User A to be deleted")
	}

	// Verify items are deleted
	items, err := queries.ListSyncItemsByUser(ctx, db.ListSyncItemsByUserParams{
		UserID: userAID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil || len(items) > 0 {
		t.Fatalf("Expected User A items to be deleted, got %d", len(items))
	}

	// Verify tombstone for User B (tombstone for shared item)
	changes, err := queries.GetDeltaEventsByUser(ctx, db.GetDeltaEventsByUserParams{
		UserID: userBID,
		ID:     0,
		Limit:  10,
	})
	if err != nil || len(changes) == 0 {
		t.Fatalf("Expected tombstone change for User B")
	}

	foundTombstone := false
	for _, c := range changes {
		if c.JoplinID == itemID && c.EventType == 3 {
			foundTombstone = true
			break
		}
	}
	if !foundTombstone {
		t.Fatalf("Expected tombstone for item %s", itemID)
	}

	// Verify filesystem is cleaned up
	userDir := filepath.Join(tmpDir, userAID)
	if _, err := os.Stat(userDir); !os.IsNotExist(err) {
		t.Fatalf("Expected user directory to be deleted, err: %v", err)
	}
}
