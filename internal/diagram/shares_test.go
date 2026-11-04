package diagram_test

import (
	"fmt"
	"strings"
	"testing"
)

// edgesOf is the lines of the edges of src as drawn, and what it is told.
func edgesOf(t *testing.T, src string) (string, string) {
	t.Helper()
	got, told := draw(t, "width: 800\nheight: 600\ntitle: T\nparts:\n"+src, inspect)
	var edges []string
	for line := range strings.Lines(got) {
		if strings.HasPrefix(line, "edge ") {
			edges = append(edges, strings.TrimSuffix(line, "\n"))
		}
	}
	return strings.Join(edges, "\n"), told
}

// The n ends written by hand on a side stand at the points that part it into n+1 equal parts, across a side and along
// one. One edge runs from middle to middle.
func TestEdgeShares(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, parts string
		want        string
	}{
		{
			"one",
			"  - box: { name: a, x: 100, y: 20, w: 120, h: 30, rows: [a] }\n  - box: { name: b, x: 100, y: 100, w: 120, h: 30, rows: [b] }\n" +
				"  - edge: { from: a, to: b }\n",
			"edge a→b M160,50 V100",
		},
		{
			"two down",
			"  - box: { name: a, x: 100, y: 20, w: 120, h: 30, rows: [a] }\n" +
				"  - box: { name: b, x: 0, y: 100, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 260, y: 100, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, fromSide: bottom, to: b, toSide: top }\n  - edge: { from: a, fromSide: bottom, to: c, toSide: top }\n",
			"edge a→b M140,50 V75 H30 V100\nedge a→c M180,50 V75 H290 V100",
		},
		{
			"three down, the middle one straight",
			"  - box: { name: a, x: 100, y: 20, w: 120, h: 30, rows: [a] }\n" +
				"  - box: { name: b, x: 0, y: 100, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 130, y: 100, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 260, y: 100, w: 60, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, fromSide: bottom, to: b, toSide: top }\n  - edge: { from: a, fromSide: bottom, to: c, toSide: top }\n  - edge: { from: a, fromSide: bottom, to: d, toSide: top }\n",
			"edge a→b M130,50 V75 H30 V100\nedge a→c M160,50 V100\nedge a→d M190,50 V75 H290 V100",
		},
		{
			"three to the right, by where they go",
			"  - box: { name: a, x: 20, y: 60, w: 100, h: 80, rows: [a] }\n" +
				"  - box: { name: b, x: 240, y: 0, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 240, y: 85, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 240, y: 170, w: 60, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, to: d, fromSide: right, toSide: left }\n  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n",
			"edge a→d M120,120 H180 V185 H240\nedge a→b M120,80 H180 V15 H240\nedge a→c M120,100 H240",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got, told := edgesOf(t, c.parts); got != c.want || told != "" {
				t.Errorf("%s\n  drew\n%s\n%s\n  want\n%s", c.parts, got, told, c.want)
			}
		})
	}
}

// Five ends part a side into six.
func TestEdgeSharesFive(t *testing.T) {
	t.Parallel()

	parts := "  - box: { name: a, x: 240, y: 20, w: 120, h: 30, rows: [a] }\n"
	for i := range 5 {
		parts += fmt.Sprintf("  - box: { name: n%d, x: %d, y: 120, w: 96, h: 30, rows: [n] }\n", i, i*120)
		parts += fmt.Sprintf("  - edge: { from: a, fromSide: bottom, to: n%d, toSide: top }\n", i)
	}
	got, told := edgesOf(t, parts)
	if told != "" {
		t.Fatal(told)
	}
	for i, x := range []string{"260", "280", "300", "320", "340"} {
		if want := fmt.Sprintf("edge a→n%d M%s,50", i, x); !strings.Contains(got, want) {
			t.Errorf("drew\n%s\nwant %s", got, want)
		}
	}
}

// An edge running straight between its middles stays straight, and the rest on its side stand evenly around it.
func TestEdgeSharesStraight(t *testing.T) {
	t.Parallel()

	const src = `  - box: { name: a, x: 20, y: 60, w: 60, h: 48, rows: [a] }
  - box: { name: s, x: 200, y: 130, w: 60, h: 30, rows: [s] }
  - box: { name: b, x: 200, y: 60, w: 60, h: 48, rows: [b] }
  - box: { name: c, x: 20, y: 0, w: 60, h: 30, rows: [c] }
  - box: { name: d, x: 20, y: 130, w: 60, h: 30, rows: [d] }
  - edge: { from: a, to: b, fromSide: right, toSide: left }
  - edge: { from: a, to: s, fromSide: right, toSide: left, crossing: true }
  - edge: { from: c, to: b, fromSide: right, toSide: left, crossing: true }
  - edge: { from: d, to: b, fromSide: right, toSide: left, crossing: true }
`
	const want = `edge a→b M80,84 H200
edge a→s M80,96 H140 V145 H200
edge c→b M80,15 H140 V72 H200
edge d→b M80,145 H140 V96 H200`
	if got, told := edgesOf(t, src); got != want || told != "" {
		t.Errorf("drew\n%s\n%s\nwant\n%s", got, told, want)
	}
}

