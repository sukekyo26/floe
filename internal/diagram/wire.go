package diagram

import (
	"cmp"
	"math"
	"slices"
)

// The grid of a channel, the room between two ranks: exits stand at x ≡ 0 and entries at x ≡ 6 (mod column), so
// that no two links of a group share a run or touch, and runs across stand on tracks a column apart. A point pushed
// off its column may instead be entered at x ≡ 0, at a column no box above stands over and no other link of the
// channel leaves by (freeColumn).
const (
	column  = 12.0
	inEnd   = 12.0 // how far in from the ends of a side its links stand
	riseMin = 24.0 // the room between ranks besides their tracks
)

// wire sets each link's exit and entry, and its track in its channel, then the tops of the ranks and the nodes, as
// laid out down. Channels are wired top down: once the exits above a rank are set, its points stand at the columns
// their links come down from, and its boxes give them room.
func (gr *group) wire() {
	gr.tracks = make([]int, max(len(gr.ranks)-1, 0))
	pos := gr.positions()
	for r := 0; r+1 < len(gr.ranks); r++ {
		for _, u := range gr.ranks[r] {
			gr.exits(u, pos, false)
		}
		gr.settle(r + 1)
		// settle may move the boxes below off the columns chosen for them: choose those again, the points' kept.
		// LIMIT: only for the rank right below; a box a settle further down moves is not chosen for again, so an
		// edge meant to enter straight may bend. Choose every exit once all ranks stand, if that comes to matter.
		for _, u := range gr.ranks[r] {
			gr.exits(u, pos, true)
		}
		lo, hi := gr.extent()
		for k, v := range gr.ranks[r+1] {
			if gr.nodes[v].box != nil {
				gr.widen(gr.ranks[r+1], k, lo, hi)
			}
		}
		for _, v := range gr.ranks[r+1] {
			gr.entries(v)
		}
		gr.tracks[r] = gr.track(r)
	}
	gr.shiftRight()
	gr.rise()
}

// rise sets the tops and bottoms of the ranks and the nodes' tops, as laid out down: a rank as tall as its tallest
// box (and, laid out right, its headings' rooms), the room above it for the frames starting there and below it for
// those ending, then its channel's tracks.
func (gr *group) rise() {
	gr.top, gr.bottom = make([]float64, len(gr.ranks)), make([]float64, len(gr.ranks))
	above, below := gr.rooms()
	y := 0.0
	for r, rank := range gr.ranks {
		y += above[r].room
		tall := column
		for _, i := range rank {
			if n := gr.nodes[i]; n.kind == kindHeading {
				tall = max(tall, gr.headAt-above[r].of[n.cl]+n.h)
			} else if n.kind == kindPlain {
				tall = max(tall, n.h)
			}
		}
		gr.top[r], gr.bottom[r] = y, y+tall
		gr.stand(r, above[r], below[r])
		gr.botRoom = append(gr.botRoom, below[r].room)
		if r < len(gr.tracks) {
			y += tall + below[r].room + riseMin + column*float64(gr.tracks[r])
		}
	}
}

// stand sets the tops of the nodes of rank r, and the tops and bottoms of the frames starting and ending there: a
// box in the middle of the rank, a point along it, a heading's room (laid out right) its offset below its frame's top,
// and where an edge enters or leaves a frame on its top or bottom.
func (gr *group) stand(r int, above, below room) {
	top, bottom := gr.top[r], gr.bottom[r]
	for _, i := range gr.ranks[r] {
		n := &gr.nodes[i]
		switch n.kind {
		case kindPlain:
			if n.box != nil {
				n.y = top + (bottom-top-n.h)/2
			} else {
				n.y, n.h = top, bottom-top
			}
		case kindHeading:
			n.y = top - above.of[n.cl] + gr.headAt
		case kindIn:
			n.y, n.h = top-above.of[n.cl], 0
		case kindOut:
			n.y, n.h = bottom+below.of[n.cl], 0
		case kindLeft, kindRight:
			n.y, n.h = top, bottom-top
		}
	}
	for c, room := range above.of {
		gr.frames[c] = [2]float64{top - room, gr.frames[c][1]}
	}
	for c, room := range below.of {
		gr.frames[c] = [2]float64{gr.frames[c][0], bottom + room}
	}
}

// columns are the x ≡ res (mod column) from lo to hi.
func columns(lo, hi, res float64) []float64 {
	var xs []float64
	for x := math.Ceil((lo-res)/column)*column + res; x <= hi; x += column {
		xs = append(xs, x)
	}
	return xs
}

