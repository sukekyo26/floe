package diagram

import (
	"math"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Problems are the layout mistakes of a diagram, all of them at once, each at the line and column of
// the part it is about.
type Problems []*Error

// Error words the mistakes in English, one per line.
func (p Problems) Error() string {
	return p.Localize(i18n.EnglishCatalog())
}

// Localize words the mistakes in the language of c, one per line.
func (p Problems) Localize(c *i18n.Catalog) string {
	lines := make([]string, len(p))
	for i, e := range p {
		lines[i] = e.Localize(c)
	}
	return strings.Join(lines, "\n")
}

// Check reports the layout mistakes of s, as diagram.mjs does: text too wide or tall for its box,
// parts sticking out of the canvas, overlapping, straddling the edge of a frame, edges running
// through boxes, labels or frame headings, and edges crossing.
func Check(s *Spec) error {
	if p := newLayout(s).check(); len(p) > 0 {
		return p
	}
	return nil
}

func (l *layout) check() Problems {
	l.checkPlacement(l.spec)
	l.checkEdges()
	l.checkCrossings()
	return l.problems
}

// checkPlacement reports parts sticking out of the canvas, overlapping and straddling frames.
func (l *layout) checkPlacement(s *Spec) {
	canvas := rect{0, 0, s.Width, s.Height, nil, Pos{Line: 0, Column: 0}}
	for _, r := range concat(l.boxes, l.marks, l.labels, l.frames, l.areas) {
		if !r.inside(canvas) {
			l.problem(r.at, "diagram_outside", r.what, formatNum(s.Width), formatNum(s.Height))
		}
	}
	solids := concat(l.boxes, l.marks, l.labels)
	for i, a := range solids {
		for _, b := range solids[i+1:] {
			if a.overlaps(b) {
				l.problem(a.at, "diagram_overlap", a.what, b.what, b.at.Line)
			}
		}
	}
	for _, b := range concat(l.boxes, l.marks) {
		for _, f := range l.frames {
			if b.overlaps(f) && !b.inside(f) {
				l.problem(b.at, "diagram_straddle", b.what, f.what, f.at.Line)
			}
		}
	}
}

// checkEdges reports edges running through boxes, labels or frame headings.
func (l *layout) checkEdges() {
	for _, p := range l.paths {
		for _, b := range l.boxes {
			if p.runsThrough(b) {
				l.problem(p.edge.Pos, "diagram_runs_through", p.edge.D, b.what, b.at.Line)
			}
		}
	}
	// An edge across a label hides part of it behind the label's background, and an edge across text
	// without a background (a plain label, a frame heading) strikes through the text.
	for _, p := range l.paths {
		for i, r := range concat(l.labels, l.heads) {
			if i != p.own && p.runsThrough(r) {
				l.problem(p.edge.Pos, "diagram_runs_through", p.edge.D, r.what, r.at.Line)
			}
		}
	}
}

// checkCrossings reports edges crossing like an X, unless one of them allows it.
func (l *layout) checkCrossings() {
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			if a.edge.Crossing || b.edge.Crossing {
				continue
			}
			if x, y, ok := firstCrossing(a.edge, b.edge); ok {
				l.problem(a.edge.Pos, "diagram_crossing", a.edge.D, b.edge.D, b.edge.Line, formatNum(x), formatNum(y))
			}
		}
	}
}

func concat(lists ...[]rect) []rect {
	var all []rect
	for _, l := range lists {
		all = append(all, l...)
	}
	return all
}

func (p path) runsThrough(r rect) bool {
	for _, s := range p.edge.Segs {
		if crosses(s, r) {
			return true
		}
	}
	return false
}

// crosses reports whether a straight run passes inside r, 1px in from its edges, sampled every 2px.
// An end touching the edge of r does not count.
func crosses(s [4]float64, r rect) bool {
	x1, y1, x2, y2 := s[0], s[1], s[2], s[3]
	steps := math.Max(1, math.Ceil(math.Hypot(x2-x1, y2-y1)/2))
	for k := 0.0; k <= steps; k++ {
		px := x1 + (x2-x1)*k/steps
		py := y1 + (y2-y1)*k/steps
		if px > r.x+1 && px < r.x+r.w-1 && py > r.y+1 && py < r.y+r.h-1 {
			return true
		}
	}
	return false
}

// firstCrossing finds where a run of a first crosses a run of b, rounded as Math.round does.
func firstCrossing(a, b Edge) (x, y float64, ok bool) {
	for _, s := range a.Segs {
		for _, q := range b.Segs {
			if x, y, ok := crossing(s, q); ok {
				return math.Floor(x + 0.5), math.Floor(y + 0.5), true
			}
		}
	}
	return 0, 0, false
}

// crossing returns where two runs cross like an X. Parallel runs (sharing a stretch included) and
// runs touching at an end (a branch, a join, a corner) do not cross.
func crossing(s, q [4]float64) (x, y float64, ok bool) {
	x1, y1, x2, y2 := s[0], s[1], s[2], s[3]
	x3, y3, x4, y4 := q[0], q[1], q[2], q[3]
	den := (x2-x1)*(y4-y3) - (y2-y1)*(x4-x3)
	if math.Abs(den) < 1e-9 {
		return 0, 0, false
	}
	t := ((x3-x1)*(y4-y3) - (y3-y1)*(x4-x3)) / den
	u := ((x3-x1)*(y2-y1) - (y3-y1)*(x2-x1)) / den
	const e = 1e-6
	if t <= e || t >= 1-e || u <= e || u >= 1-e {
		return 0, 0, false
	}
	return x1 + t*(x2-x1), y1 + t*(y2-y1), true
}
