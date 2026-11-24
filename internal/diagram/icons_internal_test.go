package diagram

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"floe/internal/usererr"
	"floe/internal/yamlpos"
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

// edgesFault is "" when l keeps g's edges in order, path k drawing edge k, or what it keeps instead.
func edgesFault(g *graph, l *layout) string {
	if len(l.paths) != len(g.Edges) {
		return fmt.Sprintf("%d paths for %d edges", len(l.paths), len(g.Edges))
	}
	for k, p := range l.paths {
		if want := edgeAt("#/parts/0/graph", k); p.at != want {
			return fmt.Sprintf("path %d is %s at %s, not at %s", k, p.what, p.at, want)
		}
	}
	return ""
}

// shapesOf is the shape of each box of g by its name, "icon" for an icon's.
func shapesOf(g *graph) map[string]string {
	shapes := map[string]string{}
	for _, b := range graphBoxes(g) {
		shapes[b.Name] = b.Shape
	}
	return shapes
}

// frameEndFault is what is wrong with an end at pt on the frame of group c of lg, or "": off its sides, within inEnd of a
// corner, or by a heading of c or a group round it: within a nodeGap below it and short of a nodeGap past its right,
// or, laid out right, within a nodeGap right of it and short of a nodeGap below it.
func frameEndFault(lg laidGroups, c int, pt [2]float64, right bool) string {
	f := lg.frames[c]
	if !onFrameSide(f, pt, right) {
		return "off the frame"
	}
	along := 0
	if right {
		along = 1
	}
	if pt[along] < f.lo(along)+inEnd-eps || pt[along] > f.lo(along)+f.size(along)-inEnd+eps {
		return "within inEnd of a corner"
	}
	for p := c; p >= 0; p = lg.clusters[p].parent {
		gr := lg.clusters[p].g
		if lg.clusters[p].lane || !headed(gr) {
			continue
		}
		h := headRect(lg.frames[p], headArea(gr))
		if beside := pt[1-along] < h.lo(1-along)+h.size(1-along)+nodeGap-eps; beside && pt[along] < h.lo(along)+h.size(along)+nodeGap-eps {
			return fmt.Sprintf("by the heading of group %d %q at %v,%v %vx%v", p, gr.Name, h.x, h.y, h.w, h.h)
		}
	}
	return ""
}

// sidePlace is where the end pt stands on icon t: its side, and how far along it. An end on no side of it is "".
func sidePlace(t target, pt [2]float64) (string, float64) {
	r, x, y := t.r, pt[0], pt[1]
	switch {
	case y == r.y && x > r.x && x < r.x+r.w:
		return "top", x - r.x
	case y == r.y+r.h+underAt(t, x) && x > r.x && x < r.x+r.w:
		return "bottom", x - r.x
	case x == r.x && y > r.y && y < r.y+r.h:
		return "left", y - r.y
	case x == r.x+r.w && y > r.y && y < r.y+r.h:
		return "right", y - r.y
	}
	return "", 0
}

func underAt(t target, x float64) float64 {
	if math.Abs(x-(t.r.x+t.r.w/2)) <= t.labelW/2 {
		return t.under
	}
	return 0
}

// iconFault is what is wrong with g laid out as l, or "": a mistake told, a path not its edge's, an end at an icon not
// laid out as one or off it, the ends on a side not parting it equally, a run at an icon too short for an arrow, or a
// label not on a run of its edge.
func iconFault(g *graph, l *layout) string {
	if fault := laidOutFault(g, l); fault != "" {
		return fault
	}
	shapes := shapesOf(g)
	type place struct{ name, side string }
	on := map[place][]float64{}
	straight := map[place]bool{}
	for k, p := range l.paths {
		from, to := g.Edges[k].From, g.Edges[k].To
		first, last := p.segs[0], p.segs[len(p.segs)-1]
		for j, e := range [2]struct {
			name string
			pt   [2]float64
			run  seg
		}{{from, [2]float64{first[0], first[1]}, first}, {to, [2]float64{last[2], last[3]}, last}} {
			if shapes[e.name] != "icon" {
				continue
			}
			t := l.names[e.name]
			if t.kind != "icon" {
				return fmt.Sprintf("icon %s is laid out as %q", e.name, t.kind)
			}
			side, along := sidePlace(t, e.pt)
			if side == "" {
				return fmt.Sprintf("end %d of %s at %v is off icon %s at %v", j, p.what, e.pt, e.name, t.r)
			}
			if math.Abs(e.run[0]-e.run[2])+math.Abs(e.run[1]-e.run[3]) < arrowRun {
				return fmt.Sprintf("end %d of %s runs %v, too short for an arrow", j, p.what, e.run)
			}
			on[place{e.name, side}] = append(on[place{e.name, side}], along)
			straight[place{e.name, side}] = len(p.segs) == 1
		}
	}
	moves, fault := iconMoves(g, l)
	if fault != "" {
		return fault
	}
	for _, name := range slices.Sorted(maps.Keys(moves)) {
		if m := moves[name]; math.Abs(m.moved) > m.may+1e-6 {
			return fmt.Sprintf("icon %s moved %v, %v at most", name, m.moved, m.may)
		}
	}
	endsAt := map[string]int{}
	for at, ends := range on {
		endsAt[at.name] += len(ends)
	}
	for at, ends := range on {
		slices.Sort(ends)
		for i, v := range ends {
			size := l.names[at.name].r.size(sideAxis(at.side))
			want := size * float64(i+1) / float64(len(ends)+1)
			m := moves[at.name]
			// offPoint: an end alone on its side, straight, stands off its point only when its icon has other ends, which
			// keep it where it is, or has moved as far as it may towards the line.
			still := endsAt[at.name] > 1 || math.Abs(m.moved) > m.may-1e-6 && m.moved*(v-want) > 0
			if len(ends) == 1 && straight[at] && still && math.Abs(v-want) <= max(0, min(iconSpareMost, size/2-inEnd))+1e-6 {
				continue
			}
			if math.Abs(v-want) > 1e-6 {
				return fmt.Sprintf("the ends on the %s of icon %s stand at %v", at.side, at.name, ends)
			}
		}
	}
	return cmpOr(runsFault(g, l), labelsOn(g, l))
}