// spread picks n of xs, spread evenly.
func spread(xs []float64, n int) []float64 {
	out := make([]float64, n)
	for i := range n {
		out[i] = xs[(2*i+1)*len(xs)/(2*n)]
	}
	return out
}

// side is where links may leave node n (out) or enter it: on a box, 12 in from its ends; a point is entered at its
// column x and left at x+w (6 to the right when it is entered bent at x ≡ 6, else the same).
func (n gnode) side(out bool) (lo, hi float64) {
	switch {
	case n.box != nil:
		return n.x + inEnd, n.x + n.w - inEnd
	case out:
		return n.x + n.w, n.x + n.w
	}
	return n.x, n.x
}

// exits sets the exits of the links out of node u, at x ≡ 0 on its bottom, in the order their ends stand in the
// rank below (ties as their edges are written): each at the column nearest to where its edge can enter the box it
// reaches straight (through the points it passes), then nearest to where it would stand spread evenly, leaving a
// column for each exit after it.
func (gr *group) exits(u int, pos []int, keep bool) {
	n := gr.nodes[u]
	if len(n.outs) == 0 {
		return
	}
	outs := slices.Clone(n.outs)
	slices.SortStableFunc(outs, func(a, b int) int {
		return cmp.Or(pos[gr.links[a].to]-pos[gr.links[b].to], gr.links[a].edge-gr.links[b].edge)
	})
	// ③ makes a box 12·k+36 wide for k links at a side, so it has a column for each; a point has one link out.
	lo, hi := n.side(true)
	cols := columns(lo, hi, 0)
	even := spread(cols, len(outs))
	fixed := func(k int) bool { return keep && gr.nodes[gr.links[outs[k]].to].box == nil }
	next := 0
	for k, l := range outs {
		if fixed(k) {
			next = slices.Index(cols, gr.links[l].exit) + 1
			continue
		}
		// The last column it may take: one for each exit after it, and before the next exit kept.
		last := len(cols) - (len(outs) - k)
		for j := k + 1; j < len(outs); j++ {
			if fixed(j) {
				last = min(last, slices.Index(cols, gr.links[outs[j]].exit)-(j-k))
				break
			}
		}
		lo, hi := gr.nodes[gr.through(gr.links[l].to, false)].side(false)
		cost := func(x float64) (float64, float64) {
			return max(lo-x, x-hi, 0), math.Abs(x - even[k])
		}
		best := next
		for i := next + 1; i <= last; i++ {
			a, b := cost(cols[i])
			c, d := cost(cols[best])
			if a < c || a == c && b < d {
				best = i
			}
		}
		gr.links[l].exit, next = cols[best], best+1
	}
}

// settle places the points of rank r at the columns their links come down from, so that each edge runs straight
// through the ranks it passes: a point is a column, and the next point stands a column on, or nodeGap past the
// boxes between. A point pushed off its column, by the point before, by the boxes between or by a group's mark,
// stands at the free column nearest to where it may (freeColumn), entered bent and left there; with none, it is
// entered bent at the first x ≡ 6 it may stand at, and left half a column on, at x ≡ 0 (not half a column before:
// the column it enters at keeps a column from the one before it). The boxes then stand as near their places as the
// points let them, nodeGap from them and from one another.
// LIMIT: a long edge's point pushes boxes aside; widen gives a box back one column at most. Let points give way if
// such boxes come to bend many edges: tried, it made more turns in all.
func (gr *group) settle(r int) {
	rank := gr.ranks[r]
	last, right := -1, 0.0 // the last point placed, by index in rank, and the column it leaves at
	for k, v := range rank {
		n := &gr.nodes[v]
		if !n.settles() {
			continue
		}
		in := n.ins[0]
		e := gr.links[in].exit
		lo, hi := gr.bounds(r, k, last, right)
		n.x, n.w = e, 0
		switch c, ok := gr.freeColumn(r, in, min(max(e, lo), hi), lo, hi); {
		case e >= lo && e <= hi:
		case ok:
			n.x = c
		case e < lo:
			n.x, n.w = onGrid(lo-column/2, column, math.Ceil)+column/2, column/2
			if n.x+n.w > hi {
				// Short of a mark of a group's, which does not move: entered bent to the left of its column.
				n.x = onGrid(hi-column, column, math.Floor) + column/2
			}
		default: // short of a mark of a group's: entered bent to the left of its column
			n.x, n.w = onGrid(hi-column, column, math.Floor)+column/2, column/2
		}
		last, right = k, n.x+n.w
	}
	// The boxes, left to right past what stands before each, then right to left before what stands after: the room
	// left for the boxes between two points holds them packed, so the second pass keeps what the first made.
	for k := 1; k < len(rank); k++ {
		if n := &gr.nodes[rank[k]]; n.box != nil {
			o := gr.nodes[rank[k-1]]
			n.x = max(n.x, onGrid(o.x+o.w+gr.gap(rank[k-1], rank[k]), 6, math.Ceil))
		}
	}
	for k := len(rank) - 2; k >= 0; k-- {
		if n := &gr.nodes[rank[k]]; n.box != nil {
			n.x = min(n.x, onGrid(gr.nodes[rank[k+1]].x-gr.gap(rank[k], rank[k+1])-n.w, 6, math.Floor))
		}
	}
}

