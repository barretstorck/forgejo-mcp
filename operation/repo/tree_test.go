package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// setupTreeServer serves a synthetic recursive tree with n blob entries
// under nested directories: dir0/file0.md, dir1/file1.md, ..., plus
// docs/deep/nested/file.md to exercise path and depth filters.
func setupTreeServer(t *testing.T, n int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/git/trees/") {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		entries := []map[string]interface{}{}
		for i := 0; i < n; i++ {
			entries = append(entries, map[string]interface{}{
				"path": fmt.Sprintf("dir%d/file%d.md", i, i), "type": "blob", "size": 100, "sha": "s", "url": "u",
			})
		}
		entries = append(entries, map[string]interface{}{
			"path": "docs/deep/nested/file.md", "type": "blob", "size": 5, "sha": "s", "url": "u",
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"sha": "root", "tree": entries, "truncated": false})
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

// treeResult unwraps the `{"Result": <payload>}` envelope that to.TextResult
// wraps every tool response in (see extractToolResultFields in file_test.go
// for the established pattern) and returns the inner payload.
func treeResult(t *testing.T, res *mcp.CallToolResult) map[string]interface{} {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %+v", res)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var wrapper struct {
		Result map[string]interface{} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		t.Fatalf("unmarshal %q: %v", text, err)
	}
	return wrapper.Result
}

func TestGetRepositoryTree_CapsEntries(t *testing.T) {
	srv := setupTreeServer(t, 500)
	defer srv.Close()
	req := newCallToolRequest(map[string]interface{}{"owner": "o", "repo": "r", "ref": "main"})
	res, err := GetRepositoryTreeFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out := treeResult(t, res)
	if int(out["returned"].(float64)) != MaxTreeEntries {
		t.Fatalf("returned = %v, want %d", out["returned"], MaxTreeEntries)
	}
	if out["truncated"] != true {
		t.Fatal("expected truncated=true")
	}
	entry := out["entries"].([]interface{})[0].(map[string]interface{})
	if _, hasSHA := entry["sha"]; hasSHA {
		t.Fatal("entries must not include sha boilerplate")
	}
	if _, hasURL := entry["url"]; hasURL {
		t.Fatal("entries must not include url boilerplate")
	}
}

func TestGetRepositoryTree_PathAndDepth(t *testing.T) {
	srv := setupTreeServer(t, 10)
	defer srv.Close()
	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "path": "docs", "depth": float64(1),
	})
	res, err := GetRepositoryTreeFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out := treeResult(t, res)
	// docs/deep/nested/file.md is 3 levels below docs/ → excluded at depth 1.
	if int(out["returned"].(float64)) != 0 {
		t.Fatalf("returned = %v, want 0 (depth filter)", out["returned"])
	}
	if int(out["total_matching"].(float64)) != 1 {
		t.Fatalf("total_matching = %v, want 1 (path filter before depth)", out["total_matching"])
	}
}

func TestSearchRepositoryContents_CapsMatches(t *testing.T) {
	srv := setupTreeServer(t, 300)
	defer srv.Close()
	req := newCallToolRequest(map[string]interface{}{"owner": "o", "repo": "r", "ref": "main", "query": "file"})
	res, err := SearchRepositoryContentsFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	out := treeResult(t, res)
	if int(out["returned"].(float64)) != MaxSearchMatches {
		t.Fatalf("returned = %v, want %d", out["returned"], MaxSearchMatches)
	}
	if out["truncated"] != true {
		t.Fatal("expected truncated=true")
	}
}
