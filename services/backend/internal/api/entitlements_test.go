package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/accounts"
	"backend/internal/labs"
)

// authedRequest builds a request carrying the user requireAuth would have
// loaded. The handlers under test read the account off the context rather than
// re-querying, so this exercises them without a database.
func authedRequest(method, target string, body string, u *accounts.User) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	ctx := context.WithValue(req.Context(), userIDKey, u.ID)
	ctx = context.WithValue(ctx, userKey, u)
	return req.WithContext(ctx)
}

// Registration is open but entitlement is not: a newly registered user has no
// grants and so must see an empty catalog, not the full one.
func TestFilterModulesHidesUngranted(t *testing.T) {
	all := labs.ListModules()
	if len(all) < 2 {
		t.Skipf("need at least 2 modules in the catalog, got %d", len(all))
	}

	if got := filterModules(all, nil); len(got) != 0 {
		t.Errorf("user with no grants: got %d modules, want 0", len(got))
	}

	got := filterModules(all, []string{all[0].Slug})
	if len(got) != 1 || got[0].Slug != all[0].Slug {
		t.Errorf("single grant: got %v, want just %q", slugsOf(got), all[0].Slug)
	}
}

// The empty case must serialize as [] and not null - the frontend maps over
// this response directly.
func TestFilterModulesEncodesEmptyAsArray(t *testing.T) {
	out, err := json.Marshal(filterModules(labs.ListModules(), nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != "[]" {
		t.Errorf("no grants encoded as %s, want []", out)
	}
}

// An admin bypasses the grants table: visibleModules returns the whole catalog
// regardless of the (unused) entitlement set, so the admin catalog never depends
// on a grants query. Pure, so it needs no pool. The handler now DOES read
// solved-state from the DB for admins too (the green-check fix), so the old
// nil-pool *handler* test no longer holds - that assertion moves down to this
// pure selection helper.
func TestVisibleModulesAdminSeesFullCatalog(t *testing.T) {
	all := labs.ListModules()
	got := visibleModules(all, true, nil)
	if len(got) != len(all) {
		t.Errorf("admin saw %d modules, want the full catalog (%d)", len(got), len(all))
	}
}

// A regular user sees only the entitled modules - visibleModules delegates to
// filterModules for the non-admin case.
func TestVisibleModulesUserFilteredByEntitlement(t *testing.T) {
	all := labs.ListModules()
	got := visibleModules(all, false, []string{all[0].Slug})
	if len(got) != 1 || got[0].Slug != all[0].Slug {
		t.Errorf("user saw %d modules, want just %q", len(got), all[0].Slug)
	}
}

// createSession reads the caller off the context rather than re-querying, so
// an unauthenticated request reaching the handler must be refused outright
// rather than falling through with an empty owner ID and provisioning a lab
// nobody owns.
func TestCreateSessionRequiresContextUser(t *testing.T) {
	all := labs.ListModules()
	if len(all) == 0 {
		t.Skip("no modules in catalog")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions",
		strings.NewReader(`{"module":"`+all[0].Slug+`"}`)) // no user in context
	rec := httptest.NewRecorder()

	createSession(nil, nil)(rec, req) // nil pool: must refuse before any query

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no context user: got %d, want 401", rec.Code)
	}
}

// A typo'd slug must be rejected rather than written as a grant that silently
// never matches anything - the user would appear entitled in the admin UI and
// still be unable to start the lab.
func TestAdminGrantRejectsUnknownModule(t *testing.T) {
	admin := &accounts.User{ID: "admin-1", Role: "admin"}
	req := authedRequest(http.MethodPost, "/api/v1/admin/users/u-1/grants",
		`{"module":"definitely-not-a-real-module"}`, admin)
	req.SetPathValue("id", "u-1")
	rec := httptest.NewRecorder()

	adminGrantLab(nil)(rec, req) // nil pool: validation must precede the DB

	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown module: got %d, want 400", rec.Code)
	}
}

// Blocking must deny every login path. finishLogin is the single chokepoint
// all three (Google, GitHub, password) funnel through, so the check belongs
// there and this asserts it holds.
func TestFinishLoginRejectsBlockedUser(t *testing.T) {
	blocked := &accounts.User{ID: "u-1", Email: "b@example.com", Blocked: true}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()

	finishLogin(rec, req, blocked)

	if rec.Code != http.StatusForbidden {
		t.Errorf("blocked user login: got %d, want 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "token") {
		t.Error("blocked user was issued a token")
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "token" && c.Value != "" {
			t.Error("blocked user was issued an auth cookie")
		}
	}
}

// An admin blocking themselves locks them out of the page needed to undo it -
// with a single admin account that bricks the deployment.
func TestAdminCannotBlockSelf(t *testing.T) {
	admin := &accounts.User{ID: "admin-1", Role: "admin"}
	req := authedRequest(http.MethodPost, "/api/v1/admin/users/admin-1/blocked", `{"blocked":true}`, admin)
	req.SetPathValue("id", "admin-1")
	rec := httptest.NewRecorder()

	adminSetBlocked(nil, nil)(rec, req) // nil pool: the guard must fire before any query

	if rec.Code != http.StatusBadRequest {
		t.Errorf("self-block: got %d, want 400", rec.Code)
	}
}

// Revoking a grant while the user has that lab open would otherwise leave them
// a live root shell in the pod until it timed out.
func TestKillSessionsScopedToUserAndModule(t *testing.T) {
	mgr := managerWithSession(t, "sess-1", "user-1")

	// Another user's session must survive a teardown aimed at user-1.
	killSessions(context.Background(), mgr, "user-2", "")
	if _, ok := mgr.Get("sess-1"); !ok {
		t.Error("tore down a session belonging to a different user")
	}

	// A revoke for a module the user isn't running must not touch this one.
	killSessions(context.Background(), mgr, "user-1", "some-other-module")
	if _, ok := mgr.Get("sess-1"); !ok {
		t.Error("tore down a session for an unrelated module")
	}

	// The matching case does tear it down.
	killSessions(context.Background(), mgr, "user-1", "ns-escape")
	if _, ok := mgr.Get("sess-1"); ok {
		t.Error("session survived a revoke of its own module")
	}
}

func slugsOf(mods []labs.Module) []string {
	out := make([]string, len(mods))
	for i, m := range mods {
		out[i] = m.Slug
	}
	return out
}
