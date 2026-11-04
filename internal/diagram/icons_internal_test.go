package diagram

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// anyIcon finds every icon.
type anyIcon struct{}

func (anyIcon) Lookup(string) ([]byte, error) { //nolint:unparam // as Icons wants
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), nil
}

// iconGraph is a graph of icons, edges as in graphOf.
func iconGraph(names []string, edges ...string) *graph {
	g := graphOf(names, edges...)
	for i, nd := range g.Nodes {
		g.Nodes[i] = graphNode{Icon: iconNode(nd.Box)} //nolint:exhaustruct_v5 // one kind of node
	}
	return g
}

func iconNode(b *graphBox) *icon {
	//nolint:exhaustruct_v5 // an icon of a graph
	return &icon{Name: b.Name, Icon: "aws/x", Label: b.Rows[0].Text}
}

// iconize makes every other box of g an icon, and its groups' and its first lane's frames take an icon.
func iconize(g *graph) {
	var walk func(nodes []graphNode)
	walk = func(nodes []graphNode) {
		for i := range nodes {
			if b := nodes[i].Box; b != nil && i%2 == 0 {
				nodes[i] = graphNode{Icon: iconNode(b)} //nolint:exhaustruct_v5 // one kind of node
			}
			if c := nodes[i].Group; c != nil {
				c.Icon = "aws/y"
				walk(c.Nodes)
			}
		}
	}
	walk(g.Nodes)
	for i := range g.Lanes {
		if i == 0 {
			g.Lanes[i].Icon = "aws/y"
		}
		walk(g.Lanes[i].Nodes)
	}
}

// names reads the two names of an edge's path.
func pathEnds(p path) (from, to string) {
	from, to, _ = strings.Cut(strings.TrimPrefix(p.what, "edge "), "→")
	return from, to
}

// sidePlace is where the end pt stands on icon t: its side, and how far along it. An end on no side of it is "".
func sidePlace(t target, pt [2]float64) (string, float64) {
	r, x, y := t.r, pt[0], pt[1]
	switch {
	case y == r.y && x > r.x && x < r.x+r.w:
		return "top", x - r.x
	case (y == r.y+r.h || y == r.y+r.h+t.under) && x > r.x && x < r.x+r.w:
		return "bottom", x - r.x
	case x == r.x && y > r.y && y < r.y+r.h:
		return "left", y - r.y
	case x == r.x+r.w && y > r.y && y < r.y+r.h:
		return "right", y - r.y
	}
	return "", 0
}

// iconFault is what is wrong with g laid out as l, or "": a mistake told, an end at an icon off the icon, the ends on a
// side not parting it equally, a run at an icon too short for an arrow, or a label not a column beside a run of its edge.
func iconFault(g *graph, l *layout) string {
	if l.failure != nil {
		return l.failure.Error()
	}
	if len(l.problems) > 0 {
		return l.problems.Error()
	}
	type place struct{ name, side string }
	on := map[place][]float64{}
	for _, p := range l.paths {
		from, to := pathEnds(p)
		first, last := p.segs[0], p.segs[len(p.segs)-1]
		for j, e := range [2]struct {
			name string
			pt   [2]float64
			run  seg
		}{{from, [2]float64{first[0], first[1]}, first}, {to, [2]float64{last[2], last[3]}, last}} {
			t, ok := l.names[e.name]
			if !ok || t.kind != "icon" {
				continue
			}
			side, along := sidePlace(t, e.pt)
			if side == "" {
				return fmt.Sprintf("end %d of %s at %v is off icon %s at %v", j, p.what, e.pt, e.name, t.r)
			}
			if math.Abs(e.run[0]-e.run[2])+math.Abs(e.run[1]-e.run[3]) < iconArrow {
				return fmt.Sprintf("end %d of %s runs %v, too short for an arrow", j, p.what, e.run)
			}
			on[place{e.name, side}] = append(on[place{e.name, side}], along)
		}
	}
	for at, ends := range on {
		slices.Sort(ends)
		for i, v := range ends {
			if want := 48 * float64(i+1) / float64(len(ends)+1); math.Abs(v-want) > 1e-6 {
				return fmt.Sprintf("the ends on the %s of icon %s stand at %v", at.side, at.name, ends)
			}
		}
	}
	return cmpOr(runsFault(g, l), labelsBeside(g, l))
}

