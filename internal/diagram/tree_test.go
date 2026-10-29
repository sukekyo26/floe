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
	want := `box "a" 180,10 40x29
box "B" 88,79 96x46
box "D" 116,165 40x29
box "C" 240,79 40x29
box "E" 208,165 40x29
box "F" 272,165 40x29
box "z" 340,10 40x29
edge a→"B" M200,39 V59 H136 V79
edge "B"→"D" M136,125 V165
edge a→"C" M200,39 V59 H260 V79
edge "C"→"E" M260,108 V145 H228 V165
edge "C"→"F" M260,108 V145 H292 V165
edge z→a M340,24.5 H220
`
	if mistakes != "" || got != want {
		t.Errorf("inspect =\n%s%s\nwant\n%s", got, mistakes, want)
	}
	// What is drawn is the path inspect tells: the way straight down has no turn of no length.
	svg, _ := draw(t, src, drawWith("en", nil))
	if !strings.Contains(svg, `d="M136,125 V165"`) {
		t.Errorf("drawn = %s, want the edge to D straight down", svg)
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
			`s.yaml:10:9: the name "a" is given before: give each part, row and node its own name`,
		},
		// Two nodes of one text and no name read alike: the place tells the first alone.
		{
			"  - tree:\n      x: 200\n      y: 10\n      root:\n        rows: [A]\n        children:\n          - rows: [S]\n          - rows: [S]\n  - box: { x: 150, y: 80, rows: [S] }\n",
			`s.yaml:11:13: box "S" overlaps box "S"`,
		},
		{
			"  - tree:\n      x: 20\n      y: 10\n      root:\n        rows: [A]\n        children:\n          - rows: [B]\n          - rows: [Wide child]\n",
			`s.yaml:11:13: edge "A"→"B" sticks out of the 400x200 canvas`,
		},
		{
			"  - tree: { x: 200, y: 10, root: { rows: [A] } }\n  - edge: { from: [10, 10], to: nope }\n",
			`s.yaml:6:5: edge: no part, row or node named "nope"`,
		},
	} {
		if _, got := draw(t, head+tt.parts, inspect); !strings.Contains(got, tt.want) {
			t.Errorf("%q: told\n%s\nwant it to hold\n%s", tt.parts, got, tt.want)
		}
	}
}
