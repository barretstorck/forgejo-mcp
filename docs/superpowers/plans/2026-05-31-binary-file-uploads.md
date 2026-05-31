# Binary File Uploads + PR #1 Review Polish — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `create_file` / `update_file` with an optional `encoding` parameter so agents can commit binary files (PDFs, images) to Forgejo repositories. Enforce a configurable decoded-size cap. Land the PR-#1 review-suggestion fixes in the same PR.

**Architecture:** New config knob (`flag.MaxFileBytes`, env `FORGEJO_MCP_MAX_FILE_BYTES`, default 25 MiB). New helper `encodeContent(content, encoding string) (sdkReady string, err error)` in `operation/repo/file.go` handles the validation/decoding/sizing — `CreateFileFn` and `UpdateFileFn` each gain a 4-line block that parses the param and calls the helper. No new packages. No new tools. No changes to `get_file_content`'s API surface (only the PR-#1 polish log line + extra tests).

**Tech Stack:** Go 1.21+, `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3`, `github.com/mark3labs/mcp-go/mcp`, stdlib `encoding/base64`, `strconv`, `strings`. Existing test infra: `net/http/httptest` + `forgejo.SetClientForTesting`.

**Spec:** `docs/superpowers/specs/2026-05-31-binary-file-uploads-design.md`

