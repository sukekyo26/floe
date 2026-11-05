package diagram

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// noShares lays nothing: every end stands in the middle of its side.
func noShares(*layout, []item) {}

// asToday is what lays s out with no edge stood straight.
func asToday(s *spec, opts Options) *layout {
	l := layoutWith(s, opts, placing{(*layout).shareEnds, (*drawing).settleEnds})
	l.check()
	return l
}

// sharesFault is what placing the ends does wrong to a spec that lays out with no mistake with them in the middle: a
// mistake, and for compared more turns or crossings. It says "" for a spec already told, and counts moved.
func sharesFault(s *spec, compared bool, moved *int) string {
	opts := Options{Lang: "en", Icons: anyIcon{}, Seq: 1}
	plain := layoutWith(s, opts, placing{noShares, (*drawing).settleEnds})
	plain.check()
	if plain.failure != nil || len(plain.problems) > 0 {
		return ""
	}
	l := newLayout(s, opts)
	l.check()
	if l.failure != nil {
		return l.failure.Error()
	}
	if len(l.problems) > 0 {
		return fmt.Sprintf("told %s %v", l.problems[0].Key, l.problems[0].Args)
	}
	for k, p := range l.paths {
		if fmt.Sprint(p.segs) != fmt.Sprint(plain.paths[k].segs) {
			*moved++
			break
		}
	}
	if !compared {
		return ""
	}
	for k, p := range l.paths {
		if len(p.segs) > len(plain.paths[k].segs) {
			return fmt.Sprintf("%s turns %d times, %d in the middle", p.what, len(p.segs)-1, len(plain.paths[k].segs)-1)
		}
	}
	if a, b := crossings(l), crossings(plain); a > b {
		return fmt.Sprintf("%d crossings, %d in the middle", a, b)
	}
	return ""
}

// Random boxes joined with no side written, and a graph with boxes above it joined to its boxes: what lays out with
// every end in the middle lays out with them placed. The boxes alone neither turn nor cross more.
func TestEdgeSharesRandom(t *testing.T) {
	t.Parallel()

	t.Run("boxes", func(t *testing.T) {
		t.Parallel()
		rnd := rand.New(rand.NewPCG(77, 78)) //nolint:gosec // G404: diagrams to try, no secret
		moved := 0
		for i := range 3000 {
			if fault := sharesFault(randomBoxes(rnd), true, &moved); fault != "" {
				t.Fatalf("spec %d: %s", i, fault)
			}
		}
		if moved < 300 {
			t.Errorf("%d specs moved, fewer than 300", moved)
		}
	})
	t.Run("graph", func(t *testing.T) {
		t.Parallel()
		rnd := rand.New(rand.NewPCG(91, 92)) //nolint:gosec // G404: diagrams to try, no secret
		moved := 0
		for i := range 10000 {
			s := graphWithHands(rnd)
			if s == nil {
				continue
			}
			if fault := sharesFault(s, false, &moved); fault != "" {
				t.Fatalf("spec %d: %s", i, fault)
			}
		}
		if moved < 100 {
			t.Errorf("%d specs moved, fewer than 100", moved)
		}
	})
}

func textBox(name string, x, y, w, h float64) part {
	//nolint:exhaustruct_v5 // one kind of part, the rest as left out
	return part{Box: &box{Name: name, X: x, Y: y, W: w, H: h, Rows: []row{{Text: Text{"en": name}, Style: ""}}}}
}

func nameEdge(from, to string) part {
	//nolint:exhaustruct_v5 // one kind of part, the rest as left out
	return part{Edge: &edge{From: end{Name: from}, To: end{Name: to}}}
}

// randomBoxes is boxes on a rough grid and edges between them at random.
func randomBoxes(rnd *rand.Rand) *spec {
	var parts []part
	var names []string
	for c := range 2 + rnd.IntN(3) {
		for r := range 2 + rnd.IntN(3) {
			if rnd.IntN(4) == 0 {
				continue
			}
			n := fmt.Sprintf("b%d_%d", c, r)
			names = append(names, n)
			parts = append(parts, textBox(n, 40+float64(c)*260+float64(rnd.IntN(4))*20, 40+float64(r)*170+float64(rnd.IntN(3))*10,
				80+float64(rnd.IntN(4))*40, 29+float64(rnd.IntN(3))*30))
		}
	}
	for range 2 + rnd.IntN(len(names)*2+1) {
		if len(names) > 0 {
			if a, b := names[rnd.IntN(len(names))], names[rnd.IntN(len(names))]; a != b {
				parts = append(parts, nameEdge(a, b))
			}
		}
	}
	//nolint:exhaustruct_v5 // no canvas
	return &spec{Width: 1400, Height: 900, Title: Text{"en": "g"}, Parts: parts}
}