// Edges of two boxes, one wider, stay straight and part the narrower side; an edge from a side of two to a side of one
// runs straight to the other's point, the side of one taking it, unless a part stands there.
func TestEdgeSharesMeet(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, parts, want string
	}{
		{
			"parallel to a wider box",
			"  - box: { name: a, x: 100, y: 20, w: 60, h: 30, rows: [a] }\n  - box: { name: b, x: 40, y: 100, w: 180, h: 30, rows: [b] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n",
			"edge a→b M120,50 V100\nedge a→b M140,50 V100",
		},
		{
			"the lone end meets the point",
			"  - box: { name: a, x: 100, y: 20, w: 90, h: 30, rows: [a] }\n  - box: { name: b, x: 60, y: 100, w: 90, h: 30, rows: [b] }\n  - box: { name: c, x: 200, y: 100, w: 90, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: a, to: c, fromSide: bottom, toSide: top }\n",
			"edge a→b M130,50 V100\nedge a→c M160,50 V75 H245 V100",
		},
		{
			"a part on the way",
			"  - box: { name: a, x: 100, y: 20, w: 90, h: 30, rows: [a] }\n  - box: { name: b, x: 60, y: 200, w: 90, h: 30, rows: [b] }\n  - box: { name: c, x: 200, y: 200, w: 90, h: 30, rows: [c] }\n  - box: { x: 110, y: 150, w: 41, h: 29, rows: [x] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: a, to: c, fromSide: bottom, toSide: top }\n",
			"edge a→b M130,50 V125 H105 V200\nedge a→c M160,50 V125 H245 V200",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got, told := edgesOf(t, c.parts); got != c.want || told != "" {
				t.Errorf("drew\n%s\n%s\nwant\n%s", got, told, c.want)
			}
		})
	}
}

// The edges of a side go in the order that keeps them from crossing by where they first turn.
func TestEdgeSharesOrder(t *testing.T) {
	t.Parallel()

	const src = `  - box: { name: hub, x: 20, y: 20, w: 60, h: 48, rows: [hub] }
  - box: { name: n1, x: 150, y: 54, w: 60, h: 30, rows: [n1] }
  - box: { name: n2, x: 380, y: 234, w: 60, h: 30, rows: [n2] }
  - edge: { from: hub, to: n1 }
  - edge: { from: hub, to: n2 }
`
	const want = "edge hub→n1 M80,52 H115 V69 H150\nedge hub→n2 M80,36 H230 V249 H380"
	if got, told := edgesOf(t, src); got != want || told != "" {
		t.Errorf("drew\n%s\n%s\nwant\n%s", got, told, want)
	}
}

