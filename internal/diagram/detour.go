package diagram

// An edge of a graph passing a frame that holds neither end goes round it (detours), and the ends moved after keep
// out of it (keptOut).

import (
	"maps"
	"math"
	"slices"
	"sort"
)

// Why an edge stays through a frame: no way on the grid, or none from its ends' exits; a limit or the drawing's work.
const (
	stayNoWay = "no way"
	stayExits = "no way out of its ends"
	stayLimit = "a limit"
)

const (
	detourFrameGap = column // off a frame gone round, and off a frame's side run along
	detourBend     = 24.0
	detourCross    = 48.0
	// LIMIT: a search takes detourCells points of its grid and detourPops points from its queue at most, and the
	// searches of one drawing detourWork of both all told; past them, an edge keeps its way through. Raise them if
	// large graphs come to keep many.
	detourCells = 200_000
	detourPops  = 40_000
	detourWork  = 800_000
)

// frameRun is a frame of a drawing and its four sides.
type frameRun struct {
	c     int
	r     rect
	sides []seg
}

// round is what the detours of one drawing share: its frames, the blocks every way keeps off (boxes, headings and
// the loops' rooms), and each edge's runs.
type round struct {
	frames []frameRun
	blocks []rect
	runs   map[int][]seg
	edges  []int // the keys of runs, in order
}

func (o *graphOut) newRound(d *drawing) *round {
	rd := &round{runs: map[int][]seg{}} //nolint:exhaustruct_v5 // filled below
	for _, c := range slices.Sorted(maps.Keys(d.frames)) {
		f := d.frames[c]
		rd.frames = append(rd.frames, frameRun{c, f, []seg{
			{f.x, f.y, f.x + f.w, f.y},
			{f.x + f.w, f.y, f.x + f.w, f.y + f.h},
			{f.x, f.y + f.h, f.x + f.w, f.y + f.h},
			{f.x, f.y, f.x, f.y + f.h},
		}})
	}
	for _, i := range slices.Sorted(maps.Keys(d.boxes)) {
		rd.blocks = append(rd.blocks, grow(d.boxes[i], keepApart))
	}
	for _, c := range slices.Sorted(maps.Keys(d.heads)) {
		rd.blocks = append(rd.blocks, grow(headRect(d.frames[c], d.heads[c]), keepApart))
	}
	for _, r := range o.loopRooms(d) { // made after: a way through one would leave the loops no clear room
		rd.blocks = append(rd.blocks, grow(r, keepApart))
	}
	rd.edges = slices.Sorted(maps.Keys(d.routes))
	for _, k := range rd.edges {
		rd.runs[k] = segsOf(d.routes[k])
	}
	return rd
}

// foreign are the frames of d that hold neither end of edge k; a lane is none, an edge between lanes crossing those
// between.
func (o *graphOut) foreign(d *drawing, k int) []rect {
	ends := o.ends[k]
	var out []rect
	for _, c := range slices.Sorted(maps.Keys(d.frames)) {
		if !o.lane(c) && !o.holds(c, ends[0]) && !o.holds(c, ends[1]) {
			out = append(out, d.frames[c])
		}
	}
	return out
}

// within reports whether run s, across or down, passes inside frame f: a run on a side is outside, one anywhere in is
// in. crossedBy keeps a box's outline and 1 in it out, as an edge may run along an outline; a frame has no such edge.
func within(s seg, f rect) bool {
	return min(s[0], s[2]) < f.x+f.w && max(s[0], s[2]) > f.x && min(s[1], s[3]) < f.y+f.h && max(s[1], s[3]) > f.y
}

// keptOut are, for each edge of d, the frames it keeps out of as its ends move (settleEnds): those that hold neither
// end and that it does not pass already (detours left it through them). A loop and a level edge keep out of none, as
// detours takes neither round.
func (o *graphOut) keptOut(d *drawing) map[int][]rect {
	out := map[int][]rect{}
	for k, pts := range d.routes {
		if _, level := d.level[k]; level || d.loops[k] {
			continue
		}
		runs := segsOf(pts)
		for _, f := range o.foreign(d, k) {
			if !slices.ContainsFunc(runs, func(s seg) bool { return within(s, f) }) {
				out[k] = append(out[k], f)
			}
		}
	}
	return out
}

