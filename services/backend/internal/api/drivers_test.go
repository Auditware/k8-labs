package api

import (
	"strings"
	"testing"
)

// cleanDriverFilename is display-only, but it takes a client-supplied string, so
// it must never yield a path component. These cases pin the base-name reduction,
// the .ko suffixing, and the module-name fallback.
func TestCleanDriverFilename(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		modName string
		want    string
	}{
		{"plain name gets .ko", "vuln", "kuaf", "vuln.ko"},
		{"already .ko kept", "vuln.ko", "kuaf", "vuln.ko"},
		{"directory components stripped", "/etc/passwd", "kuaf", "passwd.ko"},
		{"path traversal reduced to base", "../../../root/evil.ko", "kuaf", "evil.ko"},
		{"whitespace trimmed", "  driver.ko  ", "kuaf", "driver.ko"},
		{"empty falls back to modName", "", "kuaf", "kuaf.ko"},
		{"dot falls back to modName", ".", "kuaf", "kuaf.ko"},
		{"slash falls back to modName", "/", "kuaf", "kuaf.ko"},
		{"empty with no modName falls back to module.ko", "", "", "module.ko"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanDriverFilename(tc.raw, tc.modName)
			if got != tc.want {
				t.Errorf("cleanDriverFilename(%q, %q) = %q, want %q", tc.raw, tc.modName, got, tc.want)
			}
			// Whatever the input, the result is never a path.
			if strings.ContainsAny(got, "/\\") {
				t.Errorf("result %q still contains a path separator", got)
			}
		})
	}
}
