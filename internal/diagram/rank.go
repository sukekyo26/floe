package diagram

import (
	"slices"

	"github.com/sukekyo26/floe/internal/usererr"
)

// rank makes a group of the boxes members (their indexes among the boxes read, as written), its edges between boxes
// and its edges to groups (cedges; by their indexes in edges): an edge back up a cycle turned round, and each box at
// the rank of its longest path from a box no edge enters. The points of the edges passing ranks are made by split,
// once the diagram's are counted.
func (o *graphOut) rank(members, edges, cedges []int) *group {
	gr := &group{
		nodes: nil, links: nil, ranks: nil, chains: map[int][]int{}, reversed: map[int]bool{}, tracks: nil, top: nil,
		bottom: nil, arcs: nil, out: o, among: nil, cls: nil, frames: map[int][2]float64{}, botRoom: nil, headAt: 0,
	}
	local := map[int]int{}
	for j, i := range members {
		local[i] = j
		gr.nodes = append(gr.nodes, gnode{
			box: o.boxes[i].box, of: i, kind: kindPlain, cl: o.boxes[i].cl, rank: 0, w: 0, h: 0, x: 0, y: 0, ins: nil,
			outs: nil, written: j,
		})
	}
	arcs := make([]arc, 0, len(edges)+len(cedges))
	outs := make([][]int, len(members)) // the arcs out of each box, as written
	for _, k := range edges {
		from, to := local[o.ends[k][0].box], local[o.ends[k][1].box]
		outs[from] = append(outs[from], len(arcs))
		arcs = append(arcs, arc{from, to, k})
	}
	turn(arcs, outs, gr.reversed)
	for c := range o.clusters {
		if _, ok := local[o.clusters[c].boxes[0]]; ok {
			gr.cls = append(gr.cls, c)
		}
	}
	if len(cedges) == 0 {
		gr.rankLongest(arcs)
		gr.arcs = arcs
		return gr
	}
	gr.arcs = gr.rankWith(arcs, cedges, local)
	return gr
}

// rankWith ranks the group's boxes by its arcs and its edges to groups (cedges), and returns the arcs with those
// edges' added, each from or to a port of its group's (kindOut, kindIn). An edge to a group G joins the arcs as G's
// top (T) or bottom (B), stand-ins of the group's that every box of it is at or below (T) or at or above (B): X → G
// as X's bottom → G's top, G a rank or more below X, entering its frame's top; but where that makes a cycle, as G's
// bottom → X's top, drawn up, leaving its frame's bottom; where both make one, X stands among G's ranks: told, and
// not laid out. Each edge is added in the order written, so that an earlier one takes its way.
// LIMIT: the way taken is the first that makes no cycle, not the one turning the fewest edges, as with turn.
func (gr *group) rankWith(arcs []arc, cedges []int, local map[int]int) []arc {
	rk := newRanking(gr.out, local, len(gr.nodes))
	for _, a := range arcs {
		rk.add(a.from, a.to, 1)
	}
	var forms []edgeForm
	for _, k := range cedges {
		ends := gr.out.ends[k]
		up0, lo0, up1, lo1 := rk.upper(ends[0]), rk.lower(ends[0]), rk.upper(ends[1]), rk.lower(ends[1])
		switch {
		case !rk.reached(lo1)[up0]:
			rk.add(up0, lo1, 1)
			forms = append(forms, edgeForm{k, ends, false})
		case !rk.reached(lo0)[up1]:
			rk.add(up1, lo0, 1)
			forms = append(forms, edgeForm{k, [2]gref{ends[1], ends[0]}, true})
		default:
			gr.among = append(gr.among, gr.amid(k, local, rk.reached))
		}
	}
	for j, r := range rk.longest()[:len(gr.nodes)] {
		gr.nodes[j].rank = r
	}
	for _, f := range forms {
		arcs = append(arcs, gr.ported(f, local))
		gr.reversed[f.k] = f.reversed
	}
	return arcs
}

// edgeForm is an edge to a group as ranked: its upper end and its lower, as laid out down, and whether it is drawn
// against the way it is written.
type edgeForm struct {
	k        int
	ends     [2]gref
	reversed bool
}

