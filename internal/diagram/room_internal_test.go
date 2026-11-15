package diagram

import (
	"math"
	"testing"
)

// roomOf tells a box over a circle of out by the point of the box nearest its centre: over its middle, along an
// edge, from beside it; a box clear of it has room, one touching it none.
func TestRoomOf(t *testing.T) {
	t.Parallel()

	box := rect{x: 100, y: 100, w: 80, h: 20, what: "", at: ""}
	in := circle{Name: "a", CX: 140, CY: 110, R: 200, Tone: ""}
	for _, tt := range []struct {
		name string
		out  circle
		want func(float64) bool
	}{
		{"over its middle", circle{CX: 140, CY: 110, R: 5}, func(r float64) bool { return r < 0 }},
		{"along its bottom", circle{CX: 140, CY: 124, R: 5}, func(r float64) bool { return r < 0 }},
		{"from its left", circle{CX: 96, CY: 110, R: 5}, func(r float64) bool { return r < 0 }},
		{"from its right", circle{CX: 184, CY: 110, R: 5}, func(r float64) bool { return r < 0 }},
		{"clear of it", circle{CX: 140, CY: 140, R: 5}, func(r float64) bool { return r == 15 }},
		{"touching it", circle{CX: 140, CY: 125, R: 5}, func(r float64) bool { return r == 0 }},
	} {
		if got := (region{in: []circle{in}, out: []circle{tt.out}}).roomOf(box); !tt.want(got) {
			t.Errorf("a circle %s: roomOf = %v", tt.name, got)
		}
	}
	if got := (region{in: []circle{{CX: 140, CY: 110, R: 40}}, out: nil}).roomOf(box); got >= 0 {
		t.Errorf("a box over the edge of a circle of in: roomOf = %v, want it outside", got)
	}
}

// pick takes the most room and the nearest within eps of them, measured from the most and the nearest: rounding left
// in either, or the order the tries come in, picks no other.
func TestPick(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		tries []try
		mx    float64
		want  float64 // the x picked
	}{
		{"room rounded apart", []try{{10, 0, 5}, {14, 0, 5 + 1e-13}}, 12, 10},
		{"nearness rounded apart", []try{{10, 0, 5}, {14, 0, 5}}, 12 + 1e-13, 10},
		{"a chain of rooms each within eps", []try{{370, 0, 5}, {379, 0, 5 + 0.9*eps}, {400, 0, 5 + 1.5*eps}}, 380, 379},
		{"more room, farther", []try{{380, 0, 5}, {400, 0, 6}}, 380, 400},
		{"none of room", []try{{380, 0, -1}}, 380, 0},
	} {
		best, x, _ := pick(tt.tries, tt.mx, 0)
		if x != tt.want || best < 0 || math.IsInf(best, 0) {
			t.Errorf("%s: pick = %v, %v, want x %v", tt.name, best, x, tt.want)
		}
	}
}
