package diagram

import "strconv"

// item is a part to lay out, at its JSON pointer in the spec; a tree's nodes and edges are items of their own.
// who is an edge's words in mistakes and inspect, when they are not made from its ends.
type item struct {
	at  string
	p   part
	who string
}

// treeGapX is the room between the spans of siblings, treeGapY between the tallest node of a depth and the next.
const treeGapX, treeGapY = 24.0, 40.0

// expand is the parts with each tree opened into a box for each node and an edge from each node to each child,
// at the node's JSON pointer, so that they are named, laid out and checked as boxes and edges written by hand are.
// It reads the nodes read into their types, and walks no value along the schema.
// LIMIT: trees grow down alone, and a subtree's span is not shared with its neighbours' (a wide row of leaves
// widens every span above it); pack them if wide trees waste room.
func expand(parts []part) []item {
	out := make([]item, 0, len(parts))
	for i, p := range parts {
		if p.Tree == nil {
			out = append(out, item{pointer(i), p, ""})
			continue
		}
		var heights []float64
		depths(&p.Tree.Root, 0, &heights)
		place(&out, &p.Tree.Root, p.Tree.X, p.Tree.Y, 0, heights, pointer(i)+"/tree/root")
	}
	return out
}

func nodeBox(n *node, cx, y float64) *box {
	return &box{Name: n.Name, X: cx, Y: y, Origin: "centre", W: 0, H: 0, Rows: n.Rows, Tone: n.Tone, Shape: n.Shape}
}

// depths keeps in hs the height of the tallest node at each depth, so that a depth stands in one row.
func depths(n *node, d int, hs *[]float64) {
	if len(*hs) <= d {
		*hs = append(*hs, 0)
	}
	(*hs)[d] = max((*hs)[d], boxRect(nodeBox(n, 0, 0), "").h)
	for i := range n.Children {
		depths(&n.Children[i], d+1, hs)
	}
}

// treeSpan is how wide n and its subtree stand: its box, or its children's spans side by side, the wider.
func treeSpan(n *node) float64 {
	sum := 0.0
	for i := range n.Children {
		sum += treeSpan(&n.Children[i])
	}
	if len(n.Children) > 0 {
		sum += treeGapX * float64(len(n.Children)-1)
	}
	return max(boxRect(nodeBox(n, 0, 0), "").w, sum)
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
func place(out *[]item, n *node, cx, y float64, d int, hs []float64, at string) {
	b := nodeBox(n, cx, y)
	*out = append(*out, item{at, part{Box: b}, ""}) //nolint:exhaustruct_v5 // one kind of part
	if len(n.Children) == 0 {
		return
	}
	bottom := y + boxRect(b, "").h
	cy := y + hs[d] + treeGapY
	turn := y + hs[d] + treeGapY/2
	total := treeGapX * float64(len(n.Children)-1)
	for i := range n.Children {
		total += treeSpan(&n.Children[i])
	}
	x := cx - total/2
	for i := range n.Children {
		c := &n.Children[i]
		s := treeSpan(c)
		ccx := x + s/2
		from := end{Point: &[2]float64{cx, bottom}} //nolint:exhaustruct_v5 // a point
		to := end{Point: &[2]float64{ccx, cy}}      //nolint:exhaustruct_v5 // a point

		e := &edge{From: from, To: to} //nolint:exhaustruct_v5 // the rest as left out
		if ccx != cx {
			e.Via = [][2]float64{{cx, turn}, {ccx, turn}}
		}
		cat := at + "/children/" + strconv.Itoa(i)
		who := "edge " + nodeWho(n) + "→" + nodeWho(c)
		*out = append(*out, item{cat, part{Edge: e}, who}) //nolint:exhaustruct_v5 // one kind of part
		place(out, c, ccx, cy, d+1, hs, cat)
		x += s + treeGapX
	}
}
