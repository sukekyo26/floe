package diagram

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/yamlpos"
)

// graphOf is a graph of boxes named names, with edges written "a>b".
func graphOf(names []string, edges ...string) *graph {
	g := &graph{X: 20, Y: 20, Direction: "", Nodes: nil, Edges: nil}
	for _, n := range names {
		g.Nodes = append(g.Nodes, graphNode{Box: &graphBox{Name: n, Rows: []row{{Text: Text{"en": n}, Style: ""}}, Tone: "", Shape: ""}})
	}
	for _, e := range edges {
		from, to, _ := strings.Cut(e, ">")
		g.Edges = append(g.Edges, graphEdge{From: from, To: to, Dashed: false, Bold: false, Tone: "", Start: nil, End: nil})
	}
	return g
}

// laid is g laid out and checked, as the only part of a diagram.
func laid(g *graph) *layout {
	l := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: []part{{Graph: g}}, Example: ""}, //nolint:exhaustruct_v5 // one kind of part
		Options{Lang: "en", Icons: NoIcons{Err: nil}, Seq: 1})
	l.check()
	return l
}

// grouped is g read and laid out, its groups kept, as items lays it out.
func grouped(g *graph) []*group {
	o, _ := readGraph(g, "#/parts/0/graph") //nolint:errcheck // its mistakes are graphFault's
	_, _ = o.items() //nolint:errcheck // its failures are laid's
	return o.groups
}

// touch reports whether two edges share a run or touch anywhere but where they cross, and counts their crossings.
func touch(a, b path) (bool, int) {
	n := 0
	for _, s := range a.segs {
		for _, q := range b.segs {
			sv, qv := s[0] == s[2], q[0] == q[2]
			switch sv {
			case qv:
				// Both down, or both across: in one line and meeting.
				i := 0
				if sv {
					i = 1
				}
				if s[1-i] == q[1-i] && max(min(s[i], s[i+2]), min(q[i], q[i+2])) <= min(max(s[i], s[i+2]), max(q[i], q[i+2])) {
					return true, n
				}
			default:
				v, h := s, q
				if !sv {
					v, h = q, s
				}
				x, y := v[0], h[1]
				inV := y >= min(v[1], v[3]) && y <= max(v[1], v[3])
				inH := x >= min(h[0], h[2]) && x <= max(h[0], h[2])
				if !inV || !inH {
					continue
				}
				if y == v[1] || y == v[3] || x == h[0] || x == h[2] {
					return true, n
				}
				n++
			}
		}
	}
	return false, n
}

// graphFault is what is wrong with g laid out, or "": a mistake told, two of its edges sharing a run or touching,
// an end off its box's outline, or more crossings drawn than its ranks' order has.
func graphFault(g *graph) string {
	l := laid(g)
	if l.failure != nil {
		return l.failure.Error()
	}
	if len(l.problems) > 0 {
		return l.problems.Error()
	}
	drawn := 0
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			bad, n := touch(a, b)
			if bad {
				return a.what + " touches " + b.what
			}
			drawn += n
		}
	}
	shape := map[string]string{}
	for _, n := range g.Nodes {
		shape[n.Box.Name] = n.Box.Shape
	}
	for k, p := range l.paths {
		e := g.Edges[k]
		ends := [2][2]float64{{p.segs[0][0], p.segs[0][1]}, {p.segs[len(p.segs)-1][2], p.segs[len(p.segs)-1][3]}}
		for j, name := range []string{e.From, e.To} {
			r := l.names[name].r
			r.form = outlineOf(r.w, r.h, shape[name])
			if d := depth(r, ends[j][0], ends[j][1]); math.Abs(d) > 1e-9 {
				return fmt.Sprintf("%s ends %v from the outline of %s", p.what, d, name)
			}
		}
	}
	ordered := 0
	groups := grouped(g)
	least := math.Inf(1)
	for _, n := range groups[0].nodes {
		least = min(least, n.x)
	}
	if least < 0 || least >= column {
		return fmt.Sprintf("the first group starts at %v, not within a column right of 0", least)
	}
	for _, gr := range groups {
		ordered += gr.crossings()
		if fault := groupFault(gr); fault != "" {
			return fault
		}
	}
	if drawn > ordered {
		return fmt.Sprintf("%d crossings drawn, over the %d of its ranks' order", drawn, ordered)
	}
	if again := laid(g); again.inspect() != l.inspect() {
		return "laid out twice, it differs"
	}
	return ""
}

