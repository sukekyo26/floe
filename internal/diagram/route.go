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
		dx, dy := other[0]-cx, other[1]-cy
		switch {
		case t.kind == "row" || math.Abs(dx) >= math.Abs(dy):
			side = map[bool]string{true: "right", false: "left"}[dx >= 0]
		default:
			side = map[bool]string{true: "bottom", false: "top"}[dy >= 0]
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

// route is the points of an edge from a to b with no via, right-angled, or straight from point to point by via.
// A right-angled edge with via is joined by addEdge.
// LIMIT: without via, two turns at most and no way round what is in the way; via or a side gives another.
func route(a, b anchor, via [][2]float64, straight bool) [][2]float64 {
	start, last := [][2]float64{{a.x, a.y}}, [2]float64{b.x, b.y}
	if straight {
		return slices.Concat(start, via, [][2]float64{last})
	}
	return slices.Concat(start, turns(a, b), [][2]float64{last})
}

// turns are the corners from a to b without via: two out past both for sides of one way (both left, …), so that
// the edge does not run along the parts' edges; none for ends in line; one for two points (across first) and for
// sides across; two in the middle for facing sides (H V H, V H V).
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
// across first when across. It returns the points and whether the edge goes across at the end, the way kept over
// a point the same as the one before.
func through(pts, next [][2]float64, across bool) ([][2]float64, bool) {
	for _, q := range next {
		p := pts[len(pts)-1]
		switch {
		case p == q:
		case p[0] == q[0]:
			across = false
		case p[1] == q[1]:
			across = true
		case across:
			pts = append(pts, [2]float64{q[0], p[1]})
			across = false
		default:
			pts = append(pts, [2]float64{p[0], q[1]})
			across = true
		}
		pts = append(pts, q)
	}
	return pts, across
}

// facing is the side of the end e that p, the via next to it, faces: the side towards p when p is beside the end
// (left or right) or above or below it, and when p is off both ways and known, the side across the way the edge
// came into p (left or right when across). "" leaves the side to endOf, as without via: for a point, a dot, p on
// the end or in it, and p off both ways at a from end or on a straight edge. A row takes left or right alone.
// LIMIT: a graph's box may get a side its graph's edges take, and the edge runs over them (S183); a from end with
// p off both ways may turn back over itself at p (S182).
func (l *layout) facing(e end, p [2]float64, across, known bool) string {
	t, ok := l.names[e.Name]
	if e.Point != nil || !ok || t.kind == "dot" || t.kind == "circle" {
		return ""
	}
	r := t.r
	lr := map[bool]string{true: "left", false: "right"}[p[0] < r.x+r.w/2]
	tb := map[bool]string{true: "top", false: "bottom"}[p[1] < r.y+r.h/2]
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

// enter joins pts to b, the end of a right-angled edge with via, across going across at the end. When the last
// point p is outside b's side, the last run meets the side square: one turn at (p.x, b.y) for left or right, at
// (b.x, p.y) for top or bottom, or, when that turn would run back along the run into p, two turns halfway between p
// and the side. Else the edge goes on as through takes it.
// LIMIT: a side written with p not outside it (on its line, beyond the end) is met as through takes it, and the edge
// may run along the end's edge; meeting it square takes a way round the end.
func enter(pts [][2]float64, b anchor, across bool) [][2]float64 {
	p, last := pts[len(pts)-1], [2]float64{b.x, b.y}
	if b.point || !outside(p, b) {
		pts, _ = through(pts, [][2]float64{last}, across)
		return pts
	}
	in := runIn(pts)
	switch {
	case horizontal(b.side) && in[1]*(last[1]-p[1]) < 0:
		mid := (p[0] + last[0]) / 2
		pts = append(pts, [2]float64{mid, p[1]}, [2]float64{mid, last[1]})
	case vertical(b.side) && in[0]*(last[0]-p[0]) < 0:
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

// edgePoints resolves e's ends and is the points it runs through, false when an end cannot be one (told at at).
func (l *layout) edgePoints(e *edge, at string) (pts [][2]float64, fromWho, toWho string, ok bool) {
	fromSide, toSide := e.FromSide, e.ToSide
	if len(e.Via) > 0 && fromSide == "" {
		fromSide = l.facing(e.From, e.Via[0], false, false)
	}
	from, fromWho, okFrom := l.endOf(at, e.From, fromSide, l.centre(e.To))
	angled := len(e.Via) > 0 && !e.Straight
	across := false
	if angled && okFrom {
		pts, across = through([][2]float64{{from.x, from.y}}, e.Via, horizontal(from.side) || from.side == "")
	}
	if len(e.Via) > 0 && toSide == "" {
		toSide = l.facing(e.To, e.Via[len(e.Via)-1], across, angled && okFrom)
	}
	// Resolved even when from is not, so that both are told.
	to, toWho, okTo := l.endOf(at, e.To, toSide, l.centre(e.From))
	if !okFrom || !okTo {
		return nil, "", "", false
	}
	if angled {
		return enter(pts, to, across), fromWho, toWho, true
	}
	return route(from, to, e.Via, e.Straight), fromWho, toWho, true
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
	el := elem{Kind: "path", D: pathData(pts), Class: cls, Tone: e.Tone, Start: l.marker(start), End: l.marker(end)}
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
		i := l.label(at, e.Label, x, y, cmp.Or(e.anchor, "middle"), inGraph, false)
		l.labels[i].graph = e.graph
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
