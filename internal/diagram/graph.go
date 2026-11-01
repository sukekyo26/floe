package diagram

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"

	"github.com/sukekyo26/floe/internal/usererr"
)

// maxPassed is how many ranks the edges of all the graphs of a diagram may pass between their ends, all told: the
// points they pass ranks at are laid out, routed and checked like boxes, and parts have no limit of their number.
const maxPassed = 4000

// gnode is a box of a graph, or a point where an edge passes a rank, or (kind) a mark of a group's.
type gnode struct {
	box     *graphBox // nil for a point an edge passes, and a mark
	of      int       // the box's index among the boxes read; for a point or a port, its edge's index in edges
	kind    nodeKind
	cl      int // the group it stands in (-1 for none): a box's innermost, a point's edges' ends' innermost shared
	rank    int
	w, h    float64 // across the ranks and along them, as laid out down
	x, y    float64 // its left and top, as laid out down
	ins     []int   // the links into it from the rank above
	outs    []int   // the links out of it to the rank below
	written int     // the box's index among the nodes of its group, the order ties fall back on; points after
}

// glink is the run of an edge from a node of one rank to a node of the next, as laid out down.
type glink struct {
	from, to    int
	edge        int // its edge's index in edges
	exit, entry float64
	straight    bool // entering right below its exit
	track       int
}

// group is a connected part of a graph, laid out on its own: its boxes, the edges between them, and the points
// they pass ranks at.
type group struct {
	nodes    []gnode
	links    []glink
	ranks    [][]int
	chains   map[int][]int // each edge's links, by its index in edges, top to bottom
	reversed map[int]bool  // the edges turned round to break a cycle, drawn back the way they are written
	tracks   []int         // the tracks of the channel below each rank
	top      []float64     // the top of each rank, as laid out down
	bottom   []float64     // the bottom of each rank: of its tallest box, or a column below its top
	arcs     []arc         // its edges between its boxes, turned round where they break a cycle
	out      *graphOut
	among    usererr.List       // edges to groups whose other end stands among the group's ranks, told
	cls      []int              // the groups of boxes within it, as written
	frames   map[int][2]float64 // each group's frame's top and bottom, as laid out down
	botRoom  []float64          // the room below each rank for the frames ending there
	headAt   float64            // how far a heading's room stands from its frame's top, along the ranks
	levels   []int              // its level edges, by their indexes in edges, but those told
}

// graphOut is a graph read and ranked: its boxes and groups read flat, its groups of connected boxes, and the ranks
// its edges pass between their ends.
type graphOut struct {
	g        *graph
	at       string
	boxes    []gbox
	clusters []cluster
	top      []gref // the nodes at the graph's top, as written
	ends     map[int][2]gref
	groups   []*group
	passed   int
	loops    [][2]int       // the edges from a box to itself, and the box
	skip     bool           // too many nodes: told, and not laid out
	levels   []int          // the edges written level between boxes, but those told (levelMistake)
	row      []int          // the first box, as written, of each box's row: the boxes level edges stand in one rank
	facing   map[[2]int]int // where each block stands in its lane (faces)
}

// readGraph reads g's boxes and groups, resolves the names of its edges against them, and splits it into groups of
// connected boxes, each ranked. An edge naming nothing of it, or between a group and what the group holds, is dropped
// and told; one from a box to itself is a loop, kept out of the groups (drawLoops).
func readGraph(g *graph, at string) (*graphOut, usererr.List) {
	out := &graphOut{
		g: g, at: at, boxes: nil, clusters: nil, top: nil, ends: map[int][2]gref{}, groups: nil, passed: 0,
		loops: nil, skip: false, levels: nil, row: nil, facing: nil,
	}
	out.boxes, out.clusters, out.top = flatten(g, at)
	if n := len(out.boxes) + len(out.clusters); n > maxNodes {
		out.skip = true
		return out, usererr.List{usererr.NewIn("", at, "diagram_graph_many", n, maxNodes)}
	}
	byName := out.names()
	var problems usererr.List
	var edges, cedges, levels []int
	for k := range g.Edges {
		ends, mistakes := out.endsOf(k, byName)
		if len(mistakes) > 0 {
			problems = append(problems, mistakes...)
			continue
		}
		out.ends[k] = ends
		switch {
		case ends[0].box >= 0 && ends[0] == ends[1]:
			out.loops = append(out.loops, [2]int{k, ends[0].box})
		case g.Edges[k].Level:
			if mistake := out.levelMistake(k, ends); mistake != nil {
				problems = append(problems, mistake)
				continue
			}
			levels = append(levels, k)
		case ends[0].box >= 0 && ends[1].box >= 0:
			edges = append(edges, k)
		default:
			cedges = append(cedges, k)
		}
	}
	levels, told := out.unlooped(levels)
	problems = append(problems, told...)
	out.rows(levels)
	edges, told = out.across(edges)
	problems = append(problems, told...)
	for _, gr := range out.groupsOf(edges, cedges, levels) {
		problems = append(problems, gr.among...)
		problems = append(problems, gr.levelMistakes()...)
		out.groups = append(out.groups, gr)
		out.passed += gr.passed()
	}
	return out, problems
}

// names are the ends an edge may name, as written, the first of a name used twice (told by layout).
func (o *graphOut) names() map[string]gref {
	byName := map[string]gref{}
	var name func(c gref)
	name = func(c gref) {
		n := ""
		if c.box >= 0 {
			n = o.boxes[c.box].box.Name
		} else {
			n = o.clusters[c.cl].g.Name
		}
		if _, ok := byName[n]; !ok && n != "" {
			byName[n] = c
		}
		if c.cl >= 0 {
			for _, k := range o.clusters[c.cl].kids {
				name(k)
			}
		}
	}
	for _, c := range o.top {
		name(c)
	}
	return byName
}