// freeLimit is how far from where it may stand a point looks for a free column: further, it would stand far from
// its edge, lengthening the edge and pushing what stands after it.
// LIMIT: a point with no free column freeLimit near is entered at x ≡ 6; look further if its half-column turns show.
// It is most where a group's marks narrow where it may stand to columns all under boxes above or taken by exits.
const freeLimit = 4 * column

// freeColumn is the column at x ≡ 0 within lo and hi nearest to t (left of two as near), freeLimit from it at most,
// that no box of rank r−1 stands over, past 12 in from its ends, and no link of the channel above rank r but in
// leaves by: exits chooses a box's exits to its boxes again after settle, anywhere along its side.
func (gr *group) freeColumn(r, in int, t, lo, hi float64) (float64, bool) {
	best, found := 0.0, false
	for c := onGrid(max(lo, t-freeLimit), column, math.Ceil); c <= min(hi, t+freeLimit); c += column {
		if !gr.taken(r, in, c) && (!found || math.Abs(c-t) < math.Abs(best-t)) {
			best, found = c, true
		}
	}
	return best, found
}

// taken reports whether column c is under the side of a box of rank r−1, or where a link of the channel below it
// but in leaves.
func (gr *group) taken(r, in int, c float64) bool {
	for _, u := range gr.ranks[r-1] {
		n := gr.nodes[u]
		if lo, hi := n.side(true); n.box != nil && c >= lo && c <= hi {
			return true
		}
		for _, l := range n.outs {
			if l != in && gr.links[l].exit == c {
				return true
			}
		}
	}
	return false
}

// extent is how far the group's nodes reach across, as they stand.
func (gr *group) extent() (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, n := range gr.nodes {
		lo, hi = min(lo, n.x), max(hi, n.x+n.w)
	}
	return lo, hi
}

// widen widens the box at k in rank, by as little as lets each link into it whose exit stands a column or less past
// an end of its side enter straight: its right to the exit and inEnd (its left to the exit less inEnd). It widens
// only into room: a gap from the node beside it, or, with none, within the group's extent lo..hi, so that nothing
// else moves; and only where more of its links then enter straight, as entries lays them (a box with straight
// entries crowding its side may lay them bent all the same). It widens towards its loops too: they are drawn from
// the box as widened.
func (gr *group) widen(rank []int, k int, lo, hi float64) {
	v := rank[k]
	n := &gr.nodes[v]
	if before := rank[:k]; len(before) > 0 {
		p := before[len(before)-1]
		lo = gr.nodes[p].x + gr.nodes[p].w + gr.gap(p, v)
	}
	if after := rank[k+1:]; len(after) > 0 {
		hi = gr.nodes[after[0]].x - gr.gap(v, after[0])
	}
	for _, l := range n.ins {
		e := gr.links[l].exit
		slo, shi := n.side(false)
		x, w := n.x, n.w
		switch {
		case e > shi && e <= shi+column && e+inEnd <= hi:
			n.w = e + inEnd - n.x
		case e < slo && e >= slo-column && e-inEnd >= lo:
			n.w += n.x - (e - inEnd)
			n.x = e - inEnd
		default:
			continue
		}
		wx, ww, wider := n.x, n.w, gr.straightIn(v)
		n.x, n.w = x, w
		if gr.straightIn(v) < wider {
			n.x, n.w = wx, ww
		}
	}
}

// straightIn is how many links into box v enter straight, as entries lays them.
func (gr *group) straightIn(v int) int {
	gr.entries(v)
	count := 0
	for _, l := range gr.nodes[v].ins {
		if gr.links[l].straight {
			count++
		}
	}
	return count
}

