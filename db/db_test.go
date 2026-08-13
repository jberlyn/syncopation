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
	dbConn, err := sql.Open("sqlite3", ":memory:")
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
		ID:          userID,
		Email:       "test@example.com",
		Password:    "password123",
		IsAdmin:     0,
		CreatedTime: time.Now().UnixMilli(),
		UpdatedTime: time.Now().UnixMilli(),
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
	item, err := queries.UpsertItem(ctx, UpsertItemParams{
		ID:                   itemID,
		Name:                 "item1",
		MimeType:             "",
		JopID:                "jop1",
		JopParentID:          "",
		JopShareID:           "",
		JopType:              1,
		JopEncryptionApplied: 0,
		JopUpdatedTime:       100,
		UpdatedTime:          100,
		CreatedTime:          100,
	})
	if err != nil {
		t.Fatalf("UpsertItem failed: %v", err)
	}
	if item.ID != itemID {
		t.Fatalf("UpsertItem returned wrong ID")
	}

	_, err = queries.UpsertUserItem(ctx, UpsertUserItemParams{
		UserID: userID,
		ItemID: itemID,
	})
	if err != nil {
		t.Fatalf("UpsertUserItem failed: %v", err)
	}

	// Get item
	fetchedItem, err := queries.GetItemByNameAndUser(ctx, GetItemByNameAndUserParams{
		Name:   "item1",
		UserID: userID,
	})
	if err != nil || fetchedItem.ID != itemID {
		t.Fatalf("GetItemByNameAndUser failed: %v", err)
	}

	// ListItemsByUser
	items, err := queries.ListItemsByUser(ctx, ListItemsByUserParams{
		UserID: userID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil || len(items) != 1 {
		t.Fatalf("ListItemsByUser failed: %v", err)
	}

	_, err = queries.InsertChange(ctx, InsertChangeParams{
		UserID:      userID,
		ItemName:    "item1",
		ItemType:    1,
		Type:        1,
		UpdatedTime: 100,
	})
	if err != nil {
		t.Fatalf("InsertChange failed: %v", err)
	}

	// GetChangesByUser
	changes, err := queries.GetChangesByUser(ctx, GetChangesByUserParams{
		UserID:  userID,
		Counter: 0,
		Limit:   10,
	})
	if err != nil || len(changes) != 1 {
		t.Fatalf("GetChangesByUser failed: %v", err)
	}

	// Delete item
	err = queries.DeleteUserItem(ctx, DeleteUserItemParams{
		UserID: userID,
		ItemID: itemID,
	})
	if err != nil {
		t.Fatalf("DeleteUserItem failed: %v", err)
	}
	err = queries.DeleteItemByNameAndUser(ctx, DeleteItemByNameAndUserParams{
		Name:   "item1",
		UserID: userID,
	})
	if err != nil {
		t.Fatalf("DeleteItemByNameAndUser failed: %v", err)
	}

	// 3. KeyValue
	_, err = queries.SetKeyValue(ctx, SetKeyValueParams{
		Key:   "lock_1",
		Value: "data",
		Type:  1,
	})
	if err != nil {
		t.Fatalf("SetKeyValue failed: %v", err)
	}

	val, err := queries.GetKeyValue(ctx, "lock_1")
	if err != nil || val.Value != "data" {
		t.Fatalf("GetKeyValue failed: %v", err)
	}

	kvs, err := queries.ListKeyValuesByType(ctx, 1)
	if err != nil || len(kvs) != 1 {
		t.Fatalf("ListKeyValuesByType failed: %v", err)
	}

	err = queries.DeleteKeyValue(ctx, "lock_1")
	if err != nil {
		t.Fatalf("DeleteKeyValue failed: %v", err)
	}

	// 4. Session
	sessionID := uuid.New().String()
	_, err = queries.CreateSession(ctx, CreateSessionParams{
		ID:          sessionID,
		UserID:      userID,
		CreatedTime: time.Now().UnixMilli(),
		UpdatedTime: time.Now().UnixMilli(),
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
