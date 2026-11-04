package diagram

import (
	"cmp"
	"math"
	"slices"
)

// The ends of the edges written by hand on one side of a part stand at the points that part the side into equal
// parts. Which side an end takes is known only from the edge, and where an end stands on it only from all the ends
// there, so shareEnds places them all once before any edge is added; edgePoints reads where they stand (sharedEnd).

// handEnd names an end of an edge written by hand: 0 for from, 1 for to.
type handEnd struct {
	e   *edge
	end int
}

// shared is where an end stands: the side, and an x on the top or bottom, a y on the left or right.
type shared struct {
	side string
	at   float64
}

// partSide is a side of the part of a name.
type partSide struct{ name, side string }

// sideEnd is an end on a side while the side is parted. way is how its edge first turns along the side, -1 towards
// less, 1 towards more, 0 for none; out how far it runs from the side first; other the other end's centre along the
// side. held is whether it stands at at whatever else stands on the side.
type sideEnd struct {
	handEnd
	on    partSide
	way   int
	out   float64
	other float64
	held  bool
	at    float64
}

// sharing is what shareEnds reads: all the edges, each edge's points with its ends in the middle of their sides, and
// what a moved edge keeps off that it kept off there.
type sharing struct {
	l      *layout
	all    []*edge
	before map[*edge][][2]float64
	solid  []rect
	labels []rect
}

// shareEnds places the ends of the edges written by hand on their sides: the n ends on a side at the points that part
// it into n+1 equal parts, but for an edge running straight, which stays straight (hold), and the rest evenly between.
// It leaves out, and so in the middle of their sides as they would be alone, edges with via or a point (written where
// they are), straight ones, ones back to the side they leave (told as going nowhere), and ends at a dot, a row or a
// bar. An edge that this would make cross an edge, run through a box or a graph's label, or come to nothing goes back
// to the middle too, so that the writer has no end of it to answer for, and a graph's hands stay what it kept clear.
// LIMIT: labels written by hand are not compared, as their writer can move them: an edge that comes to run through one
// is told. An edge's own label is not compared either.
// LIMIT: each edge moved is compared with all edges, so that this is quadratic in the edges, again for each round that
// sends some back; a diagram of thousands of edges written by hand and sharing sides slows here. Compare by area if so.
func (l *layout) shareEnds(items []item) {
	s := sharing{l: l, all: nil, before: map[*edge][][2]float64{}, solid: nil, labels: nil}
	var moving []*edge
	told := len(l.problems)
	for _, it := range items {
		s.note(it)
		if e := it.p.Edge; e != nil {
			pts, _, _, _, ok := l.edgePoints(e, it.at)
			if ok {
				s.all, s.before[e] = append(s.all, e), pts
			}
			if ok && l.moves(e) {
				moving = append(moving, e)
			}
		}
	}
	l.problems = l.problems[:told] // told when the edges are added
	for len(moving) > 0 {
		s.place(moving)
		back := s.troubled(moving)
		if len(back) == 0 {
			break
		}
		l.shares = nil
		moving = slices.DeleteFunc(moving, func(e *edge) bool { return back[e] })
	}
}

// note keeps what an edge keeps off in the part of it: a box, a table, an icon with its label or a bar, or a graph's
// label on its edge.
func (s *sharing) note(it item) {
	p := it.p
	switch {
	case p.Box != nil:
		s.solid = append(s.solid, boxRect(p.Box, it.at))
	case p.Table != nil:
		r, _ := tableRects(p.Table, it.at)
		s.solid = append(s.solid, r)
	case p.Bar != nil:
		s.solid = append(s.solid, barRect(p.Bar, it.at))
	case p.Icon != nil:
		r := iconRect(p.Icon, it.at)
		s.solid = append(s.solid, r)
		if w := iconLabelW(p.Icon); w > 0 {
			//nolint:exhaustruct_v5 // an area
			s.solid = append(s.solid, rect{x: r.x + r.w/2 - w/2, y: r.y + r.h, w: w, h: iconUnder(p.Icon), what: "", at: it.at})
		}
	case p.Edge != nil && p.Edge.graph != "" && p.Edge.Label != nil && p.Edge.At != nil:
		e := p.Edge
		s.labels = append(s.labels, labelArea(e.Label, e.At[0], e.At[1], e.anchor, false, it.at))
	}
}

// moves reports whether e, of ends that can be, is one shareEnds places.
func (l *layout) moves(e *edge) bool {
	if e.graph != "" || e.From.Point != nil || e.To.Point != nil || len(e.Via) > 0 || e.Straight {
		return false
	}
	from, to := l.edgeSides(e)
	return e.From.Name != e.To.Name || from != to
}

// edgeSides are the sides the ends of e, an edge with no via between two names, take.
func (l *layout) edgeSides(e *edge) (from, to string) {
	side := func(n end, written string, other [2]float64) string {
		if written != "" {
			return written
		}
		return cmp.Or(l.clear(n, "", other, other), onSide(l.names[n.Name], "", other).side)
	}
	return side(e.From, e.FromSide, l.centre(e.To)), side(e.To, e.ToSide, l.centre(e.From))
}

