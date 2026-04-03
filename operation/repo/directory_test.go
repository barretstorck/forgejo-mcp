package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func setupDirectoryMockServer(t *testing.T, response interface{}, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func TestListDirectoryFn_RootDirectory(t *testing.T) {
	mockResponse := []map[string]interface{}{
		{"name": "README.md", "type": "file", "path": "README.md", "size": 100},
		{"name": "src", "type": "dir", "path": "src", "size": 0},
	}
	srv := setupDirectoryMockServer(t, mockResponse, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
	})
	result, err := ListDirectoryFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListDirectoryFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ListDirectoryFn returned tool error")
	}
}

func TestListDirectoryFn_SubDirectory(t *testing.T) {
	mockResponse := []map[string]interface{}{
		{"name": "main.go", "type": "file", "path": "src/main.go", "size": 2048},
	}
	srv := setupDirectoryMockServer(t, mockResponse, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
		"path":  "src",
		"ref":   "main",
	})
	result, err := ListDirectoryFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListDirectoryFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ListDirectoryFn returned tool error")
	}
}

func TestGetDirectoryContentFn_RootDirectory(t *testing.T) {
	mockResponse := []map[string]interface{}{
		{"name": "README.md", "type": "file", "path": "README.md", "size": 100, "sha": "abc123", "download_url": "https://example.com/README.md", "html_url": "https://example.com/repo/README.md"},
		{"name": "src", "type": "dir", "path": "src", "size": 0, "sha": "def456"},
	}
	srv := setupDirectoryMockServer(t, mockResponse, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
	})
	result, err := GetDirectoryContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetDirectoryContentFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetDirectoryContentFn returned tool error")
	}
}

func TestGetDirectoryContentFn_MissingOwner(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"repo": "testrepo",
	})
	_, err := GetDirectoryContentFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing owner, got nil")
	}
}

func TestListDirectoryFn_MissingOwner(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"repo": "testrepo",
	})
	_, err := ListDirectoryFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing owner, got nil")
	}
}
