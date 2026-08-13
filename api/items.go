package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jberlyn/syncopation/db"
	"github.com/jberlyn/syncopation/storage"
)

type ItemHandler struct {
	Queries *db.Queries
	Storage storage.Storage
}

type ItemMetadataResponse struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	MimeType       string         `json:"mime_type"`
	UpdatedTime    int64          `json:"updated_time"`
	JopUpdatedTime int64          `json:"jop_updated_time"`
	JopItem        map[string]any `json:"jopItem,omitempty"`
}

type DeltaResponse struct {
	Items   []DeltaItem `json:"items"`
	HasMore bool        `json:"has_more"`
	Cursor  string      `json:"cursor"`
}

type DeltaItem struct {
	ItemName    string `json:"item_name"`
	ItemType    int64  `json:"item_type,omitempty"`
	Type        int64  `json:"type"`
	UpdatedTime int64  `json:"updated_time"`
}

func parseItemPath(p string) (string, string, bool) {
	if strings.HasPrefix(p, "/api/items/root:/") {
		rest := p[len("/api/items/root:/"):]
		if strings.HasSuffix(rest, ":/content") {
			return rest[:len(rest)-len(":/content")], "content", true
		}
		if strings.HasSuffix(rest, ":/delta") {
			return rest[:len(rest)-len(":/delta")], "delta", true
		}
		if strings.HasSuffix(rest, "*:/children") {
			// e.g. path/*:/children -> path. But for root:/*:/children, rest is *:/children -> empty path.
			// Actually, let's just strip the suffix. If there's a trailing slash, strip it.
			path := rest[:len(rest)-len("*:/children")]
			path = strings.TrimSuffix(path, "/")
			return path, "children", true
		}
		if strings.HasSuffix(rest, ":") {
			return rest[:len(rest)-1], "stat", true
		}
	} else if strings.HasPrefix(p, "/api/items/root::") {
		rest := p[len("/api/items/root::"):]
		if rest == "/delta" {
			return "", "delta", true
		}
		if rest == "/*:/children" || rest == "*:/children" {
			return "", "children", true
		}
		if rest == "" {
			return "", "stat", true
		}
	}
	return "", "", false
}

