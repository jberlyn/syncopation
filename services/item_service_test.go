package services_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/services"
	"github.com/jberlyn/syncopation/storage"
)

func TestItemService_GetStat(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				if arg.FileName == "test.md" && arg.UserID == "user1" {
					return db.SyncItem{ID: "item1", FileName: "test.md"}, nil
				}
				return db.SyncItem{}, sql.ErrNoRows
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		item, err := svc.GetStat(ctx, "user1", "test.md")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if item.ID != "item1" {
			t.Errorf("expected item ID 'item1', got %s", item.ID)
		}
		if len(mockRepo.GetSyncItemByFileNameAndUserCalls()) != 1 {
			t.Errorf("expected 1 call to GetSyncItemByFileNameAndUser")
		}
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{}, sql.ErrNoRows
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		_, err := svc.GetStat(ctx, "user1", "missing.md")

		if err != sql.ErrNoRows {
			t.Fatalf("expected sql.ErrNoRows, got %v", err)
		}
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("database connection failed")
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{}, dbErr
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		_, err := svc.GetStat(ctx, "user1", "test.md")

		if err != dbErr {
			t.Fatalf("expected specific db error, got %v", err)
		}
	})
}

func TestItemService_PutContent(t *testing.T) {
	ctx := context.Background()
	content := []byte("hello world")

	t.Run("create new item success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{}, sql.ErrNoRows // Does not exist yet
			},
			UpsertSyncItemFunc: func(ctx context.Context, arg db.UpsertSyncItemParams) (db.SyncItem, error) {
				return db.SyncItem{ID: arg.ID, FileName: arg.FileName}, nil
			},
			UpsertUserSyncItemFunc: func(ctx context.Context, arg db.UpsertUserSyncItemParams) (db.UserSyncItem, error) {
				return db.UserSyncItem{}, nil
			},
			InsertDeltaEventFunc: func(ctx context.Context, arg db.InsertDeltaEventParams) (db.DeltaEvent, error) {
				if arg.EventType != 1 {
					t.Errorf("expected EventType 1 (Create), got %d", arg.EventType)
				}
				return db.DeltaEvent{}, nil
			},
		}

		mockStorage := &storage.StorageMock{
			WriteItemFunc: func(ctx context.Context, userID string, itemName string, content []byte) error {
				return nil
			},
		}

		svc := services.NewItemService(mockRepo, mockStorage)
		item, err := svc.PutContent(ctx, "user1", "new.md", content, "text/markdown")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if item.FileName != "new.md" {
			t.Errorf("expected item FileName 'new.md', got %s", item.FileName)
		}

		if len(mockStorage.WriteItemCalls()) != 1 {
			t.Errorf("expected WriteItem to be called once")
		}
	})

	t.Run("update existing item success", func(t *testing.T) {
		existingID := "existing-id"
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{ID: existingID, FileName: "existing.md"}, nil // Exists!
			},
			UpsertSyncItemFunc: func(ctx context.Context, arg db.UpsertSyncItemParams) (db.SyncItem, error) {
				if arg.ID != existingID {
					t.Errorf("expected UpsertSyncItem to use existing ID %s, got %s", existingID, arg.ID)
				}
				return db.SyncItem{ID: arg.ID, FileName: arg.FileName}, nil
			},
			UpsertUserSyncItemFunc: func(ctx context.Context, arg db.UpsertUserSyncItemParams) (db.UserSyncItem, error) {
				return db.UserSyncItem{}, nil
			},
			InsertDeltaEventFunc: func(ctx context.Context, arg db.InsertDeltaEventParams) (db.DeltaEvent, error) {
				if arg.EventType != 2 {
					t.Errorf("expected EventType 2 (Update), got %d", arg.EventType)
				}
				return db.DeltaEvent{}, nil
			},
		}

		mockStorage := &storage.StorageMock{
			WriteItemFunc: func(ctx context.Context, userID string, itemName string, content []byte) error {
				return nil
			},
		}

		svc := services.NewItemService(mockRepo, mockStorage)
		_, err := svc.PutContent(ctx, "user1", "existing.md", content, "text/markdown")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("storage failure", func(t *testing.T) {
		storageErr := errors.New("disk full")
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{}, sql.ErrNoRows
			},
		}

		mockStorage := &storage.StorageMock{
			WriteItemFunc: func(ctx context.Context, userID string, itemName string, content []byte) error {
				return storageErr
			},
		}

		svc := services.NewItemService(mockRepo, mockStorage)
		_, err := svc.PutContent(ctx, "user1", "fail.md", content, "text/markdown")

		if err != storageErr {
			t.Fatalf("expected storage error, got %v", err)
		}

		// Ensure we didn't try to save to DB since storage failed
		if len(mockRepo.UpsertSyncItemCalls()) != 0 {
			t.Errorf("expected no DB inserts when storage fails")
		}
	})
}

func TestItemService_GetContent(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{ID: "123", FileName: arg.FileName}, nil
			},
		}

		mockStorage := &storage.StorageMock{
			ReadItemFunc: func(ctx context.Context, userID string, itemName string) ([]byte, error) {
				return []byte("data"), nil
			},
		}

		svc := services.NewItemService(mockRepo, mockStorage)
		item, content, err := svc.GetContent(ctx, "u1", "test.md")

		if err != nil {
			t.Fatal(err)
		}
		if item.ID != "123" {
			t.Errorf("expected ID 123, got %s", item.ID)
		}
		if string(content) != "data" {
			t.Errorf("expected 'data', got %s", string(content))
		}
	})

	t.Run("not found in db", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{}, sql.ErrNoRows
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		_, _, err := svc.GetContent(ctx, "u1", "test.md")

		if err != sql.ErrNoRows {
			t.Errorf("expected sql.ErrNoRows, got %v", err)
		}
	})

	t.Run("storage read error", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{ID: "123"}, nil
			},
		}

		readErr := errors.New("read error")
		mockStorage := &storage.StorageMock{
			ReadItemFunc: func(ctx context.Context, userID string, itemName string) ([]byte, error) {
				return nil, readErr
			},
		}

		svc := services.NewItemService(mockRepo, mockStorage)
		_, _, err := svc.GetContent(ctx, "u1", "test.md")

		if err != readErr {
			t.Errorf("expected %v, got %v", readErr, err)
		}
	})
}

