package course

import (
	"strings"
	"testing"
)

// The shipped content must load, and it must pass its own lints. This is the
// test that actually protects students: content is compiled into the binary, so
// a break here would otherwise surface as a blank or wrong lesson pane in prod.
func TestEmbeddedCoursesLoadAndLint(t *testing.T) {
	slugs := Slugs()
	if len(slugs) == 0 {
		t.Fatal("no courses embedded")
	}
	for _, slug := range slugs {
		c, ok := Get(slug)
		if !ok {
			t.Fatalf("%s listed but not gettable", slug)
		}
		if len(c.Steps) == 0 {
			t.Errorf("%s: no steps", slug)
		}
		if c.QuizCount() == 0 {
			// Quizzes are formative, not the completion gate, but a lab course
			// with no self-checks at all is almost certainly a mistake.
			t.Errorf("%s: no quiz steps - a course should carry at least one self-check", slug)
		}
		// The lab is solved by capturing the flag, and the flag form is the
		// capstone screen. Every course must end on one, or there is no way to
		// mark the lab complete from the UI.
		if last := c.Steps[len(c.Steps)-1]; last.Kind != KindFlag {
			t.Errorf("%s: last step is %q (%s), not a flag capstone - the lab could never be completed",
				slug, last.Slug, last.Kind)
		}
		// And exactly one flag step: a course with two capstones is an authoring
		// slip, and the completion model assumes a single one.
		flags := 0
		for _, s := range c.Steps {
			if s.Kind == KindFlag {
				flags++
			}
		}
		if flags != 1 {
			t.Errorf("%s: expected exactly one flag step, got %d", slug, flags)
		}
		for _, f := range Lint(c) {
			t.Errorf("lint: %s", f)
		}
	}
}

// A lossy parser drops fields silently. Round-tripping every shipped step is
// how we find out before an author does.
func TestShippedStepsRoundTrip(t *testing.T) {
	for _, slug := range Slugs() {
		c, _ := Get(slug)
		for _, want := range c.Steps {
			data, err := Marshal(want)
			if err != nil {
				t.Fatalf("%s/%s: marshal: %v", slug, want.Slug, err)
			}
			filename := "00-" + want.Slug + ".md"
			got, err := Parse(filename, data)
			if err != nil {
				t.Fatalf("%s/%s: reparse: %v", slug, want.Slug, err)
			}
			// Index comes from the filename, which the synthetic name above
			// changes; everything else must survive untouched.
			got.Index = want.Index
			if got.Title != want.Title || got.Body != want.Body || got.Aha != want.Aha {
				t.Errorf("%s/%s: prose fields lost in round trip", slug, want.Slug)
			}
			if got.Ey != want.Ey || got.Eyt != want.Eyt || got.RevealLabel != want.RevealLabel {
				t.Errorf("%s/%s: eyebrow fields lost in round trip", slug, want.Slug)
			}
			if (got.Chip == nil) != (want.Chip == nil) {
				t.Errorf("%s/%s: chip lost in round trip", slug, want.Slug)
			}
			if (got.Diagram == nil) != (want.Diagram == nil) {
				t.Errorf("%s/%s: diagram lost in round trip", slug, want.Slug)
			}
			if got.Diagram != nil && len(got.Diagram.Shapes) != len(want.Diagram.Shapes) {
				t.Errorf("%s/%s: %d shapes became %d", slug, want.Slug,
					len(want.Diagram.Shapes), len(got.Diagram.Shapes))
			}
			if len(got.Opts) != len(want.Opts) {
				t.Errorf("%s/%s: %d options became %d", slug, want.Slug, len(want.Opts), len(got.Opts))
			}
		}
	}
}

func mustParse(t *testing.T, name, src string) Step {
	t.Helper()
	s, err := Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return s
}

const conceptSrc = `---
kind: concept
ey: CONCEPT 1
eyt: Thing
title: A claim
aha: An analogy
---
Body with a ` + "`term`" + ` in it.
`

