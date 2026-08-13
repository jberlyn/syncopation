package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jberlyn/joplin-sync/db"
)

const (
	LockTypeSync      = 1
	LockTypeExclusive = 2

	LockTTL = 3 * time.Minute // Stale locks expire after 3 mins
)

// LockHandler groups related HTTP handlers and injects dependencies like the database queries.
type LockHandler struct {
	Queries *db.Queries
}

type LockRequest struct {
	Type       int    `json:"type"`
	ClientType int    `json:"clientType"`
	ClientId   string `json:"clientId"`
}

type LockItem struct {
	Type        int    `json:"type"`
	ClientType  int    `json:"clientType"`
	ClientId    string `json:"clientId"`
	UpdatedTime int64  `json:"updatedTime"`
}

type LocksResponse struct {
	Items   []LockItem `json:"items"`
	HasMore bool       `json:"has_more"`
}

// buildLockKey generates a unique key for the key_value store.
func buildLockKey(req LockRequest) string {
	return fmt.Sprintf("lock_%d_%d_%s", req.Type, req.ClientType, req.ClientId)
}

func (h *LockHandler) getActiveLocks(ctx context.Context) ([]LockItem, error) {
	// Let's get both sync and exclusive locks
	// The DB query doesn't give us a "Get everything matching a prefix",
	// but we added ListKeyValuesByType to queries.sql!
	var activeLocks []LockItem

	now := time.Now().UnixMilli()

	for _, lockType := range []int{LockTypeSync, LockTypeExclusive} {
		records, err := h.Queries.ListKeyValuesByType(ctx, int64(lockType))
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		for _, rec := range records {
			var lock LockItem
			if err := json.Unmarshal([]byte(rec.Value), &lock); err != nil {
				continue // Skip malformed locks
			}

			// TTL Check: auto-expire stale locks
			expirationTime := lock.UpdatedTime + LockTTL.Milliseconds()
			if now > expirationTime {
				// Lock has expired, clean it up
				_ = h.Queries.DeleteKeyValue(ctx, rec.Key)
				continue
			}
			activeLocks = append(activeLocks, lock)
		}
	}

	return activeLocks, nil
}

func (h *LockHandler) AcquireLock(w http.ResponseWriter, r *http.Request) {
	var req LockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 1. Get all active locks and clear out stale ones
	activeLocks, err := h.getActiveLocks(ctx)
	if err != nil {
		slog.Error("Error fetching active locks", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 2. Concurrency Checks
	hasExclusiveLock := false
	for _, l := range activeLocks {
		if l.Type == LockTypeExclusive && l.ClientId != req.ClientId {
			hasExclusiveLock = true
			break
		}
	}

	if req.Type == LockTypeExclusive {
		// If requesting exclusive, no other locks can exist (unless it's our own)
		hasOtherLocks := false
		for _, l := range activeLocks {
			if l.ClientId != req.ClientId {
				hasOtherLocks = true
				break
			}
		}
		if hasOtherLocks {
			http.Error(w, "Conflict: other locks exist", http.StatusConflict)
			return
		}
	} else if req.Type == LockTypeSync {
		// If requesting sync, no exclusive locks can exist
		if hasExclusiveLock {
			http.Error(w, "Conflict: exclusive lock exists", http.StatusConflict)
			return
		}
	} else {
		http.Error(w, "Invalid lock type", http.StatusBadRequest)
		return
	}

	// 3. Save the lock
	lockItem := LockItem{
		Type:        req.Type,
		ClientType:  req.ClientType,
		ClientId:    req.ClientId,
		UpdatedTime: time.Now().UnixMilli(),
	}

	valBytes, _ := json.Marshal(lockItem)
	key := buildLockKey(req)

	_, err = h.Queries.SetKeyValue(ctx, db.SetKeyValueParams{
		Key:   key,
		Type:  int64(req.Type),
		Value: string(valBytes),
	})
	if err != nil {
		http.Error(w, "Failed to acquire lock", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lockItem)
}

func (h *LockHandler) ReleaseLock(w http.ResponseWriter, r *http.Request) {
	// The path will be /api/locks/{id}
	// e.g. /api/locks/1_1_client_device_uuid_123
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing lock ID", http.StatusBadRequest)
		return
	}

	key := "lock_" + id
	err := h.Queries.DeleteKeyValue(r.Context(), key)
	if err != nil {
		http.Error(w, "Failed to release lock", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *LockHandler) ListLocks(w http.ResponseWriter, r *http.Request) {
	activeLocks, err := h.getActiveLocks(r.Context())
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array instead of null for clients expecting []
	if activeLocks == nil {
		activeLocks = make([]LockItem, 0)
	}

	resp := LocksResponse{
		Items:   activeLocks,
		HasMore: false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