// runsFault is the first run of an edge of g at an icon not straight or of no length, an end at a box or a group not
// laid out as one, an end at a box off its outline, or an end at a group's frame where it should not stand
// (frameEndFault) or nearer than keepApart to another edge's end there, or "".
func runsFault(g *graph, l *layout) string {
	shapes := shapesOf(g)
	lg, fault := groupsOf(g, l)
	if fault != "" {
		return fault
	}
	type frameEnd struct {
		name string
		pt   [2]float64
		k    int
	}
	var framed []frameEnd
	for k, p := range l.paths {
		for _, s := range p.segs {
			if !near(s[0], s[2]) && !near(s[1], s[3]) || math.Abs(s[0]-s[2])+math.Abs(s[1]-s[3]) < 1e-6 {
				return fmt.Sprintf("%s runs %v", p.what, s)
			}
		}
		from, to := g.Edges[k].From, g.Edges[k].To
		first, last := p.segs[0], p.segs[len(p.segs)-1]
		for _, e := range [2]struct {
			name string
			pt   [2]float64
			next [2]float64
		}{{from, [2]float64{first[0], first[1]}, [2]float64{first[2], first[3]}}, {to, [2]float64{last[2], last[3]}, [2]float64{last[0], last[1]}}} {
			shape, isBox := shapes[e.name]
			if !isBox {
				c := lg.named(e.name)
				if c < 0 {
					return fmt.Sprintf("%s ends at %s, no box, icon or group of the graph", p.what, e.name)
				}
				r := lg.frames[c]
				if fault := frameEndFault(lg, c, e.pt, g.Direction == "right"); fault != "" {
					return fmt.Sprintf("%s ends at %v, %s of %s at %v", p.what, e.pt, fault, e.name, r)
				}
				for _, o := range framed {
					if o.name == e.name && o.k != k && math.Hypot(o.pt[0]-e.pt[0], o.pt[1]-e.pt[1]) < keepApart-eps {
						return fmt.Sprintf("%s ends at %v, by %s at %v on the frame of %s", p.what, e.pt, l.paths[o.k].what, o.pt, e.name)
					}
				}
				framed = append(framed, frameEnd{e.name, e.pt, k})
				continue
			}
			if shape == "icon" {
				continue
			}
			t := l.names[e.name]
			if t.kind != "box" {
				return fmt.Sprintf("box %s is laid out as %q", e.name, t.kind)
			}
			side, along := leaves(e.pt, e.next), e.pt[1]
			if side == "top" || side == "bottom" {
				along = e.pt[0]
			}
			want := outlineOf(t.r.w, t.r.h, shape).on(t.r, side, along)
			if math.Abs(want[0]-e.pt[0])+math.Abs(want[1]-e.pt[1]) > 1e-6 {
				return fmt.Sprintf("%s ends at %v, off the outline of %s at %v", p.what, e.pt, e.name, want)
			}
		}
	}
	return ""
}

// labelsOn is the first label of an edge of g, no loop's, not on a run of it, or "".
func labelsOn(g *graph, l *layout) string {
	for k, e := range g.Edges {
		if e.Label == nil || e.From == e.To {
			continue
		}
		if !slices.ContainsFunc(l.labels, func(r rect) bool {
			return (r.what == `label "`+e.Label["en"]+`"` || r.what == `label "`+e.Label["ja"]+`"`) &&
				slices.ContainsFunc(l.paths[k].segs, func(s seg) bool { return onRun(r, s) })
		}) {
			return "the label of " + l.paths[k].what + " is not on a run of it"
		}
	}
	return ""
}

// movedIcon is how far an icon stands from the middle of its rectangle across the ranks, and how far it may.
type movedIcon struct{ moved, may float64 }

