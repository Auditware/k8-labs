package course

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

// The restricted prose renderer.
//
// Hand-written rather than a Markdown library on purpose: the allowlist IS the
// implementation. There is no configuration to get wrong, no plugin that might
// re-enable raw HTML in a future version, and nothing to audit beyond this
// file. Everything not listed below is escaped and rendered as literal text.
//
// What is supported, and nothing else:
//
//	`term`        -> a mono term chip (<span class="k">), the first-use marker
//	                 the cold-term lint reads
//	`$ cmd`       -> a RUNNABLE command chip (<span class="k cmd" data-cmd>): the
//	                 author opts a backtick span in as a shell command by leading
//	                 with "$ ", and the course player hangs copy/run affordances
//	                 off it (run pastes it into the lab terminal). The bare "cmd"
//	                 still counts as introduced vocabulary for the cold-term lint.
//	**bold**      -> <b>
//	*italic*      -> <i>
//	```yaml       -> a verbatim spec/code block (<pre class="spec"><code>): the
//	 ...             one multi-line construct, for showing a student what an
//	```             insecure (or fixed) manifest actually looks like rather than
//	                describing it in prose. The fence's language tag is ignored
//	                (kept only so the source reads naturally); the content is
//	                escaped verbatim, newlines preserved, and NO term/bold/italic
//	                substitution runs inside it - a spec is shown as-authored.
//	blank line    -> a paragraph break
//
// Deliberately absent: raw HTML, raw SVG, images, and links. Links are the
// notable omission - a lesson wanting to point at documentation should name it
// in prose, because a clickable URL in a security-training product is exactly
// the affordance we teach people to distrust, and it would also hand any future
// content path a redirect primitive.

var (
	termRe   = regexp.MustCompile("`([^`\n]+)`")
	boldRe   = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	italicRe = regexp.MustCompile(`\*([^*\n]+)\*`)
	// A fenced block: ```optional-lang optional-filename\n ...content... \n``` .
	// (?s) lets the content span newlines; the language tag is matched and
	// discarded (authors write ```yaml for their own readability). Anything after
	// the language on the fence line is captured as a FILENAME (submatch 1): a
	// fence tagged with one renders as an editor tab, and consecutive filenamed
	// fences group into a tabbed window (see RenderProse / renderSpecGroup).
	fenceRe = regexp.MustCompile("(?s)```[a-zA-Z0-9_+-]*[ \t]*([^\n]*)\n(.*?)\n?```")
	// Spec-block highlighting (see highlightSpec). All run on ALREADY-ESCAPED
	// text, so they can only ever wrap our own spans around entities - never
	// introduce markup from author input.
	specCommentRe = regexp.MustCompile(`(^|\s)(#.*)$`)                  // a YAML comment (# at line start or after space)
	specDangerRe  = regexp.MustCompile(`^#\s*!\s?(.*)$`)                // a comment flagged dangerous with "!"
	specGoodRe    = regexp.MustCompile(`^#\s*\+\s?(.*)$`)               // a comment flagged as the safe choice with "+"
	specKeyRe     = regexp.MustCompile(`^(\s*(?:-\s+)?)([\w.\-/]+)(:)`) // the leading mapping key
	specStrRe     = regexp.MustCompile(`(&#34;[^&<]*?&#34;)`)           // a quoted string value (after escaping, " is &#34;)
)

// specFileIcon is the little document glyph on each editor tab. currentColor so
// it inherits the tab's active/inactive colour. Hand-written in the sidebar's
// lucide style, kept here (not in content) so no author markup is ever emitted.
const specFileIcon = `<svg class="spec-file-icon" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v11a2 2 0 0 1-2 2z"/></svg>`

// cmdPrefix marks a backtick span as a runnable command (see the file header).
const cmdPrefix = "$ "

// termChips replaces every backtick span with its chip. Input is already
// HTML-escaped (escape-first), so the inner text is safe both as element
// content and, for a command, inside the double-quoted data-cmd attribute
// (EscapeString has already turned any " into &#34;). ReplaceAllStringFunc,
// not ReplaceAllString, so a command containing "$" can't be read as a
// replacement template.
func termChips(s string) string {
	return termRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1] // strip the surrounding backticks
		if cmd, ok := strings.CutPrefix(inner, cmdPrefix); ok {
			cmd = strings.TrimSpace(cmd)
			return `<span class="k cmd" data-cmd="` + cmd + `">` + cmd + `</span>`
		}
		return `<span class="k">` + inner + `</span>`
	})
}

