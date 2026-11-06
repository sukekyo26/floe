package diagram

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
)

func TestEdgesCrossed(t *testing.T) {
	t.Parallel()

	f := rect{x: 20, y: 20, w: 100, h: 50} //nolint:exhaustruct_v5 // a frame's four sides
	for _, c := range []struct {
		name string
		a    rect
		want []edgeCrossed
	}{
		{"right", rect{x: 100, y: 30, w: 40, h: 10}, []edgeCrossed{{"x", 120, 100, 140}}},                                                                //nolint:exhaustruct_v5 // its place
		{"left", rect{x: 10, y: 30, w: 20, h: 10}, []edgeCrossed{{"x", 20, 10, 30}}},                                                                     //nolint:exhaustruct_v5 // its place
		{"top", rect{x: 40, y: 10, w: 10, h: 20}, []edgeCrossed{{"y", 20, 10, 30}}},                                                                      //nolint:exhaustruct_v5 // its place
		{"bottom", rect{x: 40, y: 60, w: 10, h: 20}, []edgeCrossed{{"y", 70, 60, 80}}},                                                                   //nolint:exhaustruct_v5 // its place
		{"a corner", rect{x: 100, y: 60, w: 40, h: 20}, []edgeCrossed{{"x", 120, 100, 140}, {"y", 70, 60, 80}}},                                          //nolint:exhaustruct_v5 // its place
		{"wider", rect{x: 10, y: 30, w: 200, h: 10}, []edgeCrossed{{"x", 20, 10, 210}, {"x", 120, 10, 210}}},                                             //nolint:exhaustruct_v5 // its place
		{"over all of it", rect{x: 0, y: 0, w: 200, h: 200}, []edgeCrossed{{"x", 20, 0, 200}, {"x", 120, 0, 200}, {"y", 20, 0, 200}, {"y", 70, 0, 200}}}, //nolint:exhaustruct_v5 // its place
		{"from an edge on", rect{x: 120, y: 30, w: 40, h: 10}, nil},                                                                                      //nolint:exhaustruct_v5 // its place
		{"inside, on the edges", rect{x: 20, y: 20, w: 100, h: 50}, nil},                                                                                 //nolint:exhaustruct_v5 // its place
	} {
		if got := edgesCrossed(c.a, f); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}

	// A frame written to the numbers told passes, though a label's end is off them by float error.
	var off int
	for _, text := range []string{"DNS", "Lambda", "Internet Gateway", "a long label on a bar", "ab", "abc", "the middle"} {
		for x := 100.0; x < 400; x++ {
			a := labelArea(Text{"en": text}, x, 60, "middle", false, "")
			edge, err := strconv.ParseFloat(num(a.x+a.w), 64)
			if err != nil {
				t.Fatal(err)
			}
			if edge != a.x+a.w {
				off++
			}
			to := rect{x: 0, y: 0, w: edge, h: 200} //nolint:exhaustruct_v5 // its place
			if got := edgesCrossed(a, to); got != nil {
				t.Errorf("%q at x %v, in a frame to %v: %v", text, x, edge, got)
			}
		}
	}
	if off == 0 {
		t.Error("no label's end was off its number, so nothing was tried")
	}

	// Whatever straddles a frame by more than float error crosses an edge of it: a mistake is never lost for want of one.
	rnd := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: rectangles to try, no secret
	straddling := 0
	for range 5000 {
		//nolint:exhaustruct_v5 // its place
		a := rect{x: rnd.Float64() * 140, y: rnd.Float64() * 90, w: rnd.Float64() * 120, h: rnd.Float64() * 70}
		if a.overlaps(f) && !a.inside(f) {
			straddling++
			if len(edgesCrossed(a, f)) == 0 {
				t.Fatalf("%+v straddles %+v and crosses none of its edges", a, f)
			}
		}
	}
	if straddling == 0 {
		t.Error("no rectangle straddled the frame")
	}
}
