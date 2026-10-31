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

// gnode is a box of a graph, or a point where an edge passes a rank.
type gnode struct {
	box     *graphBox // nil for a point an edge passes
	of      int       // the box's index in nodes; for a point, its edge's index in edges
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
}

// graphOut is a graph read and ranked: its groups, and the ranks its edges pass between their ends.
type graphOut struct {
	g      *graph
	at     string
	groups []*group
	passed int
}

// readGraph resolves the names of g's edges against its boxes, and splits it into groups, each ranked. An edge
// naming no box of it, or from a box to itself, is dropped and told.
// LIMIT: an edge from a box to itself is refused; B3 lays it out.
func readGraph(g *graph, at string) (*graphOut, usererr.List) {
	var problems usererr.List
	byName := map[string]int{}
	for i, n := range g.Nodes {
		if _, ok := byName[n.Box.Name]; !ok {
			byName[n.Box.Name] = i
		}
	}
	out := &graphOut{g: g, at: at, groups: nil, passed: 0}
	var edges []int
	for k, e := range g.Edges {
		known := true
		for _, end := range [2][2]string{{"from", e.From}, {"to", e.To}} {
			if _, ok := byName[end[1]]; !ok {
				problems = append(problems, usererr.NewIn("", edgeAt(at, k)+"/"+end[0], "diagram_graph_unknown", end[1]))
				known = false
			}
		}
		switch {
		case !known:
		case e.From == e.To:
			problems = append(problems, usererr.NewIn("", edgeAt(at, k), "diagram_edge_nowhere", "edge "+e.From+"→"+e.To))
		default:
			edges = append(edges, k)
		}
	}
	for _, gr := range groupsOf(g, edges, byName) {
		out.groups = append(out.groups, gr)
		out.passed += gr.passed()
	}
	return out, problems
}

// groupsOf splits the boxes of g into the groups the edges (by index) join, in the order of their first box, each
// ranked.
func groupsOf(g *graph, edges []int, byName map[string]int) []*group {
	root := make([]int, len(g.Nodes))
	for i := range root {
		root[i] = i
	}
	find := func(i int) int {
		for root[i] != i {
			i = root[i]
		}
		return i
	}
	for _, k := range edges {
		a, b := find(byName[g.Edges[k].From]), find(byName[g.Edges[k].To])
		root[max(a, b)] = min(a, b)
	}
	members := map[int][]int{}
	var firsts []int
	for i := range g.Nodes {
		r := find(i)
		if members[r] == nil {
			firsts = append(firsts, r)
		}
		members[r] = append(members[r], i)
	}
	groups := make([]*group, 0, len(firsts))
	for _, r := range firsts {
		var own []int
		for _, k := range edges {
			if find(byName[g.Edges[k].From]) == r {
				own = append(own, k)
			}
		}
		groups = append(groups, rank(g, members[r], own, byName))
	}
	return groups
}

func edgeAt(at string, k int) string {
	return at + "/edges/" + strconv.Itoa(k)
}

func boxAt(at string, i int) string {
	return at + "/nodes/" + strconv.Itoa(i) + "/box"
}

// items lays out each group and places its labels, side by side in the order of their first box, and opens the graph
// into a box for each of its boxes and an edge for each of its edges, at their JSON pointers, so that they are named,
// drawn and checked as boxes and edges written by hand are.
func (o *graphOut) items() ([]item, error) {
	right := o.g.Direction == "right"
	across := 0 // the axis groups stand side by side on, as drawn
	if right {
		across = 1
	}
	boxes := make([]rect, len(o.g.Nodes))
	routes := map[int][][2]float64{}
	labels := map[int]rect{}
	toward := map[int]bool{}
	left := 0.0
	for _, gr := range o.groups {
		d, err := o.draw(gr, right)
		if err != nil {
			return nil, err
		}
		left = d.stand(across, left, [2]float64{o.g.X, o.g.Y})
		maps.Copy(routes, d.routes)
		for k := range d.spots {
			labels[k], toward[k] = d.rect(k)
		}
		for i, r := range d.boxes {
			boxes[i] = r
		}
	}
	out := make([]item, 0, len(o.g.Nodes)+len(o.g.Edges))
	for i, n := range o.g.Nodes {
		r := boxes[i]
		b := &box{
			Name: n.Box.Name, X: r.x, Y: r.y, Origin: "", W: r.w, H: r.h, Rows: n.Box.Rows, Tone: n.Box.Tone,
			Shape: n.Box.Shape, graph: o.at,
		}
		out = append(out, item{boxAt(o.at, i), part{Box: b}}) //nolint:exhaustruct_v5 // one kind of part
	}
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
	return out, nil
}

// draw lays group gr out and places its labels, as drawn at the graph's place.
func (o *graphOut) draw(gr *group, right bool) (*drawing, error) {
	gr.place(right)
	gr.wire()
	d := &drawing{
		right: right, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{},
		ends: map[int][2]int{}, back: map[int]bool{}, spots: map[int]spot{},
	}
	for _, n := range gr.nodes {
		if n.box != nil {
			r := drawnOf(n, right)
			r.x, r.y = r.x+o.g.X, r.y+o.g.Y
			r.form = outlineOf(r.w, r.h, n.box.Shape)
			d.boxes[n.of], d.shapes[n.of] = r, n.box.Shape
		}
	}
	edges := slices.Sorted(maps.Keys(gr.chains))
	for _, k := range edges {
		d.routes[k] = o.route(gr, k, d.boxes, right)
		chain := gr.chains[k]
		d.ends[k] = [2]int{gr.nodes[gr.links[chain[0]].from].of, gr.nodes[gr.links[chain[len(chain)-1]].to].of}
		if gr.reversed[k] {
			d.ends[k], d.back[k] = [2]int{d.ends[k][1], d.ends[k][0]}, true
		}
	}
	for _, k := range edges {
		if t := o.g.Edges[k].Label; t != nil {
			if err := d.place(k, labelWidth(t, false)); err != nil {
				return nil, fmt.Errorf("diagram: the graph at %s: %w", o.at, err)
			}
		}
	}
	return d, nil
}