func TestItemService_GetDelta(t *testing.T) {
	ctx := context.Background()

	t.Run("success with more", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetDeltaEventsByUserFunc: func(ctx context.Context, arg db.GetDeltaEventsByUserParams) ([]db.DeltaEvent, error) {
				return []db.DeltaEvent{
					{ID: 1}, {ID: 2}, {ID: 3},
				}, nil
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		events, hasMore, lastCursor, err := svc.GetDelta(ctx, "u1", 0, 2)

		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 2 {
			t.Errorf("expected 2 events due to limit, got %d", len(events))
		}
		if !hasMore {
			t.Error("expected hasMore to be true")
		}
		if lastCursor != 2 {
			t.Errorf("expected lastCursor 2, got %d", lastCursor)
		}
	})

	t.Run("success no more", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetDeltaEventsByUserFunc: func(ctx context.Context, arg db.GetDeltaEventsByUserParams) ([]db.DeltaEvent, error) {
				return []db.DeltaEvent{
					{ID: 1},
				}, nil
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		events, hasMore, lastCursor, err := svc.GetDelta(ctx, "u1", 0, 2)

		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
		if hasMore {
			t.Error("expected hasMore to be false")
		}
		if lastCursor != 1 {
			t.Errorf("expected lastCursor 1, got %d", lastCursor)
		}
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("db error")
		mockRepo := &db.QuerierMock{
			GetDeltaEventsByUserFunc: func(ctx context.Context, arg db.GetDeltaEventsByUserParams) ([]db.DeltaEvent, error) {
				return nil, dbErr
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		_, _, _, err := svc.GetDelta(ctx, "u1", 0, 2)

		if err != dbErr {
			t.Errorf("expected db error")
		}
	})
}

func TestItemService_GetChildren(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			ListSyncItemsByUserFunc: func(ctx context.Context, arg db.ListSyncItemsByUserParams) ([]db.SyncItem, error) {
				return []db.SyncItem{
					{ID: "a"}, {ID: "b"},
				}, nil
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		items, hasMore, cursor, err := svc.GetChildren(ctx, "u1", 0, 2)

		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if hasMore {
			t.Error("expected hasMore to be false")
		}
		if cursor != 2 {
			t.Errorf("expected cursor 2, got %d", cursor)
		}
	})

	t.Run("success with more", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			ListSyncItemsByUserFunc: func(ctx context.Context, arg db.ListSyncItemsByUserParams) ([]db.SyncItem, error) {
				return []db.SyncItem{
					{ID: "a"}, {ID: "b"}, {ID: "c"},
				}, nil
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		items, hasMore, cursor, err := svc.GetChildren(ctx, "u1", 0, 2)

		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if !hasMore {
			t.Error("expected hasMore to be true")
		}
		if cursor != 2 {
			t.Errorf("expected cursor 2, got %d", cursor)
		}
	})

	t.Run("db error", func(t *testing.T) {
		dbErr := errors.New("db error")
		mockRepo := &db.QuerierMock{
			ListSyncItemsByUserFunc: func(ctx context.Context, arg db.ListSyncItemsByUserParams) ([]db.SyncItem, error) {
				return nil, dbErr
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		_, _, _, err := svc.GetChildren(ctx, "u1", 0, 2)

		if err != dbErr {
			t.Errorf("expected db error")
		}
	})
}

func TestItemService_DeleteItem(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{ID: "123"}, nil
			},
			DeleteSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.DeleteSyncItemByFileNameAndUserParams) error {
				return nil
			},
			DeleteUserSyncItemFunc: func(ctx context.Context, arg db.DeleteUserSyncItemParams) error {
				return nil
			},
			InsertDeltaEventFunc: func(ctx context.Context, arg db.InsertDeltaEventParams) (db.DeltaEvent, error) {
				if arg.EventType != 3 {
					t.Errorf("expected EventType 3, got %d", arg.EventType)
				}
				return db.DeltaEvent{}, nil
			},
		}

		mockStorage := &storage.StorageMock{
			DeleteItemFunc: func(ctx context.Context, userID string, itemName string) error {
				return nil
			},
		}

		svc := services.NewItemService(mockRepo, mockStorage)
		err := svc.DeleteItem(ctx, "u1", "test.md")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{}, sql.ErrNoRows
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		err := svc.DeleteItem(ctx, "u1", "test.md")

		// If it's already not there, deletion silently succeeds
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("db deletion error", func(t *testing.T) {
		dbErr := errors.New("db error")
		mockRepo := &db.QuerierMock{
			GetSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.GetSyncItemByFileNameAndUserParams) (db.SyncItem, error) {
				return db.SyncItem{ID: "123"}, nil
			},
			DeleteSyncItemByFileNameAndUserFunc: func(ctx context.Context, arg db.DeleteSyncItemByFileNameAndUserParams) error {
				return dbErr
			},
		}

		svc := services.NewItemService(mockRepo, nil)
		err := svc.DeleteItem(ctx, "u1", "test.md")

		if err != dbErr {
			t.Errorf("expected db error")
		}
	})
}