// endsOf are the ends of edge k by name, or its mistakes: a name of nothing of the graph, between a group and what
// it holds, or at a lane.
func (o *graphOut) endsOf(k int, byName map[string]gref) ([2]gref, usererr.List) {
	e := o.g.Edges[k]
	var ends [2]gref
	var mistakes usererr.List
	for i, end := range [2][2]string{{"from", e.From}, {"to", e.To}} {
		var ok bool
		if ends[i], ok = byName[end[1]]; !ok {
			mistakes = append(mistakes, usererr.NewIn("", edgeAt(o.at, k)+"/"+end[0], "diagram_graph_unknown", end[1]))
		}
	}
	if len(mistakes) > 0 {
		return ends, mistakes
	}
	if mistake := o.inside(k, ends); mistake != nil {
		return ends, usererr.List{mistake}
	}
	for i, end := range ends {
		if o.lane(end.cl) {
			at := edgeAt(o.at, k) + "/" + [2]string{"from", "to"}[i]
			return ends, usererr.List{usererr.NewIn("", at, "diagram_graph_lane", e.From, e.To, o.endName(end))}
		}
	}
	return ends, nil
}

// members are the boxes of end e: the box, or every box of the group.
func (o *graphOut) members(e gref) []int {
	if e.box >= 0 {
		return []int{e.box}
	}
	return o.clusters[e.cl].boxes
}

// groupsOf splits the boxes into the groups the edges (by index) join, boxes edges by groups edges (cedges) too, and
// boxes of one group at the top too, in the order of their first box, each ranked.
func (o *graphOut) groupsOf(edges, cedges, levels []int) []*group {
	find := o.joined(slices.Concat(edges, levels), cedges)
	members := map[int][]int{}
	var firsts []int
	for i := range o.boxes {
		r := find(i)
		if members[r] == nil {
			firsts = append(firsts, r)
		}
		members[r] = append(members[r], i)
	}
	groups := make([]*group, 0, len(firsts))
	for _, r := range firsts {
		var own, cown, lown []int
		for _, k := range edges {
			if find(o.ends[k][0].box) == r {
				own = append(own, k)
			}
		}
		for _, k := range levels {
			if find(o.ends[k][0].box) == r {
				lown = append(lown, k)
			}
		}
		for _, k := range cedges {
			if find(o.members(o.ends[k][0])[0]) == r {
				cown = append(cown, k)
			}
		}
		groups = append(groups, o.rank(members[r], own, cown, lown))
	}
	return groups
}

// joined is the root of each box's group of connected boxes: joined by edges, by edges to groups (all the boxes of
// both ends), by a group at the top (all its boxes) and by lanes (all the boxes).
func (o *graphOut) joined(edges, cedges []int) func(int) int {
	root := make([]int, len(o.boxes))
	for i := range root {
		root[i] = i
	}
	find := func(i int) int {
		for root[i] != i {
			i = root[i]
		}
		return i
	}
	join := func(boxes []int) {
		for _, b := range boxes[min(1, len(boxes)):] {
			x, y := find(boxes[0]), find(b)
			root[max(x, y)] = min(x, y)
		}
	}
	for _, k := range slices.Concat(edges, cedges) {
		join(slices.Concat(o.members(o.ends[k][0]), o.members(o.ends[k][1])))
	}
	for _, c := range o.clusters {
		if c.parent < 0 {
			join(c.boxes)
		}
	}
	if o.g.Lanes { // every box is in a lane
		for b := range o.boxes {
			join([]int{0, b})
		}
	}
	return find
}

func edgeAt(at string, k int) string {
	return at + "/edges/" + strconv.Itoa(k)
}

// items lays out each group and places its labels, side by side in the order of their first box, and opens the graph
// into a box for each of its boxes and an edge for each of its edges, at their JSON pointers, so that they are named,
// drawn and checked as boxes and edges written by hand are. The groups are laid out from 0,0 and moved to the graph's
// place last: room made and spots tried step by units, which a coordinate as far out as 1e17 would round away.
func (o *graphOut) items() ([]item, usererr.List, error) {
	right := o.g.Direction == "right"
	across := 0 // the axis groups stand side by side on, as drawn
	if right {
		across = 1
	}
	boxes := make([]rect, len(o.boxes))
	frames := map[int]rect{}
	routes := map[int][][2]float64{}
	labels := map[int]rect{}
	toward := map[int]bool{}
	left := 0.0
	var told usererr.List
	for _, gr := range o.groups {
		d, err := o.draw(gr, right)
		if err != nil {
			return nil, nil, err
		}
		told = append(told, d.headed(o, gr)...)
		left = d.stand(across, left)
		d.stretch(0, math.Inf(-1), o.g.X)
		d.stretch(1, math.Inf(-1), o.g.Y)
		maps.Copy(routes, d.routes)
		for k := range d.spots {
			labels[k], toward[k] = d.rect(k)
		}
		for i, r := range d.boxes {
			boxes[i] = r
		}
		maps.Copy(frames, d.frames)
	}
	out := o.opened(o.top, boxes, frames, make([]item, 0, len(o.boxes)+len(o.clusters)+len(o.g.Edges)))
	for k := range o.g.Edges {
		pts, ok := routes[k]
		if !ok { // told by readGraph
			continue
		}
		e := o.edge(k, pts)
		if r, ok := labels[k]; ok {
			e.Label, e.At, e.anchor = o.g.Edges[k].Label, &[2]float64{r.x + 5, r.y + labelH/2}, "start"
			if toward[k] {
				e.At[0], e.anchor = r.x+r.w-5, "end"
			}
		}
		out = append(out, item{edgeAt(o.at, k), part{Edge: e}}) //nolint:exhaustruct_v5 // one kind of part
	}
	return out, told, nil
}

