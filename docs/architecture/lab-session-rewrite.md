# Lab-session rewrite plan

**Goal:** turn a lab session from "two paragraphs and a terminal" into a guided
course. The left pane becomes a click-through visual primer that builds knowledge
one screen at a time; the right pane gains a Topology tab showing the student's
*own* nested cluster and nothing else.

Four independent tracks, one shared prerequisite (the content model):

- **Content** - authored steps with diagrams, schema-validated, git-only and
  embedded in the backend binary.
- **Timer** - authoritative from the API, per-lab duration set by an admin.
- **Panes** - resizable course pane; Terminal / Topology tabs on the right.
- **Scoped topology** - a *separate* endpoint built from a *separate* data
  source, so there is no outer-cluster data to leak.

## Decisions (baked in)

1. **The course engine is ported, not reinvented.** The design system, stepper,
   `aha` reveal, and quiz mechanics come from the `training-guide` skill's
   `assets/template.html`. Its two-column card stacks below 900px - diagram on
   top, prose below - and that stacked form *is* the side-pane layout. Reskin
   `--accent` + neutrals to the app's slate/blue; `--warn`/`--danger` stay
   semantic-only.

2. **One JSON Schema, one authority.** `step.schema.json` describes the parsed
   step object and is what CI validates every course file against. TS and Go
   types are generated from it, never hand-written. The schema is not the
   interesting part on its own - what it enables is A4, the lints that catch the
   two failure modes the skill's own `authoring.md` says can otherwise only be
   caught by eyeballing the page in a browser.

3. **On-disk form is Markdown + YAML frontmatter.** Lessons are prose and get
   reviewed as prose; JSON blobs don't diff usefully in a PR. The schema
   describes the *parsed* object, not the file bytes.

4. **Diagrams are declarative, never code.** The template's `svg: dSomething` is
   an arbitrary JS function - unrepresentable in Markdown, and a shape no lint
   can reason about. Instead a `diagram:` block lists boxes / wires / labels / seals,
   compiled by a TypeScript port of the template's `box`/`wire`/`lbl`/`seal`
   helpers. This is what makes the skill's two named failure modes
   *mechanically testable* (see A4) instead of eyeball-only.

5. **No raw HTML or SVG in content.** Prose goes through a restricted Markdown
   renderer with an allowlist; the template's `<span class="k">term</span>`
   becomes backtick syntax rather than literal markup. With content in git this
   is defence in depth rather than the primary control (see decision 6), but it
   costs nothing and it is what keeps the term-chip lint able to see every
   first-use of a term.

