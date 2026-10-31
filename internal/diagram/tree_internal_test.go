package diagram

import "testing"

// A child right below its parent goes straight down even where a float's error moves its centre: (0.3 − 20) + 20
// is 0.3000000000000007, which turned across by no length.
func TestTreeStraightDown(t *testing.T) {
	t.Parallel()

	rows := []row{{Text: Text{"en": "A"}, Style: ""}}
	leaf := node{Name: "", Rows: rows, Tone: "", Shape: "", Children: nil}
	root := node{Name: "", Rows: rows, Tone: "", Shape: "", Children: []node{leaf}}
	items, _ := expand([]part{{Tree: &tree{X: 0.3, Y: 10, Root: root}}}) //nolint:exhaustruct_v5 // one kind of part
	e := items[1].p.Edge
	if e == nil || e.Via != nil || e.To.Point[0] != 0.3 {
		t.Errorf("edge = %+v, want one straight down at x 0.3", e)
	}
}