// opened is out with the nodes cs opened as written, a group's frame before the nodes it holds, so that a name used
// twice is told at the later: each box at its place among boxes, each frame at its group's.
func (o *graphOut) opened(cs []gref, boxes []rect, frames map[int]rect, out []item) []item {
	for _, c := range cs {
		if c.box >= 0 {
			r, n := boxes[c.box], o.boxes[c.box]
			b := &box{
				Name: n.box.Name, X: r.x, Y: r.y, Origin: "", W: r.w, H: r.h, Rows: n.box.Rows, Tone: n.box.Tone,
				Shape: n.box.Shape, graph: o.at,
			}
			out = append(out, item{n.at, part{Box: b}}) //nolint:exhaustruct_v5 // one kind of part
			continue
		}
		r, g := frames[c.cl], o.clusters[c.cl].g
		f := &frame{
			Name: g.Name, X: r.x, Y: r.y, W: r.w, H: r.h, Label: g.Label, Accent: g.Accent, Icon: "", Square: false,
			graph: o.at,
		}
		out = append(out, item{o.clusters[c.cl].at, part{Frame: f}}) //nolint:exhaustruct_v5 // one kind of part
		out = o.opened(o.clusters[c.cl].kids, boxes, frames, out)
	}
	return out
}

// draw lays group gr out and places its labels, as drawn from 0,0.
func (o *graphOut) draw(gr *group, right bool) (*drawing, error) {
	gr.place(right)
	gr.wire()
	d := &drawing{
		right: right, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{},
		ends: map[int][2]int{}, back: map[int]bool{}, spots: map[int]spot{}, loops: map[int]bool{},
		frames: map[int]rect{}, heads: map[int]float64{}, level: map[int][2]string{}, taken: map[int]map[string]bool{},
	}
	sides := map[int][2]float64{} // each group's left and right, as laid out down
	for _, n := range gr.nodes {
		switch {
		case n.box != nil:
			r := drawnOf(n, right)
			r.form = outlineOf(r.w, r.h, n.box.Shape)
			d.boxes[n.of], d.shapes[n.of] = r, n.box.Shape
		case n.kind == kindLeft:
			sides[n.cl] = [2]float64{n.x, sides[n.cl][1]}
		case n.kind == kindRight:
			sides[n.cl] = [2]float64{sides[n.cl][0], n.x}
		}
	}
	for _, c := range gr.cls {
		f := gnode{ //nolint:exhaustruct_v5 // an area, as laid out down
			x: sides[c][0], w: sides[c][1] - sides[c][0], y: gr.frames[c][0], h: gr.frames[c][1] - gr.frames[c][0],
		}
		d.frames[c] = drawnOf(f, right)
		if t := o.clusters[c].g.Label; t != nil {
			d.heads[c] = textWidth(t, sizeSub, false)
		}
	}
	box := func(n gnode) int { // the box at an end, or -1 at a frame
		if n.box == nil {
			return -1
		}
		return n.of
	}
	edges := slices.Sorted(maps.Keys(gr.chains))
	for _, k := range edges {
		d.routes[k] = gr.route(k, d.boxes, right)
		chain := gr.chains[k]
		d.ends[k] = [2]int{box(gr.nodes[gr.links[chain[0]].from]), box(gr.nodes[gr.links[chain[len(chain)-1]].to])}
		if gr.reversed[k] {
			d.ends[k], d.back[k] = [2]int{d.ends[k][1], d.ends[k][0]}, true
		}
	}
	d.growLevels(gr)
	if err := o.drawLoops(d); err != nil {
		return nil, fmt.Errorf("diagram: the graph at %s: %w", o.at, err)
	}
	if err := o.drawLevels(gr, d); err != nil {
		return nil, fmt.Errorf("diagram: the graph at %s: %w", o.at, err)
	}
	for _, k := range slices.Sorted(slices.Values(slices.Concat(edges, gr.levels))) {
		if t := o.g.Edges[k].Label; t != nil {
			if err := d.place(k, labelWidth(t, false)); err != nil {
				return nil, fmt.Errorf("diagram: the graph at %s: %w", o.at, err)
			}
		}
	}
	return d, nil
}

// stand moves the group a column past left on axis across, its boxes and frames on the grid as laid out (a label
// before them moves it on by whole columns), and the top of what it draws to 0; it returns where the next group goes.
func (d *drawing) stand(across int, left float64) float64 {
	lo, hi := d.span(across)
	laidLo, _ := d.boxSpan(across)
	for _, f := range d.frames {
		laidLo = min(laidLo, f.lo(across))
	}
	by := left
	if lo < laidLo {
		by += onGrid(laidLo-lo, column, math.Ceil)
	}
	d.stretch(across, math.Inf(-1), by)
	if top, _ := d.span(1 - across); top < 0 {
		d.stretch(1-across, math.Inf(-1), onGrid(-top, column, math.Ceil))
	}
	return onGrid(hi+by+groupGap, column, math.Ceil)
}

// drawing is a group laid out, as drawn, while its labels are placed: its boxes and its edges' corners, and where
// its labels stand beside their runs.
type drawing struct {
	right  bool
	boxes  map[int]rect
	shapes map[int]string
	routes map[int][][2]float64
	ends   map[int][2]int // the boxes each edge's corners start and end on
	back   map[int]bool   // the edges drawn up, against the ranks
	spots  map[int]spot
	loops  map[int]bool            // the edges from a box to itself
	frames map[int]rect            // the frames of its groups, by group
	heads  map[int]float64         // the widths of its groups' headings
	level  map[int][2]string       // the level edges, and the sides of their boxes they take, from the box written first
	taken  map[int]map[string]bool // the sides of each box level edges take
}