**Working branch:** `claude/exciting-mclean-1590dc` (the existing PR #1 branch — this plan adds commits on top of PR #1's four commits).

---

## File Structure

| Path | Status | Responsibility |
|---|---|---|
| `pkg/flag/flag.go` | modify | Add `MaxFileBytes` var, `DefaultMaxFileBytes` const, `ParseMaxFileBytes` parser. |
| `pkg/flag/flag_test.go` | create | Unit tests for `ParseMaxFileBytes`. |
| `cmd/cmd.go` | modify | Wire `FORGEJO_MCP_MAX_FILE_BYTES` env var in `initConfig`. |
| `operation/repo/file.go` | modify | Add `encodeContent` helper; add `encoding` param to `CreateFileTool` / `UpdateFileTool`; call helper from `CreateFileFn` / `UpdateFileFn`; add `log.Debugf` in `GetFileContentFn`. |
| `operation/repo/file_test.go` | modify | Add unit tests for `encodeContent`, integration smoke tests for the new branches in `CreateFileFn` / `UpdateFileFn`, and four PR-#1 polish tests for `GetFileContentFn`. |
| `operation/params/params.go` | modify | Reword `Content` description; add `Encoding` description. |
| `pkg/textcheck/textcheck_test.go` | modify | Add UTF-8 BOM case. |
| `README.md` | modify | Document `encoding` param + `FORGEJO_MCP_MAX_FILE_BYTES` env var. |

---

## Task 1: Add `MaxFileBytes` config + parser

**Files:**
- Modify: `pkg/flag/flag.go`
- Create: `pkg/flag/flag_test.go`

- [ ] **Step 1: Write the failing tests**

Create `pkg/flag/flag_test.go`:

```go
package flag

import "testing"

func TestParseMaxFileBytes(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantVal int64
		wantOK  bool
	}{
		{"empty uses default", "", DefaultMaxFileBytes, true},
		{"valid positive", "104857600", 104857600, true},
		{"valid min positive", "1", 1, true},
		{"zero is invalid", "0", DefaultMaxFileBytes, false},
		{"negative is invalid", "-1", DefaultMaxFileBytes, false},
		{"non-numeric is invalid", "foo", DefaultMaxFileBytes, false},
		{"trailing garbage is invalid", "100x", DefaultMaxFileBytes, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotVal, gotOK := ParseMaxFileBytes(tc.in)
			if gotVal != tc.wantVal || gotOK != tc.wantOK {
				t.Errorf("ParseMaxFileBytes(%q) = (%d, %v), want (%d, %v)",
					tc.in, gotVal, gotOK, tc.wantVal, tc.wantOK)
			}
		})
	}
}

func TestDefaultMaxFileBytes_Is25MiB(t *testing.T) {
	const want int64 = 25 * 1024 * 1024
	if DefaultMaxFileBytes != want {
		t.Errorf("DefaultMaxFileBytes = %d, want %d", DefaultMaxFileBytes, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /app/forgejo-mcp && go test ./pkg/flag/...
```

Expected: compile error — `DefaultMaxFileBytes`, `ParseMaxFileBytes` undefined.

- [ ] **Step 3: Modify `pkg/flag/flag.go`**

Replace the file contents with:

```go
package flag

import "strconv"

// DefaultMaxFileBytes is the default cap on decoded file content size for
// create_file / update_file. 25 MiB matches typical Forgejo MAX_UPLOAD_SIZE.
const DefaultMaxFileBytes int64 = 25 * 1024 * 1024

var (
	URL       string
	SSEPort   int
	HTTPPort  int
	Token     string
	Version   string
	UserAgent string

	Debug bool

	// MaxFileBytes is the cap on decoded file content size for create_file /
	// update_file. Set from FORGEJO_MCP_MAX_FILE_BYTES at startup.
	MaxFileBytes int64 = DefaultMaxFileBytes
)

// ParseMaxFileBytes parses a FORGEJO_MCP_MAX_FILE_BYTES env-var value.
// Empty input returns (default, true). Anything that doesn't parse as a
// positive int64 returns (default, false) so the caller can warn.
func ParseMaxFileBytes(s string) (int64, bool) {
	if s == "" {
		return DefaultMaxFileBytes, true
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return DefaultMaxFileBytes, false
	}
	return n, true
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /app/forgejo-mcp && go test ./pkg/flag/... -v
```

Expected: all subtests PASS, including `TestDefaultMaxFileBytes_Is25MiB`.

- [ ] **Step 5: Commit**

```bash
cd /app/forgejo-mcp
git add pkg/flag/flag.go pkg/flag/flag_test.go
git commit -m "feat(flag): add MaxFileBytes config + ParseMaxFileBytes

Default 25 MiB cap on decoded file content for upcoming create_file /
update_file binary-upload support. Parser is unit-tested; empty input
returns default cleanly so callers can distinguish 'unset' from
'invalid' and log accordingly."
```

---

## Task 2: Wire `FORGEJO_MCP_MAX_FILE_BYTES` in `cmd/cmd.go`

**Files:**
- Modify: `cmd/cmd.go` (inside `initConfig`, just before the closing brace)

This task has no unit test — `cmd/cmd.go` has no test file in this repo. The behavior is exercised end-to-end by the next task's integration tests, which set `flag.MaxFileBytes` directly. Manual smoke test below covers the env-var path.

- [ ] **Step 1: Read current `cmd/cmd.go` lines 170-200 for context**

```bash
cd /app/forgejo-mcp && sed -n '170,200p' cmd/cmd.go
```

You should see the `if debug` / `if !debug` blocks that read `FORGEJO_DEBUG`. The new block goes right after them, before `initConfig`'s closing brace.

- [ ] **Step 2: Add the env-var read**

Find the line in `cmd/cmd.go` where `initConfig` ends (look for the closing `}` after the last `if !debug` block — currently around line 200). Insert this block immediately before that closing brace:

```go
	// Max file bytes — caps decoded content for create_file / update_file.
	if envValue := os.Getenv("FORGEJO_MCP_MAX_FILE_BYTES"); envValue != "" {
		parsed, ok := flagPkg.ParseMaxFileBytes(envValue)
		if !ok {
			log.Warn("Invalid FORGEJO_MCP_MAX_FILE_BYTES value, using default",
				log.StringField("value", envValue),
				log.IntField("default_bytes", int(flagPkg.DefaultMaxFileBytes)),
			)
		} else {
			log.Debug("Using FORGEJO_MCP_MAX_FILE_BYTES environment variable",
				log.IntField("bytes", int(parsed)),
			)
		}
		flagPkg.MaxFileBytes = parsed
	}
```

Note: `flagPkg` is the existing local alias for `codeberg.org/goern/forgejo-mcp/v2/pkg/flag` already imported at the top of `cmd/cmd.go`. `log` is `codeberg.org/goern/forgejo-mcp/v2/pkg/log`, also already imported.

- [ ] **Step 3: Build to verify it compiles**

```bash
cd /app/forgejo-mcp && go build ./...
```

Expected: clean build.

- [ ] **Step 4: Manual smoke test of env-var parsing**

```bash
cd /app/forgejo-mcp && FORGEJO_MCP_MAX_FILE_BYTES=foo FORGEJO_URL=http://x FORGEJO_ACCESS_TOKEN=x ./forgejo-mcp --help 2>&1 | head -5
```

Expected: a `WARN` line mentioning `Invalid FORGEJO_MCP_MAX_FILE_BYTES value` somewhere in the output (the binary may exit early on `--help`; the warning fires during `initConfig` before that, so it should still appear). If `forgejo-mcp` binary doesn't exist yet, build it first: `make build`.

- [ ] **Step 5: Commit**

```bash
cd /app/forgejo-mcp
git add cmd/cmd.go
git commit -m "feat(cmd): read FORGEJO_MCP_MAX_FILE_BYTES env var

Wires the existing flag.ParseMaxFileBytes parser into initConfig.
Invalid values fall back to the 25 MiB default with a Warn log."
```

---

## Task 3: Add `encodeContent` helper with unit tests

**Files:**
- Modify: `operation/repo/file.go` (add helper near the top of the file, before `GetFileContentFn`)
- Modify: `operation/repo/file_test.go` (add test function `TestEncodeContent`)

- [ ] **Step 1: Write the failing tests**

Append to `operation/repo/file_test.go`:

```go
func TestEncodeContent(t *testing.T) {
	// Lock test to a small, predictable cap so size-limit cases are easy to
	// reason about. Restore after.
	orig := flagPkg.MaxFileBytes
	flagPkg.MaxFileBytes = 100
	defer func() { flagPkg.MaxFileBytes = orig }()

	const helloB64 = "aGVsbG8=" // base64 of "hello"

	cases := []struct {
		name      string
		content   string
		encoding  string
		wantOut   string // expected SDK-ready base64; "" if wantErrSub != ""
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
```

Add to the test file's import block (at top of `file_test.go`):

```go
	"strings"

	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
```

(Place `strings` in the stdlib group; place `flagPkg` in the third-party/internal group with the other `codeberg.org` imports. Use the local alias `flagPkg` to match `cmd/cmd.go`'s convention and avoid colliding with Go's built-in `flag` package.)

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestEncodeContent
```

Expected: compile error — `encodeContent` undefined.

- [ ] **Step 3: Implement the helper**

In `operation/repo/file.go`:

(a) Add `"strings"` to the import block (in the stdlib group, alongside `encoding/base64`, `context`, `fmt`).

(b) Add a `"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"` import to the third-party group — alias it as `flagPkg` to be consistent with `cmd/cmd.go`:

```go
	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
```

(c) Add the helper function immediately before `func GetFileContentFn` (currently around line 112):

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestEncodeContent -v
```

Expected: all 13 subtests PASS.

- [ ] **Step 5: Commit**

```bash
cd /app/forgejo-mcp
git add operation/repo/file.go operation/repo/file_test.go
git commit -m "feat(repo): add encodeContent helper for utf-8/base64 input

Pure function: validates encoding param (case-insensitive utf-8 or
base64), enforces flag.MaxFileBytes with a pre-decode size guard for
base64, and returns the SDK-ready base64 string. CreateFileFn /
UpdateFileFn will call this in the next commits.

13 table-driven test cases cover happy paths (both encodings, all
casings, empty content) and rejections (unknown encoding, malformed
base64, size limit at all three checkpoints)."
```

---

## Task 4: Wire `encodeContent` into `CreateFileFn`

**Files:**
- Modify: `operation/repo/file.go` — add `encoding` param to `CreateFileTool` definition; rewrite `CreateFileFn`'s content block to call `encodeContent`.
- Modify: `operation/repo/file_test.go` — add three integration tests.

- [ ] **Step 1: Write the failing tests**

Append to `operation/repo/file_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestCreateFileFn -v
```

Expected: the three new tests fail because `CreateFileFn` ignores `encoding` and unconditionally base64-encodes — so `TestCreateFileFn_Base64EncodingPassesThrough` will see a double-encoded payload, and `TestCreateFileFn_UnknownEncodingReturnsError` will see no error. The default-encoding test should already pass.

- [ ] **Step 3: Update the `CreateFileTool` definition**

In `operation/repo/file.go`, replace the existing `CreateFileTool = mcp.NewTool(...)` block (currently around lines 39-49) with:

```go
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
```

(`params.Encoding` is added in Task 8 — if you're running tasks out of order, add it to `params.go` first; otherwise this will fail to compile until Task 8. The integration tests in this task only use `encoding` as a request argument, not via the params constant, so they don't depend on Task 8.)

- [ ] **Step 4: Rewrite `CreateFileFn` to call `encodeContent`**

Replace the existing `CreateFileFn` body (currently lines 134-159) with:

```go
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
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestCreateFileFn -v
```

Expected: all four `TestCreateFileFn_*` tests PASS (the three new ones plus the pre-existing `TestCreateFileFn_Base64EncodesContent`).

- [ ] **Step 6: Commit**

```bash
cd /app/forgejo-mcp
git add operation/repo/file.go operation/repo/file_test.go
git commit -m "feat(repo): add encoding param to create_file for binary uploads

CreateFileFn now accepts an optional encoding param (utf-8 default,
base64 for pre-encoded binary content). Default behavior unchanged for
back-compat. Three integration tests cover the base64 passthrough,
unknown-encoding rejection, and the back-compat default path."
```

---

## Task 5: Wire `encodeContent` into `UpdateFileFn`

**Files:**
- Modify: `operation/repo/file.go` — `UpdateFileTool` definition + `UpdateFileFn`.
- Modify: `operation/repo/file_test.go` — three integration tests parallel to Task 4.

- [ ] **Step 1: Write the failing tests**

Append to `operation/repo/file_test.go`:

```go
func TestUpdateFileFn_Base64EncodingPassesThrough(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	pdfMagic := []byte{0x25, 0x50, 0x44, 0x46, 0x2D} // "%PDF-"
	encoded := base64.StdEncoding.EncodeToString(pdfMagic)

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "report.pdf",
		"content":     encoded,
		"encoding":    "base64",
		"message":     "update pdf",
		"branch_name": "main",
		"sha":         "prevsha",
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
	if body.Content != encoded {
		t.Errorf("API received content = %q, want raw passthrough %q", body.Content, encoded)
	}
}

func TestUpdateFileFn_MalformedBase64ReturnsError(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "x.bin",
		"content":     "not_valid_base64!",
		"encoding":    "base64",
		"message":     "x",
		"branch_name": "main",
		"sha":         "prevsha",
	})

	_, err := UpdateFileFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for malformed base64, got nil")
	}
	if !strings.Contains(err.Error(), "invalid base64 content") {
		t.Errorf("error = %q, want substring %q", err.Error(), "invalid base64 content")
	}
	if captured != nil && len(*captured) > 0 {
		t.Errorf("SDK was called despite validation failure")
	}
}

func TestUpdateFileFn_DefaultEncodingStillBase64Encodes(t *testing.T) {
	srv, captured := setupMockServer(t)
	defer srv.Close()

	plainText := "package main\n"
	req := newCallToolRequest(map[string]interface{}{
		"owner":       "testowner",
		"repo":        "testrepo",
		"filePath":    "main.go",
		"content":     plainText,
		"message":     "x",
		"branch_name": "main",
		"sha":         "prevsha",
	})

	if _, err := UpdateFileFn(context.Background(), req); err != nil {
		t.Fatalf("UpdateFileFn returned error: %v", err)
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestUpdateFileFn -v
```

Expected: the two new non-default tests fail; the default test passes.

- [ ] **Step 3: Update the `UpdateFileTool` definition**

Replace the existing `UpdateFileTool = mcp.NewTool(...)` block (currently around lines 51-62) with:

```go
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
```

- [ ] **Step 4: Rewrite `UpdateFileFn` to call `encodeContent`**

Replace the existing `UpdateFileFn` body (currently lines 161-188) with:

```go
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
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestUpdateFileFn -v
```

Expected: all four `TestUpdateFileFn_*` tests PASS (three new + pre-existing `TestUpdateFileFn_Base64EncodesContent`).

- [ ] **Step 6: Commit**

```bash
cd /app/forgejo-mcp
git add operation/repo/file.go operation/repo/file_test.go
git commit -m "feat(repo): add encoding param to update_file for binary uploads

UpdateFileFn now accepts the same optional encoding param as
CreateFileFn. Default behavior unchanged for back-compat. Three
integration tests cover base64 passthrough, malformed-base64
rejection, and the back-compat default path."
```

---

## Task 6: PR-#1 polish — `log.Debugf` + four `GetFileContentFn` tests

**Files:**
- Modify: `operation/repo/file.go` — add `log.Debugf` inside the existing decode branch in `GetFileContentFn`.
- Modify: `operation/repo/file_test.go` — four new tests covering nil-encoding, nil-content, already-utf8, and malformed-base64 fall-through.

- [ ] **Step 1: Write the failing tests**

Append to `operation/repo/file_test.go`. These tests need a flexible mock — extend `setupGetContentsMockServer` by adding a sibling helper that accepts already-encoded content and an explicit `encoding` field (or nil) so tests can inject the exact server response shape:

```go
// setupGetContentsMockServerRaw is a more flexible variant of
// setupGetContentsMockServer: the caller supplies the encoding pointer
// (nil for "not present") and the content string verbatim. Used by the
// PR-#1 polish tests that need to mock edge-case SDK responses.
func setupGetContentsMockServerRaw(t *testing.T, name, path, sha string, encoding *string, content *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"name": name,
			"path": path,
			"sha":  sha,
			"type": "file",
			"size": 0,
		}
		if encoding != nil {
			resp["encoding"] = *encoding
		}
		if content != nil {
			resp["content"] = *content
		}
		json.NewEncoder(w).Encode(resp)
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func TestGetFileContentFn_NilEncodingPassthrough(t *testing.T) {
	content := "anything"
	srv := setupGetContentsMockServerRaw(t, "x", "x", "sha", nil /*encoding*/, &content)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "x",
	})
	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("unexpected tool error")
	}
	// Encoding was nil in the SDK response, so the rewrite branch should be
	// skipped; the content field should come through untouched.
	enc, gotContent, _ := extractToolResultFields(t, result)
	if enc != "" {
		t.Errorf("encoding = %q, want \"\" (nil pointer in response)", enc)
	}
	if gotContent != content {
		t.Errorf("content = %q, want %q", gotContent, content)
	}
}

func TestGetFileContentFn_NilContentPassthrough(t *testing.T) {
	enc := "base64"
	srv := setupGetContentsMockServerRaw(t, "x", "x", "sha", &enc, nil /*content*/)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "x",
	})
	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("unexpected tool error")
	}
	// Just asserting we didn't panic; the content branch was nil so the
	// rewrite is skipped and the SDK response is forwarded as-is.
}

func TestGetFileContentFn_AlreadyUtf8Passthrough(t *testing.T) {
	enc := "utf-8"
	content := "already decoded"
	srv := setupGetContentsMockServerRaw(t, "x", "x", "sha", &enc, &content)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "x",
	})
	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	gotEnc, gotContent, _ := extractToolResultFields(t, result)
	if gotEnc != "utf-8" {
		t.Errorf("encoding = %q, want %q (passthrough)", gotEnc, "utf-8")
	}
	if gotContent != content {
		t.Errorf("content = %q, want %q (passthrough)", gotContent, content)
	}
}

func TestGetFileContentFn_MalformedBase64FallsThrough(t *testing.T) {
	enc := "base64"
	content := "@@@not-base64@@@"
	srv := setupGetContentsMockServerRaw(t, "x", "x", "sha", &enc, &content)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "ref": "main", "filePath": "x",
	})
	result, err := GetFileContentFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetFileContentFn returned error: %v", err)
	}
	// SDK said base64 but content didn't decode — handler should NOT panic
	// and should forward the original (broken) base64 string unchanged.
	gotEnc, gotContent, _ := extractToolResultFields(t, result)
	if gotEnc != "base64" {
		t.Errorf("encoding = %q, want %q (untouched on decode failure)", gotEnc, "base64")
	}
	if gotContent != content {
		t.Errorf("content = %q, want %q (untouched on decode failure)", gotContent, content)
	}
}
```

- [ ] **Step 2: Run tests to verify the malformed-base64 one fails (or all four pass — see note)**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestGetFileContentFn -v
```

Expected: all four pass already, since the function's existing logic already handles each case correctly (skip rewrite when nil, skip when already utf-8, fall through silently on decode error). The tests are pinning behavior so future refactors can't regress it — the missing-`log.Debugf` Step 3 below doesn't change behavior, only observability.

- [ ] **Step 3: Add the `log.Debugf` line**

In `operation/repo/file.go`, find the block in `GetFileContentFn` that the PR-#1 review flagged (currently around lines 132-145). It looks like:

```go
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
```

Replace it with the explicit-`else`-on-decode-error variant so the silent failure is logged:

```go
	if content != nil &&
		content.Encoding != nil && *content.Encoding == "base64" &&
		content.Content != nil {
		decoded, decErr := base64.StdEncoding.DecodeString(*content.Content)
		if decErr != nil {
			log.Debugf("get_file_content: SDK returned encoding=base64 but content failed to decode (%s/%s/%s): %v",
				owner, repo, filePath, decErr)
		} else if textcheck.IsPlainText(decoded) {
			s := string(decoded)
			enc := "utf-8"
			content.Content = &s
			content.Encoding = &enc
		}
	}
```

- [ ] **Step 4: Re-run tests to verify they still pass**

```bash
cd /app/forgejo-mcp && go test ./operation/repo/... -run TestGetFileContentFn -v
```

Expected: all `TestGetFileContentFn_*` tests PASS (the three from PR #1 plus the four new ones).

- [ ] **Step 5: Commit**

```bash
cd /app/forgejo-mcp
git add operation/repo/file.go operation/repo/file_test.go
git commit -m "fix(repo): log silent base64 decode failure in get_file_content

Addresses PR #1 review feedback. When the Forgejo SDK returns
encoding=base64 with content that doesn't actually decode (an SDK/
server contract violation), GetFileContentFn previously fell through
silently and returned the broken base64 string. Behavior is
unchanged, but the failure is now traceable via log.Debugf.

