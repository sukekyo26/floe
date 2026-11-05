package diagram

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/sukekyo26/floe/internal/usererr"
)

// A level edge stands its two boxes in one rank and runs straight between their facing sides. The boxes level edges
// join make a row, one rank; a row's boxes that one group holds stand side by side as written (a block), and a box
// right in a lane with a level edge to another lane stands at its lane's end towards it. So what stands between two
// boxes of a level edge follows from the order written, the lanes' order and the ranks, not from how ranks are
// ordered, and is told before the graph is laid out (levelMistakes).

// levelMistake is the mistake of edge k, written level, between ends: at a group, or between two boxes neither of
// one group nor right in two lanes; or nil.
func (o *graphOut) levelMistake(k int, ends [2]gref) *usererr.Error {
	e := o.g.Edges[k]
	for i, end := range ends {
		if end.box < 0 {
			return usererr.NewIn("", edgeAt(o.at, k)+"/"+[2]string{"from", "to"}[i], "diagram_graph_level_group", e.From,
				e.To, o.endName(end))
		}
	}
	// LIMIT: two boxes of one group, or right in two lanes, whose left and right the order written or the lanes'
	// order tells; a group's place among its siblings is the order's. Lift it if groups come to stand as written.
	a, b := o.boxes[ends[0].box].cl, o.boxes[ends[1].box].cl
	if a != b && (!o.lane(a) || !o.lane(b)) {
		return usererr.NewIn("", edgeAt(o.at, k)+"/level", "diagram_graph_level_apart", e.From, e.To)
	}
	return nil
}

// unlooped is levels but those out of the side their first box's loops take, told: a box's loops take its far side
// (right, laid out down), and a level edge to a box after it would too. It is told before rows are made, so that an
// edge refused makes no row.
func (o *graphOut) unlooped(levels []int) ([]int, usererr.List) {
	looped := map[int]bool{}
	for _, l := range o.loops {
		looped[l[1]] = true
	}
	side := "right"
	if o.g.Direction == "right" {
		side = "bottom"
	}
	var told usererr.List
	return slices.DeleteFunc(levels, func(k int) bool {
		first, _ := o.sides(k)
		if !looped[first] {
			return false
		}
		e := o.g.Edges[k]
		told = append(told, usererr.NewIn("", edgeAt(o.at, k)+"/level", "diagram_graph_level_loop", e.From, e.To,
			side, o.boxes[first].box.Name))
		return true
	}), told
}

// rows joins the boxes of the level edges into rows, each by its first box as written, and finds where each block
// stands in its lane (faces).
func (o *graphOut) rows(levels []int) {
	o.levels = levels
	o.row = make([]int, len(o.boxes))
	for i := range o.row {
		o.row[i] = i
	}
	for _, k := range levels {
		unite(o.row, o.ends[k][0].box, o.ends[k][1].box)
	}
	rootAll(o.row)
	o.facing = map[[2]int]int{}
	for _, k := range levels {
		for i, j := range [2]int{o.ends[k][0].box, o.ends[k][1].box} {
			key := o.block(j)
			switch d := o.laneOf(o.ends[k][1-i].box) - o.laneOf(j); {
			case d < 0:
				o.facing[key] = -1
			case d > 0 && o.facing[key] == 0:
				o.facing[key] = 1
			}
		}
	}
}

// block is box b's block: its row and its group.
func (o *graphOut) block(b int) [2]int {
	return [2]int{o.row[b], o.boxes[b].cl}
}

// withs joins each box and the box its with names into one rank, over the rows (ranked), kept apart from them as a row
// stands side by side and a with does not; and tells a with naming no box or icon of the graph, which does nothing.
// LIMIT: the words of diagram_graph_among and diagram_graph_level_between do not name a with that stands a box where
// they tell of it; the next mistake, or dropping the with, tells. Name it if one misleads.
func (o *graphOut) withs(names map[string]gref) usererr.List {
	o.ranked, o.withed = slices.Clone(o.row), map[int]bool{}
	var told usererr.List
	for i, b := range o.boxes {
		if b.box.With == "" {
			continue
		}
		e, ok := names[b.box.With]
		if !ok || e.box < 0 {
			told = append(told, usererr.NewIn("", b.at+"/with", "diagram_graph_with_unknown", b.box.With))
			continue
		}
		unite(o.ranked, i, e.box)
		o.withed[i] = true
	}
	rootAll(o.ranked)
	return told
}