// groupFault is what is wrong with a group as wired, or "": a box too narrow for its links, a point off the grid
// (left at x ≡ 0, entered at x ≡ 6 or straight), two nodes of a rank too close (boxes nodeGap from boxes and points,
// points a column from points), or a cycle in a channel's track constraints.
func groupFault(gr *group) string {
	for _, n := range gr.nodes {
		if n.box != nil && max(len(n.ins), len(n.outs)) > 0 && n.w < float64(12*max(len(n.ins), len(n.outs))+36) {
			return fmt.Sprintf("box %s %v wide for %d links", n.box.Name, n.w, max(len(n.ins), len(n.outs)))
		}
	}
	for _, l := range gr.links {
		if math.Mod(l.exit, column) != 0 || !l.straight && math.Mod(l.entry, column) != column/2 || l.straight != (l.entry == l.exit) {
			return fmt.Sprintf("a link off the grid: exit %v, entry %v, straight %v", l.exit, l.entry, l.straight)
		}
	}
	// A point stands at its link's column, a column past what stands before it, or nodeGap past boxes between;
	// only one pushed off it is entered bent.
	for _, rank := range gr.ranks {
		last := -1
		for k, v := range rank {
			n := gr.nodes[v]
			if n.box != nil {
				continue
			}
			e := gr.links[n.ins[0]].exit
			room := math.Inf(-1)
			if last >= 0 {
				o := gr.nodes[rank[last]]
				at, gap := o.x+o.w, column
				for _, b := range rank[last+1 : k] {
					at, gap = onGrid(at+nodeGap, 6, math.Ceil)+gr.nodes[b].w, nodeGap
				}
				room = onGrid(at+gap, 6, math.Ceil)
			}
			// A point's column is its x, and it leaves w right of it.
			switch {
			case e >= room && (n.x != e || n.w != 0):
				return fmt.Sprintf("a point off its link's column %v with room: column %v, leaving %v on", e, n.x, n.w)
			case e < room && (n.w != column/2 || math.Mod(n.x, column) != column/2):
				return fmt.Sprintf("a point pushed off its link's column %v not entered bent: column %v", e, n.x)
			}
			last = k
		}
	}
	for r, rank := range gr.ranks {
		for k := 1; k < len(rank); k++ {
			a, b := gr.nodes[rank[k-1]], gr.nodes[rank[k]]
			want := nodeGap
			if a.box == nil && b.box == nil {
				want = column
			}
			if b.x-(a.x+a.w) < want {
				return fmt.Sprintf("rank %d: nodes %v apart, want %v", r, b.x-(a.x+a.w), want)
			}
		}
		if r+1 == len(gr.ranks) {
			continue
		}
		var bent []int
		for _, u := range rank {
			for _, l := range gr.nodes[u].outs {
				if !gr.links[l].straight {
					bent = append(bent, l)
				}
			}
		}
		above := gr.above(bent)
		done := map[int]bool{}
		for changed := true; changed; {
			changed = false
			for _, l := range bent {
				if done[l] {
					continue
				}
				if !slices.ContainsFunc(above[l], func(a int) bool { return !done[a] }) {
					done[l], changed = true, true
				}
			}
		}
		if len(done) != len(bent) {
			return fmt.Sprintf("channel %d: a cycle in the track constraints", r)
		}
	}
	return ""
}

// shrunk is g with boxes and edges taken off while it still fails, for a failing graph to keep in testdata/graph.
func shrunk(g *graph) *graph {
	for changed := true; changed; {
		changed = false
		for i := len(g.Edges) - 1; i >= 0; i-- {
			try := *g
			try.Edges = slices.Delete(slices.Clone(g.Edges), i, i+1)
			if graphFault(&try) != "" {
				g, changed = &try, true
			}
		}
		for i := len(g.Nodes) - 1; i >= 0 && len(g.Nodes) > 1; i-- {
			name := g.Nodes[i].Box.Name
			try := *g
			try.Nodes = slices.Delete(slices.Clone(g.Nodes), i, i+1)
			try.Edges = slices.DeleteFunc(slices.Clone(g.Edges), func(e graphEdge) bool { return e.From == name || e.To == name })
			if graphFault(&try) != "" {
				g, changed = &try, true
			}
		}
	}
	return g
}

// randomGraph is a graph as dagro's trials drew them: 2 to 31 boxes of four shapes and words of some length, 1 to
// 62 edges between two of them at random, cycles and a second edge between two boxes among them.
func randomGraph(rnd *rand.Rand, direction string) *graph {
	n := 2 + rnd.IntN(30)
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("n%d", i)
	}
	g := graphOf(names)
	g.Direction = direction
	shapes := []string{"", "pill", "hex", "state"}
	for _, nd := range g.Nodes {
		nd.Box.Shape = shapes[rnd.IntN(len(shapes))]
		nd.Box.Rows[0].Text["en"] = strings.Repeat("w", 1+rnd.IntN(16))
	}
	for range 1 + rnd.IntN(62) {
		a, b := rnd.IntN(n), rnd.IntN(n)
		if a == b {
			continue
		}
		g.Edges = append(g.Edges, graphEdge{From: names[a], To: names[b], Dashed: false, Bold: false, Tone: "", Start: nil, End: nil, Label: nil})
	}
	if rnd.IntN(3) == 0 {
		for k := range g.Edges {
			if rnd.IntN(3) == 0 {
				g.Edges[k].Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(12))}
			}
		}
	}
	return g
}

// Random graphs, down and right, lay out with no mistake, no two edges sharing a run or touching, every end on its
// box's outline, no more crossings drawn than their ranks' order has, and the same layout twice.
func TestGraphRandom(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		direction string
		seed      [2]uint64
	}{{"", [2]uint64{1, 2}}, {"right", [2]uint64{3, 4}}} {
		rnd := rand.New(rand.NewPCG(c.seed[0], c.seed[1])) //nolint:gosec // G404: graphs to try, no secret
		for i := range 3000 {
			g := randomGraph(rnd, c.direction)
			if fault := graphFault(g); fault != "" {
				b, _ := json.Marshal(map[string]any{"title": "kept", "parts": []any{map[string]any{"graph": shrunk(g)}}}) //nolint:errcheck,errchkjson // plain values
				t.Fatalf("seed %v, graph %d: %s; shrunk, to keep in testdata/graph:\n%s", c.seed, i, fault, b)
			}
		}
	}
}

