package repo

import (
	"context"
	"encoding/base64"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
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
	GetRepositoryTreeToolName    = "get_repository_tree"
)

var (
	GetFileContentTool = mcp.NewTool(
		GetFileToolName,
		mcp.WithDescription("Get file content"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
	)

	CreateFileTool = mcp.NewTool(
		CreateFileToolName,
		mcp.WithDescription("Create file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.Content)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	UpdateFileTool = mcp.NewTool(
		UpdateFileToolName,
		mcp.WithDescription("Update file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.Content)),
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
)

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
	return to.TextResult(content)
}

func CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	filePath, _ := req.GetArguments()["filePath"].(string)
	content, _ := req.GetArguments()["content"].(string)
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	newBranchName, ok := req.GetArguments()["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}
	opt := forgejo_sdk.CreateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
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
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	sha, _ := req.GetArguments()["sha"].(string)
	newBranchName, ok := req.GetArguments()["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}
	opt := forgejo_sdk.UpdateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		SHA:     sha,
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
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
