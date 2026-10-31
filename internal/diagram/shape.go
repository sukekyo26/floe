package diagram

import (
	"fmt"
	"math"
)

// outline is the shape a box is drawn in, apart from where it is: how far a hexagon's points stand in, or the
// radius of its round corners; zero is a plain rectangle. It is both drawn (el) and what an edge may not run
// through (crossedBy), so that the check reads the shape drawn.
type outline struct {
	hex    float64 // how far a hexagon's points stand in; 0 for no hexagon
	radius float64
}

// outlineOf is the outline of a w by h box of kind: a box's shape (rect, which "" is, pill, hex, state), table, bar
// or icon. Corners are round, of a radius no more than half the shorter side: a pill's ends are half circles, tall
// or wide, and no corner is an ellipse. An icon's are square.
func outlineOf(w, h float64, kind string) outline {
	radius := float64(cornerR)
	switch kind {
	case "hex":
		return outline{hex: hexInset, radius: 0}
	case "icon":
		return outline{hex: 0, radius: 0}
	case "pill":
		radius = math.Inf(1)
	case "state":
		radius = 26
	case "bar":
		radius = 3
	}
	return outline{hex: 0, radius: math.Min(radius, math.Min(w, h)/2)}
}

// el draws r in its outline: a hexagon's path, or a rectangle with round corners.
func (o outline) el(r rect) elem {
	x, y, w, h := r.x, r.y, r.w, r.h
	if o.hex > 0 {
		return elem{Kind: "path", D: fmt.Sprintf("M%s,%s H%s L%s,%s L%s,%s H%s L%s,%s Z",
			num(x+o.hex), num(y), num(x+w-o.hex), num(x+w), num(y+h/2), num(x+w-o.hex), num(y+h),
			num(x+o.hex), num(x), num(y+h/2))}
	}
	return elem{Kind: "rect", X: x, Y: y, W: w, H: h, RX: o.radius}
}

// on is where the line across side of r at t (an x on top or bottom, a y on left or right) first meets r's
// outline from that side: on the side where it is straight, else on a round corner or a hexagon's slant.
func (o outline) on(r rect, side string, t float64) [2]float64 {
	across := side == "top" || side == "bottom"
	along, length := t-r.y, r.h
	if across {
		along, length = t-r.x, r.w
	}
	in := 0.0
	switch e := math.Min(along, length-along); {
	case o.hex > 0 && across:
		// Its points are left and right: towards an end, the top and the bottom slant in to the middle's height.
		if e < o.hex {
			in = (o.hex - e) / o.hex * r.h / 2
		}
	case o.hex > 0:
		in = o.hex * math.Abs(t-r.y-r.h/2) / (r.h / 2)
	case e < o.radius:
		c := o.radius
		in = c - math.Sqrt(c*c-(c-e)*(c-e))
	}
	switch side {
	case "top":
		return [2]float64{t, r.y + in}
	case "bottom":
		return [2]float64{t, r.y + r.h - in}
	case "left":
		return [2]float64{r.x + in, t}
	default:
		return [2]float64{r.x + r.w - in, t}
	}
}

// head draws the top of r in its outline, down to y+depth: a table's heading, its corners its outline's. It draws
// round corners only, not a hexagon, as a table has no other outline.
func (o outline) head(r rect, depth float64) string {
	x, y, w, c := r.x, r.y, r.w, o.radius
	return fmt.Sprintf("M%s,%s V%s A%s,%s 0 0 1 %s,%s H%s A%s,%s 0 0 1 %s,%s V%s Z", num(x), num(y+depth), num(y+c),
		num(c), num(c), num(x+c), num(y), num(x+w-c), num(c), num(c), num(x+w), num(y+c), num(y+depth))
}

// eps is the one tolerance of crossedBy: a run counts as parallel to a side when its turn across the side is less
// than eps of its length, and a point as inside a shape when it is more than eps (in px) in, so that what is left of
// a float's rounding does not count either way.
const eps = 1e-9

// crossedBy reports whether s passes inside r's outline 1px in from its edge: an end touching the edge, or a run
// along it or within 1px of it, does not count; nor does a run past a hexagon's points or a round corner, nor a
// run of no length.
func crossedBy(s seg, r rect) bool {
	if s[0] == s[2] && s[1] == s[3] {
		return false
	}
	o := r.form
	minX, maxX, minY, maxY := r.x+1, r.x+r.w-1, r.y+1, r.y+r.h-1
	// Every outline is within its rectangle, so a run that misses the rectangle misses the outline too.
	if !throughRect(s, minX, minY, maxX, maxY) {
		return false
	}
	if o.hex > 0 {
		return throughHex(s, r, o.hex)
	}
	// A round rectangle is two rectangles across each other and a circle at each corner: a run is in it when it is
	// in one of them. A corner of radius 1 or less is sharp 1px in, and the two rectangles then hold all of it.
	c := o.radius - 1
	if throughRect(s, minX, minY+c, maxX, maxY-c) || throughRect(s, minX+c, minY, maxX-c, maxY) {
		return true
	}
	corners := [4][2]float64{{minX + c, minY + c}, {maxX - c, minY + c}, {minX + c, maxY - c}, {maxX - c, maxY - c}}
	for _, p := range corners {
		if throughCircle(s, p[0], p[1], c) {
			return true
		}
	}
	return false
}