// strayed reports whether a run of edge k passes inside a frame that holds neither end.
func (o *graphOut) strayed(d *drawing, rd *round, k int) bool {
	for _, f := range o.foreign(d, k) {
		if slices.ContainsFunc(rd.runs[k], func(s seg) bool { return within(s, f) }) {
			return true
		}
	}
	return false
}

// detours takes each edge of edges, in order, passing a frame that holds neither end round it where a way is found,
// and keeps it in o.detoured; an edge left through, in o.stayed with why; none where the way is straight. A run gone
// round leaves the room the ranks kept for it: where that was the drawing's start, the drawing moves back to start
// where it did, so that the graph's x and y stay its top left.
// LIMIT: a way round crosses what it must: no check bounds the crossings of a graph drawn with ways round (the user
// took that), only those of it drawn with none (laidFault). Bound them if graphs come to look crossed.
// LIMIT: the level edges, drawn later (drawLevels, after the loops make room), are not minded: a way round may run
// beside one or meet it end to end, which laidFault tells of random graphs and none showed. Mind them if one does.
// LIMIT: each edge is looked at once, in order, against the ways taken before it, and none past the drawing's work:
// which edges are left through a frame in a large graph hangs on their order. Look again if that comes to show.
func (o *graphOut) detours(d *drawing, edges []int) {
	if o.way.straight {
		return
	}
	var was [2]float64
	for axis := range 2 {
		was[axis], _ = d.span(axis)
	}
	rd := o.newRound(d)
	sh := &search{work: detourWork, dist: nil, prev: nil, q: nil, tables: [12][]int32{}}
	for _, k := range edges {
		if !o.strayed(d, rd, k) {
			continue
		}
		why := stayLimit
		var pts [][2]float64
		if sh.work > 0 {
			pts, why = o.detour(d, rd, k, sh, false)
		}
		if why == stayNoWay { // no way on the lines of what is kept off: half a column off the runs too
			pts, why = o.detour(d, rd, k, sh, true)
		}
		if why != "" {
			if o.stayed == nil {
				o.stayed = map[int]string{}
			}
			o.stayed[k] = why
			continue
		}
		d.routes[k], rd.runs[k] = pts, segsOf(pts)
		if o.detoured == nil {
			o.detoured = map[int]bool{}
		}
		o.detoured[k] = true
	}
	for axis := range 2 {
		if lo, _ := d.span(axis); lo > was[axis] {
			d.stretch(axis, math.Inf(-1), onGrid(was[axis]-lo, column, math.Ceil)) // back by whole columns, not past
		}
	}
}

// search is what the searches of one drawing share: the work left, and the room each reuses.
type search struct {
	work   int
	dist   []float64
	prev   []int32
	q      queue
	tables [12][]int32 // a grid's tables of differences and sums
}

// table is room for n of tables[i], zeroed.
func (sh *search) table(i, n int) []int32 {
	if cap(sh.tables[i]) < n {
		sh.tables[i] = make([]int32, n)
	}
	t := sh.tables[i][:n]
	clear(t)
	return t
}

// keptOff is what a way for an edge keeps off: interiors no run enters, walls no run crosses or meets, frames'
// sides run along a column off, and the other edges' runs.
type keptOff struct {
	blocks       []rect
	walls, sides []seg
	others       []seg
}

