# Binary File Uploads + PR #1 Review Polish

**Date:** 2026-05-31
**Author:** brainstorm session
**Status:** design, awaiting approval

## Problem

PR #1 (`feat(repo): return plain-text file content as utf-8 in get_file_content`) gave agents a usable text-reading path: plain UTF-8 in the response, base64 only for binary. Two gaps remain:

1. PR #1 picked up a small set of review suggestions that should land alongside it (a missing `log.Debugf` on a swallowed decode error, plus several missing test cases for guard-clause branches).
2. **Agents cannot upload binary files.** `create_file` and `update_file` accept only a plain-text `content` parameter (`Content (plain text, will be base64-encoded automatically)`). An agent that has generated or downloaded a PDF and wants to commit it to a repository has no path forward.

## Scope

One combined PR, layered on PR #1's branch:

- Land the PR-#1 review polish (one log line + four new tests for existing code).
- Extend `create_file` and `update_file` with an optional `encoding` parameter so agents can submit pre-encoded base64 (i.e. binary) content.
- Enforce a configurable size cap on file payloads at both `encoding` modes.
- No changes to `get_file_content` beyond the review polish — the read side is intentionally out of scope (see Rejected Alternatives below).

Estimated diff: ~350–400 lines added across 7 files.

## Non-goals

- Server-side filesystem writes on download (`dest_path` on `get_file_content`). Rejected because this repo's deployment uses HTTP-transport MCP where client and server do not share a filesystem; the feature would only help shared-FS setups. Revisit if a concrete need surfaces.
- A `source_path` upload mode (agent passes a local file path, server reads bytes). Same shared-filesystem constraint.
- New dedicated `create_binary_file` / `update_binary_file` tools. The `encoding` parameter expresses the same intent with half the tool-list surface area and stays symmetric with the response shape PR #1 established.

## Design

### Architecture

Two existing packages touched, one new tiny package:

