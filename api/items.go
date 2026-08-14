package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jberlyn/syncopation/services"
)

type ItemHandler struct {
	ItemService *services.ItemService
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
	item, err := h.ItemService.GetStat(r.Context(), userID, itemName)
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
	item, content, err := h.ItemService.GetContent(r.Context(), userID, itemName)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
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

	changes, hasMore, lastCursor, err := h.ItemService.GetDelta(r.Context(), userID, cursor, limit)
	if err != nil {
		http.Error(w, "Failed to fetch changes", http.StatusInternalServerError)
		return
	}

	deltaItems := make([]DeltaItem, 0, len(changes))
	for _, c := range changes {
		item := DeltaItem{
			ItemName:    c.FileName,
			ItemType:    c.ItemType,
			Type:        c.EventType,
			UpdatedTime: c.UpdatedAt,
		}
		deltaItems = append(deltaItems, item)
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

	items, hasMore, nextCursor, err := h.ItemService.GetChildren(r.Context(), userID, cursor, limit)
	if err != nil {
		http.Error(w, "Failed to fetch items", http.StatusInternalServerError)
		return
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
		Cursor:  strconv.FormatInt(nextCursor, 10),
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

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	item, err := h.ItemService.PutContent(r.Context(), userID, itemName, content, contentType)
	if err != nil {
		http.Error(w, "Failed to save item", http.StatusInternalServerError)
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
	err := h.ItemService.DeleteItem(r.Context(), userID, itemName)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
