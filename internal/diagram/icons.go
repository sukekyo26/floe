package diagram

import (
	"cmp"
	"maps"
	"math"
	"slices"
)

// An icon of a graph is laid out as an unseen rectangle round the icon and its label, and only when all is placed do
// its edges' ends move onto the icon (settleIcons), so that what chooses ranks, routes, loops, level edges and labels
// knows no icons. Along the ranks it keeps iconRing past the icon and its label, where an edge's last run needs its
// arrow's length; across them iconMove+iconArrow, where the icon moves onto a straight edge and keeps an arrow's length
// from the side. Laid out down the icon stands in its middle, a label's room above it too, so that the icons of a rank
// stand level with one another, labelled or not, and with the boxes.
const (
	iconRing  = 12.0
	iconMove  = 12.0
	iconArrow = 8.0
)

// iconCell is the size of the rectangle of icon i, right as laid out right.
func iconCell(i *icon, right bool) (w, h float64) {
	if right {
		return onGrid(iconWidest(i)+2*iconRing, 8, math.Ceil), iconSize(i) + iconUnder(i) + 2*(iconMove+iconArrow)
	}
	return onGrid(iconWidest(i)+2*(iconMove+iconArrow), 8, math.Ceil), iconSize(i) + 2*(labelRoom+iconRing)
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

// iconEnd is where an end goes on its icon: the side, and how far along the side it stands.
type iconEnd struct {
	side  string
	share float64
}

// settling is what the ends moved keep off: what does not move, and each edge's runs and label as they stand.
type settling struct {
	things []rect // the boxes and the headings
	sides  []seg  // the frames' sides and the runs left to edges written by hand
	runs   map[int][]seg
	labels map[int]rect // each label with what lies between it and its run, by the edge it is of
}

// near is whether a and b are one coordinate, but for a float's rounding.
func near(a, b float64) bool { return math.Abs(a-b) < eps }

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
func (d *drawing) settleIcons() {
	if len(d.icons) == 0 {
		return
	}
	d.shareSides()
	d.settling = settling{things: nil, sides: nil, runs: map[int][]seg{}, labels: map[int]rect{}}
	d.settling.things = slices.Collect(maps.Values(d.boxes))
	for c, head := range d.heads {
		d.settling.things = append(d.settling.things, headRect(d.frames[c], head))
	}
	edges := slices.Sorted(maps.Keys(d.routes))
	for _, k := range edges {
		d.noteEdge(k)
	}
	d.noteSides()
	for _, k := range edges {
		d.straighten(k)
	}
	d.noteSides() // the icons have moved, and the runs left to edges written by hand with them
	// Again while any moves: a run held off by another that has since moved may move now. Each end moves once.
	for moved := true; moved; {
		moved = false
		for _, k := range edges {
			moved = d.slide(k) || moved
		}
	}
	for _, k := range edges {
		d.bend(k)
	}
}

// noteEdge keeps edge k's runs and label as they stand now.
func (d *drawing) noteEdge(k int) {
	d.settling.runs[k] = segsOf(d.routes[k])
	for e, sp := range d.spots {
		if e == k || sp.edge == k {
			d.settling.labels[e] = heldRect(d.runOf(sp), sp)
		}
	}
}

// noteSides keeps the runs a run or a label keeps off that no edge has.
func (d *drawing) noteSides() {
	d.settling.sides = d.settling.sides[:0]
	for _, f := range d.frames {
		d.settling.sides = append(d.settling.sides, seg{f.x, f.y, f.x + f.w, f.y}, seg{f.x + f.w, f.y, f.x + f.w, f.y + f.h},
			seg{f.x, f.y + f.h, f.x + f.w, f.y + f.h}, seg{f.x, f.y, f.x, f.y + f.h})
	}
	d.settling.sides = append(d.settling.sides, d.hands()...)
}

// shareSides gives each end at an icon its place: the n ends on a side part it into n+1 equal parts, in the order they
// stand on the rectangle's side, the order wire left them in by what they reach, so that they do not cross by the side.
func (d *drawing) shareSides() {
	type side struct {
		box  int
		name string
	}
	on := map[side][]endAt{}
	for k, pts := range d.routes {
		for j, p := range [2][2]float64{pts[0], pts[len(pts)-1]} {
			if i := d.ends[k][j]; i >= 0 && d.icons[i] != nil {
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
			d.onIcon[e] = iconEnd{s.name, float64(n+1) / float64(len(ends)+1)}
		}
	}
}

// iconAt is where the icon of box i stands in its rectangle, moved by what straighten moved it.
func (d *drawing) iconAt(i int) rect {
	ic, r := d.icons[i], d.boxes[i]
	size := iconSize(ic)
	top := (r.h - size) / 2
	if d.right {
		top -= iconUnder(ic) / 2
	}
	//nolint:exhaustruct_v5 // an area
	at := rect{x: r.x + (r.w-size)/2, y: r.y + top, w: size, h: size, what: "", at: ""}
	across, _ := d.axes()
	at.set(across, at.lo(across)+d.moved[i], size)
	return at
}

// iconDrawn is where the icon of box i stands with its label.
func (d *drawing) iconDrawn(i int) rect {
	ic, at := d.icons[i], d.iconAt(i)
	if ic.Label == nil {
		return at
	}
	w := labelWidth(ic.Label, false)
	//nolint:exhaustruct_v5 // an area
	return rect{x: min(at.x, at.x+at.w/2-w/2), y: at.y, w: max(at.w, w), h: at.h + iconUnder(ic), what: "", at: ""}
}

// endPoint is where the end e stands on its icon. On the bottom side it stands under the label, or under the icon past
// the label's width: an end there would stand off from all it belongs to.
// LIMIT: the label's width is an estimate (units), so an end by its edge may touch the words. Measure if that shows.
func (d *drawing) endPoint(e endAt) [2]float64 {
	i := d.ends[e.edge][e.end]
	ic, at, place := d.icons[i], d.iconAt(i), d.onIcon[e]
	axis := sideAxis(place.side)
	//nolint:exhaustruct_v5 // an icon, which is all sidePoint reads
	t := target{r: at, under: iconUnder(ic), labelW: iconLabelW(ic)}
	return sidePoint(t, place.side, at.lo(axis)+at.size(axis)*place.share)
}

// iconReach is how far the icon of box i may move across the ranks from the middle of its rectangle, keeping its
// label within the rectangle and an arrow's length from its side: 12 or more.
func (d *drawing) iconReach(i int) float64 {
	ic, r := d.icons[i], d.boxes[i]
	across, _ := d.axes()
	stands := iconWidest(ic)
	if across == 1 {
		stands = iconSize(ic) + iconUnder(ic)
	}
	return (r.size(across)-stands)/2 - iconArrow
}

// inLine is how many of pts, from the first, stand on one line, on the axis the line does not run on.
func inLine(pts [][2]float64) (n, axis int) {
	if !near(pts[0][0], pts[1][0]) {
		axis = 1
	}
	n = 1
	for n < len(pts) && near(pts[n][axis], pts[0][axis]) {
		n++
	}
	return n, axis
}

// straighten stands a straight edge at an icon on one line: an icon not yet moved moves across the ranks, once, so that
// its point stands on the line; a box takes it anywhere along its side; a point that cannot move holds the line to it.
// LIMIT: once, and by iconReach at most, so a straight run through icons in a row may find the next short of it, and
// then bends. Solve a row of icons together if that shows.
func (d *drawing) straighten(k int) {
	pts := d.routes[k]
	n, axis := inLine(pts)
	_, on0 := d.onIcon[endAt{k, 0}]
	_, on1 := d.onIcon[endAt{k, 1}]
	if n != len(pts) || d.loops[k] || !on0 && !on1 {
		return
	}
	across, _ := d.axes()
	lo, hi := math.Inf(-1), math.Inf(1)
	var movers []endAt
	for j := range 2 {
		i := d.ends[k][j]
		e := endAt{k, j}
		_, on := d.onIcon[e]
		switch {
		case i < 0:
		case !on:
			b := d.boxes[i]
			lo, hi = max(lo, b.lo(axis)+inEnd), min(hi, b.lo(axis)+b.size(axis)-inEnd)
		case axis == across && !d.settled[i]:
			q, reach := d.endPoint(e)[axis], d.iconReach(i)
			lo, hi = max(lo, q-reach), min(hi, q+reach)
			movers = append(movers, e)
		default:
			q := d.endPoint(e)[axis]
			lo, hi = max(lo, q), min(hi, q)
		}
	}
	if lo > hi {
		return
	}
	line := max(lo, min(pts[0][axis], hi))
	all := make([]int, len(pts))
	for i := range all {
		all[i] = i
	}
	if !d.moveRun(k, all, axis, line) {
		return
	}
	for _, e := range movers {
		i := d.ends[k][e.end]
		d.moved[i] += line - d.endPoint(e)[axis]
		d.settled[i] = true
	}
	d.ontoBoxes(k, axis, line)
	d.noteEdge(k)
}

// ontoBoxes brings the ends of straight edge k at boxes back onto their outlines, on line: a hexagon's top or a round
// corner stands off the side.
func (d *drawing) ontoBoxes(k, axis int, line float64) {
	pts := d.routes[k]
	for j, at := range [2]int{0, len(pts) - 1} {
		if i := d.ends[k][j]; i >= 0 && d.icons[i] == nil {
			b, side := d.boxes[i], "left"
			switch onward := pts[len(pts)-1-at][1-axis] > pts[at][1-axis]; {
			case axis == 0 && onward:
				side = "bottom"
			case axis == 0:
				side = "top"
			case onward:
				side = "right"
			}
			pts[at] = b.form.on(b, side, line)
		}
	}
}

// boxesTake is whether the boxes at the ends of straight edge k take its run on line.
func (d *drawing) boxesTake(k, axis int, line float64) bool {
	for j := range 2 {
		if i := d.ends[k][j]; i >= 0 && d.icons[i] == nil {
			if b := d.boxes[i]; line < b.lo(axis)+inEnd || line > b.lo(axis)+b.size(axis)-inEnd {
				return false
			}
		}
	}
	return true
}

// slide moves the run from each end of edge k at an icon onto the end's point, its label with it, when a turn follows
// the run, or when it runs straight with a label, which cannot bend, and reports whether any moved. The next run only
// grows or shrinks.
func (d *drawing) slide(k int) bool {
	if _, level := d.level[k]; level {
		return false
	}
	first := d.slideEnd(k, 0)
	return d.slideEnd(k, 1) || first
}

// slideEnd is slide for end j of edge k.
func (d *drawing) slideEnd(k, j int) bool {
	e := endAt{k, j}
	place, on := d.onIcon[e]
	if !on {
		return false
	}
	pts := d.routes[k]
	from := slices.Clone(pts)
	if j == 1 {
		slices.Reverse(from)
	}
	n, axis := inLine(from)
	straight := n == len(pts)
	if axis != sideAxis(place.side) || straight && (!d.labelled(k) || d.onPoint(k, axis)) {
		return false
	}
	run := make([]int, n)
	for i := range run {
		run[i] = i
		if j == 1 {
			run[i] = len(pts) - 1 - i
		}
	}
	q := d.endPoint(e)[axis]
	if near(from[0][axis], q) || straight && !d.boxesTake(k, axis, q) || !d.moveRun(k, run, axis, q) {
		return false
	}
	if straight {
		d.ontoBoxes(k, axis, q)
	}
	d.noteEdge(k)
	return true
}

// onPoint is whether straight edge k runs on the point of an end at an icon: it can run on only one, when they differ.
func (d *drawing) onPoint(k, axis int) bool {
	for j := range 2 {
		e := endAt{k, j}
		if _, on := d.onIcon[e]; on && near(d.routes[k][0][axis], d.endPoint(e)[axis]) {
			return true
		}
	}
	return false
}

// bend gives a straight edge with no label, short of its points, one bend in the middle of its run, when nothing stands
// there. A labelled one stays, the label beside it, and gathered bends it by its rectangles.
func (d *drawing) bend(k int) {
	pts := d.routes[k]
	n, axis := inLine(pts)
	if n != len(pts) || d.loops[k] || d.labelled(k) {
		return
	}
	a, b := pts[0], pts[len(pts)-1]
	for j, p := range []*[2]float64{&a, &b} {
		if _, on := d.onIcon[endAt{k, j}]; on {
			p[axis] = d.endPoint(endAt{k, j})[axis]
		}
	}
	if near(a[axis], pts[0][axis]) && near(b[axis], pts[0][axis]) {
		return
	}
	if near(a[axis], b[axis]) {
		b[axis] = a[axis]
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
		d.noteEdge(k)
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
		if i > 0 && pts[i-1] != pts[i] {
			runs = append(runs, segAt(pts, i-1))
		}
		if i+1 < len(pts) && pts[i] != pts[i+1] {
			runs = append(runs, segAt(pts, i))
		}
	}
	if d.clear(k, runs) && (d.loops[k] || d.labelFits(k)) {
		return true
	}

	for j, i := range at {
		pts[i] = was[j]
	}
	return false
}

// clear is whether runs, of edge k, keep off what does not move, the other edges' labels and what lies between them and
// their runs, and the other edges' runs: wire leaves no two edges touching, and check does not compare two edges of a
// graph, so a run landing on another would be drawn unsaid.
// LIMIT: only runs moved here are compared, so edges may touch where gathered bends them by a rectangle's side. Part
// them along the side if that shows.
// LIMIT: the parts written by hand are not compared: where the graph's x and y put them is known only once all has
// moved (unseen). A run landing on one is told at the part, which its writer can move.
func (d *drawing) clear(k int, runs []seg) bool {
	for _, s := range runs {
		through := func(r rect) bool { return crossedBy(s, r) }
		if slices.ContainsFunc(d.settling.things, through) {
			return false
		}
		for e, r := range d.settling.labels {
			if e != k && d.spots[e].edge != k && through(r) {
				return false
			}
		}
		for e, q := range d.settling.runs {
			if e != k && slices.ContainsFunc(q, func(o seg) bool { return runsTouch(s, o) }) {
				return false
			}
		}
	}
	return true
}

// labelFits is whether the labels on edge k stand within their runs and clear, as they stand now.
func (d *drawing) labelFits(k int) bool {
	for e, sp := range d.spots {
		if e == k || sp.edge == k {
			run := d.runOf(sp)
			r, _ := d.rect(e)
			along := runAxis(run)
			if r.lo(along) < min(run[along], run[along+2]) || r.lo(along)+r.size(along) > max(run[along], run[along+2]) ||
				!d.labelClear(k, e, heldRect(run, sp)) {
				return false
			}
		}
	}
	return true
}

// labelClear is whether held, the label of edge e with what lies between it and its run on edge k, is clear.
func (d *drawing) labelClear(k, e int, held rect) bool {
	//nolint:exhaustruct_v5 // a spot
	held = rect{x: held.x - 1, y: held.y - 1, w: held.w + 2, h: held.h + 2, what: "", at: ""}
	touches := func(s seg) bool { return crossedBy(s, held) }
	if slices.ContainsFunc(d.settling.things, held.overlaps) || slices.ContainsFunc(d.settling.sides, touches) ||
		slices.ContainsFunc(segsOf(d.routes[k]), touches) {
		return false
	}
	for o, r := range d.settling.labels {
		if o != e && d.spots[o].edge != k && held.overlaps(r) {
			return false
		}
	}
	for o, q := range d.settling.runs {
		if o != k && slices.ContainsFunc(q, touches) {
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
	for _, sp := range d.spots {
		if sp.edge == k {
			return true
		}
	}
	return false
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
// side if the end stands short of the point's line, and the run from the side to the icon; no corner where it does not
// turn. The labels stand already, so that no run need keep its number.
// LIMIT: two ends bent along one side may share a stretch of it, and take the look of the edge drawn last there.
func (d *drawing) gathered(k int) [][2]float64 {
	reach := func(e endAt, pts [][2]float64) [][2]float64 {
		at, axis := d.endPoint(e), sideAxis(d.onIcon[e].side)
		if near(pts[0][axis], at[axis]) {
			at[axis] = pts[0][axis]
			return append([][2]float64{at}, pts...)
		}
		turn := pts[0]
		turn[axis] = at[axis]
		return append([][2]float64{at, turn}, pts...)
	}
	out := slices.Clone(d.routes[k])
	_, on0 := d.onIcon[endAt{k, 0}]
	_, on1 := d.onIcon[endAt{k, 1}]
	if !on0 && !on1 {
		return out
	}
	if on0 {
		out = reach(endAt{k, 0}, out)
	}
	if on1 {
		slices.Reverse(out)
		out = reach(endAt{k, 1}, out)
		slices.Reverse(out)
	}
	return noCorners(out)
}

// noCorners is pts without the points where a line runs on the same way.
func noCorners(pts [][2]float64) [][2]float64 {
	out := pts[:1]
	for i := 1; i < len(pts)-1; i++ {
		p, q, r := out[len(out)-1], pts[i], pts[i+1]
		onward := near(p[0], q[0]) && near(q[0], r[0]) && (q[1]-p[1])*(r[1]-q[1]) >= 0 ||
			near(p[1], q[1]) && near(q[1], r[1]) && (q[0]-p[0])*(r[0]-q[0]) >= 0
		if !onward {
			out = append(out, q)
		}
	}
	return append(out, pts[len(pts)-1])
}

// unseen is how far past what the group measures on axis it starts to draw: an icon's rectangle reaches past it.
func (d *drawing) unseen(axis int) float64 {
	lo, _ := d.span(axis)
	first := math.Inf(1)
	for i, r := range d.boxes {
		if d.icons[i] != nil {
			r = d.iconDrawn(i)
		}
		first = min(first, r.lo(axis))
	}
	for _, f := range d.frames {
		first = min(first, f.lo(axis))
	}
	for k := range d.spots {
		r, _ := d.rect(k)
		first = min(first, r.lo(axis))
	}
	for _, pts := range d.routes {
		for _, p := range pts {
			first = min(first, p[axis])
		}
	}
	return first - lo
}