// sharedEnd is a, end j of e, where shareEnds places it.
func (l *layout) sharedEnd(e *edge, j int, a anchor) anchor {
	at, ok := l.shares[handEnd{e, j}]
	if !ok {
		return a
	}
	name := e.From.Name
	if j == 1 {
		name = e.To.Name
	}
	p := sidePoint(l.names[name], at.side, at.at)
	return anchor{p[0], p[1], at.side, false}
}

// oneSide reports whether end j of a and b stand on one side of one part, placed by shareEnds.
func (l *layout) oneSide(a, b *edge, j int) bool {
	x, okX := l.shares[handEnd{a, j}]
	y, okY := l.shares[handEnd{b, j}]
	if !okX || !okY || x.side != y.side {
		return false
	}
	if j == 0 {
		return a.From.Name == b.From.Name
	}
	return a.To.Name == b.To.Name
}

// place sets where the ends of moving stand.
func (s *sharing) place(moving []*edge) {
	on := map[partSide][]*sideEnd{}
	var sides []partSide
	ends := map[handEnd]*sideEnd{}
	for _, e := range moving {
		from, to := s.l.edgeSides(e)
		for j, side := range [2]string{from, to} {
			name, other := e.From.Name, s.l.centre(e.To)
			if j == 1 {
				name, other = e.To.Name, s.l.centre(e.From)
			}
			if !parted(s.l.names[name].kind) {
				continue
			}
			k := partSide{name, side}
			if _, seen := on[k]; !seen {
				sides = append(sides, k)
			}
			x := s.sideEndOf(e, j, k, other)
			on[k] = append(on[k], x)
			ends[x.handEnd] = x
		}
	}
	for _, k := range sides {
		slices.SortStableFunc(on[k], leavingOrder)
	}
	for _, e := range moving {
		s.hold(e, ends[handEnd{e, 0}], ends[handEnd{e, 1}], on)
	}
	s.l.shares = map[handEnd]shared{}
	for _, k := range sides {
		s.l.spread(k, on[k])
	}
	for _, e := range moving {
		s.meet(e, ends[handEnd{e, 0}], ends[handEnd{e, 1}], on)
	}
}

// parted reports whether the ends at a part of kind share its sides: a dot has no side, and a row and a bar are too
// thin for two ends to read as two.
func parted(kind string) bool {
	return kind == "box" || kind == "table" || kind == "icon" || kind == "frame"
}

// sideEndOf is end j of e on side k, how its edge first turns read from its points with its ends in the middle.
func (s *sharing) sideEndOf(e *edge, j int, k partSide, other [2]float64) *sideEnd {
	axis := sideAxis(k.side)
	pts := slices.Clone(s.before[e])
	if j == 1 {
		slices.Reverse(pts)
	}
	x := &sideEnd{handEnd: handEnd{e, j}, on: k, way: 0, out: 0, other: other[axis], held: false, at: 0}
	for i := 1; i < len(pts); i++ {
		if d := pts[i][axis] - pts[i-1][axis]; d != 0 {
			x.way = cmp.Compare(d, 0)
			x.out = math.Abs(pts[i-1][1-axis] - pts[0][1-axis])
			break
		}
	}
	return x
}

// leavingOrder orders the ends on a side along it so that no two edges cross between the side and their first turns:
// those turning towards less first, the sooner the turn the nearer the start of the side, then those that do not turn,
// then those turning towards more, the sooner the turn the nearer the end of the side.
func leavingOrder(a, b *sideEnd) int {
	if a.way != b.way {
		return cmp.Compare(a.way, b.way)
	}
	out := cmp.Compare(a.out, b.out)
	if a.way > 0 {
		out = -out
	}
	return cmp.Or(out, cmp.Compare(a.other, b.other))
}

// parts is where x would stand by the equal parts of its side.
func (s *sharing) parts(x *sideEnd, on map[partSide][]*sideEnd) float64 {
	lo, size := s.span(x.on)
	return lo + size*float64(slices.Index(on[x.on], x)+1)/float64(len(on[x.on])+1)
}

// span is where side k starts, and how long it is.
func (s *sharing) span(k partSide) (lo, size float64) {
	r := s.l.names[k.name].r
	axis := sideAxis(k.side)
	return r.lo(axis), r.size(axis)
}

// takes reports whether x's side takes an end at at: as far in from its ends as a graph's edges stand.
func (s *sharing) takes(x *sideEnd, at float64) bool {
	lo, size := s.span(x.on)
	return at >= lo+inEnd && at <= lo+size-inEnd
}

// hold keeps e straight when it runs straight between its ends in the middle of their sides, a and b (nil for one not
// parted), holding them at one line.
func (s *sharing) hold(e *edge, a, b *sideEnd, on map[partSide][]*sideEnd) {
	one := a
	if one == nil {
		one = b
	}
	if one == nil {
		return
	}
	pts, axis := s.before[e], sideAxis(one.on.side)
	if len(pts) != 2 || !near(pts[0][axis], pts[1][axis]) || a != nil && b != nil && !opposite(a.on.side, b.on.side) {
		return
	}
	line := pts[0][axis]
	if a != nil && b != nil {
		line = s.lineOf(a, b, on, line)
	}
	for _, x := range []*sideEnd{a, b} {
		if x != nil {
			x.held, x.at = true, line
		}
	}
}