// iconMoves is the move of each icon of g, laid out as l by laid, by its name, or what is wrong. It settles g's ends
// as layoutWay does: with offPoint, unless that leaves a mistake.
func iconMoves(g *graph, l *layout) (map[string]movedIcon, string) {
	settle := (*drawing).settleOff
	if off := laidWith(g, placing{(*layout).shareStraight, graphWay{settle, false}}); off.failure != nil || len(off.problems) > 0 {
		settle = (*drawing).settleEnds
	}
	o, _ := readGraph(g, "#/parts/0/graph", graphWay{settle, false}) //nolint:errcheck // the same as items tells
	_, told, err := o.items()
	if err = cmp.Or(err, told.Err()); err != nil {
		return nil, err.Error()
	}
	axis := 0
	if g.Direction == "right" {
		axis = 1
	}
	moves := map[string]movedIcon{}
	for i, n := range o.boxes {
		if n.box == nil || n.box.icon == nil {
			continue
		}
		ic, r, at := n.box.icon, o.drawn[i].cell, o.drawn[i].icon
		if t := l.names[n.box.Name].r; t.x != at.x || t.y != at.y {
			return nil, fmt.Sprintf("icon %s stands at %v,%v, laid out at %v,%v", n.box.Name, at.x, at.y, t.x, t.y)
		}
		stands := iconWidest(ic)
		moved := at.lo(axis) + at.size(axis)/2 - (r.lo(axis) + r.size(axis)/2)
		if axis == 1 {
			stands = iconSize(ic) + iconUnder(ic)
			moved = at.y + stands/2 - (r.y + r.h/2)
		}
		moves[n.box.Name] = movedIcon{moved, (r.size(axis)-stands)/2 - arrowRun}
	}
	return moves, ""
}

// iconCells is whether a point is within the rectangle of an icon of g as laid out, and false when g does not lay out.
func iconCells(g *graph) (func([2]float64) bool, bool) {
	o, _ := readGraph(g, "#/parts/0/graph", graphWay{(*drawing).settleEnds, false}) //nolint:errcheck // its mistakes are iconFault's
	if _, _, err := o.items(); err != nil {                                         //nolint:errcheck // its mistakes are iconFault's
		return nil, false
	}
	return func(p [2]float64) bool {
		for i, n := range o.boxes {
			if c := o.drawn[i].cell; n.box.icon != nil && p[0] >= c.x-eps && p[0] <= c.x+c.w+eps && p[1] >= c.y-eps && p[1] <= c.y+c.h+eps {
				return true
			}
		}
		return false
	}, true
}

// touchesOutside counts the pairs of edges of l touching where inside, the icons' rectangles, does not hold.
func touchesOutside(l *layout, inside func([2]float64) bool) int {
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
// belongs to, a run an arrow can end, and labels beside their runs; almost no edges touch outside the icons' rectangles,
// none share a stretch, and few cross within them. The
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
		through   int   // the most edges left through a frame that holds neither end (throughFault), measured + 2
		cells     int   // the most crossings within icons' rectangles (cellCrossings), measured + 15%, 2 at least
		only      []int // the graphs to lay out, when any: the rest only take their turn of the random numbers
	}{
		{"", [2]uint64{31, 2}, 800, false, false, false, touchesPlain, 3, 1730, nil},
		{"right", [2]uint64{33, 4}, 800, false, false, false, touchesPlain, 0, 1690, nil},
		{"", [2]uint64{35, 6}, 150, true, false, false, touchesPlain, 0, 550, nil},
		{"right", [2]uint64{37, 8}, 150, true, false, false, touchesPlain, 0, 500, nil},
		// 717 has a label where a run stands once its ends settle, 54 a run a rounding off straight.
		{"right", [2]uint64{777, 1517}, 720, true, false, false, touchesPlain, 0, 5, []int{54, 717}},
		// Each named graph has a labelled straight edge between icons that slideEnd would move onto one icon's point,
		// leaving its other end past its icon's side, and the turn back would cross a label (a loop's in 946 and 1596,
		// another edge's in 4474, 4239 and 1716; in 1596 on a line in from the side) or a box (5812).
		{"right", [2]uint64{241, 26}, 5813, true, false, false, touchesPlain, 0, 5, []int{946, 4474, 5812}},
		{"right", [2]uint64{241, 28}, 4240, true, false, true, touchesPlain, 0, 2, []int{4239}},
		{"right", [2]uint64{241, 30}, 1717, true, true, false, touchesPlain, 0, 3, []int{1716}},
		{"right", [2]uint64{241, 32}, 1597, true, true, true, touchesPlain, 0, 2, []int{1596}},
		{"", [2]uint64{39, 10}, 300, false, true, false, touchesPlain, 0, 300, nil},
		{"right", [2]uint64{41, 12}, 300, false, true, false, touchesPlain, 8, 460, nil},
		{"", [2]uint64{43, 14}, 150, false, false, true, touchesPlain, 0, 60, nil},
		{"right", [2]uint64{45, 16}, 150, false, false, true, touchesPlain, 0, 28, nil},
		// 77 has a level edge through the heading of a group of no lane.
		{"right", [2]uint64{102, 1}, 78, false, false, true, touchesPlain, 0, 2, []int{77}},
	} {
		t.Run(fmt.Sprintf("%s-%d-%d", cmpOr(c.direction, "down"), c.seed[0], c.seed[1]), func(t *testing.T) {
			t.Parallel()

			rnd := rand.New(rand.NewPCG(c.seed[0], c.seed[1])) //nolint:gosec // G404: graphs to try, no secret
			touching, left, shared, cells := 0, 0, 0, 0
			graphs := c.graphs
			if c.only == nil { // a graph named by only must be reached under race too
				graphs = tries(graphs)
			}
			for i := range graphs {
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
					addLevels(rnd, g)
				}
				if c.only != nil && !slices.Contains(c.only, i) {
					continue
				}
				if c.level && levelTold(g) {
					continue
				}
				l := laid(g)
				_, o, _ := groupedRound(g) //nolint:errcheck // iconFault tells the mistakes
				if fault := cmp.Or(iconFault(g, l), throughFault(g, l, o.stayed)); fault != "" {
					t.Fatalf("seed %v, graph %d: %s", c.seed, i, fault)
				}
				if inside, ok := iconCells(g); ok {
					touching += touchesOutside(l, inside)
					cells += cellCrossings(l, inside)
				}
				through, fault := runThrough(g, l)
				if fault != "" {
					t.Fatalf("seed %v, graph %d: %s", c.seed, i, fault)
				}
				left += len(through)
				shared += sharedStretches(l)
			}
			if shared > 0 {
				t.Errorf("seed %v: %d pairs of edges share a stretch", c.seed, shared)
			}
			if cells > c.cells {
				t.Errorf("seed %v: %d crossings within icons' rectangles, more than %d", c.seed, cells, c.cells)
			}
			if left > c.through {
				t.Errorf("seed %v: %d edges run through a frame that holds neither end, more than %d", c.seed, left, c.through)
			}
			if touching > c.touching {
				t.Errorf("seed %v: %d pairs of edges touch outside the icons' rectangles, more than %d", c.seed, touching, c.touching)
			}
		})
	}
}

