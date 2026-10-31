package diagram

import (
	"cmp"
	"math"
	"slices"
)

// The room of a graph laid out down: a point is three columns of the grid wide, boxes of a rank stand nodeGap
// apart, and groups groupGap.
const (
	pointW   = 36.0
	nodeGap  = 24.0
	groupGap = 24.0
	maxPulls = 8
)

// place sizes the group's nodes, orders its ranks and sets the nodes' lefts, as laid out down: right lays out the
// boxes turned, their height across the ranks. It returns the trips order took and the pulls of the lefts.
func (gr *group) place(right bool) (trips, pulls int) {
	for i := range gr.nodes {
		n := &gr.nodes[i]
		if n.box == nil {
			continue
		}
		r := boxRect(&box{Name: "", X: 0, Y: 0, Origin: "", W: 0, H: 0, Rows: n.box.Rows, Tone: "", Shape: n.box.Shape,
			graph: ""}, "")
		n.w, n.h = r.w, r.h
		if right {
			n.w, n.h = r.h, r.w
		}
		// Room on a side for its links, a column of the grid each, 12 in from each end, and one more.
		if k := max(len(n.ins), len(n.outs)); k > 0 {
			n.w = max(n.w, ceil8(float64(12*k+36)))
		}
	}
	trips = gr.order()
	for _, rank := range gr.ranks {
		left := 0.0
		for _, i := range rank {
			gr.nodes[i].x = ceil6(left)
			left = gr.nodes[i].x + gr.nodes[i].w + nodeGap
		}
	}
	pulls = gr.pull()
	least := math.Inf(1)
	for _, n := range gr.nodes {
		least = min(least, n.x)
	}
	for i := range gr.nodes {
		gr.nodes[i].x -= least
	}
	return trips, pulls
}

// pull moves each node towards the median centre of its links' other ends, down from the rank above then up from
// the rank below, each move within nodeGap of its neighbours and on the grid of 6, so that a box stands over the
// boxes it leads to: the points first, so that an edge passing ranks runs straight, then boxes of one link in and
// one out, then the rest as written. It stops when nothing moves, or after maxPulls, and returns the pulls taken.
func (gr *group) pull() int {
	for pull := 1; pull <= maxPulls; pull++ {
		moved := false
		down := pull%2 == 1
		for k := range gr.ranks {
			r := k
			if !down {
				r = len(gr.ranks) - 1 - k
			}
			for _, i := range gr.pullOrder(r) {
				if gr.pullOne(r, i, down) {
					moved = true
				}
			}
		}
		if !moved {
			return pull
		}
	}
	return maxPulls
}

func (gr *group) pullOrder(r int) []int {
	order := slices.Clone(gr.ranks[r])
	class := func(i int) int {
		n := gr.nodes[i]
		switch {
		case n.box == nil:
			return 0
		case len(n.ins) == 1 && len(n.outs) == 1:
			return 1
		}
		return 2
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(class(a)-class(b), gr.nodes[a].written-gr.nodes[b].written)
	})
	return order
}

// pullOne moves node i of rank r towards the median centre of its links' ends above (down) or below, and reports
// whether it moved.
func (gr *group) pullOne(r, i int, down bool) bool {
	n := &gr.nodes[i]
	var centres []float64
	if down {
		for _, l := range n.ins {
			o := gr.nodes[gr.links[l].from]
			centres = append(centres, o.x+o.w/2)
		}
	} else {
		for _, l := range n.outs {
			o := gr.nodes[gr.links[l].to]
			centres = append(centres, o.x+o.w/2)
		}
	}
	if len(centres) == 0 {
		return false
	}
	slices.Sort(centres)
	m := len(centres)
	want := centres[m/2]
	if m%2 == 0 {
		want = (centres[m/2-1] + centres[m/2]) / 2
	}
	lo, hi := math.Inf(-1), math.Inf(1)
	at := slices.Index(gr.ranks[r], i)
	if at > 0 {
		o := gr.nodes[gr.ranks[r][at-1]]
		lo = ceil6(o.x + o.w + nodeGap)
	}
	if at+1 < len(gr.ranks[r]) {
		o := gr.nodes[gr.ranks[r][at+1]]
		hi = math.Floor((o.x-nodeGap-n.w)/6) * 6
	}
	x := min(max(math.Round((want-n.w/2)/6)*6, lo), hi)
	if x == n.x {
		return false
	}
	n.x = x
	return true
}

func ceil6(v float64) float64 {
	return math.Ceil(v/6) * 6
}

func ceil12(v float64) float64 {
	return math.Ceil(v/12) * 12
}

// drawn is n's area as drawn: as laid out, or turned for right.
func (gr *group) drawn(n gnode, right bool) rect {
	if right {
		return rect{x: n.y, y: n.x, w: n.h, h: n.w, what: "", at: ""}
	}
	return rect{x: n.x, y: n.y, w: n.w, h: n.h, what: "", at: ""}
}
