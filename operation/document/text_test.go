package document

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// serveContents mocks GET /repos/{o}/{r}/contents/{path}, returning each
// file in files as a base64 contents response.
func serveContents(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for path, data := range files {
			if strings.Contains(r.URL.Path, "/contents/"+path) {
				enc := base64.StdEncoding.EncodeToString(data)
				b64 := "base64"
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"type": "file", "name": path, "path": path, "sha": "sha-" + path,
					"size": len(data), "content": enc, "encoding": b64,
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

// documentTextResult unwraps the `{"Result": <payload>}` envelope that
// to.TextResult wraps every tool response in (see tree_test.go's treeResult
// helper for the established pattern) and unmarshals the inner payload into
// out.
func documentTextResult(t *testing.T, res *mcp.CallToolResult, out interface{}) {
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

func TestGetDocumentText_PDFPageRange(t *testing.T) {
	requireBinary(t, extract.New().PdftotextPath, "pdftotext")
	pdf := buildFixturePDF([]string{"alpha page one", "bravo page two", "charlie page three"})
	srv := serveContents(t, map[string][]byte{"docs/manual.pdf": pdf})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "docs/manual.pdf", "pages": "2-3",
	})
	res, err := GetDocumentTextFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Path          string `json:"path"`
		TotalPages    int    `json:"total_pages"`
		Text          string `json:"text"`
		PagesIncluded []int  `json:"pages_included"`
		Truncated     bool   `json:"truncated"`
	}
	documentTextResult(t, res, &out)
	if out.TotalPages != 3 || len(out.PagesIncluded) != 2 || out.PagesIncluded[0] != 2 {
		t.Fatalf("out = %+v", out)
	}
	if strings.Contains(out.Text, "alpha") || !strings.Contains(out.Text, "bravo") || !strings.Contains(out.Text, "charlie") {
		t.Fatalf("wrong pages in text: %q", out.Text)
	}
}

// TestGetDocumentText_UnsupportedExtension: this package follows the
// established to.ErrorResult(err) convention (see e.g.
// operation/repo/file.go), which returns (nil, err) rather than a
// CallToolResult with IsError=true. So the error surfaces through the
// function's error return, not res.IsError.
func TestGetDocumentText_UnsupportedExtension(t *testing.T) {
	srv := serveContents(t, map[string][]byte{"a.txt": []byte("plain")})
	defer srv.Close()
	req := newCallToolRequest(map[string]interface{}{"owner": "o", "repo": "r", "filePath": "a.txt"})
	_, err := GetDocumentTextFn(context.Background(), req)
	if err == nil {
		t.Fatal("unsupported extension must return an error suggesting get_file_content")
	}
	if !strings.Contains(err.Error(), "get_file_content") {
		t.Fatalf("error must suggest get_file_content, got: %v", err)
	}
}

// TestGetDocumentText_ExcludedPath verifies excluded() is wired into
// GetDocumentTextFn too — the privacy boundary applies to direct reads, not
// just search_documents.
func TestGetDocumentText_ExcludedPath(t *testing.T) {
	flagPkg.DocumentExcludeGlobs = []string{"private/**"}
	defer func() { flagPkg.DocumentExcludeGlobs = nil }()
	srv := serveContents(t, map[string][]byte{"private/manual.pdf": buildFixturePDF([]string{"secret"})})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{"owner": "o", "repo": "r", "filePath": "private/manual.pdf"})
	_, err := GetDocumentTextFn(context.Background(), req)
	if err == nil {
		t.Fatal("excluded path must return an error")
	}
	if !strings.Contains(err.Error(), "excluded from document tools") {
		t.Fatalf("error must mention exclusion, got: %v", err)
	}
}

// TestGetDocumentText_DOCXOversizedPage covers a single page (DOCX/XLSX are
// always exactly one page) whose text alone exceeds MaxTextResponseBytes.
// Before the fix, the size-cap loop dropped the oversized chunk entirely,
// leaving text empty; it must instead return a non-empty, rune-safe
// truncated prefix of that page.
func TestGetDocumentText_DOCXOversizedPage(t *testing.T) {
	big := strings.Repeat("lorem ipsum dolor sit amet consectetur adipiscing elit ", 200) // ~11 KB, over the 8 KB cap
	docx := buildFixtureDOCX([]string{big})
	srv := serveContents(t, map[string][]byte{"docs/big.docx": docx})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{"owner": "o", "repo": "r", "filePath": "docs/big.docx"})
	res, err := GetDocumentTextFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Text          string `json:"text"`
		PagesIncluded []int  `json:"pages_included"`
		Truncated     bool   `json:"truncated"`
	}
	documentTextResult(t, res, &out)
	if out.Text == "" {
		t.Fatal("text must not be empty for an oversized single page")
	}
	if len(out.Text) > MaxTextResponseBytes {
		t.Fatalf("text = %d bytes, want <= %d", len(out.Text), MaxTextResponseBytes)
	}
	if !out.Truncated {
		t.Fatal("want truncated = true")
	}
	if len(out.PagesIncluded) != 1 || out.PagesIncluded[0] != 1 {
		t.Fatalf("pages_included = %v, want [1]", out.PagesIncluded)
	}
}
