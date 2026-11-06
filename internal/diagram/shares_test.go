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
			"edge a→b M105,49 V120\nedge c→b M195,49 V120\nedge [150,60]→b M150,60 V120",
		},
		{
			"alone with a point it stays in the middle", boxes + "  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: [150, 60], to: b, toSide: top }\n",
			"edge a→b M105,49 V120\nedge [150,60]→b M150,60 V120",
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
			"a pill's bottom keeps one line between",
			"  - box: { name: a, x: 120, y: 20, w: 60, h: 40, shape: pill, rows: [a] }\n" +
				"  - box: { name: b, x: 0, y: 120, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 120, y: 120, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 240, y: 120, w: 60, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: a, to: c, fromSide: bottom, toSide: top }\n  - edge: { from: a, to: d, fromSide: bottom, toSide: top }\n",
			"V90 H30 V120",
		},
		{
			"round the side of a low box, on one line",
			"  - box: { name: a, x: 40, y: 20, w: 200, h: 30, rows: [a] }\n" +
				"  - box: { name: b, x: 40, y: 80, w: 60, h: 30, rows: [b] }\n  - box: { name: c, x: 40, y: 130, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 40, y: 180, w: 60, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: right }\n  - edge: { from: a, to: c, fromSide: right, toSide: right }\n  - edge: { from: a, to: d, fromSide: right, toSide: right }\n",
			"edge a→b M239.98,27.5 H260 V95 H100",
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

// An edge between facing sides that share a height runs straight when nothing is in its way, at the point of a side, or
// the middle of the height; when something is, it stays as it was.
func TestEdgeStraight(t *testing.T) {
	t.Parallel()

	const (
		ab = "  - box: { name: a, x: 100, y: 40, w: 80, h: 60, rows: [a] }\n  - box: { name: b, x: 300, y: 50, w: 80, h: 60, rows: [b] }\n"
		e  = "  - edge: { from: a, to: b }\n"
	)
	for _, c := range []struct{ name, parts, want string }{
		{
			"a hexagon", "  - box: { name: a, x: 40, y: 20, w: 100, h: 60, shape: hex, rows: [a] }\n  - box: { name: c, x: 300, y: 40, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, to: c, fromSide: right, toSide: left }\n",
			"edge a→c M137,55 H300",
		},
		{
			"the ends beside it stay", "  - box: { name: a, x: 40, y: 80, w: 100, h: 60, shape: hex, rows: [a] }\n  - box: { name: b, x: 300, y: 20, w: 60, h: 30, rows: [b] }\n" +
				"  - box: { name: c, x: 300, y: 100, w: 60, h: 30, rows: [c] }\n  - box: { name: d, x: 300, y: 180, w: 60, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: c, fromSide: right, toSide: left }\n  - edge: { from: a, to: d, fromSide: right, toSide: left }\n",
			"edge a→b M131,95 H220 V35 H300\nedge a→c M137,115 H300\nedge a→d M131,125 H220 V195 H300",
		},
		{"down", "  - box: { name: a, x: 40, y: 20, w: 120, h: 30, rows: [a] }\n  - box: { name: b, x: 90, y: 110, w: 120, h: 30, rows: [b] }\n  - edge: { from: a, to: b }\n", "edge a→b M125,50 V110"},
		{"under a wide box", "  - box: { name: a, x: 40, y: 20, w: 300, h: 30, rows: [a] }\n  - box: { name: b, x: 60, y: 110, w: 60, h: 30, rows: [b] }\n  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n", "edge a→b M90,50 V110"},
		{
			"off an edge at points", "  - box: { name: a, x: 130, y: 20, w: 100, h: 30, rows: [a] }\n  - box: { name: b, x: 100, y: 120, w: 100, h: 30, rows: [b] }\n  - edge: { from: a, to: b }\n  - edge: { from: [180, 90], to: [180, 110] }\n",
			"edge a→b M150,50 V120\nedge [180,90]→[180,110] M180,90 V110",
		},
		{"off a line", ab + "  - line: { x1: 250, y1: 70, x2: 290, y2: 70 }\n" + e, "edge a→b M180,80 H300"},
		{"off an axis", ab + "  - axis: { x1: 200, x2: 280, y: 70, ticks: [] }\n" + e, "edge a→b M180,80 H300"},
		{"off a frame", ab + "  - frame: { x: 200, y: 70, w: 80, h: 200 }\n" + e, "edge a→b M180,80 H300"},
		{"off a dot", ab + "  - dot: { cx: 250, cy: 70, r: 3 }\n" + e, "edge a→b M180,80 H300"},
		{"off its own label", ab + "  - edge: { from: a, to: b, label: sync, at: [270, 68] }\n", "edge a→b M180,80 H300"},
		{
			"not across an edge written to cross", ab + e + "  - edge: { from: [250, 20], to: [250, 72], crossing: true }\n",
			"edge a→b M180,80 H300\nedge [250,20]→[250,72] M250,20 V72",
		},
		{
			"on from a height along a box", "  - box: { name: a, x: 560, y: 60, w: 100, h: 50, rows: [a] }\n  - box: { name: c, x: 200, y: 70, w: 60, h: 50, rows: [c] }\n  - box: { name: x, x: 280, y: 20, w: 100, h: 64, rows: [x] }\n" +
				"  - edge: { from: a, to: c, fromSide: left, toSide: right }\n",
			"edge a→c M560,95 H260",
		},
		{
			"not onto the end of an edge", ab + e + "  - edge: { from: [250, 20], to: [250, 70] }\n",
			"edge a→b M180,80 H300\nedge [250,20]→[250,70] M250,20 V70",
		},
		{
			"not onto the end of a slanting edge", ab + e + "  - edge: { from: [230, 20], to: [300, 70], straight: true }\n",
			"edge a→b M180,80 H300\nedge [230,20]→[300,70] M230,20 L300,70",
		},
		{
			"through a box, and another edge straight", "  - box: { name: a, x: 100, y: 20, w: 90, h: 30, rows: [a] }\n  - box: { name: b, x: 60, y: 200, w: 90, h: 30, rows: [b] }\n  - box: { x: 108, y: 140, w: 41, h: 29, rows: [x] }\n" +
				"  - box: { name: c, x: 400, y: 20, w: 120, h: 30, rows: [c] }\n  - box: { name: d, x: 450, y: 110, w: 120, h: 30, rows: [d] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: c, to: d }\n",
			"edge a→b M145,50 V125 H105 V200\nedge c→d M485,50 V110",
		},
		{"sharing too little", "  - box: { name: a, x: 40, y: 20, w: 100, h: 30, rows: [a] }\n  - box: { name: b, x: 120, y: 110, w: 100, h: 30, rows: [b] }\n  - edge: { from: a, to: b }\n", "edge a→b M90,50 V80 H170 V110"},
		{
			"through a box", "  - box: { name: a, x: 100, y: 20, w: 90, h: 30, rows: [a] }\n  - box: { name: b, x: 60, y: 200, w: 90, h: 30, rows: [b] }\n  - box: { name: c, x: 200, y: 200, w: 90, h: 30, rows: [c] }\n  - box: { x: 110, y: 150, w: 41, h: 29, rows: [x] }\n" +
				"  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n  - edge: { from: a, to: c, fromSide: bottom, toSide: top }\n",
			"edge a→b M130,50 V125 H105 V200\nedge a→c M160,50 V125 H245 V200",
		},
		{
			"two facing edges too close", "  - box: { name: a, x: 40, y: 20, w: 60, h: 40, rows: [a] }\n  - box: { name: b, x: 240, y: 28, w: 60, h: 40, rows: [b] }\n" +
				"  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: b, to: a, fromSide: left, toSide: right }\n",
			"edge a→b M100,33.33 H170 V41.33 H240\nedge b→a M240,54.67 H170 V46.67 H100",
		},
		{
			"one straight already stays", "  - box: { name: p, x: 200, y: 85, w: 200, h: 59, rows: [p] }\n  - box: { name: q, x: 180, y: 210, w: 120, h: 89, rows: [q] }\n  - box: { name: r, x: 420, y: 210, w: 160, h: 29, rows: [r] }\n" +
				"  - edge: { from: p, to: r }\n  - edge: { from: r, to: p }\n  - edge: { from: q, to: r }\n",
			"edge p→r M400,104.67 H410 V217.25 H420.04\nedge r→p M420,224.5 H410 V124.33 H400\nedge q→r M300,231.75 H420.04",
		},
		{
			"five between two boxes", "  - box: { name: a, x: 80, y: 255, w: 80, h: 89, shape: state, rows: [a] }\n  - box: { name: b, x: 300, y: 225, w: 80, h: 59, shape: pill, rows: [b] }\n" +
				"  - edge: { from: b, to: a }\n  - edge: { from: b, to: a }\n  - edge: { from: a, to: b }\n  - edge: { from: a, to: b }\n  - edge: { from: b, to: a }\n",
			"edge b→a M307.51,234.83 H230 V269.83 H157.48\nedge b→a M301.69,244.67 H230 V284.67 H160\nedge a→b M160,299.5 H230 V254.5 H300\nedge a→b M160,314.33 H230 V264.33 H301.69\nedge b→a M307.51,274.17 H230 V329.17 H157.48",
		},
		{
			"touching boxes, the line too short", "  - box: { name: a, x: 300, y: 280, w: 100, h: 80, shape: state, rows: [a] }\n  - box: { name: b, x: 400, y: 240, w: 100, h: 100, shape: state, rows: [b] }\n" +
				strings.Repeat("  - edge: { from: a, to: b, fromSide: right, toSide: left }\n", 3),
			"edge a→b M399.3,300 H400 V265 H400.02\nedge a→b M400,320 V290\nedge a→b M399.3,340 H400 V315 H400.02",
		},
		{
			"at the point of the side with more ends", "  - box: { name: a, x: 40, y: 60, w: 100, h: 80, rows: [a] }\n  - box: { name: b, x: 300, y: 80, w: 100, h: 80, rows: [b] }\n  - box: { name: c, x: 40, y: 200, w: 100, h: 100, rows: [c] }\n" +
				"  - edge: { from: c, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: b, fromSide: right, toSide: left }\n  - edge: { from: a, to: b, fromSide: right, toSide: left }\n",
			"edge c→b M140,250 H220 V140 H300\nedge a→b M140,100 H300\nedge a→b M140,120 H300",
		},
		{"to a bar", "  - box: { name: a, x: 130, y: 20, w: 100, h: 30, rows: [a] }\n  - bar: { name: r, x1: 40, x2: 400, y: 120 }\n  - edge: { from: a, to: r }\n", "edge a→r M180,50 V81.5 H220 V113"},
		{
			"given up, a label in the way", "  - box: { name: l, x: 40, y: 255, w: 80, h: 29, shape: state, rows: [l] }\n  - box: { name: t, x: 560, y: 85, w: 160, h: 29, rows: [t] }\n  - box: { name: r, x: 580, y: 255, w: 200, h: 59, shape: state, rows: [r] }\n" +
				"  - edge: { from: l, to: t }\n  - edge: { from: r, to: t }\n  - edge: { from: r, to: t, label: wwwwww }\n  - edge: { from: l, to: r }\n",
			"edge l→t M119.17,264.67 H340 V99.5 H560\nedge r→t M646.67,255 V184.5 H613.33 V114\nedge r→t M713.33,255 V184.5 H666.67 V114\nedge l→r M119.17,274.33 H580.87",
		},
		{
			"given up, one line by a part left", "  - box: { name: a, x: 100, y: 20, w: 80, h: 80, rows: [a] }\n  - box: { name: b, x: 420, y: 42, w: 80, h: 83, rows: [b] }\n  - box: { name: k, x: 270, y: 20, w: 60, h: 32, rows: [k] }\n  - box: { name: c, x: 200, y: 69, w: 50, h: 29, rows: [c] }\n" +
				"  - frame: { name: f, x: 340, y: 40, w: 300, h: 280 }\n  - edge: { from: a, to: b, label: ab }\n  - edge: { from: c, to: f }\n",
			"edge a→b M180,60 H300 V83.5 H420\nedge c→f M250,83.5 H295 V180 H340",
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

// An edge written by hand may run through the label of another with an end at one part, on one line from side to side
// of it; any other run through a label is told.
func TestEdgeSharedLabel(t *testing.T) {
	t.Parallel()
	const (
		abc = "  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }\n  - box: { name: b, x: 20, y: 120, w: 60, h: 30, rows: [b] }\n" +
			"  - box: { name: c, x: 300, y: 70, w: 60, h: 30, rows: [c] }\n"
		via   = "via: [[120, 135], [120, 85]]"
		ac    = abc + "  - edge: { from: a, to: c, fromSide: right, toSide: left, via: [[120, 35], [120, 85]], label: L, at: [200, 85] }\n"
		joins = ac + "  - edge: { from: b, to: c, fromSide: right, toSide: left, " + via + " }\n"
		told  = "runs through label \"L\""
		bc    = "edge b→c " + told
	)
	for _, c := range []struct {
		name, parts, want, refused string
	}{
		{name: "joining", parts: joins, want: "edge a→c M80,35 H120 V85 H300\nedge b→c M80,135 H120 V85 H300"},
		{
			name: "the label's edge has no via",
			parts: "  - box: { name: a, x: 20, y: 70, w: 60, h: 30, rows: [a] }\n  - box: { name: b, x: 20, y: 120, w: 60, h: 30, rows: [b] }\n" +
				"  - box: { name: c, x: 300, y: 70, w: 60, h: 30, rows: [c] }\n  - edge: { from: a, to: c, fromSide: right, toSide: left, label: L, at: [200, 85] }\n" +
				"  - edge: { from: b, to: c, fromSide: right, toSide: left, " + via + " }\n",
			want: "edge a→c M80,85 H300\nedge b→c M80,135 H120 V85 H300",
		},
		{
			name: "parting",
			parts: abc + "  - edge: { from: c, to: a, fromSide: left, toSide: right, via: [[120, 85], [120, 35]], label: L, at: [200, 85] }\n" +
				"  - edge: { from: c, to: b, fromSide: left, toSide: right, via: [[120, 85], [120, 135]] }\n",
			want: "edge c→a M300,85 H120 V35 H80\nedge c→b M300,85 H120 V135 H80",
		},
		{
			name: "down",
			parts: "  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }\n  - box: { name: b, x: 200, y: 20, w: 60, h: 30, rows: [b] }\n" +
				"  - box: { name: c, x: 110, y: 200, w: 60, h: 30, rows: [c] }\n  - edge: { from: a, to: c, fromSide: right, toSide: top, via: [[140, 35]], label: L, at: [140, 130] }\n" +
				"  - edge: { from: b, to: c, fromSide: left, toSide: top, via: [[140, 35]] }\n",
			want: "edge a→c M80,35 H140 V200\nedge b→c M200,35 H140 V200",
		},
		{
			name: "no via: they part, and meet no label",
			parts: abc + "  - edge: { from: a, to: c, fromSide: right, toSide: left, label: L, at: [200, 40] }\n" +
				"  - edge: { from: b, to: c, fromSide: right, toSide: left }\n",
			want: "edge a→c M80,35 H190 V80 H300\nedge b→c M80,135 H190 V90 H300",
		},
		{
			name: "two labels apart on one run",
			parts: abc + "  - edge: { from: a, to: c, fromSide: right, toSide: left, via: [[120, 35], [120, 85]], label: L, at: [160, 85] }\n" +
				"  - edge: { from: b, to: c, fromSide: right, toSide: left, " + via + ", label: M, at: [260, 85] }\n",
			refused: bc,
		},
		{name: "half off the line", parts: ac + "  - edge: { from: b, to: c, fromSide: right, toSide: left, via: [[120, 135], [120, 85.5], [280, 85.5], [280, 85]] }\n", refused: bc},
		{name: "an edge with no part in common", parts: abc + "  - box: { name: d, x: 300, y: 170, w: 60, h: 30, rows: [d] }\n  - edge: { from: a, to: c, fromSide: right, toSide: left, via: [[120, 35], [120, 85]], label: L, at: [200, 85] }\n  - edge: { from: b, to: d, fromSide: right, toSide: left, via: [[120, 135], [120, 85], [260, 85], [260, 185]] }\n", refused: "edge b→d " + told},
		{name: "ending at points", parts: ac + "  - edge: { from: [130, 85], to: [270, 85] }\n", refused: "edge [130,85]→[270,85] " + told},
		{
			name:    "points alike name no part",
			parts:   "  - edge: { from: [20, 85], to: [300, 85], label: L, at: [200, 85] }\n  - edge: { from: [30, 85], to: [290, 85] }\n",
			refused: "edge [30,85]→[290,85] " + told,
		},
		{
			name: "one going to the part the other comes from",
			parts: abc + "  - edge: { from: a, to: c, fromSide: right, toSide: left, via: [[120, 35], [120, 85]], label: L, at: [200, 85] }\n" +
				"  - edge: { from: c, to: b, fromSide: left, toSide: right, via: [[120, 85], [120, 135]] }\n",
			refused: "edge c→b " + told,
		},
		{
			name:  "ending off the label by the longest marker",
			parts: ac + "  - edge: { from: a, to: [233, 85], fromSide: right, via: [[120, 35], [120, 85]] }\n",
			want:  "edge a→c M80,35 H120 V85 H300\nedge a→[233,85] M80,35 H120 V85 H233",
		},
		{
			name: "turning beside the label",
			parts: abc + "  - edge: { from: a, to: c, fromSide: right, toSide: left, via: [[120, 35], [120, 85]], label: L, at: [130, 85] }\n" +
				"  - edge: { from: b, to: c, fromSide: right, toSide: left, " + via + " }\n",
			want: "edge a→c M80,35 H120 V85 H300\nedge b→c M80,135 H120 V85 H300",
		},
		{name: "ending short of the longest marker", parts: ac + "  - edge: { from: a, to: [232, 85], fromSide: right, via: [[120, 35], [120, 85]] }\n", refused: told},
		{name: "starting short of the longest marker", parts: ac + "  - edge: { from: [170, 85], to: c, toSide: left }\n", refused: told},
		{name: "its arrow part hidden", parts: ac + "  - edge: { from: a, to: [209, 85], fromSide: right, via: [[120, 35], [120, 85]] }\n", refused: told},
		{name: "ending just inside", parts: ac + "  - edge: { from: a, to: [208, 85], fromSide: right, via: [[120, 35], [120, 85]] }\n", refused: told},
		{name: "starting just inside", parts: ac + "  - edge: { from: [192, 85], to: c, toSide: left }\n", refused: told},
		{name: "slanting off the line", parts: ac + "  - edge: { from: [120, 85], to: c, toSide: bottom, straight: true }\n", refused: told},
		{name: "its arrow hidden", parts: ac + "  - edge: { from: a, to: [200, 85], fromSide: right, via: [[120, 35], [120, 85]] }\n", refused: told},
		{name: "its start hidden", parts: ac + "  - edge: { from: [200, 85], to: c, toSide: left }\n", refused: told},
		{name: "wholly hidden", parts: ac + "  - edge: { from: [195, 85], to: [205, 85] }\n", refused: told},
		{name: "turning in the label", parts: ac + "  - edge: { from: b, to: c, fromSide: right, toSide: left, via: [[200, 135], [200, 85]] }\n", refused: bc},
		{
			name: "slanting",
			parts: "  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }\n  - box: { name: c, x: 200, y: 200, w: 60, h: 30, rows: [c] }\n" +
				"  - edge: { from: a, to: c, straight: true, label: L }\n  - edge: { from: a, to: c, straight: true }\n",
			refused: "edge a→c " + told,
		},
		{
			name: "over a point of the label's edge's via",
			parts: abc + "  - edge: { from: a, to: c, fromSide: right, toSide: left, via: [[120, 35], [120, 85], [200, 85]], label: L, at: [200, 85] }\n" +
				"  - edge: { from: b, to: c, fromSide: right, toSide: left, " + via + " }\n",
			refused: bc,
		},
		{
			name: "the label's edge moved off",
			parts: abc + "  - box: { name: d, x: 20, y: 200, w: 60, h: 30, rows: [d] }\n  - edge: { from: a, to: c, fromSide: right, toSide: left, label: L, at: [200, 85] }\n" +
				"  - edge: { from: b, to: c, fromSide: right, toSide: left, " + via + " }\n  - edge: { from: d, to: c, fromSide: right, toSide: left }\n",
			refused: bc,
		},
		{
			name: "on a graph's edge",
			parts: "  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n" +
				"      edges:\n        - { from: a, to: b, label: L }\n  - edge: { from: a, to: b, fromSide: bottom, toSide: top }\n",
			refused: "edge a→b " + told,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, said := edgesOf(t, c.parts)
			if c.refused != "" {
				if !strings.Contains(said, c.refused) {
					t.Errorf("%s\n  told %q\n  want %q", c.parts, said, c.refused)
				}
				return
			}
			if got != c.want || said != "" {
				t.Errorf("%s\n  drew\n%s\n%s\n  want\n%s", c.parts, got, said, c.want)
			}
		})
	}
}