// rootOf is the root of i among the sets parent holds.
func rootOf(parent []int, i int) int {
	for parent[i] != i {
		i = parent[i]
	}
	return i
}

// unite joins the sets of x and y, rooted at the smallest index, so that a set's root is its first box as written.
func unite(parent []int, x, y int) {
	x, y = rootOf(parent, x), rootOf(parent, y)
	parent[max(x, y)] = min(x, y)
}

// rootAll points each index of parent at its set's root.
func rootAll(parent []int) {
	for i := range parent {
		parent[i] = rootOf(parent, i)
	}
}

// across is edges, edges between boxes, but those between two boxes of one rank by a row or a with, told: an edge
// between ranks cannot run in one. An edge to a group never joins one row: a level edge's two boxes are of one group,
// which a group holding one holds both of (its other end inside, told before), or right in two lanes, of which a group
// holding one is that lane.
func (o *graphOut) across(edges []int) ([]int, usererr.List) {
	var told usererr.List
	return slices.DeleteFunc(slices.Clone(edges), func(k int) bool {
		a, b := o.ends[k][0].box, o.ends[k][1].box
		if o.ranked[a] != o.ranked[b] {
			return false
		}
		e := o.g.Edges[k]
		if o.row[a] != o.row[b] {
			told = append(told, usererr.NewIn("", edgeAt(o.at, k), "diagram_graph_with_rank", e.From, e.To))
			return true
		}
		told = append(told, usererr.NewIn("", edgeAt(o.at, k), "diagram_graph_level_rank", e.From, e.To,
			o.boxes[a].box.Name, o.boxes[b].box.Name))
		return true
	}), told
}

// laneOf is the place among the lanes of the lane box b stands in, or -1.
func (o *graphOut) laneOf(b int) int {
	c := o.boxes[b].cl
	for c >= 0 && o.clusters[c].parent >= 0 {
		c = o.clusters[c].parent
	}
	if !o.lane(c) {
		return -1
	}
	return slices.IndexFunc(o.top, func(t gref) bool { return t.cl == c })
}

// inBlock reports whether boxes a and b are of one block: one row, one group.
func (o *graphOut) inBlock(a, b int) bool {
	return o.row[a] == o.row[b] && o.boxes[a].cl == o.boxes[b].cl
}

// faces is where box b's block stands in its lane: -1 at the lane's start when a box of it has a level edge to a
// lane before, else 1 at its end when to a lane after, else 0 (where the order puts it).
func (o *graphOut) faces(b int) int {
	return o.facing[o.block(b)]
}

// sides are the boxes of level edge k, the one standing first (left, laid out down) and the other: in one group,
// as written; in two lanes, by the lanes' order.
func (o *graphOut) sides(k int) (first, second int) {
	a, b := o.ends[k][0].box, o.ends[k][1].box
	if la, lb := o.laneOf(a), o.laneOf(b); la != lb && la > lb || la == lb && a > b {
		return b, a
	}
	return a, b
}

// levelMistakes are the mistakes of the group's level edges its ranks, the order written and the lanes' order tell,
// before it is laid out: something standing between an edge's boxes in their rank. A level edge told is dropped
// from the group's; the row it made stays, ranked as one.
func (gr *group) levelMistakes() usererr.List {
	o := gr.out
	rankOf := map[int]int{}
	for _, n := range gr.nodes {
		if n.box != nil {
			rankOf[n.of] = n.rank
		}
	}
	var told usererr.List
	gr.levels = slices.DeleteFunc(gr.levels, func(k int) bool {
		e := o.g.Edges[k]
		at := edgeAt(o.at, k) + "/level"
		first, second := o.sides(k)
		if what := gr.between(first, second, rankOf); what != "" {
			told = append(told, usererr.NewIn("", at, "diagram_graph_level_between", e.From, e.To, what))
			return true
		}
		return false
	})
	return told
}

// between is what stands between first and second, the boxes of a level edge, in their rank (as checkEdges names
// it), or "": in one group (one lane too), a box of their block written between them; in two lanes, what
// laneBetween finds.
func (gr *group) between(first, second int, rankOf map[int]int) string {
	o := gr.out
	if o.laneOf(first) != o.laneOf(second) {
		return gr.laneBetween(first, second, rankOf)
	}
	for m := first + 1; m < second; m++ {
		if o.inBlock(m, first) {
			return `box "` + o.boxes[m].box.Name + `"`
		}
	}
	return ""
}

