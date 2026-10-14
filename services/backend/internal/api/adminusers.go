package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
	"backend/internal/labs"
)

// adminListUsers godoc
// @Summary     List users
// @Description Every account with its lab entitlements, for the admin user-management page.
// @Tags        admin
// @Produce     json
// @Success     200 {array} accounts.AdminUser
// @Failure     403 {string} string "forbidden - not an admin"
// @Security    BearerAuth
// @Router      /admin/users [get]
func adminListUsers(pool *pgxpool.Pool, mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := accounts.ListUsers(r.Context(), pool)
		if err != nil {
			http.Error(w, "failed to list users", http.StatusInternalServerError)
			return
		}
		// Overlay in-flight usage from the live session manager. This is the
		// half the database cannot answer: a running session has no lab_usage
		// row until it ends, so without this a student's consumption reads "-"
		// for the entire time they are actually consuming.
		live := mgr.LiveUsageByUser()
		for i := range users {
			if lu, ok := live[users[i].ID]; ok {
				luCopy := lu
				users[i].Live = &luCopy
			}
		}
		// The daily quota rides alongside so the usage column can draw each
		// user's 24h consumption as a fraction of their cap (0 = quota disabled,
		// the frontend then shows the number without a toward-limit bar). It is a
		// single global value, so it sits on the envelope, not on every user.
		json.NewEncoder(w).Encode(adminUsersResponse{
			Users:                users,
			DailyQuotaGiBMinutes: mgr.DailyQuotaGiBMinutes(),
		})
	}
}

type adminUsersResponse struct {
	Users                []accounts.AdminUser `json:"users"`
	DailyQuotaGiBMinutes float64              `json:"dailyQuotaGibMinutes"`
}

