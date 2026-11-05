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

// g2 is g1 with labels on the edges out of check, written from line 3 to 13: "yes" at 116,104 31.6x18, right of
// check→show; g2r is g2 laid out right, written to line 14, its "no" at 189,80 24.4x18, below check→write.
var (
	g2  = strings.NewReplacer("to: show }", `to: show, label: "yes" }`, "to: write }", `to: write, label: "no" }`).Replace(g1)
	g2r = strings.Replace(g2, "      y: 20\n", "      y: 20\n      direction: right\n", 1)
)

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
		{"a name no box has", strings.Replace(g1, "to: show", "to: shwo", 1), `s.yaml:12:30: edge: no box or group of this graph named "shwo"`},
		{
			"a name no box has, and an edge by hand to a box laid out",
			strings.Replace(g1, "to: show", "to: shwo", 1) + "  - edge: { from: show, to: [400, 200] }\n",
			`s.yaml:12:30: edge: no box or group of this graph named "shwo"`,
		},
		{
			"a box outside the graph",
			strings.Replace(g1, "to: show", "to: note", 1) + "  - box: { name: note, x: 400, y: 20, rows: [N] }\n",
			`s.yaml:12:30: edge: no box or group of this graph named "note"`,
		},
		{
			"a name given twice",
			strings.Replace(g1, "      edges:", "        - box: { name: write, rows: [Again] }\n      edges:", 1),
			`s.yaml:10:18: the name "write" is given before: give each part, row and node its own name`,
		},
		{"a label of a number", strings.Replace(g1, "to: check }", "to: check, label: 3 }", 1), `s.yaml:11:44: got number, want string or object`},
		{"a diamond", strings.Replace(g1, "to: check }", "to: check, end: diamond }", 1), `s.yaml:11:42: value must be one of 'none', 'arrow'`},
		{"x past a million", strings.Replace(g1, "x: 20\n", "x: 1000001\n", 1), `s.yaml:4:10: maximum: got 1.000001×10⁰⁶, want 1×10⁰⁶`},
		{"up", strings.Replace(g1, "y: 20\n", "y: 20\n      direction: up\n", 1), `s.yaml:6:18: value must be one of 'down', 'right'`},
		{"a box placed", strings.Replace(g1, "{ name: show,", "{ name: show, x: 10,", 1), `s.yaml:9:18: additional properties 'x' not allowed`},
		{"201 boxes", many.String(), `s.yaml:7:9: maxItems: got 201, want 200`},
		{"4,001 ranks passed", long.String(), `s.yaml: graph: the edges of this diagram's graphs pass 4001 ranks between their ends, more than 4000: split the diagram`},
	} {
		if _, got := draw(t, c.src, inspect); got != c.want {
			t.Errorf("%s:\n  told %q\n  want %q", c.name, got, c.want)
		}
	}
}

// gg is a graph with a group, a group nested in it, and a box outside.
const gg = `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: cli, rows: [CLI] }
        - group:
            name: server
            label: Server
            nodes:
              - box: { name: api, rows: [API] }
              - group:
                  name: data
                  nodes:
                    - box: { name: store, rows: [Store] }
        - box: { name: x, rows: [X] }
      edges:
        - { from: cli, to: api }
        - { from: api, to: store }
`

// lanes is a graph of two lanes, a group nested in the second.
const lanes = `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - name: user
          nodes:
            - box: { name: ask, rows: [Ask] }
        - name: dev
          nodes:
            - box: { name: build, rows: [Build] }
            - group: { name: tools, nodes: [{ box: { name: ci, rows: [CI] } }] }
      edges:
        - { from: ask, to: build }
`