6. **Course content lives in git only - there is no admin content editor.**
   Content is authored locally, reviewed in a PR, embedded in the backend binary
   (`//go:embed`, exactly as the module's manifest bundle already is), and
   deployed by the existing image-pin bump. This closes three problems at once:
   the prod backend never needs a repo-write credential; content stops being
   untrusted runtime input rendered into every student's browser, becoming a
   reviewed artifact instead; and content can never be newer than the renderer
   that has to draw it, because they ship in the same commit and the same pin.

   The line: **content in git, operational knobs in the database.** Session
   duration and idle timeout are per-deployment settings an admin should change
   without a deploy (`lab_settings`, done in C). The lesson itself is not.
   Admin -> Lab Management is a small settings page, not an editor.

7. **Scoped topology is a different builder, not a filtered one.** Built solely
   from `NestedFromKubeconfig` - the student's own disposable k3s cluster. The
   outer cluster's data never enters the code path, so there is nothing to
   forget to strip. Filtering `adminTopology` would be one missed field away
   from leaking `Layers` (kernel/runtime/firewall), `Dashboards` (ArgoCD,
   Grafana, Vault paths), `Signals` (sync state, cert expiry), node names, or
   other users' session IDs.

8. **The escape edge is not drawn for students.** `escapeEdges()` renders
   `foothold -> production / get secret` - the answer to `ns-escape`. It
   stays admin-only.

9. **Progress is keyed by user + module, not by session.** Sessions are reaped
   at 60 minutes; course progress must outlive them.

10. **Completion = every quiz answered**, right or wrong. Flag submission is a
    later phase and is explicitly out of scope here.

## Content model

```
services/labs/<slug>/
  README.md              # unchanged - the full walkthrough, for maintainers
  course/
    00-intro.md
    01-namespaces.md
    02-gut-check.md      # a quiz step
    ...
```

Each file is one screen. Frontmatter carries the structured fields
(`ey`, `eyt`, `tag`/`chip`, `diagram`, `aha`, `revealLabel`, or the quiz's
`q`/`opts`/`explain`); the Markdown body is the prose.

Git is the only source of truth. The parser still round-trips
(`parse(render(step)) == step`, asserted in CI) - not to keep a database in
sync, but because a lossy parser silently drops fields, and a lesson that loses
its `aha` reveal on the way to the renderer fails quietly rather than loudly.

**Authoring loop:** the frontend already runs under `bun dev` with HMR, so an
author edits `services/labs/<slug>/course/03-rbac.md`, saves, and the lab pane
rerenders in the real renderer. That is a faster loop than an in-app editor, and
it shows the actual component rather than an editor's approximation of it.

**Quiz answers ship to the client.** Steps are served whole, so the correct
option is in the payload and a student can read it in devtools. That costs
nothing today because completion is "every quiz answered, right or wrong" (see
decision 10) - correctness gates nothing. It does foreclose *graded* assessment:
adding one means withholding answers and moving grading to a server endpoint.
Noted here so that trade is made deliberately rather than discovered.

The course endpoint is entitlement-gated (`GET /api/v1/modules/{slug}/course`,
auth + grant check) - a user must not be able to fetch the lesson for a lab they
were never granted.

## Phases & tasks

| Task | Phase | What |
|------|-------|------|
| A1 | content | `step.schema.json`; generate TS + Go types from it |
| A2 | content | Markdown+frontmatter ⇄ step-object parser/serializer + round-trip test |
| A3 | content | Diagram DSL → SVG (TS port of `box`/`wire`/`lbl`/`seal`/`svg`) |
| A4 | content | Lint: cold-term check (every term chip first appears earlier) + diagram overlap check |
| A5 | content | Author the `ns-escape` course (concept ladder + quizzes) |
| B1 | persistence | Migration: `lab_progress` (`lab_settings` landed with C) |
| B2 | serving | `GET /api/v1/modules/{slug}/course` - entitlement-gated, served from the embedded content |
| B3 | admin | Admin → **Lab Management** tab: per-lab duration/idle settings (no content editing) |
| C1 | timer | Per-lab duration/idle onto `Session` at Create; `expiresAt` in the session API |
| C2 | timer | `expiredSessions` uses per-session values instead of the reaper's globals |
| C3 | timer | Countdown UI + idle warning at T-3min |
| C4 | timer | Fix the stale "3min idle" comment in `routes.go` (it is 20min) |
| D1 | panes | Drag-resizable course pane |
| D2 | panes | Course player component (stepper, dots, `aha`, quiz flagging, keyboard) |
| D3 | panes | Server-side progress + completion gating |
| E1 | topology | Terminal / Topology tab chrome |
| E2 | topology | `GET /api/v1/sessions/{id}/topology` - ownership-gated, nested-only |
| E3 | topology | Split `TopologyMap` into presentational + admin/scoped containers |
| E4 | topology | Opaque boundary node for "outside your lab" |
| F1 | cleanup | Remove `suid-privesc-101` and the whole legacy pod path |
| G1 | verify | Leak test: a scoped payload contains no outer-cluster identifier |
| G2 | verify | Round-trip + lint wired into CI |
| G3 | verify | Completion-gating tests |

## Critical path

`A1 → A2 → A3 → A5` gates everything content-shaped. `D2` needs `A3`;
`B2` needs `A1/A2`. The other tracks are independent:

- `C1 → C2 → C3` (timer) - small, can land first as a standalone win.
- `E2 → E1/E3/E4` (topology) - independent of all content work.
- `F1` (cleanup) - independent, and it *shrinks* the surface every other task
  touches, so doing it early is cheapest.

Rough size: ~2-3 weeks focused. **A** is the heaviest and the least reversible;
**C** and **F** are the quick wins.

## Risks

1. **Content as an injection surface** is now bounded by PR review rather than
   by runtime sanitisation (decision 6), which is a much stronger guarantee. The
   restricted renderer and declarative diagrams (decisions 4 and 5) stay as
   defence in depth - they are no longer load-bearing, but they also cost
   nothing, and they are what makes the lints in A4 possible at all.
2. **The diagram DSL will be less expressive than freeform SVG.** `authoring.md`
   explicitly permits dropping to raw SVG when a helper doesn't fit; we are
   giving that up. If a lab genuinely needs a drawing the DSL can't express, the
   answer is to extend the DSL, not to open an HTML escape hatch.
3. **F1 leaves exactly one lab in the catalog** (`ns-escape`). Fine, but it
   means the product ships with a single module until more are authored.
4. **Per-lab timers interact with two clocks.** A countdown that ignores the
   idle timeout will lie to a student reaped at 20min idle with 38:00 showing.
   Handled in C: both deadlines are on the wire and the UI warns on idle only
   when idle will actually fire first.
5. **Content changes need a deploy.** A typo fix is a PR + CI + pin bump, not an
   edit box. Accepted deliberately: lesson text is not a hotfix path, and the
   properties bought in decision 6 are worth more than instant edits.