// ported is the arc of edge f, its ends at a box or at a port of a group's, made here: where an edge leaves a frame
// (kindOut, at its group's last rank) for an upper end, where it enters one (kindIn, at its first) for a lower.
func (gr *group) ported(f edgeForm, local map[int]int) arc {
	var at [2]int
	for i, e := range f.ends {
		if e.box >= 0 {
			at[i] = local[e.box]
			continue
		}
		kind, r := kindOut, 0
		if i == 1 {
			kind, r = kindIn, len(gr.nodes)
		}
		for _, b := range gr.out.clusters[e.cl].boxes {
			if rb := gr.nodes[local[b]].rank; i == 1 {
				r = min(r, rb)
			} else {
				r = max(r, rb)
			}
		}
		at[i] = len(gr.nodes)
		gr.nodes = append(gr.nodes, gnode{
			box: nil, of: f.k, kind: kind, cl: e.cl, rank: r, w: 0, h: 0, x: 0, y: 0, ins: nil, outs: nil,
			written: len(gr.nodes),
		})
	}
	return arc{at[0], at[1], f.k}
}

// ranking is the constraints a group's boxes are ranked by: arcs of a length (1 for an edge, 0 from a group's T to
// its boxes and from them to its B), between the boxes (by their index in the group) and the T and B of the groups
// edges end at, made as needed.
type ranking struct {
	o     *graphOut
	local map[int]int
	arcs  []warc
	out   [][]int     // the arcs out of each node, by index in arcs
	tops  map[int]int // each group's T; its B follows
}

type warc struct{ from, to, w int }

func newRanking(o *graphOut, local map[int]int, n int) *ranking {
	rk := &ranking{o: o, local: local, arcs: nil, out: make([][]int, 0, n), tops: map[int]int{}}
	for range n {
		rk.out = append(rk.out, nil)
	}
	return rk
}

func (rk *ranking) add(a, b, w int) {
	rk.out[a] = append(rk.out[a], len(rk.arcs))
	rk.arcs = append(rk.arcs, warc{a, b, w})
}

// stand is group c's T, made with its B and their arcs the first time.
func (rk *ranking) stand(c int) int {
	if t, ok := rk.tops[c]; ok {
		return t
	}
	t := len(rk.out)
	rk.tops[c] = t
	rk.out = append(rk.out, nil, nil)
	for _, b := range rk.o.clusters[c].boxes {
		rk.add(t, rk.local[b], 0)
		rk.add(rk.local[b], t+1, 0)
	}
	return t
}

// upper is the node an edge leaves end e by, down: the box, or the group's B; lower, the node it enters e by.
func (rk *ranking) upper(e gref) int {
	if e.box >= 0 {
		return rk.local[e.box]
	}
	return rk.stand(e.cl) + 1
}

func (rk *ranking) lower(e gref) int {
	if e.box >= 0 {
		return rk.local[e.box]
	}
	return rk.stand(e.cl)
}

// reached are the nodes reached from the nodes from by the arcs, breadth first, from among them.
func (rk *ranking) reached(from ...int) map[int]bool {
	seen := map[int]bool{}
	for _, u := range from {
		seen[u] = true
	}
	queue := slices.Clone(from)
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, i := range rk.out[u] {
			if v := rk.arcs[i].to; !seen[v] {
				seen[v] = true
				queue = append(queue, v)
			}
		}
	}
	return seen
}

