package diagram

import (
	"math"
	"math/rand/v2"
	"testing"
)

func boxOf(x, y, w, h float64, kind string) rect {
	return rect{x, y, w, h, "", "", outlineOf(w, h, kind)}
}

// An edge runs through a box when it passes inside the shape drawn 1px in from its edge: past a hexagon's points
// or a rounded corner it does not, nor along the edge or 1px in, nor touching a point; across it, it does.
func TestOutline(t *testing.T) {
	t.Parallel()

	hex := boxOf(100, 20, 200, 52, "hex")
	// The upper left side of the hexagon, (100,46) to (118,20), moved in along its normal.
	n := [2]float64{26 / math.Hypot(26, 18), 18 / math.Hypot(26, 18)}
	along := func(by float64) seg {
		return seg{100 + n[0]*by, 46 + n[1]*by, 118 + n[0]*by, 20 + n[1]*by}
	}
	for _, c := range []struct {
		name string
		r    rect
		s    seg
		want bool
	}{
		{"hex: past its upper left point", hex, seg{95, 15, 108, 28}, false},
		{"hex: 4 in from it", hex, seg{95, 15, 112, 32}, true},
		{"hex: steeply past its right point", hex, seg{310, 10, 296, 30}, false},
		{"hex: steeply into its right point", hex, seg{310, 10, 290, 34}, true},
		{"hex: along its top", hex, seg{130, 20, 270, 20}, false},
		{"hex: 1 in from its top", hex, seg{130, 21, 270, 21}, false},
		{"hex: touching its left point", hex, seg{60, 20, 100, 46}, false},
		{"hex: along a slanted side, 1 in", hex, along(1), false},
		{"hex: along a slanted side, 1.5 in", hex, along(1.5), true},
		{"hex: across", hex, seg{60, 46, 340, 46}, true},
		{"state: past its arc", boxOf(100, 110, 200, 80, "state"), seg{60, 105, 106, 118}, false},
		{"state: into its arc", boxOf(100, 110, 200, 80, "state"), seg{60, 105, 110, 121}, true},
		{"pill: past its end", boxOf(100, 220, 200, 52, "pill"), seg{60, 215, 106, 228}, false},
		{"pill: into its end", boxOf(100, 220, 200, 52, "pill"), seg{60, 215, 110, 231}, true},
		{"rect: past its corner", boxOf(100, 300, 200, 52, ""), seg{90, 290, 102, 302}, false},
		{"rect: into its corner", boxOf(100, 300, 200, 52, ""), seg{90, 290, 106, 306}, true},
		// Both ends in the corner's square, outside its circle, the run between them inside it.
		{"rect: cutting its corner's circle", boxOf(100, 300, 200, 52, ""), seg{101.2, 305, 105, 301.2}, true},
		{"table: past its corner", boxOf(100, 380, 200, 52, "table"), seg{90, 370, 102, 382}, false},
		{"table: into its corner", boxOf(100, 380, 200, 52, "table"), seg{90, 370, 106, 386}, true},
		{"bar: within 1 of its corner", boxOf(100, 463, 200, 14, "bar"), seg{90, 455, 101.3, 464.3}, false},
		{"bar: into its corner", boxOf(100, 463, 200, 14, "bar"), seg{90, 455, 103, 466}, true},
		// A tall pill's ends are half circles of half its width, and its sides straight between them.
		{"tall pill: past its round end", boxOf(400, 20, 60, 120, "pill"), seg{390, 15, 405, 30}, false},
		{"tall pill: into it", boxOf(400, 20, 60, 120, "pill"), seg{390, 15, 415, 60}, true},
		{"tall pill: along its straight side, 1 in", boxOf(400, 20, 60, 120, "pill"), seg{401, 60, 401, 100}, false},
		{"tall pill: along its straight side, 1.5 in", boxOf(400, 20, 60, 120, "pill"), seg{401.5, 60, 401.5, 100}, true},
		// Below its round corner, along its straight side: inside, where an elliptic corner (20 by 26) would not be.
		{"narrow state: its corners round, not elliptic", boxOf(100, 100, 40, 80, "state"), seg{101.2, 120.5, 101.2, 121.5}, true},
		{"icon: past its corner, which is square", boxOf(100, 300, 48, 48, "icon"), seg{90, 290, 102, 302}, true},
		{"2 by 2: no room 1 in", boxOf(100, 100, 2, 2, "pill"), seg{90, 101, 110, 101}, false},
		{"a run of no length, inside", boxOf(100, 300, 200, 52, ""), seg{200, 326, 200, 326}, false},
	} {
		if got := crossedBy(c.s, c.r); got != c.want {
			t.Errorf("%s: crossedBy(%v) = %v, want %v", c.name, c.s, got, c.want)
		}
	}

	// An edge ends at the middle of a side, which is on the outline of every shape.
	for _, kind := range []string{"", "pill", "hex", "state", "table", "bar"} {
		r := boxOf(100, 100, 200, 52, kind)
		for _, p := range [4][2]float64{{100, 126}, {300, 126}, {200, 100}, {200, 152}} {
			if d := depth(r, p[0], p[1]); math.Abs(d) > 1e-9 {
				t.Errorf("%q: the middle of a side %v is %v from its outline, want 0", kind, p, d)
			}
		}
	}
}

