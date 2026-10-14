package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"backend/internal/labs"
	"backend/internal/topology"
)

// sensorKey turns a Defender pod selector ("app=tetragon") into the bare
// workload key the topology renderer resolves against pod app/workload/name
// ("tetragon"). Returns the input unchanged if it carries no "=".
func sensorKey(podLabel string) string {
	if _, v, ok := strings.Cut(podLabel, "="); ok {
		return v
	}
	return podLabel
}

// sessionTopology godoc
// @Summary     Get the topology of your own lab cluster
// @Description The student-facing map: their session's disposable nested cluster, plus one opaque node standing for everything outside it. Ownership-gated exactly like the terminal - any other authenticated user gets 403, not a filtered view.
// @Description Built from the session's nested kubeconfig alone, never from the admin topology. That is structural rather than a filter: the outer cluster's node names, kernel versions, dashboards, signals and other users' sessions never enter the code path, so there is nothing to forget to strip. See internal/topology/scoped.go.
// @Tags        labs
// @Produce     json
// @Param       id path string true "Session ID"
// @Success     200 {object} topology.ScopedTopology
// @Failure     401 {string} string "unauthorized"
// @Failure     403 {string} string "forbidden - not this session's owner"
// @Failure     404 {string} string "not found"
// @Security    BearerAuth
// @Router      /sessions/{id}/topology [get]
func sessionTopology(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		sess, ok := mgr.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		// Same gate as the terminal and delete. A session id is not the access
		// control here; this check is.
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess.OwnerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		kc, cached := mgr.NestedKubeconfigCached(id)
		if !cached {
			// Not fetched yet (still provisioning, or reconciled after a
			// restart). Kick off the one-time background fetch and tell the
			// student to wait rather than erroring - the next poll fills in.
			mgr.EnsureNestedKubeconfig(id)
			json.NewEncoder(w).Encode(&topology.ScopedTopology{
				Boundary: topology.BoundaryRef,
				Error:    "your lab cluster is still starting",
			})
			return
		}

		// spoilers=true: draw the attack path. The course already walks the
		// student to the answer, so the map showing the same route is a visual
		// aid, not a giveaway - this is a deliberate product choice.
		out := topology.Scoped(r.Context(), kc, true)
		// The derived escape edge (foothold -> production) covers cross-namespace
		// labs. A local escalation has none to derive, so the module declares its
		// route (see NestedSpec.AttackPath); add it here where the module is known.
		if module, ok := labs.GetModule(sess.ModuleSlug); ok && module.Nested != nil {
			if ap := module.Nested.AttackPath; ap != nil {
				out.Edges = append(out.Edges, topology.Edge{From: ap.From, To: ap.To, Port: ap.Label, Kind: "escape"})
			}
			// An eBPF module runs an in-guest sensor that watches the kernel the
			// lab pod runs on. That relationship is real and is the whole point of
			// the module, but nothing in the enumerated cluster expresses it - the
			// sensor never talks to the pod, so no Service, NetworkPolicy or
			// traffic reveals it. Name it here as an "observes" edge, which the
			// renderer draws as passive observation (eye, no port) rather than a
			// communication arrow. No Port: there is no network hop to label.
			if module.EBPF && module.Nested.Defender != nil && module.Nested.EntryPod != "" {
				out.Edges = append(out.Edges, topology.Edge{
					From: sensorKey(module.Nested.Defender.PodLabel),
					To:   module.Nested.EntryPod,
					Kind: "observes",
				})
			}
		}
		json.NewEncoder(w).Encode(out)
	}
}