func TestParseRejectsBrokenSteps(t *testing.T) {
	cases := []struct {
		name, src, wantErr string
	}{
		{
			name: "unknown field",
			// A typo'd key would otherwise silently produce a screen missing
			// the field, with nothing ever reporting it.
			src:     strings.Replace(conceptSrc, "aha: An analogy", "revealLable: oops\naha: An analogy", 1),
			wantErr: "field revealLable not found",
		},
		{
			name:    "tag and chip together",
			src:     strings.Replace(conceptSrc, "eyt: Thing", "eyt: Thing\ntag: x\nchip:\n  label: y\n  tone: accent", 1),
			wantErr: "not both",
		},
		{
			name:    "concept missing its reveal",
			src:     strings.Replace(conceptSrc, "aha: An analogy\n", "", 1),
			wantErr: "concept steps need title, body and aha",
		},
		{
			name: "quiz with no correct option",
			src: `---
kind: quiz
ey: GUT-CHECK
eyt: Check
q: A question?
opts:
  - t: one
  - t: two
explain: because
---
`,
			wantErr: "exactly one correct option, got 0",
		},
		{
			name: "quiz with two correct options",
			src: `---
kind: quiz
ey: GUT-CHECK
eyt: Check
q: A question?
opts:
  - t: one
    correct: true
  - t: two
    correct: true
explain: because
---
`,
			wantErr: "exactly one correct option, got 2",
		},
		{
			name:    "filename without an order prefix",
			src:     conceptSrc,
			wantErr: "expected NN-name.md",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := "01-x.md"
			if tc.name == "filename without an order prefix" {
				name = "intro.md"
			}
			_, err := Parse(name, []byte(tc.src))
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// The prose renderer is the security boundary for author text. Escaping must
// happen before any markup is introduced, so nothing an author writes can
// become an element.
func TestRenderProseEscapesBeforeMarkup(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"script tag is inert", `<script>alert(1)</script>`,
			`<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>`},
		{"img onerror is inert", `<img src=x onerror=alert(1)>`,
			`<p>&lt;img src=x onerror=alert(1)&gt;</p>`},
		{"term chip", "a `namespace` here", `<p>a <span class="k">namespace</span> here</p>`},
		{"bold before italic", `**strong** and *soft*`, `<p><b>strong</b> and <i>soft</i></p>`},
		{"paragraphs split on blank lines", "one\n\ntwo", `<p>one</p><p>two</p>`},
		{"soft wrap joins", "one\ntwo", `<p>one two</p>`},
		// The nastiest case: markup smuggled INSIDE a term chip. The term
		// contents are already escaped by the time the chip is built, so the
		// span can only ever contain text.
		{"markup inside a term", "a `<b>x</b>` term",
			`<p>a <span class="k">&lt;b&gt;x&lt;/b&gt;</span> term</p>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RenderProse(tc.in); got != tc.want {
				t.Errorf("RenderProse(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSpecBlocks covers the editor-style spec block: highlighting, the flaw/fix
// markers, multi-file tab grouping, and (above all) that escape-first still
// holds so no author byte ever becomes markup.
func TestSpecBlocks(t *testing.T) {
	// A plain (no-filename) fence is a single <pre class="spec">, one .spec-line
	// per source line, with the leading key and quoted strings tinted.
	plain := RenderProse("```yaml\nkind: Role\nname: \"admin\"\n```")
	for _, want := range []string{
		`<pre class="spec"><code>`,
		`<span class="spec-line"><span class="spec-key">kind</span>: Role</span>`,
		`<span class="spec-str">&#34;admin&#34;</span>`,
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain spec missing %q\n in %q", want, plain)
		}
	}
	if strings.Contains(plain, "spec-group") {
		t.Errorf("a no-filename fence must not get tab chrome:\n%s", plain)
	}

	// "# !" tints the row danger and drops the bang; "# +" tints it good.
	danger := RenderProse("```yaml\npath: / # ! reaches the node\n```")
	if !strings.Contains(danger, `<span class="spec-line is-danger">`) ||
		!strings.Contains(danger, `<span class="spec-danger"># reaches the node</span>`) ||
		strings.Contains(danger, "# !") {
		t.Errorf("danger marker wrong:\n%s", danger)
	}
	good := RenderProse("```yaml\nreadOnly: true # + least privilege\n```")
	if !strings.Contains(good, `<span class="spec-line is-good">`) ||
		!strings.Contains(good, `<span class="spec-good"># least privilege</span>`) ||
		strings.Contains(good, "# +") {
		t.Errorf("good marker wrong:\n%s", good)
	}

	// Escape-first: markup smuggled into code OR a danger comment stays inert.
	evil := RenderProse("```yaml\nx: 1 # ! <script>alert(1)</script>\n```")
	if strings.Contains(evil, "<script>") || !strings.Contains(evil, "&lt;script&gt;") {
		t.Errorf("smuggled markup was not escaped:\n%s", evil)
	}

	// Two adjacent filenamed fences group into ONE tabbed window: a .spec-group
	// with a tab per file (names, dots) and a pane per file.
	grp := RenderProse("```yaml role-flaw.yaml\nverbs: [\"bind\"] # ! grants any role\n```\n\n```yaml role-fixed.yaml\nverbs: [\"get\"] # + read only\n```")
	for _, want := range []string{
		`<div class="spec-group">`,
		`<span class="spec-tab-name">role-flaw</span>`,
		`<span class="spec-tab-ext">.yaml</span>`,
		`<span class="spec-tab-dot bad">`,  // the flaw file
		`<span class="spec-tab-dot good">`, // the fixed file
	} {
		if !strings.Contains(grp, want) {
			t.Errorf("spec group missing %q\n in %q", want, grp)
		}
	}
	if c := strings.Count(grp, `<pre class="spec">`); c != 2 {
		t.Errorf("expected 2 panes, got %d:\n%s", c, grp)
	}

	// A single filenamed fence is still a group (one tab) - a lone flaw with no fix.
	one := RenderProse("```yaml pod.yaml\nhostPath:\n  path: / # ! the node root\n```")
	if !strings.Contains(one, `<div class="spec-group">`) || strings.Count(one, `<pre class="spec">`) != 1 {
		t.Errorf("single filenamed fence should be a one-tab group:\n%s", one)
	}
}

func TestTermsInOrderOfFirstUse(t *testing.T) {
	got := Terms("a `Role` and a `RoleBinding`, then `Role` again")
	want := []string{"Role", "RoleBinding"}
	if len(got) != len(want) {
		t.Fatalf("Terms() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Terms() = %v, want %v", got, want)
		}
	}
}

