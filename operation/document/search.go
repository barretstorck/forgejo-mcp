package document

import (
	"context"
	"fmt"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// MaxSearchHits caps search_documents hits.
	MaxSearchHits = 10
	// SnippetRadius is the number of context chars kept on each side of a match.
	SnippetRadius = 150
	// MaxDocsPerSearch caps how many candidate documents a single repo-wide
	// search_documents call will examine before stopping the scan.
	MaxDocsPerSearch = 200
)

var SearchDocumentsTool = mcp.NewTool(
	SearchDocumentsToolName,
	mcp.WithDescription("Search text inside documents (PDF/images/DOCX/XLSX — scans are OCR'd) across a whole repository, or within one document when `filePath` is given. Case-insensitive substring match. Returns up to 10 hits as {path, page, snippet}. Repo-wide search examines at most 200 candidate documents per call (see limit_reached in the response). Use when you don't know which document holds the answer."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("ref", mcp.Description(params.Ref)),
	mcp.WithString("query", mcp.Required(), mcp.Description("Text to find (case-insensitive substring)")),
	mcp.WithString("filePath", mcp.Description("Restrict the search to this one document")),
)

type searchHit struct {
	Path    string `json:"path"`
	Page    int    `json:"page"`
	Snippet string `json:"snippet"`
}

type searchDocsResponse struct {
	Hits              []searchHit `json:"hits"`
	DocumentsSearched int         `json:"documents_searched"`
	DocumentsSkipped  int         `json:"documents_skipped"`
	Truncated         bool        `json:"truncated"`
	// LimitReached is true when the repo-wide scan stopped early because it
	// hit MaxDocsPerSearch, before examining every candidate document.
	LimitReached bool `json:"limit_reached"`
}

// resolveRef mirrors operation/repo/file.go's resolveRef: when ref is empty,
// look up the repository's default branch. Duplicated locally (rather than
// exported from the repo package) to keep the document package's dependency
// surface independent of operation/repo.
func resolveRef(owner, repo, ref string) (string, error) {
	if ref == "" {
		repoInfo, _, err := forgejo.Client().GetRepo(owner, repo)
		if err != nil {
			return "", fmt.Errorf("get repo err: %v", err)
		}
		ref = repoInfo.DefaultBranch
	}
	return ref, nil
}

func SearchDocumentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchDocumentsFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	query, ok := req.GetArguments()["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return to.ErrorResult(fmt.Errorf("query is required"))
	}
	ref, _ := req.GetArguments()["ref"].(string)
	onlyPath := getString(req, "filePath")

	// Collect candidate documents.
	type candidate struct{ path string }
	var candidates []candidate
	singleFile := onlyPath != ""
	if singleFile {
		candidates = []candidate{{onlyPath}}
	} else {
		treeRef, err := resolveRef(owner, repo, ref)
		if err != nil {
			return to.ErrorResult(err)
		}
		tree, _, err := forgejo.Client().GetTrees(owner, repo, treeRef, forgejo_sdk.GetTreesOptions{Recursive: true})
		if err != nil {
			return to.ErrorResult(fmt.Errorf("list repository files err: %v", err))
		}
		for _, e := range tree.Entries {
			if e.Type == "blob" && docKind(e.Path) != "" {
				candidates = append(candidates, candidate{e.Path})
			}
		}
	}

	queryLower := strings.ToLower(query)
	resp := searchDocsResponse{Hits: []searchHit{}}
	for i, c := range candidates {
		if !singleFile && i >= MaxDocsPerSearch {
			resp.DocumentsSkipped += len(candidates) - i
			resp.LimitReached = true
			break
		}
		if excluded(c.path) || docKind(c.path) == "" {
			resp.DocumentsSkipped++
			continue
		}
		data, sha, err := fetchFile(owner, repo, ref, c.path)
		if err != nil {
			if singleFile {
				return to.ErrorResult(err)
			}
			resp.DocumentsSkipped++
			continue
		}
		pages, err := extractCached(ctx, data, sha, docKind(c.path))
		if err != nil {
			if singleFile {
				return to.ErrorResult(err)
			}
			resp.DocumentsSkipped++
			continue
		}
		resp.DocumentsSearched++
		for _, p := range pages {
			idx := strings.Index(strings.ToLower(p.Text), queryLower)
			if idx < 0 {
				continue
			}
			if len(resp.Hits) >= MaxSearchHits {
				resp.Truncated = true
				break
			}
			resp.Hits = append(resp.Hits, searchHit{Path: c.path, Page: p.Page, Snippet: snippet(p.Text, idx, len(query))})
		}
		if resp.Truncated {
			break
		}
	}
	return to.TextResult(resp)
}

// snippet returns the match with up to SnippetRadius chars of context on
// each side, on rune-safe boundaries.
func snippet(text string, idx, matchLen int) string {
	start := idx - SnippetRadius
	if start < 0 {
		start = 0
	}
	end := idx + matchLen + SnippetRadius
	if end > len(text) {
		end = len(text)
	}
	for start > 0 && start < len(text) && (text[start]&0xC0) == 0x80 {
		start--
	}
	for end < len(text) && (text[end]&0xC0) == 0x80 {
		end++
	}
	s := strings.TrimSpace(text[start:end])
	s = strings.Join(strings.Fields(s), " ")
	return s
}