- **`pkg/limits`** (new, ~15 lines + tests). Reads `FORGEJO_MCP_MAX_FILE_BYTES` once at startup (default `25 * 1024 * 1024`). Exposes `MaxFileBytes() int64`. Config wired in `cmd/cmd.go`'s `initConfig()` next to the other `FORGEJO_*` env reads. If the env value parses as a non-positive number, log a warning and fall back to the default.
- **`pkg/textcheck`** (already added by PR #1). Unchanged.
- **`operation/repo/file.go`**. Tool definitions and handlers gain the `encoding` param plus the size + decode logic.

No `pkg/safepath` or `pkg/fileio` — neither is needed once `dest_path` is dropped.

### API changes

#### `create_file` and `update_file`

Add one optional string parameter to each:

```
encoding: "utf-8" | "base64"   (optional; default "utf-8"; case-insensitive)
```

Tool description for `create_file` / `update_file` updates to state:

> Create/update a file. The `encoding` parameter controls how `content` is interpreted: `"utf-8"` (default) treats `content` as plain text and the server base64-encodes it before sending to Forgejo; `"base64"` treats `content` as already-base64-encoded bytes and passes them through (use this for binary files such as PDFs or images). Decoded content larger than the server's size cap (default 25 MiB, see FORGEJO_MCP_MAX_FILE_BYTES) is rejected.

`operation/params/params.go`:
- Replace `Content = "Content (plain text, will be base64-encoded automatically)"` with `Content = "File content. Plain text when encoding=utf-8 (default); base64-encoded bytes when encoding=base64."`
- Add `Encoding = "Content encoding: \"utf-8\" (plain text, default) or \"base64\" (pre-encoded bytes for binary files)."`

#### `get_file_content`

No API change. The review-polish work (below) is internal.

### Handler ordering inside `CreateFileFn` / `UpdateFileFn`

1. Parse args (`owner`, `repo`, `filePath`, `content`, `message`, `branch_name`, `sha` for update, `new_branch_name`).
2. Parse `encoding`. Missing or empty → `"utf-8"`. Lowercase. Anything other than `"utf-8"` or `"base64"` → tool error `unsupported encoding %q: want "utf-8" or "base64"`.
3. **Pre-decode size guard.** Let `max := limits.MaxFileBytes()`.
   - If `encoding == "utf-8"`: `if int64(len(content)) > max → tool error content exceeds size limit (%d > %d bytes)`. The plain-text size equals the decoded size, so no further check needed.
   - If `encoding == "base64"`: `if int64(len(content)) > max*4/3+4 → tool error content exceeds size limit (base64-encoded, decoded would exceed %d bytes)`. The `+4` covers padding rounding; `*4/3` is the exact base64 inflation ratio.
4. **Encode for SDK.**
   - If `encoding == "utf-8"`: `sdkContent := base64.StdEncoding.EncodeToString([]byte(content))` (today's behavior).
   - If `encoding == "base64"`: validate by decoding with `base64.StdEncoding.DecodeString(content)`. Malformed → tool error `invalid base64 content: %v`. **Post-decode size check:** `if int64(len(decoded)) > max → same content-exceeds-size-limit error`. On success the original `content` string is passed straight through to the SDK (no re-encode round-trip; the decode result is discarded after validation).
5. Build `forgejo_sdk.CreateFileOptions` / `UpdateFileOptions` and call the SDK as today.

### Error handling

All errors flow through the existing `to.ErrorResult(fmt.Errorf(...))` pattern. No panics. No silent fallthroughs. Three new error surfaces on `create_file` / `update_file`:

| Condition | Message |
|---|---|
| Unknown `encoding` value | `unsupported encoding %q: want "utf-8" or "base64"` |
| Malformed base64 | `invalid base64 content: %v` |
| Size cap exceeded (utf-8) | `content exceeds size limit (%d > %d bytes)` |
| Size cap exceeded (base64 pre-decode) | `content exceeds size limit (base64-encoded, decoded would exceed %d bytes)` |
| Size cap exceeded (base64 post-decode) | `content exceeds size limit (%d > %d bytes)` |

**PR-#1 review fix in `GetFileContentFn`** (operation/repo/file.go, currently line 134 in the PR branch): inside the `if decoded, decErr := base64.StdEncoding.DecodeString(*content.Content); decErr == nil && ... {}` chain, when `decErr != nil` add a `log.Debugf("get_file_content: SDK returned encoding=base64 but content failed to decode (%s/%s/%s): %v", owner, repo, filePath, decErr)`. Behavior is unchanged — the function still falls through and returns the original base64 string to the agent — but the SDK/server contract violation is now traceable.

### Configuration

`cmd/cmd.go` `initConfig()` gains an env-var read for `FORGEJO_MCP_MAX_FILE_BYTES`. Pattern follows the existing `FORGEJO_DEBUG` block: read env, validate, log on use, fall back on parse failure. Parsed value stored in `pkg/limits` (or `pkg/flag`, depending on which feels cleaner once we look at `pkg/flag`'s shape during implementation — both are fine; this is a plan-time decision).

README gets a short table row in the existing "Environment Variables" section (if one exists; otherwise added inline near the tool docs):

| Variable | Default | Description |
|---|---|---|
| `FORGEJO_MCP_MAX_FILE_BYTES` | `26214400` (25 MiB) | Maximum decoded byte size for `create_file` / `update_file` content. |

## Testing

### New tests in `operation/repo/file_test.go`

For `create_file`:
- `TestCreateFileFn_Base64Passthrough` — `encoding="base64"` with valid base64 of binary bytes (e.g. PNG header). SDK receives those exact bytes in `opt.Content`.
- `TestCreateFileFn_InvalidBase64Errors` — `encoding="base64"` with malformed input. Returns tool error; SDK is never called (assert via the mock server receiving zero requests).
- `TestCreateFileFn_UnknownEncodingErrors` — `encoding="xyz"`. Tool error; SDK not called.
- `TestCreateFileFn_CaseInsensitiveEncoding` — table test covering `"UTF-8"`, `"utf-8"`, `"Base64"`, `"BASE64"`. All accepted; behavior matches lowercase equivalents.
- `TestCreateFileFn_OverSizeLimit_Utf8` — content larger than the configured cap. Tool error; SDK not called.
- `TestCreateFileFn_OverSizeLimit_Base64Pre` — base64 input whose length alone exceeds the inflation-adjusted cap. Pre-decode rejection.
- `TestCreateFileFn_OverSizeLimit_Base64Post` — base64 input whose length is just under the pre-decode cap but decodes to bytes over the post-decode cap (edge case the two-stage check exists for).

Same seven tests for `update_file`.

Tests should temporarily set the size cap via the `pkg/limits` test hook (a `SetForTesting(n int64) (restore func())` helper to avoid leaking state between tests) so they don't depend on env vars or the default size.

### PR-#1 review-suggestion tests (also in `operation/repo/file_test.go`)

- `TestGetFileContentFn_NilEncodingPassthrough` — mock server returns `encoding: null`. Handler doesn't panic; response forwarded unchanged.
- `TestGetFileContentFn_NilContentPassthrough` — mock server returns `content: null`. Same.
- `TestGetFileContentFn_AlreadyUtf8Passthrough` — mock server returns `encoding: "utf-8"` directly. Handler skips the rewrite branch; content forwarded unchanged.
- `TestGetFileContentFn_MalformedBase64Logged` — mock server returns `encoding: "base64"` with `content: "@@@not-base64@@@"`. Handler falls through (returns original string) without panicking. We don't assert on log output; the test exists to guard against panic and to lock in fall-through behavior.

### New tests in `pkg/textcheck/textcheck_test.go`

- One new case: `{"utf8 bom", []byte{0xEF, 0xBB, 0xBF, 'h', 'i'}, true}` — pins behavior so a future "strip BOM" refactor can't silently regress this. Other PR-#1 review suggestions (single-NUL case, leading-NUL case) are covered transitively by existing cases and are not added.

### New tests in `pkg/limits/limits_test.go`

- `TestMaxFileBytes_Default` — env unset → `25 * 1024 * 1024`.
- `TestMaxFileBytes_FromEnv` — `FORGEJO_MCP_MAX_FILE_BYTES=104857600` → 100 MiB.
- `TestMaxFileBytes_InvalidEnvFallsBack` — `FORGEJO_MCP_MAX_FILE_BYTES=foo` → default (and warning logged — we don't assert on the log, just the resulting value).
- `TestMaxFileBytes_NegativeEnvFallsBack` — `FORGEJO_MCP_MAX_FILE_BYTES=-1` → default.

Tests use `t.Setenv` to avoid leaking state.

## File-by-file diff list

```
operation/repo/file.go            +encoding param on 2 tools; +size + decode validation; +log.Debugf in GetFileContentFn
operation/repo/file_test.go       +18 tests (14 for upload, 4 for PR-#1 polish on read)
operation/params/params.go        reword Content; add Encoding
pkg/textcheck/textcheck_test.go   +1 BOM test case
pkg/limits/limits.go              new (~15 lines)
pkg/limits/limits_test.go         new (~50 lines)
cmd/cmd.go                        wire FORGEJO_MCP_MAX_FILE_BYTES into initConfig
README.md                         document encoding param + size cap env var
```

## Rejected alternatives

| Alternative | Why rejected |
|---|---|
| `dest_path` on `get_file_content` (server writes decoded bytes to a local path on download) | Useful only when client + server share a filesystem; this project's deployment uses HTTP-transport MCP where they don't. |
| `source_path` on `create_file` / `update_file` (server reads from a local path on upload) | Same shared-FS constraint. |
| New `create_binary_file` / `update_binary_file` tools | Doubles file-tool count for no expressive gain over the `encoding` param. |
| Lenient encoding validation (silently fall back to `utf-8` on unknown values) | Masks the bug where an agent intended binary and got their PDF committed as garbled text. |
| Autodetect mode (`encoding="auto"`) | Footgun: legitimate text that happens to be valid base64 (rare but real) gets silently decoded. |
| No size cap | Risk of unbounded memory allocation on a multi-GB base64 string. |
| Decode-then-check size | Allocates the full decoded buffer before rejecting. The two-stage check rejects oversize input before decode. |

## Open questions

None. All decisions resolved during brainstorming.

## References

- PR #1: https://github.com/barretstorck/forgejo-mcp/pull/1 — `feat(repo): return plain-text file content as utf-8 in get_file_content`
- AGENTS.md — project conventions for tool layout, response formatting, env-var config
- `pkg/textcheck/textcheck.go` (from PR #1) — plain-text classifier