// touchesPlain is how many pairs of edges of the graphs of a case of TestGraphIconsRandom may touch outside the icons'
// rectangles: clear keeps moved runs off other edges, compared as they then stand, and later moves may meet them.
// Within a rectangle they may cross where they turn by the icon (cellCrossings), and share no stretch.
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
	o, _ := readGraph(g, "#/parts/0/graph", graphWay{(*drawing).settleEnds, false}) //nolint:errcheck // none
	if _, _, err := o.items(); err != nil {                                         //nolint:errcheck // none but this
		t.Fatal(err)
	}
	d, _, err := o.draw(o.groups[0], false) //nolint:errcheck // none but this
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

// An end alone on its side stands off its point by what its icon's size leaves it, 12 at most.
func TestIconSpare(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ size, want float64 }{{16, 0}, {24, 0}, {32, 4}, {48, 12}, {96, 12}} {
		//nolint:exhaustruct_v5 // an icon, which is all iconSpare reads
		if got := iconSpare(&icon{Size: c.size}); got != c.want {
			t.Errorf("size %v: %v, want %v", c.size, got, c.want)
		}
	}
}

// labelledChain is three icons in a row, the labels' lengths those given, the first with two edges to the second.
func labelledChain(lengths ...int) *graph {
	g := iconGraph([]string{"a", "b", "c"}, "a>b", "b>c", "a>b")
	for i, n := range lengths {
		g.Nodes[i].Icon.Label = Text{"en": strings.Repeat("w", n)}
	}
	return g
}

// An icon moved as far as it may short of a straight edge's line leaves the edge straight, its end off its point by
// what the icon's size leaves it: none on one of 16, which stays bent.
func TestGraphIconOffPoint(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		size    float64
		lengths [3]int
		off     float64
	}{{16, [3]int{1, 1, 4}, 0}, {48, [3]int{1, 7, 9}, 1.2}, {96, [3]int{1, 14, 13}, 1.8}} {
		g := labelledChain(c.lengths[:]...)
		for i := range g.Nodes {
			g.Nodes[i].Icon.Size = c.size
		}
		l := laid(g)
		if fault := iconFault(g, l); fault != "" {
			t.Fatalf("size %v: %s", c.size, fault)
		}
		p := l.paths[1]
		if c.off == 0 {
			if len(p.segs) == 1 {
				t.Errorf("size %v: %s runs %v, want it bent", c.size, p.what, p.segs)
			}
			continue
		}
		if len(p.segs) != 1 {
			t.Errorf("size %v: %s runs %v, want one run", c.size, p.what, p.segs)
			continue
		}
		r := l.names["c"].r
		if off := math.Abs(p.segs[0][0] - (r.x + r.w/2)); math.Abs(off-c.off) > 1e-6 {
			t.Errorf("size %v: %s ends %v off the middle of c, want %v", c.size, p.what, off, c.off)
		}
	}
}