// writtenOrder is a ranked graph whose written order crosses once, in a pair at the left of two ranks, and nowhere
// else: boxes of each rank linked from the rank above in one order, a fan of 20 leaves of one median, boxes with no
// link below, and edges two ranks long from right end to right end. It returns the graph and the boxes of each
// rank as they should stand: as written, but h1 and h2 swapped.
func writtenOrder(rnd *rand.Rand) (*graph, [][]string) {
	count := 3 + rnd.IntN(6)
	gadget := 1 + rnd.IntN(count-2)
	fan := rnd.IntN(count - 1)
	ranks := make([][]string, 0, count)
	idx := 0
	name := func() string {
		idx++
		return fmt.Sprintf("n%03d", 999-idx) // names in the order opposite to the written one
	}
	for r := range count {
		var rank []string
		if r == gadget {
			rank = append(rank, "g1", "g2")
		}
		if r == gadget+1 {
			rank = append(rank, "h1", "h2")
		}
		for range 1 + rnd.IntN(4) {
			rank = append(rank, name())
		}
		ranks = append(ranks, rank)
	}
	var edges []string
	for r := 1; r < count; r++ {
		up, low := ranks[r-1], ranks[r]
		start := 0
		if r == gadget {
			edges = append(edges, up[0]+">g1", up[0]+">g2")
			start = 2
		}
		if r == gadget+1 {
			edges = append(edges, "g1>h2", "g2>h1")
			start = 2
		}
		if r == gadget+1 || r == gadget+2 {
			up = up[2:] // the pair links only to itself
		}
		// Each box below takes a run of the boxes above, the runs in one order; a box above may be skipped (no link
		// below), but not in rank 0, which would then stand alone.
		cur := 0
		for j, b := range low[start:] {
			lo := cur
			hi := min(len(up)-1, lo+rnd.IntN(2))
			if j == len(low[start:])-1 {
				hi = len(up) - 1
			}
			for p := lo; p <= hi; p++ {
				edges = append(edges, up[p]+">"+b)
			}
			cur = hi
			if r > 1 && cur+1 < len(up) && rnd.IntN(3) == 0 {
				cur++
			}
		}
	}
	// A fan of 20 leaves from the right end of a rank, at the right end of the next.
	parent := ranks[fan][len(ranks[fan])-1]
	for range 20 {
		leaf := name()
		ranks[fan+1] = append(ranks[fan+1], leaf)
		edges = append(edges, parent+">"+leaf)
	}
	for r := 0; r+2 < count; r += 2 {
		edges = append(edges, ranks[r][len(ranks[r])-1]+">"+ranks[r+2][len(ranks[r+2])-1])
	}
	var names []string
	for _, rank := range ranks {
		names = append(names, rank...)
	}
	want := make([][]string, len(ranks))
	for r, rank := range ranks {
		want[r] = slices.Clone(rank)
		if r == gadget+1 {
			want[r][0], want[r][1] = "h2", "h1"
		}
	}
	return graphOf(names, edges...), want
}

// Ranked graphs that cross once at their left lay out as written but for that pair, with nothing crossing: ties
// of one median keep their written order, a box with no link below keeps its place, and the order is sorted.
func TestGraphWrittenOrder(t *testing.T) {
	t.Parallel()

	rnd := rand.New(rand.NewPCG(7, 8)) //nolint:gosec // G404: graphs to try, no secret
	for i := range 1000 {
		g, want := writtenOrder(rnd)
		groups := grouped(g)
		if len(groups) != 1 {
			t.Fatalf("graph %d: %d groups, want 1", i, len(groups))
		}
		gr := groups[0]
		got := make([][]string, 0, len(gr.ranks))
		for _, rank := range gr.ranks {
			var names []string
			for _, n := range rank {
				if b := gr.nodes[n].box; b != nil {
					names = append(names, b.Name)
				}
			}
			slices.SortStableFunc(names, func(a, b string) int {
				return int(gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == a })].x -
					gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == b })].x)
			})
			got = append(got, names)
		}
		if !slices.EqualFunc(got, want, slices.Equal) {
			t.Fatalf("graph %d: ranks %v, want %v", i, got, want)
		}
		if fault := graphFault(g); fault != "" {
			t.Fatalf("graph %d: %s", i, fault)
		}
	}
}

// order stops at maxTrips on ranks that swing between two orders, keeping the first of fewest crossings; at a
// trip that changes nothing, crossings left; and once nothing crosses.
func TestGraphTrips(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		g     *graph
		trips int
		want  [][]string
	}{
		{
			"swinging", graphOf([]string{"0a", "0b", "1a", "1b", "1c", "1d", "2a", "2b", "2c"},
				"0a>1b", "0a>1c", "0a>1d", "0b>1a", "0b>1b", "1a>2a", "1a>2b", "1a>2c", "1c>2c", "1d>2a", "1d>2b"),
			maxTrips,
			[][]string{{"0b", "0a"}, {"1c", "1a", "1b", "1d"}, {"2c", "2a", "2b"}},
		},
		{"K2,2, crossing as written", graphOf([]string{"a", "b", "x", "y"}, "a>x", "a>y", "b>x", "b>y"), 1, [][]string{{"a", "b"}, {"x", "y"}}},
		{"one swap", graphOf([]string{"r", "a", "b", "x", "y"}, "r>a", "r>b", "a>y", "b>x"), 1, [][]string{{"r"}, {"a", "b"}, {"y", "x"}}},
		{"none crossing", graphOf([]string{"r", "a", "b", "x", "y"}, "r>a", "r>b", "a>x", "b>y"), 0, [][]string{{"r"}, {"a", "b"}, {"x", "y"}}},
		{
			"three reversed", graphOf([]string{"r", "a", "b", "c", "x", "y", "z"}, "r>a", "r>b", "r>c", "a>z", "b>y", "c>x"), 1,
			[][]string{{"r"}, {"a", "b", "c"}, {"z", "y", "x"}},
		},
	} {
		o, _ := readGraph(c.g, "#/parts/0/graph") //nolint:errcheck // every name is a box's
		gr := o.groups[0]
		trips, _ := gr.place(false)
		var got [][]string
		for _, rank := range gr.ranks {
			var names []string
			for _, n := range rank {
				names = append(names, gr.nodes[n].box.Name)
			}
			got = append(got, names)
		}
		if trips != c.trips || !slices.EqualFunc(got, c.want, slices.Equal) {
			t.Errorf("%s: %d trips, ranks %v; want %d, %v", c.name, trips, got, c.trips, c.want)
		}
	}
}