func (h *ItemHandler) HandleItems(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itemName, action, valid := parseItemPath(r.URL.Path)
	if !valid {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if action == "content" {
			h.handleGetContent(w, r, userID, itemName)
		} else if action == "delta" {
			h.handleGetDelta(w, r, userID)
		} else if action == "children" {
			h.handleGetChildren(w, r, userID)
		} else {
			h.handleGetStat(w, r, userID, itemName)
		}
	case http.MethodPut:
		if action != "content" {
			http.Error(w, "PUT only supported for /content", http.StatusBadRequest)
			return
		}
		h.handlePutContent(w, r, userID, itemName)
	case http.MethodDelete:
		if action != "stat" {
			http.Error(w, "DELETE only supported for metadata endpoint", http.StatusBadRequest)
			return
		}
		h.handleDelete(w, r, userID, itemName)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ItemHandler) handleGetStat(w http.ResponseWriter, r *http.Request, userID, itemName string) {
	item, err := h.Queries.GetSyncItemByFileNameAndUser(r.Context(), db.GetSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := ItemMetadataResponse{
		ID:             item.ID,
		Name:           item.FileName,
		MimeType:       item.MimeType,
		UpdatedTime:    item.UpdatedAt,
		JopUpdatedTime: item.ClientUpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *ItemHandler) handleGetContent(w http.ResponseWriter, r *http.Request, userID, itemName string) {
	item, err := h.Queries.GetSyncItemByFileNameAndUser(r.Context(), db.GetSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	content, err := h.Storage.ReadItem(r.Context(), userID, itemName)
	if err != nil {
		http.Error(w, "Failed to read content", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", item.MimeType)
	_, _ = w.Write(content)
}

func (h *ItemHandler) handleGetDelta(w http.ResponseWriter, r *http.Request, userID string) {
	cursorStr := r.URL.Query().Get("cursor")
	cursor := int64(0)
	if cursorStr != "" {
		c, err := strconv.ParseInt(cursorStr, 10, 64)
		if err == nil {
			cursor = c
		}
	}

	limit := int64(100)

	changes, err := h.Queries.GetDeltaEventsByUser(r.Context(), db.GetDeltaEventsByUserParams{
		UserID: userID,
		ID:     cursor,
		Limit:  limit + 1,
	})
	if err != nil {
		http.Error(w, "Failed to fetch changes", http.StatusInternalServerError)
		return
	}

	hasMore := false
	if len(changes) > int(limit) {
		hasMore = true
		changes = changes[:limit]
	}

	deltaItems := make([]DeltaItem, 0, len(changes))
	lastCursor := cursor
	for _, c := range changes {
		item := DeltaItem{
			ItemName:    c.FileName,
			ItemType:    c.ItemType,
			Type:        c.EventType,
			UpdatedTime: c.UpdatedAt,
		}
		deltaItems = append(deltaItems, item)
		lastCursor = c.ID
	}

	resp := DeltaResponse{
		Items:   deltaItems,
		HasMore: hasMore,
		Cursor:  strconv.FormatInt(lastCursor, 10),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *ItemHandler) handleGetChildren(w http.ResponseWriter, r *http.Request, userID string) {
	cursorStr := r.URL.Query().Get("cursor")
	cursor := int64(0)
	if cursorStr != "" {
		c, err := strconv.ParseInt(cursorStr, 10, 64)
		if err == nil {
			cursor = c
		}
	}

	limit := int64(100)

	items, err := h.Queries.ListSyncItemsByUser(r.Context(), db.ListSyncItemsByUserParams{
		UserID: userID,
		Limit:  limit + 1,
		Offset: cursor,
	})
	if err != nil {
		http.Error(w, "Failed to fetch items", http.StatusInternalServerError)
		return
	}

	hasMore := false
	if len(items) > int(limit) {
		hasMore = true
		items = items[:limit]
	}

	childrenItems := make([]ItemMetadataResponse, 0, len(items))
	for _, item := range items {
		childrenItems = append(childrenItems, ItemMetadataResponse{
			ID:             item.ID,
			Name:           item.FileName,
			MimeType:       item.MimeType,
			UpdatedTime:    item.UpdatedAt,
			JopUpdatedTime: item.ClientUpdatedAt,
		})
	}

	resp := struct {
		Items   []ItemMetadataResponse `json:"items"`
		HasMore bool                   `json:"has_more"`
		Cursor  string                 `json:"cursor"`
	}{
		Items:   childrenItems,
		HasMore: hasMore,
		Cursor:  strconv.FormatInt(cursor+limit, 10),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *ItemHandler) handlePutContent(w http.ResponseWriter, r *http.Request, userID, itemName string) {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	now := time.Now().UnixMilli()

	existing, err := h.Queries.GetSyncItemByFileNameAndUser(r.Context(), db.GetSyncItemByFileNameAndUserParams{
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
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	} else {
		itemID = existing.ID
		createdTime = existing.CreatedAt
		changeType = 2 // Update
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.Storage.WriteItem(r.Context(), userID, itemName, content); err != nil {
		http.Error(w, "Failed to save item content", http.StatusInternalServerError)
		return
	}

	item, err := h.Queries.UpsertSyncItem(r.Context(), db.UpsertSyncItemParams{
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
		http.Error(w, "Failed to save item", http.StatusInternalServerError)
		return
	}

	_, err = h.Queries.UpsertUserSyncItem(r.Context(), db.UpsertUserSyncItemParams{
		UserID:     userID,
		SyncItemID: itemID,
		CreatedAt:  createdTime,
		UpdatedAt:  now,
	})
	if err != nil {
		http.Error(w, "Failed to map item to user", http.StatusInternalServerError)
		return
	}

	_, err = h.Queries.InsertDeltaEvent(r.Context(), db.InsertDeltaEventParams{
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
		http.Error(w, "Failed to log change", http.StatusInternalServerError)
		return
	}

	resp := ItemMetadataResponse{
		ID:             item.ID,
		Name:           item.FileName,
		MimeType:       item.MimeType,
		UpdatedTime:    item.UpdatedAt,
		JopUpdatedTime: item.ClientUpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *ItemHandler) handleDelete(w http.ResponseWriter, r *http.Request, userID, itemName string) {
	existing, err := h.Queries.GetSyncItemByFileNameAndUser(r.Context(), db.GetSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.Queries.DeleteUserSyncItem(r.Context(), db.DeleteUserSyncItemParams{
		UserID:     userID,
		SyncItemID: existing.ID,
	})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.Queries.DeleteSyncItemByFileNameAndUser(r.Context(), db.DeleteSyncItemByFileNameAndUserParams{
		FileName: itemName,
		UserID:   userID,
	})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UnixMilli()

	_, err = h.Queries.InsertDeltaEvent(r.Context(), db.InsertDeltaEventParams{
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
		http.Error(w, "Failed to log change", http.StatusInternalServerError)
		return
	}

	_ = h.Storage.DeleteItem(r.Context(), userID, itemName)

	w.WriteHeader(http.StatusOK)
}
