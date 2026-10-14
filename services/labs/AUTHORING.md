# Lab authoring guide

Practical guide for adding labs, distilled from building the RBAC-escalation and
container-escape tracks. Read `_template/README.md` first for the anatomy; this
covers the patterns, the traps, and the local test loop that `_template` does not.

## A lab is four artifacts

1. **Manifest bundle** - `services/backend/internal/labs/modules/<slug>.yaml`,
   `//go:embed`-ed in `internal/labs/modules.go`. The attack target.
2. **Registry entry** - a `Module` in `internal/labs/registry.go` (slug, display
   name, description, hint, `Order`, `Timing`, `NestedSpec` with entry pod and
   `AttackPath`, and **`Flag`** - the exact `k8labs{...}` value the bundle plants).
   Also add the slug to `registry_order_test.go`'s `want` list in its `Order`
   position, or that test fails. `TestModuleFlagMatchesManifest` asserts `Flag`
   appears verbatim in the manifest, so the two can never drift.
3. **Course** - `internal/course/content/<slug>/NN-name.md`, one screen per file.
4. **README** - `services/labs/<slug>/README.md` (vuln class, solution, fix,
   what a defender sees, and a **verification status** note).

`Order` (10..130) arranges the sidebar as a **four-act learning path** - climbing
in difficulty within each act, keeping dependent labs adjacent (attack then its
detection; escape then what it enables): 1) Foundations, 2) Identity/RBAC abuse,
3) Break out to the node, 4) Cluster-wide persistence (see the block comment in
`registry_order_test.go` for the full sequence). Slot a new lab into the act it
belongs to, at the difficulty step it fits; keep `registry_order_test.go`'s `want`
list in sync. Leave 10-gaps for future inserts.

## Shared manifest patterns (proven across 8 labs)

- **Reuse the baked foothold image** `k8labs-foothold:1.31` with
  `imagePullPolicy: Never` for every pod - the foothold, victim pods, and pods the
  student creates. The nested cluster pulls nothing (airgap). Never introduce a
  new image unless the lab genuinely needs a running service (see the webhook
  deferral below) - that is the one boundary that leaves the "just YAML" model.
- **Declare an explicit ServiceAccount** the bundle creates and reference it from
  the entry pod, even a powerless one. The namespace's auto-created `default` SA
  races the backend's fast apply ("serviceaccount default not found").
- **The flag is bait**, planted directly in the bundle (a Secret's `stringData`,
  or a node file written by an initContainer). Never anything under `secrets/`.
- **RBAC labs**: foothold SA holds exactly the one over-broad grant; a direct read
  fails and only the exploit path succeeds. Verify with `kubectl auth can-i ...
  --as=system:serviceaccount:<ns>:<sa>` against any cluster - it exercises the real
  authorizer without needing a session.
- **Escape labs**: the foothold pod carries the dangerous setting (`hostPath /`,
  `privileged` + `hostPID`); the flag is a node file planted by an initContainer
  using the same primitive the student will use (write via the host mount, or via
  `/proc/1/root`). `nsenter` is NOT in the foothold image - use `/proc/1/root`
  (needs only coreutils) for privileged breakouts.

## Escape-lab containment (important)

"The node" a student escapes to is the **disposable nested cluster's node** - the
session-runner's **Kata microVM guest in prod**, the runner pod's **own container
under runc locally**. The microVM is the real wall, which is why it is safe to
hand a student `privileged`/`hostPath`. Consequence: **do not exploit-run an escape
lab on the outer k3d cluster** - there the "node" is your real dev node. Escape
labs are verified correct-by-construction + build/boot; the exploit-run belongs to
the isolated session.

## Course authoring: the traps that panic the backend

A malformed course file **panics the backend at boot** (`course.mustLoadAll`), so
always `go test ./internal/course/... ./internal/labs/...` before hot-swapping.
The frontmatter is YAML; the failures that bit us, all mechanical:

- **No `": "` (colon-space) in an unquoted frontmatter scalar** (`q`, `explain`,
  `title`, `aha`, `revealLabel`) - YAML reads it as a mapping. This includes inside
  backticks: `` `imagePullPolicy: Never` `` breaks; use `` `imagePullPolicy` `` to
  `` `Never` ``. Note `k8labs-foothold:1.31` is fine (colon-digit, no space).