// adminGrantLab godoc
// @Summary     Grant a lab to a user
// @Description Gives the user access to one module. Idempotent - re-granting is a no-op.
// @Tags        admin
// @Accept      json
// @Produce     json
// @Param       id path string true "user ID"
// @Param       body body object true "{\"module\": \"suid-privesc-101\"}"
// @Success     200 {object} map[string]string
// @Failure     400 {string} string "unknown module, or missing user"
// @Failure     403 {string} string "forbidden - not an admin"
// @Security    BearerAuth
// @Router      /admin/users/{id}/grants [post]
func adminGrantLab(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID := r.PathValue("id")
		var body struct {
			Module string `json:"module"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// Validate against the real catalog so a typo'd slug can't be written
		// as a grant that silently never matches anything.
		if _, ok := labs.GetModule(body.Module); !ok {
			http.Error(w, "unknown module", http.StatusBadRequest)
			return
		}
		if _, found, err := accounts.GetUser(r.Context(), pool, targetID); err != nil {
			http.Error(w, "failed to look up user", http.StatusInternalServerError)
			return
		} else if !found {
			http.Error(w, "unknown user", http.StatusBadRequest)
			return
		}

		actor := contextUser(r)
		if err := accounts.Grant(r.Context(), pool, targetID, body.Module, actor.ID); err != nil {
			http.Error(w, "failed to grant", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// adminRevokeLab godoc
// @Summary     Revoke a lab from a user
// @Description Removes the user's access to one module and tears down any session they have running for it.
// @Tags        admin
// @Produce     json
// @Param       id path string true "user ID"
// @Param       slug path string true "module slug"
// @Success     200 {object} map[string]string
// @Failure     403 {string} string "forbidden - not an admin"
// @Security    BearerAuth
// @Router      /admin/users/{id}/grants/{slug} [delete]
func adminRevokeLab(pool *pgxpool.Pool, mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID := r.PathValue("id")
		slug := r.PathValue("slug")

		if err := accounts.Revoke(r.Context(), pool, targetID, slug); err != nil {
			http.Error(w, "failed to revoke", http.StatusInternalServerError)
			return
		}
		// A revoke that leaves the user's shell open is cosmetic - they keep
		// full access to the lab pod until it happens to time out.
		killSessions(r.Context(), mgr, targetID, slug)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// adminSetBlocked godoc
// @Summary     Block or unblock a user
// @Description Blocking denies authentication entirely and tears down the user's running sessions. Grants are preserved, so unblocking restores prior access.
// @Tags        admin
// @Accept      json
// @Produce     json
// @Param       id path string true "user ID"
// @Param       body body object true "{\"blocked\": true}"
// @Success     200 {object} map[string]string
// @Failure     400 {string} string "unknown user, or admins cannot block themselves"
// @Failure     403 {string} string "forbidden - not an admin"
// @Security    BearerAuth
// @Router      /admin/users/{id}/blocked [post]
func adminSetBlocked(pool *pgxpool.Pool, mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID := r.PathValue("id")
		var body struct {
			Blocked bool `json:"blocked"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		actor := contextUser(r)
		// Blocking yourself locks you out of the very page you'd need to undo
		// it, and with one admin account that's an unrecoverable cluster.
		if actor != nil && actor.ID == targetID {
			http.Error(w, "cannot block your own account", http.StatusBadRequest)
			return
		}
		if _, found, err := accounts.GetUser(r.Context(), pool, targetID); err != nil {
			http.Error(w, "failed to look up user", http.StatusInternalServerError)
			return
		} else if !found {
			http.Error(w, "unknown user", http.StatusBadRequest)
			return
		}

		if err := accounts.SetBlocked(r.Context(), pool, targetID, body.Blocked); err != nil {
			http.Error(w, "failed to update user", http.StatusInternalServerError)
			return
		}
		if body.Blocked {
			// requireAuth already rejects a blocked user's API calls, but the
			// lab pod keeps running and its terminal websocket stays open
			// until torn down, so a block has to reach the cluster too.
			killSessions(r.Context(), mgr, targetID, "")
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// adminSetUserQuota godoc
// @Summary     Set or clear a user's daily quota override
// @Description Overrides the global LABS_DAILY_QUOTA_GIB_MINUTES for one user. gibMinutes null clears the override (back to the global default); 0 exempts the user; a positive value is their cap.
// @Tags        admin
// @Accept      json
// @Produce     json
// @Param       id path string true "user ID"
// @Param       body body object true "{\"gibMinutes\": 200}"
// @Success     200 {object} map[string]string
// @Failure     400 {string} string "unknown user, or a negative value"
// @Failure     403 {string} string "forbidden - not an admin"
// @Security    BearerAuth
// @Router      /admin/users/{id}/quota [post]
func adminSetUserQuota(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID := r.PathValue("id")
		// Pointer so an explicit null (clear the override) is distinct from a
		// number (set it). A missing field decodes to nil, same as null.
		var body struct {
			GiBMinutes *float64 `json:"gibMinutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// Negative is meaningless (0 already means unlimited); reject rather than
		// storing a value the admission gate would just treat as "off".
		if body.GiBMinutes != nil && *body.GiBMinutes < 0 {
			http.Error(w, "quota cannot be negative", http.StatusBadRequest)
			return
		}
		if _, found, err := accounts.GetUser(r.Context(), pool, targetID); err != nil {
			http.Error(w, "failed to look up user", http.StatusInternalServerError)
			return
		} else if !found {
			http.Error(w, "unknown user", http.StatusBadRequest)
			return
		}
		if err := accounts.SetUserQuota(r.Context(), pool, targetID, body.GiBMinutes); err != nil {
			http.Error(w, "failed to update quota", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// adminDeleteUser godoc
// @Summary     Delete a user
// @Description Permanently removes the account and its lab grants, and tears down its running sessions. Irreversible - block instead to suspend someone reversibly.
// @Tags        admin
// @Produce     json
// @Param       id path string true "user ID"
// @Success     200 {object} map[string]string
// @Failure     400 {string} string "unknown user, or admins cannot delete themselves"
// @Failure     403 {string} string "forbidden - not an admin"
// @Security    BearerAuth
// @Router      /admin/users/{id} [delete]
func adminDeleteUser(pool *pgxpool.Pool, mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID := r.PathValue("id")

		actor := contextUser(r)
		// Same reasoning as blocking yourself, but worse: deleting your own
		// account is unrecoverable, not just awkward to undo.
		if actor != nil && actor.ID == targetID {
			http.Error(w, "cannot delete your own account", http.StatusBadRequest)
			return
		}
		if _, found, err := accounts.GetUser(r.Context(), pool, targetID); err != nil {
			http.Error(w, "failed to look up user", http.StatusInternalServerError)
			return
		} else if !found {
			http.Error(w, "unknown user", http.StatusBadRequest)
			return
		}

		// Tear down before deleting: killSessions matches on OwnerID, and once
		// the row is gone the sessions would be orphaned with no way to
		// attribute them back to anyone.
		killSessions(r.Context(), mgr, targetID, "")
		if err := accounts.DeleteUser(r.Context(), pool, targetID); err != nil {
			http.Error(w, "failed to delete user", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// killSessions tears down the user's running sessions. An empty moduleSlug
// means all of them (blocking); a non-empty one narrows it to that module
// (revoking a single grant).
//
// Best-effort by design: a teardown failure is logged, not surfaced, because
// the entitlement change itself has already been committed and the reaper
// (session.go's StartReaper) will collect anything left behind. Failing the
// admin's request here would imply the block didn't take, which is false.
func killSessions(ctx context.Context, mgr *labs.Manager, userID, moduleSlug string) {
	if mgr == nil {
		return
	}
	for _, s := range mgr.List() {
		if s.OwnerID != userID {
			continue
		}
		if moduleSlug != "" && s.ModuleSlug != moduleSlug {
			continue
		}
		if err := mgr.Delete(ctx, s.ID); err != nil {
			log.Printf("admin: tearing down session %s for user %s: %v", s.ID, userID, err)
		}
	}
}
