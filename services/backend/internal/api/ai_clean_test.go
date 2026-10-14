package api

import "testing"

func TestCleanTerminalStripsAnsi(t *testing.T) {
	raw := "\x1b[36mroot@ns-escape\x1b[0m:\x1b[90m/\x1b[0m \x1b[32m#\x1b[0m kubectl auth can-i --list\r\n" +
		"secrets   [get list]\x1b[J\r\n\x1b[6n" +
		"\x1b[36mroot@ns-escape\x1b[0m:/ # \r\n\n\n\n"
	got := cleanTerminal(raw)
	for _, bad := range []string{"\x1b", "[36m", "[0m", "[J", "[6n", "\r"} {
		if containsStr(got, bad) {
			t.Fatalf("cleaned output still contains %q: %q", bad, got)
		}
	}
	for _, want := range []string{"root@ns-escape", "kubectl auth can-i --list", "secrets   [get list]"} {
		if !containsStr(got, want) {
			t.Fatalf("cleaned output missing %q: %q", want, got)
		}
	}
}

func containsStr(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