// spot is where a label stands, held to its run so that it moves with it: beside run seg of edge's corners (its own
// edge's, but a loop's), a column off it on side (+1 right of a run down or below a run across, -1 left or above),
// from at along it.
type spot struct {
	edge int
	seg  int
	side float64
	at   float64
	w    float64
}

// rect is where edge k's label stands now, and whether its text is set toward its run, at its right end (a label
// left of a run down).
func (d *drawing) rect(k int) (rect, bool) {
	sp := d.spots[k]
	return spotRect(d.runOf(sp), sp), sp.side < 0 && isDown(d.runOf(sp))
}

// runOf is the run a label at sp stands beside.
func (d *drawing) runOf(sp spot) seg {
	return segAt(d.routes[sp.edge], sp.seg)
}

// spotRect is where a label at sp stands beside run s.
func spotRect(s seg, sp spot) rect {
	along, size := runAxis(s), [2]float64{sp.w, labelH} // across and along a run down
	if along == 0 {
		size = [2]float64{labelH, sp.w}
	}
	r := rect{what: "", at: ""} //nolint:exhaustruct_v5 // a spot
	r.set(along, sp.at, size[1])
	cross := s[1-along]
	if sp.side > 0 {
		r.set(1-along, cross+column, size[0])
	} else {
		r.set(1-along, cross-column-size[0], size[0])
	}
	return r
}

// runAxis is the axis run s runs on: 1 (y) for a run down, 0 (x) for one across.
func runAxis(s seg) int {
	if isDown(s) {
		return 1
	}
	return 0
}

func isDown(s seg) bool { return s[0] == s[2] }

// place places edge k's label, w wide: beside the first run of its edge with room for it, from the box it is written
// from, each run from its end nearer that box; with none, it makes room beside a run (see room), the first that
// takes it. It fails when none takes it, and when room made is not clear (of where edges written by hand leave
// boxes, too): floe's fault, as room made is clear by how it is made (see room), and a failed room would leave the
// drawing stretched.
// LIMIT: each spot is compared with every box, label and run of the group (0.06s for the largest graph allowed, every
// edge labelled); compare only the things near a spot if larger graphs come to need it. Room made grows the group
// and widens the boxes a cut crosses beyond their text, and minds no edge written by hand: a band made over the
// middle of a box's side may stand in the way of one, told at it.
// LIMIT: room made puts a label below its run (right of it laid out right), so the label of the upper of two level
// edges between the same boxes stands between them, pushing the other edge and the boxes' sides apart. Make room
// above a run too (as for a loop left of its box) if pairs of labelled level edges come to look spread.
func (d *drawing) place(k int, w float64) error {
	things, runs := d.held(-1), append(d.runs(), d.hands()...)
	for i := range len(d.routes[k]) - 1 {
		s := segAt(d.routes[k], i)
		long := w
		if isDown(s) {
			long = labelH
		}
		for _, a := range starts(s, long) {
			for _, side := range []float64{1, -1} {
				sp := spot{edge: k, seg: i, side: side, at: a, w: w}
				if free(heldRect(s, sp), things, runs) {
					d.spots[k] = sp
					return nil
				}
			}
		}
	}
	for i := range len(d.routes[k]) - 1 {
		sp, ok := d.room(k, i, w)
		if !ok {
			continue
		}
		if !free(heldRect(segAt(d.routes[k], i), sp), d.held(k), append(d.runs(), d.hands()...)) {
			return fmt.Errorf("room made for edge %d's label is not clear", k) //nolint:err113 // floe's own
		}
		d.spots[k] = sp
		return nil
	}
	return fmt.Errorf("found no room for edge %d's label", k) //nolint:err113 // floe's own
}

// starts are where a label long along run s may start along it, 2 in from its ends, by 6 from its end nearer the box.
func starts(s seg, long float64) []float64 {
	along := runAxis(s)
	from, to := s[along], s[along+2]
	var at []float64
	if from < to {
		for a := from + 2; a+long <= to-2; a += 6 {
			at = append(at, a)
		}
	} else {
		for a := from - 2 - long; a >= to+2; a -= 6 {
			at = append(at, a)
		}
	}
	return at
}