// bounds are where the point at k in rank r may stand: lo, past what stands before it (the point last placed, at
// last, leaving at right, or the nearest node after it that settle does not move) with the boxes between packed;
// hi, where it must leave by, short of the nearest node after it that settle does not move, with what stands
// between packed (a point as wide as pack gives it). With no group, no node is unmoved: hi is
// infinite.
func (gr *group) bounds(r, k, last int, right float64) (lo, hi float64) {
	rank := gr.ranks[r]
	v := rank[k]
	prev, at := last, right
	for j := k - 1; j > last; j-- {
		if m := gr.nodes[rank[j]]; m.rigid() {
			prev, at = j, m.x+m.w
			break
		}
	}
	lo = math.Inf(-1)
	if prev >= 0 {
		p := rank[prev]
		between := gr.boxesOf(rank[prev+1 : k])
		for _, b := range between {
			at = max(onGrid(at+gr.gap(p, b), 6, math.Ceil), onGrid(gr.headFloor(b), 6, math.Ceil)) + gr.nodes[b].w
			p = b
		}
		lo = at + gr.gap(p, v)
		if len(between) > 0 {
			lo = onGrid(lo, 6, math.Ceil)
		}
	}
	lo = max(lo, gr.headFloor(v))
	hi = math.Inf(1)
	for j := k + 1; j < len(rank); j++ {
		if !gr.nodes[rank[j]].rigid() {
			continue
		}
		at, next := gr.nodes[rank[j]].x, rank[j]
		for i := j - 1; i > k; i-- {
			at = at - gr.gap(rank[i], next) - gr.room(rank[i])
			if gr.nodes[rank[i]].box != nil { // as the boxes' pass right to left puts it, on the grid of 6
				at = onGrid(at, 6, math.Floor)
			}
			next = rank[i]
		}
		hi = at - gr.gap(v, next)
		break
	}
	return lo, hi
}

// shiftRight moves the group across by whole columns, so that it keeps its grid, to start within a column right of
// 0: settle may have moved its boxes either way.
func (gr *group) shiftRight() {
	least := math.Inf(1)
	for _, n := range gr.nodes {
		least = min(least, n.x)
	}
	by := -onGrid(least, column, math.Floor)
	for i := range gr.nodes {
		gr.nodes[i].x += by
	}
	for i := range gr.links {
		gr.links[i].exit += by
		gr.links[i].entry += by
	}
}

// entries sets the entries of the links into node v: a point (or where an edge enters a frame) is entered where
// settle put it. Into a box, in the order of their exits (ties as their edges are written):
// a link whose exit is over v, 12 in from its ends, enters right below it; the rest at x ≡ 6 between those, spread
// evenly, not in the columns beside a straight entry, so that arrowheads stay apart. When a gap between straight
// entries (or an end) holds too few columns for its links, the straight entry closing it on the right (else on the
// left) bends, joining the gap to the next, until every gap holds its links.
func (gr *group) entries(v int) {
	n := gr.nodes[v]
	if n.settles() { // its one link enters where settle put it
		l := &gr.links[n.ins[0]]
		l.entry, l.straight = n.x, l.exit == n.x
		return
	}
	ins := slices.Clone(n.ins)
	slices.SortStableFunc(ins, func(a, b int) int {
		return cmp.Or(cmp.Compare(gr.links[a].exit, gr.links[b].exit), gr.links[a].edge-gr.links[b].edge)
	})
	lo, hi := n.side(false)
	top := entrySide{gr: gr, ins: ins, sixes: columns(lo, hi, column/2)}
	var straight []int // indexes in ins
	for k, l := range ins {
		if x := gr.links[l].exit; x >= lo && x <= hi {
			straight = append(straight, k)
		}
	}
	for {
		g, ok := top.lay(straight, false)
		if ok || len(straight) == 0 {
			break
		}
		closing := min(g, len(straight)-1)
		straight = slices.Delete(straight, closing, closing+1)
	}
	top.lay(straight, true)
}

// entrySide is the top of a node: the links into it in the order of their exits, and its columns at x ≡ 6.
type entrySide struct {
	gr    *group
	ins   []int
	sixes []float64
}

// free is count columns between two straight entries at left and right, spread evenly, not beside either; nil when
// there are fewer.
func (s entrySide) free(count int, left, right float64) []float64 {
	var xs []float64
	for _, x := range s.sixes {
		if x > left+column/2 && x < right-column/2 {
			xs = append(xs, x)
		}
	}
	if len(xs) < count {
		return nil
	}
	return spread(xs, count)
}