Also adds four guard tests that pin the existing nil-encoding,
nil-content, already-utf8, and malformed-base64 fall-through
behaviors that PR #1's review noted were uncovered."
```

---

## Task 7: Add UTF-8 BOM test case in `pkg/textcheck`

**Files:**
- Modify: `pkg/textcheck/textcheck_test.go`

- [ ] **Step 1: Read the current test cases**

```bash
cd /app/forgejo-mcp && sed -n '10,45p' pkg/textcheck/textcheck_test.go
```

You should see the `cases := []struct {...}` table.

- [ ] **Step 2: Add the BOM case**

In `pkg/textcheck/textcheck_test.go`, inside the `cases := []struct{...}{...}` literal, add this entry — placement-wise, put it right after the existing `{"utf8 multibyte", ...}` line so related cases group together:

```go
		{"utf8 bom", []byte{0xEF, 0xBB, 0xBF, 'h', 'i'}, true},
```

- [ ] **Step 3: Run the tests**

```bash
cd /app/forgejo-mcp && go test ./pkg/textcheck/... -v
```

Expected: all cases PASS, including the new `utf8_bom` case.

- [ ] **Step 4: Commit**

```bash
cd /app/forgejo-mcp
git add pkg/textcheck/textcheck_test.go
git commit -m "test(textcheck): pin UTF-8 BOM behavior