// No edge of a graph of icons turns more for the icons' ends standing off their points.
func TestGraphIconOffPointKeeps(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		direction string
		seed      [2]uint64
		every     bool
		plain     bool // the graphs alone: icons among boxes, no loop or group
	}{
		{"", [2]uint64{61, 1}, false, false},
		{"right", [2]uint64{63, 1}, false, false},
		{"", [2]uint64{65, 1}, true, false},
		{"right", [2]uint64{67, 1}, true, false},
		{"", [2]uint64{13, 0}, false, true}, // holds one where an icon with other ends moving bends them, when all of the graphs run
		{"right", [2]uint64{13, 5}, false, true},
	} {
		t.Run(fmt.Sprintf("%s-%d", cmpOr(c.direction, "down"), c.seed[0]), func(t *testing.T) {
			t.Parallel()
			rnd := rand.New(rand.NewPCG(c.seed[0], c.seed[1])) //nolint:gosec // G404: graphs to try, no secret
			opts := Options{Lang: "en", Icons: anyIcon{}, Seq: 1}
			more, fewer := 0, 0
			for range tries(400) {
				g := randomGraph(rnd, c.direction)
				if c.every {
					for k := range g.Edges {
						g.Edges[k].Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(10))}
					}
				}
				if !c.plain {
					addLoops(rnd, g, c.every)
					addGroups(rnd, g)
				}
				iconize(g)
				a, b := asToday(graphSpec(g), opts), laid(g)
				if a.failure != nil || len(a.problems) > 0 {
					continue
				}
				for k, p := range b.paths {
					switch n, was := len(p.segs), len(a.paths[k].segs); {
					case n > was:
						more++
						t.Logf("%s turns %d times, %d before: %v, %v", p.what, n-1, was-1, p.segs, a.paths[k].segs)
					case n < was:
						fewer++
					}
				}
			}
			t.Logf("OFFKEEPS %s more %d fewer %d", c.direction, more, fewer)
			if more > 0 {
				t.Errorf("%d edges turn more, %d fewer", more, fewer)
			}
		})
	}
}

// sharedStretches counts the pairs of edges of l with a stretch of one line in common.
func sharedStretches(l *layout) int {
	n := 0
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			if slices.ContainsFunc(a.segs, func(s seg) bool {
				return slices.ContainsFunc(b.segs, func(q seg) bool {
					along := runAlong(s)
					return along == runAlong(q) && near(s[1-along], q[1-along]) && overlap(s, q, along) > eps
				})
			}) {
				n++
			}
		}
	}
	return n
}

// cellCrossings counts the crossings of two edges of l where inside, the icons' rectangles, holds.
func cellCrossings(l *layout, inside func([2]float64) bool) int {
	n := 0
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			for _, s := range a.segs {
				for _, q := range b.segs {
					if x, y, ok := crossing(s, q); ok && inside([2]float64{x, y}) {
						n++
					}
				}
			}
		}
	}
	return n
}

