package diagram

import (
	"cmp"
	"maps"
	"math"
	"slices"
)

// An icon of a graph is laid out as an unseen rectangle round the icon and its label, and only when all is placed do
// its edges' ends move onto the icon (settleIcons), so that what chooses ranks, routes, loops, level edges and labels
// knows no icons. The rectangle keeps iconRing along the ranks, where an edge's last run needs its arrow's length, and
// iconMove+iconArrow across them, where the icon moves onto a straight edge and keeps an arrow's length from the side.
const (
	iconRing  = 12.0
	iconMove  = 12.0
	iconArrow = 8.0
)

// iconCell is the size of the rectangle of icon i, right as laid out right.
func iconCell(i *icon, right bool) (w, h float64) {
	across, along := iconMove+iconArrow, iconRing
	if right {
		across, along = along, across
	}
	return onGrid(iconWidest(i)+2*across, 8, math.Ceil), iconSize(i) + iconUnder(i) + 2*along
}

// iconWidest is how wide icon i stands with its label.
func iconWidest(i *icon) float64 {
	if i.Label == nil {
		return iconSize(i)
	}
	return max(iconSize(i), labelWidth(i.Label, false))
}

// endAt names an end of an edge: its index, and 0 for its first point or 1 for its last.
type endAt struct{ edge, end int }

// iconEnd is where an end goes on an icon: the box of the icon, the side, and how far along the side it stands.
type iconEnd struct {
	box   int
	side  string
	share float64
}

// sideAxis is the axis along a side.
func sideAxis(side string) int {
	if side == "top" || side == "bottom" {
		return 0
	}
	return 1
}

// sideOf is the side of r that p, an end on r, is on.
func sideOf(r rect, p [2]float64) string {
	top, bottom := math.Abs(p[1]-r.y), math.Abs(p[1]-(r.y+r.h))
	left, right := math.Abs(p[0]-r.x), math.Abs(p[0]-(r.x+r.w))
	switch min(top, bottom, left, right) {
	case top:
		return "top"
	case bottom:
		return "bottom"
	case left:
		return "left"
	}
	return "right"
}

// settleIcons moves the ends of the edges at the group's icons onto the icons, labels placed and before stand
// measures the group, so that what moves stays within what it measures.
func (d *drawing) settleIcons(o *graphOut) {
	d.shareSides(o)
	edges := slices.Sorted(maps.Keys(d.routes))
	for _, k := range edges {
		d.straighten(o, k)
	}
	for _, k := range edges {
		d.slide(o, k)
	}
	for _, k := range edges {
		d.bend(o, k)
	}
}

// shareSides gives each end at an icon its place: the n ends on a side part it into n+1 equal parts, in the order they
// stand on the rectangle's side, the order wire left them in by what they reach, so that they do not cross by the side.
func (d *drawing) shareSides(o *graphOut) {
	type side struct {
		box  int
		name string
	}
	on := map[side][]endAt{}
	for k, pts := range d.routes {
		for j, p := range [2][2]float64{pts[0], pts[len(pts)-1]} {
			if i := d.ends[k][j]; i >= 0 && o.boxes[i].box.icon != nil {
				s := side{i, sideOf(d.boxes[i], p)}
				on[s] = append(on[s], endAt{k, j})
			}
		}
	}
	point := func(e endAt) [2]float64 {
		pts := d.routes[e.edge]
		return pts[e.end*(len(pts)-1)]
	}
	for s, ends := range on {
		axis := sideAxis(s.name)
		slices.SortFunc(ends, func(a, b endAt) int {
			return cmp.Or(cmp.Compare(point(a)[axis], point(b)[axis]), cmp.Compare(a.edge, b.edge), cmp.Compare(a.end, b.end))
		})
		for n, e := range ends {
			d.onIcon[e] = iconEnd{s.box, s.name, float64(n+1) / float64(len(ends)+1)}
		}
	}
}

// iconAt is where the icon of box i stands: in the middle of its rectangle, moved by what straighten moved it.
func (d *drawing) iconAt(o *graphOut, i int) rect {
	ic, r := o.boxes[i].box.icon, d.boxes[i]
	size := iconSize(ic)
	//nolint:exhaustruct_v5 // an area
	at := rect{x: r.x + (r.w-size)/2, y: r.y + (r.h-size-iconUnder(ic))/2, w: size, h: size, what: "", at: ""}
	across, _ := d.axes()
	at.set(across, at.lo(across)+d.moved[i], size)
	return at
}