Addresses PR #1 review feedback. \xEF\xBB\xBF-prefixed files are
extremely common (Windows editors, MS-authored configs). They are
already correctly classified as plain text by IsPlainText, but the
behavior wasn't pinned; a future 'strip BOM' refactor could silently
regress it."
```

---

## Task 8: Update `params.go` descriptions

**Files:**
- Modify: `operation/params/params.go`

- [ ] **Step 1: Update the `Content` description and add `Encoding`**

In `operation/params/params.go`, find the `// File parameters` block (currently lines 28-35). Replace:

```go
	Content       = "Content (plain text, will be base64-encoded automatically)"
```

with:

```go
	Content       = "File content. Plain text when encoding=utf-8 (default); base64-encoded bytes when encoding=base64."
	Encoding      = "Content encoding: \"utf-8\" (plain text, default) or \"base64\" (pre-encoded bytes for binary files such as PDFs or images)."
```

(Add the `Encoding` line on its own line right after `Content`. Keep the existing alignment.)

- [ ] **Step 2: Build to verify it compiles**

```bash
cd /app/forgejo-mcp && go build ./...
```

Expected: clean build (Task 4 and Task 5 already reference `params.Encoding` in their tool definitions, so this is the missing piece).

- [ ] **Step 3: Run the full test suite**

