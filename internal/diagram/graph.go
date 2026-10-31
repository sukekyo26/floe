package diagram

import (
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
	box     *graphBox // nil for a point an edge passes, or its label's box
	label   Text      // the label whose box it is; nil for a box or a point
	of      int       // the box's index in nodes; for a point or a label's box, its edge's index in edges
	rank    int
	w, h    float64 // across the ranks and along them, as laid out down
	x, y    float64 // its left and top, as laid out down
	col     float64 // for a point or a label's box, the column its edge enters it at
	jog     float64 // how far right of col its edge leaves it: 0, or half a column when it is entered bent
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
	labels   map[int]Text  // the labels of its edges, by index in edges, when they take two ranks
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
	// A label on an edge kept doubles the ranks of every group of the graph (a dropped edge's does not).
	labelled := slices.ContainsFunc(edges, func(k int) bool { return g.Edges[k].Label != nil })
	groups := make([]*group, 0, len(firsts))
	for _, r := range firsts {
		var own []int
		for _, k := range edges {
			if find(byName[g.Edges[k].From]) == r {
				own = append(own, k)
			}
		}
		groups = append(groups, rank(g, members[r], own, byName, labelled))
	}
	return groups
}

func edgeAt(at string, k int) string {
	return at + "/edges/" + strconv.Itoa(k)
}

func boxAt(at string, i int) string {
	return at + "/nodes/" + strconv.Itoa(i) + "/box"
}

// items lays out each group, side by side in the order of their first box, and opens the graph into a box for each
// of its boxes and an edge for each of its edges, at their JSON pointers, so that they are named, drawn and checked
// as boxes and edges written by hand are.
func (o *graphOut) items() []item {
	right := o.g.Direction == "right"
	left := 0.0
	for _, gr := range o.groups {
		gr.place(right)
		gr.wire()
		gr.shift(left)
		width := 0.0
		for _, n := range gr.nodes {
			width = max(width, n.right())
		}
		left = onGrid(width+groupGap, 12, math.Ceil)
	}
	type spot struct {
		gr *group
		n  int
	}
	where := map[int]spot{}
	owner := map[int]*group{}
	for _, gr := range o.groups {
		for i, n := range gr.nodes {
			if n.box != nil {
				where[n.of] = spot{gr, i}
			}
		}
		for k := range gr.chains {
			owner[k] = gr
		}
	}
	drawn := make([]rect, len(o.g.Nodes))
	out := make([]item, 0, len(o.g.Nodes)+len(o.g.Edges))
	for i, n := range o.g.Nodes {
		w := where[i]
		r := drawnOf(w.gr.nodes[w.n], right)
		r.x, r.y = r.x+o.g.X, r.y+o.g.Y
		r.form = outlineOf(r.w, r.h, n.Box.Shape)
		drawn[i] = r
		b := &box{
			Name: n.Box.Name, X: r.x, Y: r.y, Origin: "", W: r.w, H: r.h, Rows: n.Box.Rows, Tone: n.Box.Tone,
			Shape: n.Box.Shape, graph: o.at,
		}
		out = append(out, item{boxAt(o.at, i), part{Box: b}}) //nolint:exhaustruct_v5 // one kind of part
	}
	for k := range o.g.Edges {
		if gr := owner[k]; gr != nil { // else told by readGraph
			at, anchor := o.labelAt(gr, k, right)
			e := o.edge(k, o.route(gr, k, drawn, right), at, anchor)
			out = append(out, item{edgeAt(o.at, k), part{Edge: e}}) //nolint:exhaustruct_v5 // one kind of part
		}
	}
	return out
}

// route is the corners of edge k of group gr as drawn, at the graph's place, its ends on its boxes' outlines,
// from the box it is written from.
func (o *graphOut) route(gr *group, k int, drawn []rect, right bool) [][2]float64 {
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

// labelAt is where edge k's label stands in its label's box, and how: down, its text starts a column right of the
// edge's column (5 in, as label pads it), so that a language narrower than the widest stays by the edge; right, it is
// centred along the box, under the edge. As drawn at the graph's place: as laid out down (the group's place added),
// turned for right, then at x and y. Nil for an edge without one.
func (o *graphOut) labelAt(gr *group, k int, right bool) (*[2]float64, string) {
	for _, l := range gr.chains[k] {
		if n := gr.nodes[gr.links[l].to]; n.label != nil {
			p, anchor := [2]float64{n.col + column + 5, n.y + (n.h-inEnd)/2}, "start"
			if right {
				p, anchor = [2]float64{n.y + (n.h-inEnd)/2, n.col + column + (n.w-column)/2}, "middle"
			}
			return &[2]float64{p[0] + o.g.X, p[1] + o.g.Y}, anchor
		}
	}
	return nil, ""
}

// edge is the graph's edge k as an edge drawn by hand: from point to point by via, straight from one to the next.
func (o *graphOut) edge(k int, pts [][2]float64, at *[2]float64, anchor string) *edge {
	e := o.g.Edges[k]
	var label Text
	if at != nil {
		label = e.Label
	}
	return &edge{
		From: end{Name: "", Point: &pts[0]}, To: end{Name: "", Point: &pts[len(pts)-1]}, FromSide: "", ToSide: "",
		Via: pts[1 : len(pts)-1], Straight: true, Label: label, At: at, Dashed: e.Dashed, Bold: e.Bold, Tone: e.Tone,
		Start: e.Start, End: e.End, Crossing: false, who: "edge " + e.From + "→" + e.To, graph: o.at, anchor: anchor,
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
