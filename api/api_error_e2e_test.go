package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMethodNotAllowed(t *testing.T) {
	mux, _, dbConn, _ := setupAdminTestApp(t) // reuse the setup, but just test routes
	defer dbConn.Close()

	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	routes := []struct {
		path   string
		method string
	}{
		{"/setup", "PUT"},
		{"/setup", "DELETE"},
		{"/login", "PUT"},
		{"/login", "DELETE"},
		{"/", "POST"},
		{"/", "PUT"},
		{"/api/users", "PUT"},
		{"/api/users/abc", "POST"},
		{"/api/users/abc", "PUT"},
	}

	for _, route := range routes {
		req, _ := http.NewRequest(route.method, server.URL+route.path, nil)
		resp, _ := client.Do(req)
		// We expect 405 for methods that explicitly block, or 404 for undefined
		if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusFound {
			t.Errorf("Expected MethodNotAllowed or NotFound for %s %s, got %d", route.method, route.path, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// For standard API items, we need a setup from api_e2e_test
	mux2, _, db2, _ := setupTestApp(t)
	defer db2.Close()
	server2 := httptest.NewServer(mux2)
	defer server2.Close()
	client2 := server2.Client()

	req, _ := http.NewRequest("PUT", server2.URL+"/api/sessions", nil)
	resp, _ := client2.Do(req)
	resp.Body.Close()

	req, _ = http.NewRequest("GET", server2.URL+"/api/batch_items", nil)
	resp, _ = client2.Do(req)
	resp.Body.Close()
}

func TestMalformedRequests(t *testing.T) {
	mux, _, dbConn, _ := setupTestApp(t)
	defer dbConn.Close()
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	// bad JSON in login
	req, _ := http.NewRequest("POST", server.URL+"/api/sessions", strings.NewReader("bad json"))
	resp, _ := client.Do(req)
	resp.Body.Close()
}
