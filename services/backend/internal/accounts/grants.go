package accounts

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/labs"
)

// ErrUserBlocked is returned by the login paths when the account exists and
// the credentials are valid but the user has been blocked by an admin. Kept
// distinct from a bad-credentials error so the handler can answer 403 rather
// than 401 - a blocked user retrying their password forever is a support
// ticket, not a security signal.
var ErrUserBlocked = errors.New("user is blocked")

// AdminUser is the admin-facing view of an account: the user plus the module
// slugs they've been granted. Used only by the admin user-management page.
type AdminUser struct {
	User
	Grants []string `json:"grants"`
	// Completed is the module slugs this user has solved - captured and submitted
	// the flag for (lab_progress.completed_at IS NOT NULL, the same signal as
	// getProgress.complete). Non-nil like Grants, so it encodes as [] not null.
	// Lets the admin grant menu mark which labs a user has actually cleared,
	// distinct from which they can access. Independent of Grants: an admin (or a
	// grandfathered account) can have a completion without a current grant row.
	Completed []string `json:"completed"`
	// Entitled is what the user can actually ACCESS: explicit Grants folded in
	// with the free progression unlocks (tier-0 ns-escape for everyone, plus each
	// tier the previous one unlocked). Distinct from Grants - a user with no grant
	// rows still has the free first lab - so the admin page counts Entitled, not
	// Grants, for "lab access"; otherwise everyone on the free tier reads as "No
	// labs". Non-nil like Grants, so it encodes as [] not null.
	Entitled []string `json:"entitled"`
	// Usage is this account's HISTORICAL consumption from finished sessions, or
	// nil for a user who has never completed one. Components stay separate all the
	// way to the UI (see UserUsage) so the page can present them however it likes
	// and any "credits" figure remains a formula over them, not a stored number.
	Usage *UserUsage `json:"usage,omitempty"`
	// Live is the user's in-flight consumption from sessions running right now,
	// nil when they have none. Populated by the handler from the labs.Manager,
	// not from the database - a running session has no lab_usage row yet. Kept
	// separate from Usage so the UI can show "using N now" distinctly from
	// lifetime totals, and so neither has to be recomputed to render the other.
	Live *labs.LiveUsage `json:"live,omitempty"`
	// QuotaOverrideGiBMinutes is this user's per-user daily quota override, nil
	// when they fall back to the global default. Admin-editable. Semantics: nil =
	// global default; 0 = unlimited (exempt); a positive value = this user's cap.
	QuotaOverrideGiBMinutes *float64 `json:"quotaOverrideGibMinutes,omitempty"`
	// LastSolvedAt is when this user most recently captured a flag
	// (MAX(lab_progress.completed_at)), nil for a user who has never solved one.
	// It is the primary sort key for the admin list - the operators an admin is
	// most likely acting on (recently active) surface first - and the frontend
	// can show it as a "last solved" hint.
	LastSolvedAt *time.Time `json:"lastSolvedAt,omitempty"`
}

// IsAdmin reports whether the role string denotes an admin. Single place to
// change if roles ever grow beyond the current two.
func IsAdmin(role string) bool { return role == "admin" }

// HasGrant reports whether the user may access the given module. Admins bypass
// everything. Otherwise a user is entitled if the module is unlocked by
// progression (tier 0 is open to all; later tiers unlock as the previous tier is
// solved - see labs.ProgressionTiers and progressionEntitled) OR an admin has
// explicitly granted it (the "rest", handed out after the Telegram request).
//
// Named HasGrant for history; it is really an entitlement check now. It is the
// single gate behind starting a session, reading a course, and recording quiz
// answers / flags, so progression and grants must be checked in exactly one
// place - here - and never re-derived per call site.
func HasGrant(ctx context.Context, pool *pgxpool.Pool, userID, role, moduleSlug string) (bool, error) {
	if IsAdmin(role) {
		return true, nil
	}
	// Progression covers the free tier-0 lab and anything unlocked by solving,
	// with a single query for the caller's solved set.
	solved, err := SolvedModules(ctx, pool, userID)
	if err != nil {
		return false, err
	}
	if progressionEntitled(solved)[moduleSlug] {
		return true, nil
	}
	// Explicit grant (admin-granted "rest").
	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM lab_grants WHERE user_id = $1 AND module_slug = $2)`,
		userID, moduleSlug,
	).Scan(&exists)
	return exists, err
}

// SolvedModules returns the set of module slugs the user has solved (captured
// the flag for - completed_at IS NOT NULL, the same signal as getProgress
// .complete). Drives progression unlocks.
func SolvedModules(ctx context.Context, pool *pgxpool.Pool, userID string) (map[string]bool, error) {
	rows, err := pool.Query(ctx,
		`SELECT module_slug FROM lab_progress WHERE user_id = $1 AND completed_at IS NOT NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	solved := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		solved[s] = true
	}
	return solved, rows.Err()
}

