package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
	"backend/internal/course"
	"backend/internal/labs"
)

// Course progress: which quiz answers a student has given for a module, and
// whether they've completed it.
//
// Always scoped to the caller - the user id comes from the session, never from
// the path - so there is no way to read or write anyone else's progress.
//
// Completion is the CTF flag, not the quiz. A lab is solved when the student
// submits the flag they captured in the live lab (putFlag below), which is the
// only thing that proves they actually performed the exploit. The quizzes remain
// as formative self-checks - answered right or wrong, they teach but they do not
// gate completion, so answering them never marks a lab solved and mis-answering
// never blocks it. The expected flag lives server-side in labs.Module.Flag and
// is never served to the client (see progress.go's flag handler and step.go's
// KindFlag), so the answer can't be read out of the course.

type progressResponse struct {
	ModuleSlug string         `json:"moduleSlug"`
	Answers    map[string]int `json:"answers"`
	Complete   bool           `json:"complete"`
	Answered   int            `json:"answered"`
	QuizCount  int            `json:"quizCount"`
}

// getProgress godoc
// @Summary     Get the caller's progress in a lab's course
// @Description Scoped to the caller - progress is keyed by (user, module), never by session, so it survives a session being reaped and the lab being restarted another day. `complete` reflects the captured flag, not the quizzes.
// @Tags        labs
// @Produce     json
// @Param       slug path string true "Module slug"
// @Success     200 {object} progressResponse
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /modules/{slug}/progress [get]
func getProgress(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		slug := r.PathValue("slug")

		answers := map[string]int{}
		var complete bool
		err := pool.QueryRow(r.Context(),
			`SELECT answers, completed_at IS NOT NULL FROM lab_progress WHERE user_id = $1 AND module_slug = $2`,
			u.ID, slug).Scan(&answers, &complete)
		if err != nil && err != pgx.ErrNoRows {
			http.Error(w, "failed to read progress", http.StatusInternalServerError)
			return
		}
		writeProgress(w, slug, answers, complete)
	}
}

type answerBody struct {
	StepSlug string `json:"stepSlug"`
	Option   int    `json:"option"`
}

// putAnswer godoc
// @Summary     Record an answer to one quiz step
// @Description Upserts a single answer into the caller's progress. Quizzes are formative: this records the answer (right or wrong) but never marks the lab complete - only submitting the captured flag does that. Rejects a step slug that isn't a quiz in this course.
// @Tags        labs
// @Accept      json
// @Produce     json
// @Param       slug path string true "Module slug"
// @Param       body body answerBody true "The step and chosen option"
// @Success     200 {object} progressResponse
// @Failure     400 {string} string "unknown step, or not a quiz step"
// @Failure     401 {string} string "unauthorized"
// @Failure     403 {string} string "forbidden - module not granted"
// @Security    BearerAuth
// @Router      /modules/{slug}/progress [put]
func putAnswer(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		slug := r.PathValue("slug")

		// Same entitlement gate as reading the course: no recording progress
		// against a lab you were never granted.
		granted, err := accounts.HasGrant(r.Context(), pool, u.ID, u.Role, slug)
		if err != nil {
			http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
			return
		}
		if !granted {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		var body answerBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		// The answer must correspond to a real quiz step with a real option.
		// Without this a client could POST arbitrary slugs, which - even though
		// quizzes no longer gate completion - would litter progress with answers
		// to steps that don't exist and skew the answered/quizCount the UI shows.
		c, ok := course.Get(slug)
		if !ok {
			http.Error(w, "no course for this module", http.StatusBadRequest)
			return
		}
		valid := false
		for _, s := range c.Steps {
			if s.Slug == body.StepSlug && s.Kind == course.KindQuiz {
				valid = body.Option >= 0 && body.Option < len(s.Opts)
				break
			}
		}
		if !valid {
			http.Error(w, "unknown quiz step or option", http.StatusBadRequest)
			return
		}

		// jsonb_set with the create_missing default builds the object
		// incrementally, so two answers recorded from two tabs don't clobber
		// each other the way a whole-map overwrite would.
		answers := map[string]int{}
		var complete bool
		err = pool.QueryRow(r.Context(), `
			INSERT INTO lab_progress (user_id, module_slug, answers, updated_at)
			VALUES ($1, $2, jsonb_build_object($3::text, $4::int), now())
			ON CONFLICT (user_id, module_slug) DO UPDATE
			SET answers = jsonb_set(lab_progress.answers, ARRAY[$3::text], to_jsonb($4::int), true),
			    updated_at = now()
			RETURNING answers, completed_at IS NOT NULL`,
			u.ID, slug, body.StepSlug, body.Option).Scan(&answers, &complete)
		if err != nil {
			http.Error(w, "failed to save progress", http.StatusInternalServerError)
			return
		}
		writeProgress(w, slug, answers, complete)
	}
}

type flagBody struct {
	Flag string `json:"flag"`
}

type flagResponse struct {
	Correct  bool `json:"correct"`
	Complete bool `json:"complete"`
}