// runsFault is the first run of an edge of g at an icon not straight or of no length, or an end at a box off its
// outline, or "".
func runsFault(g *graph, l *layout) string {
	shapes := map[string]string{}
	for _, b := range graphBoxes(g) {
		shapes[b.Name] = b.Shape
	}
	for _, p := range l.paths {
		for _, s := range p.segs {
			if s[0] != s[2] && s[1] != s[3] || math.Abs(s[0]-s[2])+math.Abs(s[1]-s[3]) < 1e-6 {
				return fmt.Sprintf("%s runs %v", p.what, s)
			}
		}
		from, to := pathEnds(p)
		first, last := p.segs[0], p.segs[len(p.segs)-1]
		for _, e := range [2]struct {
			name string
			pt   [2]float64
			next [2]float64
		}{{from, [2]float64{first[0], first[1]}, [2]float64{first[2], first[3]}}, {to, [2]float64{last[2], last[3]}, [2]float64{last[0], last[1]}}} {
			shape, isBox := shapes[e.name]
			t, named := l.names[e.name]
			if !isBox || !named || t.kind != "box" {
				continue
			}
			side, along := "left", e.pt[1]
			switch {
			case e.pt[0] == e.next[0] && e.next[1] > e.pt[1]:
				side, along = "bottom", e.pt[0]
			case e.pt[0] == e.next[0]:
				side, along = "top", e.pt[0]
			case e.next[0] > e.pt[0]:
				side = "right"
			}
			want := outlineOf(t.r.w, t.r.h, shape).on(t.r, side, along)
			if math.Abs(want[0]-e.pt[0])+math.Abs(want[1]-e.pt[1]) > 1e-6 {
				return fmt.Sprintf("%s ends at %v, off the outline of %s at %v", p.what, e.pt, e.name, want)
			}
		}
	}
	return ""
}

// labelsBeside is the first label of an edge of g, no loop's, not a column beside a run of it, or "".
func labelsBeside(g *graph, l *layout) string {
	for k, e := range g.Edges {
		if e.Label == nil || e.From == e.To {
			continue
		}
		if !slices.ContainsFunc(l.labels, func(r rect) bool {
			return (r.what == `label "`+e.Label["en"]+`"` || r.what == `label "`+e.Label["ja"]+`"`) &&
				slices.ContainsFunc(l.paths[k].segs, func(s seg) bool { return beside(r, s) })
		}) {
			return "the label of " + l.paths[k].what + " is not a column beside a run of it"
		}
	}
	return ""
}

// reachFault is the first icon of g moved past what it may, or "".
func reachFault(g *graph) string {
	o, _ := readGraph(g, "#/parts/0/graph") //nolint:errcheck // its mistakes are iconFault's
	if _, _, err := o.items(); err != nil { //nolint:errcheck // its mistakes are iconFault's
		return ""
	}
	axis := 0
	if g.Direction == "right" {
		axis = 1
	}
	for i, n := range o.boxes {
		ic, r, at := n.box.icon, o.drawn[i].cell, o.drawn[i].icon
		if ic == nil {
			continue
		}
		stands := iconWidest(ic)
		if axis == 1 {
			stands = iconSize(ic) + iconUnder(ic)
		}
		moved := at.lo(axis) + at.size(axis)/2 - (r.lo(axis) + r.size(axis)/2)
		if axis == 1 {
			moved = at.y + (iconSize(ic)+iconUnder(ic))/2 - (r.y + r.h/2)
		}
		if math.Abs(moved) > (r.size(axis)-stands)/2-iconArrow+1e-6 {
			return fmt.Sprintf("icon %s moved %v in %v", ic.Name, moved, r)
		}
	}
	return ""
}

// touchesOutside counts the pairs of edges of g laid out as l touching outside every icon's rectangle.
func touchesOutside(g *graph, l *layout) int {
	o, _ := readGraph(g, "#/parts/0/graph") //nolint:errcheck // its mistakes are iconFault's
	if _, _, err := o.items(); err != nil { //nolint:errcheck // its mistakes are iconFault's
		return 0
	}
	inside := func(p [2]float64) bool {
		for i, n := range o.boxes {
			if c := o.drawn[i].cell; n.box.icon != nil && p[0] >= c.x && p[0] <= c.x+c.w && p[1] >= c.y && p[1] <= c.y+c.h {
				return true
			}
		}
		return false
	}
	n := 0
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			if touched(a, b, inside) {
				n++
			}
		}
	}
	return n
}

