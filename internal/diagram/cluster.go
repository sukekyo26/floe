package diagram

import (
	"cmp"
	"math"
	"slices"
	"strconv"

	"github.com/sukekyo26/floe/internal/usererr"
)

// maxNodes is how many boxes and groups a graph holds, counted through its groups: the schema bounds each list of
// nodes alone.
const maxNodes = 200

// gbox is a box of a graph, read flat: its JSON pointer and its innermost group (-1 for none).
type gbox struct {
	box *graphBox
	at  string
	cl  int
}

// cluster is a group of a graph's nodes (a graph's `group`, named so apart from a group of connected boxes): its
// parent (-1 for none), every box within it, nested too, and its own nodes in the order written.
type cluster struct {
	g      *graphGroup
	at     string
	parent int
	boxes  []int
	kids   []gref
}

// gref is a node of a graph written, a box or a group, by its index among the boxes or groups read (the other
// is -1): a node a group or the graph holds, or an end of an edge.
type gref struct{ box, cl int }

// flatten reads g's nodes, as written, depth first on a stack of its own: its boxes and groups, and the nodes at its
// top.
func flatten(g *graph, at string) (boxes []gbox, clusters []cluster, top []gref) {
	type level struct {
		nodes []graphNode
		at    string
		cl    int
		next  int
	}
	stack := []level{{nodes: g.Nodes, at: at, cl: -1, next: 0}}
	for len(stack) > 0 {
		lv := &stack[len(stack)-1]
		if lv.next == len(lv.nodes) {
			stack = stack[:len(stack)-1]
			continue
		}
		n, nat, cl := lv.nodes[lv.next], lv.at+"/nodes/"+strconv.Itoa(lv.next), lv.cl
		lv.next++
		var c gref
		if n.Box != nil {
			c = gref{box: len(boxes), cl: -1}
			boxes = append(boxes, gbox{box: n.Box, at: nat + "/box", cl: cl})
			for p := cl; p >= 0; p = clusters[p].parent {
				clusters[p].boxes = append(clusters[p].boxes, c.box)
			}
		} else {
			c = gref{box: -1, cl: len(clusters)}
			clusters = append(clusters, cluster{g: n.Group, at: nat + "/group", parent: cl, boxes: nil, kids: nil})
			stack = append(stack, level{nodes: n.Group.Nodes, at: nat + "/group", cl: c.cl, next: 0})
		}
		if cl < 0 {
			top = append(top, c)
		} else {
			clusters[cl].kids = append(clusters[cl].kids, c)
		}
	}
	return boxes, clusters, top
}

// holds reports whether group c holds end e, or is it.
func (o *graphOut) holds(c int, e gref) bool {
	p := e.cl
	if e.box >= 0 {
		p = o.boxes[e.box].cl
	}
	for ; p >= 0; p = o.clusters[p].parent {
		if p == c {
			return true
		}
	}
	return false
}

// endName is how a mistake names end e.
func (o *graphOut) endName(e gref) string {
	if e.box >= 0 {
		return o.boxes[e.box].box.Name
	}
	return o.clusters[e.cl].g.Name
}

// inside is the mistake of edge k between a group and what it holds, at the end held (at its to when it is the
// group itself), or nil.
func (o *graphOut) inside(k int, ends [2]gref) *usererr.Error {
	e := o.g.Edges[k]
	for i, g := range ends {
		if g.cl < 0 {
			continue
		}
		held := 1 - i
		if !o.holds(g.cl, ends[held]) {
			continue
		}
		if ends[0] == ends[1] {
			held = 1
		}
		key := [2]string{"from", "to"}[held]
		return usererr.NewIn("", edgeAt(o.at, k)+"/"+key, "diagram_graph_inside",
			e.From, e.To, o.endName(g), o.endName(g))
	}
	return nil
}

// nodeKind is what a gnode is besides a box or a point (its zero kind): a group's left or right mark in a rank, the
// room of its heading, or where an edge to it enters (in) or leaves (out) its frame.
type nodeKind int

const (
	kindPlain nodeKind = iota // a box, or a point an edge passes
	kindLeft
	kindRight
	kindHeading
	kindIn
	kindOut
)

// isPoint reports whether n is a point an edge passes.
func (n gnode) isPoint() bool { return n.box == nil && n.kind == kindPlain }

// settles reports whether settle places n at the column its link comes down from: a point, or where an edge
// enters a frame.
func (n gnode) settles() bool { return n.isPoint() || n.kind == kindIn }

// rigid reports whether n stands where place puts it, settle moving it no more: a group's marks, its heading's room
// and where an edge leaves its frame.
func (n gnode) rigid() bool { return n.kind != kindPlain && n.kind != kindIn }