// progressionEntitled returns the module slugs a user is entitled to by
// progression alone, given the labs they have solved. Tier 0 is always included;
// tier i (>0) is included only once every lab in tier i-1 has been solved. This
// is layered on top of explicit grants by the callers, never a replacement.
//
// Pure (no I/O) so the ladder logic is unit-testable without a database, and the
// break is safe: tiers are ordered and each depends on the previous being fully
// solved, so once one tier is locked no later tier can be open either.
func progressionEntitled(solved map[string]bool) map[string]bool {
	out := map[string]bool{}
	tiers := labs.ProgressionTiers()
	for i, tier := range tiers {
		if i > 0 {
			prevSolved := true
			for _, s := range tiers[i-1] {
				if !solved[s] {
					prevSolved = false
					break
				}
			}
			if !prevSolved {
				break
			}
		}
		for _, s := range tier {
			out[s] = true
		}
	}
	return out
}

// EntitledModules returns the module slugs a non-admin user may see and run:
// their explicit grants unioned with everything unlocked by progression. This is
// what the catalog (listModules) is built from; admins are handled by the caller
// (they get the whole catalog). Non-nil and sorted for a stable catalog.
func EntitledModules(ctx context.Context, pool *pgxpool.Pool, userID string) ([]string, error) {
	grants, err := GrantedModules(ctx, pool, userID)
	if err != nil {
		return nil, err
	}
	solved, err := SolvedModules(ctx, pool, userID)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, s := range grants {
		set[s] = true
	}
	for s := range progressionEntitled(solved) {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

// GrantedModules returns the module slugs granted to a user. Only meaningful
// for non-admins; admins are handled by HasGrant/the caller, since the grants
// table says nothing about them.
func GrantedModules(ctx context.Context, pool *pgxpool.Pool, userID string) ([]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT module_slug FROM lab_grants WHERE user_id = $1 ORDER BY module_slug`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Non-nil so a user with no grants encodes as [] rather than null - the
	// frontend maps over this directly.
	slugs := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		slugs = append(slugs, s)
	}
	return slugs, rows.Err()
}

// Grant gives a user access to one module. Idempotent: re-granting an existing
// entitlement is a no-op rather than an error, so the admin UI can send the
// desired state without first reading the current one.
func Grant(ctx context.Context, pool *pgxpool.Pool, userID, moduleSlug, grantedBy string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO lab_grants (user_id, module_slug, granted_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, module_slug) DO NOTHING`,
		userID, moduleSlug, grantedBy)
	return err
}

// Revoke removes a user's access to one module. Also idempotent - revoking
// something never granted is not an error.
//
// Note this does not tear down a session the user already has running for that
// module; the caller does that (see the admin revoke handler), because session
// teardown needs the labs.Manager and this package deliberately doesn't.
func Revoke(ctx context.Context, pool *pgxpool.Pool, userID, moduleSlug string) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM lab_grants WHERE user_id = $1 AND module_slug = $2`,
		userID, moduleSlug)
	return err
}

// SetBlocked blocks or unblocks a user. Grants are left untouched so that
// unblocking restores exactly the access the user had before.
func SetBlocked(ctx context.Context, pool *pgxpool.Pool, userID string, blocked bool) error {
	_, err := pool.Exec(ctx, `UPDATE users SET blocked = $1 WHERE id = $2`, blocked, userID)
	return err
}

// SetUserQuota sets (or clears) a user's per-user daily quota override, in
// GiB-minutes. Pass nil to clear it and fall back to the global default; a
// non-nil value overrides for this user (0 = unlimited, positive = the cap).
func SetUserQuota(ctx context.Context, pool *pgxpool.Pool, userID string, gibMinutes *float64) error {
	_, err := pool.Exec(ctx, `UPDATE users SET daily_quota_gib_minutes = $1 WHERE id = $2`, gibMinutes, userID)
	return err
}

// UsernamesByIDs resolves a set of user IDs to usernames in one query, for the
// admin capacity rollup (which knows sessions by owner ID and wants to label
// them). Missing IDs simply don't appear in the map; the caller falls back to a
// short ID so a just-deleted owner still renders as something.
func UsernamesByIDs(ctx context.Context, pool *pgxpool.Pool, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := pool.Query(ctx, `SELECT id, username FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, username string
		if err := rows.Scan(&id, &username); err != nil {
			return nil, err
		}
		out[id] = username
	}
	return out, rows.Err()
}

