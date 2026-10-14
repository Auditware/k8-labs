package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"backend/internal/labs"
)

// End-to-end exercise of the AI grant flow through the real handlers, mirroring
// exactly what the browser does: owner mints a grant, then the returned token is
// used against the /ai routes (context + exec). This is the path that failed on
// prod ("Couldn't generate a key"), so it is worth pinning down deterministically
// without a live nested cluster.
func TestAIGrantFlowEndToEnd(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")

	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{ID: "sess1", OwnerID: "u1", ModuleSlug: "ns-escape", Status: "running"})
	mgr.AttachEchoShellForTest("sess1") // simulate the student's terminal being open

	// 1) Owner mints the grant. aiGrantCreate runs behind requireAuth, which
	//    injects userIDKey; simulate that here.
	createReq := httptest.NewRequest("POST", "/api/v1/sessions/sess1/ai-grant", nil)
	createReq.SetPathValue("id", "sess1")
	createReq = createReq.WithContext(context.WithValue(createReq.Context(), userIDKey, "u1"))
	createRec := httptest.NewRecorder()
	aiGrantCreate(mgr)(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("ai-grant create: status = %d, want 200 (body: %s)", createRec.Code, createRec.Body.String())
	}
	var grant aiGrantResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &grant); err != nil {
		t.Fatalf("decode grant: %v", err)
	}
	if grant.Token == "" || grant.SessionID != "sess1" {
		t.Fatalf("unexpected grant: %+v", grant)
	}

	// 2) The AI reads context with the token, through the real gate.
	ctxReq := httptest.NewRequest("GET", "/api/v1/ai/sessions/sess1/context", nil)
	ctxReq.SetPathValue("id", "sess1")
	ctxReq.Header.Set("Authorization", "Bearer "+grant.Token)
	ctxRec := httptest.NewRecorder()
	requireAIGrant(mgr, aiContext(mgr))(ctxRec, ctxReq)
	if ctxRec.Code != http.StatusOK {
		t.Fatalf("ai context: status = %d, want 200", ctxRec.Code)
	}
	if !strings.Contains(ctxRec.Body.String(), "ns-escape") {
		t.Fatalf("context missing module slug: %s", ctxRec.Body.String())
	}

	// 3) The AI runs a command; the echo shell mirrors it back as output.
	execReq := httptest.NewRequest("POST", "/api/v1/ai/sessions/sess1/exec",
		strings.NewReader(`{"command":"kubectl auth can-i --list"}`))
	execReq.SetPathValue("id", "sess1")
	execReq.Header.Set("Authorization", "Bearer "+grant.Token)
	execRec := httptest.NewRecorder()
	requireAIGrant(mgr, aiExec(mgr))(execRec, execReq)
	if execRec.Code != http.StatusOK {
		t.Fatalf("ai exec: status = %d, want 200 (body: %s)", execRec.Code, execRec.Body.String())
	}
	var out struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal(execRec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode exec: %v", err)
	}
	if !strings.Contains(out.Output, "kubectl auth can-i --list") {
		t.Fatalf("exec output did not mirror the command: %q", out.Output)
	}
}

// A research session assisted by an AI can go many minutes between commands while
// the AI and researcher discuss - and that discussion is GET /context, not exec.
// So reading context must count as activity, or a talky-but-active session
// idle-reaps mid-conversation ("Session closed"). Regression test for that fix.
func TestAIContextCountsAsActivity(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")
	mgr := labs.NewManagerForTest()
	stale := time.Now().Add(-45 * time.Minute)
	mgr.SetSessionForTest(&labs.Session{
		ID: "sess1", OwnerID: "u1", ModuleSlug: "driver-research-dev", Status: "running",
		CreatedAt: time.Now().Add(-45 * time.Minute), LastActivity: stale,
		Timing: labs.Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
	})

	createReq := httptest.NewRequest("POST", "/api/v1/sessions/sess1/ai-grant", nil)
	createReq.SetPathValue("id", "sess1")
	createReq = createReq.WithContext(context.WithValue(createReq.Context(), userIDKey, "u1"))
	createRec := httptest.NewRecorder()
	aiGrantCreate(mgr)(createRec, createReq)
	var grant aiGrantResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &grant); err != nil {
		t.Fatalf("decode grant: %v", err)
	}

	ctxReq := httptest.NewRequest("GET", "/api/v1/ai/sessions/sess1/context", nil)
	ctxReq.SetPathValue("id", "sess1")
	ctxReq.Header.Set("Authorization", "Bearer "+grant.Token)
	requireAIGrant(mgr, aiContext(mgr))(httptest.NewRecorder(), ctxReq)

	s, ok := mgr.Get("sess1")
	if !ok {
		t.Fatal("session vanished")
	}
	if !s.LastActivity.After(stale) {
		t.Fatalf("reading /context did not reset the idle clock: LastActivity still %v", s.LastActivity)
	}
}

// The prod symptom reproduced: a session the backend no longer knows about (the
// classic case being a session that predated a backend restart, since the
// Manager is restart-losable) makes ai-grant 404 - which the UI shows as
// "Couldn't generate a key". Confirms the failure is environmental, not a bug in
// the mint path.
func TestAIGrantCreateUnknownSession(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")
	mgr := labs.NewManagerForTest() // no session seeded

	req := httptest.NewRequest("POST", "/api/v1/sessions/ghost/ai-grant", nil)
	req.SetPathValue("id", "ghost")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "u1"))
	rec := httptest.NewRecorder()
	aiGrantCreate(mgr)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown session: status = %d, want 404", rec.Code)
	}
}