// lineOf is the line the straight edge between a and b holds: the point of the side with more ends, else of the other,
// when both sides take it, else the line it runs on.
func (s *sharing) lineOf(a, b *sideEnd, on map[partSide][]*sideEnd, line float64) float64 {
	first, second := a, b
	if len(on[b.on]) > len(on[a.on]) {
		first, second = b, a
	}
	for _, x := range []*sideEnd{first, second} {
		if at := s.parts(x, on); s.takes(a, at) && s.takes(b, at) {
			return at
		}
	}
	return line
}

// opposite reports whether two sides face one another's way: an edge running straight between them runs across both.
func opposite(a, b string) bool {
	return a != b && sideAxis(a) == sideAxis(b)
}

// spread places the ends of side k, in their order: those held at their lines, in the order of their lines, and the
// rest evenly between those and the ends of the side.
func (l *layout) spread(k partSide, ends []*sideEnd) {
	var held []*sideEnd
	for _, x := range ends {
		if x.held {
			held = append(held, x)
		}
	}
	slices.SortStableFunc(held, func(a, b *sideEnd) int { return cmp.Compare(a.at, b.at) })
	next := 0
	for i, x := range ends {
		if x.held {
			ends[i] = held[next]
			next++
		}
	}
	r := l.names[k.name].r
	axis := sideAxis(k.side)
	places := evenly(r.lo(axis), r.lo(axis)+r.size(axis), len(ends), func(i int) (float64, bool) {
		return ends[i].at, ends[i].held
	})
	for i, x := range ends {
		l.shares[x.handEnd] = shared{k.side, places[i]}
	}
}

// evenly are n places from lo to hi: each held one where it is held, and the rest, in order, at equal steps between
// the ones held and lo and hi. With none held they part lo to hi into n+1 equal parts.
func evenly(lo, hi float64, n int, held func(i int) (float64, bool)) []float64 {
	out := make([]float64, n)
	from, start := 0, lo
	for i := 0; i <= n; i++ {
		to, stop := hi, i == n
		if i < n {
			to, stop = held(i)
		}
		if !stop {
			continue
		}
		for j := from; j < i; j++ {
			out[j] = start + (to-start)*float64(j-from+1)/float64(i-from+1)
		}
		if i < n {
			out[i] = to
		}
		from, start = i+1, to
	}
	return out
}

// meet stands the one end on its side of e, which turns between facing sides, at the other's point, when its side takes
// it and nothing stands on the way: the edge then runs straight.
func (s *sharing) meet(e *edge, a, b *sideEnd, on map[partSide][]*sideEnd) {
	if a == nil || b == nil || a.held || !opposite(a.on.side, b.on.side) {
		return
	}
	one, other := a, b
	if len(on[a.on]) != 1 {
		one, other = b, a
	}
	if len(on[one.on]) != 1 || len(on[other.on]) == 1 {
		return
	}
	at := s.l.shares[other.handEnd].at
	if !s.takes(one, at) {
		return
	}
	was := s.l.shares[one.handEnd]
	s.l.shares[one.handEnd] = shared{was.side, at}
	pts, _, _, _, ok := s.l.edgePoints(e, "")
	if !ok || runsThrough(segsOf(pts), s.solid) && !runsThrough(segsOf(s.before[e]), s.solid) {
		s.l.shares[one.handEnd] = was
	}
}

// runsThrough reports whether a run of segs passes through one of rs.
func runsThrough(segs []seg, rs []rect) bool {
	return slices.ContainsFunc(segs, func(q seg) bool {
		return slices.ContainsFunc(rs, func(r rect) bool { return crossedBy(q, r) })
	})
}

// troubled are the edges of moving that where they now stand cross an edge, or run through a box or a graph's label,
// where with their ends in the middle they did not, or come to nothing.
func (s *sharing) troubled(moving []*edge) map[*edge]bool {
	now := map[*edge][]seg{}
	for _, e := range s.all {
		pts, _, _, _, ok := s.l.edgePoints(e, "")
		if ok {
			now[e] = segsOf(pts)
		}
	}
	line := func(e *edge, segs []seg) path {
		return path{segs: segs, what: "", at: "", own: -1, crossing: e.Crossing, graph: e.graph}
	}
	back := map[*edge]bool{}
	for _, a := range moving {
		was := segsOf(s.before[a])
		if len(now[a]) == 0 ||
			runsThrough(now[a], s.solid) && !runsThrough(was, s.solid) ||
			runsThrough(now[a], s.labels) && !runsThrough(was, s.labels) {
			back[a] = true
			continue
		}
		for _, b := range s.all {
			if a != b && crosses(line(a, now[a]), line(b, now[b])) && !crosses(line(a, was), line(b, segsOf(s.before[b]))) {
				back[a] = true
				break
			}
		}
	}
	return back
}
