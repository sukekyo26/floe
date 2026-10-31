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
	left := 0.0
	for _, gr := range o.groups {
		gr.place(right)
		gr.wire()
		d := &drawing{across: across, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{},
			ends: map[int][2]int{}, labels: map[int]rect{}}
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
				d.ends[k] = [2]int{d.ends[k][1], d.ends[k][0]}
				d.back = append(d.back, k)
			}
		}
		for _, k := range edges {
			if t := o.g.Edges[k].Label; t != nil && !d.place(k, labelWidth(t, false)) {
				return nil, fmt.Errorf("diagram: the graph at %s found no room for edge %d's label", o.at, k) //nolint:err113 // floe's own
			}
		}
		// The group a column right of left, its boxes on the grid as laid out (a label before them moves it on by
		// whole columns), and the top left of what it draws at x and y.
		start := [2]float64{o.g.X, o.g.Y}
		lo, hi := d.span(across)
		boxLo, _ := d.boxSpan(across)
		by := left
		if lo < boxLo {
			by += onGrid(boxLo-lo, column, math.Ceil)
		}
		d.stretch(across, math.Inf(-1), by)
		if top, _ := d.span(1 - across); top < start[1-across] {
			d.stretch(1-across, math.Inf(-1), start[1-across]-top)
		}
		left = onGrid(hi+by-start[across]+groupGap, column, math.Ceil)
		maps.Copy(routes, d.routes)
		maps.Copy(labels, d.labels)
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
		}
		out = append(out, item{edgeAt(o.at, k), part{Edge: e}}) //nolint:exhaustruct_v5 // one kind of part
	}
	return out, nil
}

// drawing is a group laid out, as drawn, while its labels are placed: its boxes and its edges' corners and labels.
type drawing struct {
	across int // the axis edges leave their boxes across: 0 (x) for down, 1 (y) for right
	boxes  map[int]rect
	shapes map[int]string
	routes map[int][][2]float64
	ends   map[int][2]int // the boxes each edge's corners start and end on
	back   []int          // the edges drawn up, against the ranks
	labels map[int]rect
}

// place places edge k's label, w wide: beside the first straight run of its edge with room for it, from the box it
// is written from; with none, it makes room beside the edge's first run (see room). It reports false only on
// floe's fault, when the room made is not clear.
func (d *drawing) place(k int, w float64) bool {
	pts := d.routes[k]
	things, runs := d.things(), d.runs()
	for _, s := range segsOf(pts) {
		down := s[0] == s[2]
		lo, hi := min(s[1], s[3]), max(s[1], s[3])
		long := float64(labelH)
		if !down {
			lo, hi, long = min(s[0], s[2]), max(s[0], s[2]), w
		}
		for at := lo + 2; at+long <= hi-2; at += 6 {
			for _, near := range []bool{true, false} {
				r := rect{x: s[0] + column, y: at, w: w, h: labelH, what: "", at: ""} //nolint:exhaustruct_v5 // a spot
				switch {
				case down && !near:
					r.x = s[0] - column - w
				case !down:
					r.x, r.y = at, s[1]+column
					if !near {
						r.y = s[1] - column - labelH
					}
				}
				if clear(r, things, runs) {
					d.labels[k] = r
					return true
				}
			}
		}
	}
	r := d.room(k, w)
	ok := clear(r, d.things(), d.runs())
	d.labels[k] = r
	return ok
}

// room makes room for edge k's label beside the first run of its edge, out of the box it is written from, and
// returns where the label stands: the drawing is cut across the run just past the box (past every box and label the
// cut would cross), and all that lies past the cut moves on along the run by the label's length; then it is cut along the run,
// a quarter column past it, and all that lies past that cut moves on across as far as the label needs, the boxes
// the cut crosses widening. Neither cut bends a run or makes two things meet: a straight run stays straight, and a
// run across a cut grows longer.
func (d *drawing) room(k int, w float64) rect {
	along := 1 - d.across
	size := [2]float64{w, labelH} // across and along, as laid out down
	if d.across == 1 {
		size = [2]float64{labelH, w}
	}
	pts := d.routes[k]
	p0, sign := pts[0], 1.0
	if pts[1][along] < p0[along] {
		sign = -1
	}
	cut := p0[along] + sign
	for moved := true; moved; {
		moved = false
		for _, b := range d.things() {
			if lo, hi := b.lo(along), b.lo(along)+b.size(along); lo <= cut && cut <= hi {
				cut, moved = map[bool]float64{true: hi + 1, false: lo - 1}[sign > 0], true
			}
		}
	}
	d.stretch(along, cut, size[1]+4)
	r := rect{what: "", at: ""} //nolint:exhaustruct_v5 // a spot
	r.set(along, cut+2, size[1])
	x := d.routes[k][0][d.across]
	r.set(d.across, x+column, size[0])
	if need := d.reach(r, x); need > 0 {
		d.stretch(d.across, x+3, onGrid(need, column, math.Ceil))
	}
	return r
}

