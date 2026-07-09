package document

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

var GetDocumentTextTool = mcp.NewTool(
	GetDocumentTextToolName,
	mcp.WithDescription("Extract text from a document file (PDF, PNG/JPG/WebP image, DOCX, XLSX) in a repository. Scanned pages and images are OCR'd automatically. Responses are capped at 8 KB — use `pages` (e.g. \"2\" or \"2-5\") to read further into long documents; the response reports total_pages and truncated."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("ref", mcp.Description(params.Ref)),
	mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
	mcp.WithString("pages", mcp.Description("Page or page range to return, e.g. \"3\" or \"2-5\". Default: from page 1 until the size cap.")),
)

// parsePageRange parses "3" or "2-5" into first/last (1-based, inclusive).
// Empty input returns (1, 0): start at 1, no explicit end.
func parsePageRange(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 1, 0, nil
	}
	if first, last, found := strings.Cut(s, "-"); found {
		f, err1 := strconv.Atoi(strings.TrimSpace(first))
		l, err2 := strconv.Atoi(strings.TrimSpace(last))
		if err1 != nil || err2 != nil || f < 1 || l < f {
			return 0, 0, fmt.Errorf("invalid pages %q: want \"N\" or \"N-M\" with 1 <= N <= M", s)
		}
		return f, l, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, 0, fmt.Errorf("invalid pages %q: want \"N\" or \"N-M\"", s)
	}
	return n, n, nil
}

type textResponse struct {
	Path          string `json:"path"`
	TotalPages    int    `json:"total_pages"`
	Text          string `json:"text"`
	PagesIncluded []int  `json:"pages_included"`
	Truncated     bool   `json:"truncated"`
}

func GetDocumentTextFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetDocumentTextFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	ref, _ := req.GetArguments()["ref"].(string)
	filePath, ok := req.GetArguments()["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	if excluded(filePath) {
		return to.ErrorResult(fmt.Errorf("this path is excluded from document tools"))
	}
	kind := docKind(filePath)
	if kind == "" {
		return to.ErrorResult(fmt.Errorf("%s is not a supported document type (pdf, png, jpg, webp, docx, xlsx); use get_file_content for plain files", filePath))
	}
	first, last, err := parsePageRange(getString(req, "pages"))
	if err != nil {
		return to.ErrorResult(err)
	}

	data, sha, err := fetchFile(owner, repo, ref, filePath)
	if err != nil {
		return to.ErrorResult(err)
	}
	pages, err := extractCached(ctx, data, sha, kind)
	if err != nil {
		return to.ErrorResult(err)
	}

	resp := textResponse{Path: filePath, TotalPages: len(pages), PagesIncluded: []int{}}
	var b strings.Builder
	for _, p := range pages {
		if p.Page < first || (last > 0 && p.Page > last) {
			continue
		}
		chunk := fmt.Sprintf("[page %d]\n%s\n", p.Page, p.Text)
		if b.Len()+len(chunk) > MaxTextResponseBytes {
			resp.Truncated = true
			break
		}
		b.WriteString(chunk)
		resp.PagesIncluded = append(resp.PagesIncluded, p.Page)
	}
	resp.Text = b.String()
	return to.TextResult(resp)
}

func getString(req mcp.CallToolRequest, key string) string {
	v, _ := req.GetArguments()[key].(string)
	return v
}