// The cold-term lint is the whole reason the schema exists. These are the two
// shapes it must catch.
func TestLintColdTerms(t *testing.T) {
	quizWithColdTerm := Course{ModuleSlug: "m", Steps: []Step{
		mustParse(t, "00-intro.md", conceptSrc),
		mustParse(t, "01-q.md", `---
kind: quiz
ey: GUT-CHECK
eyt: Check
q: What does a `+"`ClusterRoleBinding`"+` do?
opts:
  - t: one
    correct: true
  - t: two
explain: because
---
`),
	}}
	findings := Lint(quizWithColdTerm)
	if len(findings) == 0 {
		t.Fatal("quiz using an unintroduced term produced no finding")
	}
	if !strings.Contains(findings[0].Msg, "ClusterRoleBinding") {
		t.Errorf("finding = %q, want it to name the cold term", findings[0].Msg)
	}

	// A term first named in the reveal is also cold: a reader who skips the
	// reveal meets it unexplained on a later screen.
	revealIntroduces := Course{ModuleSlug: "m", Steps: []Step{
		mustParse(t, "00-intro.md", `---
kind: concept
ey: CONCEPT 1
eyt: Thing
title: A claim
aha: It works like a `+"`RoleBinding`"+`
---
Body with no terms.
`),
	}}
	if got := Lint(revealIntroduces); len(got) == 0 {
		t.Error("term introduced in the reveal produced no finding")
	}

	// The healthy shape must stay silent, or the lint gets ignored.
	if got := Lint(Course{ModuleSlug: "m", Steps: []Step{mustParse(t, "00-intro.md", conceptSrc)}}); len(got) != 0 {
		t.Errorf("well-formed course produced findings: %v", got)
	}
}

