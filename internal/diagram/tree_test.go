package diagram_test

import (
	"strings"
	"testing"
)

// A tree's nodes stand under their parents, each depth in one row below its tallest node; edges go down, turn
// across at one height for the depth, and go straight down to a child right below; siblings share the way down.
// A node's name is an edge's end, and names its edges as an edge's end does.
func TestTree(t *testing.T) {
	t.Parallel()

	src := head + `  - tree:
      x: 200
      y: 10
      root:
        name: a
        rows: [A]
        children:
          - rows: [B, two rows]
            children:
              - rows: [D]
          - rows: [C]
            children:
              - rows: [E]
              - rows: [F]
  - box: { name: z, x: 340, y: 10, rows: [Z] }
  - edge: { from: z, to: a }
`
	got, mistakes := draw(t, src, inspect)
	want := `box "a" 176,10 48x29
box "B" 80,79 96x46
box "D" 104,165 48x29
box "C" 236,79 48x29
box "E" 200,165 48x29
box "F" 272,165 48x29
box "z" 340,10 48x29
edge "a"→"B" M200,39 V59 H128 V79
edge "B"→"D" M128,125 V165
edge "a"→"C" M200,39 V59 H260 V79
edge "C"→"E" M260,108 V145 H224 V165
edge "C"→"F" M260,108 V145 H296 V165
edge "z"→"a" M340,24.5 H224
`
	if mistakes != "" || got != want {
		t.Errorf("inspect =\n%s%s\nwant\n%s", got, mistakes, want)
	}
	// What is drawn is the path inspect tells: the way straight down has no turn of no length.
	svg, _ := draw(t, src, drawWith("en", nil))
	if !strings.Contains(svg, `d="M128,125 V165"`) {
		t.Errorf("drawn = %s, want the edge to D straight down", svg)
	}
}

// A tree growing right is one growing down turned: each depth in one column past its longest node, siblings across it.
// With left, both sides are measured apart, and the root stands in the middle of the tallest of itself and its sides;
// x and y are the top left of it all.
func TestTreeRight(t *testing.T) {
	t.Parallel()

	const tree = "title: T\nparts:\n  - tree:\n      x: 20\n      y: 30\n      direction: right\n"
	for _, tt := range []struct{ name, parts, want string }{
		// B is shorter than its column: its edges turn halfway to the next column, not halfway from B.
		{"right", tree + `      root:
        name: a
        rows: [A]
        children:
          - rows: [Wide node of depth 1]
          - rows: [B, two rows]
            children:
              - rows: [D]
              - rows: [E]
          - rows: [C]
            children:
              - rows: [F]
  - box: { name: z, x: 600, y: 10, rows: [Z] }
  - edge: { from: z, to: a }
`, `box "a" 20,109.5 48x29
box "Wide node of depth 1" 108,30 200x29
box "B" 108,101 96x46
box "D" 348,83 48x29
box "E" 348,136 48x29
box "C" 108,189 48x29
box "F" 348,189 48x29
box "z" 600,10 48x29
edge "a"→"Wide node of depth 1" M68,124 H88 V44.5 H108
edge "a"→"B" M68,124 H108
edge "B"→"D" M204,124 H328 V97.5 H348
edge "B"→"E" M204,124 H328 V150.5 H348
edge "a"→"C" M68,124 H88 V203.5 H108
edge "C"→"F" M156,203.5 H348
edge "z"→"a" M600,24.5 H44 V109.5
`},
		// The right is as deep as the left but narrower: neither side's lengths are the other's.
		{"both sides", tree + `      root: { name: r, rows: [R], children: [{ rows: [P], children: [{ rows: [Q] }] }] }
      left:
        - rows: [Long node on the left]
          children:
            - rows: [X]
            - rows: [Y]
        - rows: [Z]
`, `box "r" 356,83 48x29
box "P" 444,83 48x29
box "Q" 532,83 48x29
box "Long node on the left" 108,56.5 208x29
box "X" 20,30 48x29
box "Y" 20,83 48x29
box "Z" 268,136 48x29
edge "r"→"P" M404,97.5 H444
edge "P"→"Q" M492,97.5 H532
edge "r"→"Long node on the left" M356,97.5 H336 V71 H316
edge "Long node on the left"→"X" M108,71 H88 V44.5 H68
edge "Long node on the left"→"Y" M108,71 H88 V97.5 H68
edge "r"→"Z" M356,97.5 H336 V150.5 H316
`},
		// The left is one deep and narrow, the right deep and wide.
		{"deeper right", tree + `      root: { rows: [R], children: [{ rows: [Long node on the right], children: [{ rows: [P], children: [{ rows: [Q] }] }] }, { rows: [S] }] }
      left: [{ rows: [L] }]
`, `box "R" 108,56.5 48x29
box "Long node on the right" 196,30 216x29
box "P" 452,30 48x29
box "Q" 540,30 48x29
box "S" 196,83 48x29
box "L" 20,56.5 48x29
edge "R"→"Long node on the right" M156,71 H176 V44.5 H196
edge "Long node on the right"→"P" M412,44.5 H452
edge "P"→"Q" M500,44.5 H540
edge "R"→"S" M156,71 H176 V97.5 H196
edge "R"→"L" M108,71 H68
`},
		// The root is the tallest, and its top is at y.
		{"tall root", tree + `      root: { rows: [a, b, c, d, e, f, g, h], children: [{ rows: [K] }] }
      left: [{ rows: [L] }]
`, `box "a" 108,30 48x148
box "K" 196,89.5 48x29
box "L" 20,89.5 48x29
edge "a"→"K" M156,104 H196
edge "a"→"L" M108,104 H68
`},
		{"left alone", tree + "      root: { rows: [A] }\n      left: [{ rows: [L] }]\n", `box "A" 108,30 48x29
box "L" 20,30 48x29
edge "A"→"L" M108,44.5 H68
`},
		{"no branches", tree + "      root: { rows: [A] }\n", "box \"A\" 20,30 48x29\n"},
	} {
		got, mistakes := draw(t, tt.parts, inspect)
		if mistakes != "" || got != tt.want {
			t.Errorf("%s: inspect =\n%s%s\nwant\n%s", tt.name, got, mistakes, tt.want)
		}
	}
}