// The mistakes of groups and edges to them: an empty one, a name of none, an edge between a group and what it holds,
// one to a group whose other end stands among its ranks, a name given twice, too many nodes through groups, an edge
// at a lane, a box among the lanes and nodes beside them.
func TestGraphGroupMistakes(t *testing.T) {
	t.Parallel()

	edge := func(e string) string {
		return strings.Replace(gg, "        - { from: api, to: store }\n", "        - { from: api, to: store }\n        - "+e+"\n", 1)
	}
	lane := func(e string) string {
		return strings.Replace(lanes, "        - { from: ask, to: build }\n", "        - { from: ask, to: build }\n        - "+e+"\n", 1)
	}
	var many strings.Builder
	many.WriteString("title: T\nparts:\n  - graph:\n      x: 0\n      y: 0\n      nodes:\n")
	for g := range 2 {
		fmt.Fprintf(&many, "        - group:\n            nodes:\n")
		for i := range 100 {
			fmt.Fprintf(&many, "              - box: { name: b%d_%d, rows: [B] }\n", g, i)
		}
	}
	tangled := `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group: { name: g, nodes: [{ box: { name: g1, rows: [G1] } }, { box: { name: g2, rows: [G2] } }] }
        - group: { name: h, nodes: [{ box: { name: h1, rows: [H1] } }] }
      edges:
        - { from: g1, to: h1 }
        - { from: h1, to: g2 }
        - { from: h, to: g }
`
	for _, c := range []struct {
		name, src, want string
	}{
		{"an empty group", strings.Replace(strings.Replace(gg, "                  nodes:\n                    - box: { name: store, rows: [Store] }\n", "                  nodes: []\n", 1), "        - { from: api, to: store }\n", "", 1), `s.yaml:15:26: minItems: got 0, want 1`},
		{"a name of none", edge("{ from: cli, to: servr }"), `s.yaml:21:28: edge: no box or group of this graph named "servr"`},
		{"from a box the group holds", edge("{ from: api, to: server }"), `s.yaml:21:19: edge api→server: both ends are group "server" or inside it; end the edge at a box or another group outside "server"`},
		{"to a box the group holds", edge("{ from: server, to: store }"), `s.yaml:21:31: edge server→store: both ends are group "server" or inside it; end the edge at a box or another group outside "server"`},
		{"from a group to itself", edge("{ from: server, to: server }"), `s.yaml:21:31: edge server→server: both ends are group "server" or inside it; end the edge at a box or another group outside "server"`},
		{"to a group nested in it", edge("{ from: server, to: data }"), `s.yaml:21:31: edge server→data: both ends are group "server" or inside it; end the edge at a box or another group outside "server"`},
		{"a box among the group's ranks", edge("{ from: api, to: x }\n        - { from: x, to: store }\n        - { from: x, to: server }"), `s.yaml:23:19: edge x→server: x can stand neither above group "server" nor below it, as api stands no lower than it and store no higher; end the edge at a box in "server"`},
		{"to a box among the group's ranks", edge("{ from: api, to: x }\n        - { from: x, to: store }\n        - { from: server, to: x }"), `s.yaml:23:31: edge server→x: x can stand neither above group "server" nor below it, as api stands no lower than it and store no higher; end the edge at a box in "server"`},
		{"a group among another's ranks", tangled, `s.yaml:12:19: edge h→g: h can stand neither above group "g" nor below it, as g1 stands no lower than it and g2 no higher; end the edge at a box in "g"`},
		{"a group named as a box before it", strings.Replace(gg, "name: server", "name: cli", 1), `s.yaml:9:13: the name "cli" is given before: give each part, row and node its own name`},
		{"a box named as a group before it", strings.Replace(gg, "name: x,", "name: server,", 1), `s.yaml:17:18: the name "server" is given before: give each part, row and node its own name`},
		{"202 boxes and groups", many.String(), `s.yaml:4:7: graph: 202 boxes and groups, past 200; split it into graphs`},
		{"from a lane", lane("{ from: user, to: build }"), `s.yaml:16:19: edge user→build: user is a lane, from end to end of the graph; end the edge at a box or a group in it`},
		{"to a lane", lane("{ from: ask, to: dev }"), `s.yaml:16:28: edge ask→dev: dev is a lane, from end to end of the graph; end the edge at a box or a group in it`},
		{"from a lane to a lane", lane("{ from: user, to: dev }"), `s.yaml:16:19: edge user→dev: user is a lane, from end to end of the graph; end the edge at a box or a group in it`},
		{"from a lane to a box it holds", lane("{ from: dev, to: build }"), `s.yaml:16:28: edge dev→build: both ends are group "dev" or inside it; end the edge at a box or another group outside "dev"`},
		{"a box among the lanes", strings.Replace(lanes, "      lanes:\n", "      lanes:\n        - box: { name: z, rows: [Z] }\n", 1), "s.yaml:7:11: additional properties 'box' not allowed\ns.yaml:7:11: missing property 'nodes'"},
		{"to a group in a lane", lane("{ from: ask, to: tools }"), ""},
		{"nodes beside lanes", strings.Replace(lanes, "      edges:\n", "      nodes:\n        - box: { name: z, rows: [Z] }\n      edges:\n", 1), `s.yaml:15:9: false schema`},
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
		{"a box over a label of the graph", g2r, "  - box: { name: note, x: 176, y: 86, rows: [N] }\n", `box "note" overlaps label "no"`},
		{"an edge by hand through a label of the graph", g2, "  - edge: { from: [110, 113], to: [160, 113] }\n", `edge [110,113]→[160,113] runs through label "yes"`},
		{
			"a box in the graph's channel", k33, "  - box: { name: note, x: 24, y: 78, rows: [N] }\n",
			`edge a→x runs through box "note"` + "\n@: " + `edge a→y runs through box "note"` + "\n@: " + `edge b→x runs through box "note"`,
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

	// A box of one graph over a box and a label of another is told at the graph of the first box, to move a graph.
	second := strings.NewReplacer("x: 20", "x: 132", "y: 20", "y: 104", "write", "write2", "check", "check2", "show", "show2").
		Replace(strings.TrimPrefix(g2, "title: T\nparts:\n"))
	if _, got := draw(t, g2+second, inspect); got != `s.yaml:4:7: box "show" overlaps box "write2" of another graph; move one of the graphs by its x and y`+"\n"+
		`s.yaml:15:7: box "write2" overlaps label "yes" of another graph; move one of the graphs by its x and y` {
		t.Errorf("two graphs: told %q", got)
	}

	// Off the canvas, what a graph laid out is told at the graph, to move it.
	_, got := draw(t, "width: 100\n"+g1, inspect)
	told := strings.Split(got, "\n")
	if !slices.Contains(told, `s.yaml:5:7: box "write" sticks out of the 100x175 canvas; move its graph by its x and y, or widen the canvas by its width and height (or leave them out)`) ||
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

// level3 is a graph of three lanes and level edges written from line 3 to line 16 of s.yaml: a and b ask and answer
// across a lane, and d hands over to ok across one, each labelled.
const level3 = `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, label: U, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: ok, rows: [OK] } }] }
        - { name: v, label: V, nodes: [{ box: { name: b, rows: [B] } }, { box: { name: q, rows: [Q] } }] }
        - { name: w, label: W, nodes: [{ box: { name: c, rows: [C] } }, { box: { name: d, rows: [D] } }] }
      edges:
        - { from: c, to: b }
        - { from: b, to: a, level: true, label: ask }
        - { from: a, to: b, level: true, label: answer }
        - { from: b, to: d }
        - { from: d, to: ok, level: true, label: ask }
`

// A level edge stands its two boxes in one rank and runs straight between their facing sides, half a column off the
// middle of the shorter side across the ranks, a column apart from another between the same boxes: laid out down
// and right, its boxes in two lanes or in one group (a block, side by side as written however the order would part
// them), and the shorter box grown for three, laid out right by its width.
func TestGraphLevel(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, src, want string
	}{
		{"lanes, down", level3, `frame "u" 20,20 144x201
frame "v" 188,20 108x201
frame "w" 320,20 108x201
the heading of frame "u" 32,29 7.68x18
the heading of frame "v" 200,29 7.68x18
the heading of frame "w" 332,29 7.68x18
box "a" 32,115 48x29
box "ok" 32,180 56x29
box "b" 236,115 48x29
box "q" 236,50 48x29
box "c" 368,50 48x29
box "d" 332,180 48x29
edge c→b M392,79 V91 H260 V115
edge b→a M236,123.5 H80
edge a→b M80,135.5 H236
edge b→d M260,144 V156 H356 V180
edge d→ok M332,200.5 H88
label "ask" 202.4,93.5 31.6x18
label "answer" 91,147.5 53.2x18
label "ask" 262.4,170.5 31.6x18
`},
		{"lanes, right", strings.Replace(level3, "      lanes:\n", "      direction: right\n      lanes:\n", 1), `frame "u" 20,20 266x54
frame "v" 20,98 266x96
frame "w" 20,218 266x114
the heading of frame "u" 32,29 7.68x18
the heading of frame "v" 32,107 7.68x18
the heading of frame "w" 32,227 7.68x18
box "a" 134,32 48x29
box "ok" 218,32 56x29
box "b" 134,134 48x48
box "q" 50,152 48x29
box "c" 50,272 48x48
box "d" 222,230 48x48
edge c→b M98,296 H110 V158 H134
edge b→a M152,134 V61
edge a→b M164,61 V134
edge b→d M182,158 H194 V254 H222
edge d→ok M252,230 V61
label "ask" 108.4,114 31.6x18
label "answer" 176,75 53.2x18
label "ask" 264,198 31.6x18
`},
		{"a block written apart by the order", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: x, rows: [X] }
        - box: { name: y, rows: [Y] }
        - box: { name: a, rows: [A] }
        - box: { name: p, rows: [P] }
        - box: { name: b, rows: [B] }
      edges:
        - { from: x, to: b }
        - { from: y, to: a }
        - { from: x, to: p }
        - { from: a, to: b, level: true }
`, `box "x" 20,20 64x29
box "y" 110,20 48x29
box "a" 92,85 48x29
box "p" 20,85 48x29
box "b" 164,85 48x29
edge x→b M62.67,49 V61 H188 V85
edge y→a M128,49 V85
edge x→p M41.33,49 V85
edge a→b M140,105.5 H164
`},
		{"four, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [Bbbbbbbbbbbbbbbbbbbbbbbbbbb] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: a, level: true }
        - { from: a, to: b, level: true }
        - { from: b, to: a, level: true }
`, `box "a" 128,20 60x29
box "b" 20,74 276x29
edge a→b M140,49 V74
edge b→a M152,74 V49
edge a→b M164,49 V74
edge b→a M176,74 V49
`},
		{"three, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [Bbbbbbbbbbbbbbbbbbbbbbbbbbb] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: a, level: true }
        - { from: a, to: b, level: true }
`, `box "a" 128,20 60x29
box "b" 20,74 276x29
edge a→b M152,49 V74
edge b→a M164,74 V49
edge a→b M176,49 V74
`},
	} {
		if got, told := draw(t, c.src, inspect); got != c.want || told != "" {
			t.Errorf("%s:\n%s%s\nwant\n%s", c.name, got, told, c.want)
		}
	}
}

// A level edge is refused at a group, between boxes of no one group nor right in two lanes, when an edge joins two
// boxes of its row, when something stands between its boxes in their rank, when it takes the side of its box's
// loops, and when it runs through a lane's heading laid out right; a box among a group's ranks is told by ranks.
// A level edge refused for loops makes no row, so an edge between its boxes is not told too. The rest lay out: a
// row of one lane however it faces, level on a loop (a loop), loops on the other side, a cycle through a row, three
// edges between two boxes, and headings that do not reach the edge.
func TestGraphLevelMistakes(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, src, want string
	}{
		{"a level edge at a group", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - group: { name: g, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: g, level: true }
`, `s.yaml:10:26: edge a→g: g is a group, and level stands two boxes in one rank; end the edge at a box, or drop level`},
		{"a level edge between a group's box and one outside", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - group: { name: g, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true }
`, `s.yaml:10:36: edge a→b: level joins two boxes of one group, or two boxes directly in two lanes; move one of them, or drop level`},
		{"an edge across a row", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
      edges:
        - { from: a, to: b, level: true }
        - { from: a, to: b }
`, `s.yaml:11:13: edge a→b: a and b stand in one rank by level edges, so this edge cannot run from one rank to another; end it elsewhere, or drop level from the edges joining them`},
		{"a box of the block written between", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
        - box: { name: c, rows: [C] }
      edges:
        - { from: a, to: c, level: true }
        - { from: a, to: b, level: true }
`, `s.yaml:11:36: edge a→c: box "b" stands between its boxes in their rank; drop level, or move it out of the way`},
		{"a box of the block on the side it faces", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, nodes: [{ box: { name: a, rows: [A] } }] }
        - { name: v, nodes: [{ box: { name: c, rows: [C] } }, { box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: c, level: true }
`, `s.yaml:10:36: edge a→b: box "c" stands between its boxes in their rank; drop level, or move it out of the way`},
		{"a group of a lane whose block faces both ways", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, nodes: [{ box: { name: s, rows: [S] } }, { box: { name: a, rows: [A] } }, { box: { name: x, rows: [X] } }] }
        - { name: v, nodes: [{ box: { name: b, rows: [B] } }, { group: { name: h, nodes: [{ box: { name: m1, rows: [M1] } }, { box: { name: m2, rows: [M2] } }] } }] }
        - { name: e, nodes: [{ box: { name: c, rows: [C] } }] }
      edges:
        - { from: s, to: a }
        - { from: m1, to: x }
        - { from: x, to: m2 }
        - { from: a, to: b, level: true }
        - { from: b, to: c, level: true }
`, `s.yaml:15:36: edge b→c: group "h" stands between its boxes in their rank; drop level, or move it out of the way`},
		{"a box of a lane between", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, nodes: [{ box: { name: a, rows: [A] } }] }
        - { name: v, nodes: [{ box: { name: m, rows: [M] } }] }
        - { name: e, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true }
`, `s.yaml:11:36: edge a→b: box "m" stands between its boxes in their rank; drop level, or move it out of the way`},
		{"a group's frame in a lane between", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, nodes: [{ box: { name: s, rows: [S] } }, { box: { name: a, rows: [A] } }, { box: { name: x, rows: [X] } }] }
        - { name: v, nodes: [{ group: { name: h, label: H, nodes: [{ box: { name: m1, rows: [M1] } }, { box: { name: m2, rows: [M2] } }] } }] }
        - { name: e, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: s, to: a }
        - { from: a, to: x }
        - { from: m1, to: x }
        - { from: x, to: m2 }
        - { from: a, to: b, level: true }
`, `s.yaml:15:36: edge a→b: group "h" stands between its boxes in their rank; drop level, or move it out of the way`},
		{"two handoffs between two lanes in a rank", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, nodes: [{ box: { name: p, rows: [P] } }, { box: { name: q, rows: [Q] } }] }
        - { name: v, nodes: [{ box: { name: r, rows: [R] } }, { box: { name: s, rows: [S] } }] }
      edges:
        - { from: p, to: r, level: true }
        - { from: q, to: s, level: true }
`, `s.yaml:10:36: edge p→r: box "q" stands between its boxes in their rank; drop level, or move it out of the way
s.yaml:11:36: edge q→s: box "p" stands between its boxes in their rank; drop level, or move it out of the way`},
		{"loops on the side the edge takes, from", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
      edges:
        - { from: a, to: b, level: true }
        - { from: a, to: a }
`, `s.yaml:10:36: edge a→b: it takes side right of a, where a's loops are; drop level or the loops`},
		{"loops on the side the edge takes, to", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
      edges:
        - { from: b, to: a, level: true }
        - { from: a, to: a }
`, `s.yaml:10:36: edge b→a: it takes side right of a, where a's loops are; drop level or the loops`},
		{"a lane's heading between, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      lanes:
        - { name: u, label: U, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: z, rows: [Z] } }] }
        - { name: v, label: "A long heading of this lane", nodes: [{ box: { name: m, rows: [M] } }] }
        - { name: e, label: E, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: z, to: m }
        - { from: a, to: b, level: true }
`, `s.yaml:13:36: edge a→b: it runs through the heading of lane "v", which reaches into the first rank laid out right; drop level, or shorten the heading`},
		{"the heading of the lane the edge enters, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      lanes:
        - { name: u, label: U, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: z, rows: [Z] } }] }
        - { name: v, label: V, nodes: [{ box: { name: m, rows: [M] } }] }
        - { name: e, label: "A long heading of this lane", nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: z, to: m }
        - { from: a, to: b, level: true }
`, `s.yaml:13:36: edge a→b: it runs through the heading of lane "e", which reaches into the first rank laid out right; drop level, or shorten the heading`},
		{"a box among a group's ranks by a row", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group: { name: g, nodes: [{ box: { name: p, rows: [P] } }, { box: { name: q, rows: [Q] } }] }
        - box: { name: x, rows: [X] }
        - box: { name: y, rows: [Y] }
      edges:
        - { from: x, to: y, level: true }
        - { from: y, to: p }
        - { from: q, to: x }
        - { from: x, to: g }
`, `s.yaml:14:19: edge x→g: x can stand neither above group "g" nor below it, as q stands no lower than it and p no higher; end the edge at a box in "g"`},
		{"loops, and an edge across the row of the level edge refused", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
      edges:
        - { from: a, to: a }
        - { from: a, to: b, level: true }
        - { from: a, to: b }
`, `s.yaml:11:36: edge a→b: it takes side right of a, where a's loops are; drop level or the loops`},
		{"a row of one lane", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: l, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: b, rows: [B] } }, { box: { name: c, rows: [C] } }] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: c, level: true }