// putFlag godoc
// @Summary     Submit the CTF flag captured in a lab
// @Description Checks the submitted flag against the module's expected flag (held server-side, never served) and, on a match, marks the lab complete - the moment and solve time are frozen once and never re-timed. A wrong flag is a normal 200 with correct=false, not an error.
// @Tags        labs
// @Accept      json
// @Produce     json
// @Param       slug path string true "Module slug"
// @Param       body body flagBody true "The captured flag"
// @Success     200 {object} flagResponse
// @Failure     400 {string} string "invalid body, or this lab has no flag"
// @Failure     401 {string} string "unauthorized"
// @Failure     403 {string} string "forbidden - module not granted"
// @Security    BearerAuth
// @Router      /modules/{slug}/flag [post]
func putFlag(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		slug := r.PathValue("slug")

		granted, err := accounts.HasGrant(r.Context(), pool, u.ID, u.Role, slug)
		if err != nil {
			http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
			return
		}
		if !granted {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		module, ok := labs.GetModule(slug)
		if !ok || module.Flag == "" {
			http.Error(w, "this lab has no flag to submit", http.StatusBadRequest)
			return
		}

		var body flagBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		correct := flagMatches(body.Flag, module.Flag)

		if !correct {
			// A wrong guess is ordinary, not an error. Report the current
			// completion so a UI that already showed "solved" doesn't flicker.
			var complete bool
			err := pool.QueryRow(r.Context(),
				`SELECT completed_at IS NOT NULL FROM lab_progress WHERE user_id = $1 AND module_slug = $2`,
				u.ID, slug).Scan(&complete)
			if err != nil && err != pgx.ErrNoRows {
				http.Error(w, "failed to read progress", http.StatusInternalServerError)
				return
			}
			writeJSON(w, flagResponse{Correct: false, Complete: complete})
			return
		}

		// Correct. Make sure a row exists (the student may have submitted the
		// flag without ever answering a quiz), then stamp completion. Both stamps
		// are COALESCEd so re-submitting an already-solved lab never moves the
		// completion moment or re-times the solve. started_at defaults to now()
		// on a fresh row, so a flag-first solve records a floor of 1 second;
		// a student who worked the quizzes first gets the real span.
		if _, err := pool.Exec(r.Context(),
			`INSERT INTO lab_progress (user_id, module_slug, answers)
			 VALUES ($1, $2, '{}'::jsonb)
			 ON CONFLICT (user_id, module_slug) DO NOTHING`, u.ID, slug); err != nil {
			http.Error(w, "failed to save progress", http.StatusInternalServerError)
			return
		}
		if _, err := pool.Exec(r.Context(),
			`UPDATE lab_progress
			 SET completed_at = COALESCE(completed_at, now()),
			     solve_seconds = COALESCE(solve_seconds, GREATEST(1, EXTRACT(EPOCH FROM (now() - started_at))::int)),
			     updated_at = now()
			 WHERE user_id = $1 AND module_slug = $2`, u.ID, slug); err != nil {
			http.Error(w, "failed to save progress", http.StatusInternalServerError)
			return
		}
		writeJSON(w, flagResponse{Correct: true, Complete: true})
	}
}

// deleteProgress godoc
// @Summary     Clear the caller's progress in a lab (restart)
// @Description Deletes the caller's whole progress row for this module - quiz answers, completion and solve time. This is the "restart the lab / retake the questions" control: after it, every quiz is unanswered again and the lab is unsolved until the flag is resubmitted. Idempotent - clearing a lab with no progress is not an error.
// @Tags        labs
// @Param       slug path string true "Module slug"
// @Success     204 {string} string "cleared"
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /modules/{slug}/progress [delete]
func deleteProgress(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		slug := r.PathValue("slug")

		if _, err := pool.Exec(r.Context(),
			// Retake clears the quiz answers and the completion so the student can
			// walk the lab again - but deliberately KEEPS solve_seconds. The
			// time-to-solve is a write-once record: without this, a student could
			// solve slowly, learn the route, reset, and re-clear in seconds to fake a
			// top time. putFlag COALESCEs solve_seconds, so a re-solve keeps the
			// original; the row is preserved (not deleted) so that frozen time
			// survives. started_at is left as-is (harmless - solve_seconds is locked).
			`UPDATE lab_progress
			 SET answers = '{}'::jsonb, completed_at = NULL, updated_at = now()
			 WHERE user_id = $1 AND module_slug = $2`, u.ID, slug); err != nil {
			http.Error(w, "failed to clear progress", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// flagMatches reports whether a submitted flag is the expected one. It is
// forgiving about how the student pastes it - surrounding whitespace and the
// trailing newline the terminal often carries along are trimmed - but the value
// itself, including everything inside the braces, must match exactly and
// case-sensitively. The compare is constant-time: not because the flag is a
// high-value secret (it is baked into the shipped manifests), but it costs
// nothing and keeps this from being a timing oracle by habit.
func flagMatches(submitted, expected string) bool {
	got := strings.TrimSpace(submitted)
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func writeProgress(w http.ResponseWriter, slug string, answers map[string]int, complete bool) {
	if answers == nil {
		answers = map[string]int{}
	}
	resp := progressResponse{ModuleSlug: slug, Answers: answers, Complete: complete}
	if c, ok := course.Get(slug); ok {
		resp.QuizCount = c.QuizCount()
		resp.Answered = answeredCount(c, answers)
	}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// answeredCount counts answers that correspond to quiz steps still in the
// course - an answer left over from a since-deleted step must not count toward
// the answered/quizCount the UI shows.
func answeredCount(c course.Course, answers map[string]int) int {
	n := 0
	for _, s := range c.Steps {
		if s.Kind != course.KindQuiz {
			continue
		}
		if _, ok := answers[s.Slug]; ok {
			n++
		}
	}
	return n
}
