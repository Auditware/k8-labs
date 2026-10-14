package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"backend/docs"
	"backend/internal/accounts"
)

// scopedSwaggerSpec serves the OpenAPI document filtered to what the caller is
// actually allowed to call.
//
// A regular user gets auth/profile/api-key/module/session routes. An admin
// additionally gets every /admin/* route. The point isn't secrecy for its own
// sake - requireAdmin already refuses those endpoints to a non-admin - it's
// that a user's API reference should describe their API, not list a dozen
// operations that will only ever answer 403. It also avoids handing an
// attacker who compromises a low-privilege account a free map of the admin
// surface.
//
// Filtering happens per request against the generated spec rather than
// maintaining a second hand-written document, so a newly added route can't
// drift out of the docs.
func scopedSwaggerSpec() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var spec map[string]any
		if err := json.Unmarshal([]byte(docs.SwaggerInfo.ReadDoc()), &spec); err != nil {
			http.Error(w, "failed to render api docs", http.StatusInternalServerError)
			return
		}

		isAdmin := accounts.IsAdmin(u.Role)
		if paths, ok := spec["paths"].(map[string]any); ok {
			for path := range paths {
				if !isAdmin && isAdminPath(path) {
					delete(paths, path)
				}
			}
		}

		// Say who the document was generated for, so it's obvious an admin's
		// copy and a user's copy legitimately differ rather than one being
		// stale.
		if info, ok := spec["info"].(map[string]any); ok {
			audience := "user"
			if isAdmin {
				audience = "admin"
			}
			if desc, ok := info["description"].(string); ok {
				info["description"] = desc + "\n\nScoped to your access level: " + audience +
					". API keys authenticate as you and carry exactly your permissions."
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spec)
	}
}

// isAdminPath reports whether a spec path is admin-only. Matched against the
// same /admin prefix the routes use; the leading segment may or may not carry
// the /api/v1 base path depending on how swaggo emitted it.
func isAdminPath(path string) bool {
	return strings.HasPrefix(path, "/admin") || strings.Contains(path, "/api/v1/admin")
}
