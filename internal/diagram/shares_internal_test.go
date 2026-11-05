package diagram

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// noShares lays nothing: every end stands in the middle of its side.
func noShares(*layout, []item) {}

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
