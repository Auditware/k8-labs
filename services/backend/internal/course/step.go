// Package course owns a lab's guided lesson: the on-disk Markdown format, its
// parser, and the restricted prose renderer that turns an author's Markdown
// into the HTML the lab pane displays.
//
// Content lives in git only - authored locally, reviewed in a PR, embedded in
// this binary, deployed by the usual image-pin bump. There is no admin editor
// and no content table. See docs/architecture/lab-session-rewrite.md, decision 6,
// for why: it keeps a repo-write credential out of the prod backend, makes
// content a reviewed artifact rather than untrusted runtime input, and stops
// content ever being newer than the renderer that has to draw it.
//
// The shapes here mirror services/labs/schema/step.schema.json, which is the
// authority CI validates every course file against.
package course

// Kind is which screen shape a step is. concept and quiz are the training-guide
// template's two; flag is the one deliberate extension - the capstone screen
// that takes the CTF flag the student captured in the live lab and is what marks
// the lab solved. A fourth kind would again be an engine change, not content.
type Kind string

const (
	KindConcept Kind = "concept"
	KindQuiz    Kind = "quiz"
	// KindFlag is the final "submit what you captured" screen. It renders like a
	// concept (title/body, optional aha/diagram) plus a submission form. The
	// expected flag is NEVER in the course content - it lives server-side in the
	// labs module (labs.Module.Flag) and is checked by the flag endpoint, so a
	// student reading the served course can't read the answer out of it. See
	// internal/api/progress.go and docs/architecture/lab-session-rewrite.md.
	KindFlag Kind = "flag"
)

// Step is one screen of a course.
//
// Body/Aha/Explain hold the author's raw Markdown. The HTML the browser
// receives is produced separately by the restricted renderer (prose.go), so
// parsing stays lossless and round-trippable - see Marshal/Parse.
type Step struct {
	// Slug is the file's basename without the ordering prefix ("01-rbac.md" ->
	// "rbac"). Stable across reordering, so progress records and deep links
	// don't break when a step is inserted in the middle.
	Slug string `yaml:"-" json:"slug"`
	// Index is the step's position, taken from the filename prefix rather than
	// a field, so "what order do these go in" is answerable by listing the
	// directory instead of opening every file.
	Index int `yaml:"-" json:"index"`

	Kind Kind   `yaml:"kind" json:"kind"`
	Ey   string `yaml:"ey" json:"ey"`
	Eyt  string `yaml:"eyt" json:"eyt"`
	// Tag and Chip compete for the same top-right slot. Both set is rejected at
	// parse time rather than silently resolved - the template picks chip, but
	// an author who wrote both meant something and should be told which won.
	Tag     string   `yaml:"tag,omitempty" json:"tag,omitempty"`
	Chip    *Chip    `yaml:"chip,omitempty" json:"chip,omitempty"`
	Diagram *Diagram `yaml:"diagram,omitempty" json:"diagram,omitempty"`

	// --- concept steps ---
	Title string `yaml:"title,omitempty" json:"title,omitempty"`
	// Body is the prose, and it is the Markdown *below* the frontmatter rather
	// than a frontmatter field - a lesson is prose and should read like prose in
	// the file, not sit quoted inside YAML.
	Body string `yaml:"-" json:"-"`
	// BodyHTML is the rendered form the client actually displays. Populated by
	// Render, empty on a freshly parsed Step.
	BodyHTML string `yaml:"-" json:"body,omitempty"`
	// Aha is the reveal: the analogy or the "so what", never more mechanics.
	Aha         string `yaml:"aha,omitempty" json:"-"`
	AhaHTML     string `yaml:"-" json:"aha,omitempty"`
	RevealLabel string `yaml:"revealLabel,omitempty" json:"revealLabel,omitempty"`

	// --- quiz steps ---
	// Q carries the author's Markdown like Body does, and renders the same way:
	// a question naming a term the course taught should show it as the same chip
	// the concept screens used, not as literal backticks. The cold-term lint
	// already reads Q for terms, so the two must agree.
	Q     string       `yaml:"q,omitempty" json:"-"`
	QHTML string       `yaml:"-" json:"q,omitempty"`
	Opts  []QuizOption `yaml:"opts,omitempty" json:"opts,omitempty"`
	// Explain is where the real teaching happens - why the intuitive answer
	// fails and the right one holds.
	Explain     string `yaml:"explain,omitempty" json:"-"`
	ExplainHTML string `yaml:"-" json:"explain,omitempty"`
}