// swinging is a graph whose boxes' lefts keep moving, pulled down and up in turn.
func swinging() *graph {
	g := graphOf([]string{"n0", "n1", "n2", "n3", "n4"},
		"n4>n1", "n2>n0", "n0>n1", "n3>n0", "n0>n2", "n4>n3", "n0>n3", "n0>n1", "n1>n4", "n1>n4")
	for i, b := range []struct {
		shape string
		n     int
	}{{"", 9}, {"", 11}, {"pill", 9}, {"hex", 16}, {"pill", 2}} {
		g.Nodes[i].Box.Shape, g.Nodes[i].Box.Rows[0].Text["en"] = b.shape, strings.Repeat("w", b.n)
	}
	return g
}

// pull stops at maxPulls on lefts that keep moving, and before it once nothing moves.
func TestGraphPulls(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		g     *graph
		pulls int
	}{
		{"swinging", swinging(), maxPulls},
		{"a diamond", graphOf([]string{"a", "b", "c", "d"}, "a>b", "a>c", "b>d", "c>d"), 3},
	} {
		o, _ := readGraph(c.g, "#/parts/0/graph") //nolint:errcheck // every name is a box's
		if _, pulls := o.groups[0].place(false); pulls != c.pulls {
			t.Errorf("%s: %d pulls, want %d", c.name, pulls, c.pulls)
		}
	}
}

// channel is a random channel as the prototype drew them: 1 to 6 boxes above and below, 1 to 12 links between them,
// a second between two boxes among them, and half the time in one order above and below.
func channel(rnd *rand.Rand) (*group, bool) {
	gr := &group{
		nodes: nil, links: nil, ranks: [][]int{nil, nil}, chains: map[int][]int{}, reversed: map[int]bool{},
		tracks: nil, top: nil, bottom: nil,
	}
	up, low := 1+rnd.IntN(6), 1+rnd.IntN(6)
	for i := range up + low {
		r := 0
		if i >= up {
			r = 1
		}
		gr.ranks[r] = append(gr.ranks[r], i)
		gr.nodes = append(gr.nodes, gnode{
			box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: i, rank: r, w: 0, h: 29,
			x: 0, y: 0, ins: nil, outs: nil, written: i,
		})
	}
	ordered := rnd.IntN(2) == 0
	pairs := make([][2]int, 1+rnd.IntN(12))
	for i := range pairs {
		pairs[i] = [2]int{rnd.IntN(up), rnd.IntN(low)}
	}
	if ordered {
		slices.SortStableFunc(pairs, func(a, b [2]int) int { return a[0] - b[0] })
		for i := 1; i < len(pairs); i++ {
			pairs[i][1] = max(pairs[i][1], pairs[i-1][1])
		}
	}
	for k, p := range pairs {
		gr.link(p[0], up+p[1], k)
	}
	for _, rank := range gr.ranks {
		x := 6 * float64(rnd.IntN(3))
		for _, i := range rank {
			n := &gr.nodes[i]
			n.w = float64(48 + rnd.IntN(200))
			if k := max(len(n.ins), len(n.outs)); k > 0 {
				n.w = max(n.w, onGrid(float64(12*k+36), 8, math.Ceil))
			}
			n.x = x
			x = onGrid(x+n.w+nodeGap+float64(rnd.IntN(40)), 6, math.Ceil)
		}
	}
	return gr, ordered
}

