package diagram

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// anchor is where an edge leaves or reaches: a point, and the side it is on ("" for a dot's centre).
type anchor struct {
	x, y  float64
	side  string
	point bool
}

func horizontal(side string) bool {
	return side == "left" || side == "right"
}

func vertical(side string) bool {
	return side == "top" || side == "bottom"
}

// noEnd is what endOf returns for an end that cannot be one.
var noEnd = anchor{x: 0, y: 0, side: "", point: false}

// endOf resolves e, an end written with side, against other, the centre of the end at the other side of the
// edge: the anchor, the words that name it, and false when it cannot be an end (told at at).
func (l *layout) endOf(at string, e end, side string, other [2]float64) (anchor, string, bool) {
	if e.Point != nil {
		p := *e.Point
		if side != "" {
			l.problem(at, "diagram_end_side", pointName(p), side)
			return noEnd, "", false
		}
		// A point takes the side across, towards the other end.
		s := "right"
		if other[0] < p[0] {
			s = "left"
		}
		return anchor{p[0], p[1], s, true}, pointName(p), true
	}
	t, ok := l.names[e.Name]
	switch {
	case !ok:
		l.problem(at, "diagram_name_unknown", e.Name)
		return noEnd, "", false
	case t.kind == "circle":
		l.problem(at, "diagram_end_kind", e.Name, t.kind)
		return noEnd, "", false
	case t.kind == "dot" && side != "", t.kind == "row" && side != "" && !horizontal(side):
		l.problem(at, "diagram_end_side", strconv.Quote(e.Name), side)
		return noEnd, "", false
	}
	if t.kind == "dot" {
		return anchor{t.r.x + t.r.w/2, t.r.y + t.r.h/2, "", false}, e.Name, true
	}
	return onSide(t, side, other), e.Name, true
}

// onSide is the middle of the side of t, or when side is "", of the side facing other: left or right when it is
// more across than up or down from the centre of t, and always for a row.
func onSide(t target, side string, other [2]float64) anchor {
	r := t.r
	cx, cy := r.x+r.w/2, r.y+r.h/2
	if side == "" {
		lr, tb := towards(r, other)
		switch {
		case t.kind == "row" || math.Abs(other[0]-cx) >= math.Abs(other[1]-cy):
			side = lr
		default:
			side = tb
		}
	}
	switch side {
	case "left":
		return anchor{r.x, cy, side, false}
	case "right":
		return anchor{r.x + r.w, cy, side, false}
	case "top":
		return anchor{cx, r.y, side, false}
	default:
		return anchor{cx, r.y + r.h, side, false}
	}
}

// towards is the side of r across towards p, left or right by r's centre (right when level with it), and the side up
// or down towards p, top or bottom (bottom when level).
func towards(r rect, p [2]float64) (lr, tb string) {
	lr, tb = "left", "top"
	if p[0] >= r.x+r.w/2 {
		lr = "right"
	}
	if p[1] >= r.y+r.h/2 {
		tb = "bottom"
	}
	return lr, tb
}

func pointName(p [2]float64) string {
	return "[" + num(p[0]) + "," + num(p[1]) + "]"
}

// centre is the centre of the end e: its point, or the area it names (none for a name unknown).
func (l *layout) centre(e end) [2]float64 {
	if e.Point != nil {
		return *e.Point
	}
	t := l.names[e.Name]
	return [2]float64{t.r.x + t.r.w/2, t.r.y + t.r.h/2}
}

// turns are the corners from a to b without via: two out past both for sides of one way (both left, …), so that
// the edge does not run along the parts' edges; none for ends in line; one for two points (across first) and for
// sides across; two in the middle for facing sides (H V H, V H V).
// LIMIT: two turns at most and no way round what is in the way; via or a side gives another.
func turns(a, b anchor) [][2]float64 {
	switch {
	case oneWay(a, b):
		return around(a, b)
	case a.x == b.x || a.y == b.y:
		return nil
	case a.point && b.point:
		return [][2]float64{{b.x, a.y}}
	case horizontal(a.side) && horizontal(b.side):
		mid := (a.x + b.x) / 2
		return [][2]float64{{mid, a.y}, {mid, b.y}}
	case vertical(a.side) && vertical(b.side):
		mid := (a.y + b.y) / 2
		return [][2]float64{{a.x, mid}, {b.x, mid}}
	case a.side == "" && horizontal(b.side):
		// From a dot, up or down first, to reach a side across.
		return [][2]float64{{a.x, b.y}}
	case horizontal(a.side) || a.side == "":
		return [][2]float64{{b.x, a.y}}
	default:
		return [][2]float64{{a.x, b.y}}
	}
}

