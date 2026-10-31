package diagram

import (
	"cmp"
	"math"
	"slices"
)

// The room of a graph laid out down: boxes of a rank stand nodeGap apart, and from the points between them, and
// groups groupGap apart.
const (
	nodeGap  = 24.0
	groupGap = 24.0
	// LIMIT: 8 pulls towards the median, no more; lefts that keep moving stop where the eighth leaves them. Settle
	// them another way (Brandes-Köpf) if a graph comes to look skewed.
	maxPulls = 8
)

// place splits the group's arcs into links, sizes its nodes, orders its ranks and sets the nodes' lefts, as laid
// out down: right lays out the boxes turned, their height across the ranks. It returns the trips order took and
// the pulls of the lefts.
func (gr *group) place(right bool) (trips, pulls int) {
	gr.split()
	for i := range gr.nodes {
		n := &gr.nodes[i]
		if n.box == nil {
			continue
		}
		r := boxRect(&box{
			Name: "", X: 0, Y: 0, Origin: "", W: 0, H: 0, Rows: n.box.Rows, Tone: "", Shape: n.box.Shape,
			graph: "",
		}, "")
		n.w, n.h = r.w, r.h
		if right {
			n.w, n.h = r.h, r.w
		}
		// Room on a side for its links, a column of the grid each, 12 in from each end, and one more.
		if k := max(len(n.ins), len(n.outs)); k > 0 {
			n.w = max(n.w, onGrid(float64(12*k+36), 8, math.Ceil))
		}
	}
	trips = gr.order()
	for _, rank := range gr.ranks {
		left := 0.0
		for _, i := range gr.boxesOf(rank) {
			gr.nodes[i].x = onGrid(left, 6, math.Ceil)
			left = gr.nodes[i].x + gr.nodes[i].w + nodeGap
		}
	}
	pulls = gr.pull()
	least := math.Inf(1)
	for _, n := range gr.nodes {
		if n.box != nil {
			least = min(least, n.x)
		}
	}
	for i := range gr.nodes {
		gr.nodes[i].x -= least
	}
	return trips, pulls
}

// boxesOf are the boxes of rank, in order: the points of a rank are placed by wire, at their links' columns.
func (gr *group) boxesOf(rank []int) []int {
	return slices.DeleteFunc(slices.Clone(rank), func(i int) bool { return gr.nodes[i].box == nil })
}

// through is the box an edge reaches from node i, going up (or down) through the points it passes.
func (gr *group) through(i int, up bool) int {
	for gr.nodes[i].box == nil {
		if up {
			i = gr.links[gr.nodes[i].ins[0]].from
		} else {
			i = gr.links[gr.nodes[i].outs[0]].to
		}
	}
	return i
}

// pull moves each box towards the median centre of the boxes its edges reach (through the points they pass), down
// from the ranks above then up from the ranks below, each move within nodeGap of the boxes beside it and on the grid
// of 6, so that a box stands over the boxes it leads to: boxes of one link in and one out first, then the rest as
// written. Points are not pulled: wire puts each at its link's column. It stops when nothing moves, or after
// maxPulls, and returns the pulls taken.
func (gr *group) pull() int {
	pull := 0
	for pull < maxPulls {
		pull++
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
			break
		}
	}
	return pull
}

func (gr *group) pullOrder(r int) []int {
	order := gr.boxesOf(gr.ranks[r])
	class := func(i int) int {
		if n := gr.nodes[i]; len(n.ins) == 1 && len(n.outs) == 1 {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(class(a)-class(b), gr.nodes[a].written-gr.nodes[b].written)
	})
	return order
}

// pullOne moves box i of rank r towards the median centre of the boxes its edges reach above (down) or below, and
// reports whether it moved.
func (gr *group) pullOne(r, i int, down bool) bool {
	n := &gr.nodes[i]
	var centres []float64
	if down {
		for _, l := range n.ins {
			o := gr.nodes[gr.through(gr.links[l].from, true)]
			centres = append(centres, o.x+o.w/2)
		}
	} else {
		for _, l := range n.outs {
			o := gr.nodes[gr.through(gr.links[l].to, false)]
			centres = append(centres, o.x+o.w/2)
		}
	}
	if len(centres) == 0 {
		return false
	}
	want := median(centres)
	lo, hi := math.Inf(-1), math.Inf(1)
	boxes := gr.boxesOf(gr.ranks[r])
	at := slices.Index(boxes, i)
	if at > 0 {
		o := gr.nodes[boxes[at-1]]
		lo = onGrid(o.x+o.w+nodeGap, 6, math.Ceil)
	}
	if at+1 < len(boxes) {
		o := gr.nodes[boxes[at+1]]
		hi = onGrid(o.x-nodeGap-n.w, 6, math.Floor)
	}
	x := min(max(onGrid(want-n.w/2, 6, math.Round), lo), hi)
	if x == n.x {
		return false
	}
	n.x = x
	return true
}

// drawn is n's area as drawn: as laid out, or turned for right.
func drawnOf(n gnode, right bool) rect {
	if right {
		return rect{x: n.y, y: n.x, w: n.h, h: n.w, what: "", at: ""}
	}
	return rect{x: n.x, y: n.y, w: n.w, h: n.h, what: "", at: ""}
}
