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

func TestParseToolsEnabled(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{" , ,", nil},
		{"get_file_content", []string{"get_file_content"}},
		{" get_file_content , list_directory ", []string{"get_file_content", "list_directory"}},
	}
	for _, c := range cases {
		got := ParseToolsEnabled(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("ParseToolsEnabled(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("ParseToolsEnabled(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}