// oneWay reports whether a and b are sides of parts that face one way, apart: one side back to itself goes
// nowhere.
func oneWay(a, b anchor) bool {
	return a.side == b.side && a.side != "" && !a.point && !b.point && (a.x != b.x || a.y != b.y)
}

// around are the corners of an edge between sides of one way: out past both ends, so that it does not run
// along the parts' edges.
func around(a, b anchor) [][2]float64 {
	const out = 20 // how far past the parts it runs
	switch a.side {
	case "left":
		x := min(a.x, b.x) - out
		return [][2]float64{{x, a.y}, {x, b.y}}
	case "right":
		x := max(a.x, b.x) + out
		return [][2]float64{{x, a.y}, {x, b.y}}
	case "top":
		y := min(a.y, b.y) - out
		return [][2]float64{{a.x, y}, {b.x, y}}
	default:
		y := max(a.y, b.y) + out
		return [][2]float64{{a.x, y}, {b.x, y}}
	}
}

// through joins pts to each of next in turn, two points off line with one turn, along the way the edge was going:
// across first when across, unless that first run would go back along the run into the point, when it turns the
// other way first. It returns the points and whether the edge goes across at the end, the way kept over a point the
// same as the one before.
// LIMIT: the turn is taken without looking at what is in the way, so turning the other way first may run through a
// part that the way the edge was going passed by, and the edge is refused; another via gives another way.
func through(pts, next [][2]float64, across bool) ([][2]float64, bool) {
	for _, q := range next {
		p, in := pts[len(pts)-1], runIn(pts)
		switch {
		case p == q:
		case p[0] == q[0]:
			across = false
		case p[1] == q[1]:
			across = true
		default:
			first := across
			if back(in, p, q, across) {
				first = !across
			}
			pts = append(pts, corner(p, q, first))
			across = !first
		}
		pts = append(pts, q)
	}
	return pts, across
}

// corner is the turn from p to q off line: across first when across.
func corner(p, q [2]float64, across bool) [2]float64 {
	if across {
		return [2]float64{q[0], p[1]}
	}
	return [2]float64{p[0], q[1]}
}

// back reports whether the first run from p towards q, off line, across when across, would go back along in, the run
// into p.
func back(in, p, q [2]float64, across bool) bool {
	if across {
		return in[0]*(q[0]-p[0]) < 0
	}
	return in[1]*(q[1]-p[1]) < 0
}

// facing is the side of the end e that p, the via next to it, faces: the side towards p when p is beside the end
// (left or right) or above or below it, and when p is off both ways and known, the side across the way the edge
// came into p (left or right when across). "" leaves the side to endOf, as without via: for a point, a dot, p on
// the end or in it, and p off both ways at a from end or on a straight edge. A row takes left or right alone.
// LIMIT: a from end with p off both ways takes the side facing the other end, and when p is behind that side the
// edge runs through the part it starts from and is refused; writing the side gives another.
func (l *layout) facing(e end, p [2]float64, across, known bool) string {
	t, ok := l.names[e.Name]
	if e.Point != nil || !ok || t.kind == "dot" || t.kind == "circle" {
		return ""
	}
	r := t.r
	lr, tb := towards(r, p)
	level := p[1] >= r.y && p[1] <= r.y+r.h
	under := p[0] >= r.x && p[0] <= r.x+r.w
	switch {
	case t.kind == "row" && !under:
		return lr
	case t.kind == "row", level && under:
		return ""
	case level, !under && known && across:
		return lr
	case under, known:
		return tb
	}
	return ""
}

