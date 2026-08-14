package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/db"
	_ "modernc.org/sqlite"
)

func TestDBIntegrationCoverage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	dbConn, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer dbConn.Close()

	schema, err := os.ReadFile("../db/schema.sql")
	if err != nil {
		t.Fatalf("Failed to read schema: %v", err)
	}
	_, _ = dbConn.Exec(string(schema))

	queries := db.New(dbConn)
	ctx := context.Background()

	// Create user
	userID := uuid.New().String()
	_, err = queries.CreateUser(ctx, db.CreateUserParams{
		ID:           userID,
		Email:        "test@example.com",
		PasswordHash: "hash",
		IsAdmin:      1,
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Session
	sessionID := uuid.New().String()
	_, _ = queries.CreateSession(ctx, db.CreateSessionParams{
		ID:        sessionID,
		UserID:    userID,
		AuthCode:  "",
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	})
	_, _ = queries.GetSession(ctx, sessionID)
	_ = queries.DeleteSession(ctx, sessionID)

	// Sync Locks
	lockKey := "lock1"
	_, _ = queries.SetSyncLock(ctx, db.SetSyncLockParams{
		LockKey:  lockKey,
		LockType: 1,
		LockData: "{}",
	})
	_, _ = queries.GetSyncLock(ctx, lockKey)
	_, _ = queries.ListSyncLocksByType(ctx, 1)
	_ = queries.DeleteSyncLock(ctx, lockKey)

	// Shares
	shareID := uuid.New().String()
	_, _ = queries.CreateShare(ctx, db.CreateShareParams{
		ID:        shareID,
		OwnerID:   userID,
		FolderID:  "folder1",
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	})
	_, _ = queries.CreateUserShare(ctx, db.CreateUserShareParams{
		ShareID:   shareID,
		UserID:    userID,
		Status:    1,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	})

	// Sync Items
	itemID := uuid.New().String()
	_, _ = queries.UpsertSyncItem(ctx, db.UpsertSyncItemParams{
		ID:              itemID,
		FileName:        "file1",
		MimeType:        "text/plain",
		JoplinID:        itemID,
		ParentID:        "",
		ShareID:         shareID,
		ItemType:        1,
		IsEncrypted:     0,
		ClientUpdatedAt: time.Now().UnixMilli(),
		OwnerID:         userID,
		CreatedAt:       time.Now().UnixMilli(),
		UpdatedAt:       time.Now().UnixMilli(),
	})
	_, _ = queries.UpsertUserSyncItem(ctx, db.UpsertUserSyncItemParams{
		UserID:     userID,
		SyncItemID: itemID,
		CreatedAt:  time.Now().UnixMilli(),
		UpdatedAt:  time.Now().UnixMilli(),
	})

	_, _ = queries.GetSyncItemByFileNameAndUser(ctx, db.GetSyncItemByFileNameAndUserParams{
		FileName: "file1",
		UserID:   userID,
	})
	_, _ = queries.ListSyncItemsByUser(ctx, db.ListSyncItemsByUserParams{
		UserID: userID,
		Limit:  10,
		Offset: 0,
	})

	// Delta Events
	_, _ = queries.InsertDeltaEvent(ctx, db.InsertDeltaEventParams{
		EventUuid:       uuid.New().String(),
		JoplinID:        itemID,
		UserID:          userID,
		FileName:        "file1",
		PreviousShareID: shareID,
		ItemType:        1,
		EventType:       1,
		CreatedAt:       time.Now().UnixMilli(),
		UpdatedAt:       time.Now().UnixMilli(),
	})
	_, _ = queries.GetDeltaEventsByUser(ctx, db.GetDeltaEventsByUserParams{
		UserID: userID,
		ID:     0,
		Limit:  10,
	})
	_ = queries.InsertShareTombstonesForDeletedUser(ctx, db.InsertShareTombstonesForDeletedUserParams{
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
		OwnerID:   userID,
	})

	// Stats
	_, _ = queries.GetInstanceStats(ctx)
	_, _ = queries.GetUserStats(ctx)

	// Clean up user and items
	_ = queries.DeleteUserSyncItem(ctx, db.DeleteUserSyncItemParams{
		UserID:     userID,
		SyncItemID: itemID,
	})
	_ = queries.DeleteSyncItemByFileNameAndUser(ctx, db.DeleteSyncItemByFileNameAndUserParams{
		FileName: "file1",
		UserID:   userID,
	})
	_ = queries.DeleteUser(ctx, userID)
	_, _ = queries.CountUsers(ctx)
}
