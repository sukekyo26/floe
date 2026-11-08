package diagram

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// anchor is where an edge leaves or reaches: a point, and the side it is on ("" for a dot's centre). rim is the line of
// the side, an x on the left or right and a y on the top or bottom, which the point stands in from on a round corner or
// a slant: what is placed by the side reads it, so that edges from one side keep to one line wherever they leave it.
type anchor struct {
	x, y  float64
	side  string
	point bool
	rim   float64
}

func horizontal(side string) bool {
	return side == "left" || side == "right"
}

func vertical(side string) bool {
	return side == "top" || side == "bottom"
}

// noEnd is what endOf returns for an end that cannot be one.
var noEnd = anchor{x: 0, y: 0, side: "", point: false, rim: 0}

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
		return anchor{p[0], p[1], s, true, p[0]}, pointName(p), true
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
		return anchor{t.r.x + t.r.w/2, t.r.y + t.r.h/2, "", false, 0}, strconv.Quote(e.Name), true
	}
	return onSide(t, side, other), strconv.Quote(e.Name), true
}

// onSide is the middle of the side of t, or when side is "", of the side facing other: left or right when it is
// more across than up or down from the centre of t.whole(), and always for a row. An icon's bottom side is under its
// label; its left and right sides stay at the middle of the icon.
// LIMIT: an edge at the bottom side going beside the label or above its bottom runs back through the label, and is
// refused: with no side, when the other part's top is higher than 2 above the label's bottom and its centre further
// down than across; with the sides written, or with a via between the icon and its label, likewise. Write
// fromSide: right, or move the other part or the via below the label.
// LIMIT: crossedBy reads a run along an edge as not through it, so these pass, plain on the page: a run along the
// label's bottom; another edge running exactly there (it reads as a branch, not a crossing); a part whose top is within
// 2 of the label's bottom, the across run in it or on its edge and the marker inside it or with no room. Refuse them if
// they come to mislead.
func onSide(t target, side string, other [2]float64) anchor {
	r := t.r
	cx, cy := r.x+r.w/2, r.y+r.h/2
	if side == "" {
		whole := t.whole()
		lr, tb := towards(whole, other)
		switch {
		case t.kind == "row" || math.Abs(other[0]-cx) >= math.Abs(other[1]-(whole.y+whole.h/2)):
			side = lr
		default:
			side = tb
		}
	}
	at := cy
	if vertical(side) {
		at = cx
	}
	return anchorOn(t, side, at)
}

// anchorOn is the end on side of t at at.
func anchorOn(t target, side string, at float64) anchor {
	p := sidePoint(t, side, at)
	r := t.r
	rim := r.y + r.h + t.under
	switch side {
	case "left":
		rim = r.x
	case "right":
		rim = r.x + r.w
	case "top":
		rim = r.y
	}
	return anchor{p[0], p[1], side, false, rim}
}

