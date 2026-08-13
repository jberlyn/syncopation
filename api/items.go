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
	"github.com/jberlyn/joplin-sync/db"
)

type ItemHandler struct {
	Queries *db.Queries
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
		if strings.HasSuffix(rest, ":") {
			return rest[:len(rest)-1], "stat", true
		}
	} else if strings.HasPrefix(p, "/api/items/root::") {
		rest := p[len("/api/items/root::"):]
		if rest == "/delta" {
			return "", "delta", true
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
	item, err := h.Queries.GetItemByNameAndUser(r.Context(), db.GetItemByNameAndUserParams{
		Name:   itemName,
		UserID: userID,
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
		Name:           item.Name,
		MimeType:       item.MimeType,
		UpdatedTime:    item.UpdatedTime,
		JopUpdatedTime: item.JopUpdatedTime,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *ItemHandler) handleGetContent(w http.ResponseWriter, r *http.Request, userID, itemName string) {
	item, err := h.Queries.GetItemByNameAndUser(r.Context(), db.GetItemByNameAndUserParams{
		Name:   itemName,
		UserID: userID,
	})

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", item.MimeType)
	_, _ = w.Write(item.Content)
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

	changes, err := h.Queries.GetChangesByUser(r.Context(), db.GetChangesByUserParams{
		UserID:  userID,
		Counter: cursor,
		Limit:   limit + 1,
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
			ItemName:    c.ItemName,
			ItemType:    c.ItemType,
			Type:        c.Type,
			UpdatedTime: c.UpdatedTime,
		}
		deltaItems = append(deltaItems, item)
		lastCursor = c.Counter
	}

	resp := DeltaResponse{
		Items:   deltaItems,
		HasMore: hasMore,
		Cursor:  strconv.FormatInt(lastCursor, 10),
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

	existing, err := h.Queries.GetItemByNameAndUser(r.Context(), db.GetItemByNameAndUserParams{
		Name:   itemName,
		UserID: userID,
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
		createdTime = existing.CreatedTime
		changeType = 2 // Update
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	item, err := h.Queries.UpsertItem(r.Context(), db.UpsertItemParams{
		ID:                   itemID,
		Name:                 itemName,
		MimeType:             contentType,
		Content:              content,
		ContentSize:          int64(len(content)),
		JopID:                "",
		JopParentID:          "",
		JopShareID:           "",
		JopType:              1,
		JopEncryptionApplied: 0,
		JopUpdatedTime:       now,
		OwnerID:              userID,
		ContentStorageID:     1,
		CreatedTime:          createdTime,
		UpdatedTime:          now,
	})
	if err != nil {
		http.Error(w, "Failed to save item", http.StatusInternalServerError)
		return
	}

	_, err = h.Queries.UpsertUserItem(r.Context(), db.UpsertUserItemParams{
		UserID:      userID,
		ItemID:      itemID,
		CreatedTime: createdTime,
		UpdatedTime: now,
	})
	if err != nil {
		http.Error(w, "Failed to map item to user", http.StatusInternalServerError)
		return
	}

	_, err = h.Queries.InsertChange(r.Context(), db.InsertChangeParams{
		ID:              uuid.New().String(),
		ItemID:          itemID,
		UserID:          userID,
		ItemName:        itemName,
		PreviousShareID: "",
		ItemType:        1,
		Type:            changeType,
		CreatedTime:     now,
		UpdatedTime:     now,
	})
	if err != nil {
		http.Error(w, "Failed to log change", http.StatusInternalServerError)
		return
	}

	resp := ItemMetadataResponse{
		ID:             item.ID,
		Name:           item.Name,
		MimeType:       item.MimeType,
		UpdatedTime:    item.UpdatedTime,
		JopUpdatedTime: item.JopUpdatedTime,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *ItemHandler) handleDelete(w http.ResponseWriter, r *http.Request, userID, itemName string) {
	existing, err := h.Queries.GetItemByNameAndUser(r.Context(), db.GetItemByNameAndUserParams{
		Name:   itemName,
		UserID: userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.Queries.DeleteUserItem(r.Context(), db.DeleteUserItemParams{
		UserID: userID,
		ItemID: existing.ID,
	})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = h.Queries.DeleteItemByNameAndUser(r.Context(), db.DeleteItemByNameAndUserParams{
		Name:   itemName,
		UserID: userID,
	})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	now := time.Now().UnixMilli()

	_, err = h.Queries.InsertChange(r.Context(), db.InsertChangeParams{
		ID:              uuid.New().String(),
		ItemID:          existing.ID,
		UserID:          userID,
		ItemName:        itemName,
		PreviousShareID: "",
		ItemType:        1,
		Type:            3, // Delete
		CreatedTime:     now,
		UpdatedTime:     now,
	})
	if err != nil {
		http.Error(w, "Failed to log change", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