// touched is whether a and b touch at a point inside does not hold of.
func touched(a, b path, inside func([2]float64) bool) bool {
	for _, s := range a.segs {
		for _, q := range b.segs {
			if !runsTouch(s, q) {
				continue
			}
			p := [2]float64{max(min(s[0], s[2]), min(q[0], q[2])), max(min(s[1], s[3]), min(q[1], q[3]))}
			if !inside(p) {
				return true
			}
		}
	}
	return false
}

// Random graphs with icons, down and right, lay out with no mistake, every end on its icon at the part of its side it
// belongs to, a run an arrow can end, and labels beside their runs; almost no edges touch outside the icons' rectangles. The
// graphs of TestGraphRandom hold for graphs of boxes alone that none touch.
func TestGraphIconsRandom(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		direction string
		seed      [2]uint64
		graphs    int
		every     bool
		lanes     bool
		level     bool
		touching  int
	}{
		{"", [2]uint64{31, 2}, 800, false, false, false, touchesPlain},
		{"right", [2]uint64{33, 4}, 800, false, false, false, touchesPlain},
		{"", [2]uint64{35, 6}, 150, true, false, false, touchesPlain},
		{"right", [2]uint64{37, 8}, 150, true, false, false, touchesPlain},
		{"", [2]uint64{39, 10}, 300, false, true, false, touchesPlain},
		{"right", [2]uint64{41, 12}, 300, false, true, false, touchesPlain},
		{"", [2]uint64{43, 14}, 150, false, false, true, touchesPlain},
		{"right", [2]uint64{45, 16}, 150, false, false, true, touchesPlain},
	} {
		t.Run(fmt.Sprintf("%s-%d", cmpOr(c.direction, "down"), c.seed[0]), func(t *testing.T) {
			t.Parallel()

			rnd := rand.New(rand.NewPCG(c.seed[0], c.seed[1])) //nolint:gosec // G404: graphs to try, no secret
			touching := 0
			for i := range c.graphs {
				g := randomGraph(rnd, c.direction)
				if c.every {
					for k := range g.Edges {
						g.Edges[k].Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(10)), "ja": strings.Repeat("字", rnd.IntN(16))}
					}
				}
				addLoops(rnd, g, c.every)
				addGroups(rnd, g)
				if c.lanes {
					inLanes(rnd, g)
				}
				iconize(g)
				if c.level {
					if addLevels(rnd, g); levelTold(g) {
						continue
					}
				}
				l := laid(g)
				if fault := cmpOr(iconFault(g, l), reachFault(g)); fault != "" {
					t.Fatalf("seed %v, graph %d: %s", c.seed, i, fault)
				}
				touching += touchesOutside(g, l)
			}
			if touching > c.touching {
				t.Errorf("seed %v: %d pairs of edges touch outside the icons' rectangles, more than %d", c.seed, touching, c.touching)
			}
		})
	}
}

// touchesPlain is how many pairs of edges of the graphs of a case of TestGraphIconsRandom may touch outside the icons'
// rectangles: clear keeps moved runs off other edges, compared as they then stand, and later moves may meet them.
// Within a rectangle they may touch (gathered).
const touchesPlain = 5

// The n edges at a side end at the points that part it into n+1 equal parts, in the order of what they reach, and the
// icons of a rank keep their tops level. One edge runs straight down the icons' middles.
func TestGraphIconParts(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 2, 3, 5, 8} {
		names := []string{"hub"}
		var edges []string
		for i := range n {
			names = append(names, fmt.Sprintf("n%d", i))
			edges = append(edges, fmt.Sprintf("hub>n%d", i), fmt.Sprintf("n%d>sink", i))
		}
		g := iconGraph(append(names, "sink"), edges...)
		l := laid(g)
		if fault := iconFault(g, l); fault != "" {
			t.Fatalf("%d edges: %s", n, fault)
		}
		hub := l.names["hub"]
		for i := range n {
			p := l.paths[2*i]
			if got, want := p.segs[0][0]-hub.r.x, 48*float64(i+1)/float64(n+1); math.Abs(got-want) > 1e-6 {
				t.Errorf("%d edges: %s leaves %v along the side, want %v", n, p.what, got, want)
			}
			if a, b := l.names["n0"].r.y, l.names[fmt.Sprintf("n%d", i)].r.y; a != b {
				t.Errorf("%d edges: the icons of a rank stand at %v and %v", n, a, b)
			}
		}
		if n != 1 {
			continue
		}
		for _, p := range l.paths {
			for _, s := range p.segs {
				if s[0] != hub.r.x+24 || s[2] != hub.r.x+24 {
					t.Errorf("%s runs %v, want it straight down the middle at %v", p.what, s, hub.r.x+24)
				}
			}
		}
	}
}

