package diagram_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"floe/internal/usererr"
	"floe/internal/yamlpos"
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

// g2 is g1 with labels on the edges out of check, written from line 3 to 13: "yes" at 88.2,132 31.6x18, on
// check→show, its background cutting it.
var g2 = strings.NewReplacer("to: show }", `to: show, label: "yes" }`, "to: write }", `to: write, label: "no" }`).Replace(g1)

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
		{"a name no box has", strings.Replace(g1, "to: show", "to: shwo", 1), `s.yaml:12:30: edge: no box, icon or group of this graph named "shwo"`},
		{
			"a name no box has, and an edge by hand to a box laid out",
			strings.Replace(g1, "to: show", "to: shwo", 1) + "  - edge: { from: show, to: [400, 200] }\n",
			`s.yaml:12:30: edge: no box, icon or group of this graph named "shwo"`,
		},
		{
			"a box outside the graph",
			strings.Replace(g1, "to: show", "to: note", 1) + "  - box: { name: note, x: 400, y: 20, rows: [N] }\n",
			`s.yaml:12:30: edge: no box, icon or group of this graph named "note"`,
		},
		{
			"a name given twice",
			strings.Replace(g1, "      edges:", "        - box: { name: write, rows: [Again] }\n      edges:", 1),
			`s.yaml:10:11: the name "write" is given before: give each part, row and node its own name`,
		},
		{"a label of a number", strings.Replace(g1, "to: check }", "to: check, label: 3 }", 1), `s.yaml:11:44: got number, want string or object`},
		{"a diamond", strings.Replace(g1, "to: check }", "to: check, end: diamond }", 1), `s.yaml:11:42: value must be one of 'none', 'arrow'`},
		{"x past a million", strings.Replace(g1, "x: 20\n", "x: 1000001\n", 1), `s.yaml:4:10: maximum: got 1.000001×10⁰⁶, want 1×10⁰⁶`},
		{"up", strings.Replace(g1, "y: 20\n", "y: 20\n      direction: up\n", 1), `s.yaml:6:18: value must be one of 'down', 'right'`},
		{"a box placed", strings.Replace(g1, "{ name: show,", "{ name: show, x: 10,", 1), `s.yaml:9:33: additional properties 'x' not allowed`},
		{"201 boxes", many.String(), `s.yaml:6:7: maxItems: got 201, want 200`},
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
	// 200 icons and the group holding 100 of them, and 101 lanes of a box each: both are counted, past 200.
	var icons, laneful strings.Builder
	icons.WriteString("title: T\nparts:\n  - graph:\n      x: 0\n      y: 0\n      nodes:\n")
	for i := range 100 {
		fmt.Fprintf(&icons, "        - icon: { name: i%d, icon: aws/amazon-rds, label: R }\n", i)
	}
	icons.WriteString("        - group:\n            nodes:\n")
	for i := range 100 {
		fmt.Fprintf(&icons, "              - icon: { name: j%d, icon: aws/amazon-rds, label: R }\n", i)
	}
	laneful.WriteString("title: T\nparts:\n  - graph:\n      x: 0\n      y: 0\n      lanes:\n")
	for i := range 101 {
		fmt.Fprintf(&laneful, "        - nodes:\n            - box: { name: b%d, rows: [B] }\n", i)
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
		{"an empty group", strings.Replace(strings.Replace(gg, "                  nodes:\n                    - box: { name: store, rows: [Store] }\n", "                  nodes: []\n", 1), "        - { from: api, to: store }\n", "", 1), `s.yaml:15:19: minItems: got 0, want 1`},
		{"a name of none", edge("{ from: cli, to: servr }"), `s.yaml:21:28: edge: no box, icon or group of this graph named "servr"`},
		{"from a box the group holds", edge("{ from: api, to: server }"), `s.yaml:21:19: edge "api"→"server": both ends are group "server" or inside it; end the edge at a box, an icon or another group outside "server"`},
		{"to a box the group holds", edge("{ from: server, to: store }"), `s.yaml:21:31: edge "server"→"store": both ends are group "server" or inside it; end the edge at a box, an icon or another group outside "server"`},
		{"from a group to itself", edge("{ from: server, to: server }"), `s.yaml:21:31: edge "server"→"server": both ends are group "server" or inside it; end the edge at a box, an icon or another group outside "server"`},
		{"to a group nested in it", edge("{ from: server, to: data }"), `s.yaml:21:31: edge "server"→"data": both ends are group "server" or inside it; end the edge at a box, an icon or another group outside "server"`},
		{"a box among the group's ranks", edge("{ from: api, to: x }\n        - { from: x, to: store }\n        - { from: x, to: server }"), `s.yaml:23:19: edge "x"→"server": "x" can stand neither above group "server" nor below it, as "api" stands no lower than it and "store" no higher; end the edge at a box or an icon in "server"`},
		{"to a box among the group's ranks", edge("{ from: api, to: x }\n        - { from: x, to: store }\n        - { from: server, to: x }"), `s.yaml:23:31: edge "server"→"x": "x" can stand neither above group "server" nor below it, as "api" stands no lower than it and "store" no higher; end the edge at a box or an icon in "server"`},
		{"a group among another's ranks", tangled, `s.yaml:12:19: edge "h"→"g": "h" can stand neither above group "g" nor below it, as "g1" stands no lower than it and "g2" no higher; end the edge at a box or an icon in "g"`},
		{"a group named as a box before it", strings.Replace(gg, "name: server", "name: cli", 1), `s.yaml:8:11: the name "cli" is given before: give each part, row and node its own name`},
		{"a box named as a group before it", strings.Replace(gg, "name: x,", "name: server,", 1), `s.yaml:17:11: the name "server" is given before: give each part, row and node its own name`},
		{"200 boxes and 2 groups", many.String(), `s.yaml:3:5: graph: 202 boxes, icons, groups and lanes, past 200; split it into graphs`},
		{"200 icons and a group", icons.String(), `s.yaml:3:5: graph: 201 boxes, icons, groups and lanes, past 200; split it into graphs`},
		{"101 lanes and 101 boxes", laneful.String(), `s.yaml:3:5: graph: 202 boxes, icons, groups and lanes, past 200; split it into graphs`},
		{"from a lane", lane("{ from: user, to: build }"), `s.yaml:16:19: edge "user"→"build": "user" is a lane, from end to end of the graph; end the edge at a box, an icon or a group in it`},
		{"to a lane", lane("{ from: ask, to: dev }"), `s.yaml:16:28: edge "ask"→"dev": "dev" is a lane, from end to end of the graph; end the edge at a box, an icon or a group in it`},
		{"from a lane to a lane", lane("{ from: user, to: dev }"), `s.yaml:16:19: edge "user"→"dev": "user" is a lane, from end to end of the graph; end the edge at a box, an icon or a group in it`},
		{"from a lane to a box it holds", lane("{ from: dev, to: build }"), `s.yaml:16:28: edge "dev"→"build": both ends are group "dev" or inside it; end the edge at a box, an icon or another group outside "dev"`},
		{"a box among the lanes", strings.Replace(lanes, "      lanes:\n", "      lanes:\n        - box: { name: z, rows: [Z] }\n", 1), "s.yaml:7:11: additional properties 'box' not allowed\ns.yaml:7:11: missing property 'nodes'"},
		{"to a group in a lane", lane("{ from: ask, to: tools }"), ""},
		{"nodes beside lanes", strings.Replace(lanes, "      edges:\n", "      nodes:\n        - box: { name: z, rows: [Z] }\n      edges:\n", 1), `s.yaml:14:7: additional properties 'nodes' not allowed`},
		{
			"nodes beside lanes in the second graph", "title: T\nparts:\n  - graph:\n      x: 10\n      y: 10\n      nodes: [{ box: { name: c, rows: [c] } }]\n" +
				"  - graph:\n      x: 10\n      y: 200\n      lanes: [{ name: team, nodes: [{ box: { name: a, rows: [a] } }] }]\n      nodes: [{ box: { name: b, rows: [b] } }]\n",
			`s.yaml:11:7: additional properties 'nodes' not allowed`,
		},
	} {
		if _, got := draw(t, c.src, inspect); got != c.want {
			t.Errorf("%s:\n  told %q\n  want %q", c.name, got, c.want)
		}
	}

	// The Japanese words number their arguments, and take each name where its English does.
	among := edge("{ from: api, to: x }\n        - { from: x, to: store }\n        - { from: x, to: server }")
	const amongJa = `s.yaml:23:19: edge "x"→"server": "x" はまとまり「server」の上にも下にも立てません（"api" が "x" と同じ段か上に、"store" が同じ段か下に立ちます）。「server」の中の箱かアイコンで終えてください`
	if got := toldInJapanese(t, among); got != amongJa {
		t.Errorf("told %q\n  want %q", got, amongJa)
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
		{"a frame a box of the graph straddles", g1, "  - frame: { name: f, x: 0, y: 0, w: 100, h: 40 }\n", `box "write" straddles the edge of frame "f" at x 100, spanning 26 to 178` + "\n@: " + `box "write" straddles the edge of frame "f" at y 40, spanning 20 to 49`},
		{"a label in the graph's channel", g1, "  - label: { text: L, x: 68, y: 61, anchor: middle }\n", `edge "write"→"check" runs through label "L"; move one of them, or go round it by via`},
		{"a box over a label of the graph", g2, "  - box: { name: note, x: 106, y: 125, rows: [N] }\n", `box "note" overlaps label "yes"`},
		{"an edge by hand through a label of the graph", g2, "  - edge: { from: [110, 141], to: [160, 141] }\n", `edge [110,141]→[160,141] runs through label "yes"; move one of them, or go round it by via`},
		{
			"a box in the graph's channel", k33, "  - box: { name: note, x: 24, y: 78, rows: [N] }\n",
			`edge "a"→"x" runs through box "note"; move one of them, or go round it by via` + "\n@: " + `edge "a"→"y" runs through box "note"; move one of them, or go round it by via` + "\n@: " + `edge "b"→"x" runs through box "note"; move one of them, or go round it by via`,
		},
		{
			"an edge by hand across the graph's edges", g1, "  - edge: { from: [0, 62], to: [400, 62] }\n",
			"edge \"check\"→\"write\" crosses edge [0,62]→[400,62] at 140,62; move one of them, or add crossing: true to one if it cannot be avoided\n" +
				"@: edge \"write\"→\"check\" crosses edge [0,62]→[400,62] at 68,62; move one of them, or add crossing: true to one if it cannot be avoided",
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

	// An icon's label across the frame of another graph is told at its graph, once for each edge crossed.
	const across = "title: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - group: { name: g1, label: g1, nodes: [{ box: { name: a, rows: [A] } }] }\n" +
		"  - graph:\n      x: 30\n      y: 34\n      nodes:\n        - icon: { name: l, icon: aws/aws-lambda, label: A long Lambda function name that is wide }\n"
	if _, got := draw(t, across, inspect); got != `s.yaml:8:5: label "A long Lambda function name that is wide" straddles the edge of frame "g1" of another graph at x 92, spanning 30 to 328; move one of the graphs by its x and y`+"\n"+
		`s.yaml:8:5: label "A long Lambda function name that is wide" straddles the edge of frame "g1" of another graph at y 91, spanning 85 to 103; move one of the graphs by its x and y` {
		t.Errorf("an icon's label across another graph's frame:\n  told %q", got)
	}

	// A box of one graph over a label of another and an edge through a box of another are told at the graph of the first
	// box, to move a graph.
	second := strings.NewReplacer("x: 20", "x: 100", "y: 20", "y: 125", "write", "write2", "check", "check2", "show", "show2").
		Replace(strings.TrimPrefix(g2, "title: T\nparts:\n"))
	if _, got := draw(t, g2+second, inspect); got != `s.yaml:14:5: box "write2" overlaps label "yes" of another graph; move one of the graphs by its x and y`+"\n"+
		`s.yaml:14:5: edge "write2"→"check2" runs through box "show" of another graph; move one of the graphs by its x and y` {
		t.Errorf("two graphs: told %q", got)
	}

	// Off the canvas, what a graph laid out is told at the graph, to move it.
	_, got := draw(t, "width: 100\n"+g1, inspect)
	told := strings.Split(got, "\n")
	if !slices.Contains(told, `s.yaml:4:5: box "write" sticks out of the 100x175 canvas; move its graph by its x and y, or widen the canvas by its width and height (or leave them out)`) ||
		slices.ContainsFunc(told, func(s string) bool { return !strings.HasPrefix(s, "s.yaml:4:5: ") }) {
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
// them), and the shorter box grown for three, laid out right by its width. Between two groups, their common group
// stands as written: a row through a group as written, not as a block of the common group's boxes; a group with no
// edge between them, its frame crossed; a group in a lane facing the other lane.
func TestGraphLevel(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, src, want string
	}{
		{"lanes, down", level3, `frame "u" 20,20 84x213
frame "v" 128,20 132x213
frame "w" 284,20 72x213
the heading of frame "u" 32,29 7.68x18
the heading of frame "v" 140,29 7.68x18
the heading of frame "w" 296,29 7.68x18
box "a" 32,115 48x41
box "ok" 32,192 56x29
box "b" 200,115 48x41
box "q" 200,50 48x29
box "c" 296,50 48x29
box "d" 296,192 48x29
edge "c"→"b" M320,79 V91 H224 V115
edge "b"→"a" M200,123.5 H80
edge "a"→"b" M80,147.5 H200
edge "b"→"d" M224,156 V168 H320 V192
edge "d"→"ok" M296,212.5 H88
label "ask" 130.2,114.5 31.6x18
label "answer" 131.4,138.5 53.2x18
label "ask" 176.2,203.5 31.6x18
`},
		{"lanes, right", strings.Replace(level3, "      lanes:\n", "      direction: right\n      lanes:\n", 1), `frame "u" 20,20 302x54
frame "v" 20,98 302x96
frame "w" 20,218 302x114
the heading of frame "u" 32,29 7.68x18
the heading of frame "v" 32,107 7.68x18
the heading of frame "w" 32,227 7.68x18
box "a" 134,32 84x29
box "ok" 254,32 56x29
box "b" 134,134 84x48
box "q" 50,152 48x29
box "c" 50,272 48x48
box "d" 258,230 48x48
edge "c"→"b" M98,296 H110 V158 H134
edge "b"→"a" M152,134 V61
edge "a"→"b" M200,61 V134
edge "b"→"d" M218,158 H230 V254 H258
edge "d"→"ok" M288,230 V61
label "ask" 136.2,100.5 31.6x18
label "answer" 173.4,100.5 53.2x18
label "ask" 272.2,136.5 31.6x18
`},
		{"a row through a group, the order written winning over the block", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - group: { name: g, label: G, nodes: [{ box: { name: b, rows: [B] } }] }
        - box: { name: c, rows: [C] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: c, level: true }
`, `frame "g" 92,20 72x71
the heading of frame "g" 104,29 7.68x18
box "a" 20,50 48x29
box "b" 104,50 48x29
box "c" 188,50 48x29
edge "a"→"b" M68,70.5 H104
edge "b"→"c" M152,70.5 H188
`},
		{"a group with no edge stands as written, crossed", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: s, rows: [S] }
        - group: { name: ga, label: A, nodes: [{ box: { name: a, rows: [A] } }] }
        - group: { name: gg, label: G, nodes: [{ box: { name: g1, rows: [G1] } }, { box: { name: g3, rows: [G3] } }] }
        - group: { name: gb, label: B, nodes: [{ box: { name: b, rows: [B] } }] }
        - box: { name: x, rows: [X] }
      edges:
        - { from: s, to: a }
        - { from: g1, to: x }
        - { from: x, to: g3 }
        - { from: a, to: b, level: true }
`, `frame "ga" 20,115 108x71
frame "gg" 152,20 84x243
frame "gb" 260,115 72x71
the heading of frame "ga" 32,124 7.68x18
the heading of frame "gg" 164,29 7.68x18
the heading of frame "gb" 272,124 7.68x18
box "s" 20,50 48x29
box "a" 68,145 48x29
box "g1" 164,50 56x29
box "g3" 164,222 56x29
box "b" 272,145 48x29
box "x" 356,145 48x29
edge "s"→"a" M44,79 V91 H92 V145
edge "g1"→"x" M192,79 V91 H380 V145
edge "x"→"g3" M380,174 V198 H192 V222
edge "a"→"b" M116,165.5 H272
`},
		{"a group in a lane to another lane", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, label: U, nodes: [{ group: { name: g, label: G, nodes: [{ box: { name: a, rows: [A] } }] } }, { box: { name: k, rows: [K] } }] }
        - { name: v, label: V, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true }
`, `frame "u" 20,20 168x113
frame "g" 104,50 72x71
frame "v" 212,20 72x113
the heading of frame "u" 32,29 7.68x18
the heading of frame "g" 116,59 7.68x18
the heading of frame "v" 224,29 7.68x18
box "a" 116,80 48x29
box "k" 32,80 48x29
box "b" 224,80 48x29
edge "a"→"b" M164,100.5 H224
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
edge "x"→"b" M62.67,49 V61 H188 V85
edge "y"→"a" M128,49 V85
edge "x"→"p" M41.33,49 V85
edge "a"→"b" M140,105.5 H164
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
edge "a"→"b" M140,49 V74
edge "b"→"a" M152,74 V49
edge "a"→"b" M164,49 V74
edge "b"→"a" M176,74 V49
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
edge "a"→"b" M152,49 V74
edge "b"→"a" M164,74 V49
edge "a"→"b" M176,49 V74
`},
	} {
		if got, told := draw(t, c.src, inspect); got != c.want || told != "" {
			t.Errorf("%s:\n%s%s\nwant\n%s", c.name, got, told, c.want)
		}
	}
}

// A level edge is refused at a group, when an edge joins two boxes of its row, when something stands between its boxes
// or icons in their rank as laid out (a box, or a group's frame but one right in their common group), when it takes
// the side of its box's loops, and when it runs through a group's or lane's heading laid out right; a box among a
// group's ranks is told by ranks. Where marks meet, the order written wins and the edge is told; a common group's boxes
// stand as written, so what is told is what is written between. A level edge refused for loops makes no row, so an
// edge between its boxes is not told too. The rest lay out: a box of a group to one outside, a fan with its first zone written between, groups of a common group as written however align would order
// them, a row of one lane however it faces, level on a loop (a loop), loops on the other side, a cycle through a row,
// three edges between two boxes, and headings that do not reach the edge.
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
`, `s.yaml:10:26: edge "a"→"g": "g" is a group, and level stands two boxes or icons in one rank; end the edge at a box or an icon, or drop level`},
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
`, `s.yaml:11:11: edge "a"→"b": "a" and "b" stand in one rank by level edges, so this edge cannot run from one rank to another; end it elsewhere, or drop level from the edges joining them`},
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
`, `s.yaml:11:36: edge "a"→"c": box "b" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
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
`, `s.yaml:10:36: edge "a"→"b": box "c" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
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
`, `s.yaml:15:36: edge "b"→"c": group "h" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
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
`, `s.yaml:11:36: edge "a"→"b": box "m" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
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
`, `s.yaml:15:36: edge "a"→"b": group "h" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
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
`, `s.yaml:10:36: edge "p"→"r": box "q" stands between its boxes or icons in their rank; drop level, or move it out of the way
s.yaml:11:36: edge "q"→"s": box "r" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"a box of the common group's node between, the order written winning over the block", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: l, rows: [L] }
        - group: { name: g, nodes: [{ box: { name: m, rows: [M] } }, { box: { name: n, rows: [N] } }] }
        - box: { name: r, rows: [R] }
      edges:
        - { from: l, to: m, level: true }
        - { from: m, to: r, level: true }
`, `s.yaml:12:36: edge "m"→"r": box "n" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"a group facing both ways, the start winning", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: z, rows: [Z] }
        - group: { name: x, label: X, nodes: [{ group: { name: y1, label: Y1, nodes: [{ box: { name: a, rows: [A] } }] } }, { group: { name: y2, label: Y2, nodes: [{ box: { name: b, rows: [B] } }] } }] }
      edges:
        - { from: a, to: b, level: true }
        - { from: z, to: b, level: true }
`, `s.yaml:11:36: edge "z"→"b": group "y1" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"two level edges between two groups in one rank", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group: { name: aza, label: AZ a, nodes: [{ box: { name: w, rows: [Writer] } }, { box: { name: ra, rows: [Redis a] } }] }
        - group: { name: azc, label: AZ c, nodes: [{ box: { name: r, rows: [Reader] } }, { box: { name: rc, rows: [Redis c] } }] }
      edges:
        - { from: w, to: r, level: true }
        - { from: ra, to: rc, level: true }
`, `s.yaml:10:36: edge "w"→"r": box "ra" stands between its boxes or icons in their rank; drop level, or move it out of the way
s.yaml:11:38: edge "ra"→"rc": box "r" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"a box written between two groups, in their rank by an edge", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group:
            name: vpc
            label: VPC
            nodes:
              - group:
                  name: aza
                  label: AZ a
                  nodes:
                    - group: { name: suba, label: Sub a, nodes: [{ box: { name: w, rows: [Writer] } }, { box: { name: o, rows: [Other] } }] }
                    - box: { name: p, rows: [P] }
              - box: { name: alb, rows: [ALB] }
              - box: { name: r, rows: [Reader] }
      edges:
        - { from: w, to: r, level: true }
        - { from: p, to: o }
`, `s.yaml:20:36: edge "w"→"r": box "alb" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"a group's frame inside a group crossed", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group: { name: ga, nodes: [{ box: { name: a, rows: [A] } }] }
        - group: { name: gb, nodes: [{ group: { name: h, nodes: [{ box: { name: b1, rows: [B1] } }, { box: { name: b2, rows: [B2] } }] } }] }
        - group: { name: gc, nodes: [{ box: { name: c, rows: [C] } }] }
      edges:
        - { from: b1, to: b2, level: true }
        - { from: a, to: c, level: true }
`, `s.yaml:12:36: edge "a"→"c": group "h" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"a box written between two groups, named though the order would move another there", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: t1, rows: [T1] }
        - box: { name: t2, rows: [T2] }
        - box: { name: p, rows: [P] }
        - group: { name: a, label: A, nodes: [{ box: { name: w, rows: [W] } }] }
        - box: { name: q, rows: [Q] }
        - group: { name: b, label: B, nodes: [{ box: { name: r, rows: [R] } }] }
      edges:
        - { from: t2, to: p }
        - { from: t1, to: q }
        - { from: t1, to: w }
        - { from: t2, to: r }
        - { from: w, to: r, level: true }
`, `s.yaml:18:36: edge "w"→"r": box "q" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"a fan from one zone to two, written in the zones' order", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group: { name: za, label: A, nodes: [{ box: { name: w, rows: [Writer] } }] }
        - group: { name: zb, label: B, nodes: [{ box: { name: rb, rows: [Reader] } }] }
        - group: { name: zc, label: C, nodes: [{ box: { name: rc, rows: [Reader] } }] }
      edges:
        - { from: w, to: rb, level: true }
        - { from: w, to: rc, level: true }
`, `s.yaml:12:37: edge "w"→"rc": box "rb" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"a group's heading, laid out right", `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      nodes:
        - group: { name: u, label: U, nodes: [{ box: { name: a, rows: [A] } }] }
        - group: { name: e, label: "A long heading of this group", nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true }
`, `s.yaml:11:36: edge "a"→"b": it runs through the heading of group or lane "e", which reaches into the first rank laid out right; drop level, or shorten the heading`},
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
`, `s.yaml:10:36: edge "a"→"b": it takes side right of "a", where "a"'s loops are; drop level or the loops`},
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
`, `s.yaml:10:36: edge "b"→"a": it takes side right of "a", where "a"'s loops are; drop level or the loops`},
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
`, `s.yaml:13:36: edge "a"→"b": it runs through the heading of group or lane "v", which reaches into the first rank laid out right; drop level, or shorten the heading`},
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
`, `s.yaml:13:36: edge "a"→"b": it runs through the heading of group or lane "e", which reaches into the first rank laid out right; drop level, or shorten the heading`},
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
`, `s.yaml:14:19: edge "x"→"g": "x" can stand neither above group "g" nor below it, as "q" stands no lower than it and "p" no higher; end the edge at a box or an icon in "g"`},
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
`, `s.yaml:11:36: edge "a"→"b": it takes side right of "a", where "a"'s loops are; drop level or the loops`},
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
`, ""},
		{"a fan from one zone to two, the zone written between", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - group: { name: zb, label: B, nodes: [{ box: { name: rb, rows: [Reader] } }] }
        - group: { name: za, label: A, nodes: [{ box: { name: w, rows: [Writer] } }] }
        - group: { name: zc, label: C, nodes: [{ box: { name: rc, rows: [Reader] } }] }
      edges:
        - { from: w, to: rb, level: true }
        - { from: w, to: rc, level: true }
`, ""},
		{"groups written in a common group keep their order however their frames stand on average", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: x1, rows: [X] }
        - box: { name: x2, rows: [X] }
        - box: { name: x3, rows: [X] }
        - box: { name: x4, rows: [X] }
        - box: { name: x5, rows: [X] }
        - box: { name: x6, rows: [X] }
        - box: { name: x7, rows: [X] }
        - box: { name: x8, rows: [X] }
        - box: { name: x9, rows: [X] }
        - box: { name: x10, rows: [X] }
        - group: { name: ga, nodes: [{ box: { name: a, rows: [A] } }, { box: { name: a2, rows: [A] } }, { box: { name: a3, rows: [A] } }] }
        - group: { name: gb, nodes: [{ box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: a2 }
        - { from: a2, to: a3 }
        - { from: a, to: x1 }
        - { from: a, to: x2 }
        - { from: a, to: x3 }
        - { from: a, to: x4 }
        - { from: a, to: x5 }
        - { from: x1, to: x7 }
        - { from: x2, to: x6 }
        - { from: x3, to: x8 }
        - { from: x4, to: x9 }
        - { from: x5, to: x10 }
        - { from: a, to: b, level: true }
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

// A level edge told through a lane's heading is taken out of the drawing whole: the other edge at its icon's side still
// turns there, and the sides it took are left to edges written by hand, as inspect shows with the mistake.
func TestGraphLevelHeadingTakenOut(t *testing.T) {
	t.Parallel()

	const icons = `title: T
parts:
  - graph:
      x: 20
      y: 20
      direction: right
      lanes:
        - { name: u, label: ww, nodes: [{ icon: { name: a, icon: aws/aws-lambda, label: wwww } }] }
        - { name: v, label: wwwwwwww, nodes: [{ icon: { name: b, icon: aws/aws-lambda, label: www } }] }
      edges:
        - { from: b, to: a, label: www, level: true }
        - { from: b, to: a, label: ww, level: true }
`
	want := `s.yaml:11:48: edge "b"→"a": it runs through the heading of group or lane "v", which reaches into the first rank laid out right; drop level, or shorten the heading`
	if _, told := draw(t, icons, inspect); told != want {
		t.Errorf("two level edges at an icon:\n  told %q\n  want %q", told, want)
	}

	const hands = `title: T
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
  - edge: { from: [70, 240], to: a }
  - edge: { from: [40, 240], to: b }
`
	_, v, err := yamlpos.ParseMap("s.yaml", []byte(hands))
	if err != nil {
		t.Fatal(err)
	}
	got, err := inspect(drawer(t), v)
	var told usererr.List
	if !errors.As(err, &told) || !slices.ContainsFunc(told, func(e *usererr.Error) bool { return e.Key == "diagram_graph_level_heading" }) {
		t.Errorf("edges written by hand by a level edge through a heading: told %v, want the heading among them", err)
	}
	for _, run := range []string{`edge [70,240]→"a" M70,240 H145 V175`, `edge [40,240]→"b" M40,240 H145 V362`} {
		if !strings.Contains(got, run+"\n") {
			t.Errorf("edges written by hand at the sides the level edge took: want %q in\n%s", run, got)
		}
	}
}

// Laid out down, a heading takes no place in its frame's first rank: only what a line reaches from above keeps right of
// it, in the rank's order as written, and the frame holds it. Laid out right, it keeps its place.
func TestGraphHeadRoom(t *testing.T) {
	t.Parallel()

	const heading = "A long heading of the frame here"
	graph := func(direction, nodes, edges string) string {
		return "title: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n" + direction + "      nodes:\n" + nodes + "      edges:\n" + edges
	}
	group := func(boxes ...string) string {
		return "        - group:\n            name: g\n            label: " + heading + "\n            nodes:\n" +
			"              - " + strings.Join(boxes, "\n              - ") + "\n"
	}
	box := func(name string) string { return "box: { name: " + name + ", rows: [" + name + "] }" }
	top := "        - " + box("top") + "\n"
	for _, c := range []struct {
		name, src, want string
	}{
		{"no line in: under the heading", graph("", group(box("a"))+"        - "+box("c")+"\n", "        - { from: a, to: c }\n"), `frame "g" 20,20 270x71
the heading of frame "g" 32,29 245.76x18
box "a" 32,50 48x29
box "c" 32,115 48x29
edge "a"→"c" M56,79 V115
`},
		{"no line in: the frame holds the heading", "title: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n" + group(box("a")), `frame "g" 20,20 270x71
the heading of frame "g" 32,29 245.76x18
box "a" 32,50 48x29
`},
		{"a line in: right of the heading", graph("", top+group(box("a")), "        - { from: top, to: a }\n"), `frame "g" 20,85 342x71
the heading of frame "g" 32,94 245.76x18
box "top" 20,20 64x29
box "a" 302,115 48x29
edge "top"→"a" M52,49 V61 H326 V115
`},
		{"a line to the frame: where it enters is right of the heading", graph("", top+group(box("a")), "        - { from: top, to: g }\n"), `frame "g" 20,85 312x71
the heading of frame "g" 32,94 245.76x18
box "top" 20,20 64x29
box "a" 32,115 48x29
edge "top"→"g" M52,49 V61 H308 V85
`},
		{"a line past the first rank: its point is right of the heading", graph("", top+group(box("a"), box("b")),
			"        - { from: top, to: a }\n        - { from: a, to: b }\n        - { from: top, to: b }\n"), `frame "g" 20,85 384x136
the heading of frame "g" 32,94 245.76x18
box "top" 290,20 64x29
box "a" 302,115 48x29
box "b" 290,180 64x29
edge "top"→"a" M326,49 V115
edge "a"→"b" M326,144 V156 H311.33 V180
edge "top"→"b" M340,49 V61 H380 V156 H332.67 V180
`},
		{"written first, with no line in: under the heading", graph("", top+group("box: { name: f, with: e, rows: [f] }", box("e")),
			"        - { from: top, to: e }\n"), `frame "g" 20,85 342x71
the heading of frame "g" 32,94 245.76x18
box "top" 20,20 64x29
box "f" 32,115 48x29
box "e" 302,115 48x29
edge "top"→"e" M52,49 V61 H326 V115
`},
		{"written after one with a line in: past it", graph("", top+group(box("e"), "box: { name: f, with: e, rows: [f] }"),
			"        - { from: top, to: e }\n"), `frame "g" 20,85 414x71
the heading of frame "g" 32,94 245.76x18
box "top" 20,20 64x29
box "e" 302,115 48x29
box "f" 374,115 48x29
edge "top"→"e" M52,49 V61 H326 V115
`},
		{"nested: a line in keeps right of both headings", graph("", top+"        - group:\n            name: o\n            label: "+heading+"\n            nodes:\n"+
			"              - group: { name: i, label: i, nodes: [{ "+box("a")+" }] }\n", "        - { from: top, to: a }\n"), `frame "o" 20,85 354x113
frame "i" 32,115 330x71
the heading of frame "o" 32,94 245.76x18
the heading of frame "i" 44,124 7.68x18
box "top" 20,20 64x29
box "a" 302,145 48x29
edge "top"→"a" M52,49 V61 H326 V145
`},
		{"laid out right: the heading keeps its place", graph("      direction: right\n", group(box("a"))+"        - "+box("c")+"\n", "        - { from: a, to: c }\n"), `frame "g" 20,20 270x114
the heading of frame "g" 32,29 245.76x18
box "a" 140,74 48x48
box "c" 314,74 48x48
edge "a"→"c" M188,98 H314
`},
	} {
		if got, told := draw(t, c.src, inspect); got != c.want || told != "" {
			t.Errorf("%s:\n%s%s\nwant\n%s", c.name, got, told, c.want)
		}
	}
}

// A line written by hand may come to run through a heading as a box stands under it: told, with the way round it.
func TestGraphHeadRoomByHand(t *testing.T) {
	t.Parallel()

	const src = "width: 500\nheight: 300\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 60\n      nodes:\n" +
		"        - group: { name: g, label: A long heading of the frame here, nodes: [{ box: { name: a, rows: [a] } }] }\n" +
		"  - edge: { from: [300, 0], to: a%s }\n"
	want := `s.yaml:10:5: edge [300,0]→"a" runs through the heading of frame "g"; move one of them, or go round it by via`
	if _, told := draw(t, fmt.Sprintf(src, ""), inspect); told != want {
		t.Errorf("told %q, want %q", told, want)
	}
	if _, told := draw(t, fmt.Sprintf(src, ", via: [[300, 95]]"), inspect); told != "" {
		t.Errorf("by via: told %q", told)
	}
}

// An icon that stands between a level edge's boxes is named as an icon, not as a box.
func TestGraphLevelBetweenIcon(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, src, want string }{
		{"an icon of their row between them", `title: T
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: a, rows: [A] }
        - icon: { name: b, icon: aws/aws-lambda, label: B }
        - box: { name: c, rows: [C] }
      edges:
        - { from: a, to: c, level: true }
        - { from: a, to: b, level: true }
`, `s.yaml:11:36: edge "a"→"c": icon "b" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
		{"an icon of the block on the side it faces", `title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: u, nodes: [{ box: { name: a, rows: [A] } }] }
        - { name: v, nodes: [{ icon: { name: c, icon: aws/aws-lambda, label: C } }, { box: { name: b, rows: [B] } }] }
      edges:
        - { from: a, to: b, level: true }
        - { from: b, to: c, level: true }
`, `s.yaml:10:36: edge "a"→"b": icon "c" stands between its boxes or icons in their rank; drop level, or move it out of the way`},
	} {
		if _, got := draw(t, c.src, drawWith("en", lambda)); got != c.want {
			t.Errorf("%s:\n  told %q\n  want %q", c.name, got, c.want)
		}
	}
}
