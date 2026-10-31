package diagram

import (
	"math"
	"slices"
	"strings"
)

// check reports the layout's mistakes: text too wide or tall for its box or frame (as it was laid out), parts
// (edges, lines and axes too) sticking out of the canvas, overlapping (frame headings too), boxes and marks straddling
// the edge of a frame, edges running through boxes, labels or frame headings, and edges crossing. A frame's own box is
// compared with no other frame's, and a frame's heading is not kept within the frame's height or a canvas written
// (its width is, by addFrame): frames may cross on purpose (a group over two columns, bands that cross), and on the
// page a heading out of its frame shows below it, one out of a canvas written cut short or gone (a canvas left
// out reaches it). A frame's icon is a
// box and its heading a text, each checked as those are.
func (l *layout) check() {
	l.checkPlacement()
	l.checkEdges()
	l.checkCrossings()
}

// checkPlacement reports parts sticking out of the canvas; boxes (a frame's icon among them), marks, labels and
// frame headings overlapping; and boxes and marks straddling a frame. Frames are not compared with one another.
// LIMIT: labels are not kept off a frame's edge, where an edge's label may sit; compare label parts if one strays.
func (l *layout) checkPlacement() {
	canvas := rect{x: 0, y: 0, w: l.w, h: l.h, what: "", at: ""}
	for _, r := range slices.Concat(l.boxes, l.marks, l.labels, l.frames, l.areas, l.strokes) {
		if !r.inside(canvas) {
			l.problem(r.at, "diagram_outside", r.what, num(l.w), num(l.h))
		}
	}
	for _, r := range l.strays {
		l.problem(r.at, "diagram_outside", r.what, num(l.w), num(l.h))
	}
	solids := slices.Concat(l.boxes, l.marks, l.labels, l.heads)
	// LIMIT: every pair is compared (O(n²)), here and for edges. If diagrams of hundreds of parts get slow,
	// compare only the parts that share a cell of a grid.
	for i, a := range solids {
		for _, b := range solids[i+1:] {
			if a.overlaps(b) {
				l.problem(a.at, "diagram_overlap", a.what, b.what)
			}
		}
	}
	for _, b := range slices.Concat(l.boxes, l.marks) {
		for _, f := range l.frames {
			if b.overlaps(f) && !b.inside(f) {
				l.problem(b.at, "diagram_straddle", b.what, f.what)
			}
		}
	}
}

// checkEdges reports edges running through boxes, labels or frame headings: an edge across a label hides part
// of it behind the label's background, and one across text without a background strikes through it.
func (l *layout) checkEdges() {
	texts := slices.Concat(l.labels, l.heads)
	for _, p := range l.paths {
		for _, b := range l.boxes {
			if p.runsThrough(b) {
				l.problem(p.at, "diagram_runs_through", p.what, b.what)
			}
		}
		for i, r := range texts {
			if i != p.own && p.runsThrough(r) {
				l.problem(p.at, "diagram_runs_through", p.what, r.what)
			}
		}
	}
}

// checkCrossings reports edges crossing like an X, unless one of them allows it.
// LIMIT: edges overlapping along a stretch pass (a ↔ drawn on purpose); refuse them if accidental overlaps mislead.
func (l *layout) checkCrossings() {
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			if a.crossing || b.crossing {
				continue
			}
			if x, y, ok := firstCrossing(a, b); ok {
				l.problem(a.at, "diagram_crossing", a.what, b.what, num(x), num(y))
			}
		}
	}
}

func (p path) runsThrough(r rect) bool {
	return slices.ContainsFunc(p.segs, func(s seg) bool { return crossedBy(s, r) })
}

// firstCrossing finds where a run of a first crosses a run of b, rounded to whole units.
func firstCrossing(a, b path) (x, y float64, ok bool) {
	for _, s := range a.segs {
		for _, q := range b.segs {
			if x, y, ok := crossing(s, q); ok {
				return math.Floor(x + 0.5), math.Floor(y + 0.5), true
			}
		}
	}
	return 0, 0, false
}

