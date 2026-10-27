package diagram

import (
	"math"
	"testing"
)

// grid ends whatever it takes, a coordinate far from 0, a huge radius or an overflow to -Inf: at most 501 points,
// each within the range and no NaN, the first, each next and the last at most step from where the one before
// is, and one at least for a range there is.
func TestGridBounded(t *testing.T) {
	t.Parallel()

	two55 := math.Pow(2, 55)
	for _, tt := range []struct {
		name         string
		lo, hi, size float64
		n            int     // the points, -1 for those the rules allow
		last         float64 // the last point, NaN for any
	}{
		{"far from 0", 5e19 - 100, 5e19 + 100, 1e20, -1, math.NaN()},
		{"a huge radius", 4676.5 - 9.683899191965114e+299, 4676.5 + 9.683899191965114e+299, 9353, 501, 9353},
		{"lo overflowed", math.Inf(-1), 0, 400, 1, 0},
		{"the whole float line", -1.7e308, 1.7e308, 1.7e308, -1, 1.7e308},
		{"to a point", 817.1, 1045.1, 1907, 58, 1045.1},
		{"past 2^55", two55 + 1, two55 + 1001, 2 * two55, -1, math.NaN()},
		{"at 2^55", two55, two55 + 3000, 2 * two55, -1, math.NaN()},
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
		if tt.n >= 0 && len(got) != tt.n || !math.IsNaN(tt.last) && last != tt.last {
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
