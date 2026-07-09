package document

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/extract"
	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// serveRepo mocks the git/trees endpoint (listing files), the contents
// endpoint (serving file bytes), and — as a fallback for any other
// /repos/{owner}/{repo} path — the repo-info endpoint used to resolve the
// default branch when ref is empty.
func serveRepo(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/trees/") {
			entries := []map[string]interface{}{}
			for path, data := range files {
				entries = append(entries, map[string]interface{}{
					"path": path, "type": "blob", "size": len(data), "sha": "sha-" + path,
				})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"sha": "root", "tree": entries})
			return
		}
		for path, data := range files {
			if strings.Contains(r.URL.Path, "/contents/"+path) {
				b64 := "base64"
				json.NewEncoder(w).Encode(map[string]interface{}{
					"type": "file", "name": path, "path": path, "sha": "sha-" + path,
					"size": len(data), "content": base64.StdEncoding.EncodeToString(data), "encoding": b64,
				})
				return
			}
		}
		// Fallback: repo-info endpoint (GET /repos/{owner}/{repo}), used by
		// resolveRef to look up the default branch when ref == "".
		json.NewEncoder(w).Encode(map[string]interface{}{"default_branch": "main"})
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

// searchDocsResult unwraps the `{"Result": <payload>}` envelope (see
// text_test.go's documentTextResult helper for the established pattern).
func searchDocsResult(t *testing.T, res *mcp.CallToolResult, out interface{}) {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	text := res.Content[0].(mcp.TextContent).Text
	var wrapper struct {
		Result json.RawMessage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		t.Fatalf("unmarshal envelope %q: %v", text, err)
	}
	if err := json.Unmarshal(wrapper.Result, out); err != nil {
		t.Fatalf("unmarshal result %q: %v", wrapper.Result, err)
	}
}

func TestSearchDocuments_RepoWide(t *testing.T) {
	requireBinary(t, extract.New().PdftotextPath, "pdftotext")
	files := map[string][]byte{
		"docs/a.pdf":  buildFixturePDF([]string{"the india token lives here"}),
		"docs/b.pdf":  buildFixturePDF([]string{"nothing relevant"}),
		"notes/c.txt": []byte("india token but wrong extension"),
	}
	srv := serveRepo(t, files)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "query": "india token",
	})
	res, err := SearchDocumentsFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Hits []struct {
			Path    string `json:"path"`
			Page    int    `json:"page"`
			Snippet string `json:"snippet"`
		} `json:"hits"`
		DocumentsSearched int `json:"documents_searched"`
	}
	searchDocsResult(t, res, &out)
	if len(out.Hits) != 1 || out.Hits[0].Path != "docs/a.pdf" || out.Hits[0].Page != 1 {
		t.Fatalf("hits = %+v", out.Hits)
	}
	if !strings.Contains(strings.ToLower(out.Hits[0].Snippet), "india token") {
		t.Fatalf("snippet %q", out.Hits[0].Snippet)
	}
	if out.DocumentsSearched != 2 { // only the two PDFs are supported types
		t.Fatalf("documents_searched = %d, want 2", out.DocumentsSearched)
	}
}

func TestSearchDocuments_ExcludeGlobs(t *testing.T) {
	requireBinary(t, extract.New().PdftotextPath, "pdftotext")
	flagPkg.DocumentExcludeGlobs = []string{"private/**"}
	defer func() { flagPkg.DocumentExcludeGlobs = nil }()
	files := map[string][]byte{
		"private/x.pdf": buildFixturePDF([]string{"juliet token secret"}),
		"public/y.pdf":  buildFixturePDF([]string{"juliet token open"}),
	}
	srv := serveRepo(t, files)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{"owner": "o", "repo": "r", "ref": "main", "query": "juliet token"})
	res, err := SearchDocumentsFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Hits []struct {
			Path string `json:"path"`
		} `json:"hits"`
	}
	searchDocsResult(t, res, &out)
	if len(out.Hits) != 1 || out.Hits[0].Path != "public/y.pdf" {
		t.Fatalf("excluded path leaked into hits: %+v", out.Hits)
	}
}

