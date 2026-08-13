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

	items, err := h.Queries.ListItemsByUser(r.Context(), db.ListItemsByUserParams{
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
			Name:           item.Name,
			MimeType:       item.MimeType,
			UpdatedTime:    item.UpdatedTime,
			JopUpdatedTime: item.JopUpdatedTime,
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

	if err := h.Storage.WriteItem(r.Context(), userID, itemName, content); err != nil {
		http.Error(w, "Failed to save item content", http.StatusInternalServerError)
		return
	}

	item, err := h.Queries.UpsertItem(r.Context(), db.UpsertItemParams{
		ID:                   itemID,
		Name:                 itemName,
		MimeType:             contentType,
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

	_ = h.Storage.DeleteItem(r.Context(), userID, itemName)

	w.WriteHeader(http.StatusOK)
}
