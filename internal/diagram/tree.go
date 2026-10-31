package diagram

import (
	"strconv"

	"github.com/sukekyo26/floe/internal/usererr"
)

// item is a part to lay out, at its JSON pointer in the spec; a tree's nodes and edges are items of their own.
type item struct {
	at string
	p  part
}

// treeGapX is the room between the spans of siblings, treeGapY between the tallest node of a depth and the next.
const treeGapX, treeGapY = 24.0, 40.0

// expand is the parts with each tree opened into a box for each node and an edge from each node to each child,
// at the node's JSON pointer, so that they are named, laid out and checked as boxes and edges written by hand are.
// It reads the nodes read into their types, and walks no value along the schema.
// LIMIT: trees grow down alone, and a subtree's span is not shared with its neighbours' (a wide row of leaves
// widens every span above it); pack them if wide trees waste room.
func expand(parts []part) ([]item, usererr.List) {
	var problems usererr.List
	graphs := map[int]*graphOut{}
	passed := 0
	for i, p := range parts {
		if p.Graph != nil {
			o, found := readGraph(p.Graph, pointer(i)+"/graph")
			problems = append(problems, found...)
			graphs[i] = o
			passed += o.passed
		}
	}
	// Over the limit, no graph is laid out: the limit is the diagram's, so that no one graph is told to change.
	// LIMIT: an edge written by hand to a box of a graph not laid out is told naming nothing too; the graphs come
	// first.
	if passed > maxPassed {
		problems = append(problems, usererr.NewIn("", "#", "diagram_graph_long", passed, maxPassed))
		graphs = nil
	}
	out := make([]item, 0, len(parts))
	for i, p := range parts {
		switch {
		case p.Tree != nil:
			m := measures{spans: map[*node]float64{}, heights: nil}
			m.measure(&p.Tree.Root, 0)
			m.place(&out, &p.Tree.Root, p.Tree.X, p.Tree.Y, 0, pointer(i)+"/tree/root")
		case p.Graph != nil:
			if o := graphs[i]; o != nil {
				out = append(out, o.items()...)
			}
		default:
			out = append(out, item{pointer(i), p})
		}
	}
	return out, problems
}

// measures are a tree's sizes, measured once from its leaves up: how wide each node stands with its subtree, and
// the height of the tallest node at each depth, so that a depth stands in one row.
type measures struct {
	spans   map[*node]float64
	heights []float64
}

func nodeBox(n *node, cx, y float64) *box {
	return &box{Name: n.Name, X: cx, Y: y, Origin: "centre", W: 0, H: 0, Rows: n.Rows, Tone: n.Tone, Shape: n.Shape, graph: ""}
}

// measure keeps n's span, its box or its children side by side, the wider, and the tallest node of its depth.
func (m *measures) measure(n *node, d int) {
	r := boxRect(nodeBox(n, 0, 0), "")
	if len(m.heights) <= d {
		m.heights = append(m.heights, 0)
	}
	m.heights[d] = max(m.heights[d], r.h)
	for i := range n.Children {
		m.measure(&n.Children[i], d+1)
	}
	m.spans[n] = max(r.w, m.childrenSpan(n))
}

// childrenSpan is how wide n's children stand side by side.
func (m *measures) childrenSpan(n *node) float64 {
	if len(n.Children) == 0 {
		return 0
	}
	sum := treeGapX * float64(len(n.Children)-1)
	for i := range n.Children {
		sum += m.spans[&n.Children[i]]
	}
	return sum
}

// nodeWho names n as an edge's end names a part: its name, or else the English of its first row in quotes.
// LIMIT: two nodes of one text and no name read alike in a mistake that names both; name them if it confuses.
func nodeWho(n *node) string {
	if n.Name != "" {
		return n.Name
	}
	return strconv.Quote(n.Rows[0].Text["en"])
}

// place adds n's box at depth d, its centre at cx and its top at y, and below it its children side by side,
// centred under it. Each edge goes down from n's bottom to the child's top, turning across at one height for the
// depth: halfway between its tallest node and the next depth. A child right below goes straight down.
func (m *measures) place(out *[]item, n *node, cx, y float64, d int, at string) {
	b := nodeBox(n, cx, y)
	*out = append(*out, item{at, part{Box: b}}) //nolint:exhaustruct_v5 // one kind of part
	if len(n.Children) == 0 {
		return
	}
	bottom := y + boxRect(b, "").h
	cy := y + m.heights[d] + treeGapY
	turn := y + m.heights[d] + treeGapY/2
	off := -m.childrenSpan(n) / 2 // the left of the next child's span, from cx
	for i := range n.Children {
		c := &n.Children[i]
		s := m.spans[c]
		ccx := cx + off + s/2
		// Compared as drawn: a child right below, but for a float's error, goes straight down.
		straight := num(off+s/2) == "0"
		if straight {
			ccx = cx
		}
		from := end{Point: &[2]float64{cx, bottom}} //nolint:exhaustruct_v5 // a point
		to := end{Point: &[2]float64{ccx, cy}}      //nolint:exhaustruct_v5 // a point
		who := "edge " + nodeWho(n) + "→" + nodeWho(c)

		e := &edge{From: from, To: to, who: who} //nolint:exhaustruct_v5 // the rest as left out
		if !straight {
			e.Via = [][2]float64{{cx, turn}, {ccx, turn}}
		}
		cat := at + "/children/" + strconv.Itoa(i)
		*out = append(*out, item{cat, part{Edge: e}}) //nolint:exhaustruct_v5 // one kind of part
		m.place(out, c, ccx, cy, d+1, cat)
		off += s + treeGapX
	}
}