// crossing returns where two runs cross like an X. Parallel runs (sharing a stretch included) and runs touching
// at an end (a branch, a join, a corner) do not cross.
func crossing(s, q seg) (x, y float64, ok bool) {
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

// region is an area of a Venn diagram: inside the circles of in, outside the others.
type region struct {
	in, out []circle
}

// roomOf is how far inside the region r is: the least of R less the distance of each corner, for each circle of
// in, and of the distance of r's nearest point, less R, for each of out; negative when it is outside.
func (g region) roomOf(r rect) float64 {
	least := math.Inf(1)
	for _, c := range g.in {
		for _, p := range [4][2]float64{{r.x, r.y}, {r.x + r.w, r.y}, {r.x, r.y + r.h}, {r.x + r.w, r.y + r.h}} {
			least = min(least, c.R-math.Hypot(p[0]-c.CX, p[1]-c.CY))
		}
	}
	for _, c := range g.out {
		nx, ny := min(max(c.CX, r.x), r.x+r.w), min(max(c.CY, r.y), r.y+r.h)
		least = min(least, math.Hypot(nx-c.CX, ny-c.CY)-c.R)
	}
	return least
}

// regionOf is the region of the circles named in, telling a name that is no circle; false when there is one.
func (l *layout) regionOf(at string, in []string) (region, bool) {
	var g region
	ok := true
	for _, name := range in {
		c, found := l.circles[name]
		if !found {
			l.problem(at, "diagram_label_not_circle", name)
			ok = false
			continue
		}
		g.in = append(g.in, c)
	}
	// Every other circle is outside the region, one of no name too.
	for _, p := range l.s.Parts {
		if c := p.Circle; c != nil && (c.Name == "" || !slices.Contains(in, c.Name)) {
			g.out = append(g.out, *c)
		}
	}
	return g, ok
}

// addLabel places a label: where it says, or, with in alone, in its region of a Venn diagram, below the labels
// placed there before it; one at a place that says in is checked to be in that region, all of it.
func (l *layout) addLabel(lb *label, at string) {
	anchor := cmpOr(lb.Anchor, "middle")
	if lb.In == nil {
		l.label(at, lb.Text, *lb.X, *lb.Y, anchor, lb.Plain, lb.Strong)
		return
	}
	g, ok := l.regionOf(at, lb.In)
	if !ok {
		return
	}
	names := strings.Join(lb.In, ", ")
	if lb.X != nil {
		i := l.label(at, lb.Text, *lb.X, *lb.Y, anchor, lb.Plain, lb.Strong)
		if g.roomOf(l.labels[i]) < 0 {
			l.problem(at, "diagram_label_region", l.labels[i].what, names)
		}
		return
	}
	x, y, st := l.place(lb, g)
	switch {
	case !st.tried:
		l.strays = append(l.strays, rect{x: 0, y: 0, w: 0, h: 0, what: what("label", "", lb.Text), at: at})
		return
	case !st.found:
		l.problem(at, "diagram_label_no_room", what("label", "", lb.Text), names)
		return
	}
	l.label(at, lb.Text, x, y, "middle", lb.Plain, lb.Strong)
}

// place finds where a label placed by its region goes: the labels of one region are stacked, the first above,
// and placed as one block where it keeps the most room; the block is placed once, for its first label.
// LIMIT: points 4 apart are tried, and at most 501 a side of where the circles all are within the canvas; try
// them closer if a region narrow enough to slip between comes to matter.
func (l *layout) place(lb *label, g region) (x, y float64, st stack) {
	key := regionKey(lb.In)
	st = l.stacks[key]
	if st.next == 0 {
		w := 0.0
		for _, m := range st.labels {
			w = max(w, labelWidth(m.Text, m.Strong))
		}
		h := labelH * float64(len(st.labels))
		// The points of a grid over where the circles of in all are (their boxes' overlap, in no order of in),
		// those within the canvas alone: a label sticks out when none of it has room in the canvas. Circles
		// whose boxes do not overlap hold no room, wherever they are.
		x0, y0, x1, y1 := math.Inf(-1), math.Inf(-1), math.Inf(1), math.Inf(1)
		for _, c := range g.in {
			x0, y0, x1, y1 = max(x0, c.CX-c.R), max(y0, c.CY-c.R), min(x1, c.CX+c.R), min(y1, c.CY+c.R)
		}
		st.tried = x0 >= x1 || y0 >= y1 ||
			max(x0, 0) < min(x1, l.w) && max(y0, 0) < min(y1, l.h)
		xs, ys := grid(x0, x1, l.w), grid(y0, y1, l.h)
		var best float64
		best, st.x, st.top = g.most(xs, ys, w, h, (x0+x1)/2, (y0+y1)/2)
		st.found = best > 0
	}
	n := st.next
	st.next++
	l.stacks[key] = st
	return st.x, st.top + labelH*float64(n) + labelH/2, st
}

// grid is at most 501 points from lo to hi within 0 and size, step apart, step being 4 or more: counted from lo,
// so that the canvas cuts the grid and does not move it, else from where the canvas cuts it. Each is made from
// its index, not by adding step again and again, which a float stops moving.
// LIMIT: where a float's spacing passes step (beyond 2^55 or so for a step of 4), points round to the same
// values, and what is between them is not tried; where it passes 500 steps (5e19 and the like), the points are
// all one. Keep the points relative to the region's origin if a diagram that large comes to matter.
func grid(lo, hi, size float64) []float64 {
	from, to := max(lo, 0), min(hi, size)
	if !(from <= to) {
		return nil
	}
	step := max(4, (to-from)/500)
	start := lo + math.Ceil((from-lo)/step)*step
	if !(start >= from && start < from+step && start <= to) {
		start = from
	}
	var out []float64
	for i := range 501 {
		p := start + float64(i)*step
		if !(p <= to) {
			break
		}
		out = append(out, p)
	}
	return out
}

// most is the room of the point of xs and ys where a block w by h, centred on it, keeps the most room, and where
// the block goes then (its middle and top), 0 for none of room: of points of one room, the one nearest mx, my,
// the middle of the circles' boxes, as room told by the point of a box nearest a circle is one while the box
// covers its centre, and a label stays on the axis of a Venn diagram of one when the middle is a point of the grid.
// LIMIT: the grid counts 4 apart from where the circles' boxes start, so a middle off it leaves the label off the
// axis, above or below as the radius goes; try the middle itself first if diagrams come to need it.
func (g region) most(xs, ys []float64, w, h, mx, my float64) (best, x, top float64) {
	near := math.Inf(1)
	for _, px := range xs {
		for _, py := range ys {
			switch room := g.roomOf(rect{x: px - w/2, y: py - h/2, w: w, h: h, what: "", at: ""}); {
			case room > best:
				best, near, x, top = room, math.Hypot(px-mx, py-my), px, py-h/2
			case room == best:
				if d := math.Hypot(px-mx, py-my); d < near {
					near, x, top = d, px, py-h/2
				}
			}
		}
	}
	return best, x, top
}

// stack is the labels of one region of a Venn diagram placed by it, and where their block went.
type stack struct {
	labels []*label
	next   int
	x, top float64
	tried  bool // false when the box of where the circles all are has no area in the canvas: the label sticks out
	found  bool
}

func regionKey(in []string) string {
	return strings.Join(slices.Sorted(slices.Values(in)), "\x00")
}
