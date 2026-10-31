package diagram

import (
	"cmp"
	"math"
	"slices"
)

// The grid of a channel, the room between two ranks: exits stand at x ≡ 0 and entries at x ≡ 6 (mod column), so
// that no two links of a group share a run or touch, and runs across stand on tracks a column apart.
const (
	column  = 12.0
	inEnd   = 12.0 // how far in from the ends of a side its links stand
	riseMin = 24.0 // the room between ranks besides their tracks
)

// wire sets each link's exit and entry, and its track in its channel, then the tops of the ranks and the nodes, as
// laid out down. Channels are wired top down: once the exits above a rank are set, its points stand at the columns
// their links come down from, and its boxes give them room.
// What it keeps, which makes two things of one graph never meet (tell takes their meeting for floe's failure): the
// nodes of a rank stand in its order, each a column past a point's or label's box's right, nodeGap past a box's, and
// each link's ends stand on its nodes' sides. settle makes it for a rank; straighten keeps it in ranks settled before,
// as it moves only points and labels' boxes, only right, only into the room roomAt finds before the next node, and
// a box's exit only into the gap before its next: no box moves, so their exits stand as chosen.
func (gr *group) wire() {
	gr.tracks = make([]int, max(len(gr.ranks)-1, 0))
	pos := gr.positions()
	for r := 0; r+1 < len(gr.ranks); r++ {
		for _, u := range gr.ranks[r] {
			gr.exits(u, pos, false)
		}
		gr.untangle(r+1, pos)
		gr.settle(r+1, pos)
		// settle may move the boxes below off the columns chosen for them: choose those again, the points' kept.
		// LIMIT: only for the rank right below; a box a settle further down moves is not chosen for again, so an
		// edge meant to enter straight may bend. Choose every exit once all ranks stand, if that comes to matter.
		for _, u := range gr.ranks[r] {
			gr.exits(u, pos, true)
		}
		for _, v := range gr.ranks[r+1] {
			gr.entries(v)
		}
	}
	// The tracks once every run stands: straighten moves runs of channels already wired.
	for r := range gr.tracks {
		gr.tracks[r] = gr.track(r)
	}
	gr.shiftRight()
	gr.top, gr.bottom = make([]float64, len(gr.ranks)), make([]float64, len(gr.ranks))
	y := 0.0
	for r, rank := range gr.ranks {
		tall := column
		for _, i := range rank {
			tall = max(tall, gr.nodes[i].h)
		}
		gr.top[r], gr.bottom[r] = y, y+tall
		for _, i := range rank {
			switch n := &gr.nodes[i]; {
			case n.box != nil:
				n.y = y + (tall-n.h)/2
			case n.label != nil:
				n.y = y
			default:
				n.y, n.h = y, tall
			}
		}
		if r < len(gr.tracks) {
			y += tall + riseMin + column*float64(gr.tracks[r])
		}
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

// right is where node n ends across: a box at its right side, a point at the column it leaves by, a label's box past
// its label.
func (n gnode) right() float64 {
	if n.box != nil {
		return n.x + n.w
	}
	return n.col + max(n.jog, n.w)
}

// side is where links may leave node n (out) or enter it: on a box, 12 in from its ends; a point or a label's box
// is entered at its column and left jog to the right of it.
func (n gnode) side(out bool) (lo, hi float64) {
	switch {
	case n.box != nil:
		return n.x + inEnd, n.x + n.w - inEnd
	case out:
		return n.col + n.jog, n.col + n.jog
	}
	return n.col, n.col
}

// exits sets the exits of the links out of node u, at x ≡ 0 on its bottom, in the order their ends stand in the
// rank below (ties as their edges are written), a column apart at least. Each takes, first, a column that leaves
// room for the labels' boxes of the exits after it; then the column nearest to where its edge can enter the box it
// reaches straight (through the points it passes); then the one nearest to where it would stand spread evenly.
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
	for k, l := range outs {
		if fixed(k) {
			continue
		}
		// From past the exit before, and its label's box where it has one, to the last column that leaves one for
		// each exit after it, before the exit kept next or the end.
		first, want := 0, 0
		if k > 0 {
			first = slices.Index(cols, gr.links[outs[k-1]].exit) + 1
			want = first - 1 + gr.gap(outs[k-1], l)
		}
		end, least, room := len(cols)-1, 0, 0
		for j := k + 1; j < len(outs); j++ {
			least, room = least+1, room+gr.gap(outs[j-1], outs[j])
			if fixed(j) {
				end = slices.Index(cols, gr.links[outs[j]].exit)
				break
			}
		}
		lo, hi := gr.nodes[gr.through(gr.links[l].to, false)].side(false)
		cost := func(i int) (int, float64, float64) {
			return max(i-(end-room), 0), max(lo-cols[i], cols[i]-hi, 0), math.Abs(cols[i] - even[k])
		}
		best := min(max(first, want), end-least)
		for i := best + 1; i <= end-least; i++ {
			a1, a2, a3 := cost(i)
			b1, b2, b3 := cost(best)
			if cmp.Or(a1-b1, cmp.Compare(a2, b2), cmp.Compare(a3, b3)) < 0 {
				best = i
			}
		}
		gr.links[l].exit = cols[best]
	}
}

// untangle orders the points and labels' boxes of rank r between each two boxes by the columns their links come
// down from (ties as they stand), so that none pushes another off its column. It crosses no more links: two of them
// stood the other way round only when their links came from different nodes and crossed above, and they cross at
// most once below.
func (gr *group) untangle(r int, pos []int) {
	rank := gr.ranks[r]
	for lo := 0; lo < len(rank); {
		hi := lo
		for hi < len(rank) && gr.nodes[rank[hi]].box == nil {
			hi++
		}
		slices.SortStableFunc(rank[lo:hi], func(a, b int) int {
			return cmp.Compare(gr.links[gr.nodes[a].ins[0]].exit, gr.links[gr.nodes[b].ins[0]].exit)
		})
		lo = hi + 1
	}
	for k, v := range rank {
		pos[v] = k
	}
}

// settle places the points and labels' boxes of rank r at the columns their links come down from, so that each
// edge runs straight through the ranks it passes: a point is a column, a label's box its column and the label
// beside it.
// Each stands a column past the one before (a label's box has room about its text), or nodeGap past the boxes
// between. One pushed off its column, by the one before or by the boxes between, is entered bent at the first
// x ≡ 6 it may stand at, and left half a column on, at x ≡ 0 (not half a column before: the column it enters at
// keeps a column from the one before it). The boxes then stand as near their places as the columns let them,
// nodeGap from them and from one another.
func (gr *group) settle(r int, pos []int) {
	rank := gr.ranks[r]
	last := -1 // the last placed, by index in rank
	for k, v := range rank {
		n := &gr.nodes[v]
		if n.box != nil {
			continue
		}
		e := gr.links[n.ins[0]].exit
		need := e
		if last >= 0 {
			need = max(e, gr.past(rank, last, k))
		}
		n.col, n.jog = need, 0
		if need != e {
			if c := onGrid(need, column, math.Ceil); gr.straighten(n.ins[0], c, pos) {
				n.col = c
			} else {
				n.col, n.jog = onGrid(need-column/2, column, math.Ceil)+column/2, column/2
			}
		}
		n.x = n.col
		last = k
	}
	// The boxes, left to right past what stands before each, then right to left before what stands after: the room
	// left for the boxes between two columns holds them packed, so the second pass keeps what the first made.
	for k := 1; k < len(rank); k++ {
		if n := &gr.nodes[rank[k]]; n.box != nil {
			o := gr.nodes[rank[k-1]]
			n.x = max(n.x, onGrid(o.right()+nodeGap, 6, math.Ceil))
		}
	}
	for k := len(rank) - 2; k >= 0; k-- {
		if n := &gr.nodes[rank[k]]; n.box != nil {
			n.x = min(n.x, onGrid(gr.nodes[rank[k+1]].x-nodeGap-n.w, 6, math.Floor))
		}
	}
}

// past is the first x the point or label's box rank[k] may stand at, past the one at rank[last] and the boxes
// between, packed as settle packs them, on the grid of 6.
func (gr *group) past(rank []int, last, k int) float64 {
	at, gap := gr.nodes[rank[last]].right(), column
	if between := gr.boxesOf(rank[last+1 : k]); len(between) > 0 {
		for _, b := range between {
			at = onGrid(at+nodeGap, 6, math.Ceil) + gr.nodes[b].w
		}
		gap = nodeGap
	}
	return onGrid(at+gap, 6, math.Ceil)
}

// straighten moves the straight run of an edge above link l, up to the box it leaves or the point it enters bent,
// across to column c, so that the node l enters stands at c straight rather than pushed. It moves nothing and
// reports false unless every node of the run has room at c in its rank, and the box leaves by c in its exits' order.
func (gr *group) straighten(l int, c float64, pos []int) bool {
	run := []int{l} // the links of the run, bottom up
	for u := gr.nodes[gr.links[l].from]; u.box == nil && u.jog == 0; u = gr.nodes[gr.links[l].from] {
		l = u.ins[0]
		run = append(run, l)
	}
	for _, l := range run {
		u := gr.nodes[gr.links[l].from]
		if u.box != nil {
			if !gr.leaves(l, c) {
				return false
			}
			continue
		}
		if !gr.roomAt(gr.links[l].from, c-u.jog, pos) {
			return false
		}
	}
	// The run's links stay straight; the lowest one's entry is set with the rank it enters.
	for _, l := range run {
		gr.links[l].exit, gr.links[l].entry = c, c
		if u := &gr.nodes[gr.links[l].from]; u.box == nil {
			u.col = c - u.jog
			u.x = u.col
			if u.jog != 0 {
				gr.links[u.ins[0]].entry = u.col
			}
		}
	}
	return true
}

// gap is how many columns the exit of link b stands right of the exit of link a, the one before it out of a node:
// one, or past a's label's box when both reach points or labels' boxes, which stand a column apart past it.
func (gr *group) gap(a, b int) int {
	ta, tb := gr.nodes[gr.links[a].to], gr.nodes[gr.links[b].to]
	if ta.box != nil || tb.box != nil {
		return 1
	}
	return int(math.Ceil((ta.w + column) / column))
}

// roomAt reports whether the point or label's box v may stand at column col, as far from what stands right of it in
// its rank as settle puts them: a column from a point or label's box, nodeGap from a box.
func (gr *group) roomAt(v int, col float64, pos []int) bool {
	n, rank := gr.nodes[v], gr.ranks[gr.nodes[v].rank]
	k := pos[v]
	if k+1 == len(rank) {
		return true
	}
	at, m := col+max(n.w, n.jog), gr.nodes[rank[k+1]]
	if m.box != nil {
		return m.x >= onGrid(at+nodeGap, 6, math.Ceil)
	}
	return m.col >= onGrid(at+column, 6, math.Ceil)
}

// leaves reports whether the box link l leaves can leave by column c instead, right of where it leaves: c on its
// side, and the gap to the next exit kept.
func (gr *group) leaves(l int, c float64) bool {
	u := gr.nodes[gr.links[l].from]
	if lo, hi := u.side(true); c < lo || c > hi {
		return false
	}
	next := -1
	for _, o := range u.outs {
		if e := gr.links[o].exit; e > gr.links[l].exit && (next < 0 || e < gr.links[next].exit) {
			next = o
		}
	}
	return next < 0 || gr.links[next].exit >= c+column*float64(gr.gap(l, next))
}

// shiftRight moves the group across by whole columns, so that it keeps its grid, to start within a column right of
// 0: settle may have moved its boxes either way.
func (gr *group) shiftRight() {
	least := math.Inf(1)
	for _, n := range gr.nodes {
		least = min(least, n.x)
	}
	gr.shift(-onGrid(least, column, math.Floor))
}

// shift moves the group right by by, as laid out down.
func (gr *group) shift(by float64) {
	for i := range gr.nodes {
		gr.nodes[i].x += by
		gr.nodes[i].col += by
	}
	for i := range gr.links {
		gr.links[i].exit += by
		gr.links[i].entry += by
	}
}

// entries sets the entries of the links into node v, in the order of their exits (ties as their edges are written):
// a link whose exit is over v, 12 in from its ends, enters right below it; the rest at x ≡ 6 between those, spread
// evenly, not in the columns beside a straight entry, so that arrowheads stay apart. When a gap between straight
// entries (or an end) holds too few columns for its links, the straight entry closing it on the right (else on the
// left) bends, joining the gap to the next, until every gap holds its links.
func (gr *group) entries(v int) {
	n := gr.nodes[v]
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

// track puts the bent links of the channel below rank r on tracks and returns how many it takes. Of two links
// whose runs across overlap, the one whose exit is within the other's run, or
// whose run holds the other's entry, goes on a track above, so that a run down does not cut a run across. Those
// constraints follow the exits' order among links going one way, and none holds between links going opposite
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
	slices.SortStableFunc(bent, func(a, b int) int {
		alo, _ := gr.span(a)
		blo, _ := gr.span(b)
		return cmp.Or(cmp.Compare(alo, blo), gr.links[a].edge-gr.links[b].edge)
	})
	return gr.fill(bent, above)
}

// fill puts the links bent, in order, on tracks from the top: each track takes, left to right, the links whose links
// above are on tracks before it, past the run before. It returns how many tracks it takes.
func (gr *group) fill(bent []int, above map[int][]int) int {
	placed := map[int]int{}
	t := 0
	for ; len(bent) > 0; t++ {
		end := math.Inf(-1)
		var rest []int
		for _, l := range bent {
			ready := true
			for _, a := range above[l] {
				if p, ok := placed[a]; !ok || p >= t {
					ready = false
				}
			}
			lo, hi := gr.span(l)
			// The grid keeps two runs half a column apart: their ends are exits at x ≡ 0 and entries at x ≡ 6.
			if ready && lo > end {
				gr.links[l].track, placed[l], end = t, t, hi
				continue
			}
			rest = append(rest, l)
		}
		bent = rest
	}
	return t
}

// span is where link l runs across.
func (gr *group) span(l int) (lo, hi float64) {
	a := gr.links[l]
	return min(a.exit, a.entry), max(a.exit, a.entry)
}

// above is, for each of the links bent, the links that must run on a track above it.
func (gr *group) above(bent []int) map[int][]int {
	within := func(x float64, l int) bool {
		lo, hi := gr.span(l)
		return x > lo && x < hi
	}
	above := map[int][]int{}
	for i, a := range bent {
		for _, b := range bent[i+1:] {
			alo, ahi := gr.span(a)
			blo, bhi := gr.span(b)
			if ahi < blo || bhi < alo {
				continue
			}
			aUp := within(gr.links[a].exit, b) || within(gr.links[b].entry, a)
			bUp := within(gr.links[b].exit, a) || within(gr.links[a].entry, b)
			switch {
			case aUp && !bUp:
				above[b] = append(above[b], a)
			case bUp && !aUp:
				above[a] = append(above[a], b)
			}
		}
	}
	return above
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
		if a := gr.nodes[l.from]; a.box == nil {
			// Half a column across near its foot: below a label's text, or near the bottom of the rank a point spans.
			step := a.y + a.h - column/2
			pts = append(pts, [2]float64{in, step}, [2]float64{l.exit, step}, [2]float64{l.exit, gr.bottom[a.rank]})
		}
		b := gr.nodes[l.to]
		if !l.straight {
			y := gr.bottom[gr.nodes[l.from].rank] + column*float64(1+l.track)
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
