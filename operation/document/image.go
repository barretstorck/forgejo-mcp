package document

import (
	"context"
	"encoding/base64"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/extract"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	DefaultImageMaxEdge = 1280
	HardImageMaxEdge    = 2048
)

var GetDocumentPageImageTool = mcp.NewTool(
	GetDocumentPageImageToolName,
	mcp.WithDescription("Render one page of a PDF (or an image file) as a PNG for visual inspection — layouts, floor plans, diagrams, anything OCR mangles. Returns MCP image content. Not useful for plain text — prefer get_document_text."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("ref", mcp.Description(params.Ref)),
	mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
	mcp.WithNumber("page", mcp.Description("Page number (1-based) for PDFs. Default 1. Ignored for image files.")),
	mcp.WithNumber("max_edge_px", mcp.Description("Longest output edge in pixels. Default 1280, max 2048.")),
)

func GetDocumentPageImageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetDocumentPageImageFn")
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
	pageArg, _ := req.GetArguments()["page"].(float64)
	page := int(pageArg)
	if page < 1 {
		page = 1
	}
	edgeArg, _ := req.GetArguments()["max_edge_px"].(float64)
	maxEdge := int(edgeArg)
	if maxEdge <= 0 {
		maxEdge = DefaultImageMaxEdge
	}
	// The max_edge_px cap must be checked before any fetch/network call so
	// an oversize request fails fast without hitting the Forgejo API.
	if maxEdge > HardImageMaxEdge {
		return to.ErrorResult(fmt.Errorf("max_edge_px %d exceeds the maximum of %d", maxEdge, HardImageMaxEdge))
	}
	if excluded(filePath) {
		return to.ErrorResult(fmt.Errorf("this path is excluded from document tools"))
	}

	kind := docKind(filePath)
	var png []byte
	switch kind {
	case "pdf":
		data, _, err := fetchFile(owner, repo, ref, filePath)
		if err != nil {
			return to.ErrorResult(err)
		}
		png, err = ext.RenderPDFPagePNG(ctx, data, page, maxEdge)
		if err != nil {
			return to.ErrorResult(err)
		}
	case "image":
		data, _, err := fetchFile(owner, repo, ref, filePath)
		if err != nil {
			return to.ErrorResult(err)
		}
		png, err = extract.NormalizeImagePNG(data, maxEdge)
		if err != nil {
			return to.ErrorResult(err)
		}
	default:
		return to.ErrorResult(fmt.Errorf("%s: page images are only available for PDF and image files", filePath))
	}
	return mcp.NewToolResultImage("page image", base64.StdEncoding.EncodeToString(png), "image/png"), nil
}
