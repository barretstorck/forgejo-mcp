package document

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestGetDocumentPageImage_PDF(t *testing.T) {
	pdf := buildFixturePDF([]string{"kilo page", "lima page"})
	srv := serveContents(t, map[string][]byte{"docs/m.pdf": pdf})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "filePath": "docs/m.pdf", "page": float64(2), "max_edge_px": float64(600),
	})
	res, err := GetDocumentPageImageFn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	var img *mcp.ImageContent
	for i := range res.Content {
		if ic, ok := res.Content[i].(mcp.ImageContent); ok {
			img = &ic
			break
		}
	}
	if img == nil {
		t.Fatalf("no ImageContent in result: %+v", res.Content)
	}
	if img.MIMEType != "image/png" {
		t.Fatalf("mime = %q", img.MIMEType)
	}
	raw, err := base64.StdEncoding.DecodeString(img.Data)
	if err != nil || !strings.HasPrefix(string(raw), "\x89PNG") {
		t.Fatal("payload is not base64 PNG")
	}
}

// TestGetDocumentPageImage_RejectsOversizeEdge adapts the brief's listing:
// this package follows the established to.ErrorResult(err) convention (see
// text_test.go's TestGetDocumentText_UnsupportedExtension), which returns
// (nil, err) rather than a CallToolResult with IsError=true. So the error
// surfaces through the function's error return, not res.IsError. It also
// relies on serveContents(t, map[string][]byte{}) never matching any path,
// so a fetch (if attempted) would 404 — proving the max_edge_px cap check
// must run before any fetch.
func TestGetDocumentPageImage_RejectsOversizeEdge(t *testing.T) {
	srv := serveContents(t, map[string][]byte{})
	defer srv.Close()
	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "filePath": "docs/m.pdf", "max_edge_px": float64(9999),
	})
	res, err := GetDocumentPageImageFn(context.Background(), req)
	if err == nil {
		t.Fatal("max_edge_px beyond hard cap must be a tool error")
	}
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
	if !strings.Contains(err.Error(), "2048") {
		t.Fatalf("error must mention the max_edge_px cap, got: %v", err)
	}
}
