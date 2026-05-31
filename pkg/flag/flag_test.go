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
