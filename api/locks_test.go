package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jberlyn/syncopation/api"
)

func TestLocksFlow(t *testing.T) {
	queries := setupTestDB(t) // reuse setupTestDB from auth_test.go
	lockHandler := &api.LockHandler{Queries: queries}

	mux := http.NewServeMux()
	// To keep things simple in this test, we test the lock handler without the AuthMiddleware.
	// In reality, it's wrapped in main.go, but here we can just test the locks logic.
	mux.HandleFunc("POST /api/locks", lockHandler.AcquireLock)
	mux.HandleFunc("DELETE /api/locks/{id}", lockHandler.ReleaseLock)
	mux.HandleFunc("GET /api/locks", lockHandler.ListLocks)

	// Helper to make API requests
	acquireLock := func(lockType, clientType int, clientId string) *httptest.ResponseRecorder {
		reqBody := map[string]interface{}{
			"type":       lockType,
			"clientType": clientType,
			"clientId":   clientId,
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/locks", bytes.NewBuffer(bodyBytes))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	releaseLock := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("DELETE", "/api/locks/"+id, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	listLocks := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/locks", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	// 1. Acquire multiple Sync Locks (Type 1)
	t.Run("Acquire Multiple Sync Locks", func(t *testing.T) {
		rr1 := acquireLock(api.LockTypeSync, 1, "client1")
		if rr1.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for sync lock 1, got %d", rr1.Code)
		}

		rr2 := acquireLock(api.LockTypeSync, 1, "client2")
		if rr2.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for sync lock 2, got %d", rr2.Code)
		}

		// Verify List Locks
		rrList := listLocks()
		var resp api.LocksResponse
		_ = json.NewDecoder(rrList.Body).Decode(&resp)
		if len(resp.Items) != 2 {
			t.Errorf("Expected 2 active sync locks, got %d", len(resp.Items))
		}
	})

	// 2. Fail to acquire Exclusive Lock (Type 2) if Sync Locks exist
	t.Run("Reject Exclusive Lock with Active Sync Locks", func(t *testing.T) {
		rr := acquireLock(api.LockTypeExclusive, 1, "client3")
		if rr.Code != http.StatusConflict {
			t.Errorf("Expected 409 Conflict, got %d", rr.Code)
		}
	})

	// 3. Release Sync Locks and acquire Exclusive Lock
	t.Run("Acquire Exclusive Lock After Release", func(t *testing.T) {
		releaseLock("1_1_client1")
		releaseLock("1_1_client2")

		rrList := listLocks()
		var resp api.LocksResponse
		_ = json.NewDecoder(rrList.Body).Decode(&resp)
		if len(resp.Items) != 0 {
			t.Errorf("Expected 0 active locks, got %d", len(resp.Items))
		}

		// Now acquire exclusive
		rrExclusive := acquireLock(api.LockTypeExclusive, 1, "client3")
		if rrExclusive.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for exclusive lock, got %d", rrExclusive.Code)
		}
	})

	// 4. Reject new locks if Exclusive Lock exists
	t.Run("Reject Locks while Exclusive Lock Active", func(t *testing.T) {
		// Try sync
		rrSync := acquireLock(api.LockTypeSync, 1, "client4")
		if rrSync.Code != http.StatusConflict {
			t.Errorf("Expected 409 Conflict for sync lock, got %d", rrSync.Code)
		}

		// Try exclusive from another client
		rrExclusive := acquireLock(api.LockTypeExclusive, 1, "client5")
		if rrExclusive.Code != http.StatusConflict {
			t.Errorf("Expected 409 Conflict for exclusive lock, got %d", rrExclusive.Code)
		}

		// Wait, can the SAME client acquire it again? Yes, it overwrites/refreshes.
		rrRefresh := acquireLock(api.LockTypeExclusive, 1, "client3")
		if rrRefresh.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for refreshing own exclusive lock, got %d", rrRefresh.Code)
		}
	})
}