// sidePoint is where an end stands on side of t at at, an x on the top or bottom, a y on the left or right: on the
// outline, and on the bottom side within a label's width, under the label.
func sidePoint(t target, side string, at float64) [2]float64 {
	p := t.r.form.on(t.r, side, at)
	if side == "bottom" && math.Abs(at-(t.r.x+t.r.w/2)) <= t.labelW/2 {
		p[1] += t.under
	}
	return p
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
// The run between the two turns, which floe places, is loose (a's and b's markers as long as marks says).
// LIMIT: two turns at most and no way round what is in the way; via or a side gives another.
func turns(a, b anchor, marks [2]float64) ([][2]float64, loose) {
	switch {
	case oneWay(a, b):
		return around(a, b)
	case a.x == b.x || a.y == b.y:
		return nil, noLoose
	case a.point && b.point:
		return [][2]float64{{b.x, a.y}}, noLoose
	case horizontal(a.side) && horizontal(b.side):
		mid := (a.rim + b.rim) / 2
		return [][2]float64{{mid, a.y}, {mid, b.y}}, between(1, false, a.rim, b.rim, marks)
	case vertical(a.side) && vertical(b.side):
		mid := (a.rim + b.rim) / 2
		return [][2]float64{{a.x, mid}, {b.x, mid}}, between(1, true, a.rim, b.rim, marks)
	case a.side == "" && horizontal(b.side):
		// From a dot, up or down first, to reach a side across.
		return [][2]float64{{a.x, b.y}}, noLoose
	case horizontal(a.side) || a.side == "":
		return [][2]float64{{b.x, a.y}}, noLoose
	default:
		return [][2]float64{{a.x, b.y}}, noLoose
	}
}

// oneWay reports whether a and b are sides of parts that face one way, apart: one side back to itself goes
// nowhere.
func oneWay(a, b anchor) bool {
	return a.side == b.side && a.side != "" && !a.point && !b.point && (a.x != b.x || a.y != b.y)
}

// around are the corners of an edge between sides of one way: out past both ends, so that it does not run
// along the parts' edges. The run out there is loose, outward only: inward it comes nearer the parts.
// LIMIT: so a run out there on a rule stays on it when moving out lays the runs to it, made longer, along a line
// (a line across from a side, beyond the rule); write a via or a side to take another way.
func around(a, b anchor) ([][2]float64, loose) {
	const out = 20 // how far past the parts it runs
	switch a.side {
	case "left":
		x := min(a.rim, b.rim) - out
		return [][2]float64{{x, a.y}, {x, b.y}}, loose{i: 1, across: false, lo: math.Inf(-1), hi: x, ok: true}
	case "right":
		x := max(a.rim, b.rim) + out
		return [][2]float64{{x, a.y}, {x, b.y}}, loose{i: 1, across: false, lo: x, hi: math.Inf(1), ok: true}
	case "top":
		y := min(a.rim, b.rim) - out
		return [][2]float64{{a.x, y}, {b.x, y}}, loose{i: 1, across: true, lo: math.Inf(-1), hi: y, ok: true}
	default:
		y := max(a.rim, b.rim) + out
		return [][2]float64{{a.x, y}, {b.x, y}}, loose{i: 1, across: true, lo: y, hi: math.Inf(1), ok: true}
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
	r := t.whole()
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
// The run between the two turns halfway is loose (b's marker as long as mark).
func enter(pts [][2]float64, b anchor, across bool, mark float64) ([][2]float64, loose) {
	p, last := pts[len(pts)-1], [2]float64{b.x, b.y}
	switch {
	case b.point, b.side == "":
		pts, _ = through(pts, [][2]float64{last}, across)
		return pts, noLoose
	case !outside(p, b):
		if p[0] != last[0] && p[1] != last[1] {
			pts = append(pts, corner(p, last, across))
		}
		return append(pts, last), noLoose
	}
	in := runIn(pts)
	free := noLoose
	switch {
	case horizontal(b.side) && back(in, p, last, false):
		mid := (p[0] + last[0]) / 2
		free = between(len(pts), false, p[0], last[0], [2]float64{0, mark})
		pts = append(pts, [2]float64{mid, p[1]}, [2]float64{mid, last[1]})
	case vertical(b.side) && back(in, p, last, true):
		mid := (p[1] + last[1]) / 2
		free = between(len(pts), true, p[1], last[1], [2]float64{0, mark})
		pts = append(pts, [2]float64{p[0], mid}, [2]float64{last[0], mid})
	case horizontal(b.side) && p[1] != last[1]:
		pts = append(pts, [2]float64{p[0], last[1]})
	case vertical(b.side) && p[0] != last[0]:
		pts = append(pts, [2]float64{last[0], p[1]})
	}
	return append(pts, last), free
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
func (l *layout) edgePoints(e *edge, at string) (pts [][2]float64, free loose, fromWho, toWho string, ok bool) {
	from, to, pts, across, fromWho, toWho, ok := l.edgeEnds(e, at)
	if !ok {
		return nil, noLoose, "", "", false
	}
	startMark, endMark := markerKinds(e)
	marks := [2]float64{markLength(startMark), markLength(endMark)}
	start, last := [][2]float64{{from.x, from.y}}, [2]float64{to.x, to.y}
	switch {
	case e.Straight:
		pts = append(pts, last)
	case len(pts) == 0:
		var mid [][2]float64
		mid, free = turns(from, to, marks)
		pts = slices.Concat(start, mid, [][2]float64{last})
	default:
		pts, free = enter(pts, to, across, marks[1])
	}
	return pts, free, fromWho, toWho, true
}

// viaOf is the points of e's via: an axis not written is the point before's, the first point's the centre of e's start,
// so that the point is known before the side it makes e start from (facing).
func (l *layout) viaOf(e *edge) [][2]float64 {
	pts := make([][2]float64, len(e.Via))
	p := l.centre(e.From)
	for i, v := range e.Via {
		if v.X != nil {
			p[0] = *v.X
		}
		if v.Y != nil {
			p[1] = *v.Y
		}
		pts[i] = p
	}
	return pts
}

// edgeEnds resolves e's ends and the sides they take, false when an end cannot be one (told at at): with via or
// straight, also the points from its start through its via, turning as written when right-angled, and whether it goes
// across at the last of them.
func (l *layout) edgeEnds(e *edge, at string) (
	from, to anchor, pts [][2]float64, across bool, fromWho, toWho string, ok bool,
) {
	fromSide, toSide := e.FromSide, e.ToSide
	fromAt, toAt := l.centre(e.From), l.centre(e.To)
	via := l.viaOf(e)
	if fromSide == "" {
		near := toAt
		if len(via) > 0 {
			fromSide, near = l.facing(e.From, via[0], false, false), via[0]
		}
		fromSide = l.clear(e.From, fromSide, toAt, near)
	}
	var okFrom, okTo bool
	from, fromWho, okFrom = l.endOf(at, e.From, fromSide, toAt)
	from = l.sharedEnd(e, 0, from)
	angled := len(via) > 0 && !e.Straight
	switch {
	case !okFrom:
	case angled:
		pts, across = through([][2]float64{{from.x, from.y}}, via, horizontal(from.side) || from.side == "")
	case e.Straight:
		pts = slices.Concat([][2]float64{{from.x, from.y}}, via)
	}
	if toSide == "" {
		near := fromAt
		if len(via) > 0 {
			near = via[len(via)-1]
			toSide = l.facing(e.To, near, across, angled && okFrom)
		}
		toSide = l.clear(e.To, toSide, fromAt, near)
	}
	// Resolved even when from is not, so that both are told.
	to, toWho, okTo = l.endOf(at, e.To, toSide, fromAt)
	to = l.sharedEnd(e, 1, to)
	return from, to, pts, across, fromWho, toWho, okFrom && okTo
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

// edgeWho is what an edge is called in a mistake and by inspect, from what its ends are.
func edgeWho(from, to string) string {
	return "edge " + from + "→" + to
}

func (l *layout) addEdge(e *edge, at string) {
	pts, free, fromWho, toWho, ok := l.edgePoints(e, at)
	if !ok {
		return
	}
	segs := segsOf(pts)
	who := e.who
	if who == "" {
		who = edgeWho(fromWho, toWho)
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
	start, end := markerKinds(e)
	drawn := l.atDots(e, pts, start, end)
	el := elem{Kind: "path", D: pathData(drawn), Class: cls, Tone: e.Tone, Start: l.marker(start), End: l.marker(end)}
	if free.ok && e.graph == "" {
		l.loose = append(l.loose, loosePath{
			loose: free, e: e, pts: pts, path: len(l.paths), el: len(l.edgeEls),
			stroke: len(l.strokes), labelled: e.Label != nil && e.At == nil,
		})
	}
	l.edgeEls = append(l.edgeEls, el)
	p := path{
		segs: segs, what: who, at: at, own: -1, crossing: e.Crossing, graph: e.graph,
		ends: [2]string{e.From.Name, e.To.Name},
	}
	if e.Label != nil {
		x, y := middle(segs)
		if e.At != nil {
			x, y = e.At[0], e.At[1]
		}
		// A loop's label stands beside its line, so on nothing to hide and with its own line checked against it.
		// LIMIT: a graph's label is not checked against a frame written by hand (offEdge), and its ground cuts the
		// frame's line; tell it if writers come to miss that.
		anchor := "middle"
		if e.beside {
			anchor = "start"
		}
		i := l.label(at, e.Label, x, y, anchor, e.beside, false)
		l.labels[i].graph, l.labels[i].offEdge = e.graph, e.At != nil && e.graph == ""
		if !e.beside {
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
		if n := segLength(s); n > longest {
			longest, best = n, s
		}
	}
	return (best[0] + best[2]) / 2, (best[1] + best[3]) / 2
}

// markerKinds are the kinds of e's markers at its start and its end.
func markerKinds(e *edge) (start, end string) {
	start, end = "none", "arrow"
	if e.Start != nil {
		start = *e.Start
	}
	if e.End != nil {
		end = *e.End
	}
	return start, end
}

// markLength is how long a marker of kind is along its run; 0 for none.
func markLength(kind string) float64 {
	for _, m := range markers {
		if m.Kind == kind {
			return m.W
		}
	}
	return 0
}

// longestMark is the length of the longest marker.
func longestMark() float64 {
	longest := 0.0
	for _, m := range markers {
		longest = max(longest, m.W)
	}
	return longest
}

// loose is a run of an edge that floe places, not where it is written: the run from pts[i] to pts[i+1], across (at
// one y) or down (at one x), which may move within lo to hi; ok false for none.
type loose struct {
	i      int
	across bool
	lo, hi float64
	ok     bool
}

// noLoose is no loose run: every run where the ends and via put it.
var noLoose = loose{i: 0, across: false, lo: 0, hi: 0, ok: false}

// between is the run at i between two ends at a and b, kept within them by the length of their markers.
func between(i int, across bool, a, b float64, marks [2]float64) loose {
	if a <= b {
		return loose{i: i, across: across, lo: a + marks[0], hi: b - marks[1], ok: true}
	}
	return loose{i: i, across: across, lo: b + marks[1], hi: a - marks[0], ok: true}
}

// loosePath is an edge with a loose run, as keepOff moves it: its points as routed, its path, drawn element and span.
type loosePath struct {
	loose
	e                *edge
	pts              [][2]float64
	path, el, stroke int
	labelled         bool // its label stands in the middle of its longest run
}

// ruleGap is how far a loose run keeps off a rule beside it, and the step it moves by.
const ruleGap = 4

// arrowRun is the room an arrow takes: an icon keeps it between its side and the end of an edge moved onto it
// (iconCell, iconReach), and an edge straightened between two sides (straightenEdge) runs at least it.
const arrowRun = 8.0

// rule is where a loose run across (or down) keeps out of: lo < y < hi (x) where it is beside from to to in x (y).
type rule struct {
	across   bool
	lo, hi   float64
	from, to float64
}

// keepOff moves the loose runs of the edges written by hand off the rules. Edges whose loose runs lie on one line
// and leave one point or reach one, or one side of a part (edges out of a part or into one, sharing the run), move as
// one. They move by steps
// of ruleGap to the nearest place within all their bounds (the smaller of two as near) where each edge stays on the
// canvas, runs through no part or text and crosses no edge (as the checks tell), and what is new of it keeps ruleGap
// off every rule, part's or frame's edge and other edge's run along it; where there is none, they stay. Edges whose
// label would stand on another run, the longest run changed, do not move there. The checks read the moved runs.
// Edges are no rules, but an edge does not move along another.
// LIMIT: an edge whose label's run would change at every place stays on the rule, and graph's and tree's edges are not
// moved; move them too if they come to matter.
func (l *layout) keepOff() {
	done := make([]bool, len(l.loose))
	for i := range l.loose {
		if done[i] {
			continue
		}
		group := l.sharing(i, done)
		bands := l.beside(group)
		for _, w := range l.places(group, bands) {
			moved, ok := l.movedTo(group, w)
			if !ok || !l.fitAll(group, moved, bands) {
				continue
			}
			for k, q := range moved {
				m := l.loose[group[k]]
				l.paths[m.path] = q.p
				l.strokes[m.stroke] = span(q.p.segs, q.p.what, q.p.at)
				start, end := markerKinds(m.e)
				l.edgeEls[m.el].D = pathData(l.atDots(m.e, q.pts, start, end))
			}
			break
		}
	}
}

// sharing are the loose edges from i on whose runs lie on i's line and leave or reach a point or a side of a part one
// of them does, marked done.
func (l *layout) sharing(i int, done []bool) []int {
	group := []int{i}
	done[i] = true
	for grew := true; grew; {
		grew = false
		for j := i + 1; j < len(l.loose); j++ {
			if done[j] {
				continue
			}
			for _, g := range group {
				if a, b := l.loose[g], l.loose[j]; a.across == b.across && a.at() == b.at() &&
					(a.pts[0] == b.pts[0] || a.pts[len(a.pts)-1] == b.pts[len(b.pts)-1] ||
						l.oneSide(a.e, b.e)) {
					group, done[j], grew = append(group, j), true, true
					break
				}
			}
		}
	}
	return group
}

// at is where m's loose run is: its y across, its x down.
func (m loosePath) at() float64 {
	if m.across {
		return m.pts[m.i][1]
	}
	return m.pts[m.i][0]
}

// beside are the bands what is new of group's moved edges keeps out of: the rules, and ruleGap about the parts'
// and frames' edges, the dots and the runs of the edges not in group.
// LIMIT: raw SVG is not measured, so an edge may move onto what it draws.
func (l *layout) beside(group []int) []rule {
	out := slices.Clone(l.rules)
	for _, r := range slices.Concat(l.boxes, l.frames) {
		for _, s := range []seg{
			{r.x, r.y, r.x + r.w, r.y},
			{r.x, r.y + r.h, r.x + r.w, r.y + r.h},
			{r.x, r.y, r.x, r.y + r.h},
			{r.x + r.w, r.y, r.x + r.w, r.y + r.h},
		} {
			out = append(out, bandOf(s, ruleGap))
		}
	}
	// A dot is kept off all of it, as an edge on its centre looks to end at it.
	for _, r := range l.marks {
		out = append(out, rule{across: true, lo: r.y - ruleGap, hi: r.y + r.h + ruleGap, from: r.x, to: r.x + r.w},
			rule{across: false, lo: r.x - ruleGap, hi: r.x + r.w + ruleGap, from: r.y, to: r.y + r.h})
	}
	for j, p := range l.paths {
		if slices.ContainsFunc(group, func(g int) bool { return l.loose[g].path == j }) {
			continue
		}
		for _, s := range p.segs {
			if s[0] == s[2] || s[1] == s[3] {
				out = append(out, bandOf(s, ruleGap))
			}
		}
	}
	return out
}

// bandOf is the band about s, a run across or down: ruleGap before it (above, left), after past it.
func bandOf(s seg, after float64) rule {
	before := float64(ruleGap)
	if s[1] == s[3] {
		return rule{across: true, lo: s[1] - before, hi: s[1] + after, from: min(s[0], s[2]), to: max(s[0], s[2])}
	}
	return rule{across: false, lo: s[0] - before, hi: s[0] + after, from: min(s[1], s[3]), to: max(s[1], s[3])}
}

// places are where the loose runs of group may go when a rule holds them, none when they are clear: just outside
// each band along them, beside them where they run, within all their bounds, nearest first (the smaller of two as
// near).
func (l *layout) places(group []int, bands []rule) []float64 {
	first := l.loose[group[0]]
	v, across := first.at(), first.across
	lo, hi, o0, o1 := math.Inf(-1), math.Inf(1), math.Inf(1), math.Inf(-1)
	held := false
	for _, g := range group {
		m := l.loose[g]
		s := seg{m.pts[m.i][0], m.pts[m.i][1], m.pts[m.i+1][0], m.pts[m.i+1][1]}
		lo, hi = max(lo, m.lo), min(hi, m.hi)
		held = held || inBand(l.rules, s)
		_, a, b := runOf(s)
		o0, o1 = min(o0, a), max(o1, b)
	}
	if !held {
		return nil
	}
	var out []float64
	for _, u := range bands {
		if u.across != across || u.from >= o1 || o0 >= u.to {
			continue
		}
		for _, w := range []float64{u.lo, u.hi} {
			if w != v && w > lo && w < hi && !slices.Contains(out, w) {
				out = append(out, w)
			}
		}
	}
	slices.SortFunc(out, func(a, b float64) int {
		return cmp.Or(cmp.Compare(math.Abs(a-v), math.Abs(b-v)), cmp.Compare(a, b))
	})
	return out
}

// runOf is where s, a run across or down, is (its y, its x) and from where to where it runs along.
func runOf(s seg) (at, a, b float64) {
	if s[1] == s[3] {
		return s[1], min(s[0], s[2]), max(s[0], s[2])
	}
	return s[0], min(s[1], s[3]), max(s[1], s[3])
}

// inBand reports whether s, a run across or down, lies in a band along it.
func inBand(bands []rule, s seg) bool {
	if s[0] != s[2] && s[1] != s[3] {
		return false
	}
	across := s[1] == s[3]
	w, o0, o1 := runOf(s)
	return slices.ContainsFunc(bands, func(u rule) bool {
		return u.across == across && u.lo < w && w < u.hi && u.from < o1 && o0 < u.to
	})
}

// movedPath is a loose edge with its loose run moved: its points and its path.
type movedPath struct {
	pts [][2]float64
	p   path
}

// movedTo is each edge of group with its loose run at w; false when one has its label and its longest run changes.
func (l *layout) movedTo(group []int, w float64) ([]movedPath, bool) {
	out := make([]movedPath, 0, len(group))
	for _, g := range group {
		m := l.loose[g]
		pts := slices.Clone(m.pts)
		c := 0
		if m.across {
			c = 1
		}
		pts[m.i][c], pts[m.i+1][c] = w, w
		q := l.paths[m.path]
		q.segs = segsOf(pts)
		x0, y0 := middle(l.paths[m.path].segs)
		if x1, y1 := middle(q.segs); m.labelled && (x1 != x0 || y1 != y0) {
			return nil, false
		}
		out = append(out, movedPath{pts: pts, p: q})
	}
	return out, true
}

// fitAll reports whether every path of moved, group's edges moved, is as the checks take it with no mistake, the
// others of moved as moved, and what is new of it lies in none of bands.
// LIMIT: each place tried compares the edges moved with every part, text and edge (places × edges × runs); compare
// only those near them if diagrams of hundreds of edges on rules get slow.
func (l *layout) fitAll(group []int, moved []movedPath, bands []rule) bool {
	all := slices.Clone(l.paths)
	for k, q := range moved {
		all[l.loose[group[k]].path] = q.p
	}
	canvas := l.canvas()
	for k, q := range moved {
		i := l.loose[group[k]].path
		if !span(q.p.segs, "", "").inside(canvas) || len(l.throughs(q.p)) > 0 {
			return false
		}
		for j, o := range all {
			if j != i && crosses(q.p, o) {
				return false
			}
		}
		for _, s := range newRuns(q.p.segs, l.paths[i].segs) {
			if inBand(bands, s) {
				return false
			}
		}
	}
	return true
}

// newRuns are the stretches of segs, runs across or down, that no run of was along them holds.
func newRuns(segs, was []seg) []seg {
	var out []seg
	for _, s := range segs {
		across := s[1] == s[3]
		a, b := min(s[0], s[2]), max(s[0], s[2])
		if !across {
			a, b = min(s[1], s[3]), max(s[1], s[3])
		}
		left := [][2]float64{{a, b}}
		for _, o := range was {
			oa, ob := min(o[0], o[2]), max(o[0], o[2])
			on := across && o[1] == s[1] && o[3] == s[1]
			if !across {
				oa, ob = min(o[1], o[3]), max(o[1], o[3])
				on = o[0] == s[0] && o[2] == s[0]
			}
			if on {
				left = cut(left, oa, ob)
			}
		}
		for _, r := range left {
			if across {
				out = append(out, seg{r[0], s[1], r[1], s[1]})
			} else {
				out = append(out, seg{s[0], r[0], s[0], r[1]})
			}
		}
	}
	return out
}

// cut is spans with a to b cut out of them.
func cut(spans [][2]float64, a, b float64) [][2]float64 {
	var out [][2]float64
	for _, r := range spans {
		if r[0] < a {
			out = append(out, [2]float64{r[0], min(r[1], a)})
		}
		if r[1] > b {
			out = append(out, [2]float64{max(r[0], b), r[1]})
		}
	}
	return out
}