```bash
cd /app/forgejo-mcp && go test ./...
```

Expected: all tests PASS. This is also a good checkpoint that the full PR (so far) hangs together.

- [ ] **Step 4: Commit**

```bash
cd /app/forgejo-mcp
git add operation/params/params.go
git commit -m "feat(params): add Encoding description; reword Content

Content no longer claims base64 encoding happens unconditionally —
the new encoding parameter determines that. Encoding is a new shared
description used by create_file and update_file."
```

---

## Task 9: Update README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Find the file/tool table updated by PR #1**

```bash
cd /app/forgejo-mcp && grep -n "get_file_content\|create_file\|FORGEJO" README.md | head -20
```

You should see the row PR #1 already updated for `get_file_content`, and a section listing `FORGEJO_*` environment variables (or, if there isn't one, the tool table near line 159).

- [ ] **Step 2: Update the `create_file` and `update_file` rows in the tool table**

Find:

```
| `create_file` | Create a new file |
| `update_file` | Update an existing file |
```

Replace with:

```
| `create_file` | Create a new file. Set `encoding` to `"base64"` for binary content (PDFs, images). |
| `update_file` | Update an existing file. Set `encoding` to `"base64"` for binary content. |
```

- [ ] **Step 3: Document the env var**

Search for an existing "Environment Variables" or "Configuration" section:

