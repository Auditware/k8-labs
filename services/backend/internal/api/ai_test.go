package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/labs"
)

// A ScopeAI token must be refused by requireAuth on the normal API surface, so a
// leaked AI key can never reach a user route (flag submission, profile, ...).
// This is the deterministic confinement the whole design leans on.
func TestRequireAuthRejectsAIScopedToken(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")
	token, err := auth.IssueAIToken("user-1", "s1", "jti-1", time.Hour)
	if err != nil {
		t.Fatalf("IssueAIToken: %v", err)
	}

	called := false
	// pool is never reached: the scope check 401s before any DB lookup.
	h := requireAuth(nil, func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest("GET", "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("handler must not run for an AI-scoped token")
	}
}

func TestRequireAIGrant(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")

	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{ID: "s1", OwnerID: "u1", ModuleSlug: "ns-escape", Status: "running"})
	mgr.GrantAI("s1", "jti-1", time.Hour)

	goodToken, _ := auth.IssueAIToken("u1", "s1", "jti-1", time.Hour)

	call := func(id, authz string) *httptest.ResponseRecorder {
		reached := false
		h := requireAIGrant(mgr, func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(200) })
		req := httptest.NewRequest("GET", "/api/v1/ai/sessions/"+id+"/context", nil)
		req.SetPathValue("id", id)
		if authz != "" {
			req.Header.Set("Authorization", authz)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		_ = reached
		return rec
	}

	if rec := call("s1", "Bearer "+goodToken); rec.Code != 200 {
		t.Fatalf("valid grant: status = %d, want 200", rec.Code)
	}
	if rec := call("s1", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth: status = %d, want 401", rec.Code)
	}
	// A normal login token (no ai scope) must not open /ai.
	loginTok, _ := auth.IssueLoginToken("u1", "u@example.com", time.Hour)
	if rec := call("s1", "Bearer "+loginTok); rec.Code != http.StatusUnauthorized {
		t.Fatalf("login token on /ai: status = %d, want 401", rec.Code)
	}
	// Token bound to s1 cannot be used on another session's path.
	if rec := call("s2", "Bearer "+goodToken); rec.Code != http.StatusUnauthorized {
		t.Fatalf("cross-session token: status = %d, want 401", rec.Code)
	}
	// Revoked grant is dead even with a cryptographically valid token.
	mgr.RevokeAI("s1")
	if rec := call("s1", "Bearer "+goodToken); rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked grant: status = %d, want 401", rec.Code)
	}
}

func TestRequireAIGrantSessionGone(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")
	mgr := labs.NewManagerForTest()
	// Grant exists but the session was torn down (not in the live map).
	mgr.GrantAI("s1", "jti-1", time.Hour)
	tok, _ := auth.IssueAIToken("u1", "s1", "jti-1", time.Hour)

	h := requireAIGrant(mgr, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	req := httptest.NewRequest("GET", "/api/v1/ai/sessions/s1/context", nil)
	req.SetPathValue("id", "s1")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("session gone: status = %d, want 410", rec.Code)
	}
}