// room makes room for edge k's label beside run i of its edge, and returns where it stands: the drawing is cut
// across the run a unit past its end nearer the box (past every box the cut would cross), and all that lies past the
// cut moves on along the run by the label's length and what labels leave in the band (crossed); then it is cut
// along the run, short of the nearest thing right of (or below) it by the label (see reach), and all that lies past
// that cut moves on across as far as the label needs, the boxes the cut crosses widening. The room is clear: in the
// band lie only runs that cross it and what labels leave short of the room; across, nothing by the label stands
// between the run and the second cut, and all past it moves beyond the label. Neither cut bends a run or makes two
// things meet: a straight run stays straight, and a run across a cut grows longer. With the first cut past the run's
// other end, it makes none and reports false.
func (d *drawing) room(k, i int, w float64) (spot, bool) {
	s := segAt(d.routes[k], i)
	along := runAxis(s)
	size := [2]float64{w, labelH} // across and along a run down
	if along == 0 {
		size = [2]float64{labelH, w}
	}
	from, to, sign := s[along], s[along+2], 1.0
	if to < from {
		sign = -1
	}
	_, level := d.level[k]
	if level { // a level edge's run starts within its box's bounds, on a slant or a round end
		b := d.boxes[d.ends[k][0]]
		from = b.lo(along) + max(sign, 0)*b.size(along)
	}
	cut := from + sign
	for moved := true; moved; {
		moved = false
		for _, b := range d.boxes {
			if lo, hi := b.lo(along), b.lo(along)+b.size(along); lo <= cut && cut <= hi {
				cut, moved = lo-1, true
				if sign > 0 {
					cut = hi + 1
				}
			}
		}
	}
	if (cut-to)*sign >= 0 && level { // a level edge's only run: cut through the boxes in the way, widening them
		cut = from + sign
	}
	if (cut-to)*sign >= 0 {
		return spot{edge: k, seg: 0, side: 0, at: 0, w: 0}, false
	}
	cross := s[1-along]
	stays, moves := d.crossed(along, cut, cross+1, cross+column+size[0])
	by := stays + size[1] + 4 + moves
	if level { // across the ranks, by whole columns, keeping the grid of points and edges
		by = onGrid(by, column, math.Ceil)
	}
	d.stretch(along, cut, by)
	sp := spot{edge: k, seg: i, side: 1, at: cut + stays + 2, w: w}
	s = segAt(d.routes[k], i)
	if at, need := d.reach(spotRect(s, sp), s[1-along], along); need > 0 {
		d.stretch(1-along, at, onGrid(need, column, math.Ceil))
	}
	return sp, true
}

// crossed is how far the labels across from lo to hi (where the label to place stands with its column) reach into
// the band a cut at cut on axis along opens, by how stretch moves them: a label stays whole, if its run does, or
// moves whole with its run, so that what of it lay past the cut (it stays) or short of it (it moves) lies in the
// band, before (stays) or after (moves) the room made there. A label beside that span stays clear of the room.
func (d *drawing) crossed(along int, cut, lo, hi float64) (stays, moves float64) {
	for _, sp := range d.spots {
		s := d.runOf(sp)
		r := heldRect(s, sp)
		if a := r.lo(1 - along); a >= hi || a+r.size(1-along) <= lo {
			continue
		}
		lo, hi := r.lo(along), r.lo(along)+r.size(along)
		if runAxis(s) == along && sp.at > cut || runAxis(s) != along && s[along] > cut {
			moves = max(moves, cut-lo)
		} else {
			stays = max(stays, hi-cut)
		}
	}
	return stays, moves
}

// reach is where to cut along the run at cross, and how far on across, for r beside it: the nearest thing past the
// run within r's span along it (a run, a box, a label held to its run, where an edge written by hand leaves a box)
// stands short of r's far end by need, and a little; the cut is between the run and that thing, a quarter column
// past the run or halfway, the nearer, so that nothing within r's span stays between the run and the cut. need is 0
// when nothing stands in r's way.
func (d *drawing) reach(r rect, cross float64, along int) (at, need float64) {
	across := 1 - along
	lo, hi := r.lo(along), r.lo(along)+r.size(along)
	end := r.lo(across) + r.size(across) + 2
	near := math.Inf(1)
	for _, b := range d.held(-1) {
		if b.lo(along) < hi && lo < b.lo(along)+b.size(along) && b.lo(across) > cross {
			near = min(near, b.lo(across))
		}
	}
	for _, s := range d.runs() {
		a, b := min(s[along], s[along+2]), max(s[along], s[along+2])
		if c := min(s[across], s[across+2]); b > lo && a < hi && c > cross {
			near = min(near, c)
		}
	}
	// Where an edge written by hand leaves a box past the cut reaches back from the box, maybe over the run, and
	// moves with it: it adds to need from where it starts, but the cut stands short of the nearest box, run or
	// label, its box among them. A hand of a box short of the cut does not move; room made is checked after.
	at = cross + min(column/4, (near-cross)/2)
	first := near
	looped := d.looped()
	for i, box := range d.boxes {
		if box.lo(across) <= at {
			continue
		}
		for _, s := range d.handsOf(i, looped[i]) {
			a, b := min(s[along], s[along+2]), max(s[along], s[along+2])
			if b >= lo && a <= hi {
				first = min(first, s[across], s[across+2])
			}
		}
	}
	if first >= end {
		return 0, 0
	}
	return at, end - first
}

// stretch moves everything past at on axis on by by, the labels with their runs; a box at moves wider, and the
// edges that end on a box widened end on its outline again (a widened hex leaves an end short of its slant).
func (d *drawing) stretch(axis int, at, by float64) {
	widened := map[int]bool{}
	for i, b := range d.boxes {
		switch lo := b.lo(axis); {
		case lo > at:
			b.set(axis, lo+by, b.size(axis))
		case lo+b.size(axis) > at:
			b.set(axis, lo, b.size(axis)+by)
			b.form, widened[i] = outlineOf(b.w, b.h, d.shapes[i]), true
		}
		d.boxes[i] = b
	}
	for c, f := range d.frames {
		switch lo := f.lo(axis); {
		case lo > at:
			f.set(axis, lo+by, f.size(axis))
		case lo+f.size(axis) > at:
			f.set(axis, lo, f.size(axis)+by)
		}
		d.frames[c] = f
	}
	for k, pts := range d.routes {
		for i := range pts {
			if pts[i][axis] > at {
				pts[i][axis] += by
			}
		}
		d.endsOn(k, widened)
	}
	for k, sp := range d.spots {
		if s := d.runOf(sp); runAxis(s) == axis && sp.at > at {
			sp.at += by
			d.spots[k] = sp
		}
	}
}

