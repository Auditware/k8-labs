package api

import "testing"

// A non-admin's copy of the spec must not describe the admin surface. This is
// the classifier that decides what gets stripped, so it has to be right in both
// directions: missing an admin path leaks the admin API into a user's docs,
// while over-matching would silently hide routes users need.
func TestIsAdminPath(t *testing.T) {
	adminPaths := []string{
		"/admin/health",
		"/admin/topology",
		"/admin/users",
		"/admin/users/{id}",
		"/admin/users/{id}/grants",
		"/admin/users/{id}/grants/{slug}",
		"/admin/users/{id}/blocked",
		"/api/v1/admin/users",
	}
	for _, p := range adminPaths {
		if !isAdminPath(p) {
			t.Errorf("isAdminPath(%q) = false, want true - admin route would leak into a user's docs", p)
		}
	}

	userPaths := []string{
		"/auth/me",
		"/auth/google/callback",
		"/profile",
		"/profile/avatar",
		"/apikeys",
		"/apikeys/{id}",
		"/modules",
		"/sessions",
		"/sessions/{id}",
		"/sessions/{id}/events",
	}
	for _, p := range userPaths {
		if isAdminPath(p) {
			t.Errorf("isAdminPath(%q) = true, want false - user route would be hidden from its own docs", p)
		}
	}
}
