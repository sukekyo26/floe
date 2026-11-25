package diagram

import (
	"strconv"

	"floe/internal/usererr"
)

// item is a part to lay out, at its JSON pointer in the spec; a tree's nodes and edges are items of their own.
type item struct {
	at string
	p  part
}

// treeGapAcross is the room between the spans of siblings, treeGapAlong between the largest node of a depth and the
// next depth.
const treeGapAcross, treeGapAlong = 24.0, 40.0

// expand is the parts with each tree opened into a box for each node and an edge from each node to each child,
// at the node's JSON pointer, so that they are named, laid out and checked as boxes and edges written by hand are, each
// graph's ends moved by settle once all of it is placed.
// It reads the nodes read into their types, and walks no value along the schema. Its error is floe's failure.
// LIMIT: a subtree's span is not shared with its neighbours' (a wide row of leaves widens every span above it); pack
// them if wide trees waste room. A depth is as long as its longest node, so a leaf of a long name moves the next depth
// of a tree growing right away from every node; leave leaves out of a depth's length if such trees come out too wide.
func expand(parts []part, way graphWay) ([]item, usererr.List, error) {
	var problems usererr.List
	graphs := map[int]*graphOut{}
	passed := 0
	for i, p := range parts {
		if p.Graph != nil {
			o, found := readGraph(p.Graph, pointer(i)+"/graph", way)
			problems = append(problems, found...)
			graphs[i] = o
			passed += o.passed
		}
	}
	// Over the limit, no graph is laid out: the limit is the diagram's, so that no one graph is told to change.
	// LIMIT: an edge written by hand to a box or group of a graph not laid out (over this limit, or one graph past
	// maxNodes) is told naming nothing too; the graphs' mistakes come first.
	if passed > maxPassed {
		problems = append(problems, usererr.NewIn("", "#", "diagram_graph_long", passed, maxPassed))
		graphs = nil
	}
	out := make([]item, 0, len(parts))
	for i, p := range parts {
		switch {
		case p.Tree != nil:
			openTree(&out, p.Tree, pointer(i)+"/tree")
		case p.Graph != nil:
			if o := graphs[i]; o != nil && !o.skip {
				its, told, err := o.items()
				if err != nil {
					return nil, nil, err
				}
				problems = append(problems, told...)
				out = append(out, its...)
			}
		default:
			out = append(out, item{pointer(i), p})
		}
	}
	return out, problems, nil
}

// growth is the way nodes grow from a root: along an axis (0 is x, 1 is y), forward or, for sign -1, backward, with
// siblings across it. near is the side of a node its edge comes to, far the side its children's leave.
type growth struct {
	along     int
	sign      float64
	near, far string
	beside    [2]string
}

func growsDown() growth  { return growth{1, 1, "top", "bottom", [2]string{"left", "right"}} }
func growsRight() growth { return growth{0, 1, "left", "right", [2]string{"top", "bottom"}} }
func growsLeft() growth  { return growth{0, -1, "right", "left", [2]string{"top", "bottom"}} }

// point is the point at along and across as drawn.
func (g growth) point(along, across float64) [2]float64 {
	if g.along == 1 {
		return [2]float64{across, along}
	}
	return [2]float64{along, across}
}

// box is n's box, its near side at start and its middle across at mid.
func (g growth) box(n *node, start, mid float64) *box {
	if g.along == 1 {
		return nodeBox(n, mid, start)
	}
	r := nodeRect(n)
	return nodeBox(n, start+g.sign*r.w/2, mid-r.h/2)
}

// hands are the sides of a node left to edges written by hand: those beside, the near side if near, the far if far.
func (g growth) hands(near, far bool) []string {
	sides := []string{g.beside[0], g.beside[1]}
	if near {
		sides = append(sides, g.near)
	}
	if far {
		sides = append(sides, g.far)
	}
	return sides
}

// measures are the sizes of the nodes growing one way from a root, measured once from the leaves up: how much room each
// node takes across with its subtree, and the length along of the longest node at each depth, so that a depth stands in
// one row or column.
type measures struct {
	growth

	spans map[*node]float64
	long  []float64
}

// measured measures kids, growing the way g from root.
func measured(g growth, root *node, kids []node) *measures {
	m := &measures{growth: g, spans: map[*node]float64{}, long: []float64{nodeRect(root).size(g.along)}}
	for i := range kids {
		m.measure(&kids[i], 1)
	}
	return m
}

func nodeBox(n *node, cx, y float64) *box {
	return &box{
		Name: n.Name, X: cx, Y: y, Origin: "centre", W: 0, H: 0, Rows: n.Rows, Tone: n.Tone, Shape: n.Shape, graph: "",
		handSides: nil,
	}
}

