package api

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
	"backend/internal/course"
)

// getCourse godoc
// @Summary     Get a lab's guided course
// @Description The lesson shown in the lab's left pane. Entitlement-gated exactly like starting the lab: a user who was never granted this module gets 403, not the content. A module with no course authored yet returns an empty step list rather than 404 - a lab without a lesson is incomplete, not broken.
// @Tags        labs
// @Produce     json
// @Param       slug path string true "Module slug"
// @Success     200 {object} course.Course
// @Failure     401 {string} string "unauthorized"
// @Failure     403 {string} string "forbidden - module not granted"
// @Security    BearerAuth
// @Router      /modules/{slug}/course [get]
func getCourse(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		slug := r.PathValue("slug")

		// Same gate as creating a session. Course content names the lab's flaw
		// outright - the last screen of ns-escape is the fix - so serving
		// it to someone who can't run the lab would leak the answer to a lab
		// they might be granted later.
		ok, err := accounts.HasGrant(r.Context(), pool, u.ID, u.Role, slug)
		if err != nil {
			http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		c, found := course.Get(slug)
		if !found {
			// Not an error: the module exists, its lesson just hasn't been
			// written. The UI shows the terminal and an empty course pane.
			c = course.Course{ModuleSlug: slug}
		}
		w.Header().Set("Content-Type", "application/json")
		// RenderCourse turns the authors' Markdown into the HTML the pane
		// displays - see internal/course/prose.go for why that conversion is
		// hand-written and what it refuses to emit.
		json.NewEncoder(w).Encode(course.RenderCourse(c))
	}
}