// RenderProse converts an author's restricted Markdown to HTML.
//
// Escaping happens FIRST, before any markup is introduced, so author text can
// never inject an element - by the time the substitutions below run, every <,
// >, & and quote in the source is already an entity, and the only angle
// brackets in the output are the ones this function wrote.
func RenderProse(src string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}
	// Fenced blocks are lifted out first and rendered verbatim, because the
	// paragraph pipeline below would collapse their newlines and chip their
	// contents - a spec must survive as authored. Everything between fences goes
	// through the normal inline pipeline. Escape-first still holds within each
	// segment: prose and code are both escaped before any markup is introduced.
	var out []string
	last := 0
	matches := fenceRe.FindAllStringSubmatchIndex(src, -1)
	for i := 0; i < len(matches); {
		m := matches[i]
		out = append(out, renderParagraphs(src[last:m[0]])...)
		file := strings.TrimSpace(src[m[2]:m[3]]) // submatch 1: the fence's filename
		if file == "" {
			// A plain spec block - no filename, no tab chrome. This is every spec
			// authored before the editor treatment, rendered exactly as before but
			// with the per-line highlighting highlightSpec now emits.
			out = append(out, `<pre class="spec"><code>`+highlightSpec(src[m[4]:m[5]])+`</code></pre>`)
			last = m[1]
			i++
			continue
		}
		// A filenamed fence: gather it and any immediately-following filenamed
		// fences (only blank space between them) into one tabbed editor window.
		j := i
		for j+1 < len(matches) {
			gap := src[matches[j][1]:matches[j+1][0]]
			nextFile := strings.TrimSpace(src[matches[j+1][2]:matches[j+1][3]])
			if strings.TrimSpace(gap) != "" || nextFile == "" {
				break
			}
			j++
		}
		out = append(out, renderSpecGroup(src, matches[i:j+1]))
		last = matches[j][1]
		i = j + 1
	}
	out = append(out, renderParagraphs(src[last:])...)
	return strings.Join(out, "")
}

// renderSpecGroup turns a run of filenamed fences into a tabbed editor window: a
// title bar with one tab per file (each carrying a red/green dot for a file that
// holds a flaw / only safe markers) and one pane per file. The filename is
// escaped and the code runs through highlightSpec, so escape-first still holds.
func renderSpecGroup(src string, group [][]int) string {
	var tabs, panes strings.Builder
	for i, m := range group {
		file := strings.TrimSpace(src[m[2]:m[3]])
		code := src[m[4]:m[5]]
		name, ext := file, ""
		if dot := strings.LastIndexByte(file, '.'); dot > 0 {
			name, ext = file[:dot], file[dot:]
		}
		active := ""
		if i == 0 {
			active = " is-active"
		}
		var dot string
		switch specGroupMark(code) {
		case "bad":
			dot = `<span class="spec-tab-dot bad"></span>`
		case "good":
			dot = `<span class="spec-tab-dot good"></span>`
		}
		idx := strconv.Itoa(i)
		tabs.WriteString(`<button type="button" class="spec-tab` + active + `" data-spec-i="` + idx + `">` +
			specFileIcon + `<span class="spec-tab-name">` + html.EscapeString(name) + `</span>` +
			`<span class="spec-tab-ext">` + html.EscapeString(ext) + `</span>` + dot + `</button>`)
		panes.WriteString(`<div class="spec-pane` + active + `" data-spec-i="` + idx +
			`"><pre class="spec"><code>` + highlightSpec(code) + `</code></pre></div>`)
	}
	return `<div class="spec-group"><div class="spec-bar">` +
		`<span class="spec-dots"><i></i><i></i><i></i></span>` +
		`<div class="spec-tabs">` + tabs.String() + `</div></div>` +
		`<div class="spec-panes">` + panes.String() + `</div></div>`
}

// specGroupMark reports whether a file holds a flagged flaw ("bad"), only safe
// markers ("good"), or neither (""), for the coloured dot on its tab.
func specGroupMark(code string) string {
	good := false
	for _, line := range strings.Split(code, "\n") {
		m := specCommentRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if specDangerRe.MatchString(m[2]) {
			return "bad"
		}
		if specGoodRe.MatchString(m[2]) {
			good = true
		}
	}
	if good {
		return "good"
	}
	return ""
}

// highlightSpec renders a fenced spec/manifest block with lightweight, SAFE
// YAML highlighting - enough to make a misconfiguration legible in a review, not
// a real YAML parser. Escape-first still holds: every line is HTML-escaped
// before any span is introduced (see highlightSpecLine), so the only markup in
// the output is the fixed set of <span> tags this code writes around entities.
// No author byte can become markup.
//
// A comment beginning with "!" - e.g. `verbs: ["bind"]   # ! grants any role` -
// marks the dangerous line: its comment renders in the danger colour and the
// line's value is emphasised, so the misconfiguration is unmistakable. The "!"
// is dropped from the shown text. This is the author's way of pointing at the
// exact field to recognise in a real manifest, the whole point of showing a spec.
func highlightSpec(raw string) string {
	// Each line is its own block-level .spec-line span (the frontend draws the
	// line-number gutter off a CSS counter over these, and a flagged line gets its
	// row tinted). NB: no "\n" between rows - .spec-line is display:block, so a
	// newline here would double every line and drift it off the gutter.
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		b.WriteString(highlightSpecLine(line))
	}
	return b.String()
}

