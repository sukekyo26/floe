package diagram

import (
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

// route is the points an edge runs through, from a to b by via: straight from point to point, or right-angled.
// LIMIT: two turns at most, and no way round what is in the way; via or a side gives another.
func route(a, b anchor, via [][2]float64, straight bool) [][2]float64 {
	start, last := [][2]float64{{a.x, a.y}}, [2]float64{b.x, b.y}
	switch {
	case straight:
		return slices.Concat(start, via, [][2]float64{last})
	case len(via) == 0:
		return slices.Concat(start, turns(a, b), [][2]float64{last})
	default:
		return through(start, slices.Concat(via, [][2]float64{last}), horizontal(a.side) || a.side == "")
	}
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
// across first when across.
func through(pts, next [][2]float64, across bool) [][2]float64 {
	for _, q := range next {
		p := pts[len(pts)-1]
		switch {
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
	return pts
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

func (l *layout) addEdge(e *edge, at string) {
	from, fromWho, okFrom := l.endOf(at, e.From, e.FromSide, l.centre(e.To))
	to, toWho, okTo := l.endOf(at, e.To, e.ToSide, l.centre(e.From))
	if !okFrom || !okTo {
		return
	}
	pts := route(from, to, e.Via, e.Straight)
	segs := make([]seg, 0, len(pts)-1)
	for i := 1; i < len(pts); i++ {
		if pts[i] != pts[i-1] {
			segs = append(segs, seg{pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]})
		}
	}
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
	cls += tone(e.Tone)
	start, end := "none", "arrow"
	if e.Start != nil {
		start = *e.Start
	}
	if e.End != nil {
		end = *e.End
	}
	el := elem{Kind: "path", D: pathData(pts), Class: cls, Start: l.marker(start), End: l.marker(end)}
	l.edgeEls = append(l.edgeEls, el)
	p := path{segs: segs, what: who, at: at, own: -1, crossing: e.Crossing}
	if e.Label != nil {
		x, y := middle(segs)
		if e.At != nil {
			x, y = e.At[0], e.At[1]
		}
		p.own = l.label(at, e.Label, x, y, "middle", false, false)
	}
	l.paths = append(l.paths, p)
	if len(segs) > 0 {
		l.strokes = append(l.strokes, span(segs, who, at))
	}
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