// clear is side, the side taken for the end e with none written ("" for the side facing other, as endOf takes it),
// or, when e is a box or frame a graph or a tree laid out and side is one their edges take, a side of those left to
// edges written by hand (target.handSides) that p, the via next to e or else other, is outside of: across towards p
// first, then up or down towards it. With neither, side is kept: another would run along e's edge or back over the
// edge, where this one meets theirs, mostly told as a crossing.
// LIMIT: with no side left that p is outside of, the edge keeps a side the graph's or the tree's edges take, and may
// run over theirs silently; write the via outside a side left, or write the side. The graph's labels keep off a box's
// sides across its edges alone (hands), not a box's top or bottom left, nor a frame's sides left, so an edge to one
// may meet a label and be refused. A box's sides left are counted from the ends of the graph's edges, so one running
// past a box, skipping ranks, may cross or run beside an edge ending at the box's side.
func (l *layout) clear(e end, side string, other, p [2]float64) string {
	t, ok := l.names[e.Name]
	if e.Point != nil || !ok || t.handSides == nil {
		return side
	}
	if side == "" {
		side = onSide(t, "", other).side
	}
	if slices.Contains(t.handSides, side) {
		return side
	}
	lr, tb := towards(t.r, p)
	for _, s := range []string{lr, tb} {
		if slices.Contains(t.handSides, s) && outside(p, onSide(t, s, p)) {
			return s
		}
	}
	return side
}

// enter joins pts to b, the end of a right-angled edge with via, across going across at the end. A point or a dot is
// joined as through takes it. When the last point p is outside b's side, the last run meets the side square: one turn
// at (p.x, b.y) for left or right, at (b.x, p.y) for top or bottom, or, when that turn would run back along the run
// into p, two turns halfway between p and the side. Else the edge goes on the way it was going, with no turn the other
// way first: that would run along the end's edge, drawn, where this runs back, mostly through the end, and is refused.
// LIMIT: a side with p not outside it (on its line, beyond the end, or p in the end) is met the way the edge was going,
// and the edge may run along the end's edge or through it; meeting it square takes a way round the end. A p level with
// the side, come to by a run going away from it, runs straight back along that run; turning off it takes a way round as
// well.
func enter(pts [][2]float64, b anchor, across bool) [][2]float64 {
	p, last := pts[len(pts)-1], [2]float64{b.x, b.y}
	switch {
	case b.point, b.side == "":
		pts, _ = through(pts, [][2]float64{last}, across)
		return pts
	case !outside(p, b):
		if p[0] != last[0] && p[1] != last[1] {
			pts = append(pts, corner(p, last, across))
		}
		return append(pts, last)
	}
	in := runIn(pts)
	switch {
	case horizontal(b.side) && back(in, p, last, false):
		mid := (p[0] + last[0]) / 2
		pts = append(pts, [2]float64{mid, p[1]}, [2]float64{mid, last[1]})
	case vertical(b.side) && back(in, p, last, true):
		mid := (p[1] + last[1]) / 2
		pts = append(pts, [2]float64{p[0], mid}, [2]float64{last[0], mid})
	case horizontal(b.side) && p[1] != last[1]:
		pts = append(pts, [2]float64{p[0], last[1]})
	case vertical(b.side) && p[0] != last[0]:
		pts = append(pts, [2]float64{last[0], p[1]})
	}
	return append(pts, last)
}

// outside reports whether p is on the far side of b's side from the part b is on: left of a left side, and so on.
func outside(p [2]float64, b anchor) bool {
	switch b.side {
	case "left":
		return p[0] < b.x
	case "right":
		return p[0] > b.x
	case "top":
		return p[1] < b.y
	case "bottom":
		return p[1] > b.y
	}
	return false
}

// runIn is the last run of pts of some length, as a vector; none when every point is the first, and then nothing
// to turn back along.
func runIn(pts [][2]float64) [2]float64 {
	for i := len(pts) - 1; i > 0; i-- {
		if pts[i] != pts[i-1] {
			return [2]float64{pts[i][0] - pts[i-1][0], pts[i][1] - pts[i-1][1]}
		}
	}
	return [2]float64{0, 0}
}

// pathData writes the points as SVG path data: H and V along an axis, L across.
func pathData(pts [][2]float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "M%s,%s", num(pts[0][0]), num(pts[0][1]))
	for i := 1; i < len(pts); i++ {
		p, q := pts[i-1], pts[i]
		switch {
		case p[1] == q[1]:
			fmt.Fprintf(&b, " H%s", num(q[0]))
		case p[0] == q[0]:
			fmt.Fprintf(&b, " V%s", num(q[1]))
		default:
			fmt.Fprintf(&b, " L%s,%s", num(q[0]), num(q[1]))
		}
	}
	return b.String()
}

