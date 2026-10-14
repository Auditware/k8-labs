package api

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
	"backend/internal/labs"
)

// Admin -> Lab Management. Per-lab operator knobs, currently the session
// duration and idle timeout. Admin-only, like every other /admin route.
//
// These are not cosmetic: duration_minutes is what the lab UI counts down to
// and what the reaper enforces, so a change here changes how long a student's
// work survives. Already-running sessions keep the timing they started with
// (see labs.Session.Timing) - shortening a lab must not yank a session out from
// under someone mid-exercise.

// adminListLabSettings godoc
// @Summary     List per-lab settings
// @Description Effective settings for every module - modules with no override report isDefault=true with the built-in values, so the UI can render one row per lab without knowing which were customised.
// @Tags        admin
// @Produce     json
// @Success     200 {array} accounts.LabSetting
// @Failure     403 {string} string "forbidden - admin only"
// @Security    BearerAuth
// @Router      /admin/labs/settings [get]
func adminListLabSettings(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mods := labs.ListModules()
		slugs := make([]string, 0, len(mods))
		for _, m := range mods {
			slugs = append(slugs, m.Slug)
		}
		settings, err := accounts.ListLabSettings(r.Context(), pool, slugs)
		if err != nil {
			http.Error(w, "failed to list lab settings", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	}
}

type labSettingBody struct {
	DurationMinutes int `json:"durationMinutes"`
	IdleMinutes     int `json:"idleMinutes"`
}

// adminSetLabSetting godoc
// @Summary     Set a lab's session limits
// @Description Upserts the duration and idle timeout for one module. Rejected (400) rather than silently clamped when out of range, or when the idle timeout exceeds the duration - an idle timeout past the hard cap could never fire, so accepting it would show an admin a setting that does nothing.
// @Tags        admin
// @Accept      json
// @Produce     json
// @Param       slug path string true "Module slug"
// @Param       body body labSettingBody true "Limits in minutes"
// @Success     204 {string} string "updated"
// @Failure     400 {string} string "invalid limits"
// @Failure     403 {string} string "forbidden - admin only"
// @Failure     404 {string} string "unknown module"
// @Security    BearerAuth
// @Router      /admin/labs/settings/{slug} [put]
func adminSetLabSetting(pool *pgxpool.Pool, store *accounts.TimingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		// Only slugs that actually exist - otherwise a typo writes a row that
		// silently governs nothing and looks configured in the table.
		if _, ok := labs.GetModule(slug); !ok {
			http.Error(w, "unknown module", http.StatusNotFound)
			return
		}
		var body labSettingBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		adminID, _ := r.Context().Value(userIDKey).(string)
		if err := accounts.SetLabSetting(r.Context(), pool, store, slug, body.DurationMinutes, body.IdleMinutes, adminID); err != nil {
			// The validation errors here are the admin's to see and act on -
			// "between 5 and 480 minutes" is actionable, "bad request" isn't.
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// adminResetLabSetting godoc
// @Summary     Reset a lab to the default limits
// @Description Drops the module's override so it falls back to the built-in default (60m duration, 20m idle). Resetting a lab that was never customised is not an error.
// @Tags        admin
// @Param       slug path string true "Module slug"
// @Success     204 {string} string "reset"
// @Failure     403 {string} string "forbidden - admin only"
// @Security    BearerAuth
// @Router      /admin/labs/settings/{slug} [delete]
func adminResetLabSetting(pool *pgxpool.Pool, store *accounts.TimingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := accounts.ResetLabSetting(r.Context(), pool, store, r.PathValue("slug")); err != nil {
			http.Error(w, "failed to reset lab setting", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