// An edge that would newly cross another, or run through a part or a graph's label, or come to nothing, or run through
// a box of its own stays in the middle, and what it would be told of is not told. A label written by hand is told.
func TestEdgeSharesStays(t *testing.T) {
	t.Parallel()

	const fan = "  - box: { name: a, x: 20, y: 20, w: 60, h: 60, rows: [a] }\n  - box: { name: b, x: 200, y: 0, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 200, y: 100, w: 60, h: 30, rows: [c] }\n" +
		"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n"
	for _, c := range []struct {
		name, parts, want, told string
	}{
		{
			name:  "a box on the way of one of them, both back in the middle",
			parts: fan + "  - box: { x: 96, y: 55, w: 44, h: 29, rows: [x] }\n",
			want:  "edge a→b M80,50 H140 V15 H200\nedge a→c M80,50 H140 V115 H200",
		},
		{
			name:  "an edge written on the way",
			parts: fan + "  - edge: { from: [90, 55], to: [90, 90] }\n",
			want:  "edge a→b M80,50 H140 V15 H200\nedge a→c M80,50 H140 V115 H200\nedge [90,55]→[90,90] M90,55 V90",
		},
		{
			name: "its own box",
			parts: "  - box: { name: a, x: 100, y: 20, w: 90, h: 30, rows: [a] }\n  - box: { name: b, x: 150, y: 120, w: 90, h: 40, rows: [b] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: left }\n  - edge: { from: a, to: b, fromSide: bottom, toSide: left }\n",
			want: "edge a→b M145,50 V140 H150\nedge a→b M145,50 V140 H150",
		},
		{
			name: "a lone end that would come to nothing",
			parts: "  - box: { name: a, x: 100, y: 80, w: 90, h: 30, rows: [a] }\n  - box: { name: b, x: 190, y: 70, w: 90, h: 60, rows: [b] }\n  - box: { name: c, x: 190, y: 0, w: 90, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n",
			want: "edge a→b M190,95 V100",
		},
		{
			name:  "a label written by hand",
			parts: fan + "  - label: { text: L, x: 110, y: 66 }\n",
			want:  "edge a→b M80,50 H140 V15 H200\nedge a→c M80,50 H140 V115 H200",
		},
		{
			name: "a frame's heading",
			parts: "  - frame: { name: f, x: 100, y: 100, w: 300, h: 120, label: A heading of it }\n  - box: { name: b, x: 120, y: 130, w: 260, h: 29, rows: [b] }\n" +
				"  - box: { name: a, x: 120, y: 20, w: 60, h: 29, rows: [a] }\n  - box: { name: c, x: 220, y: 20, w: 60, h: 29, rows: [c] }\n  - box: { name: d, x: 320, y: 20, w: 60, h: 29, rows: [d] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: c, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: d, to: b, fromSide: bottom, toSide: top }\n",
			want: "edge a→b M150,49 V89.5 H250 V130",
		},
		{
			name: "another edge's label at a place of its own",
			parts: fan + "  - box: { name: p, x: 90, y: 200, w: 60, h: 30, rows: [p] }\n  - box: { name: q, x: 300, y: 200, w: 60, h: 30, rows: [q] }\n" +
				"  - edge: { from: p, to: q, label: L, at: [110, 66] }\n",
			want: "edge a→b M80,50 H140 V15 H200\nedge a→c M80,50 H140 V115 H200",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, told := edgesOf(t, c.parts)
			if c.told != "" {
				if !strings.Contains(told, c.told) {
					t.Errorf("drew\n%s\ntold %q, want %q", got, told, c.told)
				}
				return
			}
			if told != "" || !strings.Contains(got, c.want) {
				t.Errorf("drew\n%s\ntold %q, want %s", got, told, c.want)
			}
		})
	}
}

// What is written where it stands does not move and is not counted: via, a point, straight. An edge back to its side
// goes nowhere, as ever.
func TestEdgeSharesFixed(t *testing.T) {
	t.Parallel()

	const boxes = "  - box: { name: a, x: 20, y: 20, w: 100, h: 29, rows: [a] }\n  - box: { name: c, x: 180, y: 20, w: 100, h: 29, rows: [c] }\n  - box: { name: b, x: 90, y: 120, w: 120, h: 29, rows: [b] }\n"
	for _, c := range []struct {
		name, parts, want string
	}{
		{
			"a point is not counted", boxes + "  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: c, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: [150, 60], to: b, toSide: top }\n",
			"edge a→b M70,49 V84.5 H130 V120\nedge c→b M230,49 V84.5 H170 V120\nedge [150,60]→b M150,60 V120",
		},
		{
			"alone with a point it stays in the middle", boxes + "  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: [150, 60], to: b, toSide: top }\n",
			"edge a→b M70,49 V84.5 H150 V120\nedge [150,60]→b M150,60 V120",
		},
		{
			"via stays in the middle", boxes + "  - edge: { from: a, via: [[70, 80], [150, 80]], to: b, toSide: top }\n  - edge: { from: c, via: [[230, 80], [150, 80]], to: b, toSide: top }\n",
			"edge a→b M70,49 V80 H150 V120\nedge c→b M230,49 V80 H150 V120",
		},
		{
			"straight stays", boxes + "  - edge: { from: a, to: b, fromSide: bottom, toSide: top, straight: true }\n  - edge: { from: c, to: b, fromSide: bottom, toSide: top, straight: true }\n",
			"edge a→b M70,49 L150,120\nedge c→b M230,49 L150,120",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got, told := edgesOf(t, c.parts); got != c.want || told != "" {
				t.Errorf("drew\n%s\n%s\nwant\n%s", got, told, c.want)
			}
		})
	}
	if _, told := edgesOf(t, "  - box: { name: a, x: 100, y: 20, w: 80, h: 29, rows: [a] }\n  - edge: { from: a, fromSide: left, to: a, toSide: left }\n"); !strings.Contains(told, "starts where it ends") {
		t.Errorf("an edge back to its side is told %q", told)
	}
}

