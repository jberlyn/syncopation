package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/storage"
)

type BatchItemHandler struct {
	Queries *db.Queries
	DB      *sql.DB
	Storage storage.Storage
}

type BatchPutRequest struct {
	Items []BatchPutItem `json:"items"`
}

type BatchPutItem struct {
	Name string `json:"name"`
	Body string `json:"body"` // Usually raw markdown text for batch uploads
}

type BatchDeleteRequest struct {
	Items []string `json:"items"`
}

type BatchPutResponse struct {
	Items   map[string]BatchPutResult `json:"items"`
	HasMore bool                      `json:"has_more"`
}

type BatchPutResult struct {
	Item  *ItemMetadataResponse `json:"item,omitempty"`
	Error *string               `json:"error"`
}

type BatchDeleteResponse struct {
	Items   map[string]struct{} `json:"items"`
	HasMore bool                `json:"has_more"`
}

func (h *BatchItemHandler) HandleBatchItems(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.handlePutBatch(w, r, userID)
	case http.MethodDelete:
		h.handleDeleteBatch(w, r, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *BatchItemHandler) handlePutBatch(w http.ResponseWriter, r *http.Request, userID string) {
	var req BatchPutRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback() }()

	qtx := h.Queries.WithTx(tx)
	now := time.Now().UnixMilli()
	results := make(map[string]BatchPutResult)

	for _, reqItem := range req.Items {
		itemName := reqItem.Name
		content := []byte(reqItem.Body)

		existing, err := qtx.GetSyncItemByFileNameAndUser(ctx, db.GetSyncItemByFileNameAndUserParams{
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
			errStr := err.Error()
			results[itemName] = BatchPutResult{Error: &errStr}
			continue
		} else {
			itemID = existing.ID
			createdTime = existing.CreatedAt
			changeType = 2 // Update
		}

		if err := h.Storage.WriteItem(ctx, userID, itemName, content); err != nil {
			errStr := err.Error()
			results[itemName] = BatchPutResult{Error: &errStr}
			continue
		}

		item, err := qtx.UpsertSyncItem(ctx, db.UpsertSyncItemParams{
			ID:              itemID,
			FileName:        itemName,
			MimeType:        "text/plain", // Default to text/plain for batch notes
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
			errStr := err.Error()
			results[itemName] = BatchPutResult{Error: &errStr}
			continue
		}

		_, err = qtx.UpsertUserSyncItem(ctx, db.UpsertUserSyncItemParams{
			UserID:     userID,
			SyncItemID: itemID,
			CreatedAt:  createdTime,
			UpdatedAt:  now,
		})
		if err != nil {
			errStr := err.Error()
			results[itemName] = BatchPutResult{Error: &errStr}
			continue
		}

		_, err = qtx.InsertDeltaEvent(ctx, db.InsertDeltaEventParams{
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
			errStr := err.Error()
			results[itemName] = BatchPutResult{Error: &errStr}
			continue
		}

		results[itemName] = BatchPutResult{
			Item: &ItemMetadataResponse{
				ID:             item.ID,
				Name:           item.FileName,
				MimeType:       item.MimeType,
				UpdatedTime:    item.UpdatedAt,
				JopUpdatedTime: item.ClientUpdatedAt,
			},
			Error: nil,
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	resp := BatchPutResponse{
		Items:   results,
		HasMore: false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *BatchItemHandler) handleDeleteBatch(w http.ResponseWriter, r *http.Request, userID string) {
	var req BatchDeleteRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback() }()

	qtx := h.Queries.WithTx(tx)
	now := time.Now().UnixMilli()
	results := make(map[string]struct{})

	for _, itemName := range req.Items {
		existing, err := qtx.GetSyncItemByFileNameAndUser(ctx, db.GetSyncItemByFileNameAndUserParams{
			FileName: itemName,
			UserID:   userID,
		})
		if err != nil {
			if err == sql.ErrNoRows {
				results[itemName] = struct{}{}
				continue
			}
			// Skip item if DB error other than Not Found
			continue
		}

		err = qtx.DeleteSyncItemByFileNameAndUser(ctx, db.DeleteSyncItemByFileNameAndUserParams{
			FileName: itemName,
			UserID:   userID,
		})
		if err != nil {
			continue
		}

		err = qtx.DeleteUserSyncItem(ctx, db.DeleteUserSyncItemParams{
			UserID:     userID,
			SyncItemID: existing.ID,
		})
		if err != nil {
			continue
		}

		_, err = qtx.InsertDeltaEvent(ctx, db.InsertDeltaEventParams{
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
			continue
		}

		_ = h.Storage.DeleteItem(ctx, userID, itemName)

		results[itemName] = struct{}{}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	resp := BatchDeleteResponse{
		Items:   results,
		HasMore: false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