// keptFor is what a way for edge k keeps off: the boxes and headings half a column, a frame holding neither end (but
// a lane) a column; a frame holding both ends, a lane's too, is a wall.
func (o *graphOut) keptFor(rd *round, k int) keptOff {
	ko := keptOff{blocks: slices.Clone(rd.blocks)} //nolint:exhaustruct_v5 // filled below
	ends := o.ends[k]
	for _, f := range rd.frames {
		h0, h1 := o.holds(f.c, ends[0]), o.holds(f.c, ends[1])
		switch {
		case h0 && h1:
			ko.walls = append(ko.walls, f.sides...)
		case o.lane(f.c):
		case !h0 && !h1:
			ko.blocks = append(ko.blocks, grow(f.r, detourFrameGap))
		}
		ko.sides = append(ko.sides, f.sides...)
	}
	for _, e := range rd.edges {
		if e != k {
			ko.others = append(ko.others, rd.runs[e]...)
		}
	}
	return ko
}

// clear reports whether the run a–b, an end's short run off its box (own), keeps off ko as a way does, but for the
// box it leaves: it enters no other block, crosses or meets no wall, and runs within half a column of no frame's side
// or other run, nor meets one end to end.
func (ko keptOff) clear(a, b [2]float64, own rect) bool {
	run := seg{a[0], a[1], b[0], b[1]}
	near := func(q seg) bool { return alongside(run, q) }
	return !ko.enters(run, own) && !slices.ContainsFunc(ko.walls, func(w seg) bool { return meets(run, w) }) &&
		!slices.ContainsFunc(ko.sides, near) && !slices.ContainsFunc(ko.others, near)
}

// enters reports whether run enters a block of ko but own's.
func (ko keptOff) enters(run seg, own rect) bool {
	lo, hi := [2]float64{min(run[0], run[2]), min(run[1], run[3])}, [2]float64{max(run[0], run[2]), max(run[1], run[3])}
	mine := grow(own, keepApart)
	for _, r := range ko.blocks {
		if r.x == mine.x && r.y == mine.y && r.w == mine.w && r.h == mine.h {
			continue
		}
		if lo[0] < r.x+r.w-eps && hi[0] > r.x+eps && lo[1] < r.y+r.h-eps && hi[1] > r.y+eps {
			return true
		}
	}
	return false
}

// alongside reports whether run s runs along q within half a column, or meets it end to end.
func alongside(s, q seg) bool {
	if isDown(s) != isDown(q) {
		return false
	}
	across := 0
	if !isDown(s) {
		across = 1
	}
	along := 1 - across
	return math.Abs(s[across]-q[across]) < keepApart-eps &&
		min(s[along], s[along+2]) < max(q[along], q[along+2])+keepApart &&
		max(s[along], s[along+2]) > min(q[along], q[along+2])-keepApart
}

// detour is a way for edge k round the frames it passes that hold neither end, on the lines of what it keeps off and
// of its ends; beside, on the lines half a column off the other runs too. Where it finds none, it tells why.
func (o *graphOut) detour(d *drawing, rd *round, k int, sh *search, beside bool) ([][2]float64, string) {
	pts := d.routes[k]
	n := len(pts)
	p0, pn := pts[0], pts[n-1]
	u0, un := dunit(p0, pts[1]), dunit(pts[n-2], pn)
	// The search starts and ends a column off the ends' boxes' rectangles: an end on a slant or a round end stands
	// inside its rectangle.
	ownBox := func(j int) (rect, bool) {
		i := d.ends[k][j]
		if i < 0 {
			return rect{}, false //nolint:exhaustruct_v5 // none
		}
		return d.boxes[i], true
	}
	off := func(j int, p, u [2]float64) [2]float64 {
		out := column
		if r, ok := ownBox(j); ok {
			switch dirOf(u) {
			case 0:
				out += max(r.x+r.w-p[0], 0)
			case 1:
				out += max(p[0]-r.x, 0)
			case 2:
				out += max(r.y+r.h-p[1], 0)
			case 3:
				out += max(p[1]-r.y, 0)
			}
		}
		return [2]float64{p[0] + out*u[0], p[1] + out*u[1]}
	}
	s, t := off(0, p0, u0), off(1, pn, [2]float64{-un[0], -un[1]})
	ko := o.keptFor(rd, k)
	r0, _ := ownBox(0)
	r1, _ := ownBox(1)
	// LIMIT: the exits are checked where the ends stand before settleEnds moves them to their sides' equal parts, so
	// an edge among ends packed on a side may stay through as no way out (stayExits) though it could leave once they
	// part. Check them where they will stand if that comes to show.
	if !ko.clear(p0, s, r0) || !ko.clear(t, pn, r1) {
		return nil, stayExits
	}
	g, ok := newDgrid(sh, ko, s, t, beside)
	if !ok {
		return nil, stayLimit
	}
	way, found, done := g.search(sh, s, t, dirOf(u0), dirOf(un))
	switch {
	case !done:
		return nil, stayLimit
	case !found:
		return nil, stayNoWay
	}
	full := append([][2]float64{p0}, way...)
	return corners(append(full, pn)), ""
}

