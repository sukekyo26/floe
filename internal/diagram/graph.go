package diagram

import (
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
}

// graphOut is a graph read and ranked: its groups, its self-loops, and the boxes of each name.
type graphOut struct {
	g      *graph
	at     string
	groups []*group
	loops  []int // the edges from a box to itself, by index
	passed int   // the ranks its edges pass between their ends
}

// readGraph resolves the names of g's edges against its boxes, an edge naming no box of it dropped and told, and
// splits it into groups, each ranked. A self-loop is kept apart, to be drawn as an edge by name.
func readGraph(g *graph, at string) (*graphOut, usererr.List) {
	var problems usererr.List
	byName := map[string]int{}
	for i, n := range g.Nodes {
		if _, ok := byName[n.Box.Name]; !ok {
			byName[n.Box.Name] = i
		}
	}
	out := &graphOut{g: g, at: at, groups: nil, loops: nil, passed: 0}
	var edges []int
	for k, e := range g.Edges {
		known := true
		for _, name := range []string{e.From, e.To} {
			if _, ok := byName[name]; !ok {
				problems = append(problems, usererr.NewIn("", edgeAt(at, k), "diagram_graph_unknown", name))
				known = false
			}
		}
		switch {
		case !known:
		case e.From == e.To:
			out.loops = append(out.loops, k)
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

// passed is how many points the group's edges pass ranks at.
func (gr *group) passed() int {
	n := 0
	for _, nd := range gr.nodes {
		if nd.box == nil {
			n++
		}
	}
	return n
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

// items lays out each group, side by side in the order of their first box, and opens the graph into a box for each
// of its boxes and an edge for each of its edges, at their JSON pointers, so that they are named, drawn and checked
// as boxes and edges written by hand are.
func (o *graphOut) items() []item {
	right := o.g.Direction == "right"
	left := 0.0
	for _, gr := range o.groups {
		gr.place(right)
		gr.wire()
		width := 0.0
		for i := range gr.nodes {
			gr.nodes[i].x += left
			width = max(width, gr.nodes[i].x+gr.nodes[i].w)
		}
		for i := range gr.links {
			gr.links[i].exit += left
			gr.links[i].entry += left
		}
		left = ceil12(width + groupGap)
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
	for k, e := range o.g.Edges {
		var p part
		switch gr := owner[k]; {
		case gr != nil:
			p.Edge = o.edge(k, o.route(gr, k, drawn, right))
		case slices.Contains(o.loops, k):
			p.Edge = &edge{ //nolint:exhaustruct_v5 // the rest as left out
				From: end{Name: e.From, Point: nil}, To: end{Name: e.To, Point: nil}, Dashed: e.Dashed, Bold: e.Bold,
				Tone: e.Tone, Start: e.Start, End: e.End, graph: o.at,
			}
		default:
			continue // naming no box of the graph, told
		}
		out = append(out, item{edgeAt(o.at, k), p})
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
