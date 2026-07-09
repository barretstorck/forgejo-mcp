package document

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// serveRepo mocks both the git/trees endpoint (listing files) and the
// contents endpoint (serving file bytes).
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
		http.NotFound(w, r)
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
		"private/deep/file.pdf": true,
		"public/file.pdf":       false,
		"old.bak":               true,
	}
	for p, want := range cases {
		if excluded(p) != want {
			t.Fatalf("excluded(%q) = %v, want %v", p, !want, want)
		}
	}
}