var dirs = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// dgrid is the lines a way may run on, and what each step between two neighbouring points of it meets: right[i*ny+j]
// counts what keeps the step from point i,j to i+1,j off, down the step to i,j+1; the End ones, for a step out of the
// start or into the end, count a frame's side half a column off, not a column; the X ones, the runs a step crosses.
type dgrid struct {
	xs, ys                                        []float64
	right, down, rightEnd, downEnd, rightX, downX []int32
}

// stepSpan are the steps along lines that meet the open span lo–hi.
func stepSpan(steps []float64, lo, hi float64) (first, past int) {
	return max(sort.SearchFloat64s(steps, lo+eps)-1, 0), sort.SearchFloat64s(steps, hi-eps)
}

// closedSpan are the lines within lo–hi, its ends' too.
func closedSpan(vs []float64, lo, hi float64) (first, past int) {
	return sort.SearchFloat64s(vs, lo-eps), sort.SearchFloat64s(vs, hi+eps)
}

// lineSpan are the lines strictly within lo–hi.
func lineSpan(vs []float64, lo, hi float64) (first, past int) {
	return sort.SearchFloat64s(vs, lo+eps), sort.SearchFloat64s(vs, hi-eps)
}

// gridLines are the lines a way from s to t keeping off ko runs on: those of s and t, of the blocks' sides, a column
// either side of the frames' sides, and, beside, half a column either side of the other runs.
func gridLines(ko keptOff, s, t [2]float64, beside bool) (xs, ys []float64) {
	xset, yset := map[float64]bool{s[0]: true, t[0]: true}, map[float64]bool{s[1]: true, t[1]: true}
	off := func(q seg, by float64) {
		if isDown(q) {
			xset[q[0]-by], xset[q[0]+by] = true, true
		} else {
			yset[q[1]-by], yset[q[1]+by] = true, true
		}
	}
	for _, b := range ko.blocks {
		xset[b.x], xset[b.x+b.w], yset[b.y], yset[b.y+b.h] = true, true, true, true
	}
	for _, q := range ko.sides {
		off(q, detourFrameGap)
	}
	for _, q := range ko.others {
		if beside {
			off(q, keepApart)
		}
	}
	return slices.Sorted(maps.Keys(xset)), slices.Sorted(maps.Keys(yset))
}

// newDgrid is the grid for a way from s to t keeping off ko, or false past detourCells points or the drawing's work.
func newDgrid(sh *search, ko keptOff, s, t [2]float64, beside bool) (*dgrid, bool) {
	g := &dgrid{} //nolint:exhaustruct_v5 // filled below
	g.xs, g.ys = gridLines(ko, s, t, beside)
	nx, ny := len(g.xs), len(g.ys)
	// The work of a grid: its points, and its marks, each a few searches of its lines.
	work := nx*ny + 8*(len(ko.blocks)+len(ko.walls)+len(ko.sides)+2*len(ko.others))
	if work > sh.work {
		sh.work = 0
	}
	if nx*ny > detourCells || sh.work == 0 {
		return nil, false
	}
	sh.work -= work
	var diffs [6][]int32
	for i := range diffs {
		diffs[i] = sh.table(i, nx*(ny+1))
	}
	m := marks{g: g, nx: nx, ny: ny}
	m.keep(ko, diffs)
	for i, out := range []*[]int32{&g.right, &g.down, &g.rightEnd, &g.downEnd, &g.rightX, &g.downX} {
		*out = m.sum(diffs[i], sh.table(6+i, nx*ny))
	}
	return g, true
}