func nodeRect(n *node) rect {
	return boxRect(nodeBox(n, 0, 0), "")
}

// measure keeps n's span, its box or its children side by side, the wider, and the longest node of its depth.
func (m *measures) measure(n *node, d int) {
	r := nodeRect(n)
	if len(m.long) <= d {
		m.long = append(m.long, 0)
	}
	m.long[d] = max(m.long[d], r.size(m.along))
	for i := range n.Children {
		m.measure(&n.Children[i], d+1)
	}
	m.spans[n] = max(r.size(1-m.along), m.span(n.Children))
}

// span is how much room kids take across, side by side.
func (m *measures) span(kids []node) float64 {
	if len(kids) == 0 {
		return 0
	}
	sum := treeGapAcross * float64(len(kids)-1)
	for i := range kids {
		sum += m.spans[&kids[i]]
	}
	return sum
}

// beyond is how far past the root the nodes reach along.
func (m *measures) beyond() float64 {
	sum := 0.0
	for _, long := range m.long[1:] {
		sum += treeGapAlong + long
	}
	return sum
}

// nodeWho is what node box b is called, as its box is (calling).
// LIMIT: two nodes of one text and no name read alike in a mistake that names both; name them if it confuses.
func nodeWho(b *box) string {
	text, class := firstRow(b)
	call, _ := calling(b.Name, text, class, boxRect(b, ""))
	return call
}

// openTree adds t's boxes and edges: growing down, the root's centre at x and its top at y; growing right, the top left
// of all of it there, the root in the middle of the tallest of its box and its two sides.
func openTree(out *[]item, t *tree, at string) {
	m := measured(growsDown(), &t.Root, t.Root.Children)
	start, mid := t.Y, t.X
	var left *measures
	if t.Direction == "right" {
		m = measured(growsRight(), &t.Root, t.Root.Children)
		left = measured(growsLeft(), &t.Root, t.Left)
		tallest := max(nodeRect(&t.Root).h, m.span(t.Root.Children), left.span(t.Left))
		start, mid = t.X+left.beyond(), t.Y+tallest/2
	}
	b := m.box(&t.Root, start, mid)
	b.handSides = m.hands(len(t.Left) == 0, len(t.Root.Children) == 0)
	*out = append(*out, item{at + "/root", part{Box: b}}) //nolint:exhaustruct_v5 // one kind of part
	m.grow(out, b, t.Root.Children, start, mid, 0, at+"/root/children")
	if left != nil {
		left.grow(out, b, t.Left, start+boxRect(b, "").w, mid, 0, at+"/left")
	}
}

// grow adds kids, the children at depth d+1 of the node whose box is parent, its near side at start and its middle
// across at mid: side by side, centred on it. Each edge leaves the middle of the parent's far side for the middle of
// the child's near side, turning across at one place for the depth, halfway between its longest node and the next
// depth. A child straight ahead is joined straight.
func (m *measures) grow(out *[]item, parent *box, kids []node, start, mid float64, d int, at string) {
	if len(kids) == 0 {
		return
	}
	who := nodeWho(parent)
	far := start + m.sign*boxRect(parent, "").size(m.along)
	next := start + m.sign*m.long[d] + m.sign*treeGapAlong
	turn := start + m.sign*m.long[d] + m.sign*treeGapAlong/2
	off := -m.span(kids) / 2 // the start of the next child's span, from mid
	for i := range kids {
		c := &kids[i]
		s := m.spans[c]
		cmid := mid + off + s/2
		// Compared as drawn: a child straight ahead, but for a float's error, is joined straight.
		straight := num(off+s/2) == "0"
		if straight {
			cmid = mid
		}
		b := m.box(c, next, cmid)
		b.handSides = m.hands(false, len(c.Children) == 0)
		from, to := m.point(far, mid), m.point(next, cmid)
		e := &edge{ //nolint:exhaustruct_v5 // the rest as left out
			From: end{Point: &from}, To: end{Point: &to}, who: edgeWho(who, nodeWho(b)), //nolint:exhaustruct_v5 // points
		}
		if !straight {
			e.Via = viaAt(m.point(turn, mid), m.point(turn, cmid))
		}
		cat := at + "/" + strconv.Itoa(i)
		*out = append(*out, item{cat, part{Edge: e}}, item{cat, part{Box: b}}) //nolint:exhaustruct_v5 // one kind of part
		m.grow(out, b, c.Children, next, cmid, d+1, cat+"/children")
		off += s + treeGapAcross
	}
}