// The overlap lint replaces "open it in a browser and look", which is what
// authoring.md otherwise prescribes.
func TestLintDiagramGeometry(t *testing.T) {
	cases := []struct {
		name    string
		diagram Diagram
		want    string
	}{
		{
			name: "two labels printed on top of each other",
			diagram: Diagram{Shapes: []Shape{
				{Type: "label", X: 300, Y: 150, Text: "first label"},
				{Type: "label", X: 305, Y: 152, Text: "second label"},
			}},
			want: "overlaps",
		},
		{
			name: "a label buried in a box",
			diagram: Diagram{Shapes: []Shape{
				{Type: "box", X: 100, Y: 100, W: 200, H: 80, Label: "thing"},
				{Type: "label", X: 200, Y: 140, Text: "on top of it"},
			}},
			want: "overlaps",
		},
		{
			name: "content past the safe area",
			diagram: Diagram{Shapes: []Shape{
				{Type: "box", X: 500, Y: 100, W: 200, H: 80, Label: "too wide"},
			}},
			want: "outside the safe area",
		},
		{
			name: "a label in the reserved caption band",
			diagram: Diagram{Caption: "a caption", Shapes: []Shape{
				{Type: "label", X: 320, Y: 258, Text: "collides with caption"},
			}},
			want: "caption band",
		},
		{
			name: "wire endpoint off canvas",
			diagram: Diagram{Shapes: []Shape{
				{Type: "wire", From: &Point{X: 40, Y: 150}, To: &Point{X: 630, Y: 150}},
			}},
			want: "outside the safe area",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgs := lintDiagram(tc.diagram)
			if len(msgs) == 0 {
				t.Fatalf("expected a finding containing %q, got none", tc.want)
			}
			joined := strings.Join(msgs, "; ")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("findings = %q, want one containing %q", joined, tc.want)
			}
		})
	}

	// A label tucked against a box edge is a normal annotation. Flagging those
	// would train authors to ignore the lint.
	ok := Diagram{Shapes: []Shape{
		{Type: "box", X: 100, Y: 100, W: 200, H: 60, Label: "thing"},
		{Type: "label", X: 200, Y: 180, Text: "below it"},
	}}
	if msgs := lintDiagram(ok); len(msgs) != 0 {
		t.Errorf("clean diagram produced findings: %v", msgs)
	}
}

// Quiz questions and options render through the same restricted renderer as
// the prose. Found by looking at the real course in a browser: they were
// plain-text-only at first, so a question naming a term the lesson had taught
// showed literal backticks while every concept screen showed a chip.
func TestQuizTextRendersTerms(t *testing.T) {
	step := mustParse(t, "01-q.md", `---
kind: quiz
ey: GUT-CHECK
eyt: Check
q: What does a `+"`ClusterRole`"+` do?
opts:
  - t: binds via a `+"`RoleBinding`"+`
    correct: true
  - t: nothing
explain: because a `+"`Role`"+` is just a list
---
`)
	got := Render(step)

	if !strings.Contains(got.QHTML, `<span class="k">ClusterRole</span>`) {
		t.Errorf("question did not render its term chip: %q", got.QHTML)
	}
	if !strings.Contains(got.Opts[0].THTML, `<span class="k">RoleBinding</span>`) {
		t.Errorf("option did not render its term chip: %q", got.Opts[0].THTML)
	}
	if !strings.Contains(got.ExplainHTML, `<span class="k">Role</span>`) {
		t.Errorf("explanation did not render its term chip: %q", got.ExplainHTML)
	}
	// Inline rendering must not wrap in <p> - a paragraph inside a button.
	if strings.Contains(got.QHTML, "<p>") || strings.Contains(got.Opts[0].THTML, "<p>") {
		t.Error("inline text was wrapped in a paragraph")
	}
	// And the escaping guarantee has to hold on this path too, not just prose.
	esc := RenderInline(`<script>alert(1)</script>`)
	if strings.Contains(esc, "<script") {
		t.Errorf("RenderInline failed to escape: %q", esc)
	}
}

// The catalog is meant to grow: authoring a lab is adding content, not changing
// the engine. This locks that in - more than one course loads, and each is
// independently valid - so a regression that only ever exercised a single
// course (an index bug, a shared-state bug in the loader) fails here.
func TestMultipleCoursesLoadIndependently(t *testing.T) {
	slugs := Slugs()
	if len(slugs) < 2 {
		t.Fatalf("expected the catalog to carry more than one course, got %d (%v) - "+
			"the second lab is the proof that the loader handles more than a single course", len(slugs), slugs)
	}
	for _, slug := range slugs {
		c, ok := Get(slug)
		if !ok || len(c.Steps) == 0 {
			t.Errorf("%s did not load independently", slug)
		}
		// Each course must start at index 0 and be contiguous - a shared counter
		// or off-by-one in the loader would show up as one course's steps
		// bleeding into another's ordering.
		for i, s := range c.Steps {
			if s.Index != i {
				t.Errorf("%s step %d has Index %d - ordering is not per-course", slug, i, s.Index)
			}
		}
	}
}
