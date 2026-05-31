package repo

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/textcheck"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetFileToolName       = "get_file_content"
	CreateFileToolName    = "create_file"
	UpdateFileToolName    = "update_file"
	DeleteFileToolName    = "delete_file"
	ListDirectoryToolName      = "list_directory"
	GetDirectoryContentToolName  = "get_directory_content"
	GetRepositoryTreeToolName       = "get_repository_tree"
	SearchRepositoryContentsToolName = "search_repository_contents"
)

var (
	GetFileContentTool = mcp.NewTool(
		GetFileToolName,
		mcp.WithDescription("Get file content. The response's `encoding` field is `\"utf-8\"` when the file is plain-text (content is the decoded string) or `\"base64\"` when the file is binary (content is base64-encoded bytes). The `sha` field is preserved in both cases for use with update_file."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
	)

	CreateFileTool = mcp.NewTool(
		CreateFileToolName,
		mcp.WithDescription("Create file. The `encoding` parameter controls how `content` is interpreted: `\"utf-8\"` (default) treats content as plain text and the server base64-encodes it; `\"base64\"` treats content as already-base64-encoded bytes and passes them through (use this for binary files such as PDFs or images). Decoded content larger than the server's size cap (default 25 MiB, see FORGEJO_MCP_MAX_FILE_BYTES) is rejected."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.Content)),
		mcp.WithString("encoding", mcp.Description(params.Encoding)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	UpdateFileTool = mcp.NewTool(
		UpdateFileToolName,
		mcp.WithDescription("Update file. The `encoding` parameter controls how `content` is interpreted: `\"utf-8\"` (default) treats content as plain text and the server base64-encodes it; `\"base64\"` treats content as already-base64-encoded bytes and passes them through (use this for binary files such as PDFs or images). Decoded content larger than the server's size cap (default 25 MiB, see FORGEJO_MCP_MAX_FILE_BYTES) is rejected."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.Content)),
		mcp.WithString("encoding", mcp.Description(params.Encoding)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("sha", mcp.Required(), mcp.Description(params.SHA)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	DeleteFileTool = mcp.NewTool(
		DeleteFileToolName,
		mcp.WithDescription("Delete file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("sha", mcp.Required(), mcp.Description(params.SHA)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	ListDirectoryTool = mcp.NewTool(
		ListDirectoryToolName,
		mcp.WithDescription("List contents of a directory in a repository. Returns file and directory names, types, paths, and sizes."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("path", mcp.Description("Directory path relative to repo root. Empty or omitted for root directory.")),
		mcp.WithString("ref", mcp.Description(params.Ref)),
	)
	GetDirectoryContentTool = mcp.NewTool(
		GetDirectoryContentToolName,
		mcp.WithDescription("List directory contents with full metadata including SHA, download URL, and HTML URL for each entry."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("path", mcp.Description("Directory path relative to repo root. Empty or omitted for root directory.")),
		mcp.WithString("ref", mcp.Description(params.Ref)),
	)

	GetRepositoryTreeTool = mcp.NewTool(
		GetRepositoryTreeToolName,
		mcp.WithDescription("Get the full file tree of a repository. Returns all files and directories, optionally recursive. Useful for understanding repository structure."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Description(params.Ref)),
		mcp.WithBoolean("recursive", mcp.Description("Recurse into subdirectories. Default: true.")),
	)

	SearchRepositoryContentsTool = mcp.NewTool(
		SearchRepositoryContentsToolName,
		mcp.WithDescription("Search for files by name within a repository. Performs case-insensitive matching against file paths in the repository tree."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query to match against file paths (case-insensitive)")),
		mcp.WithString("ref", mcp.Description(params.Ref)),
	)
)

// encodeContent converts agent-supplied content into the base64 form the
// Forgejo SDK expects. encoding is case-insensitive; supported values are
// "utf-8" (default; treats content as plain text and base64-encodes it) and
// "base64" (treats content as already-encoded bytes and validates them).
// Decoded payload size is checked against flagPkg.MaxFileBytes; oversize
// input is rejected, and the base64 branch does a cheap pre-decode size
// guard so multi-GB payloads are rejected without allocating the decoded
// buffer.
func encodeContent(content, encoding string) (string, error) {
	enc := strings.ToLower(encoding)
	if enc == "" {
		enc = "utf-8"
	}
	max := flagPkg.MaxFileBytes
	switch enc {
	case "utf-8":
		if int64(len(content)) > max {
			return "", fmt.Errorf("content exceeds size limit (%d > %d bytes)", len(content), max)
		}
		return base64.StdEncoding.EncodeToString([]byte(content)), nil
	case "base64":
		// Pre-decode guard: base64 inflates by 4/3. Reject obvious oversize
		// before allocating the decoded buffer. The +4 covers padding rounding.
		if int64(len(content)) > max*4/3+4 {
			return "", fmt.Errorf("content exceeds size limit (base64-encoded, decoded would exceed %d bytes)", max)
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return "", fmt.Errorf("invalid base64 content: %v", err)
		}
		if int64(len(decoded)) > max {
			return "", fmt.Errorf("content exceeds size limit (%d > %d bytes)", len(decoded), max)
		}
		return content, nil
	default:
		return "", fmt.Errorf("unsupported encoding %q: want \"utf-8\" or \"base64\"", encoding)
	}
}

func GetFileContentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetFileFn")
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
	content, _, err := forgejo.Client().GetContents(owner, repo, ref, filePath)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get file err: %v", err))
	}

	// If the SDK returned base64-encoded content, decode it and — when the
	// bytes are plain UTF-8 text — replace the response's content/encoding
	// fields so the agent receives readable text instead of base64.
	if content != nil &&
		content.Encoding != nil && *content.Encoding == "base64" &&
		content.Content != nil {
		if decoded, decErr := base64.StdEncoding.DecodeString(*content.Content); decErr == nil &&
			textcheck.IsPlainText(decoded) {
			s := string(decoded)
			enc := "utf-8"
			content.Content = &s
			content.Encoding = &enc
		}
	}

	return to.TextResult(content)
}

func CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	filePath, _ := req.GetArguments()["filePath"].(string)
	content, _ := req.GetArguments()["content"].(string)
	encoding, _ := req.GetArguments()["encoding"].(string)
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	newBranchName, ok := req.GetArguments()["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}

	sdkContent, err := encodeContent(content, encoding)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.CreateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		Content: sdkContent,
	}
	fileResp, _, err := forgejo.Client().CreateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create file error: %v", err))
	}
	return to.TextResult(fileResp)
}

func UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateFileFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	filePath, _ := req.GetArguments()["filePath"].(string)
	content, _ := req.GetArguments()["content"].(string)
	encoding, _ := req.GetArguments()["encoding"].(string)
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	sha, _ := req.GetArguments()["sha"].(string)
	newBranchName, ok := req.GetArguments()["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}

	sdkContent, err := encodeContent(content, encoding)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.UpdateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		SHA:     sha,
		Content: sdkContent,
	}
	fileResp, _, err := forgejo.Client().UpdateFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update file error: %v", err))
	}
	return to.TextResult(fileResp)
}

func DeleteFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteFileFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	filePath, ok := req.GetArguments()["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	sha, ok := req.GetArguments()["sha"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("sha is required"))
	}
	opt := forgejo_sdk.DeleteFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:    message,
			BranchName: branchName,
		},
		SHA: sha,
	}
	_, err := forgejo.Client().DeleteFile(owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete file err: %v", err))
	}
	return to.TextResult("Delete file success")
}

func ListDirectoryFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListDirectoryFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	path, _ := req.GetArguments()["path"].(string)
	ref, _ := req.GetArguments()["ref"].(string)

	contents, _, err := forgejo.Client().ListContents(owner, repo, ref, path)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list directory err: %v", err))
	}

	type entry struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	entries := make([]entry, len(contents))
	for i, c := range contents {
		entries[i] = entry{Name: c.Name, Type: c.Type, Path: c.Path, Size: c.Size}
	}
	return to.TextResult(entries)
}

func GetDirectoryContentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetDirectoryContentFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	path, _ := req.GetArguments()["path"].(string)
	ref, _ := req.GetArguments()["ref"].(string)

	contents, _, err := forgejo.Client().ListContents(owner, repo, ref, path)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get directory content err: %v", err))
	}
	return to.TextResult(contents)
}

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

func GetRepositoryTreeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetRepositoryTreeFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	ref, _ := req.GetArguments()["ref"].(string)
	recursive, ok := req.GetArguments()["recursive"].(bool)
	if !ok {
		recursive = true
	}

	ref, err := resolveRef(owner, repo, ref)
	if err != nil {
		return to.ErrorResult(err)
	}

	opts := forgejo_sdk.GetTreesOptions{Recursive: recursive}
	tree, _, err := forgejo.Client().GetTrees(owner, repo, ref, opts)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get repository tree err: %v", err))
	}
	return to.TextResult(tree)
}

func SearchRepositoryContentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchRepositoryContentsFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	query, ok := req.GetArguments()["query"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("query is required"))
	}
	ref, _ := req.GetArguments()["ref"].(string)

	ref, err := resolveRef(owner, repo, ref)
	if err != nil {
		return to.ErrorResult(err)
	}

	opts := forgejo_sdk.GetTreesOptions{Recursive: true}
	tree, _, err := forgejo.Client().GetTrees(owner, repo, ref, opts)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search repository contents err: %v", err))
	}

	queryLower := strings.ToLower(query)
	type match struct {
		Path string `json:"path"`
		Type string `json:"type"`
		Size int64  `json:"size"`
	}
	var matches []match
	for _, entry := range tree.Entries {
		if strings.Contains(strings.ToLower(entry.Path), queryLower) {
			matches = append(matches, match{Path: entry.Path, Type: entry.Type, Size: entry.Size})
		}
	}
	return to.TextResult(matches)
}
