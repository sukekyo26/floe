package diagram

import (
	"cmp"
	"iter"
	"math"
	"slices"
	"strconv"

	"github.com/sukekyo26/floe/internal/usererr"
)

// maxNodes is how many boxes and groups a graph holds, counted through its groups: the schema bounds each list of
// nodes or lanes alone.
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
	lane   bool // a lane: one of the graph's lanes
	boxes  []int
	kids   []gref
}

// gref is a node of a graph written, a box or a group, by its index among the boxes or groups read (the other
// is -1): a node a group or the graph holds, or an end of an edge.
type gref struct{ box, cl int }

// flatBox is node n at nat in group cl, a box or an icon, read flat as a box.
func flatBox(n graphNode, nat string, cl int) gbox {
	if n.Icon != nil {
		//nolint:exhaustruct_v5 // an icon read flat as a box
		box := &graphBox{icon: n.Icon, Name: n.Icon.Name, With: n.Icon.With, Shape: "icon"}
		return gbox{box: box, at: nat + "/icon", cl: cl}
	}
	return gbox{box: n.Box, at: nat + "/box", cl: cl}
}

// flatten reads g's nodes, or its lanes, as written, depth first on a stack of its own: its boxes and groups, and the
// nodes at its top.
func flatten(g *graph, at string) (boxes []gbox, clusters []cluster, top []gref) {
	type level struct {
		nodes []graphNode
		at    string
		cl    int
		next  int
	}
	add := func(c gref, cl int) {
		if cl < 0 {
			top = append(top, c)
		} else {
			clusters[cl].kids = append(clusters[cl].kids, c)
		}
	}
	walk := func(nodes []graphNode, at string, cl int) {
		stack := []level{{nodes: nodes, at: at, cl: cl, next: 0}}
		for len(stack) > 0 {
			lv := &stack[len(stack)-1]
			if lv.next == len(lv.nodes) {
				stack = stack[:len(stack)-1]
				continue
			}
			n, nat, cl := lv.nodes[lv.next], lv.at+"/nodes/"+strconv.Itoa(lv.next), lv.cl
			lv.next++
			var c gref
			if n.Box != nil || n.Icon != nil {
				c = gref{box: len(boxes), cl: -1}
				boxes = append(boxes, flatBox(n, nat, cl))
				for p := cl; p >= 0; p = clusters[p].parent {
					clusters[p].boxes = append(clusters[p].boxes, c.box)
				}
			} else {
				c = gref{box: -1, cl: len(clusters)}
				clusters = append(clusters, cluster{g: n.Group, at: nat + "/group", parent: cl, lane: false, boxes: nil, kids: nil})
				stack = append(stack, level{nodes: n.Group.Nodes, at: nat + "/group", cl: c.cl, next: 0})
			}
			add(c, cl)
		}
	}
	if len(g.Lanes) == 0 {
		walk(g.Nodes, at, -1)
	}
	for i := range g.Lanes {
		lat, c := at+"/lanes/"+strconv.Itoa(i), gref{box: -1, cl: len(clusters)}
		clusters = append(clusters, cluster{g: &g.Lanes[i], at: lat, parent: -1, lane: true, boxes: nil, kids: nil})
		add(c, -1)
		walk(g.Lanes[i].Nodes, lat, c.cl)
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

// lane reports whether group c is a lane: at the top of a graph of lanes.
func (o *graphOut) lane(c int) bool {
	return c >= 0 && o.clusters[c].lane
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

// pointIn is the group the point of arc a at rank r stands in: of the groups holding either end, the innermost
// whose ranks r is among, the lower end's on a tie (-1 for none), so that an edge between groups passes inside
// one where it can, not round their frames. A group never takes a point at a rank past its own, so a point never
// stretches a frame.
func (gr *group) pointIn(spans map[int][2]int, a arc, r int) int {
	depth := func(c int) int {
		d := 0
		for ; c >= 0; c = gr.out.clusters[c].parent {
			d++
		}
		return d
	}
	spanning := func(c int) int {
		for ; c >= 0; c = gr.out.clusters[c].parent {
			if s, ok := spans[c]; ok && s[0] <= r && r <= s[1] {
				return c
			}
		}
		return -1
	}
	lower, upper := spanning(gr.nodes[a.to].cl), spanning(gr.nodes[a.from].cl)
	if depth(upper) > depth(lower) {
		return upper
	}
	return lower
}

// spans are the first and last ranks of each group of the group of boxes, by its boxes, nested too; a lane spans
// every rank, the last taken from the nodes, as gr.ranks may not be made yet.
func (gr *group) spans() map[int][2]int {
	out := map[int][2]int{}
	last := 0
	for _, n := range gr.nodes {
		last = max(last, n.rank)
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
	for c := range out {
		if gr.out.lane(c) {
			out[c] = [2]int{0, last}
		}
	}
	return out
}

// mark puts each group's left and right marks in every rank it spans, and notes its heading (its label's width, rounded
// up so that the frame holds it measured again, by a row): laid out down in heads, above the rank (headRoom); laid out
// right as a room first in its first rank, turned later by place, as a heading may reach into the rank there. It then
// lists each rank as the groups hold it: in each group, its left mark and any heading's room, its nodes as written (a
// box, or a group nested), its points and ports by their edges' order, and its right mark, so that a group stands side
// by side in every rank.
//
// LIMIT: an edge between a box of a group and one outside meets the frame's side, not its top or bottom, when the
// box inside is past the group's first rank (an edge in) or before its last (out): its points pass by the group's
// marks, not through a port. Give such edges ports too if frames come to look crossed.
// LIMIT: a long heading, and an icon with it, widens its group as far as it holds the heading and what a line reaches
// from above (down), or deepens the first rank (right). Give the heading a rank's room of its own if long labels come
// to look stretched.
// LIMIT: boxes under a heading start a layout elsewhere, and widen and pull may then widen a few graphs, as narrowing
// a heading alone does: of 1550 random graphs laid out down, the area of 30 grows, of one by 29%, and of 410 shrinks.
// Look into widen if laid-out graphs come to look stretched.
// LIMIT: a line written by hand from above into a box of a group's first rank that no edge of the graph reaches may run
// through the heading, as this does not read those edges, and is told; go round it by via. Read those edges here if
// writers come to meet it often.
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
		if g := gr.out.clusters[c].g; headed(g) {
			a := headArea(g)
			w := math.Ceil(a.x + a.w - headParts(false, 0).x)
			if gr.out.g.Direction == "right" {
				heading[c] = add(c, spans[c][0], kindHeading, w, a.h)
			} else {
				gr.heads[c] = headRoom{left: marks[[2]int{c, spans[c][0]}][0], w: w}
			}
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

// unit is a run of a rank within group c: one node of c's own, a block of c's boxes (see level.go), or every node of
// a group nested in c (cl).
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
		if n := len(out); n > 0 && out[n-1].cl == d && (d >= 0 || gr.sameBlock(out[n-1].nodes[0], i)) {
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
// Lanes have no key.
func (gr *group) sortIn(seq []int, c int, keys map[int]float64) []int {
	head, rest, tail := gr.pinned(seq, c)
	us := gr.units(rest, c)
	key := map[int]float64{}
	var moving []int
	for j, u := range us {
		if gr.out.lane(u.cl) {
			continue
		}
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
	if gr.out.lane(c) { // a lane's blocks facing another lane stand at its end towards it, the rest as ordered
		slices.SortStableFunc(order, func(a, b int) int { return gr.facing(us[a]) - gr.facing(us[b]) })
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
	pos := gr.places()
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
// it may first stand, and half a column to leave by, entered bent there. A point at a free column of its own may
// stand further, up to freeLimit, as far as the nodes after it leave it room (bounds).
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
func (gr *group) pack(floor map[int]float64) {
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
	next, into := gr.packArcs(n, at, slot)
	x := make([]float64, len(next))
	var was []float64 // packed again: where each stands, nothing moving left of it, nor a point left of its floor
	if floor != nil {
		was = make([]float64, len(next))
		for i := range gr.nodes {
			was[slot(i)] = gr.nodes[i].x
			x[slot(i)] = max(x[slot(i)], gr.nodes[i].x, floor[i])
		}
	}
	longest(next, into, x, func(s int, at float64) float64 {
		switch {
		case was != nil && s >= n: // a group's marks by whole columns, so that a frame moved keeps what it holds on the grid
			return was[s] + onGrid(at-was[s], column, math.Ceil)
		case s < n:
			return onGrid(at, gr.step(s), math.Ceil)
		}
		return onGrid(at, 6, math.Ceil)
	})
	for i := range gr.nodes {
		gr.nodes[i].x = x[slot(i)]
	}
}

// headRoom is a heading laid out down: it stands above its group's first rank, whose left mark is left, and takes no
// place in it, what a line reaches from above keeping right of it and the frame holding it (headsOver, pack).
type headRoom struct {
	left int
	w    float64
}

// clear is how far right of the frame's left what a line reaches from above stands.
func (h headRoom) clear() float64 { return inEnd + h.w + nodeGap }

// headsOver are the headings over node i, if a line reaches it from above its rank: a point, where an edge enters a
// frame, a box with an edge in.
func (gr *group) headsOver(i int) iter.Seq[headRoom] {
	return func(yield func(headRoom) bool) {
		n := gr.nodes[i]
		if len(gr.heads) == 0 || !n.settles() && (n.box == nil || len(n.ins) == 0) {
			return
		}
		for c := n.cl; c >= 0; c = gr.out.clusters[c].parent {
			if h, ok := gr.heads[c]; ok && gr.nodes[h.left].rank == n.rank && !yield(h) {
				return
			}
		}
	}
}

// headFloor is how far left node i may stand for the headings over it. settle keeps to it by bounds. widen need not:
// a link it widens a box left for stands at most a column from the box's end, so at its left or further right.
func (gr *group) headFloor(i int) float64 {
	lo := math.Inf(-1)
	for h := range gr.headsOver(i) {
		lo = max(lo, gr.nodes[h.left].x+h.clear())
	}
	return lo
}

// packArcs are the constraints of pack among its slots: each node past the one before it in its rank, what a line
// reaches from above past the headings over it, and each group's right mark past its heading.
func (gr *group) packArcs(n int, at map[int]int, slot func(int) int) (next [][]packArc, into []int) {
	next = make([][]packArc, n+2*len(gr.cls))
	into = make([]int, len(next))
	arc := func(from, to int, by float64) {
		next[from] = append(next[from], packArc{to, by})
		into[to]++
	}
	for _, rank := range gr.ranks {
		for k := 1; k < len(rank); k++ {
			arc(slot(rank[k-1]), slot(rank[k]), gr.room(rank[k-1])+gr.gap(rank[k-1], rank[k]))
		}
	}
	for i := range gr.nodes {
		for h := range gr.headsOver(i) {
			arc(slot(h.left), slot(i), h.clear())
		}
	}
	for _, c := range gr.cls {
		if h, ok := gr.heads[c]; ok {
			arc(slot(h.left), n+2*at[c]+1, h.w+2*inEnd)
		}
	}
	return next, into
}

// wanted is how far right each point at the graph's top, outside every group, should stand when packed again, so
// that settle can put it at the column its link comes down from: that column less its room, by whole columns from
// where it stands, so that the frames it pushes keep their places on the grid. The column is the exit exits gives a
// link from a box, as the boxes stand now; a port's x for a link from a frame; the upper point's (or port's) column
// for a link from one. A point in a group is not pushed: it would widen its own frame.
// LIMIT: the columns are exits' as the boxes stand before the second pull; a box it moves may leave a point off its
// column again. Push points in groups too, without widening their frames, if such turns come to show.
// LIMIT: in lanes every point is in a group, so none is pushed. Push points in groups too if lanes come to look bent.
func (gr *group) wanted() map[int]float64 {
	pos := gr.positions()
	for u, n := range gr.nodes {
		if n.box != nil {
			gr.exits(u, pos, false)
		}
	}
	col := map[int]float64{}
	out := map[int]float64{}
	for _, rank := range gr.ranks {
		for _, v := range rank {
			n := gr.nodes[v]
			if !n.settles() {
				continue
			}
			l := gr.links[n.ins[0]]
			c := l.exit
			switch u := gr.nodes[l.from]; {
			case u.settles():
				c = col[l.from]
			case u.box == nil: // where an edge leaves a frame
				c = u.x
			}
			col[v] = c
			if want := c - pointRoom; n.cl < 0 && want > n.x {
				out[v] = n.x + onGrid(want-n.x, column, math.Ceil)
			}
		}
	}
	return out
}

// packArc is a constraint of pack: the slot to, at least by past the slot it leaves.
type packArc struct {
	to int
	by float64
}

// longest sets x, from where each slot starts, to the longest path of the arcs next (into counting the arcs entering
// each slot), in Kahn's way, each slot put onto its grid as it is reached.
func longest(next [][]packArc, into []int, x []float64, onto func(s int, at float64) float64) {
	var queue []int
	for s := range next {
		if into[s] == 0 {
			queue = append(queue, s)
		}
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		x[s] = onto(s, x[s])
		for _, a := range next[s] {
			x[a.to] = max(x[a.to], x[s]+a.by)
			if into[a.to]--; into[a.to] == 0 {
				queue = append(queue, a.to)
			}
		}
	}
}

// step is the grid pack puts node i on: a column for where an edge leaves a frame, at x ≡ 0, else 6.
func (gr *group) step(i int) float64 {
	if gr.nodes[i].kind == kindOut {
		return column
	}
	return 6
}

// room is the room above (or below) a rank for the frames starting (or ending) there: the most, and each group's.
type room struct {
	room float64
	of   map[int]float64
}

// roomBare is the room above a rank for a group starting there with no heading, and below one for each ending there.
const roomBare = 12.0

// roomUnder is what a heading keeps below it, above the rank.
const roomUnder = 3.0

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
		if g := o.clusters[c].g; top && headed(g) {
			a := headArea(g)
			own = a.y + a.h + roomUnder
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
	gr.lineUp(spans, above, below)
	return above, below
}

// lineUp gives every lane the most room of any above its first rank and below its last, the graph's, so that the
// lanes start and end as one.
func (gr *group) lineUp(spans map[int][2]int, above, below []room) {
	for _, c := range gr.cls {
		if s := spans[c]; gr.out.lane(c) {
			above[s[0]].of[c], below[s[1]].of[c] = above[s[0]].room, below[s[1]].room
		}
	}
}

// below is the room below rank r for the frames ending there.
func (gr *group) below(r int) float64 {
	if r < len(gr.botRoom) {
		return gr.botRoom[r]
	}
	return 0
}