// The edges at an icon's side that turn beside it turn apart, each arrow on a run of its length: of one icon to three
// boxes (S387), in and out at one side, two into one icon laid out right, and two into one icon laid out down.
func TestGraphIconBends(t *testing.T) {
	t.Parallel()

	inOut := `title: in and out
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - icon: { name: n0, icon: lucide/server, label: wwwwwwwww }
        - box: { name: n1, rows: [wwwwwwwwww] }
        - icon: { name: n2, icon: lucide/server, label: wwwwwwwwwwwwww }
        - box: { name: n3, rows: [wwwww], shape: pill }
        - icon: { name: n4, icon: lucide/server, label: wwwwwwwwwwww }
        - box: { name: n5, rows: [wwwwwwwwwwww], shape: pill }
        - icon: { name: n6, icon: lucide/server, label: wwwww }
        - box: { name: n7, rows: [ww] }
        - icon: { name: n8, icon: lucide/server, label: wwwwwwwwwwww }
      edges:
        - { from: n7, to: n3 }
        - { from: n5, to: n4 }
        - { from: n6, to: n4 }
        - { from: n4, to: n7 }
        - { from: n1, to: n3 }
        - { from: n0, to: n2 }
        - { from: n2, to: n3 }
        - { from: n3, to: n6 }
        - { from: n4, to: n3 }
        - { from: n5, to: n2 }
        - { from: n8, to: n1 }
`
	intoRight := `title: into one icon
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      nodes:
        - icon: { name: n0, icon: lucide/server, label: wwwww }
        - box: { name: n1, rows: [wwwwwwwwwwwww], shape: pill }
        - icon: { name: n2, icon: lucide/server, label: wwwwwwwwww }
        - box: { name: n3, rows: [wwwwwwwwwwww], shape: pill }
        - icon: { name: n4, icon: lucide/server, label: w }
        - box: { name: n5, rows: [wwwwww], shape: hex }
      edges:
        - { from: n0, to: n3 }
        - { from: n4, to: n1 }
        - { from: n4, to: n0 }
        - { from: n4, to: n2 }
        - { from: n3, to: n4 }
        - { from: n2, to: n0 }
        - { from: n5, to: n2 }
        - { from: n1, to: n3 }
        - { from: n1, to: n5 }
`
	s387 := `title: S387
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      nodes:
        - icon: { name: browser, icon: lucide/globe, label: Browser }
        - group:
            label: Windows
            nodes:
              - box: { name: winfloe, rows: [floe.exe] }
              - box: { name: vscode, rows: [VS Code] }
        - group:
            label: WSL
            nodes:
              - box: { name: wslfloe, rows: [floe] }
              - box: { name: store, rows: [store] }
      edges:
        - { from: browser, to: winfloe, label: http }
        - { from: browser, to: vscode }
        - { from: browser, to: wslfloe, label: http }
        - { from: vscode, to: store }
`
	cases := map[string]*graph{}
	for name, src := range map[string]string{"S387": s387, "in and out": inOut, "into one icon, right": intoRight} {
		_, v, err := yamlpos.ParseMap("s.yaml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		s, err := read(v)
		if err != nil {
			t.Fatal(err)
		}
		cases[name] = s.Parts[0].Graph
	}
	// Two into n0 at its bottom, drawn up against the ranks (n1→n0, n2→n0).
	rnd := rand.New(rand.NewPCG(49, 20)) //nolint:gosec // G404: a graph to try, no secret
	for i := range 41 {
		g := randomGraph(rnd, "")
		iconize(g)
		if i == 40 {
			cases["into one icon, down"] = g
		}
	}
	for name, g := range cases {
		l := laid(g)
		if fault := iconFault(g, l); fault != "" {
			t.Errorf("%s: %s", name, fault)
			continue
		}
		if n := sharedStretches(l); n > 0 {
			t.Errorf("%s: %d pairs of edges share a stretch", name, n)
		}
	}
}

// An edge between an icon and a group ends on the group's frame, inEnd or more from its corners and clear of its heading
// and the headings of the groups round it, however the icon's end moves it: straightened, slid with its label, or by a
// heading.
func TestGraphIconFrameEnds(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"into a group, down": `title: S418
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - icon: { name: a, icon: lucide/server, label: wwwwwwwwwwwwwwww }
        - icon: { name: b, icon: lucide/server, label: wwwwwwwww }
        - icon: { name: c, icon: lucide/server, label: wwww }
        - group:
            name: g
            icon: lucide/box
            nodes:
              - icon: { name: d, icon: lucide/server, label: wwwww }
      edges:
        - { from: c, to: a, label: wwwwwwww }
        - { from: b, to: g, label: wwwwww }
        - { from: a, to: g }
`,
		"out of a group, right": `title: S418 right
parts:
  - graph:
      direction: right
      x: 20
      y: 20
      nodes:
        - box: { name: n3, rows: [wwwwwww] }
        - icon: { name: n4, icon: lucide/server, label: wwwwwww }
        - box: { name: n5, rows: [wwwwwwwwwwwwwww], shape: state }
        - group:
            name: g0
            icon: lucide/box
            nodes:
              - icon: { name: n7, icon: lucide/server, label: wwwww }
              - box: { name: n10, rows: [ww] }
        - box: { name: n9, rows: [wwwwwwwwwwwwwww] }
        - box: { name: n12, rows: [wwwwwwwwwwwwwww] }
        - box: { name: n14, rows: [wwwwwww] }
        - icon: { name: n15, icon: lucide/server, label: ww }
        - box: { name: n16, rows: [wwwww] }
      edges:
        - { from: n12, to: n4 }
        - { from: n3, to: n10 }
        - { from: n12, to: n16 }
        - { from: n9, to: n14 }
        - { from: n16, to: n5 }
        - { from: g0, to: n15 }
        - { from: n10, to: n16 }
        - { from: n14, to: n10 }
        - { from: n16, to: n9 }
        - { from: n5, to: g0 }
`,
		"a labelled edge slid, right": `title: S418 A
parts:
  - graph:
      direction: right
      x: 20
      y: 20
      nodes:
        - icon: { name: n0, icon: lucide/server, label: www }
        - group:
            name: g0
            label: wwww
            icon: lucide/box
            nodes:
              - icon: { name: n1, icon: lucide/server, label: ww }
        - group:
            name: g2
            label: wwwwwwwwww
            icon: lucide/box
            nodes:
              - icon: { name: n2, icon: lucide/server, label: wwwwwwwwwwwwwww }
              - box: { name: n4, rows: [wwwwwwwwwwwwwww] }
              - icon: { name: n10, icon: lucide/server, label: wwww }
              - box: { name: n15, rows: [ww], shape: state }
        - box: { name: n3, rows: [wwwwwwwwww] }
        - group:
            name: g1
            label: wwwwwwwwwwww
            icon: lucide/box
            nodes:
              - icon: { name: n6, icon: lucide/server, label: w }
        - box: { name: n8, rows: [wwwwwwwwwwwwww], shape: hex }
        - icon: { name: n9, icon: lucide/server, label: wwwwwwwwwww }
        - box: { name: n11, rows: [wwwww], shape: hex }
        - box: { name: n13, rows: [www], shape: pill }
      edges:
        - { from: n0, to: n2, label: { en: wwwwwww, ja: 字字字 } }
        - { from: n9, to: n3, label: { en: www } }
        - { from: n9, to: n15, label: { en: wwwwwww, ja: 字字字字字字 } }
        - { from: n0, to: n9, label: { en: ww, ja: 字字字字字字 } }
        - { from: n3, to: n10, label: { en: wwwwwwwwww, ja: 字字字字字字 } }
        - { from: n8, to: n15, label: { en: wwwwwwwwww, ja: 字字字 } }
        - { from: n0, to: n10, label: { en: wwww, ja: 字字字字字字 } }
        - { from: n13, to: g1, label: { en: w, ja: 字字字字字字字字字 } }
        - { from: n1, to: g2, label: { en: wwwwww, ja: 字字字字字字字字字字字 } }
        - { from: n6, to: g0, label: { en: wwwwwww, ja: 字字字 } }
        - { from: n1, to: n2, label: { en: wwwwww, ja: 字字字 } }
        - { from: n11, to: n0, label: { en: wwwww, ja: 字字字字字字字字字字 } }
        - { from: n3, to: n6, label: { en: wwwwwwwwww, ja: 字 } }
        - { from: n13, to: n2, label: { en: wwww, ja: 字字字字字字字字字字字字字字 } }
        - { from: n10, to: n15, label: { en: w, ja: 字字字字字字 } }
`,
		"by the heading, down": `title: S418 B
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group:
            name: g0
            label: wwwwwwwwwwww
            icon: lucide/box
            nodes:
              - icon: { name: n0, icon: lucide/server, label: www }
              - box: { name: n1, rows: [wwww] }
        - box: { name: n2, rows: [www] }
        - icon: { name: n7, icon: lucide/server, label: wwwwwwwwwww }
      edges:
        - { from: n7, to: g0, label: ww }
        - { from: n2, to: n0 }
        - { from: n2, to: n7, label: wwwwww }
`,
		"a reversed edge by the heading, down": `title: S418 C
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group:
            name: g0
            label: wwwwwwwwwwwwwww
            icon: lucide/box
            nodes:
              - icon: { name: n0, icon: lucide/server, label: wwww }
        - box: { name: n1, rows: [www] }
        - icon: { name: n2, icon: lucide/server, label: wwwwwwwwwww }
        - box: { name: n3, rows: [wwwww] }
        - icon: { name: n4, icon: lucide/server, label: wwwwwww }
      edges:
        - { from: n3, to: n0 }
        - { from: g0, to: n1 }
        - { from: n2, to: n0, label: { en: ww } }
        - { from: n0, to: n4 }
        - { from: g0, to: n2, label: { en: wwwwwwwww } }
`,
		"by the heading, right": `title: S418 D
parts:
  - graph:
      direction: right
      x: 20
      y: 20
      nodes:
        - group:
            name: g0
            label: wwwwww
            nodes:
              - icon: { name: n0, icon: lucide/server, label: wwwwwwwwww }
              - box: { name: n1, rows: [ww] }
        - group:
            name: g1
            label: wwwwwww
            nodes:
              - icon: { name: n2, icon: lucide/server, label: wwwwww }
        - icon: { name: n3, icon: lucide/server, label: wwwwww }
        - box: { name: n4, rows: [wwwwwwww] }
        - icon: { name: n5, icon: lucide/server, label: www }
        - box: { name: n6, rows: [wwwwwwwwwwww] }
      edges:
        - { from: n2, to: n4 }
        - { from: n2, to: g0 }
        - { from: g1, to: n0, label: wwwwwwww }
`,
		"by the heading of a group round it, down": `title: S418 F
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group:
            name: g1
            label: wwwwwwwwwwwwwww
            nodes:
              - group:
                  name: g0
                  icon: lucide/box
                  nodes:
                    - icon: { name: n0, icon: lucide/server, label: ww }
        - box: { name: n1, rows: [w] }
        - icon: { name: n2, icon: lucide/server, label: wwwwwwwww }
        - box: { name: n3, rows: [wwww] }
      edges:
        - { from: n1, to: g0, label: ww }
        - { from: n2, to: g0 }
`,
		"by the heading of an unnamed group round it, down": `title: S418 F, g1 of no name
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group:
            label: wwwwwwwwwwwwwww
            nodes:
              - group:
                  name: g0
                  icon: lucide/box
                  nodes:
                    - icon: { name: n0, icon: lucide/server, label: ww }
        - box: { name: n1, rows: [w] }
        - icon: { name: n2, icon: lucide/server, label: wwwwwwwww }
        - box: { name: n3, rows: [wwww] }
      edges:
        - { from: n1, to: g0, label: ww }
        - { from: n2, to: g0 }
`,
	} {
		g := graphOfSpec(t, src)
		if fault := iconFault(g, laid(g)); fault != "" {
			t.Errorf("%s: %s", name, fault)
		}
	}
}

