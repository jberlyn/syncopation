package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jberlyn/syncopation/db"
)

func TestAPIDBErrors(t *testing.T) {
	mux, queries, dbConn, _ := setupTestApp(t)
	defer dbConn.Close()

	// Seed a user
	seedTestUser(t, queries, "dberr@example.com", "pass")

	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	// Create session via DB so we don't have to parse JSON
	sess, _ := queries.CreateSession(context.Background(), db.CreateSessionParams{
		ID: "err-sess", UserID: "err-user",
	})

	// Drop all tables! This will cause ALL DB queries to fail with 500
	_, _ = dbConn.Exec("DROP TABLE items;")
	_, _ = dbConn.Exec("DROP TABLE user_items;")
	_, _ = dbConn.Exec("DROP TABLE changes;")
	_, _ = dbConn.Exec("DROP TABLE locks;")
	// don't drop sessions or users yet because we need auth to pass!

	doReq := func(method, path string, body string) {
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		req.Header.Set("X-API-AUTH", sess.ID)
		_, _ = client.Do(req)
	}

	// 1. Items
	doReq("GET", "/api/items/root:/item-1:/content", "")
	doReq("PUT", "/api/items/root:/item-1:/content", "data")
	doReq("DELETE", "/api/items/root:/item-1:", "")
	doReq("GET", "/api/items/root::/delta", "")
	doReq("GET", "/api/items/root::/*:/children", "")
	doReq("GET", "/api/items/root:/item-1:", "")

	// 2. Batch Items
	batchBody := `{"items":[{"id":"1","name":"note1","type_":1}]}`
	doReq("PUT", "/api/batch_items", batchBody)
	doReq("DELETE", "/api/batch_items", `["1"]`)

	// 3. Locks
	doReq("GET", "/api/locks", "")
	doReq("POST", "/api/locks", `{"type":1,"clientType":1,"clientId":"cli"}`)
	doReq("DELETE", "/api/locks/lock_1", "")

	// 4. Storage Errors
	// Recreate DB but delete storage
	mux2, queries2, dbConn2, localFS2 := setupTestApp(t)
	defer dbConn2.Close()
	seedTestUser(t, queries2, "storage@example.com", "pass")
	sess2, _ := queries2.CreateSession(context.Background(), db.CreateSessionParams{
		ID: "err-sess2", UserID: "err-user2",
	})
	_, _ = queries2.UpsertItem(context.Background(), db.UpsertItemParams{
		ID: "item-3", Name: "item-3",
	})
	_, _ = queries2.UpsertUserItem(context.Background(), db.UpsertUserItemParams{
		UserID: sess2.UserID, ItemID: "item-3",
	})

	server2 := httptest.NewServer(mux2)
	defer server2.Close()
	doReq2 := func(method, path string, body string) {
		req, _ := http.NewRequest(method, server2.URL+path, strings.NewReader(body))
		req.Header.Set("X-API-AUTH", sess2.ID)
		_, _ = server2.Client().Do(req)
	}

	// Break storage by setting it to a non-existent path without permissions
	localFS2.DataDir = "/root/invalid-dir-no-access-999"

	doReq2("GET", "/api/items/root:/item-3:/content", "")
	doReq2("PUT", "/api/items/root:/item-3:/content", "data")
	doReq2("DELETE", "/api/items/root:/item-3:", "")
}
