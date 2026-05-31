// Package textcheck classifies file bytes as plain UTF-8 text or binary.
package textcheck

import "unicode/utf8"

// SniffSize is the number of leading bytes inspected by IsPlainText.
const SniffSize = 8 * 1024

// IsPlainText reports whether b appears to be UTF-8 plain text:
// the first SniffSize bytes (or all of b, if shorter) are valid UTF-8
// and contain no NUL byte. This is the same heuristic git uses to
// distinguish text files from binary files.
func IsPlainText(b []byte) bool {
	head := b
	if len(head) > SniffSize {
		head = head[:SniffSize]
	}
	for _, c := range head {
		if c == 0 {
			return false
		}
	}
	return utf8.Valid(head)
}