func highlightSpecLine(line string) string {
	// Escape FIRST. Everything below only ever wraps this escaped text in our own
	// spans, so no author input can turn into markup.
	esc := html.EscapeString(line)

	// Peel off a trailing (or whole-line) YAML comment. A '#' inside a numeric
	// entity like &#34; is preceded by '&', not whitespace, so it never matches.
	code, comment := esc, ""
	if m := specCommentRe.FindStringSubmatchIndex(esc); m != nil {
		code, comment = esc[:m[4]], esc[m[4]:] // m[4:6] is submatch 2, the "#..."
	}

	// "# !" marks the dangerous line (rose, value emphasised); "# +" marks the
	// safe choice (green) - the affirmative mirror. Either way the marker char is
	// dropped from the shown text. mark drives both the inline comment colour and
	// the row tint below.
	mark := ""
	if comment != "" {
		if dm := specDangerRe.FindStringSubmatch(comment); dm != nil {
			mark, comment = "danger", "# "+dm[1]
		} else if gm := specGoodRe.FindStringSubmatch(comment); gm != nil {
			mark, comment = "good", "# "+gm[1]
		}
	}

	code = specKeyRe.ReplaceAllString(code, `$1<span class="spec-key">$2</span>$3`)
	code = specStrRe.ReplaceAllString(code, `<span class="spec-str">$1</span>`)
	if mark == "danger" {
		code = `<span class="spec-hot">` + code + `</span>`
	}
	if comment != "" {
		cls := "spec-comment"
		switch mark {
		case "danger":
			cls = "spec-danger"
		case "good":
			cls = "spec-good"
		}
		comment = `<span class="` + cls + `">` + comment + `</span>`
	}

	row := "spec-line"
	switch mark {
	case "danger":
		row += " is-danger"
	case "good":
		row += " is-good"
	}
	return `<span class="` + row + `">` + code + comment + `</span>`
}

// renderParagraphs turns a run of prose (no fenced blocks) into <p> elements
// via the inline allowlist, one paragraph per blank line.
func renderParagraphs(src string) []string {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	escaped := html.EscapeString(src)

	// Order matters: bold before italic, or `**x**` is eaten as an italic `*`
	// wrapping `*x*`.
	var out []string
	for _, para := range strings.Split(escaped, "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// A single newline inside a paragraph is a soft wrap in the source, not
		// a line break - authors hard-wrap Markdown for readable diffs.
		para = strings.ReplaceAll(para, "\n", " ")
		para = termChips(para)
		para = boldRe.ReplaceAllString(para, `<b>$1</b>`)
		para = italicRe.ReplaceAllString(para, `<i>$1</i>`)
		out = append(out, "<p>"+para+"</p>")
	}
	return out
}

// Terms returns every backticked term in a step's prose, in order of first
// appearance. This is what the cold-term lint walks: the single most common way
// these lessons fail is a later screen using vocabulary the course never
// introduced, and a term chip is exactly the marker for "I am naming this now".
func Terms(src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range termRe.FindAllStringSubmatch(src, -1) {
		t := strings.TrimSpace(m[1])
		// A "$ cmd" span introduces the command "cmd" itself as vocabulary -
		// showing a student the command is introducing it - so strip the marker
		// and count the bare command, letting a later screen reference it.
		if after, ok := strings.CutPrefix(t, cmdPrefix); ok {
			t = strings.TrimSpace(after)
		}
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// Render fills in the HTML fields the client displays, leaving the Markdown
// source fields untouched so a rendered Step still round-trips.
func Render(s Step) Step {
	s.BodyHTML = RenderProse(s.Body)
	s.AhaHTML = RenderProse(s.Aha)
	s.ExplainHTML = RenderProse(s.Explain)
	// A question and its options are single lines, so they render inline - a
	// <p> wrapper here would put a paragraph break inside a button.
	s.QHTML = RenderInline(s.Q)
	if len(s.Opts) > 0 {
		opts := make([]QuizOption, len(s.Opts))
		for i, o := range s.Opts {
			o.THTML = RenderInline(o.T)
			opts[i] = o
		}
		s.Opts = opts
	}
	return s
}

// RenderInline is RenderProse without the paragraph wrapper, for the one-line
// strings (a question, an option) that live inside an element of their own.
// Same allowlist, same escape-first ordering.
func RenderInline(src string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}
	out := html.EscapeString(src)
	out = strings.ReplaceAll(out, "\n", " ")
	out = termChips(out)
	out = boldRe.ReplaceAllString(out, `<b>$1</b>`)
	out = italicRe.ReplaceAllString(out, `<i>$1</i>`)
	return out
}

// RenderCourse renders every step for serving.
func RenderCourse(c Course) Course {
	steps := make([]Step, len(c.Steps))
	for i, s := range c.Steps {
		steps[i] = Render(s)
	}
	return Course{ModuleSlug: c.ModuleSlug, Steps: steps}
}
