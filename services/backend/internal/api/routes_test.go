package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"backend/internal/accounts"
	"backend/internal/labs"
)

// managerWithSession builds a Manager holding one running session owned by
// ownerID. Manager.sessions is unexported, so seed it the way prod does after
// a restart - Reconcile adopting a labelled session-runner pod off the cluster.
func managerWithSession(t *testing.T, id, ownerID string) *labs.Manager {
	t.Helper()

	cfg := labs.Config{RunnerNamespace: "labs-runners"}
	clientset := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "runner-" + id,
			Namespace: cfg.RunnerNamespace,
			Labels: map[string]string{
				"app": "session-runner", "session": id, "module": "ns-escape", "owner": ownerID,
			},
		},
	})

	mgr := labs.NewManager(clientset, nil, cfg)
	if err := mgr.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if _, ok := mgr.Get(id); !ok {
		t.Fatalf("expected session %q to be adopted by Reconcile", id)
	}
	return mgr
}

// GET /sessions/{id} was registered without requireAuth while its POST/DELETE
// siblings had it, so an unauthenticated caller reached the handler and got a
// 404/200 on session existence rather than a 401. Assert the wiring in
// NewRouter, not just the handler - the bug was in the route table.
func TestGetSessionRouteRequiresAuth(t *testing.T) {
	mgr := managerWithSession(t, "abc123", "user-1")
	router := NewRouter(nil, mgr, accounts.Config{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/abc123", nil) // no cookie, no bearer
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated GET: got %d, want 401 - the route is missing requireAuth", rec.Code)
	}
}

// Auth alone isn't enough: any signed-in user could otherwise read any session
// by ID. Same ownership gate as deleteSession/terminalProxy.
func TestGetSessionRejectsNonOwner(t *testing.T) {
	mgr := managerWithSession(t, "abc123", "user-1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/abc123", nil)
	req.SetPathValue("id", "abc123")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-2"))
	rec := httptest.NewRecorder()
	getSession(mgr)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("non-owner GET: got %d, want 403", rec.Code)
	}
}

func TestGetSessionAllowsOwner(t *testing.T) {
	mgr := managerWithSession(t, "abc123", "user-1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/abc123", nil)
	req.SetPathValue("id", "abc123")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
	rec := httptest.NewRecorder()
	getSession(mgr)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("owner GET: got %d, want 200", rec.Code)
	}
}

// An unknown ID must stay a plain 404 for everyone - returning 403 there would
// turn the endpoint into an existence oracle for other users' session IDs.
func TestGetSessionUnknownIDIsNotFound(t *testing.T) {
	mgr := managerWithSession(t, "abc123", "user-1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/deadbeef", nil)
	req.SetPathValue("id", "deadbeef")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-2"))
	rec := httptest.NewRecorder()
	getSession(mgr)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown session GET: got %d, want 404", rec.Code)
	}
}

// Once a session is torn down, GET returns its tombstone - status "ended" and
// the reason - to the owner, instead of a bare 404. That's what lets the lab UI
// say WHY the lab stopped (idle vs time limit) rather than a generic message.
func TestGetSessionReturnsTombstoneAfterDelete(t *testing.T) {
	mgr := managerWithSession(t, "abc123", "user-1")
	if err := mgr.Delete(context.Background(), "abc123"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/abc123", nil)
	req.SetPathValue("id", "abc123")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-1"))
	rec := httptest.NewRecorder()
	getSession(mgr)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("owner GET after delete: got %d, want 200 (tombstone)", rec.Code)
	}
	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode tombstone: %v", err)
	}
	if body.Status != "ended" || body.Reason != labs.EndReasonStopped {
		t.Errorf("tombstone = {status:%q reason:%q}, want {ended stopped}", body.Status, body.Reason)
	}
}

// The tombstone is ownership-gated exactly like a live session: a non-owner
// gets 403, never the reason, and the endpoint doesn't become an oracle that
// distinguishes "was reaped" from "never existed" for someone else's session.
func TestGetSessionTombstoneRejectsNonOwner(t *testing.T) {
	mgr := managerWithSession(t, "abc123", "user-1")
	if err := mgr.Delete(context.Background(), "abc123"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/abc123", nil)
	req.SetPathValue("id", "abc123")
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-2"))
	rec := httptest.NewRecorder()
	getSession(mgr)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("non-owner tombstone GET: got %d, want 403", rec.Code)
	}
}