// endsOn moves the ends of edge k on a box widened onto its outline again, a loop's onto its loop side.
func (d *drawing) endsOn(k int, widened map[int]bool) {
	pts, ends := d.routes[k], d.ends[k]
	last := len(pts) - 1
	if sides, ok := d.level[k]; ok {
		_, along := d.axes()
		for j, i := range [2]int{0, last} {
			if b := d.boxes[ends[j]]; widened[ends[j]] {
				pts[i] = b.form.on(b, sides[j], pts[i][along])
			}
		}
		return
	}
	if d.loops[k] {
		if widened[ends[0]] {
			pts[0], pts[last] = d.loopEnd(ends[0], pts[0]), d.loopEnd(ends[0], pts[last])
		}
		return
	}
	if widened[ends[0]] {
		pts[0] = endOn(d.boxes[ends[0]], pts[0], d.right, !d.back[k])
	}
	if widened[ends[1]] {
		pts[last] = endOn(d.boxes[ends[1]], pts[last], d.right, d.back[k])
	}
}

// span is where what the group draws (boxes, edges and labels) starts and ends on axis; boxSpan, its boxes alone.
func (d *drawing) span(axis int) (lo, hi float64) {
	lo, hi = d.boxSpan(axis)
	for _, f := range d.frames {
		lo, hi = min(lo, f.lo(axis)), max(hi, f.lo(axis)+f.size(axis))
	}
	for k := range d.spots {
		r, _ := d.rect(k)
		lo, hi = min(lo, r.lo(axis)), max(hi, r.lo(axis)+r.size(axis))
	}
	for _, pts := range d.routes {
		for _, p := range pts {
			lo, hi = min(lo, p[axis]), max(hi, p[axis])
		}
	}
	return lo, hi
}

func (d *drawing) boxSpan(axis int) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, r := range d.boxes {
		lo, hi = min(lo, r.lo(axis)), max(hi, r.lo(axis)+r.size(axis))
	}
	return lo, hi
}

// held are the group's boxes, and its labels but edge but's, each with the column between it and its run
// (heldRect).
func (d *drawing) held(but int) []rect {
	out := slices.Collect(maps.Values(d.boxes))
	for c, w := range d.heads {
		out = append(out, headRect(d.frames[c], w))
	}
	for k, sp := range d.spots {
		if k != but {
			out = append(out, heldRect(d.runOf(sp), sp))
		}
	}
	return out
}

// headRect is where the heading w wide of the frame f stands, as addFrame puts it.
func headRect(f rect, w float64) rect {
	return rect{x: f.x + 12, y: f.y + 9, w: w, h: labelH, what: "", at: ""} //nolint:exhaustruct_v5 // a heading's room
}

// heldRect is a label at sp beside run s with the column between them, short of the run by a unit: what is kept
// clear about a label, so that a cut between a label and its run, which moves the label with its run, moves it
// where nothing stands.
func heldRect(s seg, sp spot) rect {
	r := spotRect(s, sp)
	across := 1 - runAxis(s)
	if sp.side > 0 {
		r.set(across, s[across]+1, r.lo(across)+r.size(across)-s[across]-1)
	} else {
		r.set(across, r.lo(across), s[across]-1-r.lo(across))
	}
	return r
}

// runs are the group's edges' runs.
func (d *drawing) runs() []seg {
	out := make([]seg, 0, len(d.routes)*3+len(d.frames)*4)
	for _, pts := range d.routes {
		out = append(out, segsOf(pts)...)
	}
	for _, f := range d.frames { // a label or a loop keeps off a frame's sides too
		out = append(out, seg{f.x, f.y, f.x + f.w, f.y}, seg{f.x + f.w, f.y, f.x + f.w, f.y + f.h},
			seg{f.x, f.y + f.h, f.x + f.w, f.y + f.h}, seg{f.x, f.y, f.x, f.y + f.h})
	}
	return out
}

// hands are the runs from the middle of the sides of each box the graph's edges do not use (left and right laid
// out down, top and bottom laid out right), 2 columns out: where an edge written by hand to the box leaves it, as
// the docs ask, which a label of the graph would stand in the way of. A box's side with loops is theirs.
func (d *drawing) hands() []seg {
	out := make([]seg, 0, 2*len(d.boxes))
	looped := d.looped()
	for i := range d.boxes {
		out = append(out, d.handsOf(i, looped[i])...)
	}
	return out
}

// handsOf are the hands of box i (see hands), but on its loop side when it has loops and on the sides its level
// edges take.
func (d *drawing) handsOf(i int, looped bool) []seg {
	b := d.boxes[i]
	cx, cy, far := b.x+b.w/2, b.y+b.h/2, 2*column
	type hand struct {
		side string
		run  seg
	}
	sides := []hand{{"left", seg{b.x - far, cy, b.x, cy}}, {"right", seg{b.x + b.w, cy, b.x + b.w + far, cy}}}
	if d.right {
		sides = []hand{{"top", seg{cx, b.y - far, cx, b.y}}, {"bottom", seg{cx, b.y + b.h, cx, b.y + b.h + far}}}
	}
	var out []seg
	for _, h := range sides {
		if !d.taken[i][h.side] && (!looped || h.side != d.loopSide()) {
			out = append(out, h.run)
		}
	}
	return out
}

// looped are the boxes with loops.
func (d *drawing) looped() map[int]bool {
	out := map[int]bool{}
	for k := range d.loops {
		out[d.ends[k][0]] = true
	}
	return out
}

// free reports whether r, and 1 about it, meets none of things, and none of runs runs through it: a label touching
// something would meet it once moves add up their rounding.
func free(r rect, things []rect, runs []seg) bool {
	r = rect{x: r.x - 1, y: r.y - 1, w: r.w + 2, h: r.h + 2, what: "", at: ""} //nolint:exhaustruct_v5 // a spot
	return !slices.ContainsFunc(things, r.overlaps) &&
		!slices.ContainsFunc(runs, func(s seg) bool { return crossedBy(s, r) })
}

