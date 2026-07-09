package document

import (
	"os"
	"testing"
)

// requireBinary skips the test when the named CLI tool is missing, unless
// REQUIRE_DOC_TOOLS is set (the containerized suite), where absence is a
// failure. Duplicated from pkg/extract's test helper of the same name —
// test helpers can't cross packages.
func requireBinary(t *testing.T, path, name string) {
	t.Helper()
	if path != "" {
		return
	}
	if os.Getenv("REQUIRE_DOC_TOOLS") != "" {
		t.Fatalf("%s not found in test container — image is broken", name)
	}
	t.Skipf("%s not installed; run via make test-docker for full coverage", name)
}