// marks adds over rectangles of the steps of g in tables of differences, each summed once: O(points + marks).
type marks struct {
	g      *dgrid
	nx, ny int
}

// add adds 1 over i in [i0,i1), j in [j0,j1) of tb.
func (m marks) add(tb []int32, i0, i1, j0, j1 int) {
	if i0 >= i1 || j0 >= j1 {
		return
	}
	tb[i0*(m.ny+1)+j0]++
	tb[i0*(m.ny+1)+j1]--
	if i1 < m.nx {
		tb[i1*(m.ny+1)+j0]--
		tb[i1*(m.ny+1)+j1]++
	}
}

// across marks the steps across (i, an x step) on the lines y within ylo–yhi, meeting xlo–xhi.
func (m marks) across(ylo, yhi, xlo, xhi float64, tbs ...[]int32) {
	j0, j1 := lineSpan(m.g.ys, ylo, yhi)
	i0, i1 := stepSpan(m.g.xs, xlo, xhi)
	for _, tb := range tbs {
		m.add(tb, i0, i1, j0, j1)
	}
}

// along marks the steps down (j, a y step) on the lines x within xlo–xhi, meeting ylo–yhi.
func (m marks) along(xlo, xhi, ylo, yhi float64, tbs ...[]int32) {
	i0, i1 := lineSpan(m.g.xs, xlo, xhi)
	j0, j1 := stepSpan(m.g.ys, ylo, yhi)
	for _, tb := range tbs {
		m.add(tb, i0, i1, j0, j1)
	}
}

// beside marks the steps within gap of run q, and half a column past its ends: runs meeting end to end touch.
func (m marks) beside(q seg, gap float64, tbs ...[]int32) {
	if isDown(q) {
		m.along(q[0]-gap, q[0]+gap, min(q[1], q[3])-keepApart, max(q[1], q[3])+keepApart, tbs...)
	} else {
		m.across(q[1]-gap, q[1]+gap, min(q[0], q[2])-keepApart, max(q[0], q[2])+keepApart, tbs...)
	}
}

// keep marks what ko keeps a way off in t, the tables right, down, rightEnd, downEnd, rightX and downX of dgrid.
func (m marks) keep(ko keptOff, t [6][]int32) {
	right, down, rightEnd, downEnd, rightX, downX := t[0], t[1], t[2], t[3], t[4], t[5]
	for _, b := range ko.blocks {
		m.across(b.y, b.y+b.h, b.x, b.x+b.w, right, rightEnd)
		m.along(b.x, b.x+b.w, b.y, b.y+b.h, down, downEnd)
	}
	for _, w := range ko.walls {
		m.wall(w, right, down, rightEnd, downEnd)
	}
	for _, q := range ko.sides {
		if isDown(q) {
			m.beside(q, detourFrameGap, down)
			m.beside(q, keepApart, downEnd)
		} else {
			m.beside(q, detourFrameGap, right)
			m.beside(q, keepApart, rightEnd)
		}
	}
	for _, q := range ko.others {
		if isDown(q) {
			m.beside(q, keepApart, down, downEnd)
			m.across(min(q[1], q[3]), max(q[1], q[3]), q[0]-eps, q[0]+eps, rightX)
		} else {
			m.beside(q, keepApart, right, rightEnd)
			m.along(min(q[0], q[2]), max(q[0], q[2]), q[1]-eps, q[1]+eps, downX)
		}
	}
}