`, ""},
		{"a row of one lane facing a lane before, more of the lane in its rank", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: m, nodes: [{ box: { name: c, rows: [C] } }] }
        - { name: l, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: b, rows: [B] } }, { box: { name: x, rows: [X] } }, { group: { name: g, nodes: [{ box: { name: g1, rows: [G1] } }] } }] }
      edges:
        - { from: a, to: b, level: true }
        - { from: c, to: a, level: true }
`, ""},
		{"level on a loop", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
      edges:
        - { from: a, to: a, level: true }
`, ""},
		{"loops on the other side", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: b }
`, ""},
		{"a cycle through a row", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
        - box: { name: c, rows: [C] }
      edges:
        - { from: a, to: b, level: true }
        - { from: a, to: c }
        - { from: c, to: b }
`, ""},
		{"three edges grow the shorter box", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - box: { name: b, rows: [B] }
        - box: { name: c, rows: [C] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: a, level: true }
        - { from: a, to: b, level: true, label: again }
        - { from: c, to: a }
`, ""},
		{"a short heading, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      lanes:
        - { name: u, label: U, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: z, rows: [Z] } }] }
        - { name: v, label: V, nodes: [{ box: { name: m, rows: [M] } }] }
        - { name: e, label: E, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: z, to: m }
        - { from: a, to: b, level: true }
`, ""},
		{"a long heading, laid out down", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, label: U, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: z, rows: [Z] } }] }
        - { name: v, label: "A long heading of this lane", nodes: [{ box: { name: m, rows: [M] } }] }
        - { name: e, label: E, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: z, to: m }
        - { from: a, to: b, level: true }
`, ""},
		{"a long heading of a group in a lane, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      lanes:
        - { name: u, label: U, nodes: [{ group: { name: g, label: "A long heading of a group", nodes: [{ box: { name: z, rows: [Z] } }] } }, { box: { name: a, rows: [A] } }] }
        - { name: v, label: V, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true }
`, ""},
		{"a long heading of the group of a row, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      nodes:
        - group: { name: g, label: "A long heading of a group", nodes: [{ box: { name: a, rows: [A] } }, { box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true, label: ab }
`, ""},
	} {
		if _, got := draw(t, c.src, inspect); got != c.want {
			t.Errorf("%s:\n  told %q\n  want %q", c.name, got, c.want)
		}
	}
}
