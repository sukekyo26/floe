package diagram

import (
	"math"
	"testing"
)

// grid gives at most 501 points, each within the range, the first, each next and the last at most step from where
// the one before is, and one at least for a range there is, at the ends of what may be written too.
func TestGridBounded(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name         string
		lo, hi, size float64
		n            int     // the points
		last         float64 // the last point, NaN for any
	}{
		{"far from 0", 999000 - 100, 999000 + 100, 1e7, 51, 999100},
		{"the largest radius", -2e6, 2e6, 1e7, 501, 2e6},
		{"to a point", 817.1, 1045.1, 1907, 58, 1045.1},
		{"one point", 10, 10, 100, 1, 10},
		{"narrower than step", -1, 2, 100, 1, 0},
		{"no range", 50, 10, 100, 0, math.NaN()},
		{"out of the canvas", 120, 130, 100, 0, math.NaN()},
	} {
		got := grid(tt.lo, tt.hi, tt.size)
		last := math.NaN()
		if len(got) > 0 {
			last = got[len(got)-1]
		}
		if len(got) != tt.n || !math.IsNaN(tt.last) && last != tt.last {
			t.Errorf("%s: grid gives %d points, the last %v; want %d, %v", tt.name, len(got), last, tt.n, tt.last)
		}
		if why := gridBroken(got, tt.lo, tt.hi, tt.size); why != "" {
			t.Errorf("%s: grid(%v, %v, %v) %s", tt.name, tt.lo, tt.hi, tt.size, why)
		}
	}
}

// gridBroken tells how points, of grid(lo, hi, size), break its rules; "" for none. The room left for rounding
// is four of the float's spacing at the range's ends.
func gridBroken(points []float64, lo, hi, size float64) string {
	from, to := max(lo, 0), min(hi, size)
	switch {
	case !(from <= to):
		if len(points) > 0 {
			return "has points for no range"
		}
		return ""
	case len(points) == 0:
		return "has no point"
	case len(points) > 501:
		return "has over 501 points"
	}
	edge := max(math.Abs(from), math.Abs(to))
	most := max(4, (to-from)/500) + 4*(math.Nextafter(edge, math.Inf(1))-edge)
	at := from
	for _, p := range points {
		switch {
		case !(p >= from && p <= to):
			return "has a point out of the range"
		case p-at > most:
			return "leaves a gap over step"
		}
		at = p
	}
	if to-at > most {
		return "ends over step before the range does"
	}
	return ""
}
