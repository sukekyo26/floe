package diagram_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// g1 is a graph of three boxes, an edge back up among its edges, written from line 3 to line 13 of s.yaml.
const g1 = `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: write, rows: [Write the data] }
        - box: { name: check, shape: hex, rows: ["Checks pass?"] }
        - box: { name: show, rows: [Show the page] }
      edges:
        - { from: write, to: check }
        - { from: check, to: show }
        - { from: check, to: write }
`

// A graph's mistakes are told in the words of the parts written by hand, at the graph's own values: an edge naming
// no box of its graph is dropped alone, and the limit of the ranks passed is told at the diagram's root.
func TestGraphMistakes(t *testing.T) {
	t.Parallel()

	var many strings.Builder
	many.WriteString("title: T\nparts:\n  - graph:\n      x: 0\n      y: 0\n      nodes:\n")
	for i := range 201 {
		fmt.Fprintf(&many, "        - box: { name: b%d, rows: [B] }\n", i)
	}
	// A chain of 42 boxes, and 100 edges from its first to its last, which pass 40 ranks each, and one more.
	var long strings.Builder
	long.WriteString("title: T\nparts:\n  - graph:\n      x: 0\n      y: 0\n      nodes:\n")
	for i := range 42 {
		fmt.Fprintf(&long, "        - box: { name: c%d, rows: [C] }\n", i)
	}
	long.WriteString("      edges:\n")
	for i := 1; i < 42; i++ {
		fmt.Fprintf(&long, "        - { from: c%d, to: c%d }\n", i-1, i)
	}
	for range 100 {
		long.WriteString("        - { from: c0, to: c41 }\n")
	}
	long.WriteString("        - { from: c39, to: c41 }\n")

	for _, c := range []struct {
		name, src, want string
	}{
		{"a name no box has", strings.Replace(g1, "to: show", "to: shwo", 1), `s.yaml:12:30: edge: no box of this graph named "shwo"`},
		{
			"a name no box has, and an edge by hand to a box laid out",
			strings.Replace(g1, "to: show", "to: shwo", 1) + "  - edge: { from: show, to: [400, 200] }\n",
			`s.yaml:12:30: edge: no box of this graph named "shwo"`,
		},
		{
			"a box outside the graph",
			strings.Replace(g1, "to: show", "to: note", 1) + "  - box: { name: note, x: 400, y: 20, rows: [N] }\n",
			`s.yaml:12:30: edge: no box of this graph named "note"`,
		},
		{
			"a name given twice",
			strings.Replace(g1, "      edges:", "        - box: { name: write, rows: [Again] }\n      edges:", 1),
			`s.yaml:10:18: the name "write" is given before: give each part, row and node its own name`,
		},
		{"an edge from a box to itself", strings.Replace(g1, "{ from: check, to: write }", "{ from: write, to: write }", 1), `s.yaml:13:13: edge write→write starts where it ends: end it at another part or point`},
		{"a label of a number", strings.Replace(g1, "to: check }", "to: check, label: 3 }", 1), `s.yaml:11:44: got number, want string or object`},
		{"a diamond", strings.Replace(g1, "to: check }", "to: check, end: diamond }", 1), `s.yaml:11:42: value must be one of 'none', 'arrow'`},
		{"up", strings.Replace(g1, "y: 20\n", "y: 20\n      direction: up\n", 1), `s.yaml:6:18: value must be one of 'down', 'right'`},
		{"a box placed", strings.Replace(g1, "{ name: show,", "{ name: show, x: 10,", 1), `s.yaml:9:18: additional properties 'x' not allowed`},
		{"201 boxes", many.String(), `s.yaml:7:9: maxItems: got 201, want 200`},
		{"4,001 ranks passed", long.String(), `s.yaml: graph: the edges of this diagram's graphs pass 4001 ranks between their ends, more than 4000: split the diagram (in a graph with an edge label, every edge passes about twice as many ranks)`},
	} {
		if _, got := draw(t, c.src, inspect); got != c.want {
			t.Errorf("%s:\n  told %q\n  want %q", c.name, got, c.want)
		}
	}
}

