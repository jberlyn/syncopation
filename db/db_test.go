package db

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupDB(t *testing.T) (*Queries, *sql.DB) {
	dbConn, err := sql.Open("sqlite3", ":memory:?_fk=1")
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}

	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}

	if _, err := dbConn.Exec(string(schema)); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	return New(dbConn), dbConn
}

func TestQueries(t *testing.T) {
	queries, dbConn := setupDB(t)
	defer dbConn.Close()
	ctx := context.Background()

	// 1. Users
	userID := uuid.New().String()
	user, err := queries.CreateUser(ctx, CreateUserParams{
		ID:           userID,
		Email:        "test@example.com",
		PasswordHash: "password123",
		IsAdmin:      0,
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.ID != userID {
		t.Fatalf("CreateUser returned wrong ID")
	}

	fetchedUser, err := queries.GetUserByEmail(ctx, "test@example.com")
	if err != nil || fetchedUser.ID != userID {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}

	// 2. Items
	itemID := uuid.New().String()
	item, err := queries.UpsertSyncItem(ctx, UpsertSyncItemParams{
		ID:              itemID,
		FileName:        "item1",
		MimeType:        "",
		JoplinID:        "jop1",
		ParentID:        "",
		ShareID:         "",
		ItemType:        1,
		IsEncrypted:     0,
		ClientUpdatedAt: 100,
		OwnerID:         userID,
		UpdatedAt:       100,
		CreatedAt:       100,
	})
	if err != nil {
		t.Fatalf("UpsertSyncItem failed: %v", err)
	}
	if item.ID != itemID {
		t.Fatalf("UpsertSyncItem returned wrong ID")
	}

	_, err = queries.UpsertUserSyncItem(ctx, UpsertUserSyncItemParams{
		UserID:     userID,
		SyncItemID: itemID,
	})
	if err != nil {
		t.Fatalf("UpsertUserSyncItem failed: %v", err)
	}

	// Get item
	fetchedItem, err := queries.GetSyncItemByFileNameAndUser(ctx, GetSyncItemByFileNameAndUserParams{
		FileName: "item1",
		UserID:   userID,
	})
	if err != nil || fetchedItem.ID != itemID {
		t.Fatalf("GetSyncItemByFileNameAndUser failed: %v", err)
	}

	items, err := queries.ListSyncItemsByUser(ctx, ListSyncItemsByUserParams{
		UserID: userID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil || len(items) != 1 {
		t.Fatalf("ListSyncItemsByUser failed: %v", err)
	}

	_, err = queries.InsertDeltaEvent(ctx, InsertDeltaEventParams{
		UserID:          userID,
		EventUuid:       "event-1",
		JoplinID:        "jop1",
		FileName:        "item1",
		ItemType:        1,
		EventType:       1,
		UpdatedAt:       100,
		CreatedAt:       100,
		PreviousShareID: "",
	})
	if err != nil {
		t.Fatalf("InsertDeltaEvent failed: %v", err)
	}

	changes, err := queries.GetDeltaEventsByUser(ctx, GetDeltaEventsByUserParams{
		UserID: userID,
		ID:     0,
		Limit:  10,
	})
	if err != nil || len(changes) != 1 {
		t.Fatalf("GetDeltaEventsByUser failed: %v", err)
	}

	err = queries.DeleteUserSyncItem(ctx, DeleteUserSyncItemParams{
		UserID:     userID,
		SyncItemID: itemID,
	})
	if err != nil {
		t.Fatalf("DeleteUserSyncItem failed: %v", err)
	}
	err = queries.DeleteSyncItemByFileNameAndUser(ctx, DeleteSyncItemByFileNameAndUserParams{
		FileName: "item1",
		UserID:   userID,
	})
	if err != nil {
		t.Fatalf("DeleteSyncItemByFileNameAndUser failed: %v", err)
	}

	_, err = queries.SetSyncLock(ctx, SetSyncLockParams{
		LockKey:  "lock_1",
		LockData: "data",
		LockType: 1,
	})
	if err != nil {
		t.Fatalf("SetSyncLock failed: %v", err)
	}

	val, err := queries.GetSyncLock(ctx, "lock_1")
	if err != nil || val.LockData != "data" {
		t.Fatalf("GetSyncLock failed: %v", err)
	}

	kvs, err := queries.ListSyncLocksByType(ctx, 1)
	if err != nil || len(kvs) != 1 {
		t.Fatalf("ListSyncLocksByType failed: %v", err)
	}

	err = queries.DeleteSyncLock(ctx, "lock_1")
	if err != nil {
		t.Fatalf("DeleteSyncLock failed: %v", err)
	}

	// 4. Session
	sessionID := uuid.New().String()
	_, err = queries.CreateSession(ctx, CreateSessionParams{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	sess, err := queries.GetSession(ctx, sessionID)
	if err != nil || sess.UserID != userID {
		t.Fatalf("GetSession failed: %v", err)
	}

	err = queries.DeleteSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
}