// edgePoints resolves e's ends and is the points it runs through, false when an end cannot be one (told at at):
// straight from point to point by via, right-angled by turns without via, or right-angled through via, turning as the
// via are written and entering the last side by enter.
// LIMIT: the runs between via go round nothing in the way.
func (l *layout) edgePoints(e *edge, at string) (pts [][2]float64, fromWho, toWho string, ok bool) {
	fromSide, toSide := e.FromSide, e.ToSide
	fromAt, toAt := l.centre(e.From), l.centre(e.To)
	if fromSide == "" {
		near := toAt
		if len(e.Via) > 0 {
			fromSide, near = l.facing(e.From, e.Via[0], false, false), e.Via[0]
		}
		fromSide = l.clear(e.From, fromSide, toAt, near)
	}
	from, fromWho, okFrom := l.endOf(at, e.From, fromSide, toAt)
	angled := len(e.Via) > 0 && !e.Straight
	across := false
	if angled && okFrom {
		pts, across = through([][2]float64{{from.x, from.y}}, e.Via, horizontal(from.side) || from.side == "")
	}
	if toSide == "" {
		near := fromAt
		if len(e.Via) > 0 {
			near = e.Via[len(e.Via)-1]
			toSide = l.facing(e.To, near, across, angled && okFrom)
		}
		toSide = l.clear(e.To, toSide, fromAt, near)
	}
	// Resolved even when from is not, so that both are told.
	to, toWho, okTo := l.endOf(at, e.To, toSide, fromAt)
	if !okFrom || !okTo {
		return nil, "", "", false
	}
	start, last := [][2]float64{{from.x, from.y}}, [2]float64{to.x, to.y}
	switch {
	case e.Straight:
		pts = slices.Concat(start, e.Via, [][2]float64{last})
	case len(e.Via) == 0:
		pts = slices.Concat(start, turns(from, to), [][2]float64{last})
	default:
		pts = enter(pts, to, across)
	}
	return pts, fromWho, toWho, true
}

// atDots is pts, an edge's points as routed, as drawn: each end at a dot it names moved back from the centre towards
// the first point off it (rimEnd), both measured on pts. Only the drawing moves: the edge is checked and labelled as
// routed, so that what it is refused for and where its label stands are as with the end at the centre. Ends on the one
// run between two dots move only when the run keeps room for both.
// LIMIT: an edge between two dots that touch or overlap ends at their centres, its arrows hidden; move them apart.
// LIMIT: the label of an edge, placed as routed, may stand over an arrow moved back on a short run; write at.
func (l *layout) atDots(e *edge, pts [][2]float64, start, end string) [][2]float64 {
	last := len(pts) - 1
	a, aOK := l.rimEnd(e.From, pts, 0, 1, start)
	b, bOK := l.rimEnd(e.To, pts, last, -1, end)
	if !aOK && !bOK {
		return pts
	}
	_, fromDot := l.dot(e.From)
	_, toDot := l.dot(e.To)
	if fromDot && toDot && offFrom(pts, 0, 1) == last && (!aOK || !bOK || a.run-a.back-b.back <= max(a.keep, b.keep)) {
		return pts
	}
	lo, hi := 0, last
	var out [][2]float64
	if aOK {
		out, lo = append(out, a.at), a.off
	}
	if bOK {
		hi = b.off
	}
	if lo <= hi {
		out = append(out, pts[lo:hi+1]...)
	}
	if bOK {
		out = append(out, b.at)
	}
	return out
}

// rimmed is where an end at a dot is drawn: at, back from the centre along the run to the first point off it, pts[off],
// run long; back, how far it moves, keep, how much of the run its marker needs behind it.
type rimmed struct {
	at              [2]float64
	off             int
	run, back, keep float64
}