// graphWithHands is a small graph with boxes above it, each joined to its boxes by 2 or 3 edges; nil for a larger one.
func graphWithHands(rnd *rand.Rand) *spec {
	dir := ""
	if rnd.IntN(2) == 0 {
		dir = "right"
	}
	g := randomGraph(rnd, dir)
	if len(g.Edges) > 12 {
		return nil
	}
	g.X, g.Y = 40, 300
	boxes := graphBoxes(g)
	//nolint:exhaustruct_v5 // one kind of part
	parts := []part{{Graph: g}}
	for i := range 1 + rnd.IntN(3) {
		n := fmt.Sprintf("h%d", i)
		parts = append(parts, textBox(n, 60+float64(i)*140, 20, 100, 40))
		to := boxes[rnd.IntN(len(boxes))].Name
		for range 2 + rnd.IntN(2) {
			parts = append(parts, nameEdge(n, to))
			if rnd.IntN(2) == 0 {
				to = boxes[rnd.IntN(len(boxes))].Name
			}
		}
	}
	//nolint:exhaustruct_v5 // no canvas
	return &spec{Width: 3000, Height: 3000, Title: Text{"en": "g"}, Parts: parts}
}

// An edge that would run through a graph's label, or cross a graph's edge, stays in the middle: nothing of the graph
// stands where the graph kept clear, and it is only that which an edge in the middle comes to.
func TestEdgeSharesGraph(t *testing.T) {
	t.Parallel()

	fan := []part{
		textBox("a", 20, 20, 60, 60), textBox("b", 200, 0, 60, 30), textBox("c", 200, 100, 60, 30),
		sidesEdge("a", "b"), sidesEdge("a", "c"),
	}
	//nolint:exhaustruct_v5 // one kind of part, the rest as left out
	label := part{Edge: &edge{
		From: end{Point: &[2]float64{400, 300}}, To: end{Point: &[2]float64{500, 300}}, graph: "#/g",
		Label: Text{"en": "label"}, At: &[2]float64{110, 66},
	}}
	//nolint:exhaustruct_v5 // one kind of part, the rest as left out
	across := part{Edge: &edge{From: end{Point: &[2]float64{110, 55}}, To: end{Point: &[2]float64{110, 90}}, graph: "#/g"}}
	for name, extra := range map[string]part{"a label": label, "an edge": across} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			//nolint:exhaustruct_v5 // no canvas
			l := newLayout(&spec{Width: 800, Height: 600, Title: Text{"en": "g"}, Parts: append(fan[:len(fan):len(fan)], extra)},
				Options{Lang: "en", Icons: anyIcon{}, Seq: 1})
			l.check()
			if len(l.problems) > 0 || l.failure != nil {
				t.Fatalf("%v %v", l.problems, l.failure)
			}
			if got := l.paths[0].segs[0][1]; got != 50 {
				t.Errorf("a→b leaves at %v, want the middle", got)
			}
			if got := l.paths[1].segs[0][1]; got != 50 {
				t.Errorf("a→c leaves at %v, want the middle", got)
			}
		})
	}
}

func sidesEdge(from, to string) part {
	//nolint:exhaustruct_v5 // one kind of part, the rest as left out
	return part{Edge: &edge{From: end{Name: from}, To: end{Name: to}, FromSide: "right", ToSide: "left"}}
}

