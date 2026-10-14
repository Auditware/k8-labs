package accounts

import (
	"slices"
	"testing"
)

// KeyHasScope gates sensitive capabilities, so its semantics are load-bearing:
// an empty scope set (a general key) must NOT satisfy any scope, and a scope is
// matched only by exact name.
func TestKeyHasScope(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		want   string
		ok     bool
	}{
		{"empty set grants nothing", nil, ScopeDriversWrite, false},
		{"empty slice grants nothing", []string{}, ScopeDriversWrite, false},
		{"exact scope present", []string{ScopeDriversWrite}, ScopeDriversWrite, true},
		{"other scope only", []string{"something:else"}, ScopeDriversWrite, false},
		{"among several", []string{"a:b", ScopeDriversWrite, "c:d"}, ScopeDriversWrite, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := KeyHasScope(tc.scopes, tc.want); got != tc.ok {
				t.Errorf("KeyHasScope(%v, %q) = %v, want %v", tc.scopes, tc.want, got, tc.ok)
			}
		})
	}
}

func TestValidAPIKeyScope(t *testing.T) {
	if !ValidAPIKeyScope(ScopeDriversWrite) {
		t.Errorf("%q should be a valid scope", ScopeDriversWrite)
	}
	for _, bad := range []string{"", "drivers", "admin:*", "drivers:read"} {
		if ValidAPIKeyScope(bad) {
			t.Errorf("%q should not be a valid scope", bad)
		}
	}
}

func TestNormalizeScopes(t *testing.T) {
	// Blanks dropped, duplicates collapsed, order preserved, never nil.
	got := normalizeScopes([]string{"", ScopeDriversWrite, " ", ScopeDriversWrite})
	if !slices.Equal(got, []string{ScopeDriversWrite}) {
		t.Errorf("normalizeScopes = %v, want [%q]", got, ScopeDriversWrite)
	}
	if empty := normalizeScopes(nil); empty == nil {
		t.Error("normalizeScopes(nil) must return a non-nil empty slice")
	}
}
