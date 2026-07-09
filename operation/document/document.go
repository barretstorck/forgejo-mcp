// Package document provides knowledgebase-oriented MCP tools for reading
// and searching documents (PDF, images, OOXML) stored in a repository.
package document

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/doccache"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/extract"
	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	"github.com/mark3labs/mcp-go/server"
)

const (
	GetDocumentTextToolName      = "get_document_text"
	SearchDocumentsToolName      = "search_documents"
	GetDocumentPageImageToolName = "get_document_page_image"

	// MaxTextResponseBytes caps get_document_text responses.
	MaxTextResponseBytes = 8192
)

var (
	ext   = extract.New()
	cache = doccache.New(256)
)

// RegisterTool registers the document tools: get_document_text,
// search_documents, and get_document_page_image.
func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetDocumentTextTool, GetDocumentTextFn)
	s.AddTool(SearchDocumentsTool, SearchDocumentsFn)
	s.AddTool(GetDocumentPageImageTool, GetDocumentPageImageFn)
}

// excluded reports whether a repo path matches DOCUMENT_EXCLUDE_GLOBS.
// Supported: path.Match patterns against the full path, and "dir/**"
// prefix patterns.
func excluded(p string) bool {
	for _, g := range flagPkg.DocumentExcludeGlobs {
		if strings.HasSuffix(g, "/**") {
			if strings.HasPrefix(p, strings.TrimSuffix(g, "**")) {
				return true
			}
			continue
		}
		if ok, _ := path.Match(g, p); ok {
			return true
		}
	}
	return false
}

// docKind classifies a path by extension: "pdf", "image", "docx", "xlsx",
// or "" when unsupported.
func docKind(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".pdf":
		return "pdf"
	case ".png", ".jpg", ".jpeg", ".webp":
		return "image"
	case ".docx":
		return "docx"
	case ".xlsx":
		return "xlsx"
	}
	return ""
}

// fetchFile downloads a repo file via the contents API, enforcing the
// configured size cap, and returns raw bytes plus the git blob SHA.
func fetchFile(owner, repo, ref, filePath string) ([]byte, string, error) {
	content, _, err := forgejo.Client().GetContents(owner, repo, ref, filePath)
	if err != nil {
		return nil, "", fmt.Errorf("get file err: %v", err)
	}
	if content == nil || content.Type != "file" {
		return nil, "", fmt.Errorf("%s is not a file", filePath)
	}
	if content.Size > flagPkg.MaxFileBytes {
		return nil, "", fmt.Errorf("%s is %d bytes, over the %d-byte cap (FORGEJO_MCP_MAX_FILE_BYTES)",
			filePath, content.Size, flagPkg.MaxFileBytes)
	}
	if content.Content == nil || content.Encoding == nil || *content.Encoding != "base64" {
		return nil, "", fmt.Errorf("unexpected contents encoding for %s", filePath)
	}
	data, err := base64.StdEncoding.DecodeString(*content.Content)
	if err != nil {
		return nil, "", fmt.Errorf("decode %s: %v", filePath, err)
	}
	return data, content.SHA, nil
}

// extractCached extracts a document's page texts through the cache.
func extractCached(ctx context.Context, data []byte, sha, kind string) ([]extract.PageText, error) {
	key := sha + "|" + extract.Version
	if pages, ok := cache.Get(key); ok {
		return pages, nil
	}
	var (
		pages []extract.PageText
		err   error
	)
	switch kind {
	case "pdf":
		pages, err = ext.ExtractPDFTextOCR(ctx, data)
	case "image":
		pages, err = ext.ExtractImageText(ctx, data)
	case "docx":
		pages, err = extract.ExtractDOCXText(data)
	case "xlsx":
		pages, err = extract.ExtractXLSXText(data)
	default:
		err = fmt.Errorf("unsupported document type")
	}
	if err != nil {
		return nil, err
	}
	cache.Put(key, pages)
	return pages, nil
}