// lay lays the links between the straight entries (indexes in ins) into their gaps, setting their entries when set;
// on a gap that cannot hold its own, it returns the gap's index and false.
func (s entrySide) lay(straight []int, set bool) (int, bool) {
	links := s.gr.links
	left, from := math.Inf(-1), 0
	for g, k := range append(slices.Clone(straight), len(s.ins)) {
		right := math.Inf(1)
		if k < len(s.ins) {
			right = links[s.ins[k]].exit
		}
		xs := s.free(k-from, left, right)
		if xs == nil && k > from {
			return g, false
		}
		if set {
			for j, x := range xs {
				links[s.ins[from+j]].entry, links[s.ins[from+j]].straight = x, false
			}
			if k < len(s.ins) {
				links[s.ins[k]].entry, links[s.ins[k]].straight = right, true
			}
		}
		left, from = right, k+1
	}
	return 0, true
}

// track puts the bent links of the channel below rank r on tracks (nest) and returns how many it takes. On the grid
// the constraints follow the exits' order among links going one way, and none holds between links going opposite
// ways, so they make no cycle.
func (gr *group) track(r int) int {
	var bent []int
	for _, u := range gr.ranks[r] {
		for _, l := range gr.nodes[u].outs {
			if !gr.links[l].straight {
				bent = append(bent, l)
			}
		}
	}
	above := gr.above(bent)
	runs, at := gr.runs(bent)
	slices.SortStableFunc(bent, func(a, b int) int {
		alo, _ := nestSpan(runs[at[a]])
		blo, _ := nestSpan(runs[at[b]])
		return cmp.Or(cmp.Compare(alo, blo), gr.links[a].edge-gr.links[b].edge)
	})
	return gr.fill(bent, above)
}

// fill puts the links bent, in order, on tracks from the top (nestFill), and returns how many tracks it takes. Two
// runs on a track keep half a column apart: their ends are exits at x ≡ 0 and entries at x ≡ 6, or a point's entry
// at x ≡ 0 that taken keeps off every exit.
func (gr *group) fill(bent []int, above map[int][]int) int {
	runs, at := gr.runs(bent)
	ups := make([][]int, len(bent))
	for i, l := range bent {
		for _, a := range above[l] {
			ups[i] = append(ups[i], at[a])
		}
	}
	tracks, n := nestFill(runs, allOf(len(runs)), ups, 0)
	for i, l := range bent {
		gr.links[l].track = tracks[i]
	}
	return n
}

// above is, for each of the links bent, the links that must run on a track above it (nestAbove).
func (gr *group) above(bent []int) map[int][]int {
	runs, _ := gr.runs(bent)
	above := map[int][]int{}
	for i, ups := range nestAbove(runs) {
		for _, a := range ups {
			above[bent[i]] = append(above[bent[i]], bent[a])
		}
	}
	return above
}

// runs are the links bent as runs from their exits to their entries, and each link's index among them.
func (gr *group) runs(bent []int) ([]nestRun, map[int]int) {
	runs, at := make([]nestRun, len(bent)), map[int]int{}
	for i, l := range bent {
		runs[i], at[l] = nestRun{from: gr.links[l].exit, to: gr.links[l].entry}, i
	}
	return runs, at
}

// nestRun is a run across from one line to another: from a link's exit to its entry, or from an end's point on an
// icon to where its route leaves the icon's rectangle.
type nestRun struct{ from, to float64 }

// nestSpan is where r runs.
func nestSpan(r nestRun) (lo, hi float64) {
	return min(r.from, r.to), max(r.from, r.to)
}

// nest puts runs on tracks, the first nearest their froms, so that a run off a track does not cut a run along one
// (nestAbove), each track taking, in order of where they start, the runs past the one before by more than gap
// (nestFill). It returns each run's track and how many it takes.
func nest(runs []nestRun, gap float64) ([]int, int) {
	order := allOf(len(runs))
	slices.SortStableFunc(order, func(a, b int) int {
		alo, _ := nestSpan(runs[a])
		blo, _ := nestSpan(runs[b])
		return cmp.Compare(alo, blo)
	})
	return nestFill(runs, order, nestAbove(runs), gap)
}