// The ends of the sides by what stands there: laid out right, the left and right sides of the icon; past a short
// label's width, the icon's bottom; under the label, the label's; a loop's two ends at two parts of its side; with no
// label, the icon's bottom; two icons' level edge straight between the middles of their sides.
func TestGraphIconSides(t *testing.T) {
	t.Parallel()

	g := iconGraph([]string{"a", "b", "c"}, "a>b", "a>c")
	g.Direction = "right"
	l := laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	if side, _ := sidePlace(l.names["a"], [2]float64{l.paths[0].segs[0][0], l.paths[0].segs[0][1]}); side != "right" {
		t.Errorf("right: a's edge leaves its %s side", side)
	}

	names := []string{"a", "b", "c", "d", "e", "f"}
	g = iconGraph(names, "a>b", "a>c", "a>d", "a>e", "a>f")
	g.Nodes[0].Icon.Label = Text{"en": "S3"}
	l = laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	a := l.names["a"]
	half := labelWidth(Text{"en": "S3"}, false) / 2
	for _, p := range l.paths {
		x, y := p.segs[0][0], p.segs[0][1]
		want := a.r.y + a.r.h + a.under
		if math.Abs(x-(a.r.x+24)) > half {
			want = a.r.y + a.r.h
		}
		if y != want {
			t.Errorf("short label: %s leaves at %v,%v, want y %v", p.what, x, y, want)
		}
	}

	g = iconGraph([]string{"a", "b"}, "a>b", "a>a")
	g.Nodes[0].Icon.Label = nil
	l = laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	a = l.names["a"]
	if y := l.paths[0].segs[0][1]; y != a.r.y+a.r.h {
		t.Errorf("no label: the edge leaves at y %v, want the icon's bottom %v", y, a.r.y+a.r.h)
	}
	loop := l.paths[1].segs
	if first, last := loop[0][1], loop[len(loop)-1][3]; first == last {
		t.Errorf("the loop leaves and comes back at y %v, want two parts of its side", first)
	}

	g = iconGraph([]string{"a", "b"}, "a>b")
	g.Edges[0].Level = true
	l = laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	mid := l.names["a"].r.y + 24
	for _, s := range l.paths[0].segs {
		if s[1] != mid || s[3] != mid {
			t.Errorf("level: runs %v, want it straight at y %v", l.paths[0].segs, mid)
		}
	}
}

// A frame's icon and heading stand within the room headArea keeps for them, and are measured as drawn. Its icon is the
// graph's own when an edge written by hand meets it.
func TestGraphFrameHead(t *testing.T) {
	t.Parallel()

	for _, direction := range []string{"", "right"} {
		for _, label := range []Text{{"en": "A subnet of a long name"}, nil} {
			//nolint:exhaustruct_v5 // a group with an icon
			frame := &graphGroup{Name: "f", Label: label, Icon: "aws/y", Nodes: []graphNode{{Box: &graphBox{Name: "a", Rows: []row{{Text: Text{"en": "a"}, Style: ""}}, Tone: "", Shape: ""}}}}
			g := &graph{X: 20, Y: 20, Direction: direction, Nodes: []graphNode{{Group: frame}}, Edges: nil} //nolint:exhaustruct_v5 // a group alone
			l := laid(g)
			if l.failure != nil || len(l.problems) > 0 {
				t.Fatal(l.failure, l.problems)
			}
			head := headRect(l.names["f"].r, headArea(frame))
			for _, r := range slices.Concat(l.heads, l.boxes[:1]) {
				if r.x < head.x-1e-6 || r.y < head.y-1e-6 || r.x+r.w > head.x+head.w+1e-6 || r.y+r.h > head.y+head.h+1e-6 {
					t.Errorf("%q: %s at %v,%v %vx%v stands past the heading's room %v", direction, r.what, r.x, r.y, r.w, r.h, head)
				}
			}
			if r := l.boxes[0]; !strings.HasPrefix(r.what, "the icon of") || r.graph == "" {
				t.Errorf("%q: the frame's icon is %q of graph %q", direction, r.what, r.graph)
			}
		}
	}
}

