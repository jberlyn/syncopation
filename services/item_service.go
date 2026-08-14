package services

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/storage"
)

type ItemService struct {
	repo    db.Querier
	storage storage.Storage
}

func NewItemService(repo db.Querier, storage storage.Storage) *ItemService {
	return &ItemService{
		repo:    repo,
		storage: storage,
	}
}

func (s *ItemService) GetStat(ctx context.Context, userID, itemName string) (*db.SyncItem, error) {
	item, err := s.repo.GetSyncItemByFileNameAndUser(ctx, db.GetSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ItemService) GetContent(ctx context.Context, userID, itemName string) (*db.SyncItem, []byte, error) {
	item, err := s.repo.GetSyncItemByFileNameAndUser(ctx, db.GetSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})
	if err != nil {
		return nil, nil, err
	}

	content, err := s.storage.ReadItem(ctx, userID, itemName)
	if err != nil {
		return nil, nil, err
	}

	return &item, content, nil
}

func (s *ItemService) GetDelta(ctx context.Context, userID string, cursor int64, limit int64) ([]db.DeltaEvent, bool, int64, error) {
	changes, err := s.repo.GetDeltaEventsByUser(ctx, db.GetDeltaEventsByUserParams{
		UserID: userID,
		ID:     cursor,
		Limit:  limit + 1,
	})
	if err != nil {
		return nil, false, 0, err
	}

	hasMore := false
	if len(changes) > int(limit) {
		hasMore = true
		changes = changes[:limit]
	}

	var lastCursor int64 = cursor
	if len(changes) > 0 {
		lastCursor = changes[len(changes)-1].ID
	}

	return changes, hasMore, lastCursor, nil
}

func (s *ItemService) GetChildren(ctx context.Context, userID string, cursor int64, limit int64) ([]db.SyncItem, bool, int64, error) {
	items, err := s.repo.ListSyncItemsByUser(ctx, db.ListSyncItemsByUserParams{
		UserID: userID,
		Limit:  limit + 1,
		Offset: cursor,
	})
	if err != nil {
		return nil, false, 0, err
	}

	hasMore := false
	if len(items) > int(limit) {
		hasMore = true
		items = items[:limit]
	}

	return items, hasMore, cursor + limit, nil
}

func (s *ItemService) PutContent(ctx context.Context, userID, itemName string, content []byte, contentType string) (*db.SyncItem, error) {
	now := time.Now().UnixMilli()

	existing, err := s.repo.GetSyncItemByFileNameAndUser(ctx, db.GetSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})

	var itemID string
	var createdTime int64
	var changeType int64

	if err == sql.ErrNoRows {
		itemID = uuid.New().String()
		createdTime = now
		changeType = 1 // Create
	} else if err != nil {
		return nil, err
	} else {
		itemID = existing.ID
		createdTime = existing.CreatedAt
		changeType = 2 // Update
	}

	if err := s.storage.WriteItem(ctx, userID, itemName, content); err != nil {
		return nil, err
	}

	item, err := s.repo.UpsertSyncItem(ctx, db.UpsertSyncItemParams{
		ID:              itemID,
		FileName:        itemName,
		MimeType:        contentType,
		JoplinID:        "",
		ParentID:        "",
		ShareID:         "",
		ItemType:        1,
		IsEncrypted:     0,
		ClientUpdatedAt: now,
		OwnerID:         userID,
		CreatedAt:       createdTime,
		UpdatedAt:       now,
	})
	if err != nil {
		return nil, err
	}

	_, err = s.repo.UpsertUserSyncItem(ctx, db.UpsertUserSyncItemParams{
		UserID:     userID,
		SyncItemID: itemID,
		CreatedAt:  createdTime,
		UpdatedAt:  now,
	})
	if err != nil {
		return nil, err
	}

	_, err = s.repo.InsertDeltaEvent(ctx, db.InsertDeltaEventParams{
		EventUuid:       uuid.New().String(),
		JoplinID:        itemID,
		UserID:          userID,
		FileName:        itemName,
		PreviousShareID: "",
		ItemType:        1,
		EventType:       changeType,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (s *ItemService) DeleteItem(ctx context.Context, userID, itemName string) error {
	existing, err := s.repo.GetSyncItemByFileNameAndUser(ctx, db.GetSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	err = s.repo.DeleteSyncItemByFileNameAndUser(ctx, db.DeleteSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})
	if err != nil {
		return err
	}

	err = s.repo.DeleteUserSyncItem(ctx, db.DeleteUserSyncItemParams{
		UserID:     userID,
		SyncItemID: existing.ID,
	})
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	_, err = s.repo.InsertDeltaEvent(ctx, db.InsertDeltaEventParams{
		EventUuid:       uuid.New().String(),
		JoplinID:        existing.ID,
		UserID:          userID,
		FileName:        itemName,
		PreviousShareID: "",
		ItemType:        1,
		EventType:       3, // Delete
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return err
	}

	return s.storage.DeleteItem(ctx, userID, itemName)
}
