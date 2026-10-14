package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
	"backend/internal/drivers"
)

// The research driver catalog user surface. A research-grant user uploads a
// compiled .ko (raw body, like the avatar upload), sees the catalog they may
// pick from, and manages their own uploads. Admin vetting lives in
// admindrivers.go. See docs/architecture/research-driver-catalog.md.

// researchSlugs are the modules whose grant lets a user use the driver catalog.
// Holding a grant for either is enough - the catalog is shared across both.
var researchSlugs = []string{"driver-research-dev", "driver-research-hard"}

// hasResearchAccess reports whether the user may use the driver catalog: an
// admin always may; otherwise they must hold a grant for a research lab.
func hasResearchAccess(r *http.Request, pool *pgxpool.Pool, u *accounts.User) (bool, error) {
	if accounts.IsAdmin(u.Role) {
		return true, nil
	}
	for _, slug := range researchSlugs {
		ok, err := accounts.HasGrant(r.Context(), pool, u.ID, u.Role, slug)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// listDrivers godoc
// @Summary     List research drivers you may use
// @Description The shared vetted catalog plus your own uploads (any status). Research-grant gated.
// @Tags        drivers
// @Produce     json
// @Success     200 {array} drivers.Driver
// @Failure     403 {string} string "forbidden - needs a research lab grant"
// @Security    BearerAuth
// @Router      /drivers [get]
func listDrivers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if ok, err := hasResearchAccess(r, pool, u); err != nil {
			http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
			return
		} else if !ok {
			http.Error(w, "forbidden - research lab access required", http.StatusForbidden)
			return
		}
		list, err := drivers.ListForUser(r.Context(), pool, u.ID)
		if err != nil {
			http.Error(w, "failed to list drivers", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

// uploadDriver godoc
// @Summary     Upload a compiled kernel module (.ko)
// @Description Raw .ko bytes as the request body (like avatar upload). Filename via ?filename=. Parsed and filed as pending for admin vetting. Research-grant gated.
// @Tags        drivers
// @Accept      octet-stream
// @Produce     json
// @Success     200 {object} drivers.Driver
// @Failure     400 {string} string "not a valid .ko"
// @Failure     403 {string} string "forbidden"
// @Failure     409 {string} string "already uploaded"
// @Failure     413 {string} string "module too large"
// @Failure     429 {string} string "per-user upload quota or rate limit exceeded"
// @Security    BearerAuth
// @Router      /drivers [post]
func uploadDriver(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if ok, err := hasResearchAccess(r, pool, u); err != nil {
			http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
			return
		} else if !ok {
			http.Error(w, "forbidden - research lab access required", http.StatusForbidden)
			return
		}

		// An API key may upload only if it explicitly carries the drivers:write
		// scope - a general key acting as the same user cannot, so agent keys are
		// least-privilege by default. Browser (cookie/JWT) sessions are not
		// scope-limited and fall through on the research-access check above.
		if scopes, viaKey := apiKeyScopes(r); viaKey && !accounts.KeyHasScope(scopes, accounts.ScopeDriversWrite) {
			http.Error(w, "forbidden - this API key lacks the drivers:write scope", http.StatusForbidden)
			return
		}

		// Per-user upload limits keep one research user from filling the bytea
		// store (pending quota) or flooding it (trailing-window rate). Checked
		// before the body is read, so a throttled request streams nothing. Admins
		// are trusted operators (they seed the shared catalog) and are exempt.
		if !accounts.IsAdmin(u.Role) {
			if err := drivers.EnforceUploadLimits(r.Context(), pool, u.ID); err != nil {
				var rl *drivers.RateLimitedError
				switch {
				case errors.As(err, &rl):
					w.Header().Set("Retry-After", strconv.Itoa(int(rl.RetryAfter.Seconds())+1))
					http.Error(w, "too many driver uploads recently; try again later", http.StatusTooManyRequests)
				case errors.Is(err, drivers.ErrQuotaExceeded):
					http.Error(w, fmt.Sprintf("you have %d driver uploads awaiting review (the per-user limit); wait for an admin to vet them before uploading more", drivers.MaxPendingPerUser), http.StatusTooManyRequests)
				default:
					http.Error(w, "failed to check upload limits", http.StatusInternalServerError)
				}
				return
			}
		}

		// Cap what we read into memory, +1 so MaxBytesReader's error distinguishes
		// "at the limit" from "over it" (same idiom as uploadAvatar).
		r.Body = http.MaxBytesReader(w, r.Body, drivers.MaxKoBytes+1)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "module too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Parse and validate the ELF before touching the DB. Never trust the
		// client's filename for anything but display; sanitize it to a base name.
		parsed, err := drivers.ParseKo(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		filename := cleanDriverFilename(r.URL.Query().Get("filename"), parsed.ModName)

		d, err := drivers.Create(r.Context(), pool, u.ID, filename, len(data), data, parsed)
		if errors.Is(err, drivers.ErrDuplicate) {
			// Re-upload of identical content. Tell the caller its current status
			// (notably an already-denied module) rather than making a second copy.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(d)
			return
		}
		if err != nil {
			http.Error(w, "failed to store module", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(d)
	}
}

// cleanDriverFilename reduces a client-supplied name to a safe base filename
// (no directory components, .ko-suffixed), falling back to the module's own
// name. Display only - it is never used as a path.
func cleanDriverFilename(raw, modName string) string {
	base := path.Base(strings.TrimSpace(raw))
	if base == "" || base == "." || base == "/" {
		if modName != "" {
			return modName + ".ko"
		}
		return "module.ko"
	}
	if !strings.HasSuffix(base, ".ko") {
		base += ".ko"
	}
	return base
}