// In random channels, no two links share a run or touch, entries into a box stand a column apart, every link on a
// track below the links it must run under, no two links crossing more than their ports' order does, and none at
// all where the boxes keep one order above and below.
func TestWireChannel(t *testing.T) {
	t.Parallel()

	rnd := rand.New(rand.NewPCG(9, 10)) //nolint:gosec // G404: channels to try, no secret
	for run := range 200000 {
		gr, ordered := channel(rnd)
		gr.wire()
		paths := make([]path, len(gr.links))
		for k := range gr.links {
			pts := gr.points([]int{k}, false)
			for i := 1; i < len(pts); i++ {
				paths[k].segs = append(paths[k].segs, seg{pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]})
			}
		}
		crossed := 0
		for i := range gr.links {
			for j := i + 1; j < len(gr.links); j++ {
				a, b := gr.links[i], gr.links[j]
				bad, n := touch(paths[i], paths[j])
				if bad {
					t.Fatalf("run %d: links %d and %d touch: %+v %+v", run, i, j, a, b)
				}
				if a.to == b.to && math.Abs(a.entry-b.entry) < column {
					t.Fatalf("run %d: entries %v and %v less than a column apart", run, a.entry, b.entry)
				}
				order := 0
				if (a.exit-b.exit)*(a.entry-b.entry) < 0 {
					order = 1
				}
				if n > order {
					t.Fatalf("run %d: links %d and %d cross %d times, their ports' order %d", run, i, j, n, order)
				}
				crossed += n
			}
		}
		var bent []int
		for k, l := range gr.links {
			if !l.straight {
				bent = append(bent, k)
			}
		}
		for l, ups := range gr.above(bent) {
			for _, a := range ups {
				if gr.links[a].track >= gr.links[l].track {
					t.Fatalf("run %d: link %d on track %d, not above link %d on track %d", run, a, gr.links[a].track, l, gr.links[l].track)
				}
			}
		}
		if ordered && crossed > 0 {
			t.Fatalf("run %d: %d crossings in a channel of one order", run, crossed)
		}
	}
}

// Of two links whose runs overlap, the one whose exit is in the other's run goes above; two going opposite ways
// have no order; and saul's two cases: two links crossing the boxes' order share nothing and cross once, and two
// shifted right do not cross.
func TestWireCases(t *testing.T) {
	t.Parallel()

	links := func(pairs ...[2]float64) *group {
		gr := &group{
			nodes: []gnode{{box: nil, of: 0, rank: 0, w: 0, h: 0, x: 0, y: 0, ins: nil, outs: nil, written: 0}},
			links: nil, ranks: [][]int{{0}}, chains: map[int][]int{}, reversed: map[int]bool{}, tracks: nil, top: nil, bottom: nil,
		}
		for k, p := range pairs {
			gr.links = append(gr.links, glink{from: 0, to: 0, edge: k, exit: p[0], entry: p[1], straight: false, track: 0})
			gr.nodes[0].outs = append(gr.nodes[0].outs, k)
		}
		return gr
	}
	shifted := links([2]float64{0, 30}, [2]float64{12, 42})
	shifted.track(0)
	if a, b := shifted.links[0].track, shifted.links[1].track; b >= a {
		t.Errorf("0→30 and 12→42: tracks %d and %d, want the second above", a, b)
	}
	if got := links([2]float64{0, 42}, [2]float64{36, 6}).above([]int{0, 1}); len(got) != 0 {
		t.Errorf("0→42 and 36→6 going opposite ways: above %v, want none", got)
	}

	// a→d and b→c, the boxes' order crossed once.
	gr := &group{
		nodes: nil, links: nil, ranks: [][]int{{0, 1}, {2, 3}}, chains: map[int][]int{}, reversed: map[int]bool{},
		tracks: nil, top: nil, bottom: nil,
	}
	for i, x := range []float64{12, 114, 12, 114} {
		gr.nodes = append(gr.nodes, gnode{
			box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: i, rank: i / 2, w: 72, h: 29,
			x: x, y: 0, ins: nil, outs: nil, written: i,
		})
	}
	gr.link(0, 3, 0)
	gr.link(1, 2, 1)
	gr.wire()
	var ps [2]path
	for k := range 2 {
		pts := gr.points([]int{k}, false)
		for i := 1; i < len(pts); i++ {
			ps[k].segs = append(ps[k].segs, seg{pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]})
		}
	}
	if bad, n := touch(ps[0], ps[1]); bad || n != 1 {
		t.Errorf("a→d and b→c: touching %v, crossing %d times; want apart, once", bad, n)
	}
}

// A gap between straight entries without the columns for its links bends the straight entry closing it, on its
// right (else its left), and no other.
func TestWireBend(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name     string
		exits    []float64
		straight []bool
	}{
		// 24's gap on the left has no column for two: 18 is 6 from it. 24 bends; 204 stays.
		{"left gap", []float64{-60, -48, 24, 204}, []bool{false, false, false, true}},
		// 216's gap on the right, to the box's end, has none for two: 216 bends, the one closing it on the left.
		{"right gap", []float64{24, 216, 300, 312}, []bool{true, false, false, false}},
		{"room enough", []float64{-60, 120, 204}, []bool{false, true, true}},
	} {
		gr := &group{
			nodes: []gnode{{
				box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: 0, rank: 1, w: 240, h: 29, x: 0,
				y: 0, ins: nil, outs: nil, written: 0,
			}}, links: nil, ranks: nil, chains: map[int][]int{}, reversed: map[int]bool{},
			tracks: nil, top: nil, bottom: nil,
		}
		for k, x := range c.exits {
			gr.links = append(gr.links, glink{from: 0, to: 0, edge: k, exit: x, entry: 0, straight: false, track: 0})
			gr.nodes[0].ins = append(gr.nodes[0].ins, k)
		}
		gr.entries(0)
		got := make([]bool, len(gr.links))
		for k, l := range gr.links {
			got[k] = l.straight
		}
		if !slices.Equal(got, c.straight) {
			t.Errorf("%s: straight %v, want %v", c.name, got, c.straight)
		}
	}
}

// chainGraph is a chain of n boxes with long edges from its first box to its last, which pass n-2 ranks each.
func chainGraph(n, long int) *graph {
	names := make([]string, n)
	var edges []string
	for i := range n {
		names[i] = fmt.Sprintf("c%d", i)
		if i > 0 {
			edges = append(edges, names[i-1]+">"+names[i])
		}
	}
	for range long {
		edges = append(edges, names[0]+">"+names[n-1])
	}
	return graphOf(names, edges...)
}

