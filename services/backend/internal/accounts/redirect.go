package accounts

import "strings"

// localDevOrigins are always allowed regardless of CORS_ORIGIN - 7373 is
// Vite's dev server (bun run dev, the primary local dev loop - see
// services/frontend/vite.config.ts for why not Vite's 5173 default), 7374 is
// the backend's own local port (go run / docker-compose, hit directly when
// not going through Vite), and 8080 is Traefik's port on the local k3d
// cluster. 5173 stays too - already
// registered as a real callback URL in the Google OAuth client console (see
// secrets/backend.env), removing it would break that registration rather
// than just stop using it.
var localDevOrigins = []string{
	"http://localhost:7373",
	"http://127.0.0.1:7373",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
	"http://localhost:7374",
	"http://127.0.0.1:7374",
	"http://localhost:8080",
	"http://127.0.0.1:8080",
}

// IsAllowedRedirect guards against an attacker supplying an arbitrary
// redirect_uri to redirect the OAuth code (and thus the resulting session)
// somewhere they control - checked before any code exchange happens. Origin
// match only (no open subdomain bypass), same approach as our other services.
func (c Config) IsAllowedRedirect(redirectURI string) bool {
	origins := localDevOrigins
	if c.AllowedOrigin != "" {
		origins = append(origins, c.AllowedOrigin)
	}
	for _, origin := range origins {
		if redirectURI == origin || strings.HasPrefix(redirectURI, origin+"/") {
			return true
		}
	}
	return false
}
