// Package accounts owns user identity: Google/GitHub "Sign in with..."
// (authorization code flow - exchange the code the frontend received for an
// access token, fetch the provider's userinfo endpoint, then find-or-create
// a local user row). OAuth is the only way in: username/password login and
// the seeded admin account were removed so local dev exercises the same auth
// path as prod. Admin rights come from the ADMIN_EMAILS allowlist (grants.go).
// OAuth logic/flow adapted from our earlier Rust implementation's
// auth service and auth API handlers - the reference implementation
// this was ported from Rust to Go.
package accounts

import "os"

type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	// AllowedOrigin is compared against the frontend-supplied redirect_uri
	// before any code exchange happens - see redirect.go. Empty disables
	// the app's own origin from the allowlist (still permits the hardcoded
	// local dev ports below), matching our CORS_ORIGIN convention.
	AllowedOrigin string
	// AdminEmails is the OAuth admin allowlist: any account whose email
	// matches is promoted to role='admin' on every sign-in (see
	// PromoteIfAllowlisted). Comma-separated, matched case-insensitively.
	//
	// This is the bootstrap path for real environments. Registration is open
	// but every new account is role='user' with zero lab grants, so without
	// this the first person to sign up has no admin to grant them anything and
	// nobody can ever administer the system. Set on the deployment (prod uses
	// Vault - see apps/backend/values.yaml's secretTemplate), never defaulted
	// in code: a hardcoded admin email would be a backdoor in anyone else's
	// deployment of this chart.
	AdminEmails string
}

func ConfigFromEnv() Config {
	return Config{
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		AllowedOrigin:      os.Getenv("CORS_ORIGIN"),
		AdminEmails:        os.Getenv("ADMIN_EMAILS"),
	}
}
