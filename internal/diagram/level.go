package diagram

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"

	"floe/internal/usererr"
)

// A level edge stands its two boxes in one rank and runs straight between their facing sides. The boxes level edges
// join make a row, one rank; a row's boxes that one group holds stand side by side as written (a block). A level edge
// between boxes of two groups lays out the lowest group holding both (its common group; the graph's, -1, a graph of
// lanes' from the start) as lanes are: its groups, boxes and icons stand as written (its points, of longer edges,
// still move), making no blocks, and the edge's ends under them and the groups holding those stand at their ends
// towards the other end (faces). Where these meet, the order written wins, and what stands between two boxes after
// all is told once laid out (drawLevels).

// levelMistake is the mistake of edge k, written level, at a group, or nil.
func (o *graphOut) levelMistake(k int, ends [2]gref) *usererr.Error {
	e := o.g.Edges[k]
	for i, end := range ends {
		if end.box < 0 {
			return edgeMistake(edgeAt(o.at, k)+"/"+[2]string{"from", "to"}[i], "diagram_graph_level_group", e,
				o.endName(end))
		}
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
		told = append(told, edgeMistake(edgeAt(o.at, k)+"/level", "diagram_graph_level_loop", e, side,
			strconv.Quote(o.boxes[first].box.Name)))
		return true
	}), told
}

// rows joins the boxes of the level edges into rows, each by its first box as written, finds the common groups of
// those between two groups, and where each block and group under a common group stands (faces).
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
	o.common = map[int]bool{}
	if len(o.g.Lanes) > 0 {
		o.common[-1] = true
	}
	for _, k := range levels {
		if a, b := o.ends[k][0].box, o.ends[k][1].box; o.boxes[a].cl != o.boxes[b].cl {
			o.common[o.commonOf(a, b)] = true
		}
	}
	o.facing, o.faced = map[[2]int]int{}, map[int]int{}
	face := func(was, towards int) int { // a start wins over an end
		if towards < 0 || was == 0 {
			return towards
		}
		return was
	}
	for _, k := range levels {
		first, second := o.sides(k)
		c := o.commonOf(first, second)
		for _, end := range [2][2]int{{first, 1}, {second, -1}} {
			b, towards := end[0], end[1]
			d := o.boxes[b].cl
			if d == c { // stands as written, or in a block
				continue
			}
			o.facing[o.block(b)] = face(o.facing[o.block(b)], towards)
			for ; o.clusters[d].parent != c; d = o.clusters[d].parent {
				o.faced[d] = face(o.faced[d], towards)
			}
		}
	}
}

// commonOf is the lowest group holding boxes a and b, -1 for the graph.
func (o *graphOut) commonOf(a, b int) int {
	for c := o.boxes[a].cl; c >= 0; c = o.clusters[c].parent {
		if o.holds(c, gref{box: b, cl: -1}) {
			return c
		}
	}
	return -1
}