// shared is the innermost group holding both group a and group b (-1 for none, or when either is).
func (o *graphOut) shared(a, b int) int {
	if a < 0 || b < 0 {
		return -1
	}
	up := map[int]bool{}
	for ; a >= 0; a = o.clusters[a].parent {
		up[a] = true
	}
	for b >= 0 && !up[b] {
		b = o.clusters[b].parent
	}
	return b
}

// spans are the first and last ranks of each group of the group of boxes, by its boxes, nested too.
func (gr *group) spans() map[int][2]int {
	out := map[int][2]int{}
	for _, n := range gr.nodes {
		if n.box == nil {
			continue
		}
		for c := n.cl; c >= 0; c = gr.out.clusters[c].parent {
			s, ok := out[c]
			if !ok {
				s = [2]int{n.rank, n.rank}
			}
			out[c] = [2]int{min(s[0], n.rank), max(s[1], n.rank)}
		}
	}
	return out
}

// mark puts each group's left and right marks in every rank it spans, and the room of its heading (its label's
// width, rounded up so that the frame holds it measured again, by a row, turned for right later by place) first in
// its first rank, then lists each rank as the groups hold it: in each group, its left mark and its heading, its nodes
// as written (a box, or a group nested), its points and ports by their edges' order, and its right mark, so that a
// group stands side by side in every rank.
//
// LIMIT: an edge between a box of a group and one outside meets the frame's side, not its top or bottom, when the
// box inside is past the group's first rank (an edge in) or before its last (out): its points pass by the group's
// marks, not through a port. Give such edges ports too if frames come to look crossed.
// LIMIT: a long heading widens its group (down), its room standing before the boxes of the first rank, or deepens
// the first rank (right). Give the heading a rank's room of its own if long labels come to look stretched.
func (gr *group) mark() {
	spans := gr.spans()
	marks := map[[2]int][2]int{} // a group's left and right marks in a rank
	heading := map[int]int{}
	add := func(c, r int, kind nodeKind, w, h float64) int {
		gr.nodes = append(gr.nodes, gnode{
			box: nil, of: c, kind: kind, cl: c, rank: r, w: w, h: h, x: 0, y: 0, ins: nil, outs: nil,
			written: len(gr.nodes),
		})
		return len(gr.nodes) - 1
	}
	for _, c := range gr.cls {
		for r := spans[c][0]; r <= spans[c][1]; r++ {
			marks[[2]int{c, r}] = [2]int{add(c, r, kindLeft, 0, 0), add(c, r, kindRight, 0, 0)}
		}
		if t := gr.out.clusters[c].g.Label; t != nil {
			heading[c] = add(c, spans[c][0], kindHeading, math.Ceil(textWidth(t, sizeSub, false)), labelH)
		}
	}
	gr.list(spans, marks, heading)
}

// list lists each rank as the groups hold it (see mark).
func (gr *group) list(spans map[int][2]int, marks map[[2]int][2]int, heading map[int]int) {
	ls := listing{gr: gr, spans: spans, marks: marks, heading: heading, boxAt: map[int]int{}, others: map[[2]int][]int{}}
	for i, n := range gr.nodes {
		if n.box != nil {
			ls.boxAt[n.of] = i
		}
	}
	for r, rank := range gr.ranks {
		for _, i := range rank {
			if n := gr.nodes[i]; n.box == nil {
				ls.others[[2]int{n.cl, r}] = append(ls.others[[2]int{n.cl, r}], i)
			}
		}
	}
	for r := range gr.ranks {
		gr.ranks[r] = ls.emit(-1, r)
	}
}

// listing is what list lists ranks by: each group's span and marks, its heading's room, each box's node, and the
// points and ports of each group in each rank, as listed before.
type listing struct {
	gr      *group
	spans   map[int][2]int
	marks   map[[2]int][2]int
	heading map[int]int
	boxAt   map[int]int
	others  map[[2]int][]int
}

// emit is group c's nodes in rank r, as list lists them (c < 0: the graph's).
func (ls listing) emit(c, r int) []int {
	o := ls.gr.out
	var out []int
	kids := o.top
	if c >= 0 {
		out = append(out, ls.marks[[2]int{c, r}][0])
		if h, ok := ls.heading[c]; ok && ls.spans[c][0] == r {
			out = append(out, h)
		}
		kids = o.clusters[c].kids
	}
	for _, k := range kids {
		if i, ok := ls.boxAt[k.box]; k.box >= 0 && ok && ls.gr.nodes[i].rank == r {
			out = append(out, i)
		}
		if s, ok := ls.spans[k.cl]; k.cl >= 0 && ok && s[0] <= r && r <= s[1] {
			out = append(out, ls.emit(k.cl, r)...)
		}
	}
	out = append(out, ls.others[[2]int{c, r}]...)
	if c >= 0 {
		out = append(out, ls.marks[[2]int{c, r}][1])
	}
	return out
}

