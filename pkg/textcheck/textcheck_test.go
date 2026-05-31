package textcheck

import (
	"bytes"
	"testing"
)

func TestIsPlainText(t *testing.T) {
	// 9 KB of 'a' followed by a NUL — head is clean, tail is not.
	bigCleanHead := append(bytes.Repeat([]byte("a"), 9*1024), 0x00)

	// Head with a NUL at byte 4000 — should be classified binary.
	nulInHead := append(bytes.Repeat([]byte("a"), 4000), 0x00)
	nulInHead = append(nulInHead, bytes.Repeat([]byte("a"), 100)...)

	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"empty", []byte{}, true},
		{"ascii source", []byte("package main\n\nfunc main() {}\n"), true},
		{"utf8 multibyte", []byte("héllo 🌍 world"), true},
		{"utf8 bom", []byte{0xEF, 0xBB, 0xBF, 'h', 'i'}, true},
		{"png header", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0D}, false},
		{"utf16 le bom", []byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00}, false},
		{"nul at byte 4000", nulInHead, false},
		{"large file with clean head", bigCleanHead, true},
		{"invalid utf8", []byte{'a', 'b', 0xFF, 'c'}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPlainText(tc.in); got != tc.want {
				t.Errorf("IsPlainText(%q...) = %v, want %v", truncate(tc.in, 16), got, tc.want)
			}
		})
	}
}

func truncate(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}
