package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// apiFileRequest mirrors the Forgejo API request body for create/update file.
type apiFileRequest struct {
	Content string `json:"content"`
}

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// setupMockServer creates an httptest server that captures the request body
// and returns a minimal valid FileResponse. It returns the server and a
// pointer to the captured request body bytes.
func setupMockServer(t *testing.T) (*httptest.Server, *[]byte) {
	t.Helper()
	var captured []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		captured = body

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": map[string]interface{}{
				"name": "test.txt",
				"path": "test.txt",
				"sha":  "abc123",
			},
		})
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv, &captured
}

func TestCreateFileFn_Base64EncodesContent(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	plainText := "Hello, World!\nThis is a test file."

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "test.txt",
		"content":     plainText,
		"message":     "add test file",
		"branch_name": "main",
	})

	result, err := CreateFileFn(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateFileFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CreateFileFn returned tool error")
	}

	var body apiFileRequest
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("unmarshaling captured body: %v", err)
	}

	expected := base64.StdEncoding.EncodeToString([]byte(plainText))
	if body.Content != expected {
		decoded, _ := base64.StdEncoding.DecodeString(body.Content)
		t.Errorf("content sent to API is not correctly base64-encoded\n  got decoded: %q\n  want:        %q", string(decoded), plainText)
	}
}

// setupGetContentsMockServer creates an httptest server that returns the given
// raw bytes as a Forgejo ContentsResponse with base64-encoded content.
// It wires the test client through forgejo.SetClientForTesting.
func setupGetContentsMockServer(t *testing.T, name, path, sha string, raw []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoding := "base64"
		content := base64.StdEncoding.EncodeToString(raw)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":     name,
			"path":     path,
			"sha":      sha,
			"type":     "file",
			"size":     len(raw),
			"encoding": encoding,
			"content":  content,
		})
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

// extractToolResultFields unmarshals the JSON wrapper produced by to.TextResult
// (which is `{"Result": <ContentsResponse>}`) and returns the inner encoding and
// content fields. Both fields are pointer-typed in the SDK; the helper returns
// "" for nil pointers.
func extractToolResultFields(t *testing.T, result *mcp.CallToolResult) (encoding, content, sha string) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("tool result has no content blocks")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("tool result first block is %T, want mcp.TextContent", result.Content[0])
	}
	var wrapper struct {
		Result struct {
			Encoding *string `json:"encoding"`
			Content  *string `json:"content"`
			SHA      string  `json:"sha"`
		} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(tc.Text), &wrapper); err != nil {
		t.Fatalf("unmarshal tool result text %q: %v", tc.Text, err)
	}
	if wrapper.Result.Encoding != nil {
		encoding = *wrapper.Result.Encoding
	}
	if wrapper.Result.Content != nil {
		content = *wrapper.Result.Content
	}
	sha = wrapper.Result.SHA
	return encoding, content, sha
}

func TestGetFileContentFn_PlainTextDecoded(t *testing.T) {
	plainText := "package main\n\nfunc main() {}\n"
	srv := setupGetContentsMockServer(t, "main.go", "main.go", "sha1", []byte(plainText))
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"ref":      "main",
		"filePath": "main.go",
	})

	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetFileContentFn returned tool error")
	}

	encoding, content, sha := extractToolResultFields(t, result)
	if encoding != "utf-8" {
		t.Errorf("encoding = %q, want %q", encoding, "utf-8")
	}
	if content != plainText {
		t.Errorf("content = %q, want %q", content, plainText)
	}
	if sha != "sha1" {
		t.Errorf("sha = %q, want %q (preserved across plain-text decoding)", sha, "sha1")
	}
}

func TestGetFileContentFn_BinaryStaysBase64(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0D}
	srv := setupGetContentsMockServer(t, "logo.png", "img/logo.png", "sha2", pngHeader)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"ref":      "main",
		"filePath": "img/logo.png",
	})

	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetFileContentFn returned tool error")
	}

	encoding, content, sha := extractToolResultFields(t, result)
	if encoding != "base64" {
		t.Errorf("encoding = %q, want %q", encoding, "base64")
	}
	expected := base64.StdEncoding.EncodeToString(pngHeader)
	if content != expected {
		t.Errorf("content = %q, want %q (untouched base64)", content, expected)
	}
	if sha != "sha2" {
		t.Errorf("sha = %q, want %q (preserved across base64 passthrough)", sha, "sha2")
	}
}

func TestGetFileContentFn_EmptyFileBecomesUtf8(t *testing.T) {
	srv := setupGetContentsMockServer(t, "empty.txt", "empty.txt", "sha3", []byte{})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":    "testowner",
		"repo":     "testrepo",
		"ref":      "main",
		"filePath": "empty.txt",
	})

	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetFileContentFn returned tool error")
	}

	encoding, content, sha := extractToolResultFields(t, result)
	if encoding != "utf-8" {
		t.Errorf("encoding = %q, want %q", encoding, "utf-8")
	}
	if content != "" {
		t.Errorf("content = %q, want empty string", content)
	}
	if sha != "sha3" {
		t.Errorf("sha = %q, want %q", sha, "sha3")
	}
}

func TestUpdateFileFn_Base64EncodesContent(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	plainText := "Updated content with special chars: <>&\"\n\ttabs too"

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "test.txt",
		"content":     plainText,
		"message":     "update test file",
		"branch_name": "main",
		"sha":         "abc123",
	})

	result, err := UpdateFileFn(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateFileFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("UpdateFileFn returned tool error")
	}

	var body apiFileRequest
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("unmarshaling captured body: %v", err)
	}

	expected := base64.StdEncoding.EncodeToString([]byte(plainText))
	if body.Content != expected {
		decoded, _ := base64.StdEncoding.DecodeString(body.Content)
		t.Errorf("content sent to API is not correctly base64-encoded\n  got decoded: %q\n  want:        %q", string(decoded), plainText)
	}
}
