package course

import (
	"fmt"
	"math"
	"strings"
)

// The lints.
//
// These exist because the training-guide skill's own authoring.md names exactly
// two failure modes as the most common - "cold terms" and "overlapping labels" -
// and says the only defence is opening the page in a browser and looking at it.
// Manual inspection does not survive a growing catalogue. Both become
// mechanical here, and that is the whole reason the diagram format is a
// declarative shape list rather than an author-supplied drawing function.

// Finding is one lint failure, addressed to the author.
type Finding struct {
	Module string
	Step   string
	Msg    string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s/%s: %s", f.Module, f.Step, f.Msg)
}

// Lint runs every check over one course.
func Lint(c Course) []Finding {
	var out []Finding
	out = append(out, lintColdTerms(c)...)
	for _, s := range c.Steps {
		if s.Diagram != nil {
			for _, msg := range lintDiagram(*s.Diagram) {
				out = append(out, Finding{Module: c.ModuleSlug, Step: s.Slug, Msg: msg})
			}
		}
	}
	return out
}

// lintColdTerms enforces the golden rule: every term a later screen leans on
// must have been introduced on an earlier one.
//
// A backticked term is the author saying "I am naming this now", so the first
// screen to use a term is its introduction and every later use is fine. What
// this catches is the reverse shape - a gotcha screen that opens with
// `RoleBinding` when the course never taught what a RoleBinding is.
//
// Necessarily a heuristic: it can prove a term was never introduced *as a
// term*, not that the surrounding prose actually explained it. It catches the
// mechanical failure, which is the one that actually happens.
func lintColdTerms(c Course) []Finding {
	var out []Finding
	introduced := map[string]bool{}

	for _, s := range c.Steps {
		// A quiz leans on vocabulary rather than teaching it, so a term
		// appearing for the first time in a question is exactly the cold term
		// this is looking for. A concept teaches; a flag step is the capstone
		// prose recapping the exploit in already-taught terms, so treat it like a
		// concept - it names things, it does not spring cold ones.
		teaches := s.Kind == KindConcept || s.Kind == KindFlag

		// Within one step, the title and body introduce; the aha/explain
		// reveal comes after and must not be where a term debuts - the reveal
		// is meant to carry the analogy, and a reader who skips it would then
		// meet the term cold on a later screen.
		for _, term := range Terms(s.Title + "\n" + s.Body + "\n" + s.Q) {
			if teaches {
				introduced[strings.ToLower(term)] = true
			} else if !introduced[strings.ToLower(term)] {
				out = append(out, Finding{c.ModuleSlug, s.Slug, fmt.Sprintf(
					"quiz uses the term %q, which no earlier concept screen introduced", term)})
			}
		}
		for _, src := range []string{s.Aha, s.Explain} {
			for _, term := range Terms(src) {
				if !introduced[strings.ToLower(term)] {
					out = append(out, Finding{c.ModuleSlug, s.Slug, fmt.Sprintf(
						"the reveal introduces the term %q for the first time - name it in the body instead, "+
							"so a reader who skips the reveal still meets it", term)})
					introduced[strings.ToLower(term)] = true
				}
			}
		}
	}
	return out
}

// The drawing canvas, from authoring.md. Content must stay inside the safe
// area; the bottom band is reserved for the caption.
const (
	canvasW      = 640.0
	canvasH      = 300.0
	safeMinX     = 20.0
	safeMaxX     = 620.0
	safeMinY     = 30.0
	safeMaxY     = 270.0
	captionBandY = 240.0
)

// connectTolerance is how far a wire endpoint may sit from a shape and still
// count as attached. Real wires land within ~15px of their target box's edge;
// a genuinely stranded arrow is 38px+ adrift. This threshold separates the two.
const connectTolerance = 24.0

// rect is a shape's approximate bounding box in canvas coordinates.
type rect struct {
	x0, y0, x1, y1 float64
	what           string
}

// wireEnds is one wire's two endpoints, kept so connectivity can be checked
// after every shape is seen - a wire often points at a box declared later in
// the shape list, so it cannot be resolved in a single forward pass.
type wireEnds struct {
	i        int
	from, to *Point
}

func (r rect) overlaps(o rect) bool {
	return r.x0 < o.x1 && o.x0 < r.x1 && r.y0 < o.y1 && o.y0 < r.y1
}

// distTo is the shortest distance from a point to this rect, or 0 if the point
// is inside it.
func (r rect) distTo(p Point) float64 {
	dx := math.Max(math.Max(r.x0-p.X, p.X-r.x1), 0)
	dy := math.Max(math.Max(r.y0-p.Y, p.Y-r.y1), 0)
	return math.Hypot(dx, dy)
}

// nearestRectDist is the distance from a point to the closest of the given
// rects (math.Inf if there are none - a diagram with a wire but no shape to
// attach it to, which the connectivity check then flags).
func nearestRectDist(p Point, rects []rect) float64 {
	best := math.Inf(1)
	for _, r := range rects {
		if d := r.distTo(p); d < best {
			best = d
		}
	}
	return best
}