// straightSpec is boxes on a rough grid, edges between them, and, by kind, labels on half of the edges, a quarter of
// them straight, or those with all else a diagram holds: lines, axes, dots, frames an edge may end at, a table, a graph,
// via, ends at a point, edges written to cross, labels with a place and without, and the size left out.
func straightSpec(rnd *rand.Rand, kind string) *spec {
	var parts []part
	var names []string
	for c := range 2 + rnd.IntN(3) {
		for r := range 2 + rnd.IntN(3) {
			if rnd.IntN(4) == 0 {
				continue
			}
			name := fmt.Sprintf("b%d_%d", c, r)
			names = append(names, name)
			b := textBox(name, float64(40+c*260+rnd.IntN(4)*20), float64(40+r*170+rnd.IntN(4)*15), float64(80+rnd.IntN(4)*40), float64(29+rnd.IntN(3)*30))
			if kind == "touching" {
				b = textBox(name, float64(40+c*100), float64(40+r*80+rnd.IntN(3)*20), 100, float64(40+rnd.IntN(4)*20))
			}
			b.Box.Shape = []string{"", "pill", "hex", "state"}[rnd.IntN(4)]
			parts = append(parts, b)
		}
	}
	if len(names) < 2 {
		return nil
	}
	spot := func() [2]float64 { return [2]float64{float64(30 + rnd.IntN(1000)), float64(30 + rnd.IntN(700))} }
	if kind == "all" {
		parts, names = withAll(rnd, parts, names, spot)
	}
	for range 2 + rnd.IntN(len(names)*2) {
		a, b := names[rnd.IntN(len(names))], names[rnd.IntN(len(names))]
		if a == b {
			continue
		}
		p := nameEdge(a, b)
		switch {
		case kind == "labels" && rnd.IntN(2) == 0:
			p.Edge.Label = Text{"en": strings.Repeat("w", 2+rnd.IntN(8))}
		case kind == "straight" && rnd.IntN(4) == 0:
			p.Edge.Straight = true
		case kind == "all":
			writeAny(rnd, p.Edge, spot)
		}
		parts = append(parts, p)
	}
	s := &spec{Width: 1400, Height: 900, Title: Text{"en": "g"}, Parts: parts} //nolint:exhaustruct_v5 // the rest as left out
	if kind == "all" && rnd.IntN(2) == 0 {
		s.Width, s.Height = 0, 0
	}
	return s
}

// withAll adds to parts what straightSpec's kind all holds beside boxes.
func withAll(rnd *rand.Rand, parts []part, names []string, spot func() [2]float64) ([]part, []string) {
	//nolint:exhaustruct_v5 // each one kind of part, the rest as left out
	for range rnd.IntN(4) {
		at := spot()
		switch rnd.IntN(5) {
		case 0:
			parts = append(parts, part{Line: &line{X1: at[0], Y1: at[1], X2: at[0] + float64(40+rnd.IntN(300)), Y2: at[1]}})
		case 1:
			parts = append(parts, part{Line: &line{X1: at[0], Y1: at[1], X2: at[0], Y2: at[1] + float64(40+rnd.IntN(300))}})
		case 2:
			parts = append(parts, part{Dot: &dot{CX: at[0], CY: at[1], R: 4}})
		case 3:
			parts = append(parts, part{Axis: &axis{X1: at[0], X2: at[0] + float64(100+rnd.IntN(300)), Y: at[1], Ticks: nil}})
		case 4:
			name := fmt.Sprintf("f%d", len(parts))
			names = append(names, name)
			parts = append(parts, part{Frame: &frame{Name: name, X: at[0], Y: at[1], W: float64(200 + rnd.IntN(500)), H: float64(150 + rnd.IntN(300))}})
		}
	}
	if rnd.IntN(4) == 0 {
		at := spot()
		name := fmt.Sprintf("t%d", len(parts))
		names = append(names, name)
		//nolint:exhaustruct_v5 // one kind of part, the rest as left out
		parts = append(parts, part{Table: &table{Name: name, X: at[0], Y: at[1], W: 120, Title: Text{"en": "T"}, Rows: []tableRow{{Name: "r", Cells: []Text{{"en": "r"}}}}}})
	}
	if rnd.IntN(3) == 0 {
		g := graphOf([]string{"g1", "g2", "g3"}, "g1>g2", "g1>g3")
		at := spot()
		g.X, g.Y = at[0], at[1]
		parts = append(parts, part{Graph: g}) //nolint:exhaustruct_v5 // one kind of part
		names = append(names, "g1", "g2")
	}
	return parts, names
}

// writeAny writes on e, sometimes, what straightSpec's kind all holds on an edge.
func writeAny(rnd *rand.Rand, e *edge, spot func() [2]float64) {
	switch rnd.IntN(8) {
	case 0:
		e.Straight = true
	case 1:
		e.Crossing = true
	case 2:
		e.Via = [][2]float64{spot()}
	case 3:
		at := spot()
		e.From = end{Name: "", Point: &at}
	case 4:
		e.Label = Text{"en": "wwww"}
		if rnd.IntN(2) == 0 {
			at := spot()
			e.At = &at
		}
	}
}