// unit is a run of a rank within group c: one node of c's own, or every node of a group nested in c (cl).
type unit struct {
	cl    int // -1 for a node of c's own
	nodes []int
}

// units splits seq, the nodes of group c (-1 for the graph) in a rank but c's marks and heading, into its units.
func (gr *group) units(seq []int, c int) []unit {
	var out []unit
	for _, i := range seq {
		d := gr.nodes[i].cl
		for d >= 0 && d != c && gr.out.clusters[d].parent != c {
			d = gr.out.clusters[d].parent
		}
		if d == c {
			d = -1
		}
		if n := len(out); d >= 0 && n > 0 && out[n-1].cl == d {
			out[n-1].nodes = append(out[n-1].nodes, i)
			continue
		}
		out = append(out, unit{cl: d, nodes: []int{i}})
	}
	return out
}

// pinned splits seq, group c's nodes in a rank, into its left mark and heading (head), the rest, and its right
// mark (tail); the graph's (c < 0) have neither.
func (gr *group) pinned(seq []int, c int) (head, rest, tail []int) {
	if c < 0 {
		return nil, seq, nil
	}
	n := 1
	if len(seq) > 2 && gr.nodes[seq[1]].kind == kindHeading {
		n = 2
	}
	return seq[:n], seq[n : len(seq)-1], seq[len(seq)-1:]
}

// sortIn orders seq, group c's nodes in a rank, by keys as sweep does, a group nested by the mean of its nodes' keys:
// the units with no key keep their places, and the others fill those left, stably; then each group nested inside.
func (gr *group) sortIn(seq []int, c int, keys map[int]float64) []int {
	head, rest, tail := gr.pinned(seq, c)
	us := gr.units(rest, c)
	key := map[int]float64{}
	var moving []int
	for j, u := range us {
		sum, n := 0.0, 0
		for _, i := range u.nodes {
			if k, ok := keys[i]; ok {
				sum, n = sum+k, n+1
			}
		}
		if n > 0 {
			key[j] = sum / float64(n)
			moving = append(moving, j)
		}
	}
	slices.SortStableFunc(moving, func(a, b int) int { return cmp.Compare(key[a], key[b]) })
	order := make([]int, len(us))
	k := 0
	for j := range us {
		order[j] = j
		if _, ok := key[j]; ok {
			order[j] = moving[k]
			k++
		}
	}
	out := slices.Clone(head)
	for _, j := range order {
		u := us[j]
		if u.cl >= 0 {
			out = append(out, gr.sortIn(u.nodes, u.cl, keys)...)
		} else {
			out = append(out, u.nodes...)
		}
	}
	return append(out, tail...)
}

// align orders the groups nested in each group (or in the graph) one way in every rank, by where their left marks
// stand on average (ties as written), so that their frames, each one rectangle, keep apart.
func (gr *group) align() {
	pos := gr.positions()
	sum, count := map[int]float64{}, map[int]int{}
	for i, n := range gr.nodes {
		if n.kind == kindLeft {
			sum[n.cl] += float64(pos[i])
			count[n.cl]++
		}
	}
	rank := func(c int) float64 { return sum[c] / float64(count[c]) }
	var walk func(seq []int, c int) []int
	walk = func(seq []int, c int) []int {
		head, rest, tail := gr.pinned(seq, c)
		us := gr.units(rest, c)
		var slots []int
		for j, u := range us {
			if u.cl >= 0 {
				slots = append(slots, j)
			}
		}
		groups := make([]unit, len(slots))
		for j, s := range slots {
			groups[j] = us[s]
		}
		slices.SortStableFunc(groups, func(a, b unit) int { return cmp.Or(cmp.Compare(rank(a.cl), rank(b.cl)), a.cl-b.cl) })
		for j, s := range slots {
			us[s] = groups[j]
		}
		out := slices.Clone(head)
		for _, u := range us {
			if u.cl >= 0 {
				out = append(out, walk(u.nodes, u.cl)...)
			} else {
				out = append(out, u.nodes...)
			}
		}
		return append(out, tail...)
	}
	for r, seq := range gr.ranks {
		gr.ranks[r] = walk(seq, -1)
	}
}