// The work of laying out is held: the points edges pass ranks at are counted, order and pull stop at their caps,
// and a diagram whose graphs pass more than maxPassed ranks, all told, is refused at its root, with no graph laid
// out.
func TestGraphLimit(t *testing.T) {
	t.Parallel()

	rnd := rand.New(rand.NewPCG(5, 6)) //nolint:gosec // G404: a graph to try, no secret
	big := graphOf(nil)
	for i := range 200 {
		big.Nodes = append(big.Nodes, graphOf([]string{fmt.Sprintf("b%d", i)}).Nodes...)
	}
	// Each edge between boxes a few apart as written, either way: between any two, as dagro's trials drew them,
	// the edges pass 6,911 ranks, over maxPassed.
	for range 400 {
		a := rnd.IntN(200)
		b := min(199, a+1+rnd.IntN(6))
		if a == b {
			b = a - 1
		}
		if rnd.IntN(2) == 0 {
			a, b = b, a
		}
		big.Edges = append(big.Edges, graphOf(nil, fmt.Sprintf("b%d>b%d", a, b)).Edges...)
	}
	exact := chainGraph(42, 100) // 100 edges of 40 points
	for _, g := range []*graph{big, exact} {
		o, _ := readGraph(g, "#/parts/0/graph") //nolint:errcheck // every name is a box's
		if o.passed > maxPassed {
			t.Fatalf("%d boxes: %d points, over %d", len(g.Nodes), o.passed, maxPassed)
		}
		for _, gr := range o.groups {
			if trips, pulls := gr.place(false); trips > maxTrips || pulls > maxPulls {
				t.Errorf("%d boxes: %d trips and %d pulls, over %d and %d", len(g.Nodes), trips, pulls, maxTrips, maxPulls)
			}
		}
		if l := laid(g); l.failure != nil || len(l.problems) > 0 {
			t.Errorf("%d boxes: %v %v", len(g.Nodes), l.failure, l.problems)
		}
	}
	if o, _ := readGraph(exact, "#"); o.passed != maxPassed { //nolint:errcheck // every name is a box's
		t.Errorf("the chain passes %d ranks, want %d", o.passed, maxPassed)
	}

	// Counted before any point is made: graphs whose edges pass far more ranks than the limit make none, and are
	// refused all told.
	huge := chainGraph(200, 200)               // 200 edges of 198 points
	o, _ := readGraph(huge, "#/parts/0/graph") //nolint:errcheck // every name is a box's
	if gr := o.groups[0]; o.passed != 200*198 || len(gr.nodes) != 200 || len(gr.links) != 0 {
		t.Errorf("200 boxes: %d ranks passed, %d nodes, %d links; want %d, 200, none made", o.passed, len(gr.nodes), len(gr.links), 200*198)
	}
	hundred := make([]part, 100)
	for i := range hundred {
		hundred[i] = part{Graph: chainGraph(200, 200)} //nolint:exhaustruct_v5 // one kind of part
	}
	if items, found, err := expand(hundred); err != nil || len(items) != 0 || len(found) != 1 || fmt.Sprint(found[0].Args) != "[3960000 4000]" {
		t.Errorf("a hundred graphs: %d items, %v; want none, one diagram_graph_long", len(items), found)
	}

	over := chainGraph(42, 100)
	over.Edges = append(over.Edges, graphOf(nil, "c39>c41").Edges...)
	split := graphOf([]string{"a", "b", "c"}, "a>b", "b>c", "a>c")
	split.X = 2000
	for _, parts := range [][]part{{{Graph: over}}, {{Graph: exact}, {Graph: split}}} { //nolint:exhaustruct_v5 // one kind of part
		items, found, err := expand(parts)
		_ = err
		if len(items) != 0 || len(found) != 1 || found[0].Key != "diagram_graph_long" || found[0].At() != "#" ||
			fmt.Sprint(found[0].Args) != "[4001 4000]" {
			t.Errorf("%d graphs: %d items, %v; want none, diagram_graph_long at # of 4001", len(parts), len(items), found)
		}
	}
}

// The pairs of edges checkCrossings compares leave out two of one graph, and no other.
func TestCrossPairs(t *testing.T) {
	t.Parallel()

	bipartite := func(up, low int, x float64) *graph {
		names, edges := make([]string, 0, up+low), make([]string, 0, up*low)
		for i := range up {
			names = append(names, fmt.Sprintf("u%v-%d", x, i))
		}
		for j := range low {
			names = append(names, fmt.Sprintf("l%v-%d", x, j))
		}
		for i := range up {
			for j := range low {
				edges = append(edges, fmt.Sprintf("u%v-%d>l%v-%d", x, i, x, j))
			}
		}
		g := graphOf(names, edges...)
		g.X = x
		return g
	}
	hand := part{Edge: &edge{From: end{Name: "", Point: &[2]float64{0, 0}}, To: end{Name: "", Point: &[2]float64{0, 10}}}} //nolint:exhaustruct_v5 // the rest as left out
	for _, c := range []struct {
		parts []part
		want  int
	}{
		{[]part{{Graph: bipartite(20, 20, 0)}, hand}, 400},                                           //nolint:exhaustruct_v5 // one kind of part
		{[]part{{Graph: bipartite(10, 20, 0)}, {Graph: bipartite(10, 20, 5000)}, hand}, 40000 + 400}, //nolint:exhaustruct_v5 // one kind of part
	} {
		l := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: c.parts, Example: ""},
			Options{Lang: "en", Icons: NoIcons{Err: nil}, Seq: 1})
		n := 0
		for range l.crossPairs() {
			n++
		}
		if n != c.want {
			t.Errorf("%d parts: %d pairs, want %d", len(c.parts), n, c.want)
		}
	}
}

