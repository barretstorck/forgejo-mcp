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