// block is box b's block: its row and its group; a box of a common group is a block of its own.
func (o *graphOut) block(b int) [2]int {
	if cl := o.boxes[b].cl; o.common[cl] {
		return [2]int{b, cl}
	}
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
// between ranks cannot run in one. An edge to a group from a box of a row whose other box the group holds stands among
// the group's ranks, told by rankWith.
func (o *graphOut) across(edges []int) ([]int, usererr.List) {
	var told usererr.List
	return slices.DeleteFunc(slices.Clone(edges), func(k int) bool {
		a, b := o.ends[k][0].box, o.ends[k][1].box
		if o.ranked[a] != o.ranked[b] {
			return false
		}
		e := o.g.Edges[k]
		if o.row[a] != o.row[b] {
			told = append(told, edgeMistake(edgeAt(o.at, k), "diagram_graph_with_rank", e))
			return true
		}
		told = append(told, edgeMistake(edgeAt(o.at, k), "diagram_graph_level_rank", e,
			strconv.Quote(o.boxes[a].box.Name), strconv.Quote(o.boxes[b].box.Name)))
		return true
	}), told
}

// inBlock reports whether boxes a and b are of one block.
func (o *graphOut) inBlock(a, b int) bool {
	return o.block(a) == o.block(b)
}

// faces is where box b's block stands in its group: -1 at the start when a box of it has a level edge to a box
// standing before its common group's node holding b, else 1 at its end when to one after, else 0 (where the order puts
// it).
func (o *graphOut) faces(b int) int {
	return o.facing[o.block(b)]
}

// sides are the boxes of level edge k, the one standing first (left, laid out down) and the other: as written, which
// is the order of the nodes holding them in their common group, as it stands them.
func (o *graphOut) sides(k int) (first, second int) {
	a, b := o.ends[k][0].box, o.ends[k][1].box
	return min(a, b), max(a, b)
}

// kindName is how a mistake names the box or icon m by what it is: `box "x"` or `icon "x"`.
func (o *graphOut) kindName(m int) string {
	b := o.boxes[m].box
	if b.icon != nil {
		return "icon " + strconv.Quote(b.Name)
	}
	return "box " + strconv.Quote(b.Name)
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

// facing is where unit u stands in its group (see faces): a block's, a group's under a common group, or 0.
func (gr *group) facing(u unit) int {
	if u.cl >= 0 {
		return gr.out.faced[u.cl]
	}
	if n := gr.nodes[u.nodes[0]]; n.box != nil {
		return gr.out.faces(n.of)
	}
	return 0
}

// written reports whether unit u, of group c, stands as written: a group, box or icon of a common group c.
func (gr *group) written(u unit, c int) bool {
	return gr.out.common[c] && (u.cl >= 0 || gr.nodes[u.nodes[0]].box != nil)
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
// tells, and leaves out of the group's, the level edges something stands between in their rank as laid out: a box or
// icon, or the frame of a group holding neither box, but for a group right in their common group, which they cross.
// LIMIT: two boxes standing the other way round fail as floe's own: the nodes of their common group holding them
// stand as written, and no graph tried turns them. Tell it if one comes to.
func (o *graphOut) drawLevels(gr *group, d *drawing) (usererr.List, error) {
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
	var told usererr.List
	pairs, byPair := gr.levelPairs()
	for _, p := range pairs {
		if at[p[0]] > at[p[1]] {
			//nolint:err113 // floe's own
			return nil, fmt.Errorf("the boxes %v of level edges stand the other way round", p)
		}
		if what := gr.between(p, at[p[0]], at[p[1]]); what != "" {
			for _, k := range byPair[p] {
				told = append(told, edgeMistake(edgeAt(o.at, k)+"/level", "diagram_graph_level_between", o.g.Edges[k], what))
			}
			gr.levels = slices.DeleteFunc(gr.levels, func(k int) bool { return slices.Contains(byPair[p], k) })
			continue
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
	return told, nil
}

// between is what stands between boxes p, at places from and to of their rank (as checkEdges names it), or "".
func (gr *group) between(p [2]int, from, to int) string {
	o := gr.out
	c := o.commonOf(p[0], p[1])
	for _, i := range gr.ranks[gr.nodeOf(p[0]).rank][from+1 : to] {
		n := gr.nodes[i]
		switch {
		case n.box != nil:
			return o.kindName(n.of)
		case n.kind == kindLeft && o.clusters[n.cl].parent != c &&
			!o.holds(n.cl, gref{box: p[0], cl: -1}) && !o.holds(n.cl, gref{box: p[1], cl: -1}):
			return "group " + strconv.Quote(o.clusters[n.cl].g.Name)
		}
	}
	return ""
}

// nodeOf is the node of box b.
func (gr *group) nodeOf(b int) gnode {
	return gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.of == b })]
}

// headed tells, and takes out with its label, each level edge running through a group's or lane's heading: laid out
// right, a heading longer than the room above the group's first rank reaches into it. It is told on the drawing as it
// ends, labels placed: room made for a label moves the drawing apart.
func (d *drawing) headed(o *graphOut, gr *group) usererr.List {
	var told usererr.List
	for _, k := range gr.levels {
		pts, ok := d.routes[k]
		if !ok {
			continue
		}
		for _, c := range slices.Sorted(maps.Keys(d.heads)) {
			f := d.frames[c]
			//nolint:exhaustruct_v5 // as addFrame's
			if head := headRect(f, d.heads[c]); crossedBy(segAt(pts, 0), head) {
				e := o.g.Edges[k]
				told = append(told, edgeMistake(edgeAt(o.at, k)+"/level", "diagram_graph_level_heading", e, o.clusters[c].g.Name))
				d.forget(k)
				break
			}
		}
	}
	return told
}

// forget takes level edge k out of all that is read once the ends have settled: the turns at its icons' sides (bends,
// keyed by its ends' boxes, so before ends goes) and the sides of boxes level edges take, found again from those left.
// LIMIT: what was placed by k before stays in the graph told and refused, as inspect shows it: labels kept off the
// exits left to edges written by hand on its sides (hands, read before), and the other ends on its sides at their
// shares of two (a third and two thirds), not the middle. Place them again if a refused graph's inspect comes to
// mislead.
func (d *drawing) forget(k int) {
	for j := range 2 {
		end := endAt{k, j}
		if place, on := d.places[end]; on {
			side := endSide{d.ends[k][j], place.side}
			d.bends[side] = slices.DeleteFunc(d.bends[side], func(other endAt) bool { return other == end })
		}
		delete(d.places, end)
	}
	delete(d.routes, k)
	delete(d.spots, k)
	delete(d.level, k)
	delete(d.ends, k)
	d.taken = map[int]map[string]bool{}
	for left, sides := range d.level {
		d.take(d.ends[left][0], sides[0])
		d.take(d.ends[left][1], sides[1])
	}
}

// take marks side of box i as taken by a level edge.
func (d *drawing) take(i int, side string) {
	if d.taken[i] == nil {
		d.taken[i] = map[string]bool{}
	}
	d.taken[i][side] = true
}