// nestAbove is, for each of runs, the runs that must stand on a track nearer their froms: of two runs that overlap,
// the one whose from is within the other, whose span holds the other's to, or whose from is the other's to. All
// three are compared within eps, as the ends of runs at an icon are its side's parts.
// LIMIT: two runs each with its from within the other or at its to have no order and may share a stretch: a's from at
// b's to with a's to and b's from on one side of it ([(4,3),(6,0),(3,5)]). Such froms come out of order with their
// tos; no graph of the random tests has one. Ordering by the from at the to first makes cycles.
func nestAbove(runs []nestRun) [][]int {
	within := func(x float64, r nestRun) bool {
		lo, hi := nestSpan(r)
		return x > lo+eps && x < hi-eps
	}
	above := make([][]int, len(runs))
	for i, a := range runs {
		for j := i + 1; j < len(runs); j++ {
			b := runs[j]
			alo, ahi := nestSpan(a)
			blo, bhi := nestSpan(b)
			if ahi < blo-eps || bhi < alo-eps {
				continue
			}
			aUp := within(a.from, b) || within(b.to, a) || near(a.from, b.to)
			bUp := within(b.from, a) || within(a.to, b) || near(b.from, a.to)
			switch {
			case aUp && !bUp:
				above[j] = append(above[j], i)
			case bUp && !aUp:
				above[i] = append(above[i], j)
			}
		}
	}
	return above
}

// nestFill puts runs, in order, on tracks from the first: each track takes the runs whose runs above stand on
// tracks before it, past the run before it on the track by more than gap (within eps). When none of the runs left
// can go on a track, they wait on one another (a cycle): the first of them goes on it all the same, so that each
// track takes one run at least, and nestFill takes len(runs) tracks at most. It returns each run's track and how many
// it takes.
// LIMIT: a run put on a track to end a cycle may cross or share a stretch with one it waited on. No runs of the
// bends at an icon's side have made a cycle, compared within eps.
func nestFill(runs []nestRun, order []int, above [][]int, gap float64) ([]int, int) {
	track := make([]int, len(runs))
	placed := map[int]int{}
	t := 0
	for ; len(order) > 0 && t < len(runs); t++ {
		ready := func(l int) bool {
			for _, a := range above[l] {
				if p, ok := placed[a]; !ok || p >= t {
					return false
				}
			}
			return true
		}
		force := -1
		if !slices.ContainsFunc(order, ready) {
			force = order[0]
		}
		end := math.Inf(-1)
		var rest []int
		for _, l := range order {
			lo, hi := nestSpan(runs[l])
			if (l == force || ready(l)) && lo > end+gap-eps {
				track[l], placed[l], end = t, t, hi
				continue
			}
			rest = append(rest, l)
		}
		order = rest
	}
	return track, t
}

// points are the corners of the edge of chain, top down, as drawn (turned for right), its ends on the sides of
// its boxes: down from each exit to its track, across, and down to its entry; through a point straight, or by a
// step across its middle when it is entered and left in different columns.
func (gr *group) points(chain []int, right bool) [][2]float64 {
	first := gr.links[chain[0]]
	u := gr.nodes[first.from]
	pts := [][2]float64{{first.exit, u.y + u.h}}
	in := 0.0 // where the link before entered
	for _, li := range chain {
		l := gr.links[li]
		if a := gr.nodes[l.from]; a.isPoint() {
			mid := (gr.top[a.rank] + gr.bottom[a.rank]) / 2
			pts = append(pts, [2]float64{in, mid}, [2]float64{l.exit, mid}, [2]float64{l.exit, gr.bottom[a.rank]})
		}
		b := gr.nodes[l.to]
		if !l.straight {
			r := gr.nodes[l.from].rank
			y := gr.bottom[r] + gr.below(r) + column*float64(1+l.track)
			pts = append(pts, [2]float64{l.exit, y}, [2]float64{l.entry, y})
		}
		pts = append(pts, [2]float64{l.entry, b.y})
		in = l.entry
	}
	pts = corners(pts)
	if right {
		for i, p := range pts {
			pts[i] = [2]float64{p[1], p[0]}
		}
	}
	return pts
}

// corners drops the points of pts that turn nothing: one where the last was, and one in line with its neighbours.
func corners(pts [][2]float64) [][2]float64 {
	out := pts[:1]
	for _, p := range pts[1:] {
		if p == out[len(out)-1] {
			continue
		}
		if n := len(out); n >= 2 {
			a, b := out[n-2], out[n-1]
			if a[0] == b[0] && b[0] == p[0] || a[1] == b[1] && b[1] == p[1] {
				out[n-1] = p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}