// gap is the room between node a and node b after it in a rank: a column between points (and ports), 12 inside a
// group's marks, twice that across an empty one, and nodeGap else.
func (gr *group) gap(a, b int) float64 {
	na, nb := gr.nodes[a], gr.nodes[b]
	thin := func(n gnode) bool { return n.isPoint() || n.kind == kindIn || n.kind == kindOut }
	switch {
	case na.kind == kindLeft && nb.kind == kindRight:
		return 2 * inEnd
	case na.kind == kindLeft, nb.kind == kindRight:
		return inEnd
	case na.kind == kindRight, nb.kind == kindLeft:
		return nodeGap
	case thin(na) && thin(nb):
		return column
	}
	return nodeGap
}

// pointRoom is the room settle needs for a point between nodes it does not move: a column to reach x ≡ 6 from where
// it may first stand, and half a column to leave by, entered bent.
const pointRoom = 1.5 * column

// room is how wide node i stands when packed: a point (or where an edge enters a frame) as settle may need it.
func (gr *group) room(i int) float64 {
	if gr.nodes[i].settles() {
		return pointRoom
	}
	return gr.nodes[i].w
}

// pack sets the nodes' lefts, as laid out down, as far left as every rank lets them: each node a gap past the one
// before it in its rank, a group's left and right marks one place in all its ranks, so that its frame is one
// rectangle (the longest path of those constraints, in Kahn's way). The points take their room too, so that settle
// has room for each where it is placed, the marks unmoved.
func (gr *group) pack() {
	n := len(gr.nodes)
	at := map[int]int{} // each group's place among the group's
	for j, c := range gr.cls {
		at[c] = j
	}
	slot := func(i int) int {
		switch nd := gr.nodes[i]; nd.kind {
		case kindLeft:
			return n + 2*at[nd.cl]
		case kindRight:
			return n + 2*at[nd.cl] + 1
		case kindPlain, kindHeading, kindIn, kindOut:
		}
		return i
	}
	type step struct {
		to int
		by float64
	}
	next := make([][]step, n+2*len(gr.cls))
	into := make([]int, len(next))
	for _, rank := range gr.ranks {
		for k := 1; k < len(rank); k++ {
			a, b := rank[k-1], rank[k]
			next[slot(a)] = append(next[slot(a)], step{slot(b), gr.room(a) + gr.gap(a, b)})
			into[slot(b)]++
		}
	}
	x := make([]float64, len(next))
	var queue []int
	for s := range next {
		if into[s] == 0 {
			queue = append(queue, s)
		}
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		step := 6.0
		if s < n && gr.nodes[s].kind == kindOut { // an exit stands at x ≡ 0
			step = column
		}
		x[s] = onGrid(x[s], step, math.Ceil)
		for _, st := range next[s] {
			x[st.to] = max(x[st.to], x[s]+st.by)
			if into[st.to]--; into[st.to] == 0 {
				queue = append(queue, st.to)
			}
		}
	}
	for i := range gr.nodes {
		gr.nodes[i].x = x[slot(i)]
	}
}

// room is the room above (or below) a rank for the frames starting (or ending) there: the most, and each group's.
type room struct {
	room float64
	of   map[int]float64
}

// The room above a rank, for each group starting there, by its label, and below one, for each ending there.
const (
	roomHeaded = 30.0 // a heading's 18 and its 9 above and 3 below
	roomBare   = 12.0
)

// rooms are the room above and below each rank: a group starting at a rank has its own room above it, and that of
// the groups nested in it starting there too, outermost highest; one ending, its own below and theirs.
func (gr *group) rooms() (above, below []room) {
	above, below = make([]room, len(gr.ranks)), make([]room, len(gr.ranks))
	for r := range gr.ranks {
		above[r].of, below[r].of = map[int]float64{}, map[int]float64{}
	}
	if len(gr.cls) == 0 {
		return above, below
	}
	spans := gr.spans()
	o := gr.out
	var depth func(c, end int, top bool) float64
	depth = func(c, end int, top bool) float64 {
		own := roomBare
		if top && o.clusters[c].g.Label != nil {
			own = roomHeaded
		}
		inner := 0.0
		for _, k := range o.clusters[c].kids {
			if s, ok := spans[k.cl]; k.cl >= 0 && ok && (top && s[0] == end || !top && s[1] == end) {
				inner = max(inner, depth(k.cl, end, top))
			}
		}
		return own + inner
	}
	for _, c := range gr.cls {
		s := spans[c]
		a, b := depth(c, s[0], true), depth(c, s[1], false)
		above[s[0]].of[c], below[s[1]].of[c] = a, b
		above[s[0]].room, below[s[1]].room = max(above[s[0]].room, a), max(below[s[1]].room, b)
	}
	return above, below
}

// below is the room below rank r for the frames ending there.
func (gr *group) below(r int) float64 {
	if r < len(gr.botRoom) {
		return gr.botRoom[r]
	}
	return 0
}
