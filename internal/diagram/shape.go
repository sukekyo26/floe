package diagram

import (
	"fmt"
	"math"
)

// outline is the shape a box is drawn in, apart from where it is: how far a hexagon's points stand in, or its
// corners' radii as drawn; zero is a plain rectangle. It is both drawn (el) and what an edge may not run through
// (crossedBy), so that the check reads the shape drawn.
type outline struct {
	hex    float64 // how far a hexagon's points stand in; 0 for no hexagon
	rx, ry float64
}

// sharp is a plain rectangle's outline: an icon's, and what is not a box (a label, a mark).
var sharp = outline{hex: 0, rx: 0, ry: 0}

// outlineOf is the outline of a w by h box of kind: a box's shape (rect, which "" is, pill, hex, state), table or
// bar. Corners are cut down to half of each side, as floe writes them, so that no browser cuts them otherwise.
func outlineOf(w, h float64, kind string) outline {
	radius := float64(cornerR)
	switch kind {
	case "hex":
		return outline{hex: hexInset, rx: 0, ry: 0}
	case "pill":
		radius = h / 2
	case "state":
		radius = math.Min(26, h/2)
	case "bar":
		radius = 3
	}
	return outline{hex: 0, rx: math.Min(radius, w/2), ry: math.Min(radius, h/2)}
}

// el draws r in its outline: a hexagon's path, or a rectangle with its corners' radii, ry when it is not rx.
func (o outline) el(r rect) elem {
	x, y, w, h := r.x, r.y, r.w, r.h
	if o.hex > 0 {
		return elem{Kind: "path", D: fmt.Sprintf("M%s,%s H%s L%s,%s L%s,%s H%s L%s,%s Z",
			num(x+o.hex), num(y), num(x+w-o.hex), num(x+w), num(y+h/2), num(x+w-o.hex), num(y+h),
			num(x+o.hex), num(x), num(y+h/2))}
	}
	el := elem{Kind: "rect", X: x, Y: y, W: w, H: h, RX: o.rx}
	if o.ry != o.rx {
		el.RY = o.ry
	}
	return el
}

// head draws the top of r in its outline, down to y+depth: a table's heading, its corners its outline's.
func (o outline) head(r rect, depth float64) string {
	x, y, w := r.x, r.y, r.w
	return fmt.Sprintf("M%s,%s V%s A%s,%s 0 0 1 %s,%s H%s A%s,%s 0 0 1 %s,%s V%s Z", num(x), num(y+depth), num(y+o.ry),
		num(o.rx), num(o.ry), num(x+o.rx), num(y), num(x+w-o.rx), num(o.rx), num(o.ry), num(x+w), num(y+o.ry), num(y+depth))
}

// inside is how far within the shape 1 in from a box's edge a point must be to count as in it, so that what is
// left of a float's rounding does not.
const inside = 1e-9

// crossedBy reports whether s passes inside r's outline 1px in from its edge: an end touching the edge, or a run
// along it or within 1px of it, does not count; nor does a run past a hexagon's points or a rounded corner.
func crossedBy(s seg, r rect) bool {
	o := r.form
	minX, maxX, minY, maxY := r.x+1, r.x+r.w-1, r.y+1, r.y+r.h-1
	if o.hex > 0 {
		return throughHex(s, r, o.hex)
	}
	// A rounded rectangle is two rectangles across each other and an ellipse at each corner: a run is in it when it
	// is in one of them. Corners of a radius 1 or less are sharp 1px in.
	a, b := o.rx-1, o.ry-1
	if a <= 0 || b <= 0 {
		return throughRect(s, minX, minY, maxX, maxY)
	}
	if throughRect(s, minX, minY+b, maxX, maxY-b) || throughRect(s, minX+a, minY, maxX-a, maxY) {
		return true
	}
	corners := [4][2]float64{{minX + a, minY + b}, {maxX - a, minY + b}, {minX + a, maxY - b}, {maxX - a, maxY - b}}
	for _, c := range corners {
		if throughEllipse(s, c[0], c[1], a, b) {
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
	t0, t1 := 0.0, 1.0
	for _, c := range [4][2]float64{{-dx, x1 - minX}, {dx, maxX - x1}, {-dy, y1 - minY}, {dy, maxY - y1}} {
		var ok bool
		if t0, t1, ok = clip(t0, t1, c[0], c[1]); !ok {
			return false
		}
	}
	mx, my := x1+dx*(t0+t1)/2, y1+dy*(t0+t1)/2
	return mx > minX+inside && mx < maxX-inside && my > minY+inside && my < maxY-inside
}

// clip narrows t0 to t1 to where p·t <= q, a side of Liang-Barsky's; false when nothing is left. A run parallel to
// the side (p 0, or as good as) is kept or dropped whole.
func clip(t0, t1, p, q float64) (lo, hi float64, ok bool) {
	switch {
	case math.Abs(p) < 1e-12:
		return t0, t1, q >= 0
	case p < 0:
		t0 = max(t0, q/p)
	default:
		t1 = min(t1, q/p)
	}
	return t0, t1, t0 <= t1
}

// throughEllipse reports whether s passes strictly inside the ellipse of centre cx, cy and radii a, b: the line's
// roots on it (scaled to a circle), an interval of s strictly inside.
func throughEllipse(s seg, cx, cy, a, b float64) bool {
	u0, v0 := (s[0]-cx)/a, (s[1]-cy)/b
	du, dv := (s[2]-s[0])/a, (s[3]-s[1])/b
	qa, qb, qc := du*du+dv*dv, 2*(u0*du+v0*dv), u0*u0+v0*v0-1
	if qa == 0 {
		return false
	}
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
	mu, mv := u0+du*t, v0+dv*t
	return mu*mu+mv*mv < 1-inside
}

// throughHex reports whether s passes strictly inside the hexagon in r of points standing in by in, 1px in from its
// edge: clipped to its six sides each moved 1 in (Cyrus-Beck), what is left is a chord of it, and when any of the
// chord is strictly inside, its midpoint is too.
func throughHex(s seg, r rect, in float64) bool {
	x1, y1, x2, y2 := s[0], s[1], s[2], s[3]
	dx, dy := x2-x1, y2-y1
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
		if t0, t1, ok = clip(t0, t1, -(nx*dx + ny*dy), nx*(x1-ax)+ny*(y1-ay)-1); !ok {
			return false
		}
	}
	mx, my := x1+dx*(t0+t1)/2, y1+dy*(t0+t1)/2
	for _, sd := range sides {
		if sd[0]*(mx-sd[2])+sd[1]*(my-sd[3])-1 <= inside {
			return false
		}
	}
	return true
}
