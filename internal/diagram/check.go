package diagram

import (
	"math"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Text sizes and box metrics of diagram.mjs.
const (
	sizeMain  = 13
	sizeSub   = 12
	pad       = 16 // a box's inner margin on each side
	lineGap   = 17 // between the rows of a box
	tableHead = 30
	tableRow  = 22
	tableSep  = 9
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

// rect is the area a part takes, with the words that name it in messages.
type rect struct {
	x, y, w, h float64
	what       *usererr.Error
	at         Pos
}

func (a rect) overlaps(b rect) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

func (a rect) inside(b rect) bool {
	return a.x >= b.x && a.y >= b.y && a.x+a.w <= b.x+b.w && a.y+a.h <= b.y+b.h
}

type path struct {
	edge Edge
	own  int // the index in labels of the edge's label, which it may run through; -1 without one
}

// layout is what the checks compare, gathered from the parts in the order diagram.mjs gathers them.
type layout struct {
	file     string
	boxes    []rect // edges may not run through these
	marks    []rect // may not overlap, but edges may reach their centre (dots)
	frames   []rect
	areas    []rect // may overlap (Venn circles)
	labels   []rect
	heads    []rect // frame headings, which edges may not cross
	paths    []path
	problems Problems
	seen     map[string]bool
}

// Check reports the layout mistakes of s, as diagram.mjs does: text too wide or tall for its box,
// parts sticking out of the canvas, overlapping, straddling the edge of a frame, edges running
// through boxes, labels or frame headings, and edges crossing. file names s in the messages.
func Check(file string, s *Spec) error {
	l := &layout{
		file: file, boxes: nil, marks: nil, frames: nil, areas: nil, labels: nil, heads: nil, paths: nil,
		problems: nil, seen: map[string]bool{},
	}
	for _, p := range s.Parts {
		l.add(p)
	}
	l.checkPlacement(s)
	l.checkEdges()
	l.checkCrossings()
	if len(l.problems) == 0 {
		return nil
	}
	return l.problems
}

func (l *layout) problem(at Pos, key string, args ...any) {
	e := &Error{File: l.file, Pos: at, Err: usererr.New(key, args...)}
	if !l.seen[e.Error()] {
		l.seen[e.Error()] = true
		l.problems = append(l.problems, e)
	}
}

func (l *layout) add(p Part) {
	switch p := p.(type) {
	case Frame:
		l.addFrame(p)
	case Icon:
		l.boxes = append(l.boxes, rect{p.X, p.Y, p.Size, p.Size, usererr.New("diagram_what_icon", p.Name), p.Pos})
		if p.Label != "" {
			l.label(p.Pos, p.Label, p.X+p.Size/2, p.Y+p.Size+16, "middle", false)
		}
	case Box:
		l.addBox(p)
	case Table:
		l.addTable(p)
	case Edge:
		e := path{edge: p, own: -1}
		if p.Label != "" {
			e.own = l.label(p.Pos, p.Label, p.LabelAt[0], p.LabelAt[1], "middle", false)
		}
		l.paths = append(l.paths, e)
	case Dot:
		what := usererr.New("diagram_what_dot", formatNum(p.CX), formatNum(p.CY))
		l.marks = append(l.marks, rect{p.CX - p.R, p.CY - p.R, p.R * 2, p.R * 2, what, p.Pos})
	case Bar:
		l.addBar(p)
	case Axis:
		for _, t := range p.Ticks {
			l.label(p.Pos, t.Text, t.X, p.Y+17, "middle", false)
		}
	case Circle:
		what := usererr.New("diagram_what_circle", formatNum(p.CX), formatNum(p.CY))
		l.areas = append(l.areas, rect{p.CX - p.R, p.CY - p.R, p.R * 2, p.R * 2, what, p.Pos})
	case Label:
		l.label(p.Pos, p.Text, p.X, p.Y, p.Anchor, p.Strong)
	}
}

// addFrame keeps the frame's heading too: inside the top left corner, or right of the corner icon.
func (l *layout) addFrame(f Frame) {
	l.frames = append(l.frames, rect{f.X, f.Y, f.W, f.H, usererr.New("diagram_what_frame", f.Label), f.Pos})
	if f.Label == "" {
		return
	}
	hx, hy, size := f.X+12, f.Y+18, float64(sizeSub)
	if f.Icon != "" {
		hx, hy, size = f.X+40, f.Y+15, sizeMain
	}
	head := usererr.New("diagram_what_heading", f.Label)
	l.heads = append(l.heads, rect{hx, hy - 9, textWidth(f.Label, size, false), 18, head, f.Pos})
}

func (l *layout) addBar(b Bar) {
	name := b.Label
	if name == "" {
		name = formatNum(b.X1) + "-" + formatNum(b.X2)
	}
	w := math.Max(b.X2-b.X1, 4)
	l.boxes = append(l.boxes, rect{b.X1, b.Y - 7, w, 14, usererr.New("diagram_what_bar", name), b.Pos})
	if b.Label != "" {
		l.label(b.Pos, b.Label, b.X1+w+10, b.Y, "start", false)
	}
}

// label places text at x, y as diagram.mjs does: 18px tall, with 5px around it. It returns the
// label's index in labels.
func (l *layout) label(at Pos, text string, x, y float64, anchor string, strong bool) int {
	size := float64(sizeSub)
	if strong {
		size = sizeMain
	}
	w := textWidth(text, size, false) + 10
	x0 := x - w/2
	switch anchor {
	case "start":
		x0 = x - 5
	case "end":
		x0 = x - w + 5
	}
	l.labels = append(l.labels, rect{x0, y - 9, w, 18, usererr.New("diagram_what_label", text), at})
	return len(l.labels) - 1
}

func (l *layout) addBox(b Box) {
	what := usererr.New("diagram_what_box", b.Rows[0].Text)
	room := b.W
	if b.Shape == "hex" {
		room -= 36
	}
	room -= pad * 2
	for _, r := range b.Rows {
		size := float64(sizeMain)
		if strings.HasPrefix(r.Style, "sub") {
			size = sizeSub
		}
		l.fit(b.Pos, what, r.Text, size, strings.HasSuffix(r.Style, "code"), room)
	}
	if need := len(b.Rows)*lineGap + 12; float64(need) > b.H {
		l.problem(b.Pos, "diagram_too_short", what, need, formatNum(b.H))
	}
	l.boxes = append(l.boxes, rect{b.X, b.Y, b.W, b.H, what, b.Pos})
}

func (l *layout) addTable(t Table) {
	what := usererr.New("diagram_what_table", t.Title)
	l.fit(t.Pos, what, t.Title, sizeMain, false, t.W-pad*2)
	h := float64(tableHead)
	for _, r := range t.Rows {
		if r.Sep {
			h += tableSep
			continue
		}
		l.fit(t.Pos, what, r.Name+"  "+r.Type, sizeSub, true, t.W-24)
		h += tableRow
	}
	if len(t.Rows) > 0 {
		h += 6
	}
	l.boxes = append(l.boxes, rect{t.X, t.Y, t.W, h, what, t.Pos})
}

func (l *layout) fit(at Pos, what *usererr.Error, text string, size float64, code bool, room float64) {
	if tw := textWidth(text, size, code); tw > room {
		l.problem(at, "diagram_too_narrow", what, text, int(math.Ceil(tw-room)))
	}
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
	texts := concat(l.labels, l.heads)
	for _, p := range l.paths {
		for i, r := range texts {
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

// textWidth estimates the width of s: a full-width character (above U+2E80) is size wide, any other
// 0.6 of it, or 0.62 in code.
// LIMIT: an estimate, as diagram.mjs makes it. Measure the text instead if a font change makes it miss.
func textWidth(s string, size float64, code bool) float64 {
	narrow := size * 0.6
	if code {
		narrow = size * 0.62
	}
	w := 0.0
	for _, c := range s {
		if c > 0x2e80 {
			w += size
		} else {
			w += narrow
		}
	}
	return w
}

// formatNum writes v as JavaScript's Number#toString does, so that numbers in messages and SVG read
// as diagram.mjs writes them.
func formatNum(v float64) string {
	if v == 0 {
		return "0" // and -0
	}
	if a := math.Abs(v); a >= 1e21 || a < 1e-6 {
		mant, exp, _ := strings.Cut(strconv.FormatFloat(v, 'e', -1, 64), "e")
		return mant + "e" + exp[:1] + strings.TrimLeft(exp[1:], "0")
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
