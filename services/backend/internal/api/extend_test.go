package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend/internal/labs"
)

// runningResearchSession seeds a running research-lab session created `age` ago
// with the given MaxAge, plus an attached (echo) shell so /context reports ready.
func runningResearchSession(t *testing.T, mgr *labs.Manager, id string, age, maxAge time.Duration) {
	t.Helper()
	mgr.SetSessionForTest(&labs.Session{
		ID:           id,
		OwnerID:      "u1",
		ModuleSlug:   "driver-research-dev",
		Status:       "running",
		CreatedAt:    time.Now().Add(-age),
		LastActivity: time.Now(),
		Timing:       labs.Timing{MaxAge: maxAge, Idle: 30 * time.Minute},
	})
	mgr.AttachEchoShellForTest(id)
}

// --- POST /ai/.../extend (the AI's grant-gated self-extend) ---

func TestAIExtendHappyPath(t *testing.T) {
	mgr := labs.NewManagerForTest()
	runningResearchSession(t, mgr, "sess1", 0, 2*time.Hour)

	req := httptest.NewRequest("POST", "/api/v1/ai/sessions/sess1/extend", nil)
	req.SetPathValue("id", "sess1")
	rec := httptest.NewRecorder()
	aiExtend(mgr)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var body struct {
		ExpiresAt        string `json:"expiresAt"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Started at ~2h, +2h step => ~4h remaining. Allow slack for test runtime.
	if body.ExpiresInSeconds < int((4*time.Hour - time.Minute).Seconds()) {
		t.Fatalf("expiresInSeconds = %d, want ~4h", body.ExpiresInSeconds)
	}
	if body.ExpiresAt == "" {
		t.Fatal("expiresAt missing")
	}
}

func TestAIExtendNonResearchForbidden(t *testing.T) {
	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{
		ID: "g1", OwnerID: "u1", ModuleSlug: "ns-escape", Status: "running",
		CreatedAt: time.Now(), Timing: labs.Timing{MaxAge: time.Hour, Idle: 30 * time.Minute},
	})

	req := httptest.NewRequest("POST", "/api/v1/ai/sessions/g1/extend", nil)
	req.SetPathValue("id", "g1")
	rec := httptest.NewRecorder()
	aiExtend(mgr)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestAIExtendNotRunningConflict(t *testing.T) {
	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{
		ID: "sess1", OwnerID: "u1", ModuleSlug: "driver-research-dev", Status: "starting",
		CreatedAt: time.Now(), Timing: labs.Timing{MaxAge: 2 * time.Hour, Idle: 30 * time.Minute},
	})

	req := httptest.NewRequest("POST", "/api/v1/ai/sessions/sess1/extend", nil)
	req.SetPathValue("id", "sess1")
	rec := httptest.NewRecorder()
	aiExtend(mgr)(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestAIExtendAtCapConflict(t *testing.T) {
	mgr := labs.NewManagerForTest()
	// Already at the 48h ceiling: further extend is refused.
	runningResearchSession(t, mgr, "sess1", 0, labs.ExtendHardCap)

	req := httptest.NewRequest("POST", "/api/v1/ai/sessions/sess1/extend", nil)
	req.SetPathValue("id", "sess1")
	rec := httptest.NewRecorder()
	aiExtend(mgr)(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestAIExtendMissingGone(t *testing.T) {
	mgr := labs.NewManagerForTest()
	req := httptest.NewRequest("POST", "/api/v1/ai/sessions/ghost/extend", nil)
	req.SetPathValue("id", "ghost")
	rec := httptest.NewRecorder()
	aiExtend(mgr)(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", rec.Code)
	}
}

// --- POST /sessions/{id}/extend (the researcher's own auth-gated extend) ---

func TestExtendSessionOwnerGate(t *testing.T) {
	mgr := labs.NewManagerForTest()
	runningResearchSession(t, mgr, "sess1", 0, 2*time.Hour)

	req := httptest.NewRequest("POST", "/api/v1/sessions/sess1/extend", nil)
	req.SetPathValue("id", "sess1")
	// A different authenticated user must be refused before learning anything else.
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "someone-else"))
	rec := httptest.NewRecorder()
	extendSession(mgr)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestExtendSessionHappyPath(t *testing.T) {
	mgr := labs.NewManagerForTest()
	runningResearchSession(t, mgr, "sess1", 0, 2*time.Hour)
	before, _ := mgr.Get("sess1")
	beforeExp := before.ExpiresAt()

	req := httptest.NewRequest("POST", "/api/v1/sessions/sess1/extend", nil)
	req.SetPathValue("id", "sess1")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "u1"))
	rec := httptest.NewRecorder()
	extendSession(mgr)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	// The returned session carries the freshly-bumped expiresAt (MarshalJSON).
	var body struct {
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.ExpiresAt.After(beforeExp) {
		t.Fatalf("expiresAt %v did not advance past %v", body.ExpiresAt, beforeExp)
	}
	if got := body.ExpiresAt.Sub(beforeExp); got != labs.ExtendStep {
		t.Fatalf("expiresAt advanced by %v, want %v", got, labs.ExtendStep)
	}
}

func TestExtendSessionNonResearchForbidden(t *testing.T) {
	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{
		ID: "g1", OwnerID: "u1", ModuleSlug: "ns-escape", Status: "running",
		CreatedAt: time.Now(), Timing: labs.Timing{MaxAge: time.Hour, Idle: 30 * time.Minute},
	})

	req := httptest.NewRequest("POST", "/api/v1/sessions/g1/extend", nil)
	req.SetPathValue("id", "g1")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "u1"))
	rec := httptest.NewRecorder()
	extendSession(mgr)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestExtendSessionMissing(t *testing.T) {
	mgr := labs.NewManagerForTest()
	req := httptest.NewRequest("POST", "/api/v1/sessions/ghost/extend", nil)
	req.SetPathValue("id", "ghost")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "u1"))
	rec := httptest.NewRecorder()
	extendSession(mgr)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// --- POST /sessions/{id}/keepalive (the visible-tab presence ping) ---

func TestKeepaliveResetsIdleForOwner(t *testing.T) {
	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{
		ID: "s", OwnerID: "u1", ModuleSlug: "driver-research-dev", Status: "running",
		CreatedAt: time.Now().Add(-40 * time.Minute), LastActivity: time.Now().Add(-40 * time.Minute),
		Timing: labs.Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
	})
	stale, _ := mgr.Get("s")
	before := stale.LastActivity

	req := httptest.NewRequest("POST", "/api/v1/sessions/s/keepalive", nil)
	req.SetPathValue("id", "s")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "u1"))
	rec := httptest.NewRecorder()
	keepaliveSession(mgr)(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	s, _ := mgr.Get("s")
	if !s.LastActivity.After(before) {
		t.Fatalf("keepalive did not reset the idle clock: LastActivity still %v", s.LastActivity)
	}
}

func TestKeepaliveRejectsNonOwner(t *testing.T) {
	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{
		ID: "s", OwnerID: "u1", ModuleSlug: "driver-research-dev", Status: "running",
		Timing: labs.Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
	})
	req := httptest.NewRequest("POST", "/api/v1/sessions/s/keepalive", nil)
	req.SetPathValue("id", "s")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "someone-else"))
	rec := httptest.NewRecorder()
	keepaliveSession(mgr)(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner keepalive = %d, want 403", rec.Code)
	}
}

// --- GET /ai/.../context expiry reporting ---

func TestAIContextReportsExpiresInSeconds(t *testing.T) {
	mgr := labs.NewManagerForTest()
	runningResearchSession(t, mgr, "sess1", 0, 2*time.Hour)

	req := httptest.NewRequest("GET", "/api/v1/ai/sessions/sess1/context", nil)
	req.SetPathValue("id", "sess1")
	rec := httptest.NewRecorder()
	aiContext(mgr)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp aiContextResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// expiresInSeconds reports the SOONER of the two clocks - here the 30m idle
	// deadline, not the 2h hard cap. Reading /context reset the idle clock, so it
	// measures ~30m from now. This is the deadline the AI must actually respect;
	// reporting the hard cap was what let it think it had hours before an idle reap.
	if lo, hi := int((29*time.Minute).Seconds()), int((31*time.Minute).Seconds()); resp.ExpiresInSeconds < lo || resp.ExpiresInSeconds > hi {
		t.Fatalf("expiresInSeconds = %d, want ~30m (the idle deadline)", resp.ExpiresInSeconds)
	}
	if _, warned := resp.Notes["expiry"]; warned {
		t.Fatalf("did not expect an expiry warning with ~30m left: %v", resp.Notes)
	}
}

func TestAIContextWarnsNearExpiry(t *testing.T) {
	mgr := labs.NewManagerForTest()
	// ~4 minutes left on a 2h clock: inside the 5-minute warning window.
	runningResearchSession(t, mgr, "sess1", 2*time.Hour-4*time.Minute, 2*time.Hour)

	req := httptest.NewRequest("GET", "/api/v1/ai/sessions/sess1/context", nil)
	req.SetPathValue("id", "sess1")
	rec := httptest.NewRecorder()
	aiContext(mgr)(rec, req)

	var resp aiContextResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, warned := resp.Notes["expiry"]; !warned {
		t.Fatalf("expected an expiry warning ~4min out, notes = %v", resp.Notes)
	}
}

func TestAIContextNoExtendWarnForGuidedLab(t *testing.T) {
	mgr := labs.NewManagerForTest()
	// A guided lab is not extendable: even near its deadline it gets no extend
	// nudge (the prompt never tells it to extend).
	mgr.SetSessionForTest(&labs.Session{
		ID: "g1", OwnerID: "u1", ModuleSlug: "ns-escape", Status: "running",
		CreatedAt: time.Now().Add(-(15*time.Minute - 4*time.Minute)),
		Timing:    labs.Timing{MaxAge: 15 * time.Minute, Idle: 30 * time.Minute},
	})
	mgr.AttachEchoShellForTest("g1")

	req := httptest.NewRequest("GET", "/api/v1/ai/sessions/g1/context", nil)
	req.SetPathValue("id", "g1")
	rec := httptest.NewRecorder()
	aiContext(mgr)(rec, req)

	var resp aiContextResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, warned := resp.Notes["expiry"]; warned {
		t.Fatalf("guided lab must not get an extend warning: %v", resp.Notes)
	}
}