// reach is how far past the run at x the nearest thing within r's span along stands short of r's far end, and a
// little: everything past the cut, its edge at r's near end too, moves on across with the cut. 0 when nothing does.
func (d *drawing) reach(r rect, x float64) float64 {
	along := 1 - d.across
	lo, hi := r.lo(along), r.lo(along)+r.size(along)
	end := r.lo(d.across) + r.size(d.across) + 2
	near := math.Inf(1)
	for _, b := range d.things() {
		if b.lo(along) < hi && lo < b.lo(along)+b.size(along) && b.lo(d.across) > x && b.lo(d.across) < end {
			near = min(near, b.lo(d.across))
		}
	}
	for _, s := range d.runs() {
		{
			a, b := min(s[along], s[along+2]), max(s[along], s[along+2])
			if c := min(s[d.across], s[d.across+2]); b > lo && a < hi && c > x && c < end {
				near = min(near, c)
			}
		}
	}
	if math.IsInf(near, 1) {
		return 0
	}
	return end - near
}

// stretch moves everything past at on axis on by by; a box at moves wider.
func (d *drawing) stretch(axis int, at, by float64) {
	for i, b := range d.boxes {
		switch lo := b.lo(axis); {
		case lo > at:
			b.set(axis, lo+by, b.size(axis))
		case lo+b.size(axis) > at:
			b.set(axis, lo, b.size(axis)+by)
			b.form = outlineOf(b.w, b.h, d.shapes[i])
		}
		d.boxes[i] = b
	}
	for k, pts := range d.routes {
		for i := range pts {
			if pts[i][axis] > at {
				pts[i][axis] += by
			}
		}
		// Each end back onto its box's outline: a box widened may leave an end on a side's slant short of it.
		ends, leaves := d.ends[k], [2]bool{true, false}
		if slices.Contains(d.back, k) {
			leaves = [2]bool{false, true}
		}
		pts[0] = endOn(d.boxes[ends[0]], pts[0], d.across == 1, leaves[0])
		pts[len(pts)-1] = endOn(d.boxes[ends[1]], pts[len(pts)-1], d.across == 1, leaves[1])
	}
	for k, r := range d.labels {
		if r.lo(axis) > at {
			r.set(axis, r.lo(axis)+by, r.size(axis))
			d.labels[k] = r
		}
	}
}

// span is where what the group draws (boxes, edges and labels) starts and ends on axis; boxSpan, its boxes alone.
func (d *drawing) span(axis int) (lo, hi float64) {
	lo, hi = d.boxSpan(axis)
	for _, r := range d.labels {
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

// things are the group's boxes and labels.
func (d *drawing) things() []rect {
	return slices.Concat(slices.Collect(maps.Values(d.boxes)), slices.Collect(maps.Values(d.labels)))
}

// runs are the group's edges' runs.
func (d *drawing) runs() []seg {
	var out []seg
	for _, pts := range d.routes {
		out = append(out, segsOf(pts)...)
	}
	return out
}

// clear reports whether r, and 1 about it, meets none of things, and none of runs runs through it: a label touching
// something would meet it once moves add up their rounding.
func clear(r rect, things []rect, runs []seg) bool {
	r = rect{x: r.x - 1, y: r.y - 1, w: r.w + 2, h: r.h + 2, what: "", at: ""} //nolint:exhaustruct_v5 // a spot
	return !slices.ContainsFunc(things, r.overlaps) && !slices.ContainsFunc(runs, func(s seg) bool { return crossedBy(s, r) })
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

// segsOf is the runs between corners pts.
func segsOf(pts [][2]float64) []seg {
	segs := make([]seg, 0, len(pts)-1)
	for i := 1; i < len(pts); i++ {
		segs = append(segs, seg{pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]})
	}
	return segs
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
		Start: e.Start, End: e.End, Crossing: false, who: "edge " + e.From + "→" + e.To, graph: o.at,
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