// laneBetween is what stands between first and second, boxes right in two lanes, in their rank r: a box of first's
// block written after it or of second's before it, a block of either lane standing at the same end (the order would
// pick which stands outer), anything of first's lane when its block stands at the start, and in a lane between, a
// box or a group's frame spanning r.
func (gr *group) laneBetween(first, second int, rankOf map[int]int) string {
	o := gr.out
	r := rankOf[first]
	for _, m := range slices.Sorted(maps.Keys(rankOf)) {
		if rankOf[m] == r && m != first && m != second && o.inWay(m, first, second) {
			return `box "` + o.boxes[m].box.Name + `"`
		}
	}
	return gr.laneFrame(first, second, rankOf)
}

// inWay reports whether box m, in the rank of first and second, boxes right in two lanes, stands between them (see
// laneBetween).
func (o *graphOut) inWay(m, first, second int) bool {
	in, direct := o.laneOf(m), o.lane(o.boxes[m].cl)
	switch in {
	case o.laneOf(first):
		if o.inBlock(m, first) {
			return m > first
		}
		return o.faces(first) < 0 || direct && o.faces(m) > 0
	case o.laneOf(second):
		if o.inBlock(m, second) {
			return m < second
		}
		return direct && o.faces(m) < 0
	}
	return o.laneOf(first) < in && in < o.laneOf(second)
}

// laneFrame is the group of a lane (none) whose frame spans the rank of first and second, boxes right in two lanes,
// and stands between them: in a lane between, or in first's when its block stands at the start.
func (gr *group) laneFrame(first, second int, rankOf map[int]int) string {
	o := gr.out
	r, lf, ls := rankOf[first], o.laneOf(first), o.laneOf(second)
	for c, cl := range o.clusters {
		lo, hi := math.MaxInt, -1
		for _, m := range cl.boxes {
			if rm, ok := rankOf[m]; ok {
				lo, hi = min(lo, rm), max(hi, rm)
			}
		}
		in := o.laneOf(cl.boxes[0])
		if !o.lane(c) && lo <= r && r <= hi && (lf < in && in < ls || in == lf && o.faces(first) < 0) {
			return `group "` + cl.g.Name + `"`
		}
	}
	return ""
}

// rowUp is rank with the boxes of each block side by side, as written, where its first stands.
func (gr *group) rowUp(rank []int) []int {
	blocks := map[[2]int][]int{} // the boxes of a rank are listed as written
	for _, i := range rank {
		if n := gr.nodes[i]; n.box != nil {
			blocks[gr.out.block(n.of)] = append(blocks[gr.out.block(n.of)], i)
		}
	}
	out := make([]int, 0, len(rank))
	for _, i := range rank {
		n := gr.nodes[i]
		if n.box == nil {
			out = append(out, i)
			continue
		}
		if b := blocks[gr.out.block(n.of)]; b[0] == i {
			out = append(out, b...)
		}
	}
	return out
}

// sameBlock reports whether nodes i and j are boxes of one block.
func (gr *group) sameBlock(i, j int) bool {
	a, b := gr.nodes[i], gr.nodes[j]
	return len(gr.levels) > 0 && a.box != nil && b.box != nil && gr.out.inBlock(a.of, b.of)
}

// facing is where unit u, of a lane, stands in it (see faces): a block's, or 0.
func (gr *group) facing(u unit) int {
	if n := gr.nodes[u.nodes[0]]; u.cl < 0 && n.box != nil {
		return gr.out.faces(n.of)
	}
	return 0
}

// levelPairs are the group's level edges by the two boxes they join, in the order of their first edge.
func (gr *group) levelPairs() ([][2]int, map[[2]int][]int) {
	byPair := map[[2]int][]int{}
	var pairs [][2]int
	for _, k := range gr.levels {
		first, second := gr.out.sides(k)
		p := [2]int{first, second}
		if byPair[p] == nil {
			pairs = append(pairs, p)
		}
		byPair[p] = append(byPair[p], k)
	}
	return pairs, byPair
}

// short is the one of boxes p whose side across the ranks (its height laid out down, its width laid out right) is
// the shorter.
func (d *drawing) short(p [2]int) rect {
	_, along := d.axes()
	a, b := d.boxes[p[0]], d.boxes[p[1]]
	if b.size(along) < a.size(along) {
		return b
	}
	return a
}