// endPoint is where e stands on its icon. On the bottom side it stands under the label, or under the icon past the
// label's width: an end there would stand off from all it belongs to.
// LIMIT: the label's width is an estimate (units), so an end by its edge may touch the words. Measure if that shows.
func (d *drawing) endPoint(o *graphOut, e iconEnd) [2]float64 {
	ic, at := o.boxes[e.box].box.icon, d.iconAt(o, e.box)
	along := at.w * e.share
	switch e.side {
	case "top":
		return [2]float64{at.x + along, at.y}
	case "bottom":
		x, y := at.x+along, at.y+at.h
		if ic.Label != nil && math.Abs(along-at.w/2) <= labelWidth(ic.Label, false)/2 {
			y += iconUnder(ic)
		}
		return [2]float64{x, y}
	case "left":
		return [2]float64{at.x, at.y + along}
	}
	return [2]float64{at.x + at.w, at.y + along}
}

// iconReach is how far the icon of box i may move across the ranks from the middle of its rectangle, keeping its
// label within the rectangle and an arrow's length from its side: 12 or more.
func (d *drawing) iconReach(o *graphOut, i int) float64 {
	ic, r := o.boxes[i].box.icon, d.boxes[i]
	across, _ := d.axes()
	stands := iconWidest(ic)
	if across == 1 {
		stands = iconSize(ic) + iconUnder(ic)
	}
	return (r.size(across)-stands)/2 - iconArrow
}

// straighten stands a straight edge at an icon on one line: an icon not yet moved moves across the ranks, once, so that
// its point stands on the line; a box takes it anywhere along its side; a point that cannot move holds the line to it.
// LIMIT: once, and by iconReach at most, so a straight run through icons in a row may find the next short of it, and
// then bends. Solve a row of icons together if that shows.
func (d *drawing) straighten(o *graphOut, k int) {
	pts := d.routes[k]
	_, on0 := d.onIcon[endAt{k, 0}]
	_, on1 := d.onIcon[endAt{k, 1}]
	if len(pts) != 2 || d.loops[k] || !on0 && !on1 {
		return
	}
	axis := runAxis(segAt(pts, 0)) ^ 1
	across, _ := d.axes()
	lo, hi := math.Inf(-1), math.Inf(1)
	var movers []iconEnd
	for j := range 2 {
		i := d.ends[k][j]
		e, on := d.onIcon[endAt{k, j}]
		switch {
		case i < 0:
		case !on:
			b := d.boxes[i]
			lo, hi = max(lo, b.lo(axis)+inEnd), min(hi, b.lo(axis)+b.size(axis)-inEnd)
		case axis == across && !d.settled[i]:
			q, reach := d.endPoint(o, e)[axis], d.iconReach(o, i)
			lo, hi = max(lo, q-reach), min(hi, q+reach)
			movers = append(movers, e)
		default:
			q := d.endPoint(o, e)[axis]
			lo, hi = max(lo, q), min(hi, q)
		}
	}
	if lo > hi {
		return
	}
	line := max(lo, min(pts[0][axis], hi))
	if !d.moveRun(k, []int{0, 1}, axis, line) {
		return
	}
	for _, e := range movers {
		d.moved[e.box] += line - d.endPoint(o, e)[axis]
		d.settled[e.box] = true
	}
}

// slide moves the run from each end of edge k at an icon, when a run across follows it, onto the end's point, its label
// with it: the next run only grows or shrinks.
func (d *drawing) slide(o *graphOut, k int) {
	if _, level := d.level[k]; level || d.loops[k] || len(d.routes[k]) < 3 {
		return
	}
	last := len(d.routes[k]) - 1
	for j, run := range [2][2]int{{0, 1}, {last, last - 1}} {
		if e, on := d.onIcon[endAt{k, j}]; on {
			axis := sideAxis(e.side)
			d.moveRun(k, run[:], axis, d.endPoint(o, e)[axis])
		}
	}
}

// bend gives a straight edge with no label, short of its points, one bend in the middle of its run, when nothing stands
// there. A labelled one stays, the label beside it, and gathered bends it by its rectangles.
func (d *drawing) bend(o *graphOut, k int) {
	pts := d.routes[k]
	if len(pts) != 2 || d.loops[k] || d.labelled(k) {
		return
	}
	axis := runAxis(segAt(pts, 0)) ^ 1
	a, b := pts[0], pts[1]
	for j, p := range []*[2]float64{&a, &b} {
		if e, on := d.onIcon[endAt{k, j}]; on {
			p[axis] = d.endPoint(o, e)[axis]
		}
	}
	if a == pts[0] && b == pts[1] {
		return
	}
	bent := [][2]float64{a, b}
	if a[axis] != b[axis] {
		m, n := a, b
		m[1-axis] = (a[1-axis] + b[1-axis]) / 2
		n[1-axis] = m[1-axis]
		bent = [][2]float64{a, m, n, b}
	}
	if d.clear(k, segsOf(bent)) {
		d.routes[k] = bent
	}
}