// A mistake that only things graphs laid out make is told at the first's graph, to move it: two graphs' boxes
// overlapping, an edge of one through a box of another, crossing an edge of another, and a box off the canvas. With
// a thing written by hand, it is told at that.
func TestGraphTold(t *testing.T) {
	t.Parallel()

	const ga, gb = "#/parts/0/graph", "#/parts/1/graph"
	box := func(x float64, graph string) rect {
		return rect{x: x, y: 0, w: 40, h: 40, what: "box at " + num(x), at: graph + "/nodes/0/box", form: outline{hex: 0, radius: 0}, graph: graph}
	}
	across := path{segs: []seg{{0, 20, 100, 20}}, what: "edge a→b", at: ga + "/edges/0", own: -1, crossing: false, graph: ga}
	down := path{segs: []seg{{50, -10, 50, 100}}, what: "edge c→d", at: gb + "/edges/0", own: -1, crossing: false, graph: gb}
	hand := down
	hand.at, hand.graph, hand.what = "#/parts/1", "", "edge [50,-10]→[50,100]"
	handBox := box(20, "")
	handBox.at = "#/parts/1"
	for _, c := range []struct {
		name    string
		l       *layout
		key, at string
	}{
		{"two graphs' boxes", &layout{boxes: []rect{box(0, ga), box(20, gb)}, w: 500, h: 500}, "diagram_graph_overlap", ga},                              //nolint:exhaustruct_v5 // what it checks
		{"a graph's box and one by hand", &layout{boxes: []rect{box(0, ga), handBox}, w: 500, h: 500}, "diagram_overlap", "#/parts/1"},                   //nolint:exhaustruct_v5 // what it checks
		{"an edge through another graph's box", &layout{boxes: []rect{box(30, gb)}, paths: []path{across}, w: 500, h: 500}, "diagram_graph_through", ga}, //nolint:exhaustruct_v5 // what it checks
		{"two graphs' edges", &layout{paths: []path{across, down}, w: 500, h: 500}, "diagram_graph_crossing", ga},                                        //nolint:exhaustruct_v5 // what it checks
		{"the other first", &layout{paths: []path{down, across}, w: 500, h: 500}, "diagram_graph_crossing", gb},                                          //nolint:exhaustruct_v5 // what it checks
		{"a graph's edge and one by hand", &layout{paths: []path{across, hand}, w: 500, h: 500}, "diagram_crossing", "#/parts/1"},                        //nolint:exhaustruct_v5 // what it checks
		{"a graph's box off the canvas", &layout{boxes: []rect{box(480, ga)}, w: 500, h: 500}, "diagram_graph_outside", ga},                              //nolint:exhaustruct_v5 // what it checks
	} {
		c.l.check()
		if len(c.l.problems) != 1 || c.l.problems[0].Key != c.key || c.l.problems[0].At() != c.at {
			t.Errorf("%s: %v, want %s at %s", c.name, c.l.problems, c.key, c.at)
		}
	}
}

// boxIn is the area of the box named name in l.
func boxIn(l *layout, name string) rect {
	return l.names[name].r
}