// depth is how far p is inside r's outline (less than 0 outside), told without crossedBy: a hexagon's distance to
// its nearest side, a round corner's to its arc, else to the nearest side of the rectangle.
func depth(r rect, px, py float64) float64 {
	o := r.form
	x0, y0, x1, y1 := r.x, r.y, r.x+r.w, r.y+r.h
	if o.hex > 0 {
		cy, half := r.y+r.h/2, r.h/2
		l := math.Hypot(half, o.hex)
		d := math.Min(py-y0, y1-py)
		for _, s := range [4][4]float64{{half, o.hex, x0, cy}, {half, -o.hex, x0, cy}, {-half, o.hex, x1, cy}, {-half, -o.hex, x1, cy}} {
			d = math.Min(d, (s[0]*(px-s[2])+s[1]*(py-s[3]))/l)
		}
		return d
	}
	edge := math.Min(math.Min(px-x0, x1-px), math.Min(py-y0, y1-py))
	c := o.radius
	if c == 0 {
		return edge
	}
	cx, cy := x0+c, y0+c
	switch {
	case px > x1-c:
		cx = x1 - c
	case px >= x0+c:
		return edge
	}
	switch {
	case py > y1-c:
		cy = y1 - c
	case py >= y0+c:
		return edge
	}
	return c - math.Hypot(px-cx, py-cy)
}

// Random runs near each shape agree with points along them: a run with a point deeper than 1 in is crossedBy (none
// missed), and a run crossedBy has a point 1 in, checked again closely if the first points miss it (none extra).
func TestOutlineRandom(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		r    rect
	}{
		{"hex", boxOf(100, 100, 200, 52, "hex")},
		{"rect", boxOf(100, 100, 200, 52, "")},
		{"pill", boxOf(100, 100, 200, 52, "pill")},
		{"state", boxOf(100, 100, 200, 80, "state")},
		{"bar", boxOf(100, 100, 200, 14, "bar")},
		{"tall pill", boxOf(100, 100, 60, 120, "pill")},
		{"narrow state", boxOf(100, 100, 40, 80, "state")},
		// Its two rectangles of no width, its four circles one.
		{"square pill", boxOf(100, 100, 60, 60, "pill")},
		{"table", boxOf(100, 100, 200, 80, "table")},
	} {
		rnd := rand.New(rand.NewPCG(11, 12)) //nolint:gosec // G404: runs to try, no secret
		pt := func() (float64, float64) {
			return c.r.x - 20 + rnd.Float64()*(c.r.w+40), c.r.y - 20 + rnd.Float64()*(c.r.h+40)
		}
		deep := func(s seg, n int, by float64) bool {
			for i := 0; i <= n; i++ {
				f := float64(i) / float64(n)
				if depth(c.r, s[0]+(s[2]-s[0])*f, s[1]+(s[3]-s[1])*f) > by {
					return true
				}
			}
			return false
		}
		const runs = 4000
		for range runs {
			x1, y1 := pt()
			x2, y2 := pt()
			s := seg{x1, y1, x2, y2}
			got := crossedBy(s, c.r)
			if !got && deep(s, 300, 1+1e-6) {
				t.Errorf("%s: %v has a point deeper than 1 in, but is not crossedBy", c.name, s)
			}
			if got && !deep(s, 300, 1-1e-6) && !deep(s, 5000, 1-1e-6) {
				t.Errorf("%s: %v is crossedBy with no point 1 in", c.name, s)
			}
		}
	}
}