// ListUsers returns every account with its grants, for the admin user
// management page. One query for users plus one for all grants, joined in
// memory - the alternative (a per-user grants query) is N+1, and at MVP scale
// (50 users) pulling the whole grants table is trivially small.
func ListUsers(ctx context.Context, pool *pgxpool.Pool) ([]AdminUser, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, email, username, oauth_provider, role, blocked, daily_quota_gib_minutes, created_at
		 FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// created_at is kept in a side map (not on AdminUser) purely to break ties in
	// the sort below - "more recently registered first" is the secondary order.
	createdAt := map[string]time.Time{}
	users := []AdminUser{}
	for rows.Next() {
		var u AdminUser
		var created time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.OAuthProvider, &u.Role, &u.Blocked, &u.QuotaOverrideGiBMinutes, &created); err != nil {
			return nil, err
		}
		u.Grants = []string{}
		u.Completed = []string{}
		createdAt[u.ID] = created
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Last solve per user (MAX completed_at). Drives the primary sort order and
	// rides along on the response as LastSolvedAt. Same bulk-query-then-join shape
	// as grants/completions above.
	solvedRows, err := pool.Query(ctx,
		`SELECT user_id, MAX(completed_at) FROM lab_progress WHERE completed_at IS NOT NULL GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer solvedRows.Close()

	lastSolved := map[string]time.Time{}
	for solvedRows.Next() {
		var userID string
		var at time.Time
		if err := solvedRows.Scan(&userID, &at); err != nil {
			return nil, err
		}
		lastSolved[userID] = at
	}
	if err := solvedRows.Err(); err != nil {
		return nil, err
	}

	grantRows, err := pool.Query(ctx, `SELECT user_id, module_slug FROM lab_grants`)
	if err != nil {
		return nil, err
	}
	defer grantRows.Close()

	byUser := map[string][]string{}
	for grantRows.Next() {
		var userID, slug string
		if err := grantRows.Scan(&userID, &slug); err != nil {
			return nil, err
		}
		byUser[userID] = append(byUser[userID], slug)
	}
	if err := grantRows.Err(); err != nil {
		return nil, err
	}

	// Completions: which labs each user has actually solved. Same bulk
	// join-in-memory shape as grants above - one query over the whole table
	// beats a per-user lookup at MVP scale. A row counts as solved only once
	// completed_at is set (the submitted flag), never merely for answered
	// quizzes, matching getProgress.complete and the leaderboard.
	completedRows, err := pool.Query(ctx, `SELECT user_id, module_slug FROM lab_progress WHERE completed_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer completedRows.Close()

	completedByUser := map[string][]string{}
	for completedRows.Next() {
		var userID, slug string
		if err := completedRows.Scan(&userID, &slug); err != nil {
			return nil, err
		}
		completedByUser[userID] = append(completedByUser[userID], slug)
	}
	if err := completedRows.Err(); err != nil {
		return nil, err
	}

	// Usage is a third bulk query for the same reason grants is: one aggregate
	// over lab_usage beats a per-user lookup, and it keeps the admin page a fixed
	// number of round trips as the user count grows.
	//
	// A failure here degrades rather than fails the request. User management has
	// to keep working - blocking an account, revoking a grant - even if the usage
	// rollup is unavailable, so the page renders without the consumption columns
	// instead of returning an error.
	usage, err := ListUserUsage(ctx, pool)
	if err != nil {
		slog.Error("list users: usage rollup unavailable, returning users without it", "error", err)
		usage = nil
	}

	for i := range users {
		if g, ok := byUser[users[i].ID]; ok {
			users[i].Grants = g
		}
		if c, ok := completedByUser[users[i].ID]; ok {
			users[i].Completed = c
		}
		if u, ok := usage[users[i].ID]; ok {
			users[i].Usage = &u
		}
		if at, ok := lastSolved[users[i].ID]; ok {
			atCopy := at
			users[i].LastSolvedAt = &atCopy
		}
		users[i].Entitled = entitledFrom(users[i].Grants, users[i].Completed)
	}

	// Order the admin list by recent activity: most-recently-solved first, then
	// (for users with no solve, or an exact tie) most-recently-registered. This
	// puts the operators an admin is likely acting on at the top, and keeps the
	// order stable by falling back to ID when both keys match.
	sort.SliceStable(users, func(i, j int) bool {
		si, sj := users[i].LastSolvedAt, users[j].LastSolvedAt
		if (si != nil) != (sj != nil) {
			return si != nil // a user with any solve outranks one with none
		}
		if si != nil && !si.Equal(*sj) {
			return si.After(*sj)
		}
		ci, cj := createdAt[users[i].ID], createdAt[users[j].ID]
		if !ci.Equal(cj) {
			return ci.After(cj)
		}
		return users[i].ID < users[j].ID
	})
	return users, nil
}

// entitledFrom returns the modules a user can access: explicit grants folded in
// with the free progression unlocks (tier-0 for everyone, plus each tier the
// previous one unlocked). Pure counterpart to EntitledModules - same union, but
// over slices the caller already has rather than a DB read - so the admin access
// column stays unit-testable.
func entitledFrom(grants, completed []string) []string {
	solved := make(map[string]bool, len(completed))
	for _, s := range completed {
		solved[s] = true
	}
	set := make(map[string]bool, len(grants))
	for _, s := range grants {
		set[s] = true
	}
	for s := range progressionEntitled(solved) {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// PromoteIfAllowlisted raises a user to admin when their email is in
// cfg.AdminEmails. Called on every OAuth sign-in rather than only at creation,
// so adding an email to the allowlist takes effect on that person's next login
// without any manual database surgery.
//
// Deliberately one-way: it never demotes. Removing an email from the allowlist
// won't strip an existing admin (do that from the user-management page), so a
// config typo can't silently lock every admin out of the system.
//
// Returns the (possibly updated) role so the caller can hand the client an
// accurate one without a second read.
func PromoteIfAllowlisted(ctx context.Context, pool *pgxpool.Pool, cfg Config, u *User) (string, error) {
	if u == nil || IsAdmin(u.Role) || !emailAllowlisted(cfg.AdminEmails, u.Email) {
		if u == nil {
			return "", nil
		}
		return u.Role, nil
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, u.ID); err != nil {
		return u.Role, err
	}
	u.Role = "admin"
	return u.Role, nil
}

// emailAllowlisted reports whether email appears in a comma-separated list.
// Case-insensitive and whitespace-tolerant, because the list is hand-written
// config and email comparison is case-insensitive in practice. An empty list
// matches nothing - importantly, an empty email can't match an empty entry.
func emailAllowlisted(list, email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || strings.TrimSpace(list) == "" {
		return false
	}
	for _, entry := range strings.Split(list, ",") {
		if e := strings.TrimSpace(strings.ToLower(entry)); e != "" && e == email {
			return true
		}
	}
	return false
}

// DeleteUser permanently removes an account. Its lab_grants rows go with it
// via ON DELETE CASCADE (see db/schema.go), so this is a single statement.
//
// Irreversible - SetBlocked is the reversible option, and is what the UI
// steers toward. Any running sessions are torn down by the caller, which owns
// the labs.Manager.
func DeleteUser(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	_, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

// GetUser looks up a single account by ID. Used by the admin endpoints to
// validate their target exists before mutating it, and by requireAuth to
// re-check role/blocked on every request.
func GetUser(ctx context.Context, pool *pgxpool.Pool, id string) (*User, bool, error) {
	return findUser(ctx, pool, "id", id)
}