// rimEnd is the end of pts at i, going on by step, moved back from the centre of a dot e names so that it, or the
// tip of its marker kind, meets the dot's outline; false for an end at no dot, or whose run cannot keep its marker
// (or is no longer than the radius, with none).
// LIMIT: an end written as a point at a dot's centre is no dot's and stays, its arrow hidden: name the dot.
func (l *layout) rimEnd(e end, pts [][2]float64, i, step int, kind string) (rimmed, bool) {
	t, ok := l.dot(e)
	if !ok {
		return rimmed{}, false //nolint:exhaustruct_v5 // none
	}
	off := offFrom(pts, i, step)
	if off < 0 {
		return rimmed{}, false //nolint:exhaustruct_v5 // none
	}
	c, q := pts[i], pts[off]
	run := math.Hypot(q[0]-c[0], q[1]-c[1])
	back, keep := pull(t.r.w/2, kind)
	// LIMIT: a run too short for its marker (a via in line within it too) leaves the end at the centre, hidden.
	if run-back <= keep {
		return rimmed{}, false //nolint:exhaustruct_v5 // none
	}
	at := [2]float64{c[0] + (q[0]-c[0])/run*back, c[1] + (q[1]-c[1])/run*back}
	return rimmed{at: at, off: off, run: run, back: back, keep: keep}, true
}

// dot is the dot e names, false for an end that names no dot.
func (l *layout) dot(e end) (target, bool) {
	t, ok := l.names[e.Name]
	return t, e.Point == nil && ok && t.kind == "dot"
}

// offFrom is the index of the first of pts off pts[i], going on from i by step; -1 when all stand on it.
func offFrom(pts [][2]float64, i, step int) int {
	for j := i + step; j >= 0 && j < len(pts); j += step {
		if pts[j] != pts[i] {
			return j
		}
	}
	return -1
}

// pull is how far an end at a dot of radius r moves back for the tip of its marker of kind (none: the end) to meet the
// outline, and how much of its run the marker needs behind the end.
func pull(r float64, kind string) (back, keep float64) {
	for _, m := range markers {
		if m.Kind == kind {
			return r + m.W - m.RX, m.RX
		}
	}
	return r, 0
}

func (l *layout) addEdge(e *edge, at string) {
	pts, fromWho, toWho, ok := l.edgePoints(e, at)
	if !ok {
		return
	}
	segs := segsOf(pts)
	who := e.who
	if who == "" {
		who = "edge " + fromWho + "→" + toWho
	}
	if len(segs) == 0 {
		l.problem(at, "diagram_edge_nowhere", who)
		return
	}
	cls := "edge"
	if e.Dashed {
		cls += " dashed"
	}
	if e.Bold {
		cls += " bold"
	}
	start, end := "none", "arrow"
	if e.Start != nil {
		start = *e.Start
	}
	if e.End != nil {
		end = *e.End
	}
	drawn := l.atDots(e, pts, start, end)
	el := elem{Kind: "path", D: pathData(drawn), Class: cls, Tone: e.Tone, Start: l.marker(start), End: l.marker(end)}
	l.edgeEls = append(l.edgeEls, el)
	p := path{segs: segs, what: who, at: at, own: -1, crossing: e.Crossing, graph: e.graph}
	if e.Label != nil {
		x, y := middle(segs)
		if e.At != nil {
			x, y = e.At[0], e.At[1]
		}
		// A graph's label stands beside its own line, not on it: nothing to hide under it, and its own line is
		// checked against it like any other.
		inGraph := e.graph != ""
		i := l.label(at, e.Label, nil, x, y, cmp.Or(e.anchor, "middle"), inGraph, false)
		l.labels[i].graph, l.labels[i].pinned = e.graph, e.At != nil && !inGraph
		if !inGraph {
			p.own = i
		}
	}
	l.paths = append(l.paths, p)
	if len(segs) > 0 {
		r := span(segs, who, at)
		r.graph = e.graph
		l.strokes = append(l.strokes, r)
	}
}

// segsOf is the runs between corners pts, those of no length left out.
func segsOf(pts [][2]float64) []seg {
	segs := make([]seg, 0, len(pts)-1)
	for i := 1; i < len(pts); i++ {
		if pts[i] != pts[i-1] {
			segs = append(segs, seg{pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]})
		}
	}
	return segs
}

// middle is the middle of the longest run, where an edge's label goes unless it says where.
func middle(segs []seg) (x, y float64) {
	longest, best := -1.0, seg{}
	for _, s := range segs {
		if n := math.Hypot(s[2]-s[0], s[3]-s[1]); n > longest {
			longest, best = n, s
		}
	}
	return (best[0] + best[2]) / 2, (best[1] + best[3]) / 2
}
