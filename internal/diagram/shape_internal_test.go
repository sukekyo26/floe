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
		{"tall pill: past its elliptic corner", boxOf(400, 20, 60, 120, "pill"), seg{390, 15, 405, 30}, false},
		{"tall pill: into it", boxOf(400, 20, 60, 120, "pill"), seg{390, 15, 415, 60}, true},
		{"icon: past its corner, which is square", rect{100, 300, 48, 48, "", "", sharp}, seg{90, 290, 102, 302}, true},
		{"2 by 2: no room 1 in", boxOf(100, 100, 2, 2, "pill"), seg{90, 101, 110, 101}, false},
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

// depth is how far p is inside r's outline (less than 0 outside, roughly), told without crossedBy: a hexagon's
// distance to its nearest side, a rounded corner's to its arc (an ellipse's found by search), else to the nearest
// side of the rectangle.
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
	if o.rx == 0 || o.ry == 0 {
		return edge
	}
	cx, cy := x0+o.rx, y0+o.ry
	switch {
	case px > x1-o.rx:
		cx = x1 - o.rx
	case px >= x0+o.rx:
		return edge
	}
	switch {
	case py > y1-o.ry:
		cy = y1 - o.ry
	case py >= y0+o.ry:
		return edge
	}
	return toEllipse(px-cx, py-cy, o.rx, o.ry)
}

// toEllipse is how far u, v is inside the ellipse of radii a, b about 0 (less than 0 outside): its distance to the
// nearest point on it, searched over the quarter u, v is in.
func toEllipse(u, v, a, b float64) float64 {
	in := (u/a)*(u/a)+(v/b)*(v/b) < 1
	au, av := math.Abs(u), math.Abs(v)
	dist := func(th float64) float64 { return math.Hypot(a*math.Cos(th)-au, b*math.Sin(th)-av) }
	best, bt := math.Inf(1), 0.0
	for i := 0; i <= 256; i++ {
		th := math.Pi / 2 * float64(i) / 256
		if d := dist(th); d < best {
			best, bt = d, th
		}
	}
	lo, hi := math.Max(0, bt-math.Pi/512), math.Min(math.Pi/2, bt+math.Pi/512)
	for range 60 {
		m1, m2 := lo+(hi-lo)/3, hi-(hi-lo)/3
		if dist(m1) < dist(m2) {
			hi = m2
		} else {
			lo = m1
		}
	}
	best = dist((lo + hi) / 2)
	if in {
		return best
	}
	return -best
}

// Random runs near each shape agree with points along them: a run with a point deeper than 1 in is crossedBy (none
// missed). A run crossedBy with no point so deep, checked again closely, is counted: none for a shape of circular
// corners or sides; for an elliptic corner, measured 1 in by an ellipse of radii 1 less (a LIMIT), some may be.
func TestOutlineRandom(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name     string
		r        rect
		elliptic bool
	}{
		{"hex", boxOf(100, 100, 200, 52, "hex"), false},
		{"rect", boxOf(100, 100, 200, 52, ""), false},
		{"pill", boxOf(100, 100, 200, 52, "pill"), false},
		{"state", boxOf(100, 100, 200, 80, "state"), false},
		{"bar", boxOf(100, 100, 200, 14, "bar"), false},
		{"tall pill", boxOf(100, 100, 60, 120, "pill"), true},
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
		const runs = 10000
		over := 0
		for range runs {
			x1, y1 := pt()
			x2, y2 := pt()
			s := seg{x1, y1, x2, y2}
			got := crossedBy(s, c.r)
			if !got && deep(s, 500, 1+1e-6) {
				t.Errorf("%s: %v has a point deeper than 1 in, but is not crossedBy", c.name, s)
			}
			if got && !deep(s, 500, 1-1e-6) && !deep(s, 50000, 1-1e-6) {
				over++
			}
		}
		if over > 0 && !c.elliptic {
			t.Errorf("%s: %d runs crossedBy with no point 1 in, want none", c.name, over)
		}
	}
}
