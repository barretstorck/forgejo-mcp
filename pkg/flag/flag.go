package flag

import (
	"strconv"
	"strings"
)

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

	// ToolsEnabled is the tool allowlist from TOOLS_ENABLED. nil or empty
	// means all tools are exposed (backward compatible).
	ToolsEnabled []string

	// DocumentExcludeGlobs excludes matching paths from document extraction
	// and search. Patterns: path.Match syntax against the full repo path,
	// plus "dir/**" prefix patterns.
	DocumentExcludeGlobs []string
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

// ParseToolsEnabled splits a comma-separated tool allowlist, trimming
// whitespace and dropping empty items. Empty input returns nil.
func ParseToolsEnabled(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseExcludeGlobs splits a comma-separated glob list, trimming
// whitespace and dropping empty items.
func ParseExcludeGlobs(s string) []string {
	return ParseToolsEnabled(s) // same parsing semantics
}