// A row, a bar and a dot keep their edges in the middle; the ends on a round box or a hexagon stand on the outline; the
// ends on an icon's bottom side stand under its label within its width.
func TestEdgeSharesKinds(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, parts, want string
	}{
		{
			"bar",
			"  - bar: { name: a, x1: 100, x2: 200, y: 44 }\n  - bar: { name: b, x1: 300, x2: 400, y: 20 }\n  - bar: { name: c, x1: 300, x2: 400, y: 90 }\n" +
				"  - edge: { from: a, fromSide: right, to: b, toSide: left }\n  - edge: { from: a, fromSide: right, to: c, toSide: left }\n",
			"edge a→b M200,44 H250 V20 H300\nedge a→c M200,44 H250 V90 H300",
		},
		{
			"row",
			"  - table: { x: 20, y: 40, w: 120, title: T, rows: [{ name: r, cells: [r] }] }\n  - box: { name: b, x: 300, y: 0, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 300, y: 130, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: r, to: b, fromSide: right, toSide: left }\n  - edge: { from: r, to: c, fromSide: right, toSide: left }\n",
			"H220 V15 H300",
		},
		{
			"a bar's straight edge holds the box's",
			"  - bar: { name: a, x1: 40, x2: 100, y: 50 }\n  - box: { name: b, x: 300, y: 20, w: 60, h: 60, rows: [b] }\n  - box: { name: c, x: 100, y: 200, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, fromSide: right, to: b, toSide: left }\n  - edge: { from: c, fromSide: right, to: b, toSide: left, crossing: true }\n",
			"edge a→b M100,50 H300",
		},
		{
			"dot",
			"  - dot: { name: a, cx: 150, cy: 30 }\n  - box: { name: b, x: 100, y: 100, w: 100, h: 30, rows: [b] }\n" +
				"  - edge: { from: a, to: b, toSide: top }\n  - edge: { from: b, to: a, fromSide: top, crossing: true }\n",
			"V100",
		},
		{
			"a round box's corner",
			"  - box: { name: a, x: 100, y: 20, w: 90, h: 60, shape: state, rows: [a] }\n" +
				"  - box: { name: b, x: 300, y: 0, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 300, y: 40, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 300, y: 90, w: 60, h: 30, rows: [d] }\n  - box: { name: e, x: 300, y: 130, w: 60, h: 30, rows: [e] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n  - edge: { from: a, to: d, fromSide: right, toSide: left }\n  - edge: { from: a, to: e, fromSide: right, toSide: left }\n",
			"edge a→b M185.91,32 H245 V15 H300\nedge a→c M189.92,44 H245 V55 H300\nedge a→d M189.92,56 H245 V105 H300\nedge a→e M185.91,68 H245 V145 H300",
		},
		{
			"a hexagon's slant",
			"  - box: { name: a, x: 100, y: 20, w: 120, h: 60, shape: hex, rows: [a] }\n" +
				"  - box: { name: b, x: 300, y: 0, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 300, y: 80, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n",
			"edge a→b M",
		},
		{
			"a low box's round corners keep the parts, and one line between",
			"  - box: { name: a, x: 40, y: 20, w: 100, h: 30, rows: [a] }\n" +
				"  - box: { name: b, x: 300, y: 0, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 300, y: 50, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 300, y: 100, w: 60, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n  - edge: { from: a, to: d, fromSide: right, toSide: left }\n",
			"edge a→b M139.98,27.5 H220 V15 H300\nedge a→c M140,35 H220 V65 H300\nedge a→d M139.98,42.5 H220 V115 H300",
		},
		{
			"a hexagon keeps one line between",
			"  - box: { name: a, x: 40, y: 150, w: 100, h: 60, shape: hex, rows: [a] }\n" +
				"  - box: { name: b, x: 300, y: 100, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 300, y: 200, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 300, y: 300, w: 60, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n  - edge: { from: a, to: d, fromSide: right, toSide: left }\n",
			"H220 V115 H300",
		},
		{
			"a frame's round corner",
			"  - frame: { name: f, x: 100, y: 20, w: 200, h: 24, label: F }\n  - box: { name: b, x: 400, y: 0, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 400, y: 60, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: f, to: b, fromSide: right, toSide: left }\n  - edge: { from: f, to: c, fromSide: right, toSide: left }\n",
			"edge f→b M299.8,28",
		},
		{
			"an icon's bottom, under the label",
			"  - icon: { name: a, x: 100, y: 20, icon: aws/aws-lambda, label: a long label }\n  - box: { name: b, x: 20, y: 200, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 200, y: 200, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: a, to: c, fromSide: bottom, toSide: top }\n",
			"edge a→b M116,89",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, told := edgesOf(t, c.parts)
			if told != "" || !strings.Contains(got, c.want) {
				t.Errorf("drew\n%s\ntold %q, want %s", got, told, c.want)
			}
		})
	}
}