```bash
cd /app/forgejo-mcp && grep -n -i "environment\|FORGEJO_URL\|FORGEJO_ACCESS" README.md | head -10
```

If a table exists, add a row:

```
| `FORGEJO_MCP_MAX_FILE_BYTES` | `26214400` (25 MiB) | Maximum decoded byte size accepted by `create_file` / `update_file`. |
```

If no such table exists, add a short subsection after the tool table:

```markdown
### Limits

`create_file` and `update_file` reject content whose decoded size exceeds `FORGEJO_MCP_MAX_FILE_BYTES` (default `26214400` — 25 MiB). Set the env var to an integer number of bytes to raise or lower the cap.
```

- [ ] **Step 4: Commit**

```bash
cd /app/forgejo-mcp
git add README.md
git commit -m "docs: document encoding param + FORGEJO_MCP_MAX_FILE_BYTES"
```

---

## Task 10: Final verification

- [ ] **Step 1: Run the full suite one more time**

```bash
cd /app/forgejo-mcp && go test ./...
```

Expected: all PASS.

- [ ] **Step 2: Build**

```bash
cd /app/forgejo-mcp && go build ./...
```

Expected: clean.

- [ ] **Step 3: `go mod tidy`**

```bash
cd /app/forgejo-mcp && go mod tidy
```