// lo is where r starts on axis (0 x, 1 y), size how long it is on it, and set sets both.
func (a rect) lo(axis int) float64 {
	return [2]float64{a.x, a.y}[axis]
}

func (a rect) size(axis int) float64 {
	return [2]float64{a.w, a.h}[axis]
}

func (a *rect) set(axis int, lo, size float64) {
	if axis == 0 {
		a.x, a.w = lo, size
	} else {
		a.y, a.h = lo, size
	}
}

// segAt is the run from corner i of pts to the next. A graph's edge has no run of no length (corners leaves none).
func segAt(pts [][2]float64, i int) seg {
	return seg{pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1]}
}

// route is the corners of edge k of gr as drawn from 0,0, its ends on its boxes' outlines,
// from the box it is written from.
func (gr *group) route(k int, drawn map[int]rect, right bool) [][2]float64 {
	chain := gr.chains[k]
	pts := gr.points(chain, right)
	top, bottom := gr.nodes[gr.links[chain[0]].from], gr.nodes[gr.links[chain[len(chain)-1]].to]
	// An end at a port is on its frame's side already.
	if top.box != nil {
		pts[0] = endOn(drawn[top.of], pts[0], right, true)
	}
	if bottom.box != nil {
		pts[len(pts)-1] = endOn(drawn[bottom.of], pts[len(pts)-1], right, false)
	}
	if gr.reversed[k] {
		slices.Reverse(pts)
	}
	return pts
}

// edge is the graph's edge k as an edge drawn by hand: from point to point by via, straight from one to the next.
func (o *graphOut) edge(k int, pts [][2]float64) *edge {
	e := o.g.Edges[k]
	return &edge{
		From: end{Name: "", Point: &pts[0]}, To: end{Name: "", Point: &pts[len(pts)-1]}, FromSide: "", ToSide: "",
		Via: pts[1 : len(pts)-1], Straight: true, Label: nil, At: nil, Dashed: e.Dashed, Bold: e.Bold, Tone: e.Tone,
		Start: e.Start, End: e.End, Crossing: false, who: "edge " + e.From + "→" + e.To, graph: o.at, anchor: "",
	}
}

// endOn moves p, an end on a side of r, across the side onto r's outline: out of its bottom (or right) when it
// leaves, into its top (or left) when it enters.
func endOn(r rect, p [2]float64, right, leaves bool) [2]float64 {
	side, t := map[bool]string{true: "bottom", false: "top"}[leaves], p[0]
	if right {
		side, t = map[bool]string{true: "right", false: "left"}[leaves], p[1]
	}
	return r.form.on(r, side, t)
}

// loopGap is the least room between the ends of loops on a box's side; loopDepth, how far in from the side's line
// an end may stand on the outline: short of a column, where the edges leaving the next side stand (B1).
const (
	loopGap   = 6.0
	loopDepth = column - 2
)

// loopFar is past the side any box needs for its loops beyond its labels: 400 loops on a pill, the most a graph
// holds, need some 600,000, its corners as round as a circle half as wide.
const loopFar = 1e7

// axes are the axis a loop goes out on from its box's side (x laid out down, y laid out right), and the side's.
func (d *drawing) axes() (across, along int) {
	if d.right {
		return 1, 0
	}
	return 0, 1
}

// at is the point a on axis across and c on the other.
func (d *drawing) at(a, c float64) [2]float64 {
	across, along := d.axes()
	var p [2]float64
	p[across], p[along] = a, c
	return p
}

// drawLoops draws the loops of the group's boxes (see loop), each box's in the order written, the boxes in the order
// of their first loop. All boxes grow for their loops before any loop is drawn: a box grown, cut across its middle,
// moves the middles of the boxes of its rank, where edges written by hand leave them and labels keep clear. A box
// grows once, as long as its loops need (loopLong) less what it has grown with the boxes before it.
// LIMIT: the cut across a box's middle runs through its rank, so the boxes of the rank grow with it; and laid out
// right, a loop's labels stand side by side along the bottom, so a long one widens its box. Cut around the box alone,
// or stack the labels, if it comes to matter.
func (o *graphOut) drawLoops(d *drawing) error {
	onBox := map[int][]int{}
	var order []int
	widths := map[int]float64{}
	for _, l := range o.loops {
		k, i := l[0], l[1]
		if _, ok := d.boxes[i]; !ok {
			continue
		}
		if onBox[i] == nil {
			order = append(order, i)
		}
		onBox[i] = append(onBox[i], k)
		if t := o.g.Edges[k].Label; t != nil {
			widths[k] = labelWidth(t, false)
		}
	}
	_, along := d.axes()
	for _, i := range order {
		b := d.boxes[i]
		long, err := d.loopLong(i, onBox[i], widths)
		if err != nil {
			return err
		}
		if long > b.size(along) {
			d.stretch(along, b.lo(along)+b.size(along)/2, long-b.size(along))
		}
	}
	for _, i := range order { // a box grows with the boxes after it, which leaves its loops room as they were
		if !d.loopFit(i, onBox[i], widths, d.boxes[i].size(along)).ok {
			return fmt.Errorf("box %d grown for its loops is short of them", i) //nolint:err113 // floe's own
		}
	}
	for _, i := range order {
		if err := d.loop(i, onBox[i], widths); err != nil {
			return err
		}
	}
	return nil
}

// fit is where the loops of a box stand on its side: their ends part apart, out from the middle, and their labels,
// stack long one after another along the side and broad across, and whether the box is long enough for them.
type fit struct {
	part, stack, broad float64
	ok                 bool
}

