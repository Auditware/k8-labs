package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/course"
	"backend/internal/labs"
)

// The AI tutor must see only the course slides the student has already reached.
// Capping the lesson at currentStep is what stops an eager assistant from
// synthesizing and printing the whole solution (up to and including the flag
// step) the moment it reads /context - the spoiler a student reported even when
// they had not asked for the answer. These pin the gate: mid-course the AI gets
// steps 0..N and never the later ones; with no reported position it gets only
// the opening step, never the solve.
func TestAIContextCapsLessonAtCurrentStep(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")

	c, ok := course.Get("ns-escape")
	if !ok || len(c.Steps) < 4 {
		t.Fatalf("ns-escape course not loaded / too short: ok=%v steps=%d", ok, len(c.Steps))
	}
	lastIdx := c.Steps[len(c.Steps)-1].Index // the flag / capture step

	fetch := func(t *testing.T, mgr *labs.Manager, id string) aiContextResponse {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/ai/sessions/"+id+"/context", nil)
		req.SetPathValue("id", id)
		rec := httptest.NewRecorder()
		aiContext(mgr)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("context status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var resp aiContextResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode context: %v", err)
		}
		return resp
	}

	// Student is on step 2: the AI sees steps 0..2 and nothing past it.
	mgr := labs.NewManagerForTest()
	mgr.SetSessionForTest(&labs.Session{ID: "sess1", OwnerID: "u1", ModuleSlug: "ns-escape", Status: "running"})
	mgr.AttachEchoShellForTest("sess1")
	mgr.SetGuideStep("sess1", 2, len(c.Steps))

	resp := fetch(t, mgr, "sess1")
	if len(resp.Lesson) == 0 {
		t.Fatalf("expected some lesson steps at currentStep=2, got 0")
	}
	for _, s := range resp.Lesson {
		if s.Index > 2 {
			t.Fatalf("lesson leaked step index %d past currentStep=2 (spoiler)", s.Index)
		}
		if s.Index == lastIdx {
			t.Fatalf("flag/capture step %d exposed to the AI mid-course", lastIdx)
		}
	}
	if got := resp.Lesson[len(resp.Lesson)-1].Index; got != 2 {
		t.Fatalf("highest visible step = %d, want 2", got)
	}

	// Each reached slide's visual is handed to the AI as text, so the tutor can
	// reinforce the picture before a command rather than jumping to the answer.
	var sawDiagram bool
	for _, s := range resp.Lesson {
		if strings.TrimSpace(s.Diagram) != "" {
			sawDiagram = true
		}
	}
	if !sawDiagram {
		t.Fatalf("no visible lesson step carried a diagram summary; the AI cannot reinforce the visual")
	}

	// No reported position: the AI gets only the opening step, never the solve.
	mgr2 := labs.NewManagerForTest()
	mgr2.SetSessionForTest(&labs.Session{ID: "sess2", OwnerID: "u1", ModuleSlug: "ns-escape", Status: "running"})
	mgr2.AttachEchoShellForTest("sess2")

	resp2 := fetch(t, mgr2, "sess2")
	if len(resp2.Lesson) != 1 || resp2.Lesson[0].Index != 0 {
		got := -1
		if len(resp2.Lesson) > 0 {
			got = resp2.Lesson[0].Index
		}
		t.Fatalf("unknown position should expose only step 0, got %d steps (first idx %d)", len(resp2.Lesson), got)
	}
	// Step 0's visual reaches the AI as its caption, so the tutor opens on the
	// same picture the student is looking at.
	if !strings.Contains(resp2.Lesson[0].Diagram, "one wrong word turns a fence into a suggestion") {
		t.Fatalf("step 0 diagram summary missing its caption: %q", resp2.Lesson[0].Diagram)
	}
}
