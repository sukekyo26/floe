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
// laid out down. Channels are wired top down, as a point's exit keeps the column it is entered at.
func (gr *group) wire() {
	gr.tracks = make([]int, max(len(gr.ranks)-1, 0))
	for r := 0; r+1 < len(gr.ranks); r++ {
		for _, u := range gr.ranks[r] {
			gr.exits(u)
		}
		for _, v := range gr.ranks[r+1] {
			gr.entries(v)
		}
		gr.tracks[r] = gr.track(r)
	}
	gr.top, gr.bottom = make([]float64, len(gr.ranks)), make([]float64, len(gr.ranks))
	y := 0.0
	for r, rank := range gr.ranks {
		tall := column
		for _, i := range rank {
			tall = max(tall, gr.nodes[i].h)
		}
		gr.top[r], gr.bottom[r] = y, y+tall
		for _, i := range rank {
			if gr.nodes[i].box != nil {
				gr.nodes[i].y = y + (tall-gr.nodes[i].h)/2
			} else {
				gr.nodes[i].y, gr.nodes[i].h = y, tall
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

func (gr *group) centre(i int) float64 {
	return gr.nodes[i].x + gr.nodes[i].w/2
}

// exits sets the exits of the links out of node u: at x ≡ 0 on its bottom, in the order of the centres they lead to
// (ties as their edges are written), spread evenly; a point's at the column it is entered at, when that is one.
func (gr *group) exits(u int) {
	n := gr.nodes[u]
	if len(n.outs) == 0 {
		return
	}
	lo, hi := n.x+inEnd, n.x+n.w-inEnd
	if n.box == nil && len(n.ins) == 1 {
		if in := gr.links[n.ins[0]].entry; math.Mod(in, column) == 0 && in >= lo && in <= hi {
			gr.links[n.outs[0]].exit = in
			return
		}
	}
	outs := slices.Clone(n.outs)
	slices.SortStableFunc(outs, func(a, b int) int {
		return cmp.Or(cmp.Compare(gr.centre(gr.links[a].to), gr.centre(gr.links[b].to)),
			gr.links[a].edge-gr.links[b].edge)
	})
	for k, x := range spread(columns(lo, hi, 0), len(outs)) {
		gr.links[outs[k]].exit = x
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
	lo, hi := n.x+inEnd, n.x+n.w-inEnd
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
// whose runs across overlap (or come within half a column), the one whose exit is within the other's run, or
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
// above are on tracks before it, half a column past the run before. It returns how many tracks it takes.
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
			if ready && lo >= end+column/2 {
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
			if ahi+column/2 <= blo || bhi+column/2 <= alo {
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
			mid := (gr.top[a.rank] + gr.bottom[a.rank]) / 2
			pts = append(pts, [2]float64{in, mid}, [2]float64{l.exit, mid}, [2]float64{l.exit, gr.bottom[a.rank]})
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