Expected: no changes to `go.mod` / `go.sum` (no new external dependencies were added — only stdlib `strconv` and `strings`).

- [ ] **Step 4: `go vet`**

```bash
cd /app/forgejo-mcp && go vet ./...
```

Expected: no output (clean).

- [ ] **Step 5: Manual end-to-end smoke (optional but recommended)**

If you have access to a Forgejo instance:

```bash
cd /app/forgejo-mcp
make build
# In one shell, start the MCP server pointing at your Forgejo:
FORGEJO_URL=https://your-forgejo FORGEJO_ACCESS_TOKEN=xxx ./forgejo-mcp -stdio
```

Use an MCP client to call `create_file` with a small PDF, e.g.:
```
{"name":"create_file","arguments":{"owner":"you","repo":"sandbox","filePath":"hello.pdf","content":"<base64 of a tiny PDF>","encoding":"base64","message":"binary upload test","branch_name":"main"}}
```
Verify the file appears in the repo with the correct bytes.

- [ ] **Step 6: Push** (per AGENTS.md's "Landing the Plane" workflow)

```bash
cd /app/forgejo-mcp
git pull --rebase
git push
git status   # MUST show "up to date with origin"
```

---

## Self-Review Summary

**Spec coverage check** (mapping each spec section to tasks):

| Spec section | Tasks |
|---|---|
| `pkg/limits` or `pkg/flag` config | Task 1 (chose `pkg/flag` for plan time — flat-var consistency with existing code) |
| Env var wiring in `cmd/cmd.go` | Task 2 |
| `encoding` param on `create_file` | Task 4 (depends on Task 3 helper and Task 8 params) |
| `encoding` param on `update_file` | Task 5 (depends on Task 3 helper and Task 8 params) |
| Two-stage size check (pre-decode + post-decode) | Task 3 (in `encodeContent`) |
| Error messages | Task 3 (all five error surfaces in the helper) |
| PR-#1 `log.Debugf` polish | Task 6 |
| PR-#1 nil-encoding / nil-content / already-utf8 / malformed-base64 tests | Task 6 |
| UTF-8 BOM test | Task 7 |
| `params.Content` rewording + new `params.Encoding` | Task 8 |
| README updates | Task 9 |

Total commits: 9 (one per implementation task). Each task ends with a working, testable state.

**Test layout deviation from spec (intentional plan-time refinement):** The spec listed seven integration tests for `CreateFileFn` and seven for `UpdateFileFn` (14 total). This plan extracts an `encodeContent(content, encoding) (string, error)` helper (Task 3) and unit-tests it exhaustively (13 table-driven cases covering all encoding/size/validation paths), then keeps only three integration tests per handler (Tasks 4 & 5) to confirm the helper is wired correctly. Net test count: **24 cases** (13 unit + 6 integration + 4 PR-#1 polish + 1 BOM) vs. the spec's 18. Cleaner separation of concerns and faster to run. If you'd rather match the spec's layout exactly (no helper extraction), say so before execution — Tasks 3, 4, 5 would need a rewrite.

**Task ordering note:** Tasks 4 and 5 reference `params.Encoding` (added in Task 8). Tests in 4 and 5 don't depend on the constant — they pass `encoding` as a literal string in `Arguments`. The code added in 4 and 5 *does* reference `params.Encoding`, so if executed strictly in order, Tasks 4 and 5 will not build until Task 8 lands. This is intentional: it keeps the params change as a pure-rename commit that's easy to review. An executing agent that's running tasks sequentially should be fine since the test step in Task 4/5 runs `go test` on just the `operation/repo/...` package — but it WILL fail with a compile error. **Recommendation: do Task 8 before Task 4** (re-order during execution) if you prefer green at every step. The plan is written in this order to keep the spec/code/test mapping intuitive.