// loopFit is how the loops ks of box i fit its side: their ends stand loopGap apart or more, where the outline is
// within loopDepth of the side's line however wide the box grows across later (as room is made, cut along its
// side), its corners rounding no wider than the side allows, and their labels within the side; measured as if the
// side were long.
func (d *drawing) loopFit(i int, ks []int, widths map[int]float64, long float64) fit {
	across, along := d.axes()
	var f fit
	for _, k := range ks {
		if w, ok := widths[k]; ok {
			stacks, broad := float64(labelH), w
			if d.right {
				stacks, broad = w, labelH
			}
			f.stack, f.broad = f.stack+stacks+2, max(f.broad, broad)
		}
	}
	n := float64(len(ks))
	b := d.boxes[i]
	b.set(along, b.lo(along), long)
	// However wide the box grows across, its corners round no wider than half its side: measured on the box as
	// wide as its side is long. A hexagon laid out right slants across its bottom as deep as it grows tall: its
	// loops stay off the slants.
	worst := b
	worst.set(across, b.lo(across), max(b.size(across), b.size(along)))
	worst.form = outlineOf(worst.w, worst.h, d.shapes[i])
	far, mid := worst.lo(across)+worst.size(across), b.lo(along)+b.size(along)/2
	deep := func(off float64) bool {
		if d.right && d.shapes[i] == "hex" && b.size(along)/2-off < hexInset {
			return true
		}
		return max(far-worst.form.on(worst, d.loopSide(), mid-off)[across],
			far-worst.form.on(worst, d.loopSide(), mid+off)[across]) > loopDepth
	}
	f.part = math.Floor(b.size(along) / (2*n + 2))
	for f.part >= loopGap && deep(n*f.part) {
		f.part--
	}
	f.ok = f.part >= loopGap && b.size(along) >= f.stack+2
	return f
}

// loopLong is how long box i's loop side is to be for its loops ks: as long as it is, or longer by whole columns, the
// least that fits them (loopFit, which a longer side fits too) found by halving between a side too short and one
// long enough, the labels' length first. It fails past a side no box of a graph's bounds needs.
func (d *drawing) loopLong(i int, ks []int, widths map[int]float64) (float64, error) {
	_, along := d.axes()
	have := d.boxes[i].size(along)
	f := d.loopFit(i, ks, widths, have)
	if f.ok {
		return have, nil
	}
	short, long := have, have+max(column, onGrid(f.stack+2-have, column, math.Ceil))
	labelled := long
	for step := column; !d.loopFit(i, ks, widths, long).ok; step *= 2 {
		if long-labelled > loopFar {
			return 0, fmt.Errorf("box %d is short of its loops %v long", i, loopFar) //nolint:err113 // floe's own
		}
		short, long = long, long+step
	}
	for long-short > column {
		mid := short + onGrid((long-short)/2, column, math.Floor)
		if d.loopFit(i, ks, widths, mid).ok {
			long = mid
		} else {
			short = mid
		}
	}
	return long, nil
}

// loopSide is the side of a box its loops leave: the one the graph's edges do not use.
func (d *drawing) loopSide() string {
	if d.right {
		return "bottom"
	}
	return "right"
}

// loop draws ks, the edges from box i to itself, on its loop side: the k-th out of the side (k+1) parts before its
// middle, out by k+2 columns and back in as far past, so that they nest; their labels stand one after another a
// column out from the outer loop's run across the side, within the side. With something in the way of the loops,
// their labels and a column more, room is made as for a label beside a run (reach): a cut along the side, short of
// the nearest thing, moves all past it on. Room made is clear by how it is made; it fails when it is not.
func (d *drawing) loop(i int, ks []int, widths map[int]float64) error {
	across, along := d.axes()
	n := float64(len(ks))
	f := d.loopFit(i, ks, widths, d.boxes[i].size(along))
	b := d.boxes[i]
	far, mid := b.lo(across)+b.size(across), b.lo(along)+b.size(along)/2
	area := func() rect {
		var r rect
		r.set(across, far+1, column*(n+2)-1+column+f.broad)
		r.set(along, min(mid-n*f.part, mid-f.stack/2), max(2*n*f.part, f.stack))
		return r
	}
	for _, k := range ks { // its side is the loops' now, no hand's
		d.ends[k], d.loops[k] = [2]int{i, i}, true
	}
	if at, need := d.reach(area(), far, along); need > 0 {
		d.stretch(across, at, onGrid(need, column, math.Ceil))
		b = d.boxes[i]
		far = b.lo(across) + b.size(across)
	}
	if !free(area(), d.held(-1), append(d.runs(), d.hands()...)) {
		return fmt.Errorf("room made for the loops of box %d is not clear", i) //nolint:err113 // floe's own
	}
	for j, k := range ks {
		out, off := far+column*float64(j+2), f.part*float64(j+1)
		d.routes[k] = [][2]float64{
			d.loopEnd(i, d.at(far, mid-off)), d.at(out, mid-off), d.at(out, mid+off), d.loopEnd(i, d.at(far, mid+off)),
		}
	}
	outer, at := ks[len(ks)-1], mid-f.stack/2+1
	for _, k := range ks {
		if w, ok := widths[k]; ok {
			d.spots[k] = spot{edge: outer, seg: 1, side: 1, at: at, w: w}
			at += labelH + 2
			if d.right {
				at += w - labelH
			}
		}
	}
	return nil
}

// loopEnd moves p, an end of a loop on box i's loop side, onto its outline.
func (d *drawing) loopEnd(i int, p [2]float64) [2]float64 {
	b := d.boxes[i]
	_, along := d.axes()
	return b.form.on(b, d.loopSide(), p[along])
}