func TestExcluded(t *testing.T) {
	flagPkg.DocumentExcludeGlobs = []string{"private/**", "*.bak"}
	defer func() { flagPkg.DocumentExcludeGlobs = nil }()
	cases := map[string]bool{
		"private/deep/file.pdf":   true,
		"public/file.pdf":         false,
		"old.bak":                 true,
		"public/../private/x.pdf": true, // dot-dot traversal must not bypass the exclusion
		"./private/x.pdf":         true, // leading "./" must not bypass the exclusion
	}
	for p, want := range cases {
		if excluded(p) != want {
			t.Fatalf("excluded(%q) = %v, want %v", p, !want, want)
		}
	}
}

// TestSearchDocuments_RepoWide_NoRef covers the case where a repo-wide
// search omits ref entirely. GetTrees rejects an empty ref path segment, so
// SearchDocumentsFn must resolve the default branch first (mirroring
// operation/repo/file.go's resolveRef) before calling GetTrees.
func TestSearchDocuments_RepoWide_NoRef(t *testing.T) {
	requireBinary(t, extract.New().PdftotextPath, "pdftotext")
	// Uses a path not reused by any other test in this file: the mock
	// server's fake blob SHA is derived from the path alone ("sha-"+path),
	// and extractCached's package-level cache is keyed by that SHA, so
	// reusing a path already searched by an earlier test would return
	// stale cached content instead of exercising this test's fixture.
	files := map[string][]byte{
		"docs/kilo.pdf": buildFixturePDF([]string{"kilo token lives here"}),
	}
	srv := serveRepo(t, files)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "query": "kilo token",
	})
	res, err := SearchDocumentsFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Hits []struct {
			Path string `json:"path"`
		} `json:"hits"`
		DocumentsSearched int `json:"documents_searched"`
	}
	searchDocsResult(t, res, &out)
	if len(out.Hits) != 1 || out.Hits[0].Path != "docs/kilo.pdf" {
		t.Fatalf("hits = %+v", out.Hits)
	}
	if out.DocumentsSearched != 1 {
		t.Fatalf("documents_searched = %d, want 1", out.DocumentsSearched)
	}
}

// TestSearchDocuments_DocCapLimitReached verifies the repo-wide scan stops
// after MaxDocsPerSearch candidates, flags limit_reached, and counts the
// unexamined remainder as skipped.
func TestSearchDocuments_DocCapLimitReached(t *testing.T) {
	const n = MaxDocsPerSearch + 50
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/git/trees/") {
			entries := make([]map[string]interface{}, 0, n)
			for i := 0; i < n; i++ {
				entries = append(entries, map[string]interface{}{
					"path": fmt.Sprintf("docs/%04d.docx", i), "type": "blob", "size": 10, "sha": fmt.Sprintf("sha-%d", i),
				})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"sha": "root", "tree": entries})
			return
		}
		// No file content is ever served, so every fetch 404s — the point
		// of this test is that the scan stops at the cap, not that any
		// document is actually searched.
		http.NotFound(w, r)
	}))
	defer srv.Close()
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	forgejo.SetClientForTesting(client)

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "query": "anything",
	})
	res, err := SearchDocumentsFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		DocumentsSearched int  `json:"documents_searched"`
		DocumentsSkipped  int  `json:"documents_skipped"`
		LimitReached      bool `json:"limit_reached"`
	}
	searchDocsResult(t, res, &out)
	if !out.LimitReached {
		t.Fatal("want limit_reached = true")
	}
	if out.DocumentsSearched > MaxDocsPerSearch {
		t.Fatalf("documents_searched = %d, want <= %d", out.DocumentsSearched, MaxDocsPerSearch)
	}
	if out.DocumentsSkipped < n-MaxDocsPerSearch {
		t.Fatalf("documents_skipped = %d, want >= %d (unexamined remainder)", out.DocumentsSkipped, n-MaxDocsPerSearch)
	}
}

// TestSearchDocuments_SingleFileMissingPath_ReturnsError verifies that
// single-file search mode (filePath given) surfaces fetch/extract errors
// via to.ErrorResult instead of silently reporting documents_skipped=1.
// to.ErrorResult returns (nil, err), so the error surfaces through the
// function's error return.
func TestSearchDocuments_SingleFileMissingPath_ReturnsError(t *testing.T) {
	srv := serveRepo(t, map[string][]byte{})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "query": "anything", "filePath": "docs/missing.pdf",
	})
	res, err := SearchDocumentsFn(context.Background(), req)
	if err == nil {
		t.Fatal("single-file search for a missing path must return an error")
	}
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
}
