package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
)

// activityWindowDays is how far back the profile/showcase activity calendar
// reaches. ~20 weeks so the contribution grid reads like GitHub's without being
// so long it dwarfs a young account.
const activityWindowDays = 140

// showcaseCount is how many top operators the landing deck renders.
const showcaseCount = 3

// operatorShowcase godoc
// @Summary     Top operators for the landing showcase
// @Description Public. The top few ranked operators with their real stats and activity calendar, for the landing "track record" deck.
// @Tags        labs
// @Produce     json
// @Success     200 {array} accounts.PublicProfile
// @Router      /operators [get]
//
// Collection route (not a username) so it can never collide with an operator
// literally named "showcase" or "top".
func operatorShowcase(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profiles, err := accounts.TopProfiles(r.Context(), pool, showcaseCount, activityWindowDays)
		if err != nil {
			http.Error(w, "failed to load operators", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profiles)
	}
}

// operatorProfile godoc
// @Summary     A public operator profile
// @Description Public. One operator's handle, self-published bio/website, leaderboard standing, and daily activity calendar. Never an email or session detail.
// @Tags        labs
// @Produce     json
// @Param       username path string true "operator handle"
// @Success     200 {object} accounts.PublicProfile
// @Failure     404 {string} string "not found"
// @Router      /operators/{username} [get]
func operatorProfile(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := accounts.GetPublicProfile(r.Context(), pool, r.PathValue("username"), activityWindowDays)
		if err != nil {
			http.Error(w, "failed to load profile", http.StatusInternalServerError)
			return
		}
		if p == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

// serveOperatorAvatar returns an operator's avatar by handle, publicly. Unlike
// the admin-facing /users/{id}/avatar this needs no session - a public profile
// page (signed-out visitors included) has to render the picture. It exposes only
// an image the operator chose to publish, with the same hardening as the
// authenticated avatar route: a fixed Content-Type from our allowlist and
// nosniff, so a stored file can't be coaxed into executing.
//
// Deliberately unannotated for swaggo - it returns image bytes, not JSON.
func serveOperatorAvatar(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, contentType, err := accounts.GetAvatarByUsername(r.Context(), pool, r.PathValue("username"))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "failed to load avatar", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		// Public, so a shared cache may hold it; short TTL so a changed avatar
		// rolls out quickly.
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	}
}