// throughRect reports whether s passes strictly inside the rectangle: clipped to it (Liang-Barsky), what is left
// is a chord of it, and when any of the chord is strictly inside, its midpoint is too.
func throughRect(s seg, minX, minY, maxX, maxY float64) bool {
	x1, y1, x2, y2 := s[0], s[1], s[2], s[3]
	dx, dy := x2-x1, y2-y1
	length := math.Hypot(dx, dy)
	t0, t1 := 0.0, 1.0
	for _, c := range [4][2]float64{{-dx, x1 - minX}, {dx, maxX - x1}, {-dy, y1 - minY}, {dy, maxY - y1}} {
		var ok bool
		if t0, t1, ok = clip(t0, t1, c[0], c[1], length); !ok {
			return false
		}
	}
	mx, my := x1+dx*(t0+t1)/2, y1+dy*(t0+t1)/2
	return mx > minX+eps && mx < maxX-eps && my > minY+eps && my < maxY-eps
}

// clip narrows t0 to t1 to where p·t <= q, a side of Liang-Barsky's for a run of the length given; false when
// nothing is left. A run parallel to the side (p within eps of its length) is kept or dropped whole.
func clip(t0, t1, p, q, length float64) (lo, hi float64, ok bool) {
	switch {
	case math.Abs(p) <= eps*length:
		return t0, t1, q >= 0
	case p < 0:
		t0 = max(t0, q/p)
	default:
		t1 = min(t1, q/p)
	}
	return t0, t1, t0 <= t1
}

// throughCircle reports whether s passes strictly inside the circle of centre cx, cy and radius c: the line's
// roots on it, an interval of s strictly inside.
func throughCircle(s seg, cx, cy, c float64) bool {
	x0, y0 := s[0]-cx, s[1]-cy
	dx, dy := s[2]-s[0], s[3]-s[1]
	qa, qb, qc := dx*dx+dy*dy, 2*(x0*dx+y0*dy), x0*x0+y0*y0-c*c
	disc := qb*qb - 4*qa*qc
	if disc <= 0 {
		return false
	}
	sq := math.Sqrt(disc)
	t0, t1 := max(0, (-qb-sq)/(2*qa)), min(1, (-qb+sq)/(2*qa))
	if t0 >= t1 {
		return false
	}
	t := (t0 + t1) / 2
	return math.Hypot(x0+dx*t, y0+dy*t) < c-eps
}

// throughHex reports whether s passes strictly inside the hexagon in r of points standing in by in, 1px in from its
// edge: clipped to its six sides each moved 1 in (Cyrus-Beck), what is left is a chord of it, and when any of the
// chord is strictly inside, its midpoint is too.
func throughHex(s seg, r rect, in float64) bool {
	x1, y1, x2, y2 := s[0], s[1], s[2], s[3]
	dx, dy := x2-x1, y2-y1
	length := math.Hypot(dx, dy)
	half := r.h / 2
	cy := r.y + half
	l := math.Hypot(half, in)
	// Each side as its inward unit normal and a point on it: the four slanted from the side points, then the top
	// and the bottom.
	sides := [6][4]float64{
		{half / l, in / l, r.x, cy},
		{half / l, -in / l, r.x, cy},
		{-half / l, in / l, r.x + r.w, cy},
		{-half / l, -in / l, r.x + r.w, cy},
		{0, 1, r.x, r.y},
		{0, -1, r.x, r.y + r.h},
	}
	t0, t1 := 0.0, 1.0
	for _, sd := range sides {
		nx, ny, ax, ay := sd[0], sd[1], sd[2], sd[3]
		// n·(p + t·d − a) >= 1, as p·t <= q.
		var ok bool
		if t0, t1, ok = clip(t0, t1, -(nx*dx + ny*dy), nx*(x1-ax)+ny*(y1-ay)-1, length); !ok {
			return false
		}
	}
	mx, my := x1+dx*(t0+t1)/2, y1+dy*(t0+t1)/2
	for _, sd := range sides {
		if sd[0]*(mx-sd[2])+sd[1]*(my-sd[3])-1 <= eps {
			return false
		}
	}
	return true
}