// moveRun moves points at of edge k to q on axis, unless that lands on something, and reports whether it did.
func (d *drawing) moveRun(k int, at []int, axis int, q float64) bool {
	pts := d.routes[k]
	was := make([][2]float64, len(at))
	for j, i := range at {
		was[j] = pts[i]
		pts[i][axis] = q
	}
	var runs []seg
	for _, i := range at {
		if i > 0 {
			runs = append(runs, segAt(pts, i-1))
		}
		if i+1 < len(pts) {
			runs = append(runs, segAt(pts, i))
		}
	}
	if d.clear(k, runs) && d.labelsFit(k) {
		return true
	}
	for j, i := range at {
		pts[i] = was[j]
	}
	return false
}

// clear is whether runs, of edge k, keep off the boxes, the headings, the other edges' labels and what lies between
// them and their runs, and the other edges' runs: wire leaves no two edges touching, and check does not compare two
// edges of a graph, so a run landing on another would be drawn unsaid.
// LIMIT: only runs moved here are compared, so edges may touch where gathered bends them by a rectangle's side. Part
// them along the side if that shows.
func (d *drawing) clear(k int, runs []seg) bool {
	things := slices.Collect(maps.Values(d.boxes))
	for c, head := range d.heads {
		things = append(things, headRect(d.frames[c], head))
	}
	for e, sp := range d.spots {
		if e != k && sp.edge != k {
			things = append(things, heldRect(d.runOf(sp), sp))
		}
	}
	for _, s := range runs {
		if slices.ContainsFunc(things, func(r rect) bool { return crossedBy(s, r) }) {
			return false
		}
		for e, pts := range d.routes {
			if e != k && slices.ContainsFunc(segsOf(pts), func(q seg) bool { return runsTouch(s, q) }) {
				return false
			}
		}
	}
	return true
}

// labelsFit is whether the labels on edge k stand within their runs and clear, as they stand now.
func (d *drawing) labelsFit(k int) bool {
	for e, sp := range d.spots {
		if e != k && sp.edge != k {
			continue
		}
		r, _ := d.rect(e)
		run := d.runOf(sp)
		along := runAxis(run)
		if r.lo(along) < min(run[along], run[along+2]) || r.lo(along)+r.size(along) > max(run[along], run[along+2]) ||
			!free(heldRect(run, sp), d.held(k), append(d.runs(), d.hands()...)) {
			return false
		}
	}
	return true
}

// labelled is whether a label stands on edge k.
func (d *drawing) labelled(k int) bool {
	if _, ok := d.spots[k]; ok {
		return true
	}
	return slices.ContainsFunc(slices.Collect(maps.Values(d.spots)), func(sp spot) bool { return sp.edge == k })
}

// runsTouch is whether two runs share a stretch, or one ends on the other.
func runsTouch(s, q seg) bool {
	down, qDown := s[0] == s[2], q[0] == q[2]
	if down == qDown {
		along := 0
		if down {
			along = 1
		}
		return s[1-along] == q[1-along] && max(min(s[along], s[along+2]), min(q[along], q[along+2])) <=
			min(max(s[along], s[along+2]), max(q[along], q[along+2]))
	}
	v, h := s, q
	if !down {
		v, h = q, s
	}
	x, y := v[0], h[1]
	if y < min(v[1], v[3]) || y > max(v[1], v[3]) || x < min(h[0], h[2]) || x > max(h[0], h[2]) {
		return false
	}
	return y == v[1] || y == v[3] || x == h[0] || x == h[2]
}

// gathered is edge k's points as drawn: with each end at an icon reaching its point, by a bend along the rectangle's
// side if the end stands short of the point's line, and the run from the side to the icon.
// LIMIT: two ends bent along one side may share a stretch of it, and take the look of the edge drawn last there.
func (d *drawing) gathered(o *graphOut, k int) [][2]float64 {
	reach := func(e iconEnd, pts [][2]float64) [][2]float64 {
		at, axis := d.endPoint(o, e), sideAxis(e.side)
		if pts[0][axis] == at[axis] {
			return append([][2]float64{at}, pts...)
		}
		turn := pts[0]
		turn[axis] = at[axis]
		return append([][2]float64{at, turn}, pts...)
	}
	out := slices.Clone(d.routes[k])
	if e, on := d.onIcon[endAt{k, 0}]; on {
		out = reach(e, out)
	}
	if e, on := d.onIcon[endAt{k, 1}]; on {
		slices.Reverse(out)
		out = reach(e, out)
		slices.Reverse(out)
	}
	return out
}
