package api

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
)

// devLoginEnabled reports whether the localhost-only username/password route
// should be mounted. It is driven solely by DEV_LOGIN=1, which docker-compose
// sets for local dev and which prod (Vault/values.yaml) never sets - so the
// route simply does not exist in a real deployment.
func devLoginEnabled() bool {
	return os.Getenv("DEV_LOGIN") == "1"
}

// devLogin is the localhost-only credential path. It exists so a developer can
// sign in without the Google/GitHub round-trip (handy on a machine with no
// OAuth client IDs, and for poking at the app on other viewports). Any username
// is accepted and mapped to a <username>@dev.local admin account; the shared
// password comes from DEV_LOGIN_PASSWORD (default "labs"). Registered only when
// devLoginEnabled() (see NewRouter) - never in prod.
func devLogin(pool *pgxpool.Pool) http.HandlerFunc {
	want := os.Getenv("DEV_LOGIN_PASSWORD")
	if want == "" {
		want = "labs"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		// Constant-time compare on the password; a plain != would still be fine
		// for a dev secret, but this keeps the one credential check in the code
		// from reading like a timing-oracle example.
		if body.Username == "" || subtle.ConstantTimeCompare([]byte(body.Password), []byte(want)) != 1 {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		user, err := accounts.FindOrCreateLocalUser(r.Context(), pool, body.Username)
		if err != nil {
			slog.Error("dev login failed", "error", err)
			http.Error(w, "login failed", http.StatusInternalServerError)
			return
		}
		finishLogin(w, r, user)
	}
}
