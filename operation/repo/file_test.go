package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
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

func TestEncodeContent(t *testing.T) {
	// Lock test to a small, predictable cap so size-limit cases are easy to
	// reason about. Restore after.
	orig := flagPkg.MaxFileBytes
	flagPkg.MaxFileBytes = 100
	defer func() { flagPkg.MaxFileBytes = orig }()

	const helloB64 = "aGVsbG8=" // base64 of "hello"

	cases := []struct {
		name       string
		content    string
		encoding   string
		wantOut    string // expected SDK-ready base64; "" if wantErrSub != ""
		wantErrSub string // substring expected in error message; "" if no error expected
	}{
		// happy paths
		{"utf-8 default (empty encoding)", "hello", "", helloB64, ""},
		{"utf-8 explicit lowercase", "hello", "utf-8", helloB64, ""},
		{"utf-8 explicit uppercase", "hello", "UTF-8", helloB64, ""},
		{"utf-8 mixed case", "hello", "Utf-8", helloB64, ""},
		{"base64 valid", helloB64, "base64", helloB64, ""},
		{"base64 uppercase param", helloB64, "BASE64", helloB64, ""},
		{"empty utf-8 content", "", "utf-8", "", ""},
		{"empty base64 content", "", "base64", "", ""},

		// rejections
		{"unknown encoding", "hello", "ascii", "", `unsupported encoding "ascii"`},
		{"malformed base64", "not_valid_base64!", "base64", "", "invalid base64 content"},
		{"utf-8 exceeds size", string(make([]byte, 101)), "utf-8", "", "content exceeds size limit"},
		// base64 pre-decode guard: input length > 100*4/3+4 = 137
		{"base64 pre-decode oversize", strings.Repeat("A", 138), "base64", "", "base64-encoded, decoded would exceed"},
		// base64 within pre-decode guard but decoded > limit: 101 bytes decoded → 136 base64 chars (with padding "==")
		{"base64 post-decode oversize", base64.StdEncoding.EncodeToString(make([]byte, 101)), "base64", "", "content exceeds size limit"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := encodeContent(tc.content, tc.encoding)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (out=%q)", tc.wantErrSub, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantOut {
				t.Errorf("encodeContent(%q, %q) = %q, want %q",
					tc.content, tc.encoding, got, tc.wantOut)
			}
		})
	}
}

func TestCreateFileFn_Base64EncodingPassesThrough(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	encoded := base64.StdEncoding.EncodeToString(pngHeader)

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "logo.png",
		"content":     encoded,
		"encoding":    "base64",
		"message":     "add logo",
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
	if body.Content != encoded {
		t.Errorf("API received content = %q, want raw passthrough %q", body.Content, encoded)
	}
}

func TestCreateFileFn_UnknownEncodingReturnsError(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "test.txt",
		"content":     "hello",
		"encoding":    "ascii",
		"message":     "x",
		"branch_name": "main",
	})

	_, err := CreateFileFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unknown encoding, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported encoding "ascii"`) {
		t.Errorf("error = %q, want substring %q", err.Error(), `unsupported encoding "ascii"`)
	}
	if captured != nil && len(*captured) > 0 {
		t.Errorf("SDK was called despite validation failure; captured = %q", string(*captured))
	}
}

func TestCreateFileFn_DefaultEncodingStillBase64Encodes(t *testing.T) {
	// Back-compat: callers that don't pass `encoding` should still get the
	// original "content is plain text, server base64-encodes it" behavior.
	srv, captured := setupMockServer(t)
	defer srv.Close()

	plainText := "package main\n"
	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "main.go",
		"content":     plainText, // no encoding param
		"message":     "x",
		"branch_name": "main",
	})

	if _, err := CreateFileFn(context.Background(), req); err != nil {
		t.Fatalf("CreateFileFn returned error: %v", err)
	}

	var body apiFileRequest
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("unmarshaling captured body: %v", err)
	}
	expected := base64.StdEncoding.EncodeToString([]byte(plainText))
	if body.Content != expected {
		t.Errorf("API received content = %q, want base64(%q) = %q", body.Content, plainText, expected)
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