// k33 is a graph of three boxes over three, each edge to each, its channel four tracks tall.
const k33 = `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
        - box: { name: c, rows: [C] }
        - box: { name: x, rows: [X] }
        - box: { name: y, rows: [Y] }
        - box: { name: z, rows: [Z] }
      edges:
        - { from: a, to: x }
        - { from: a, to: y }
        - { from: a, to: z }
        - { from: b, to: x }
        - { from: b, to: y }
        - { from: b, to: z }
        - { from: c, to: x }
        - { from: c, to: y }
        - { from: c, to: z }
`

// A mistake between a thing a graph laid out and one written by hand is told at the one written by hand, which its
// writer can move, wherever the graph is written; two edges of different graphs crossing are told to move a graph.
func TestGraphBlame(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, graph, part, want string
	}{
		{"a box over a box of the graph", g1, "  - box: { name: note, x: 100, y: 0, rows: [N] }\n", `box "write" overlaps box "note"`},
		{"a frame a box of the graph straddles", g1, "  - frame: { name: f, x: 0, y: 0, w: 100, h: 40 }\n", `box "write" straddles the edge of frame "f"`},
		{"a label in the graph's channel", g1, "  - label: { text: L, x: 68, y: 61, anchor: middle }\n", `edge write→check runs through label "L"`},
		{
			"a box in the graph's channel", k33, "  - box: { name: note, x: 24, y: 78, rows: [N] }\n",
			`edge a→x runs through box "note"` + "\n@: " + `edge b→x runs through box "note"`,
		},
		{
			"an edge by hand across the graph's edges", g1, "  - edge: { from: [0, 62], to: [400, 62] }\n",
			"edge check→write crosses edge [0,62]→[400,62] at 140,62; move one of them, or add crossing: true to one if it cannot be avoided\n" +
				"@: edge write→check crosses edge [0,62]→[400,62] at 68,62; move one of them, or add crossing: true to one if it cannot be avoided",
		},
	} {
		lines := strings.Count(c.graph, "\n") + 1
		if _, got := draw(t, c.graph+c.part, inspect); got != strings.ReplaceAll(fmt.Sprintf("s.yaml:%d:5: ", lines)+c.want, "@", fmt.Sprintf("s.yaml:%d:5", lines)) {
			t.Errorf("%s:\n  told %q\n  want at line %d %q", c.name, got, lines, c.want)
		}
		// The part by hand first, at line 3, and the graph after it: told at the same part, its words in the order
		// written.
		first := "title: T\nparts:\n" + c.part + strings.TrimPrefix(c.graph, "title: T\nparts:\n")
		_, got := draw(t, first, inspect)
		told := strings.Split(got, "\n")
		if len(told) != strings.Count(c.want, "\n")+1 || slices.ContainsFunc(told, func(s string) bool { return !strings.HasPrefix(s, "s.yaml:3:5: ") }) {
			t.Errorf("%s, written first:\n  told %q\n  want each at line 3", c.name, got)
		}
	}

	// Off the canvas, what a graph laid out is told at the graph, to move it.
	_, got := draw(t, "width: 100\n"+g1, inspect)
	told := strings.Split(got, "\n")
	if !slices.Contains(told, `s.yaml:5:7: box "write" sticks out of the 100x175 canvas; move its graph by its x and y`) ||
		slices.ContainsFunc(told, func(s string) bool { return !strings.HasPrefix(s, "s.yaml:5:7: ") }) {
		t.Errorf("off the canvas: told %q", got)
	}
}

// The graphs kept in testdata/graph, failures of random graphs once and the graphs the browser measures, lay out
// with no mistake.
func TestGraphKept(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("testdata", "graph", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no graph kept: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, got := draw(t, string(src), inspect); got != "" {
			t.Errorf("%s: %s", f, got)
		}
	}
}
