package course

import (
	"bytes"
	"fmt"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// A course file is YAML frontmatter between --- fences, then the prose body as
// Markdown:
//
//	---
//	kind: concept
//	ey: CONCEPT 1
//	eyt: Namespaces
//	title: A namespace is not a security boundary
//	aha: Like desks in an open-plan office...
//	---
//	A `namespace` groups objects for naming and quota...
//
// The prose is below the fence rather than a YAML field on purpose: a lesson is
// prose and should read like prose in the file, not sit quoted inside YAML
// where every colon needs escaping and diffs are unreadable.
const fence = "---"

// Parse reads one course file. filename supplies the ordering prefix and slug
// ("01-rbac.md" -> Index 1, Slug "rbac"), so step order is answerable by
// listing the directory instead of opening every file.
func Parse(filename string, data []byte) (Step, error) {
	idx, slug, err := parseFilename(filename)
	if err != nil {
		return Step{}, err
	}

	front, body, err := splitFrontmatter(data)
	if err != nil {
		return Step{}, fmt.Errorf("%s: %w", filename, err)
	}

	var step Step
	dec := yaml.NewDecoder(bytes.NewReader(front))
	// Unknown fields are an error, not a shrug: a typo'd `revealLable` would
	// otherwise silently produce a screen missing its reveal, and nothing would
	// ever report it.
	dec.KnownFields(true)
	if err := dec.Decode(&step); err != nil {
		return Step{}, fmt.Errorf("%s: frontmatter: %w", filename, err)
	}
	step.Index, step.Slug = idx, slug
	step.Body = strings.TrimSpace(string(body))

	if err := step.validate(); err != nil {
		return Step{}, fmt.Errorf("%s: %w", filename, err)
	}
	return step, nil
}

// Marshal writes a Step back to its file form. Exists so the parser can be
// round-trip tested: a lossy parser drops fields silently, and a lesson that
// loses its `aha` reveal on the way through fails quietly rather than loudly.
func Marshal(step Step) ([]byte, error) {
	front, err := yaml.Marshal(step)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(fence + "\n")
	buf.Write(front)
	buf.WriteString(fence + "\n")
	if step.Body != "" {
		buf.WriteString("\n" + step.Body + "\n")
	}
	return buf.Bytes(), nil
}

// parseFilename pulls the ordering prefix and slug out of "NN-name.md".
func parseFilename(filename string) (int, string, error) {
	base := strings.TrimSuffix(path.Base(filename), ".md")
	prefix, rest, ok := strings.Cut(base, "-")
	if !ok {
		return 0, "", fmt.Errorf("%s: expected NN-name.md", filename)
	}
	idx, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, "", fmt.Errorf("%s: %q is not a numeric order prefix", filename, prefix)
	}
	if rest == "" {
		return 0, "", fmt.Errorf("%s: missing name after the order prefix", filename)
	}
	return idx, rest, nil
}

// splitFrontmatter separates the fenced YAML head from the Markdown body.
func splitFrontmatter(data []byte) (front, body []byte, err error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, fence+"\n") {
		return nil, nil, fmt.Errorf("missing opening %q frontmatter fence", fence)
	}
	rest := text[len(fence)+1:]
	end := strings.Index(rest, "\n"+fence)
	if end < 0 {
		return nil, nil, fmt.Errorf("missing closing %q frontmatter fence", fence)
	}
	after := rest[end+len(fence)+1:]
	return []byte(rest[:end]), []byte(strings.TrimPrefix(after, "\n")), nil
}

// validate enforces the rules the JSON Schema states, so a malformed step is
// caught at load time (which fails the build and the test suite) rather than
// rendering as a blank screen for a student.
func (s Step) validate() error {
	if s.Ey == "" || s.Eyt == "" {
		return fmt.Errorf("ey and eyt are required")
	}
	// tag and chip compete for the same top-right slot. The template silently
	// picks chip; refusing is better, because an author who wrote both meant
	// something and should be told which one is being dropped.
	if s.Tag != "" && s.Chip != nil {
		return fmt.Errorf("set either tag or chip, not both - they occupy the same slot")
	}
	if s.Chip != nil {
		switch s.Chip.Tone {
		case "accent", "warn", "danger":
		default:
			return fmt.Errorf("chip.tone %q must be accent, warn or danger", s.Chip.Tone)
		}
	}

	switch s.Kind {
	case KindConcept:
		if s.Title == "" || s.Body == "" || s.Aha == "" {
			return fmt.Errorf("concept steps need title, body and aha")
		}
		if s.Q != "" || len(s.Opts) > 0 || s.Explain != "" {
			return fmt.Errorf("concept step carries quiz fields (q/opts/explain)")
		}
	case KindQuiz:
		if s.Q == "" || s.Explain == "" {
			return fmt.Errorf("quiz steps need q and explain")
		}
		if len(s.Opts) < 2 || len(s.Opts) > 4 {
			return fmt.Errorf("quiz needs 2-4 options, got %d", len(s.Opts))
		}
		correct := 0
		for _, o := range s.Opts {
			if o.Correct {
				correct++
			}
		}
		// Zero correct answers makes the quiz unanswerable-correctly; more than
		// one makes the "right answer" arbitrary. Both are silent in the UI.
		if correct != 1 {
			return fmt.Errorf("quiz needs exactly one correct option, got %d", correct)
		}
		if s.Title != "" || s.Aha != "" {
			return fmt.Errorf("quiz step carries concept fields (title/aha)")
		}
	case KindFlag:
		// A flag step is the capstone: title + body prompt the student to paste
		// what they captured, and the form itself is implied by the kind. aha is
		// optional (a one-line "why this proves it" reveal), diagram optional. The
		// expected flag is never authored here - it lives in labs.Module.Flag - so
		// there is deliberately no answer field to carry.
		if s.Title == "" || s.Body == "" {
			return fmt.Errorf("flag steps need title and body")
		}
		if s.Q != "" || len(s.Opts) > 0 || s.Explain != "" {
			return fmt.Errorf("flag step carries quiz fields (q/opts/explain)")
		}
	default:
		return fmt.Errorf("kind %q must be concept, quiz or flag", s.Kind)
	}

	if s.Diagram != nil {
		if err := s.Diagram.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (d Diagram) validate() error {
	if len(d.Shapes) == 0 {
		return fmt.Errorf("diagram has no shapes")
	}
	for i, sh := range d.Shapes {
		switch sh.Type {
		case "box":
			if sh.Label == "" {
				return fmt.Errorf("shape %d: box needs a label", i)
			}
			if sh.W <= 0 || sh.H <= 0 {
				return fmt.Errorf("shape %d: box needs w and h", i)
			}
		case "frame":
			if sh.W <= 0 || sh.H <= 0 {
				return fmt.Errorf("shape %d: frame needs w and h", i)
			}
		case "wire":
			if sh.From == nil || sh.To == nil {
				return fmt.Errorf("shape %d: wire needs from and to", i)
			}
		case "label":
			if sh.Text == "" {
				return fmt.Errorf("shape %d: label needs text", i)
			}
		case "seal":
			// cx/cy default to 0,0 which is a legal but almost certainly
			// unintended corner; the overlap lint catches it as out-of-band.
		default:
			return fmt.Errorf("shape %d: unknown type %q", i, sh.Type)
		}
		switch sh.Tone {
		case "", "neutral", "accent", "warn", "danger":
		default:
			return fmt.Errorf("shape %d: tone %q must be neutral, accent, warn or danger", i, sh.Tone)
		}
	}
	return nil
}
