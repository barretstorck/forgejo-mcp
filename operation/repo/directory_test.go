package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// setupTreeMockServer creates a mock that routes requests based on URL path.
// It handles both /repos/:owner/:repo (GetRepo) and /repos/:owner/:repo/git/trees/:ref (GetTrees).
func setupTreeMockServer(t *testing.T, repoResponse, treeResponse interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/trees/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(treeResponse)
			return
		}
		// Default: repo info endpoint
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(repoResponse)
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func TestGetRepositoryTreeFn_WithRef(t *testing.T) {
	treeResponse := map[string]interface{}{
		"sha": "abc123",
		"tree": []map[string]interface{}{
			{"path": "README.md", "type": "blob", "size": 100, "sha": "aaa111"},
			{"path": "src", "type": "tree", "size": 0, "sha": "bbb222"},
			{"path": "src/main.go", "type": "blob", "size": 2048, "sha": "ccc333"},
		},
		"truncated": false,
	}
	// When ref is provided, GetRepo is not called, so repoResponse can be nil
	srv := setupTreeMockServer(t, nil, treeResponse)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":     "testowner",
		"repo":      "testrepo",
		"ref":       "main",
		"recursive": true,
	})
	result, err := GetRepositoryTreeFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetRepositoryTreeFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetRepositoryTreeFn returned tool error")
	}
}

func TestGetRepositoryTreeFn_DefaultRef(t *testing.T) {
	repoResponse := map[string]interface{}{
		"default_branch": "main",
		"name":           "testrepo",
		"owner":          map[string]interface{}{"login": "testowner"},
	}
	treeResponse := map[string]interface{}{
		"sha": "abc123",
		"tree": []map[string]interface{}{
			{"path": "README.md", "type": "blob", "size": 100, "sha": "aaa111"},
		},
		"truncated": false,
	}
	srv := setupTreeMockServer(t, repoResponse, treeResponse)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
	})
	result, err := GetRepositoryTreeFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetRepositoryTreeFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetRepositoryTreeFn returned tool error")
	}
}

func TestGetRepositoryTreeFn_MissingOwner(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"repo": "testrepo",
	})
	_, err := GetRepositoryTreeFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing owner, got nil")
	}
}