// stand moves the group a column past left on axis across, its boxes on the grid as laid out (a label before them
// moves it on by whole columns), and the top left of what it draws to start; it returns where the next group goes.
func (d *drawing) stand(across int, left float64, start [2]float64) float64 {
	lo, hi := d.span(across)
	boxLo, _ := d.boxSpan(across)
	by := left
	if lo < boxLo {
		by += onGrid(boxLo-lo, column, math.Ceil)
	}
	d.stretch(across, math.Inf(-1), by)
	if top, _ := d.span(1 - across); top < start[1-across] {
		d.stretch(1-across, math.Inf(-1), onGrid(start[1-across]-top, column, math.Ceil))
	}
	return onGrid(hi+by-start[across]+groupGap, column, math.Ceil)
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
}

// spot is where a label stands, held to its run so that it moves with it: beside run seg of its edge's corners, a
// column off it on side (+1 right of a run down or below a run across, -1 left or above), from at along it.
type spot struct {
	seg  int
	side float64
	at   float64
	w    float64
}

// rect is where edge k's label stands now, and whether its text is set toward its run, at its right end (a label
// left of a run down).
func (d *drawing) rect(k int) (rect, bool) {
	sp := d.spots[k]
	return spotRect(segAt(d.routes[k], sp.seg), sp), sp.side < 0 && isDown(segAt(d.routes[k], sp.seg))
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
				sp := spot{seg: i, side: side, at: a, w: w}
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
	if (cut-to)*sign >= 0 {
		return spot{seg: 0, side: 0, at: 0, w: 0}, false
	}
	cross := s[1-along]
	stays, moves := d.crossed(along, cut, cross+1, cross+column+size[0])
	d.stretch(along, cut, stays+size[1]+4+moves)
	sp := spot{seg: i, side: 1, at: cut + stays + 2, w: w}
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
	for k, sp := range d.spots {
		s := segAt(d.routes[k], sp.seg)
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
// run within r's span along it (a run, a box, a label held to its run) stands short of r's far end by need, and a
// little; the cut is between the run and that thing, a quarter column past the run or halfway, the nearer, so that
// nothing within r's span stays between the run and the cut. need is 0 when nothing stands in r's way.
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
	if near >= end {
		return 0, 0
	}
	return cross + min(column/4, (near-cross)/2), end - near
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
	for k, pts := range d.routes {
		for i := range pts {
			if pts[i][axis] > at {
				pts[i][axis] += by
			}
		}
		ends := d.ends[k]
		if widened[ends[0]] {
			pts[0] = endOn(d.boxes[ends[0]], pts[0], d.right, !d.back[k])
		}
		if widened[ends[1]] {
			pts[len(pts)-1] = endOn(d.boxes[ends[1]], pts[len(pts)-1], d.right, d.back[k])
		}
	}
	for k, sp := range d.spots {
		if s := segAt(d.routes[k], sp.seg); runAxis(s) == axis && sp.at > at {
			sp.at += by
			d.spots[k] = sp
		}
	}
}

// span is where what the group draws (boxes, edges and labels) starts and ends on axis; boxSpan, its boxes alone.
func (d *drawing) span(axis int) (lo, hi float64) {
	lo, hi = d.boxSpan(axis)
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
	for k, sp := range d.spots {
		if k != but {
			out = append(out, heldRect(segAt(d.routes[k], sp.seg), sp))
		}
	}
	return out
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
	out := make([]seg, 0, len(d.routes)*3)
	for _, pts := range d.routes {
		out = append(out, segsOf(pts)...)
	}
	return out
}

// hands are the runs from the middle of the sides of each box the graph's edges do not use (left and right laid
// out down, top and bottom laid out right), 2 columns out: where an edge written by hand to the box leaves it, as
// the docs ask, which a label of the graph would stand in the way of.
func (d *drawing) hands() []seg {
	var out []seg
	for _, b := range d.boxes {
		cx, cy, far := b.x+b.w/2, b.y+b.h/2, 2*column
		if d.right {
			out = append(out, seg{cx, b.y - far, cx, b.y}, seg{cx, b.y + b.h, cx, b.y + b.h + far})
		} else {
			out = append(out, seg{b.x - far, cy, b.x, cy}, seg{b.x + b.w, cy, b.x + b.w + far, cy})
		}
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

// route is the corners of edge k of group gr as drawn, at the graph's place, its ends on its boxes' outlines,
// from the box it is written from.
func (o *graphOut) route(gr *group, k int, drawn map[int]rect, right bool) [][2]float64 {
	chain := gr.chains[k]
	pts := gr.points(chain, right)
	for i := range pts {
		pts[i] = [2]float64{pts[i][0] + o.g.X, pts[i][1] + o.g.Y}
	}
	top, bottom := gr.nodes[gr.links[chain[0]].from], gr.nodes[gr.links[chain[len(chain)-1]].to]
	pts[0] = endOn(drawn[top.of], pts[0], right, true)
	pts[len(pts)-1] = endOn(drawn[bottom.of], pts[len(pts)-1], right, false)
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