// longest is each node's rank: its longest path, by the arcs' lengths, from a node no arc enters (Kahn's way).
func (rk *ranking) longest() []int {
	ranks := make([]int, len(rk.out))
	into := make([]int, len(rk.out))
	for _, a := range rk.arcs {
		into[a.to]++
	}
	var queue []int
	for i := range rk.out {
		if into[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, i := range rk.out[u] {
			a := rk.arcs[i]
			ranks[a.to] = max(ranks[a.to], ranks[u]+a.w)
			if into[a.to]--; into[a.to] == 0 {
				queue = append(queue, a.to)
			}
		}
	}
	return ranks
}

// amid is the mistake of edge k, to a group G whose other end X stands among G's ranks: a box of G leads to X, and X
// to a box of G, the first of each as written. X is the box when one end is, else its from.
func (gr *group) amid(k int, local map[int]int, reached func(from ...int) map[int]bool) *usererr.Error {
	o := gr.out
	ends := o.ends[k]
	x, g, key := 0, 1, "from"
	if ends[0].box < 0 && ends[1].box >= 0 {
		x, g, key = 1, 0, "to"
	}
	members := o.members(ends[x])
	xs := make([]int, 0, len(members))
	for _, m := range members {
		xs = append(xs, local[m])
	}
	fromX := reached(xs...)
	var before, after string
	for _, b := range o.clusters[ends[g].cl].boxes {
		if after == "" && fromX[local[b]] {
			after = o.boxes[b].box.Name
		}
		if before == "" {
			fromB := reached(local[b])
			if slices.ContainsFunc(xs, func(m int) bool { return fromB[m] }) {
				before = o.boxes[b].box.Name
			}
		}
	}
	e := o.g.Edges[k]
	name := o.endName(ends[g])
	return usererr.NewIn("", edgeAt(o.at, k)+"/"+key, "diagram_graph_among",
		e.From, e.To, o.endName(ends[x]), name, before, after, name)
}

// passed is how many ranks the group's arcs pass between their ends: the points split makes.
func (gr *group) passed() int {
	n := 0
	for _, a := range gr.arcs {
		n += gr.nodes[a.to].rank - gr.nodes[a.from].rank - 1
	}
	return n
}

// split breaks each arc passing ranks into links by a point at each rank it passes, and lists the nodes of each
// rank: the boxes as written, then the points by their edge's order.
func (gr *group) split() {
	for _, a := range gr.arcs {
		prev := a.from
		for r := gr.nodes[a.from].rank + 1; r <= gr.nodes[a.to].rank; r++ {
			next := a.to
			if r < gr.nodes[a.to].rank {
				next = len(gr.nodes)
				gr.nodes = append(gr.nodes, gnode{
					box: nil, of: a.edge, kind: kindPlain, cl: gr.out.shared(gr.nodes[a.from].cl, gr.nodes[a.to].cl),
					rank: r, w: 0, h: 0, x: 0, y: 0, ins: nil, outs: nil, written: len(gr.nodes),
				})
			}
			gr.link(prev, next, a.edge)
			prev = next
		}
	}
	for i, nd := range gr.nodes {
		for len(gr.ranks) <= nd.rank {
			gr.ranks = append(gr.ranks, nil)
		}
		gr.ranks[nd.rank] = append(gr.ranks[nd.rank], i)
	}
	if len(gr.cls) > 0 {
		gr.mark()
	}
}

func (gr *group) link(from, to, edge int) {
	gr.nodes[from].outs = append(gr.nodes[from].outs, len(gr.links))
	gr.nodes[to].ins = append(gr.nodes[to].ins, len(gr.links))
	gr.chains[edge] = append(gr.chains[edge], len(gr.links))
	gr.links = append(gr.links, glink{from: from, to: to, edge: edge, exit: 0, entry: 0, straight: false, track: 0})
}

// arc is an edge of a group between two of its boxes, by their indexes in its nodes.
type arc struct{ from, to, edge int }

// turn turns round each arc back up a cycle, found depth first from each box as written on a stack of its own (no
// recursion): an arc to a box on the way. outs are the arcs out of each box, as written.
// LIMIT: the arcs turned are not the fewest that break every cycle (that is NP-hard); find fewer with a heuristic
// (Eades) if a graph turns more than it needs.
func turn(arcs []arc, outs [][]int, reversed map[int]bool) {
	const (
		unseen = iota
		onWay
		done
	)
	state := make([]int, len(outs))
	type frame struct{ node, next int }
	for s := range outs {
		if state[s] != unseen {
			continue
		}
		stack := []frame{{s, 0}}
		state[s] = onWay
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next == len(outs[top.node]) {
				state[top.node] = done
				stack = stack[:len(stack)-1]
				continue
			}
			a := &arcs[outs[top.node][top.next]]
			top.next++
			switch state[a.to] {
			case onWay:
				reversed[a.edge] = true
				a.from, a.to = a.to, a.from
			case unseen:
				state[a.to] = onWay
				stack = append(stack, frame{a.to, 0})
			}
		}
	}
}

// rankLongest ranks each box by its longest path from a box no arc enters (Kahn's way), taking boxes as written.
// LIMIT: the longest path, not the ranks of fewest points (network simplex): a box leading to one far below sits
// high, its edge long. Balance the ranks if graphs come to pass many.
func (gr *group) rankLongest(arcs []arc) {
	n := len(gr.nodes)
	into := make([]int, n)
	down := make([][]int, n)
	for i, a := range arcs {
		into[a.to]++
		down[a.from] = append(down[a.from], i)
	}
	var queue []int
	for i := range n {
		if into[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, i := range down[u] {
			v := arcs[i].to
			gr.nodes[v].rank = max(gr.nodes[v].rank, gr.nodes[u].rank+1)
			if into[v]--; into[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
}