// A tree's mistakes are at its nodes, as those of boxes and edges are at the parts.
func TestTreeMistakes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ parts, want string }{
		{
			"  - tree:\n      x: 200\n      y: 10\n      root:\n        rows: [A]\n        children:\n          - rows: [B]\n          - tone: good\n",
			"s.yaml:12:13: missing property 'rows'",
		},
		{
			"  - box: { name: a, x: 10, y: 150, rows: [X] }\n  - tree:\n      x: 200\n      y: 10\n      root:\n        name: a\n        rows: [A]\n",
			`s.yaml:9:7: the name "a" is given before: give each part, row and node its own name`,
		},
		// A tree's node and a box written by hand: at the box, which its writer moves, written after the tree or before.
		// Two nodes of one text and no name read alike, and told at the one box, both mistakes are told once (LIMIT in
		// tell).
		{
			"  - tree:\n      x: 200\n      y: 10\n      root:\n        rows: [A]\n        children:\n          - rows: [S]\n          - rows: [S]\n  - box: { x: 150, y: 80, rows: [S] }\n",
			`s.yaml:13:5: box "S" overlaps box "S"`,
		},
		{
			"  - box: { x: 150, y: 80, rows: [S] }\n  - tree:\n      x: 200\n      y: 10\n      root:\n        rows: [A]\n        children:\n          - rows: [S]\n          - rows: [T]\n",
			`s.yaml:5:5: box "S" overlaps box "S"`,
		},
		// A node with no name is named by its widest text, its box and its edges alike.
		{
			"  - tree:\n      x: 200\n      y: 10\n      root:\n        rows: [{ en: Top, ja: \"いちばん上の節\" }]\n        children:\n          - rows: [{ en: Kid, ja: \"子の節\" }]\n" +
				"  - box: { name: b, x: 180, y: 50, w: 60, h: 30, rows: [b] }\n",
			`s.yaml:12:5: edge "いちばん上の節"→"子の節" runs through box "b"; move one of them, or go round it by via`,
		},
		{
			"  - tree:\n      x: 20\n      y: 10\n      root:\n        rows: [A]\n        children:\n          - rows: [B]\n          - rows: [Wide child]\n",
			`s.yaml:11:13: edge "A"→"B" sticks out of the 400x200 canvas`,
		},
		{
			"  - tree: { x: 200, y: 10, root: { rows: [A] } }\n  - edge: { from: [10, 10], to: nope }\n",
			`s.yaml:6:5: edge: no part, row or node named "nope"`,
		},
		// left is of a tree growing right alone: the mistake names the key to write, or the value to change.
		{"  - tree: { x: 200, y: 10, left: [{ rows: [B] }], root: { rows: [A] } }\n", "s.yaml:5:5: missing property 'direction'"},
		{"  - tree:\n      x: 200\n      y: 10\n      direction: down\n      left: [{ rows: [B] }]\n      root: { rows: [A] }\n", "s.yaml:8:18: "},
		{"  - tree:\n      x: 200\n      y: 10\n      direction: up\n      root: { rows: [A] }\n", "s.yaml:8:18: "},
		{"  - tree:\n      x: 10\n      y: 10\n      direction: right\n      root: { rows: [A] }\n      left:\n        - tone: good\n", "s.yaml:11:11: missing property 'rows'"},
		// A left node's and its child's mistakes are at them.
		{
			"  - box: { name: a, x: 10, y: 150, rows: [X] }\n  - tree:\n      x: 10\n      y: 10\n      direction: right\n      root: { rows: [R] }\n      left:\n        - rows: [L]\n          name: a\n",
			`s.yaml:12:11: the name "a" is given before: give each part, row and node its own name`,
		},
		{
			"  - tree:\n      x: 10\n      y: 10\n      direction: right\n      root: { rows: [R] }\n      left:\n        - rows: [L]\n          children:\n            - rows: [Long node past the bottom of the canvas, b, c, d, e, f, g, h, i, j, k, l]\n",
			`s.yaml:13:15: box "Long node past the bottom of the canvas" sticks out of the 400x200 canvas`,
		},
		// A left node and a box written by hand: at the box, written before the tree or after.
		{
			"  - tree:\n      x: 10\n      y: 10\n      direction: right\n      root: { rows: [R] }\n      left:\n        - rows: [L]\n  - box: { x: 20, y: 10, rows: [S] }\n",
			`s.yaml:12:5: box "L" overlaps box "S"`,
		},
		{
			"  - box: { x: 20, y: 10, rows: [S] }\n  - tree:\n      x: 10\n      y: 10\n      direction: right\n      root: { rows: [R] }\n      left:\n        - rows: [L]\n",
			`s.yaml:5:5: box "S" overlaps box "L"`,
		},
	} {
		if _, got := draw(t, head+tt.parts, inspect); !strings.Contains(got, tt.want) {
			t.Errorf("%q: told\n%s\nwant it to hold\n%s", tt.parts, got, tt.want)
		}
	}
}