func (r rect) area(o rect) float64 {
	w := math.Min(r.x1, o.x1) - math.Max(r.x0, o.x0)
	h := math.Min(r.y1, o.y1) - math.Max(r.y0, o.y0)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// lintDiagram checks a drawing for the things that make it unreadable: content
// outside the safe area, text colliding with other text or with a box, and a
// caption band that something has wandered into.
//
// Free labels are the hard case, because SVG text is not measured until it
// renders. The estimate below is intentionally conservative (a slightly wide
// box) - a false positive costs an author one nudge, a false negative ships a
// diagram with two labels printed on top of each other.
func lintDiagram(d Diagram) []string {
	var msgs []string
	var boxes, texts, connectable []rect
	var wires []wireEnds

	for i, sh := range d.Shapes {
		switch sh.Type {
		case "box":
			r := rect{sh.X, sh.Y, sh.X + sh.W, sh.Y + sh.H, fmt.Sprintf("box %q", sh.Label)}
			boxes = append(boxes, r)
			connectable = append(connectable, r)
			msgs = append(msgs, outOfBounds(r)...)
		case "frame":
			// A frame exists to contain other shapes, so it is deliberately NOT
			// added to `boxes` - every one of its contents would otherwise be
			// reported as overlapping it. Bounds still apply: a frame running
			// off the canvas clips just as badly as anything else. It IS a valid
			// wire target, though - a wire may run to a container's edge.
			fr := rect{sh.X, sh.Y, sh.X + sh.W, sh.Y + sh.H, fmt.Sprintf("frame %q", sh.Label)}
			connectable = append(connectable, fr)
			msgs = append(msgs, outOfBounds(fr)...)
		case "label":
			r := labelRect(sh)
			texts = append(texts, r)
			msgs = append(msgs, outOfBounds(r)...)
			if d.Caption != "" && r.y1 > captionBandY {
				msgs = append(msgs, fmt.Sprintf(
					"%s sits in the caption band (y>%.0f); move it up or drop the caption",
					r.what, captionBandY))
			}
		case "seal":
			r := sh.R
			if r == 0 {
				r = 26
			}
			b := rect{sh.CX - r, sh.CY - r, sh.CX + r, sh.CY + r, fmt.Sprintf("seal %d", i)}
			boxes = append(boxes, b)
			connectable = append(connectable, b)
			msgs = append(msgs, outOfBounds(b)...)
		case "wire":
			wires = append(wires, wireEnds{i, sh.From, sh.To})
			for _, p := range []*Point{sh.From, sh.To} {
				if p.X < safeMinX || p.X > safeMaxX || p.Y < safeMinY || p.Y > safeMaxY {
					msgs = append(msgs, fmt.Sprintf(
						"wire %d has an endpoint (%.0f,%.0f) outside the safe area x %.0f-%.0f, y %.0f-%.0f",
						i, p.X, p.Y, safeMinX, safeMaxX, safeMinY, safeMaxY))
				}
			}
		}
	}

	// A wire is a connector, so each end must actually land on something - a box,
	// a seal, or a frame edge. A wire whose endpoint floats in empty space is the
	// failure that shipped the pod-exec "secret" box stranded top-right with the
	// arrow pointing past it into the void (the reward box was raised to y=60 but
	// the connecting wire was left running flat at y~149). Bounds-checking the
	// endpoint did not catch it - the point was inside the canvas, just attached
	// to nothing. The tolerance sits well above a normal edge-connection gap
	// (real wires land within ~15px of their target) and well below a genuine
	// miss (the stranded arrows were 38px+ adrift), so it flags the disconnect
	// without nagging authors who leave a few pixels of breathing room.
	for _, w := range wires {
		for _, e := range []struct {
			name string
			p    *Point
		}{{"from", w.from}, {"to", w.to}} {
			if nearestRectDist(*e.p, connectable) > connectTolerance {
				msgs = append(msgs, fmt.Sprintf(
					"wire %d's %s endpoint (%.0f,%.0f) connects to nothing - no box, seal, or frame within %.0fpx; "+
						"point it at a shape or it renders as an arrow into empty space",
					w.i, e.name, e.p.X, e.p.Y, connectTolerance))
			}
		}
	}

	// Text on text is always wrong - two labels printed over each other are
	// simply unreadable.
	for i := range texts {
		for j := i + 1; j < len(texts); j++ {
			if texts[i].overlaps(texts[j]) {
				msgs = append(msgs, fmt.Sprintf("%s overlaps %s", texts[i].what, texts[j].what))
			}
		}
	}

	// Text on a box is only reported past a real overlap, not a 1px kiss: a
	// label deliberately tucked against a box edge is a normal annotation, and
	// flagging those would train authors to ignore this lint.
	const tolerance = 12.0
	for _, t := range texts {
		for _, b := range boxes {
			if t.area(b) > tolerance {
				msgs = append(msgs, fmt.Sprintf("%s overlaps %s", t.what, b.what))
			}
		}
	}

	// Boxes on boxes: a genuine overlap, not two boxes sharing an edge.
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			if boxes[i].area(boxes[j]) > tolerance {
				msgs = append(msgs, fmt.Sprintf("%s overlaps %s", boxes[i].what, boxes[j].what))
			}
		}
	}
	return msgs
}

// labelRect estimates a free label's bounding box. Mono at ~11px averages
// close to 6.2px per character; height is one line plus a little slack.
func labelRect(sh Shape) rect {
	const charW, lineH = 6.2, 15.0
	w := float64(len([]rune(sh.Text))) * charW
	var x0 float64
	switch sh.Anchor {
	case "start":
		x0 = sh.X
	case "end":
		x0 = sh.X - w
	default: // middle
		x0 = sh.X - w/2
	}
	return rect{x0, sh.Y - lineH*0.75, x0 + w, sh.Y + lineH*0.25, fmt.Sprintf("label %q", sh.Text)}
}

func outOfBounds(r rect) []string {
	if r.x0 >= safeMinX && r.x1 <= safeMaxX && r.y0 >= safeMinY && r.y1 <= safeMaxY {
		return nil
	}
	return []string{fmt.Sprintf(
		"%s extends outside the safe area (x %.0f-%.0f, y %.0f-%.0f); it will be clipped or crowd the edge",
		r.what, safeMinX, safeMaxX, safeMinY, safeMaxY)}
}
