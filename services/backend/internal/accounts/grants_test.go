package accounts

import "testing"

// The allowlist is the only bootstrap path to admin in a real environment, so
// both directions matter: it must match the intended address despite casing
// and spacing, and it must never match anything else - especially not on an
// empty email or an empty list, which is the state every deployment that
// hasn't configured it is in.
func TestEmailAllowlisted(t *testing.T) {
	const list = "superadmin@example.com, other@example.com"

	cases := []struct {
		name  string
		list  string
		email string
		want  bool
	}{
		{"exact match", list, "superadmin@example.com", true},
		{"case-insensitive", list, "SuperAdmin@Example.com", true},
		{"surrounding whitespace on the email", list, "  superadmin@example.com  ", true},
		{"second entry in the list", list, "other@example.com", true},
		{"single-entry list", "solo@example.com", "solo@example.com", true},

		{"not in the list", list, "someone@example.com", false},
		{"empty list matches nothing", "", "superadmin@example.com", false},
		{"empty email never matches", list, "", false},
		{"empty email against empty list", "", "", false},
		{"whitespace-only list", "   ", "superadmin@example.com", false},
		{"empty email against a list of empty entries", ",,", "", false},
		// A substring must not match - "admin@example.com" is a different account
		// from "superadmin@example.com".
		{"substring is not a match", list, "admin@example.com", false},
		{"superstring is not a match", list, "xsuperadmin@example.com", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := emailAllowlisted(tc.list, tc.email); got != tc.want {
				t.Errorf("emailAllowlisted(%q, %q) = %v, want %v", tc.list, tc.email, got, tc.want)
			}
		})
	}
}