// A graph's shapes: a chain runs straight, an edge back up a cycle points the way it is written, groups stand side
// by side as written, the languages lay out alike, a box widens for its edges, boxes of a rank stand nodeGap apart,
// a leaf added keeps the other ranks' order, and right is down turned.
func TestGraphShapes(t *testing.T) {
	t.Parallel()

	chain := laid(graphOf([]string{"a", "b", "c", "d"}, "a>b", "b>c", "c>d"))
	for _, p := range chain.paths {
		if len(p.segs) != 1 {
			t.Errorf("chain: %s turns: %v", p.what, p.segs)
		}
	}

	// Edges that can run straight do: every edge of the kept flow, and K3,3's edges straight down; of the two edges
	// of fan a reader once saw bend, c→hub runs straight and x→a turns twice at most.
	for _, c := range []struct {
		file  string
		turns map[string]int // the most turns of the edges named; of every edge, none, when nil
	}{
		{"flow.yaml", nil},
		{"fan.yaml", map[string]int{"edge c→hub": 0, "edge x→a": 2}},
	} {
		src, err := os.ReadFile(filepath.Join("testdata", "graph", c.file))
		if err != nil {
			t.Fatal(err)
		}
		_, v, err := yamlpos.ParseMap(c.file, src)
		if err != nil {
			t.Fatal(err)
		}
		s, err := read(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range laid(s.Parts[0].Graph).paths {
			most, ok := c.turns[p.what]
			if c.turns != nil && !ok {
				continue
			}
			if len(p.segs)-1 > most {
				t.Errorf("%s: %s turns %d times, want %d at most: %v", c.file, p.what, len(p.segs)-1, most, p.segs)
			}
		}
	}
	// A box settle moves off its exit's column takes its exit again: six turns, not eight.
	moved := graphOf([]string{"n0", "n1", "n2"}, "n0>n1", "n2>n1", "n2>n0", "n1>n0", "n1>n0", "n2>n1")
	for i, b := range []struct {
		shape string
		n     int
	}{{"pill", 9}, {"state", 10}, {"state", 15}} {
		moved.Nodes[i].Box.Shape, moved.Nodes[i].Box.Rows[0].Text["en"] = b.shape, strings.Repeat("w", b.n)
	}
	movedTurns := 0
	for _, p := range laid(moved).paths {
		movedTurns += len(p.segs) - 1
	}
	if movedTurns > 6 {
		t.Errorf("a box moved by settle: %d turns, want 6 at most", movedTurns)
	}
	k33 := laid(graphOf([]string{"a", "b", "c", "x", "y", "z"},
		"a>x", "a>y", "a>z", "b>x", "b>y", "b>z", "c>x", "c>y", "c>z"))
	for _, k := range []int{0, 4, 8} {
		if p := k33.paths[k]; len(p.segs) != 1 {
			t.Errorf("K3,3: %s turns: %v", p.what, p.segs)
		}
	}

	cycle := laid(graphOf([]string{"a", "b", "c"}, "a>b", "b>c", "c>a"))
	back := cycle.paths[2].segs
	if a := boxIn(cycle, "a"); back[len(back)-1][3] != a.y+a.h {
		t.Errorf("cycle: c→a ends at %v, want at the bottom of a, %v", back[len(back)-1], a.y+a.h)
	}

	apart := laid(graphOf([]string{"a", "b", "c", "d"}, "a>b", "c>d"))
	if a, c := boxIn(apart, "a"), boxIn(apart, "c"); c.x < a.x+a.w+groupGap {
		t.Errorf("two groups: c at %v, not right of a (%v to %v)", c.x, a.x, a.x+a.w)
	}

	langs := graphOf([]string{"a", "b"}, "a>b")
	langs.Nodes[1].Box.Rows[0].Text = Text{"en": "Save", "ja": "データファイルと画像の写しを保存する"}
	en := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: []part{{Graph: langs}}, Example: ""}, //nolint:exhaustruct_v5 // one kind of part
		Options{Lang: "en", Icons: NoIcons{Err: nil}, Seq: 1})
	ja := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: []part{{Graph: langs}}, Example: ""}, //nolint:exhaustruct_v5 // one kind of part
		Options{Lang: "ja", Icons: NoIcons{Err: nil}, Seq: 1})
	if en.inspect() != ja.inspect() {
		t.Errorf("languages: en\n%s\nja\n%s", en.inspect(), ja.inspect())
	}

	// Eleven edges at one box: five in, three out, and three back up a cycle, turned to leave it too.
	hub := laid(graphOf([]string{"a", "b", "c", "d", "e", "hub", "x", "y", "z"},
		"a>hub", "b>hub", "c>hub", "d>hub", "e>hub", "hub>x", "hub>y", "hub>z", "x>hub", "y>hub", "z>hub"))
	if len(hub.problems) > 0 {
		t.Errorf("eleven edges: %v", hub.problems)
	}
	if h := boxIn(hub, "hub"); h.w < 12*6+36 {
		t.Errorf("eleven edges: the box %v wide, want %v or more for six edges out", h.w, 12*6+36)
	}

	beside := laid(graphOf([]string{"r", "a", "b", "c"}, "r>a", "r>b", "r>c"))
	for _, pair := range [][2]string{{"a", "b"}, {"b", "c"}} {
		if l, r := boxIn(beside, pair[0]), boxIn(beside, pair[1]); r.x-(l.x+l.w) < nodeGap {
			t.Errorf("beside: %s and %s %v apart, want %v or more", pair[0], pair[1], r.x-(l.x+l.w), nodeGap)
		}
	}

	// A leaf added keeps the order of the other ranks.
	order := func(g *graph) []string {
		var out []string
		for _, gr := range grouped(g) {
			for _, rank := range gr.ranks[:2] {
				names := []string{}
				for _, n := range rank {
					if b := gr.nodes[n].box; b != nil {
						names = append(names, b.Name)
					}
				}
				slices.SortFunc(names, func(a, b string) int {
					return int(gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == a })].x -
						gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == b })].x)
				})
				out = append(out, strings.Join(names, " "))
			}
		}
		return out
	}
	tree := graphOf([]string{"r", "a", "b", "c", "x", "y"}, "r>a", "r>b", "r>c", "a>x", "c>y")
	more := graphOf([]string{"r", "a", "b", "c", "x", "y", "w"}, "r>a", "r>b", "r>c", "a>x", "c>y", "b>w")
	if a, b := order(tree), order(more); !slices.Equal(a, b) {
		t.Errorf("a leaf added: ranks %v, then %v", a, b)
	}

	// Right runs the edges left to right, as down runs them top down.
	g := graphOf([]string{"a", "b", "c"}, "a>b", "a>c", "b>c")
	down := laid(g)
	turned := *g
	turned.Direction = "right"
	r := laid(&turned)
	for _, e := range g.Edges {
		if f, to := boxIn(down, e.From), boxIn(down, e.To); to.y < f.y+f.h {
			t.Errorf("down: %s→%s, %v not below %v", e.From, e.To, to, f)
		}
		if f, to := boxIn(r, e.From), boxIn(r, e.To); to.x < f.x+f.w {
			t.Errorf("right: %s→%s, %v not right of %v", e.From, e.To, to, f)
		}
	}
	if len(r.problems) > 0 {
		t.Errorf("right: %v", r.problems)
	}
}