// A graph's x and y are the top left of what it draws, whatever stands round its icons unseen.
func TestGraphIconsTopLeft(t *testing.T) {
	t.Parallel()

	for _, direction := range []string{"", "right"} {
		g := iconGraph([]string{"a", "b"}, "a>b")
		g.Direction, g.X, g.Y = direction, 16, 16
		l := laid(g)
		if fault := iconFault(g, l); fault != "" {
			t.Fatal(fault)
		}
		left, top := math.Inf(1), math.Inf(1)
		for _, r := range slices.Concat(l.boxes, l.labels) {
			left, top = min(left, r.x), min(top, r.y)
		}
		if left != 16 || top != 16 {
			t.Errorf("%q: it starts at %v,%v, want 16,16", direction, left, top)
		}
	}
}

// Laid out down, the icons of a rank stand level, labelled or not, and a level edge between icons, or between an icon
// and a box, runs straight.
func TestGraphIconsLevel(t *testing.T) {
	t.Parallel()

	g := iconGraph([]string{"a", "b", "c"}, "a>b", "b>c")
	g.Nodes[1].Icon.Label = nil
	g.Nodes[2] = graphNode{Box: &graphBox{Name: "c", Rows: []row{{Text: Text{"en": "c"}, Style: ""}}, Tone: "", Shape: "", icon: nil}} //nolint:exhaustruct_v5 // one kind of node
	g.Edges[0].Level, g.Edges[1].Level = true, true
	l := laid(g)
	if l.failure != nil || len(l.problems) > 0 {
		t.Fatal(l.failure, l.problems)
	}
	a, b := l.names["a"].r, l.names["b"].r
	if a.y != b.y {
		t.Errorf("the icons stand at y %v and %v", a.y, b.y)
	}
	for _, p := range l.paths {
		if len(p.segs) != 1 {
			t.Errorf("%s runs %v, want it straight", p.what, p.segs)
		}
	}
}

// What is left to an edge written by hand runs from the icon's side, not its rectangle's.
func TestGraphIconsHands(t *testing.T) {
	t.Parallel()

	g := iconGraph([]string{"a"})
	g.Nodes[0].Icon.Label = Text{"en": "a label wider than its icon"}
	o, _ := readGraph(g, "#/parts/0/graph") //nolint:errcheck // none
	if _, _, err := o.items(); err != nil { //nolint:errcheck // none but this
		t.Fatal(err)
	}
	d, err := o.draw(o.groups[0], false, [2]float64{})
	if err != nil {
		t.Fatal(err)
	}
	at := d.iconAt(0)
	for _, s := range d.handsOf(0, false) {
		if s[1] != at.y+at.h/2 || s[3] != at.y+at.h/2 {
			t.Errorf("the run %v does not stand at the icon's middle %v", s, at.y+at.h/2)
		}
		if s[0] != at.x && s[2] != at.x && s[0] != at.x+at.w && s[2] != at.x+at.w {
			t.Errorf("the run %v does not reach the icon at %v", s, at)
		}
	}
}

// turnsOf is how many times p turns.
func turnsOf(p path) int {
	n := 0
	for i := 1; i < len(p.segs); i++ {
		if isDown(p.segs[i]) != isDown(p.segs[i-1]) {
			n++
		}
	}
	return n
}

// Where it can, an edge at an icon turns no more than its way asks: a run held off by another that moves later moves
// after it; a loop's runs move, so that it turns twice; a labelled straight edge short of a point turns twice, on one
// point and by the other's rectangle.
func TestGraphIconTurns(t *testing.T) {
	t.Parallel()

	names, edges := make([]string, 2, 10), make([]string, 0, 16)
	names[0], names[1] = "hub", "sink"
	for i := range 8 {
		names = append(names, fmt.Sprintf("n%d", i))
		edges = append(edges, fmt.Sprintf("hub>n%d", i), fmt.Sprintf("n%d>sink", i))
	}
	g := iconGraph(names, edges...)
	l := laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	for _, p := range l.paths {
		if n := turnsOf(p); n > 2 {
			t.Errorf("eight edges: %s turns %d times: %v", p.what, n, p.segs)
		}
	}

	g = iconGraph([]string{"a", "b"}, "a>b", "a>a")
	l = laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	if n := turnsOf(l.paths[1]); n != 2 {
		t.Errorf("the loop turns %d times, want 2: %v", n, l.paths[1].segs)
	}

	g = iconGraph([]string{"a", "b", "c", "d"}, "a>b", "b>c", "c>d")
	g.Nodes[2].Icon.Label = Text{"en": "a label of some width"}
	g.Edges[2].Label = Text{"en": "words"}
	l = laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	for _, p := range l.paths {
		if n := turnsOf(p); n > 2 {
			t.Errorf("in a row: %s turns %d times: %v", p.what, n, p.segs)
		}
	}
}