// pairsWorse is the first pair of edges of b that cross, touch or stand within keepApart where those of a do not, or "".
func pairsWorse(a, b *layout) string {
	count := func(p, q path) int {
		n := 0
		for _, s := range p.segs {
			for _, o := range q.segs {
				if _, _, ok := crossing(s, o); ok {
					n++
				}
				if runsTouch(s, o) {
					n += 10
				}
				if (s[0] == s[2] || s[1] == s[3]) && (o[0] == o[2] || o[1] == o[3]) && runsClose(s, o) {
					n += 100
				}
			}
		}
		return n
	}
	for i := range a.paths {
		for j := i + 1; j < len(a.paths); j++ {
			if count(b.paths[i], b.paths[j]) > count(a.paths[i], a.paths[j]) {
				return b.paths[i].what + " and " + b.paths[j].what
			}
		}
	}
	return ""
}

// What lays out with no edge stood straight lays out with them, of every kind of diagram, and no edge turns more and no
// pair of edges comes to cross, touch or stand close. keepOff moving another's run within keepApart afterwards and a
// slanting edge are its LIMITs, and do not come up here. The diagrams given up are few, and checked once more are
// unchanged.
func TestEdgeStraightKeeps(t *testing.T) {
	t.Parallel()

	opts := Options{Lang: "en", Icons: anyIcon{}, Seq: 1}
	for _, kind := range []string{"plain", "labels", "straight", "all", "touching"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			rnd := rand.New(rand.NewPCG(177, uint64(len(kind)))) //nolint:gosec // G404: diagrams to try, no secret
			laidOut, moved, given := 0, 0, 0
			for i := range 3000 {
				s := straightSpec(rnd, kind)
				if s == nil {
					continue
				}
				a := asToday(s, opts)
				if a.failure != nil || len(a.problems) > 0 {
					continue
				}
				laidOut++
				b := newLayout(s, opts)
				was := b.inspect()
				b.check()
				if b.failure != nil || len(b.problems) > 0 {
					t.Fatalf("spec %d: told %v %v", i, b.failure, b.problems)
				}
				if b.inspect() != was {
					t.Fatalf("spec %d: it differs checked once more", i)
				}
				if fault := pairsWorse(a, b); fault != "" {
					t.Fatalf("spec %d: %s are worse", i, fault)
				}
				for k, p := range b.paths {
					if len(p.segs) > len(a.paths[k].segs) {
						t.Fatalf("spec %d: %s turns %d times, %d before", i, p.what, len(p.segs)-1, len(a.paths[k].segs)-1)
					}
				}
				moved += b.straightened
				try := layoutWith(s, opts, placing{(*layout).shareStraight, (*drawing).settleOff})
				if try.straightened > 0 && b.straightened == 0 {
					given++
				}
			}
			t.Logf("KEEPS %s laid out %d, moved %d, given up %d", kind, laidOut, moved, given)
			if moved == 0 {
				t.Errorf("no edge moved of %d diagrams", laidOut)
			}
			if given*10 > laidOut {
				t.Errorf("%d diagrams of %d given up, over one in ten", given, laidOut)
			}
		})
	}
}

// A mistake left by the straight edges gives them up, those of a graph too: here a straight edge written by hand across
// where offPoint stands the icons' edge.
func TestEdgeStraightGivesUpIcons(t *testing.T) {
	t.Parallel()

	opts := Options{Lang: "en", Icons: anyIcon{}, Seq: 1}
	g := labelledChain(17, 9, 16)
	across := part{Edge: &edge{From: end{Name: "", Point: &[2]float64{83.2, 273.5}}, To: end{Name: "", Point: &[2]float64{89.2, 273.5}}, Straight: true}} //nolint:exhaustruct_v5 // one kind of part
	s := &spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: []part{{Graph: g}, across}}                                                            //nolint:exhaustruct_v5 // two kinds of part
	a, b := asToday(s, opts), newLayout(s, opts)
	b.check()
	if len(a.problems) > 0 || len(b.problems) > 0 || b.failure != nil {
		t.Fatalf("told: as today %v, now %v %v", a.problems, b.problems, b.failure)
	}
	if got, want := b.inspect(), a.inspect(); got != want {
		t.Errorf("got\n%swant as today\n%s", got, want)
	}
}