// headedGraph is a graph of 3 to 8 nodes, every other an icon, at random from rnd: a group g0 of the first 1 to 3, its
// label left out a time in four, and half the time a headed group g1 round g0 and the next node, or of the next 1 or 2;
// their icons there or not; and 2 to 8 edges, half of them labelled and many to or from a group.
func headedGraph(rnd *rand.Rand, direction string) *graph {
	names := make([]string, 3+rnd.IntN(6))
	for i := range names {
		names[i] = fmt.Sprintf("n%d", i)
	}
	g := graphOf(names)
	g.Direction = direction
	for _, nd := range g.Nodes {
		nd.Box.Rows[0].Text["en"] = strings.Repeat("w", 1+rnd.IntN(12))
	}
	size := 1 + rnd.IntN(3)
	label := strings.Repeat("w", 1+rnd.IntN(16))
	if rnd.IntN(4) == 0 { // a group headed by its icon alone, or not at all, beneath a headed one round it
		label = ""
	}
	inGroup(g, "g0", label, names[:size]...)
	ends, held := append(slices.Clone(names), "g0", "g0", "g0"), map[string][]string{"g0": names[:size]}
	if rnd.IntN(2) == 0 && size+2 < len(names) {
		members, holds := names[size:size+1+rnd.IntN(2)], []string{}
		if rnd.IntN(2) == 0 {
			members = []string{"g0", names[size]}
			holds = append(holds, "g0")
			holds = append(holds, names[:size]...)
		}
		inGroup(g, "g1", strings.Repeat("w", 1+rnd.IntN(16)), members...)
		ends, held["g1"] = append(ends, "g1", "g1"), append(holds, members...)
	}
	for range 2 + rnd.IntN(7) {
		a, b := ends[rnd.IntN(len(ends))], ends[rnd.IntN(len(ends))]
		if a == b || slices.Contains(held[a], b) || slices.Contains(held[b], a) {
			continue
		}
		e := graphEdge{From: a, To: b, Dashed: false, Bold: false, Tone: "", Start: nil, End: nil, Label: nil}
		if rnd.IntN(2) == 0 {
			e.Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(10))}
		}
		g.Edges = append(g.Edges, e)
	}
	iconize(g)
	_, clusters, _ := flatten(g, "#") // in the order written, so that a seed and a graph's number make it again
	for _, c := range clusters {
		if rnd.IntN(2) == 0 {
			c.g.Icon = ""
		}
	}
	return g
}

