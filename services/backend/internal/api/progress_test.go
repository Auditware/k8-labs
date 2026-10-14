package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/accounts"
)

// flagMatches is the whole correctness of the capstone: accept the flag the
// student captured however they pasted it, reject anything else. A false accept
// hands out a solve nobody earned; a false reject blocks a legitimate one.
func TestFlagMatches(t *testing.T) {
	const want = "k8labs{cross_ns_secret_read_via_overbroad_clusterrolebinding}"
	cases := []struct {
		name      string
		submitted string
		ok        bool
	}{
		{"exact", want, true},
		{"trailing newline", want + "\n", true},
		{"surrounding whitespace", "  " + want + "\t\n", true},
		{"empty", "", false},
		{"wrong value", "k8labs{nope}", false},
		{"case differs", "K8LABS{cross_ns_secret_read_via_overbroad_clusterrolebinding}", false},
		{"inner whitespace is not trimmed", "k8labs{cross ns}", false},
		{"prefix only", "k8labs{", false},
		{"extra suffix", want + "x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flagMatches(tc.submitted, want); got != tc.ok {
				t.Errorf("flagMatches(%q) = %v, want %v", tc.submitted, got, tc.ok)
			}
		})
	}
}

// The flag and reset routes must be behind requireAuth in the route table, like
// their progress siblings - an unauthenticated caller must never reach the
// handler (and its pool). This asserts the wiring in NewRouter, the exact place
// the GET-session auth bug once hid.
func TestProgressMutationRoutesRequireAuth(t *testing.T) {
	router := NewRouter(nil, nil, accounts.Config{}, nil, nil, nil)

	cases := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/modules/ns-escape/flag"},
		{http.MethodDelete, "/api/v1/modules/ns-escape/progress"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil) // no cookie, no bearer
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("unauthenticated %s %s: got %d, want 401 - route is missing requireAuth",
					tc.method, tc.path, rec.Code)
			}
		})
	}
}