// Chip is the labelled pill in the top-right slot.
type Chip struct {
	Label string `yaml:"label" json:"label"`
	// Tone is semantic, never a colour: accent = a key idea, warn = caution,
	// danger = a pitfall.
	Tone string `yaml:"tone" json:"tone"`
}

// QuizOption is one answer. Exactly one option in a quiz must be correct.
//
// Correct ships to the client: steps are served whole, so a student can read
// the answer in devtools. That costs nothing while completion is "every quiz
// answered, right or wrong" - correctness gates nothing. Adding graded
// assessment later means withholding this field and moving grading to a server
// endpoint. See docs/architecture/lab-session-rewrite.md.
type QuizOption struct {
	// T is the author's Markdown; THTML is what the client renders, so an
	// option naming a term shows the same chip the lesson used.
	T       string `yaml:"t" json:"-"`
	THTML   string `yaml:"-" json:"t"`
	Correct bool   `yaml:"correct,omitempty" json:"correct,omitempty"`
}

// Diagram is a declarative drawing, compiled to SVG on the client by the same
// box/wire/label/seal primitives the training-guide template uses.
//
// Declarative rather than an author-supplied function or raw SVG, for a reason
// beyond safety: a structured shape list can be checked for overlapping labels
// by a linter, which authoring.md otherwise says can only be caught by opening
// the page and looking at it.
type Diagram struct {
	// Caption renders in the reserved y 240-270 band, so it can never collide
	// with the shapes above it.
	Caption string  `yaml:"caption,omitempty" json:"caption,omitempty"`
	Shapes  []Shape `yaml:"shapes" json:"shapes"`
}

// Shape is one drawing primitive. A single struct with a Type discriminator
// rather than an interface: it keeps the YAML flat and readable for authors,
// and it is what the schema describes.
type Shape struct {
	Type string `yaml:"type" json:"type"` // box | wire | label | seal

	// box
	X     float64 `yaml:"x,omitempty" json:"x,omitempty"`
	Y     float64 `yaml:"y,omitempty" json:"y,omitempty"`
	W     float64 `yaml:"w,omitempty" json:"w,omitempty"`
	H     float64 `yaml:"h,omitempty" json:"h,omitempty"`
	Label string  `yaml:"label,omitempty" json:"label,omitempty"`
	Sub   string  `yaml:"sub,omitempty" json:"sub,omitempty"`

	// wire
	From   *Point `yaml:"from,omitempty" json:"from,omitempty"`
	To     *Point `yaml:"to,omitempty" json:"to,omitempty"`
	Dashed bool   `yaml:"dashed,omitempty" json:"dashed,omitempty"`
	// Arrow defaults to true, so it is a pointer: a plain bool couldn't tell
	// "author wrote arrow: false" from "author didn't mention arrows".
	Arrow *bool `yaml:"arrow,omitempty" json:"arrow,omitempty"`

	// label
	Text   string `yaml:"text,omitempty" json:"text,omitempty"`
	Anchor string `yaml:"anchor,omitempty" json:"anchor,omitempty"`

	// seal
	CX     float64 `yaml:"cx,omitempty" json:"cx,omitempty"`
	CY     float64 `yaml:"cy,omitempty" json:"cy,omitempty"`
	R      float64 `yaml:"r,omitempty" json:"r,omitempty"`
	Broken bool    `yaml:"broken,omitempty" json:"broken,omitempty"`

	Tone  string  `yaml:"tone,omitempty" json:"tone,omitempty"`
	Delay float64 `yaml:"delay,omitempty" json:"delay,omitempty"`
}

type Point struct {
	X float64 `yaml:"x" json:"x"`
	Y float64 `yaml:"y" json:"y"`
}

// Course is one lab's ordered lesson.
type Course struct {
	ModuleSlug string `json:"moduleSlug"`
	Steps      []Step `json:"steps"`
}

// QuizCount is how many steps a student must answer to complete the course.
// Completion is "every quiz answered, right or wrong" - it measures attendance,
// not mastery, which is the deliberate choice recorded in the rewrite plan.
func (c Course) QuizCount() int {
	n := 0
	for _, s := range c.Steps {
		if s.Kind == KindQuiz {
			n++
		}
	}
	return n
}