// growLevels lengthens the rank of each pair of boxes whose shorter side across the ranks is short of its level
// edges (see drawLevels), cut across its middle as for loops. The outermost stands half a column short of a column per
// two edges from the middle, and off the box's round corner.
// LIMIT: the corner of a box and a table; a pill's or a state's end is rounder, so an edge may enter its curve there.
// Make it the box's own if one comes to look wrong.
func (d *drawing) growLevels(gr *group) {
	_, along := d.axes()
	pairs, byPair := gr.levelPairs()
	for _, p := range pairs {
		need := 2 * (column*math.Ceil(float64(len(byPair[p]))/2) - column/2 + cornerR)
		if s := d.short(p); s.size(along) < need {
			d.stretch(along, s.lo(along)+s.size(along)/2, onGrid(need-s.size(along), column, math.Ceil))
		}
	}
}

// drawLevels draws the group's level edges, each straight from the far side of its first box to the near side of
// the other, a column apart about the middle of the shorter side across the ranks, but off it by half a column:
// where a point between them turns to its column. So an odd number of them stands one more below the middle. It
// fails when something stands between two boxes after all: levelMistakes tells what does.
func (o *graphOut) drawLevels(gr *group, d *drawing) error {
	_, along := d.axes()
	far, near := "right", "left"
	if d.right {
		far, near = "bottom", "top"
	}
	at := map[int]int{} // each box's place in its rank
	for _, rank := range gr.ranks {
		for j, i := range rank {
			if gr.nodes[i].box != nil {
				at[gr.nodes[i].of] = j
			}
		}
	}
	pairs, byPair := gr.levelPairs()
	for _, p := range pairs {
		if at[p[0]] > at[p[1]] {
			//nolint:err113 // floe's own
			return fmt.Errorf("the boxes %v of level edges stand the other way round", p)
		}
		r := gr.ranks[gr.nodeOf(p[0]).rank]
		for _, i := range r[at[p[0]]+1 : at[p[1]]] {
			if gr.nodes[i].box != nil {
				//nolint:err113 // floe's own
				return fmt.Errorf("box %d stands between the boxes %v of level edges", gr.nodes[i].of, p)
			}
		}
		s := d.short(p)
		mid := s.lo(along) + s.size(along)/2
		ks := byPair[p]
		for j, k := range ks {
			t := mid + column*(float64(j)-math.Floor(float64(len(ks))/2)) + column/2
			a, b := d.boxes[p[0]], d.boxes[p[1]]
			pts := [][2]float64{a.form.on(a, far, t), b.form.on(b, near, t)}
			d.ends[k], d.level[k] = p, [2]string{far, near}
			if o.ends[k][0].box != p[0] { // drawn from the box it is written from
				slices.Reverse(pts)
				d.ends[k], d.level[k] = [2]int{p[1], p[0]}, [2]string{near, far}
			}
			d.routes[k] = pts
			d.take(p[0], far)
			d.take(p[1], near)
		}
	}
	return nil
}

// nodeOf is the node of box b.
func (gr *group) nodeOf(b int) gnode {
	return gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.of == b })]
}

// headed tells, and takes out with its label, each level edge running through a lane's heading: laid out right, a
// heading longer than the room above the first rank reaches into it. It is told on the drawing as it ends, labels
// placed: room made for a label moves the drawing apart. A group's heading never stands between (levelMistakes).
func (d *drawing) headed(o *graphOut, gr *group) usererr.List {
	var told usererr.List
	for _, k := range gr.levels {
		pts, ok := d.routes[k]
		if !ok {
			continue
		}
		for _, c := range slices.Sorted(maps.Keys(d.heads)) {
			if !o.lane(c) {
				continue
			}
			f := d.frames[c]
			//nolint:exhaustruct_v5 // as addFrame's
			if head := headRect(f, d.heads[c]); crossedBy(segAt(pts, 0), head) {
				e := o.g.Edges[k]
				told = append(told, usererr.NewIn("", edgeAt(o.at, k)+"/level", "diagram_graph_level_heading", e.From,
					e.To, o.clusters[c].g.Name))
				delete(d.routes, k)
				delete(d.spots, k)
				delete(d.level, k)
				delete(d.ends, k)
				break
			}
		}
	}
	return told
}

// take marks side of box i as taken by a level edge.
func (d *drawing) take(i int, side string) {
	if d.taken[i] == nil {
		d.taken[i] = map[string]bool{}
	}
	d.taken[i][side] = true
}