// Laid out, the ends at the headed groups of random small graphs stand on their frames, inEnd from the corners, a
// nodeGap clear of the headings as drawn and keepApart from the other edges' ends (frameEndFault), and the routes place
// them so before any moves.
func TestGraphIconFrameEndsRandom(t *testing.T) {
	t.Parallel()

	for _, direction := range []string{"", "right"} {
		rnd := rand.New(rand.NewPCG(53, 18)) //nolint:gosec // G404: graphs to try, no secret
		for i := range tries(2000) {
			g := headedGraph(rnd, direction)
			l := laid(g)
			if len(l.problems) > 0 {
				if at := slices.IndexFunc(l.problems, func(p *usererr.Error) bool {
					return p.Key != "diagram_graph_inside" && p.Key != "diagram_graph_among"
				}); at >= 0 {
					t.Fatalf("%q, graph %d: %s", direction, i, l.problems[at].Error())
				}
				continue // an edge between a group and what it holds, or what stands among its ranks: told
			}
			if fault := cmp.Or(iconFault(g, l), runsFault(g, routed(g))); fault != "" {
				t.Fatalf("%q, graph %d: %s", direction, i, fault)
			}
		}
	}
}

// graphOfSpec is the graph of src, a diagram spec of one graph.
func graphOfSpec(t *testing.T, src string) *graph {
	t.Helper()
	_, v, err := yamlpos.ParseMap("s.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	s, err := read(v)
	if err != nil {
		t.Fatal(err)
	}
	return s.Parts[0].Graph
}

// An edge into a group laid out right that runs straight past its heading stays straight: the heading is kept clear
// of as drawn, a nodeGap below it.
func TestGraphIconFrameEndsStraight(t *testing.T) {
	t.Parallel()

	g := graphOfSpec(t, `title: right 686
parts:
  - graph:
      direction: right
      x: 20
      y: 20
      nodes:
        - group:
            name: g0
            label: wwwwwwwwwwwww
            icon: lucide/box
            nodes:
              - icon: { name: n0, icon: lucide/server, label: ww }
        - box: { name: n1, rows: [wwwwwwwwwwww] }
        - icon: { name: n2, icon: lucide/server, label: wwwww }
        - box: { name: n3, rows: [wwwwwwwwwww] }
        - icon: { name: n4, icon: lucide/server, label: wwwwwwwwwwww }
        - box: { name: n5, rows: [wwwwww] }
        - icon: { name: n6, icon: lucide/server, label: w }
        - box: { name: n7, rows: [wwwwwwwww] }
      edges:
        - { from: n2, to: n5 }
        - { from: n6, to: g0, label: wwwwwwwwww }
        - { from: n6, to: g0 }
        - { from: n7, to: n0 }
`)
	l := laid(g)
	if fault := iconFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	for k, e := range g.Edges {
		if e.To == "g0" && len(l.paths[k].segs) != 1 {
			t.Errorf("%s runs %v, want it straight", l.paths[k].what, l.paths[k].segs)
		}
	}
}