- **A scalar may not lead with a backtick** - `` q: `builder` needs... `` chokes the
  tokenizer. Start with a word: `The `builder` account...`.
- **Cold-term lint**: every backticked term used in a quiz `q`/`explain` or a
  concept `aha` must have been `backtick`-introduced in an **earlier concept body**.
  Introduce compound terms in the exact form you will reuse (`` `imagePullPolicy` ``
  and `` `Never` `` separately, not `` `imagePullPolicy: Never` ``).
- **Diagram bounds/limits** (schema): canvas 640x300, safe area x 20-620 / y 30-270,
  caption band y>240. `box.label` <=28, `box.sub` <=32, free `label.text` <=40,
  `caption` <=64. A container box that holds other boxes must be `type: frame`
  (exempt from the overlap lint; `frame` has no `sub`). Tones: neutral/accent/warn/
  danger.
- Quiz: 2-4 options, exactly one `correct`. Concept needs `title`+`body`+`aha`.
- **Flag capstone**: every course ends with exactly one `kind: flag` step (needs
  `title`+`body`, `aha` optional) - the "paste what you captured" screen that
  marks the lab solved. The expected value is NOT in the content; it lives in the
  registry's `Module.Flag`. Completion is the flag, not the quizzes: quizzes are
  formative self-checks now, answered right or wrong. `TestEmbeddedCoursesLoadAndLint`
  enforces the single trailing flag step.

A preflight script that catches the first three lives at `services/labs/lint_course.py`
(run from `services/backend`: `python3 ../labs/lint_course.py <slug>`) - run it before `go test` to save round-trips. Structure
each course like `ns-escape`: START -> concepts -> interleaved GUT-CHECK quizzes ->
THE LAB (foothold, with runnable `$ cmd` chips for recon but the final payoff left
for the student to assemble) -> RECAP/the fix -> CAPTURE (the `kind: flag` capstone).
~11-13 screens, ~3 quizzes, one flag.

## Local test loop (Tilt not required)

Content is embedded, so a new lab needs the backend rebuilt and reloaded. When Tilt
is not running, hot-swap manually - see the `local-backend-k3d-tilt` memory for the
exact steps. The one non-obvious bit: trigger the restart with a **content write**
(`date -u +%s > /tmp/.restart-proc`), not `touch`, and never `kill` the process
(that resets the container to the image binary). Verify via the stamped `/healthz`
sha.

## The service-needing lab: malicious-webhook (built)

All ten are built. `static-pod-persist` was unblocked by confirming k3s's
`staticPodPath` (`/var/lib/rancher/k3s/agent/pod-manifests`, from the kubelet
`configz`) and verifying the mirror-pod mechanism on the live node.

`malicious-webhook` is the one lab that needs a **running TLS server** (an
admission webhook must serve HTTPS) - the boundary that leaves the pure-YAML model.
It was solved **without a shared-image change**: the server is a self-contained,
stdlib-only Go binary (`malicious-webhook/webhook-server/main.go`, dual-arch),
carried in the (generated) bundle gzipped + chunked as `binaryData` ConfigMaps and
reassembled by an initContainer - same airgap-friendly spirit as `suid-privesc`'s
embedded binary, just larger (~5 MB added to the backend via `//go:embed`).

- **Verified**: server AdmissionReview/JSONPatch logic (standalone over TLS),
  byte-identical dual-arch reassembly, student RBAC (`--as`).
- **Session-only**: the end-to-end injection wiring (API server -> webhook Service
  DNS, cert SAN, the roll, the sidecar read) - more untested surface than any other
  lab, standard k8s and correct-by-construction. The course guides safe scoping
  (`namespaceSelector: production`, `failurePolicy: Ignore`) so a student can't
  wedge the cluster.
- **Future refactor (option B)**: bake the `webhook` binary into the shared
  foothold image for a tiny manifest, reused by future TLS labs - a deliberate
  follow-up, not required to ship this one.

**Takeaway for future labs**: anything needing a running service (not just a shell)
is the boundary that leaves the pure-YAML model. A self-contained static binary is
the answer - carried in the bundle (as here) for zero infra change, or baked into
the shared foothold image (option B) once there are several such labs.
