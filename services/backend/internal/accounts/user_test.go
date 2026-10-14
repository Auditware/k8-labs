package accounts

import "testing"

// The handle is public: it's shown on the leaderboard and is the path segment
// in /operators/{username}, so it has to stay URL-safe and bounded. This guards
// the shape half of that rule (uniqueness is a DB check, not covered here).
func TestValidateUsername(t *testing.T) {
	cases := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"simple", "nullbyte", false},
		{"with underscore", "capsh_kelly", false},
		{"with hyphen", "r00t-kitten", false},
		{"digits and letters", "r00tkitten", false},
		{"min length", "abc", false},
		{"max length", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // 32

		{"empty", "", true},
		{"too short", "ab", true},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true}, // 33
		{"leading separator", "_alice", true},
		{"trailing separator", "alice-", true},
		{"space inside", "alice smith", true},
		{"slash - would break the URL path", "a/b", true},
		{"dot", "alice.smith", true},
		{"at sign", "alice@host", true},
		{"unicode", "álice", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUsername(tc.username)
			if tc.wantErr && err == nil {
				t.Errorf("validateUsername(%q) = nil, want an error", tc.username)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateUsername(%q) = %v, want nil", tc.username, err)
			}
		})
	}
}