// wall marks the steps across wall w, or to a point on it, its ends' lines in.
func (m marks) wall(w seg, right, down, rightEnd, downEnd []int32) {
	if isDown(w) {
		j0, j1 := closedSpan(m.g.ys, min(w[1], w[3]), max(w[1], w[3]))
		i0, i1 := stepSpan(m.g.xs, w[0]-1, w[0]+1)
		m.add(right, i0, i1, j0, j1)
		m.add(rightEnd, i0, i1, j0, j1)
		return
	}
	i0, i1 := closedSpan(m.g.xs, min(w[0], w[2]), max(w[0], w[2]))
	j0, j1 := stepSpan(m.g.ys, w[1]-1, w[1]+1)
	m.add(down, i0, i1, j0, j1)
	m.add(downEnd, i0, i1, j0, j1)
}

// sum is out, tb summed over i and j.
func (m marks) sum(tb, out []int32) []int32 {
	for i := range m.nx {
		run := int32(0)
		for j := range m.ny {
			run += tb[i*(m.ny+1)+j]
			out[i*m.ny+j] = run
			if i > 0 {
				out[i*m.ny+j] += out[(i-1)*m.ny+j]
			}
		}
	}
	return out
}

// walk is a search's ends on a grid: from point si,sj leaving the way start goes, to ti,tj and on the way end goes.
type walk struct {
	g                  *dgrid
	si, sj, ti, tj, ny int
	end                int
}

// bound is the least a way from point i,j to the end costs: the distance, and a bend where neither axis is the end's.
func (w walk) bound(i, j int) float64 {
	b := 0.0
	if i != w.ti && j != w.tj {
		b = detourBend
	}
	return math.Abs(w.g.xs[i]-w.g.xs[w.ti]) + math.Abs(w.g.ys[j]-w.g.ys[w.tj]) + b
}

// step is what the step from point i,j, come the way dir goes, to its neighbour the way di goes costs, or false where
// it may not be taken: its length, a bend, a bend at the end unless the way goes on the end's way, and the runs it
// crosses.
func (w walk) step(i, j, dir, di int) (float64, bool) {
	g := w.g
	ni, nj := i+dirs[di][0], j+dirs[di][1]
	if di == dir^1 || ni < 0 || nj < 0 || ni >= len(g.xs) || nj >= w.ny {
		return 0, false
	}
	at := ni == w.ti && nj == w.tj
	bad, cross := w.meets(i, j, ni, nj, i == w.si && j == w.sj || at)
	if bad > 0 || at && di == w.end^1 {
		return 0, false
	}
	cost := math.Abs(g.xs[ni]-g.xs[i]) + math.Abs(g.ys[nj]-g.ys[j]) + detourCross*float64(cross)
	if di != dir {
		cost += detourBend
	}
	if at && di != w.end {
		cost += detourBend
	}
	return cost, true
}

// meets are what keeps the step from point i,j to its neighbour ni,nj off (a frame's side half a column off where
// the step leaves the start or enters the end, ends) and the runs it crosses.
func (w walk) meets(i, j, ni, nj int, ends bool) (bad, cross int32) {
	g := w.g
	if j == nj {
		c := min(i, ni)*w.ny + j
		if ends {
			return g.rightEnd[c], g.rightX[c]
		}
		return g.right[c], g.rightX[c]
	}
	c := i*w.ny + min(j, nj)
	if ends {
		return g.downEnd[c], g.downX[c]
	}
	return g.down[c], g.downX[c]
}

// room is the search's distances and steps back for size points, the distances unknown.
func (sh *search) room(size int) (dist []float64, prev []int32) {
	if cap(sh.dist) < size {
		sh.dist, sh.prev = make([]float64, size), make([]int32, size)
	}
	dist, prev = sh.dist[:size], sh.prev[:size]
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	return dist, prev
}

// trace is the way to point goal of g, by the steps back of prev, from the start.
func (g *dgrid) trace(prev []int32, goal int) [][2]float64 {
	ny := len(g.ys)
	var way [][2]float64
	for c := goal; c >= 0; c = int(prev[c]) {
		way = append(way, [2]float64{g.xs[c/4/ny], g.ys[c/4%ny]})
	}
	slices.Reverse(way)
	return way
}

// search is the least costly way on g from s, leaving the way start goes, to t and on the way end goes (A*: length,
// a bend 24, the one at t too, a run crossed 48; bound). It reports whether it found one, and whether it ran to its
// end rather than to detourPops or the drawing's work.
// The points of a grid number at most detourCells times four ways, within int32 (see the conversions below).
func (g *dgrid) search(sh *search, s, t [2]float64, start, end int) (way [][2]float64, found, done bool) {
	ny := len(g.ys)
	w := walk{
		g: g, si: slices.Index(g.xs, s[0]), sj: slices.Index(g.ys, s[1]), ti: slices.Index(g.xs, t[0]),
		tj: slices.Index(g.ys, t[1]), ny: ny, end: end,
	}
	dist, prev := sh.room(len(g.xs) * ny * 4)
	at := func(i, j, d int) int { return (i*ny+j)*4 + d }
	first := at(w.si, w.sj, start)
	dist[first], prev[first] = 0, -1
	q, seq := sh.q[:0], int32(0)
	q = q.push(qitem{w.bound(w.si, w.sj), seq, int32(first)}) //nolint:gosec // G115: within detourCells*4
	defer func() { sh.q = q }()
	goal, pops := -1, 0
	for len(q) > 0 && goal < 0 {
		var it qitem
		q, it = q.pop()
		ci := int(it.n)
		i, j, dir := ci/4/ny, ci/4%ny, ci%4
		if it.cost-w.bound(i, j) > dist[ci]+eps {
			continue
		}
		if pops++; pops > detourPops || sh.work <= 0 {
			return nil, false, false
		}
		sh.work--
		if i == w.ti && j == w.tj && dir != end^1 {
			goal = ci
			continue
		}
		for di := range dirs {
			c, ok := w.step(i, j, dir, di)
			ni, nj := i+dirs[di][0], j+dirs[di][1]
			if nn := at(ni, nj, di); ok && dist[ci]+c < dist[nn]-eps {
				dist[nn], prev[nn] = dist[ci]+c, int32(ci) //nolint:gosec // G115: within detourCells*4
				seq++
				q = q.push(qitem{dist[nn] + w.bound(ni, nj), seq, int32(nn)}) //nolint:gosec // G115: within detourCells*4
			}
		}
	}
	if goal < 0 {
		return nil, false, true
	}
	return g.trace(prev, goal), true, true
}

// qitem is a point of a search waiting, by its cost and bound, then the order it came in.
type qitem struct {
	cost float64
	seq  int32
	n    int32
}

func (a qitem) before(b qitem) bool { return a.cost < b.cost || a.cost == b.cost && a.seq < b.seq }

// queue is a binary heap of qitems.
type queue []qitem

func (q queue) push(it qitem) queue {
	q = append(q, it)
	for i := len(q) - 1; i > 0; {
		p := (i - 1) / 2
		if !q[i].before(q[p]) {
			break
		}
		q[i], q[p] = q[p], q[i]
		i = p
	}
	return q
}

func (q queue) pop() (queue, qitem) {
	top := q[0]
	n := len(q) - 1
	q[0] = q[n]
	q = q[:n]
	for i := 0; ; {
		l, m := 2*i+1, i
		if l < n && q[l].before(q[m]) {
			m = l
		}
		if r := l + 1; r < n && q[r].before(q[m]) {
			m = r
		}
		if m == i {
			break
		}
		q[i], q[m] = q[m], q[i]
		i = m
	}
	return q, top
}

func dunit(a, b [2]float64) [2]float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	l := math.Hypot(dx, dy)
	return [2]float64{dx / l, dy / l}
}

func dirOf(u [2]float64) int {
	for i, d := range dirs {
		if near(float64(d[0]), u[0]) && near(float64(d[1]), u[1]) {
			return i
		}
	}
	return 0
}

// grow is r grown by m on every side, of no part.
func grow(r rect, m float64) rect {
	return rect{x: r.x - m, y: r.y - m, w: r.w + 2*m, h: r.h + 2*m, what: "", at: ""} //nolint:exhaustruct_v5 // no part
}
