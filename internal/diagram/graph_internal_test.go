package diagram

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// graphOf is a graph of boxes named names, with edges written "a>b".
// graphBoxes are g's boxes, through its groups, as written.
func graphBoxes(g *graph) []*graphBox {
	boxes, _, _ := flatten(g, "#")
	out := make([]*graphBox, len(boxes))
	for i, b := range boxes {
		out[i] = b.box
	}
	return out
}

func graphOf(names []string, edges ...string) *graph {
	g := &graph{X: 20, Y: 20, Direction: "", Nodes: nil, Edges: nil}
	for _, n := range names {
		g.Nodes = append(g.Nodes, graphNode{Box: &graphBox{Name: n, Rows: []row{{Text: Text{"en": n}, Style: ""}}, Tone: "", Shape: ""}})
	}
	for _, e := range edges {
		from, to, _ := strings.Cut(e, ">")
		g.Edges = append(g.Edges, graphEdge{From: from, To: to, Dashed: false, Bold: false, Tone: "", Start: nil, End: nil})
	}
	return g
}

// graphSpec is g as the only part of a diagram.
func graphSpec(g *graph) *spec {
	return &spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: []part{{Graph: g}}, Example: ""} //nolint:exhaustruct_v5 // one kind of part
}

// laid is g laid out and checked, as the only part of a diagram.
func laid(g *graph) *layout {
	l := newLayout(graphSpec(g), Options{Lang: "en", Icons: anyIcon{}, Seq: 1})
	l.check()
	return l
}

// routed is g laid out and checked with its ends where the routes leave them, before settleEnds moves them.
func routed(g *graph) *layout {
	l := layoutWith(graphSpec(g), Options{Lang: "en", Icons: anyIcon{}, Seq: 1}, placing{(*layout).shareEnds, func(*drawing) {}})
	l.check()
	return l
}

// grouped is g read and laid out, its groups kept, as items lays it out.
func grouped(g *graph) ([]*group, error) {
	o, _ := readGraph(g, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // its mistakes are graphFault's
	_, _, err := o.items()                                         //nolint:errcheck // the mistakes are told by laid
	return o.groups, err
}

// noLoops is g with no edge from a box to itself.
func noLoops(g *graph) *graph {
	c := *g
	c.Edges = slices.DeleteFunc(slices.Clone(g.Edges), func(e graphEdge) bool { return e.From == e.To })
	return &c
}

// loopFault is what is wrong with the loops of g, l its routes before settleEnds, or "": a loop not turning 3 times, or
// another edge turning more or less often than in the routes of g with no loop, or more or fewer crossings (a loop
// crosses nothing).
func loopFault(g *graph, l *layout) string {
	if !slices.ContainsFunc(g.Edges, func(e graphEdge) bool { return e.From == e.To }) {
		return ""
	}
	plain := routed(noLoops(g))
	if plain.failure != nil {
		return plain.failure.Error()
	}
	j := 0
	for k, e := range g.Edges {
		p := l.paths[k]
		if e.From == e.To {
			if len(p.segs) != 3 {
				return fmt.Sprintf("%s turns %d times", p.what, len(p.segs)-1)
			}
			continue
		}
		if len(p.segs) != len(plain.paths[j].segs) {
			return fmt.Sprintf("%s turns %d times by loops, %d times not", p.what, len(p.segs)-1, len(plain.paths[j].segs)-1)
		}
		j++
	}
	if a, b := crossings(l), crossings(plain); a != b {
		return fmt.Sprintf("%d crossings by loops, %d not", a, b)
	}
	return ""
}

// unlabelled is g with no label on its edges.
func unlabelled(g *graph) *graph {
	c := *g
	c.Edges = slices.Clone(g.Edges)
	for k := range c.Edges {
		c.Edges[k].Label = nil
	}
	return &c
}

// labelFault is what is wrong with the labels of g laid out as l, or "", its routes before settleEnds being r: an edge
// turning or crossing another more or less often than in the routes of g with no label, or a label not a column beside
// a run of its own edge.
func labelFault(g *graph, l, r *layout) string {
	if !slices.ContainsFunc(g.Edges, func(e graphEdge) bool { return e.Label != nil }) {
		return ""
	}
	plain := routed(unlabelled(g))
	for k, p := range r.paths {
		if len(p.segs) != len(plain.paths[k].segs) {
			return fmt.Sprintf("%s turns %d times labelled, %d times not", p.what, len(p.segs)-1, len(plain.paths[k].segs)-1)
		}
	}
	if a, b := crossings(r), crossings(plain); a != b {
		return fmt.Sprintf("%d crossings labelled, %d not", a, b)
	}
	// Across the ranks, room made and labels before the boxes move them by whole columns, keeping their grid.
	across := 0
	if g.Direction == "right" {
		across = 1
	}
	for _, b := range graphBoxes(g) {
		if x, y := l.names[b.Name].r, plain.names[b.Name].r; math.Mod(x.lo(across)-y.lo(across), column) != 0 {
			return fmt.Sprintf("box %s at %v,%v labelled, %v,%v not: off its grid", b.Name, x.x, x.y, y.x, y.y)
		}
	}
	if fault := handFault(g, l); fault != "" {
		return fault
	}
	j := 0
	for k, e := range g.Edges {
		if e.Label == nil {
			continue
		}
		r := l.labels[j]
		j++
		if e.From == e.To {
			if fault := loopLabelFault(g, l, k, r); fault != "" {
				return fault
			}
			continue
		}
		if !slices.ContainsFunc(l.paths[k].segs, func(s seg) bool { return onRun(r, s) }) {
			return fmt.Sprintf("the label of %s at %v, %v not on a run of it", l.paths[k].what, r.x, r.y)
		}
	}
	return ""
}

// loopLabelFault is what is wrong with r, the label of loop k of g laid out as l, or "": it stands a column right of
// (below) the outer loop of its box, within the box's side.
func loopLabelFault(g *graph, l *layout, k int, r rect) string {
	name := g.Edges[k].From
	outer := -1
	for k2, e := range g.Edges {
		if e.From == name && e.To == name {
			outer = k2
		}
	}
	run, box := l.paths[outer].segs[1], l.names[name].r
	across := 0
	if g.Direction == "right" {
		across = 1
	}
	along := 1 - across
	if math.Abs(r.lo(across)-run[across]-column) > 1e-6 || r.lo(along) < box.lo(along) ||
		r.lo(along)+r.size(along) > box.lo(along)+box.size(along) {
		return fmt.Sprintf("the label of %s at %v, %v not a column beside the outer loop of %s, within its side", l.paths[k].what, r.x, r.y, name)
	}
	return ""
}

// handFault is a mistake of a label of g, laid out as l, with an edge written by hand from the middle of a side of a
// box its edges do not use (left and right laid out down, top and bottom laid out right), 20 out, or "".
func handFault(g *graph, l *layout) string {
	parts := []part{{Graph: g}} //nolint:exhaustruct_v5 // one kind of part
	for _, b := range graphBoxes(g) {
		n := graphNode{Box: b, Group: nil}
		r := l.names[n.Box.Name].r
		ends := map[string][2]float64{"left": {r.x - 20, r.y + r.h/2}, "right": {r.x + r.w + 20, r.y + r.h/2}}
		loopSide := "right"
		if g.Direction == "right" {
			ends = map[string][2]float64{"top": {r.x + r.w/2, r.y - 20}, "bottom": {r.x + r.w/2, r.y + r.h + 20}}
			loopSide = "bottom"
		}
		if slices.ContainsFunc(g.Edges, func(e graphEdge) bool { return e.From == n.Box.Name && e.To == n.Box.Name }) {
			delete(ends, loopSide) // its loops use it
		}
		for _, side := range levelSides(g, l, n.Box.Name) { // and its level edges theirs
			delete(ends, side)
		}
		for _, side := range slices.Sorted(maps.Keys(ends)) {
			p := ends[side]
			parts = append(parts, part{Edge: &edge{
				From: end{Name: n.Box.Name, Point: nil}, To: end{Name: "", Point: &p}, //nolint:exhaustruct_v5 // one kind of part
				FromSide: side, ToSide: "", Via: nil, Straight: true, Label: nil, At: nil, Dashed: false, Bold: false, Tone: "",
				Start: nil, End: nil, Crossing: true, who: "", graph: "", beside: false,
			}})
		}
	}
	hands := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: parts, Example: ""}, //nolint:exhaustruct_v5 // a graph and edges
		Options{Lang: "en", Icons: NoIcons{Err: nil}, Seq: 1})
	hands.check()
	for _, p := range hands.problems {
		if slices.ContainsFunc(p.Args, func(a any) bool { return strings.HasPrefix(fmt.Sprint(a), `label "`) }) {
			return "an edge written by hand: " + p.Error()
		}
	}
	return ""
}

// onRun reports whether r stands on run s, its middle on the line, within its length.
func onRun(r rect, s seg) bool {
	along := runAxis(s)
	across := 1 - along
	lo, hi := min(s[along], s[along+2]), max(s[along], s[along+2])
	near := math.Abs(r.lo(across)+r.size(across)/2-s[across]) < 1e-6
	return near && r.lo(along) >= lo && r.lo(along)+r.size(along) <= hi
}

// crossings counts the pairs of l's edges that cross.
func crossings(l *layout) int {
	n := 0
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			_, c := touch(a, b)
			n += c
		}
	}
	return n
}

// touch reports whether two edges share a run or touch anywhere but where they cross, and counts their crossings.
func touch(a, b path) (bool, int) {
	n := 0
	for _, s := range a.segs {
		for _, q := range b.segs {
			if runsTouch(s, q) {
				return true, n
			}
			if _, _, ok := crossing(s, q); ok {
				n++
			}
		}
	}
	return false, n
}

// closeRuns counts the pairs of runs of two edges of l nearer than keepApart along a stretch they share.
func closeRuns(l *layout) int {
	n := 0
	for i, a := range l.paths {
		for _, b := range l.paths[i+1:] {
			for _, s := range a.segs {
				for _, q := range b.segs {
					if runsClose(s, q) {
						n++
					}
				}
			}
		}
	}
	return n
}

// settleFault is what settleEnds did wrong, l being g's layout and r its routes before settleEnds, or "": an edge
// turning more, more crossings, or more runs close beside another's.
func settleFault(l, r *layout) string {
	for k, p := range l.paths {
		if len(p.segs) > len(r.paths[k].segs) {
			return fmt.Sprintf("%s turns %d times, %d times before its ends moved", p.what, len(p.segs)-1, len(r.paths[k].segs)-1)
		}
	}
	if a, b := crossings(l), crossings(r); a > b {
		return fmt.Sprintf("%d crossings, %d before the ends moved", a, b)
	}
	if a, b := closeRuns(l), closeRuns(r); a > b {
		return fmt.Sprintf("%d pairs of runs nearer than %v, %d before the ends moved", a, float64(keepApart), b)
	}
	return ""
}

// boxParts counts the ends at the boxes of g laid out as l, no loop's or level edge's, and those of them at the part of
// their side they belong to. Its share of 1/50 of a side is what a share read back from the drawing can be off by.
func boxParts(g *graph, l *layout) (parted, all int) {
	type place struct{ name, side string }
	on := map[place][]float64{}
	for k, e := range g.Edges {
		if e.From == e.To || e.Level {
			continue
		}
		segs := l.paths[k].segs
		first, last := segs[0], segs[len(segs)-1]
		for j, name := range []string{e.From, e.To} {
			t, ok := l.names[name]
			if !ok || t.kind != "box" {
				continue
			}
			p, q := [2]float64{first[0], first[1]}, [2]float64{first[2], first[3]}
			if j == 1 {
				p, q = [2]float64{last[2], last[3]}, [2]float64{last[0], last[1]}
			}
			side := leaves(p, q)
			axis := sideAxis(side)
			on[place{name, side}] = append(on[place{name, side}], (p[axis]-t.r.lo(axis))/t.r.size(axis))
		}
	}
	for _, ends := range on {
		slices.Sort(ends)
		for i, v := range ends {
			all++
			if math.Abs(v-float64(i+1)/float64(len(ends)+1)) < 0.02 {
				parted++
			}
		}
	}
	return parted, all
}

// graphFault is what is wrong with g laid out, or "": a mistake told, two of its edges sharing a run or touching,
// an end off its box's outline, more crossings drawn than its ranks' order has, or more turns or crossings by moving
// the ends.
func graphFault(g *graph) string {
	return laidFault(g, laid(g))
}

// laidFault is graphFault of g, laid out as l.
func laidFault(g *graph, l *layout) string {
	if l.failure != nil {
		return l.failure.Error()
	}
	if len(l.problems) > 0 {
		return l.problems.Error()
	}
	r := routed(g)
	if fault := loopFault(g, r); fault != "" {
		return fault
	}
	if fault := labelFault(g, l, r); fault != "" {
		return fault
	}
	if fault := settleFault(l, r); fault != "" {
		return fault
	}
	if fault := levelFault(g, l); fault != "" {
		return fault
	}
	drawn := 0
	for i, a := range l.paths {
		for j, b := range l.paths[i+1:] {
			bad, n := touch(a, b)
			if bad {
				return a.what + " touches " + b.what
			}
			if !g.Edges[i].Level && !g.Edges[i+1+j].Level { // a level edge crosses the edges passing its rank
				drawn += n
			}
		}
	}
	shape := map[string]string{}
	for _, b := range graphBoxes(g) {
		shape[b.Name] = b.Shape
	}
	for k, p := range l.paths {
		e := g.Edges[k]
		ends := [2][2]float64{{p.segs[0][0], p.segs[0][1]}, {p.segs[len(p.segs)-1][2], p.segs[len(p.segs)-1][3]}}
		for j, name := range []string{e.From, e.To} {
			r := l.names[name].r
			if _, ok := shape[name]; !ok { // a group's: on its frame's top or bottom (left or right, laid out right)
				if !onFrameSide(r, ends[j], g.Direction == "right") {
					return fmt.Sprintf("%s ends at %v, off the top and bottom of %s's frame %v", p.what, ends[j], name, r)
				}
				continue
			}
			r.form = outlineOf(r.w, r.h, shape[name])
			if d := depth(r, ends[j][0], ends[j][1]); math.Abs(d) > 1e-9 {
				return fmt.Sprintf("%s ends %v from the outline of %s", p.what, d, name)
			}
		}
	}
	ordered := 0
	groups, err := grouped(g)
	if err != nil {
		return err.Error()
	}
	least := math.Inf(1)
	for _, n := range groups[0].nodes {
		least = min(least, n.x)
	}
	if least < 0 || least >= column {
		return fmt.Sprintf("the first group starts at %v, not within a column right of 0", least)
	}
	for _, gr := range groups {
		ordered += gr.crossings()
		if fault := groupFault(gr); fault != "" {
			return fault
		}
	}
	if drawn > ordered {
		return fmt.Sprintf("%d crossings drawn, over the %d of its ranks' order", drawn, ordered)
	}
	if again := laid(g); again.inspect() != l.inspect() {
		return "laid out twice, it differs"
	}
	if fault := laneFault(g, l); fault != "" {
		return fault
	}
	return clusterFault(g, l)
}

// laneFault is what is wrong with the lanes of g laid out as l, or "": a lane not as long as the one written before
// it, from the same place along the ranks, or not after it, nodeGap apart.
func laneFault(g *graph, l *layout) string {
	_, clusters, _ := flatten(g, "#")
	frames := map[string]rect{}
	for _, f := range l.frames {
		frames[f.at] = f
	}
	along, across := 1, 0
	if g.Direction == "right" {
		along, across = 0, 1
	}
	var last *rect
	for _, c := range clusters {
		if !c.lane {
			continue
		}
		f := frames["#/parts/0/graph"+strings.TrimPrefix(c.at, "#")]
		if last != nil {
			if f.lo(along) != last.lo(along) || f.size(along) != last.size(along) {
				return fmt.Sprintf("lane %v is not from end to end as lane %v before it", f, *last)
			}
			if gap := f.lo(across) - last.lo(across) - last.size(across); math.Abs(gap-nodeGap) > 1e-9 {
				return fmt.Sprintf("lane %v stands %v after lane %v, not nodeGap", f, gap, *last)
			}
		}
		last = &f
	}
	return ""
}

// onFrameSide reports whether p is on r's top or bottom (left or right, right), within its other sides.
func onFrameSide(r rect, p [2]float64, right bool) bool {
	along, across := 0, 1
	if right {
		along, across = 1, 0
	}
	return (near(p[across], r.lo(across)) || near(p[across], r.lo(across)+r.size(across))) &&
		p[along] > r.lo(along) && p[along] < r.lo(along)+r.size(along)
}

// clusterFault is what is wrong with the groups of g laid out as l, or "": a frame not holding a box of its group
// (nested too) or the corners of an edge between them, holding a box of none of them, a frame outside or touching
// another but within it (nested), one above or left of the graph's x and y, or an edge's end at a frame under its
// heading.
func clusterFault(g *graph, l *layout) string {
	boxes, clusters, _ := flatten(g, "#")
	if len(clusters) == 0 {
		return ""
	}
	frames := map[string]rect{}
	for _, f := range l.frames {
		frames[f.at] = f
	}
	within := func(r, f rect) bool {
		return r.x > f.x && r.y > f.y && r.x+r.w < f.x+f.w && r.y+r.h < f.y+f.h
	}
	frameOf := func(c int) rect { return frames["#/parts/0/graph"+strings.TrimPrefix(clusters[c].at, "#")] }
	held := func(c int, name string) bool {
		for _, b := range clusters[c].boxes {
			if boxes[b].box.Name == name {
				return true
			}
		}
		for d := range clusters {
			for p := d; p >= 0; p = clusters[p].parent {
				if p == c && clusters[d].g.Name == name && name != "" {
					return true
				}
			}
		}
		return false
	}
	if s := frameStart(g, l); s != "" {
		return s
	}
	for c := range clusters {
		f := frameOf(c)
		for i, b := range boxes {
			r := l.names[b.box.Name].r
			switch in := slices.Contains(clusters[c].boxes, i); {
			case in && !within(r, f):
				return fmt.Sprintf("the frame of group %d %v does not hold its box %s %v", c, f, b.box.Name, r)
			case !in && r.overlaps(f):
				return fmt.Sprintf("the frame of group %d %v holds box %s %v, of none of its", c, f, b.box.Name, r)
			}
		}
		for k, e := range g.Edges {
			if k >= len(l.paths) || !held(c, e.From) || !held(c, e.To) {
				continue
			}
			for _, s := range l.paths[k].segs {
				if s[0] < f.x || s[2] < f.x || s[0] > f.x+f.w || s[2] > f.x+f.w || s[1] < f.y || s[3] < f.y ||
					s[1] > f.y+f.h || s[3] > f.y+f.h {
					return fmt.Sprintf("%s, of group %d, runs out of its frame %v", l.paths[k].what, c, f)
				}
			}
		}
		for d := c + 1; d < len(clusters); d++ {
			o := frameOf(d)
			nested := false
			for p := clusters[d].parent; p >= 0; p = clusters[p].parent {
				nested = nested || p == c
			}
			grown := rect{x: f.x - 1, y: f.y - 1, w: f.w + 2, h: f.h + 2, what: "", at: ""} //nolint:exhaustruct_v5 // a frame touching
			switch {
			case nested && !within(o, f):
				return fmt.Sprintf("the frame of group %d %v, nested in group %d's, is not within it %v", d, o, c, f)
			case !nested && grown.overlaps(o):
				return fmt.Sprintf("the frames of groups %d %v and %d %v meet", c, f, d, o)
			}
		}
		if t := clusters[c].g.Label; t != nil {
			head := rect{x: f.x + 12, y: f.y + 9, w: textWidth(t, frameClass("")), h: labelH, what: "", at: ""} //nolint:exhaustruct_v5 // as addFrame's
			for _, p := range l.paths {
				for _, end := range [][2]float64{{p.segs[0][0], p.segs[0][1]}, {p.segs[len(p.segs)-1][2], p.segs[len(p.segs)-1][3]}} {
					if end[0] >= head.x && end[0] <= head.x+head.w && end[1] >= f.y && end[1] <= head.y+head.h {
						return fmt.Sprintf("%s ends at %v, under the heading of group %d", p.what, end, c)
					}
				}
			}
		}
	}
	return ""
}

// frameStart is what is wrong with where a graph's frames start, or "": none starts above or left of the graph's x
// and y, and the frame starting first on an axis, with nothing drawn before it, starts at x (or y).
func frameStart(g *graph, l *layout) string {
	at := [2]float64{g.X, g.Y}
	for axis := range 2 {
		frame, other := math.Inf(1), math.Inf(1)
		for _, f := range l.frames {
			frame = min(frame, f.lo(axis))
		}
		for _, r := range slices.Concat(l.boxes, l.labels) {
			other = min(other, r.lo(axis))
		}
		for _, p := range l.paths {
			for _, s := range p.segs {
				other = min(other, s[axis], s[axis+2])
			}
		}
		if frame < at[axis] || frame <= other && frame != at[axis] {
			return fmt.Sprintf("the first frame on axis %d starts at %v, the graph at %v", axis, frame, at[axis])
		}
	}
	return ""
}

// freeFault is a point of gr entered bent at a column of its own (x ≡ 0) more than freeLimit from where it may stand,
// or that a box above stands over or another link of its channel leaves by, or "".
func freeFault(gr *group) string {
	for r := 1; r < len(gr.ranks); r++ {
		last, right := -1, 0.0 // as settle walks the rank
		for k, v := range gr.ranks[r] {
			n := gr.nodes[v]
			if !n.settles() {
				continue
			}
			lo, hi := gr.bounds(r, k, last, right)
			last, right = k, n.x+n.w
			if n.w != 0 || gr.links[n.ins[0]].straight {
				continue
			}
			// The boxes as they stand now (widen may have widened them) pack no looser than settle saw them: t
			// here lies between settle's and the point, so a point freeLimit from settle's t is here too.
			if t := min(max(gr.links[n.ins[0]].exit, lo), hi); math.Abs(n.x-t) > freeLimit {
				return fmt.Sprintf("a point at its own column %v, %v from where it may stand", n.x, math.Abs(n.x-t))
			}
			for _, u := range gr.ranks[r-1] {
				o := gr.nodes[u]
				if o.box != nil && n.x >= o.x+inEnd && n.x <= o.x+o.w-inEnd {
					return fmt.Sprintf("a point at its own column %v under box %s %v..%v", n.x, o.box.Name, o.x, o.x+o.w)
				}
				for _, l := range o.outs {
					if l != n.ins[0] && gr.links[l].exit == n.x {
						return fmt.Sprintf("a point at its own column %v where link %d leaves", n.x, l)
					}
				}
			}
		}
	}
	return ""
}

// bentAt reports whether point n stands as one entered bent does: at a column of its own (x ≡ 0, leaving where it
// enters), or entered at x ≡ 6 and left half a column on.
func bentAt(n gnode) bool {
	return n.w == 0 && math.Mod(n.x, column) == 0 || n.w == column/2 && math.Mod(n.x, column) == column/2
}

// groupFault is what is wrong with a group as wired, or "": a box too narrow for its links, a point off the grid
// (left at x ≡ 0; entered at x ≡ 6, straight, or bent at a free column of its own, x ≡ 0), two nodes of a rank too
// close (boxes nodeGap from boxes and points, points a column from points), or a cycle in a channel's track
// constraints.
func groupFault(gr *group) string {
	for _, n := range gr.nodes {
		if n.box != nil && max(len(n.ins), len(n.outs)) > 0 && n.w < float64(12*max(len(n.ins), len(n.outs))+36) {
			return fmt.Sprintf("box %s %v wide for %d links", n.box.Name, n.w, max(len(n.ins), len(n.outs)))
		}
	}
	for _, l := range gr.links {
		// A bent link enters a box at x ≡ 6, and a point at x ≡ 6 or at a column of its own, x ≡ 0.
		bentOff := !l.straight && math.Mod(l.entry, column) != column/2 && (!gr.nodes[l.to].settles() || math.Mod(l.entry, column) != 0)
		if math.Mod(l.exit, column) != 0 || bentOff || l.straight != (l.entry == l.exit) {
			return fmt.Sprintf("a link off the grid: exit %v, entry %v, straight %v", l.exit, l.entry, l.straight)
		}
	}
	// A point entered bent at a column of its own stands under no box above, past 12 in from its ends, and where no
	// other link of the channel leaves.
	if s := freeFault(gr); s != "" {
		return s
	}
	// A point stands at its link's column, a column past what stands before it, or nodeGap past boxes between;
	// only one pushed off it is entered bent.
	for _, rank := range gr.ranks {
		last := -1
		for k, v := range rank {
			n := gr.nodes[v]
			if !n.settles() {
				continue
			}
			e := gr.links[n.ins[0]].exit
			if len(gr.cls) > 0 { // a mark of a group's stops it short of its column, entered bent either way
				if n.x != e && !bentAt(n) {
					return fmt.Sprintf("a point off its link's column %v not entered bent: column %v", e, n.x)
				}
				last = k
				continue
			}
			room := math.Inf(-1)
			if last >= 0 {
				o := gr.nodes[rank[last]]
				at, gap := o.x+o.w, column
				for _, b := range rank[last+1 : k] {
					at, gap = onGrid(at+nodeGap, 6, math.Ceil)+gr.nodes[b].w, nodeGap
				}
				room = onGrid(at+gap, 6, math.Ceil)
			}
			// A point's column is its x, and it leaves w right of it.
			switch {
			case e >= room && (n.x != e || n.w != 0):
				return fmt.Sprintf("a point off its link's column %v with room: column %v, leaving %v on", e, n.x, n.w)
			case e < room && (!bentAt(n) || n.x < room):
				return fmt.Sprintf("a point pushed off its link's column %v not entered bent: column %v", e, n.x)
			}
			last = k
		}
	}
	for r, rank := range gr.ranks {
		for k := 1; k < len(rank); k++ {
			a, b := gr.nodes[rank[k-1]], gr.nodes[rank[k]]
			want := gr.gap(rank[k-1], rank[k])
			if b.x-(a.x+a.w) < want {
				return fmt.Sprintf("rank %d: nodes %v apart, want %v", r, b.x-(a.x+a.w), want)
			}
		}
		if r+1 == len(gr.ranks) {
			continue
		}
		var bent []int
		for _, u := range rank {
			for _, l := range gr.nodes[u].outs {
				if !gr.links[l].straight {
					bent = append(bent, l)
				}
			}
		}
		above := gr.above(bent)
		done := map[int]bool{}
		for changed := true; changed; {
			changed = false
			for _, l := range bent {
				if done[l] {
					continue
				}
				if !slices.ContainsFunc(above[l], func(a int) bool { return !done[a] }) {
					done[l], changed = true, true
				}
			}
		}
		if len(done) != len(bent) {
			return fmt.Sprintf("channel %d: a cycle in the track constraints", r)
		}
	}
	return ""
}

// shrunk is g with boxes and edges taken off while it still fails, for a failing graph to keep in testdata/graph.
func shrunk(g *graph) *graph {
	for changed := true; changed; {
		changed = false
		for i := len(g.Edges) - 1; i >= 0; i-- {
			try := *g
			try.Edges = slices.Delete(slices.Clone(g.Edges), i, i+1)
			if graphFault(&try) != "" {
				g, changed = &try, true
			}
		}
		for i := len(g.Nodes) - 1; i >= 0 && len(g.Nodes) > 1; i-- {
			if g.Nodes[i].Box == nil { // a group stays whole
				continue
			}
			name := g.Nodes[i].Box.Name
			try := *g
			try.Nodes = withoutWith(slices.Delete(slices.Clone(g.Nodes), i, i+1), name)
			try.Edges = slices.DeleteFunc(slices.Clone(g.Edges), func(e graphEdge) bool { return e.From == name || e.To == name })
			if graphFault(&try) != "" {
				g, changed = &try, true
			}
		}
	}
	return g
}

// withoutWith is nodes, copied, none with a with of name: a graph shrunk fails as it did, not of a with naming nothing.
func withoutWith(nodes []graphNode, name string) []graphNode {
	out := slices.Clone(nodes)
	for i, n := range out {
		switch {
		case n.Box != nil && n.Box.With == name:
			b := *n.Box
			b.With = ""
			out[i].Box = &b
		case n.Icon != nil && n.Icon.With == name:
			c := *n.Icon
			c.With = ""
			out[i].Icon = &c
		case n.Group != nil:
			g := *n.Group
			g.Nodes = withoutWith(g.Nodes, name)
			out[i].Group = &g
		}
	}
	return out
}

// randomGraph is a graph as dagro's trials drew them: 2 to 31 boxes of four shapes and words of some length, 1 to
// 62 edges between two of them at random, cycles and a second edge between two boxes among them.
func randomGraph(rnd *rand.Rand, direction string) *graph {
	n := 2 + rnd.IntN(30)
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("n%d", i)
	}
	g := graphOf(names)
	g.Direction = direction
	shapes := []string{"", "pill", "hex", "state"}
	for _, nd := range g.Nodes {
		nd.Box.Shape = shapes[rnd.IntN(len(shapes))]
		nd.Box.Rows[0].Text["en"] = strings.Repeat("w", 1+rnd.IntN(16))
	}
	for range 1 + rnd.IntN(62) {
		a, b := rnd.IntN(n), rnd.IntN(n)
		if a == b {
			continue
		}
		g.Edges = append(g.Edges, graphEdge{From: names[a], To: names[b], Dashed: false, Bold: false, Tone: "", Start: nil, End: nil, Label: nil})
	}
	if rnd.IntN(3) == 0 {
		for k := range g.Edges {
			if rnd.IntN(3) == 0 {
				g.Edges[k].Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(12))}
			}
		}
	}
	return g
}

// addLoops adds to half of the graphs g, at random from rnd, 1 or 2 loops on a box of four, half of them labelled
// (every box and every loop, every).
func addLoops(rnd *rand.Rand, g *graph, every bool) {
	if !every && rnd.IntN(2) == 0 {
		return
	}
	for _, b := range graphBoxes(g) {
		n := graphNode{Box: b, Group: nil}
		if !every && rnd.IntN(4) != 0 {
			continue
		}
		for range 1 + rnd.IntN(2) {
			e := graphEdge{From: n.Box.Name, To: n.Box.Name, Dashed: false, Bold: false, Tone: "", Start: nil, End: nil, Label: nil}
			if every || rnd.IntN(2) == 0 {
				e.Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(12))}
			}
			g.Edges = append(g.Edges, e)
		}
	}
}

// Random graphs, down and right, lay out with no mistake, no two edges sharing a run or touching, every end on its
// box's outline, no more crossings drawn than their ranks' order has, no more turns or crossings by moving the ends,
// and the same layout twice; their loops (half of the graphs) leave the other edges as with no loop.
func TestGraphRandom(t *testing.T) {
	t.Parallel()

	// Then, graphs with a label on every edge, in English and Japanese, crowding labels into every channel; then,
	// graphs in lanes; last, graphs with level edges between boxes picked at random, with no group (plain), groups
	// and lanes, those told only of their level edges skipped, and at least least of them laid out.
	// LIMIT: each graph lays out several times over, its ends moved or not, and the time of the job that runs them all
	// (Coverage gate) sets the counts. Raise them if the moves come to be cheaper.
	for _, c := range []struct {
		direction string
		seed      [2]uint64
		graphs    int
		every     bool
		lanes     bool
		level     string
		least     int
		parts     int // the least share of ends at boxes on their equal parts, in percent, a little under what it measures
	}{
		{"", [2]uint64{1, 2}, 1000, false, false, "", 0, 69},
		{"right", [2]uint64{3, 4}, 1000, false, false, "", 0, 77},
		{"", [2]uint64{5, 6}, 150, true, false, "", 0, 47},
		{"right", [2]uint64{7, 8}, 150, true, false, "", 0, 64},
		{"", [2]uint64{9, 10}, 400, false, true, "", 0, 77},
		{"right", [2]uint64{11, 12}, 400, false, true, "", 0, 84},
		{"", [2]uint64{13, 14}, 300, false, false, "plain", 132, 68},
		{"right", [2]uint64{15, 16}, 300, false, false, "plain", 123, 76},
		{"", [2]uint64{17, 18}, 300, false, false, "groups", 97, 71},
		{"right", [2]uint64{19, 20}, 300, false, false, "groups", 107, 80},
		{"", [2]uint64{21, 22}, 300, false, true, "lanes", 91, 78},
		{"right", [2]uint64{23, 24}, 300, false, true, "lanes", 88, 84},
	} {
		// A name of no mark -run reads as a pattern: down-1-2 (down for the direction left out); -run
		// TestGraphRandom/down-1-2$ picks that one alone, as down-1 is down-13-14 and down-17-18 too.
		t.Run(fmt.Sprintf("%s-%d-%d", cmpOr(c.direction, "down"), c.seed[0], c.seed[1]), func(t *testing.T) {
			t.Parallel()

			rnd := rand.New(rand.NewPCG(c.seed[0], c.seed[1]))       //nolint:gosec // G404: graphs to try, no secret
			loops := rand.New(rand.NewPCG(c.seed[0]+10, c.seed[1]))  //nolint:gosec // G404: graphs to try, no secret
			groups := rand.New(rand.NewPCG(c.seed[0]+20, c.seed[1])) //nolint:gosec // G404: graphs to try, no secret
			lanes := rand.New(rand.NewPCG(c.seed[0]+30, c.seed[1]))  //nolint:gosec // G404: graphs to try, no secret
			levels := rand.New(rand.NewPCG(c.seed[0]+40, c.seed[1])) //nolint:gosec // G404: graphs to try, no secret
			laidOut, parted, ends, short, written := 0, 0, 0, 0, 0
			for i := range tries(c.graphs) {
				g := randomGraph(rnd, c.direction)
				for k := range g.Edges {
					if c.every {
						g.Edges[k].Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(10)), "ja": strings.Repeat("字", rnd.IntN(16))}
					}
				}
				addLoops(loops, g, c.every)
				if c.level != "plain" {
					addGroups(groups, g)
				}
				if c.lanes {
					inLanes(lanes, g)
				}
				if c.level != "" {
					if addLevels(levels, g); levelTold(g) {
						continue
					}
					laidOut++
				}
				l := laid(g)
				if fault := laidFault(g, l); fault != "" {
					b, _ := json.Marshal(map[string]any{"title": "kept", "parts": []any{map[string]any{"graph": shrunk(g)}}}) //nolint:errcheck,errchkjson // plain values
					t.Fatalf("seed %v, graph %d: %s; shrunk, to keep in testdata/graph:\n%s", c.seed, i, fault, b)
				}
				on, all := boxParts(g, l)
				parted, ends = parted+on, ends+all
				if fault := turnFault(g, l); fault != "" {
					t.Fatalf("seed %v, graph %d: %s", c.seed, i, fault)
				}
				clipped, labels := endsClipped(g, l)
				short, written = short+clipped, written+labels
			}
			if underRace {
				return // the bounds below are measured on all of the graphs
			}
			if 1000*short > 5*written {
				t.Errorf("seed %v: %d of %d labels leave less than %v of their run at an end, over 0.5%%", c.seed, short, written, labelEnds)
			}
			if share := 100 * parted / ends; share < c.parts {
				t.Errorf("seed %v: %d of %d ends at boxes on their equal parts, under %d%%", c.seed, parted, ends, c.parts)
			}
			if laidOut < c.least {
				t.Errorf("seed %v: %d graphs with level edges laid out, fewer than %d", c.seed, laidOut, c.least)
			}
		})
	}
}

// turnFault is an edge of g, of boxes only, laid out as l, turning more than once beyond its turns with no label, or "".
// Among icons an edge may turn more, with labels or not (LIMIT in graph.go).
func turnFault(g *graph, l *layout) string {
	plain := laid(unlabelled(g))
	for k, p := range l.paths {
		if more := len(p.segs) - len(plain.paths[k].segs); more > 1 {
			return fmt.Sprintf("%s turns %d more times labelled: %v, %v", p.what, more, p.segs, plain.paths[k].segs)
		}
	}
	return ""
}

// endsClipped is how many labels of g on edges, no loop's, laid out as l, leave less than labelEnds of their run at an
// end, and how many there are.
func endsClipped(g *graph, l *layout) (clipped, labels int) {
	j := 0
	for k, e := range g.Edges {
		if e.Label == nil {
			continue
		}
		r := l.labels[j]
		j++
		if e.From == e.To {
			continue
		}
		labels++
		for _, s := range l.paths[k].segs {
			if along := runAxis(s); onRun(r, s) {
				if r.lo(along)-min(s[along], s[along+2]) < labelEnds-1e-6 || max(s[along], s[along+2])-r.lo(along)-r.size(along) < labelEnds-1e-6 {
					clipped++
				}
				break
			}
		}
	}
	return clipped, labels
}

// levelSides are the sides of box name its level edges take, laid out as l: towards the other box.
func levelSides(g *graph, l *layout, name string) []string {
	before, after, axis := "left", "right", 0
	if g.Direction == "right" {
		before, after, axis = "top", "bottom", 1
	}
	var out []string
	for _, e := range g.Edges {
		other := e.To
		if e.To == name {
			other = e.From
		}
		if !e.Level || e.From == e.To || e.From != name && e.To != name {
			continue
		}
		if l.names[other].r.lo(axis) < l.names[name].r.lo(axis) {
			out = append(out, before)
		} else {
			out = append(out, after)
		}
	}
	return out
}

// levelFault is what is wrong with the level edges of g laid out as l, or "": an edge of more than one run, or
// between boxes whose middles across the ranks differ, which stand in two ranks.
func levelFault(g *graph, l *layout) string {
	along := 1
	if g.Direction == "right" {
		along = 0
	}
	for k, e := range g.Edges {
		if !e.Level || e.From == e.To {
			continue
		}
		a, b := l.names[e.From].r, l.names[e.To].r
		if len(l.paths[k].segs) != 1 || math.Abs(a.lo(along)+a.size(along)/2-b.lo(along)-b.size(along)/2) > eps {
			return fmt.Sprintf("%s runs %v between %v and %v", l.paths[k].what, l.paths[k].segs, a, b)
		}
	}
	return ""
}

// addLevels adds to g, at random from rnd, 1 to 3 edges written level between two boxes, half of them labelled.
func addLevels(rnd *rand.Rand, g *graph) {
	boxes := graphBoxes(g)
	for range 1 + rnd.IntN(3) {
		a, b := boxes[rnd.IntN(len(boxes))].Name, boxes[rnd.IntN(len(boxes))].Name
		if a == b {
			continue
		}
		e := graphEdge{From: a, To: b, Label: nil, Dashed: false, Bold: false, Tone: "", Start: nil, End: nil, Level: true}
		if rnd.IntN(2) == 0 {
			e.Label = Text{"en": strings.Repeat("w", 1+rnd.IntN(12))}
		}
		g.Edges = append(g.Edges, e)
	}
}

// levelTold reports whether g is told, and told only of mistakes of its level edges (and of boxes among a group's
// ranks, which a row makes too).
func levelTold(g *graph) bool {
	l := laid(g)
	for _, p := range l.problems {
		if !strings.HasPrefix(p.Key, "diagram_graph_level_") && p.Key != "diagram_graph_among" {
			return false
		}
	}
	return l.failure == nil && len(l.problems) > 0
}

// writtenOrder is a ranked graph whose written order crosses once, in a pair at the left of two ranks, and nowhere
// else: boxes of each rank linked from the rank above in one order, a fan of 20 leaves of one median, boxes with no
// link below, and edges two ranks long from right end to right end. It returns the graph and the boxes of each
// rank as they should stand: as written, but h1 and h2 swapped.
func writtenOrder(rnd *rand.Rand) (*graph, [][]string) {
	count := 3 + rnd.IntN(6)
	gadget := 1 + rnd.IntN(count-2)
	fan := rnd.IntN(count - 1)
	ranks := make([][]string, 0, count)
	idx := 0
	name := func() string {
		idx++
		return fmt.Sprintf("n%03d", 999-idx) // names in the order opposite to the written one
	}
	for r := range count {
		var rank []string
		if r == gadget {
			rank = append(rank, "g1", "g2")
		}
		if r == gadget+1 {
			rank = append(rank, "h1", "h2")
		}
		for range 1 + rnd.IntN(4) {
			rank = append(rank, name())
		}
		ranks = append(ranks, rank)
	}
	var edges []string
	for r := 1; r < count; r++ {
		up, low := ranks[r-1], ranks[r]
		start := 0
		if r == gadget {
			edges = append(edges, up[0]+">g1", up[0]+">g2")
			start = 2
		}
		if r == gadget+1 {
			edges = append(edges, "g1>h2", "g2>h1")
			start = 2
		}
		if r == gadget+1 || r == gadget+2 {
			up = up[2:] // the pair links only to itself
		}
		// Each box below takes a run of the boxes above, the runs in one order; a box above may be skipped (no link
		// below), but not in rank 0, which would then stand alone.
		cur := 0
		for j, b := range low[start:] {
			lo := cur
			hi := min(len(up)-1, lo+rnd.IntN(2))
			if j == len(low[start:])-1 {
				hi = len(up) - 1
			}
			for p := lo; p <= hi; p++ {
				edges = append(edges, up[p]+">"+b)
			}
			cur = hi
			if r > 1 && cur+1 < len(up) && rnd.IntN(3) == 0 {
				cur++
			}
		}
	}
	// A fan of 20 leaves from the right end of a rank, at the right end of the next.
	parent := ranks[fan][len(ranks[fan])-1]
	for range 20 {
		leaf := name()
		ranks[fan+1] = append(ranks[fan+1], leaf)
		edges = append(edges, parent+">"+leaf)
	}
	for r := 0; r+2 < count; r += 2 {
		edges = append(edges, ranks[r][len(ranks[r])-1]+">"+ranks[r+2][len(ranks[r+2])-1])
	}
	var names []string
	for _, rank := range ranks {
		names = append(names, rank...)
	}
	want := make([][]string, len(ranks))
	for r, rank := range ranks {
		want[r] = slices.Clone(rank)
		if r == gadget+1 {
			want[r][0], want[r][1] = "h2", "h1"
		}
	}
	return graphOf(names, edges...), want
}

// Ranked graphs that cross once at their left lay out as written but for that pair, with nothing crossing: ties
// of one median keep their written order, a box with no link below keeps its place, and the order is sorted.
func TestGraphWrittenOrder(t *testing.T) {
	t.Parallel()

	rnd := rand.New(rand.NewPCG(7, 8)) //nolint:gosec // G404: graphs to try, no secret
	for i := range tries(1000) {
		g, want := writtenOrder(rnd)
		groups, err := grouped(g)
		if err != nil || len(groups) != 1 {
			t.Fatalf("graph %d: %d groups, want 1: %v", i, len(groups), err)
		}
		gr := groups[0]
		got := make([][]string, 0, len(gr.ranks))
		for _, rank := range gr.ranks {
			var names []string
			for _, n := range rank {
				if b := gr.nodes[n].box; b != nil {
					names = append(names, b.Name)
				}
			}
			slices.SortStableFunc(names, func(a, b string) int {
				return int(gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == a })].x -
					gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == b })].x)
			})
			got = append(got, names)
		}
		if !slices.EqualFunc(got, want, slices.Equal) {
			t.Fatalf("graph %d: ranks %v, want %v", i, got, want)
		}
		if fault := graphFault(g); fault != "" {
			t.Fatalf("graph %d: %s", i, fault)
		}
	}
}

// order stops at maxTrips on ranks that swing between two orders, keeping the first of fewest crossings; at a
// trip that changes nothing, crossings left; and once nothing crosses.
func TestGraphTrips(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		g     *graph
		trips int
		want  [][]string
	}{
		{
			"swinging", graphOf([]string{"0a", "0b", "1a", "1b", "1c", "1d", "2a", "2b", "2c"},
				"0a>1b", "0a>1c", "0a>1d", "0b>1a", "0b>1b", "1a>2a", "1a>2b", "1a>2c", "1c>2c", "1d>2a", "1d>2b"),
			maxTrips,
			[][]string{{"0b", "0a"}, {"1c", "1a", "1b", "1d"}, {"2c", "2a", "2b"}},
		},
		{"K2,2, crossing as written", graphOf([]string{"a", "b", "x", "y"}, "a>x", "a>y", "b>x", "b>y"), 1, [][]string{{"a", "b"}, {"x", "y"}}},
		{"one swap", graphOf([]string{"r", "a", "b", "x", "y"}, "r>a", "r>b", "a>y", "b>x"), 1, [][]string{{"r"}, {"a", "b"}, {"y", "x"}}},
		{"none crossing", graphOf([]string{"r", "a", "b", "x", "y"}, "r>a", "r>b", "a>x", "b>y"), 0, [][]string{{"r"}, {"a", "b"}, {"x", "y"}}},
		{
			"three reversed", graphOf([]string{"r", "a", "b", "c", "x", "y", "z"}, "r>a", "r>b", "r>c", "a>z", "b>y", "c>x"), 1,
			[][]string{{"r"}, {"a", "b", "c"}, {"z", "y", "x"}},
		},
	} {
		o, _ := readGraph(c.g, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // every name is a box's
		gr := o.groups[0]
		trips, _ := gr.place(false)
		var got [][]string
		for _, rank := range gr.ranks {
			var names []string
			for _, n := range rank {
				names = append(names, gr.nodes[n].box.Name)
			}
			got = append(got, names)
		}
		if trips != c.trips || !slices.EqualFunc(got, c.want, slices.Equal) {
			t.Errorf("%s: %d trips, ranks %v; want %d, %v", c.name, trips, got, c.trips, c.want)
		}
	}
}

// swinging is a graph whose boxes' lefts keep moving, pulled down and up in turn.
func swinging() *graph {
	g := graphOf([]string{"n0", "n1", "n2", "n3", "n4"},
		"n4>n1", "n2>n0", "n0>n1", "n3>n0", "n0>n2", "n4>n3", "n0>n3", "n0>n1", "n1>n4", "n1>n4")
	for i, b := range []struct {
		shape string
		n     int
	}{{"", 9}, {"", 11}, {"pill", 9}, {"hex", 16}, {"pill", 2}} {
		g.Nodes[i].Box.Shape, g.Nodes[i].Box.Rows[0].Text["en"] = b.shape, strings.Repeat("w", b.n)
	}
	return g
}

// pull stops at maxPulls on lefts that keep moving, and before it once nothing moves.
func TestGraphPulls(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		g     *graph
		pulls int
	}{
		{"swinging", swinging(), maxPulls},
		{"a diamond", graphOf([]string{"a", "b", "c", "d"}, "a>b", "a>c", "b>d", "c>d"), 3},
	} {
		o, _ := readGraph(c.g, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // every name is a box's
		if _, pulls := o.groups[0].place(false); pulls != c.pulls {
			t.Errorf("%s: %d pulls, want %d", c.name, pulls, c.pulls)
		}
	}
}

// channel is a random channel as the prototype drew them: 1 to 6 boxes above and below, 1 to 12 links between them,
// a second between two boxes among them, and half the time in one order above and below.
func channel(rnd *rand.Rand) (*group, bool) {
	gr := &group{
		nodes: nil, links: nil, ranks: [][]int{nil, nil}, chains: map[int][]int{}, reversed: map[int]bool{},
		tracks: nil, top: nil, bottom: nil, arcs: nil, out: nil, among: nil, cls: nil, frames: nil, botRoom: nil, headAt: 0,
	}
	up, low := 1+rnd.IntN(6), 1+rnd.IntN(6)
	for i := range up + low {
		r := 0
		if i >= up {
			r = 1
		}
		gr.ranks[r] = append(gr.ranks[r], i)
		gr.nodes = append(gr.nodes, gnode{
			box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: i, kind: kindPlain, cl: -1, rank: r, w: 0, h: 29,
			x: 0, y: 0, ins: nil, outs: nil, written: i,
		})
	}
	ordered := rnd.IntN(2) == 0
	pairs := make([][2]int, 1+rnd.IntN(12))
	for i := range pairs {
		pairs[i] = [2]int{rnd.IntN(up), rnd.IntN(low)}
	}
	if ordered {
		slices.SortStableFunc(pairs, func(a, b [2]int) int { return a[0] - b[0] })
		for i := 1; i < len(pairs); i++ {
			pairs[i][1] = max(pairs[i][1], pairs[i-1][1])
		}
	}
	for k, p := range pairs {
		gr.link(p[0], up+p[1], k)
	}
	for _, rank := range gr.ranks {
		x := 6 * float64(rnd.IntN(3))
		for _, i := range rank {
			n := &gr.nodes[i]
			n.w = float64(48 + rnd.IntN(200))
			if k := max(len(n.ins), len(n.outs)); k > 0 {
				n.w = max(n.w, onGrid(float64(12*k+36), 8, math.Ceil))
			}
			n.x = x
			x = onGrid(x+n.w+nodeGap+float64(rnd.IntN(40)), 6, math.Ceil)
		}
	}
	return gr, ordered
}

// In random channels, no two links share a run or touch, entries into a box stand a column apart, every link on a
// track below the links it must run under, no two links crossing more than their ports' order does, and none at
// all where the boxes keep one order above and below.
func TestWireChannel(t *testing.T) {
	t.Parallel()

	rnd := rand.New(rand.NewPCG(9, 10)) //nolint:gosec // G404: channels to try, no secret
	for run := range tries(200000) {
		gr, ordered := channel(rnd)
		gr.wire()
		paths := make([]path, len(gr.links))
		for k := range gr.links {
			pts := gr.points([]int{k}, false)
			for i := 1; i < len(pts); i++ {
				paths[k].segs = append(paths[k].segs, seg{pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]})
			}
		}
		crossed := 0
		for i := range gr.links {
			for j := i + 1; j < len(gr.links); j++ {
				a, b := gr.links[i], gr.links[j]
				bad, n := touch(paths[i], paths[j])
				if bad {
					t.Fatalf("run %d: links %d and %d touch: %+v %+v", run, i, j, a, b)
				}
				if a.to == b.to && math.Abs(a.entry-b.entry) < column {
					t.Fatalf("run %d: entries %v and %v less than a column apart", run, a.entry, b.entry)
				}
				order := 0
				if (a.exit-b.exit)*(a.entry-b.entry) < 0 {
					order = 1
				}
				if n > order {
					t.Fatalf("run %d: links %d and %d cross %d times, their ports' order %d", run, i, j, n, order)
				}
				crossed += n
			}
		}
		var bent []int
		for k, l := range gr.links {
			if !l.straight {
				bent = append(bent, k)
			}
		}
		for l, ups := range gr.above(bent) {
			for _, a := range ups {
				if gr.links[a].track >= gr.links[l].track {
					t.Fatalf("run %d: link %d on track %d, not above link %d on track %d", run, a, gr.links[a].track, l, gr.links[l].track)
				}
			}
		}
		if ordered && crossed > 0 {
			t.Fatalf("run %d: %d crossings in a channel of one order", run, crossed)
		}
	}
}

// Of two links whose runs overlap, the one whose exit is in the other's run goes above; two going opposite ways
// have no order; and saul's two cases: two links crossing the boxes' order share nothing and cross once, and two
// shifted right do not cross.
func TestWireCases(t *testing.T) {
	t.Parallel()

	links := func(pairs ...[2]float64) *group {
		gr := &group{
			nodes: []gnode{{box: nil, of: 0, kind: kindPlain, cl: -1, rank: 0, w: 0, h: 0, x: 0, y: 0, ins: nil, outs: nil, written: 0}},
			links: nil, ranks: [][]int{{0}}, chains: map[int][]int{}, reversed: map[int]bool{}, tracks: nil, top: nil, bottom: nil, arcs: nil, out: nil, among: nil, cls: nil, frames: nil, botRoom: nil, headAt: 0,
		}
		for k, p := range pairs {
			gr.links = append(gr.links, glink{from: 0, to: 0, edge: k, exit: p[0], entry: p[1], straight: false, track: 0})
			gr.nodes[0].outs = append(gr.nodes[0].outs, k)
		}
		return gr
	}
	shifted := links([2]float64{0, 30}, [2]float64{12, 42})
	shifted.track(0)
	if a, b := shifted.links[0].track, shifted.links[1].track; b >= a {
		t.Errorf("0→30 and 12→42: tracks %d and %d, want the second above", a, b)
	}
	if got := links([2]float64{0, 42}, [2]float64{36, 6}).above([]int{0, 1}); len(got) != 0 {
		t.Errorf("0→42 and 36→6 going opposite ways: above %v, want none", got)
	}

	// a→d and b→c, the boxes' order crossed once.
	gr := &group{
		nodes: nil, links: nil, ranks: [][]int{{0, 1}, {2, 3}}, chains: map[int][]int{}, reversed: map[int]bool{},
		tracks: nil, top: nil, bottom: nil, arcs: nil, out: nil, among: nil, cls: nil, frames: nil, botRoom: nil, headAt: 0,
	}
	for i, x := range []float64{12, 114, 12, 114} {
		gr.nodes = append(gr.nodes, gnode{
			box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: i, kind: kindPlain, cl: -1, rank: i / 2, w: 72, h: 29,
			x: x, y: 0, ins: nil, outs: nil, written: i,
		})
	}
	gr.link(0, 3, 0)
	gr.link(1, 2, 1)
	gr.wire()
	var ps [2]path
	for k := range 2 {
		pts := gr.points([]int{k}, false)
		for i := 1; i < len(pts); i++ {
			ps[k].segs = append(ps[k].segs, seg{pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1]})
		}
	}
	if bad, n := touch(ps[0], ps[1]); bad || n != 1 {
		t.Errorf("a→d and b→c: touching %v, crossing %d times; want apart, once", bad, n)
	}
}

// A gap between straight entries without the columns for its links bends the straight entry closing it, on its
// right (else its left), and no other.
func TestWireBend(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name     string
		exits    []float64
		straight []bool
	}{
		// 24's gap on the left has no column for two: 18 is 6 from it. 24 bends; 204 stays.
		{"left gap", []float64{-60, -48, 24, 204}, []bool{false, false, false, true}},
		// 216's gap on the right, to the box's end, has none for two: 216 bends, the one closing it on the left.
		{"right gap", []float64{24, 216, 300, 312}, []bool{true, false, false, false}},
		{"room enough", []float64{-60, 120, 204}, []bool{false, true, true}},
	} {
		gr := &group{
			nodes: []gnode{{
				box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: 0, kind: kindPlain, cl: -1, rank: 1, w: 240, h: 29, x: 0,
				y: 0, ins: nil, outs: nil, written: 0,
			}}, links: nil, ranks: nil, chains: map[int][]int{}, reversed: map[int]bool{},
			tracks: nil, top: nil, bottom: nil, arcs: nil, out: nil, among: nil, cls: nil, frames: nil, botRoom: nil, headAt: 0,
		}
		for k, x := range c.exits {
			gr.links = append(gr.links, glink{from: 0, to: 0, edge: k, exit: x, entry: 0, straight: false, track: 0})
			gr.nodes[0].ins = append(gr.nodes[0].ins, k)
		}
		gr.entries(0)
		got := make([]bool, len(gr.links))
		for k, l := range gr.links {
			got[k] = l.straight
		}
		if !slices.Equal(got, c.straight) {
			t.Errorf("%s: straight %v, want %v", c.name, got, c.straight)
		}
	}
}

// chainGraph is a chain of n boxes with long edges from its first box to its last, which pass n-2 ranks each.
func chainGraph(n, long int) *graph {
	names := make([]string, n)
	var edges []string
	for i := range n {
		names[i] = fmt.Sprintf("c%d", i)
		if i > 0 {
			edges = append(edges, names[i-1]+">"+names[i])
		}
	}
	for range long {
		edges = append(edges, names[0]+">"+names[n-1])
	}
	return graphOf(names, edges...)
}

// A graph placed far out lays out and places its labels, room made among them, as it does at 20,20, moved by the
// difference: it is laid out from 0,0 and moved to its place last, so that room made and spots tried step by units
// on small coordinates.
func TestGraphFarOut(t *testing.T) {
	t.Parallel()
	moved := func(it item, dx, dy float64) string {
		if b := it.p.Box; b != nil {
			return fmt.Sprint(b.X+dx, b.Y+dy, b.W, b.H)
		}
		e := it.p.Edge
		pts := [][2]float64{*e.From.Point}
		for _, v := range e.Via {
			pts = append(pts, [2]float64{*v.X, *v.Y})
		}
		pts = append(pts, *e.To.Point)
		out := ""
		for _, p := range pts {
			out += fmt.Sprint(p[0]+dx, p[1]+dy, " ")
		}
		if e.At != nil {
			out += fmt.Sprint("at ", e.At[0]+dx, e.At[1]+dy)
		}
		return out
	}
	for _, direction := range []string{"", "right"} {
		g := graphOf([]string{"a", "b"}, "a>b", "a>b", "a>b", "b>b")
		g.Direction = direction
		for k := range g.Edges {
			g.Edges[k].Label = Text{"en": "wwwwww"}
		}
		near, _ := readGraph(g, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // a graph with no mistake
		want, _, err := near.items()                                      //nolint:errcheck // the mistakes are told by laid
		if err != nil {
			t.Fatal(err)
		}
		g.X, g.Y = 999000, 998000
		far, _ := readGraph(g, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // a graph with no mistake
		got, _, err := far.items()                                       //nolint:errcheck // the mistakes are told by laid
		if err != nil {
			t.Fatal(err)
		}
		for i := range want {
			if a, b := moved(got[i], 0, 0), moved(want[i], 999000-20, 998000-20); a != b {
				t.Errorf("direction %q, %s: at %s far out, %s moved from 20,20", direction, want[i].at, a, b)
			}
		}
	}
}

// The work of laying out is held: the points edges pass ranks at are counted, order and pull stop at their caps,
// and a diagram whose graphs pass more than maxPassed ranks, all told, is refused at its root, with no graph laid
// out.
func TestGraphLimit(t *testing.T) {
	t.Parallel()

	rnd := rand.New(rand.NewPCG(5, 6)) //nolint:gosec // G404: a graph to try, no secret
	big := graphOf(nil)
	for i := range 200 {
		big.Nodes = append(big.Nodes, graphOf([]string{fmt.Sprintf("b%d", i)}).Nodes...)
	}
	// Each edge between boxes a few apart as written, either way: between any two, as dagro's trials drew them,
	// the edges pass 6,911 ranks, over maxPassed.
	for range 400 {
		a := rnd.IntN(200)
		b := min(199, a+1+rnd.IntN(6))
		if a == b {
			b = a - 1
		}
		if rnd.IntN(2) == 0 {
			a, b = b, a
		}
		big.Edges = append(big.Edges, graphOf(nil, fmt.Sprintf("b%d>b%d", a, b)).Edges...)
	}
	exact := chainGraph(42, 100) // 100 edges of 40 points
	// Two groups of 99 boxes, 400 edges from one to the other; and 10 groups nested, 190 boxes, big's edges.
	two := graphOf(nil)
	for _, name := range []string{"a", "b"} {
		names := make([]string, 0, 99)
		for i := range 99 {
			names = append(names, fmt.Sprintf("%s%d", name, i))
		}
		two.Nodes = append(two.Nodes, inGroup(graphOf(names), name, name, names...).Nodes...)
	}
	for range 400 {
		two.Edges = append(two.Edges, graphOf(nil, "a>b").Edges...)
	}
	deep := graphOf(nil)
	for d := range 10 {
		var names []string
		for i := range 19 {
			names = append(names, fmt.Sprintf("b%d", 19*d+i))
		}
		deep.Nodes = append(deep.Nodes, graphOf(names).Nodes...)
		if d > 0 {
			names = append(names, fmt.Sprintf("g%d", d-1))
		}
		inGroup(deep, fmt.Sprintf("g%d", d), "", names...)
	}
	for _, e := range big.Edges {
		a, _ := strconv.Atoi(e.From[1:]) //nolint:errcheck // b and a number, as made
		b, _ := strconv.Atoi(e.To[1:])   //nolint:errcheck // as above
		if a < 190 && b < 190 {
			deep.Edges = append(deep.Edges, e)
		}
	}
	// 100 lanes of a box each, a chain through them 100 ranks long: 20,000 marks.
	lanes := graphOf(nil)
	for i := range 100 {
		name := fmt.Sprintf("b%d", i)
		lanes.Nodes = append(lanes.Nodes, inGroup(graphOf([]string{name}), fmt.Sprintf("l%d", i), "", name).Nodes...)
		if i > 0 {
			lanes.Edges = append(lanes.Edges, graphOf(nil, fmt.Sprintf("b%d>%s", i-1, name)).Edges...)
		}
	}
	inLaned(lanes)
	for _, g := range []*graph{big, exact, two, deep, lanes} {
		o, _ := readGraph(g, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // every name is a box's
		if o.passed > maxPassed {
			t.Fatalf("%d boxes: %d points, over %d", len(g.Nodes), o.passed, maxPassed)
		}
		for _, gr := range o.groups {
			if trips, pulls := gr.place(false); trips > maxTrips || pulls > maxPulls {
				t.Errorf("%d boxes: %d trips and %d pulls, over %d and %d", len(g.Nodes), trips, pulls, maxTrips, maxPulls)
			}
		}
		if l := laid(g); l.failure != nil || len(l.problems) > 0 {
			t.Errorf("%d boxes: %v %v", len(g.Nodes), l.failure, l.problems)
		}
	}
	if o, _ := readGraph(exact, "#", (*drawing).settleEnds); o.passed != maxPassed { //nolint:errcheck // every name is a box's
		t.Errorf("the chain passes %d ranks, want %d", o.passed, maxPassed)
	}

	// Edges to a group refused, each named by the boxes it stands among, found by a search from each end, not from
	// each pair: groups g and h of 99 boxes each, g a chain between h's first and last, and 300 edges g→h.
	among := graphOf(nil)
	for _, name := range []string{"g", "h"} {
		names := make([]string, 0, 99)
		for i := range 99 {
			names = append(names, fmt.Sprintf("%s%d", name, i))
		}
		among.Nodes = append(among.Nodes, inGroup(graphOf(names), name, "", names...).Nodes...)
	}
	for i := range 98 {
		among.Edges = append(among.Edges, graphOf(nil, fmt.Sprintf("g%d>g%d", i, i+1)).Edges...)
	}
	among.Edges = append(among.Edges, graphOf(nil, "h0>g0", "g98>h98").Edges...)
	for range 300 {
		among.Edges = append(among.Edges, graphOf(nil, "g>h").Edges...)
	}
	start := time.Now()
	if _, errs := readGraph(among, "#", (*drawing).settleEnds); len(errs) != 300 || time.Since(start) > 3*time.Second {
		t.Errorf("300 edges among: %d mistakes in %v, want 300 within 3s", len(errs), time.Since(start))
	}

	// Counted before any point is made: graphs whose edges pass far more ranks than the limit make none, and are
	// refused all told.
	huge := chainGraph(200, 200)                                      // 200 edges of 198 points
	o, _ := readGraph(huge, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // every name is a box's
	if gr := o.groups[0]; o.passed != 200*198 || len(gr.nodes) != 200 || len(gr.links) != 0 {
		t.Errorf("200 boxes: %d ranks passed, %d nodes, %d links; want %d, 200, none made", o.passed, len(gr.nodes), len(gr.links), 200*198)
	}
	hundred := make([]part, 100)
	for i := range hundred {
		hundred[i] = part{Graph: chainGraph(200, 200)} //nolint:exhaustruct_v5 // one kind of part
	}
	if items, found, err := expand(hundred, (*drawing).settleEnds); err != nil || len(items) != 0 || len(found) != 1 || fmt.Sprint(found[0].Args) != "[3960000 4000]" {
		t.Errorf("a hundred graphs: %d items, %v; want none, one diagram_graph_long", len(items), found)
	}

	over := chainGraph(42, 100)
	over.Edges = append(over.Edges, graphOf(nil, "c39>c41").Edges...)
	split := graphOf([]string{"a", "b", "c"}, "a>b", "b>c", "a>c")
	split.X = 2000
	for _, parts := range [][]part{{{Graph: over}}, {{Graph: exact}, {Graph: split}}} { //nolint:exhaustruct_v5 // one kind of part
		items, found, err := expand(parts, (*drawing).settleEnds)
		if err != nil || len(items) != 0 || len(found) != 1 || found[0].Key != "diagram_graph_long" || found[0].At() != "#" ||
			fmt.Sprint(found[0].Args) != "[4001 4000]" {
			t.Errorf("%d graphs: %d items, %v; want none, diagram_graph_long at # of 4001", len(parts), len(items), found)
		}
	}
}

// The pairs of edges checkCrossings compares leave out two of one graph, and no other.
func TestCrossPairs(t *testing.T) {
	t.Parallel()

	bipartite := func(up, low int, x float64) *graph {
		names, edges := make([]string, 0, up+low), make([]string, 0, up*low)
		for i := range up {
			names = append(names, fmt.Sprintf("u%v-%d", x, i))
		}
		for j := range low {
			names = append(names, fmt.Sprintf("l%v-%d", x, j))
		}
		for i := range up {
			for j := range low {
				edges = append(edges, fmt.Sprintf("u%v-%d>l%v-%d", x, i, x, j))
			}
		}
		g := graphOf(names, edges...)
		g.X = x
		return g
	}
	hand := part{Edge: &edge{From: end{Name: "", Point: &[2]float64{0, 0}}, To: end{Name: "", Point: &[2]float64{0, 10}}}} //nolint:exhaustruct_v5 // the rest as left out
	for _, c := range []struct {
		parts []part
		want  int
	}{
		{[]part{{Graph: bipartite(20, 20, 0)}, hand}, 400},                                           //nolint:exhaustruct_v5 // one kind of part
		{[]part{{Graph: bipartite(10, 20, 0)}, {Graph: bipartite(10, 20, 5000)}, hand}, 40000 + 400}, //nolint:exhaustruct_v5 // one kind of part
	} {
		l := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: c.parts, Example: ""},
			Options{Lang: "en", Icons: NoIcons{Err: nil}, Seq: 1})
		n := 0
		for range l.crossPairs() {
			n++
		}
		if n != c.want {
			t.Errorf("%d parts: %d pairs, want %d", len(c.parts), n, c.want)
		}
	}
}

// A mistake that only things graphs laid out make is told at the first's graph, to move it: two graphs' boxes
// overlapping, an edge of one through a box of another, crossing an edge of another, and a box off the canvas. With
// a thing written by hand, it is told at that.
func TestGraphTold(t *testing.T) {
	t.Parallel()

	const ga, gb = "#/parts/0/graph", "#/parts/1/graph"
	box := func(x float64, graph string) rect {
		return rect{x: x, y: 0, w: 40, h: 40, what: "box at " + num(x), at: graph + "/nodes/0/box", form: outline{hex: 0, radius: 0}, graph: graph}
	}
	frame := func(x float64, graph string) rect {
		return rect{x: x, y: 0, w: 100, h: 100, what: "frame at " + num(x), at: graph + "/nodes/1/group", graph: graph} //nolint:exhaustruct_v5 // a frame
	}
	across := path{segs: []seg{{0, 20, 100, 20}}, what: "edge a→b", at: ga + "/edges/0", own: -1, crossing: false, graph: ga}
	down := path{segs: []seg{{50, -10, 50, 100}}, what: "edge c→d", at: gb + "/edges/0", own: -1, crossing: false, graph: gb}
	hand := down
	hand.at, hand.graph, hand.what = "#/parts/1", "", "edge [50,-10]→[50,100]"
	handBox := box(20, "")
	handBox.at = "#/parts/1"
	for _, c := range []struct {
		name    string
		l       *layout
		key, at string
	}{
		{"two graphs' boxes", &layout{boxes: []rect{box(0, ga), box(20, gb)}, w: 500, h: 500}, "diagram_graph_overlap", ga},                                      //nolint:exhaustruct_v5 // what it checks
		{"a graph's box and one by hand", &layout{boxes: []rect{box(0, ga), handBox}, w: 500, h: 500}, "diagram_overlap", "#/parts/1"},                           //nolint:exhaustruct_v5 // what it checks
		{"an edge through another graph's box", &layout{boxes: []rect{box(30, gb)}, paths: []path{across}, w: 500, h: 500}, "diagram_graph_through", ga},         //nolint:exhaustruct_v5 // what it checks
		{"two graphs' edges", &layout{paths: []path{across, down}, w: 500, h: 500}, "diagram_graph_crossing", ga},                                                //nolint:exhaustruct_v5 // what it checks
		{"the other first", &layout{paths: []path{down, across}, w: 500, h: 500}, "diagram_graph_crossing", gb},                                                  //nolint:exhaustruct_v5 // what it checks
		{"a graph's edge and one by hand", &layout{paths: []path{across, hand}, w: 500, h: 500}, "diagram_crossing", "#/parts/1"},                                //nolint:exhaustruct_v5 // what it checks
		{"a graph's box off the canvas", &layout{boxes: []rect{box(480, ga)}, w: 500, h: 500}, "diagram_graph_outside", ga},                                      //nolint:exhaustruct_v5 // what it checks
		{"a graph's frame and a box by hand", &layout{boxes: []rect{handBox}, frames: []rect{frame(40, ga)}, w: 500, h: 500}, "diagram_straddle", "#/parts/1"},   //nolint:exhaustruct_v5 // what it checks
		{"a graph's frame and another's box", &layout{boxes: []rect{box(120, gb)}, frames: []rect{frame(40, ga)}, w: 500, h: 500}, "diagram_graph_straddle", gb}, //nolint:exhaustruct_v5 // what it checks
	} {
		c.l.check()
		if len(c.l.problems) != 1 || c.l.problems[0].Key != c.key || c.l.problems[0].At() != c.at {
			t.Errorf("%s: %v, want %s at %s", c.name, c.l.problems, c.key, c.at)
		}
	}
	// Things of one graph meet only by a fault of its layout: floe's failure, not a mistake to move a graph, which
	// checked returns as Drawer reads it.
	for _, l := range []*layout{
		{boxes: []rect{box(0, ga), box(20, ga)}, w: 500, h: 500},                   //nolint:exhaustruct_v5 // what it checks
		{boxes: []rect{box(0, ga)}, frames: []rect{frame(20, ga)}, w: 500, h: 500}, //nolint:exhaustruct_v5 // what it checks
		{boxes: []rect{box(30, ga)}, paths: []path{across}, w: 500, h: 500},        //nolint:exhaustruct_v5 // what it checks
		{labels: []rect{{x: 40, y: 10, w: 30, h: 18, what: "label", at: ga + "/edges/0", graph: ga}}, //nolint:exhaustruct_v5 // what it checks
			paths: []path{across}, w: 500, h: 500},
	} {
		if err := l.checked(); err == nil || len(l.problems) != 0 {
			t.Errorf("one graph's things meeting: failure %v, mistakes %v", err, l.problems)
		}
	}
}

// A point pushed off its column stands at the free column nearest to where it may, freeLimit from it at most: not
// under the side of a box above, nor where another link of the channel leaves; the left of two as near.
func TestGraphFreeColumn(t *testing.T) {
	t.Parallel()
	gr := &group{ //nolint:exhaustruct_v5 // what freeColumn reads
		nodes: []gnode{
			{box: &graphBox{Name: "a", Rows: nil, Tone: "", Shape: ""}, of: 0, kind: kindPlain, cl: -1, rank: 0, w: 100, h: 29, x: 0, y: 0, ins: nil, outs: nil, written: 0},
			{box: nil, of: 0, kind: kindPlain, cl: -1, rank: 0, w: 0, h: 0, x: 144, y: 0, ins: nil, outs: []int{0}, written: 1},
		},
		links: []glink{{from: 1, to: 2, edge: 0, exit: 144, entry: 0, straight: false, track: 0}},
		ranks: [][]int{{0, 1}, {2}},
	}
	for _, c := range []struct {
		t, lo, hi, want float64
		ok              bool
	}{
		{96, 96, 300, 96, true},  // free where it may first stand
		{60, 0, 300, 96, true},   // under a's side (12 to 88) as far as 96, right; left of 12 is too far
		{144, 0, 300, 132, true}, // the point's exit taken: the left of 132 and 156
		{105, 100, 300, 108, true},
		{150, 145, 155, 0, false}, // no column within lo and hi
	} {
		got, ok := gr.freeColumn(1, 99, c.t, c.lo, c.hi)
		if got != c.want || ok != c.ok {
			t.Errorf("t %v in %v..%v: %v %v, want %v %v", c.t, c.lo, c.hi, got, ok, c.want, c.ok)
		}
	}
	wide := &group{ //nolint:exhaustruct_v5 // what freeColumn reads
		nodes: []gnode{{box: &graphBox{Name: "a", Rows: nil, Tone: "", Shape: ""}, of: 0, kind: kindPlain, cl: -1, rank: 0, w: 300, h: 29, x: 0, y: 0, ins: nil, outs: nil, written: 0}},
		ranks: [][]int{{0}, {}},
	}
	if got, ok := wide.freeColumn(1, -1, 150, 0, 400); ok { // the nearest free columns, 0 and 300, past freeLimit
		t.Errorf("under a wide box: %v, want none within %v", got, freeLimit)
	}
	// freeLimit from t, not from the column nearest it: pushed to lo 54, a box over 60 to 96 leaves 108, 54 from t.
	over := &group{ //nolint:exhaustruct_v5 // what freeColumn reads
		nodes: []gnode{{box: &graphBox{Name: "a", Rows: nil, Tone: "", Shape: ""}, of: 0, kind: kindPlain, cl: -1, rank: 0, w: 60, h: 29, x: 48, y: 0, ins: nil, outs: nil, written: 0}},
		ranks: [][]int{{0}, {}},
	}
	if got, ok := over.freeColumn(1, -1, 54, 54, 300); ok {
		t.Errorf("pushed to 54 under a box over 60 to 96: %v, want none within %v of 54", got, freeLimit)
	}
}

// A box widens, into room only, by as little as lets a link whose exit stands a column or less past an end of its
// side enter straight.
func TestGraphWiden(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name         string
		exit         float64
		beside       float64 // the left of a box right of it, or 0 for none
		extent       float64 // the group's right
		wantX, wantW float64
	}{
		{"a column past its right", 156, 0, 500, 100, 68},
		{"two columns past", 168, 0, 500, 100, 64},
		{"a column past its left", 100, 0, 500, 88, 76},
		{"no room before the box beside it", 156, 180, 500, 100, 64},
		{"room before the box beside it", 156, 192, 500, 100, 68},
		{"at the end, within the group", 156, 0, 170, 100, 68},
		{"at the end, past the group", 156, 0, 160, 100, 64},
	} {
		box := func(x float64, ins []int) gnode {
			return gnode{box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: 0, kind: kindPlain, cl: -1, rank: 1, w: 64, h: 29, x: x, y: 0, ins: ins, outs: nil, written: 0}
		}
		gr := &group{ //nolint:exhaustruct_v5 // what widen reads
			nodes: []gnode{box(100, []int{0})},
			links: []glink{{from: 9, to: 0, edge: 0, exit: c.exit, entry: 0, straight: false, track: 0}},
		}
		rank := []int{0}
		if c.beside > 0 {
			gr.nodes = append(gr.nodes, box(c.beside, nil))
			rank = append(rank, 1)
		}
		gr.widen(rank, 0, 0, c.extent)
		if n := gr.nodes[0]; n.x != c.wantX || n.w != c.wantW {
			t.Errorf("%s: x %v w %v, want %v %v", c.name, n.x, n.w, c.wantX, c.wantW)
		}
	}
	// Widened left for the link from 216, the box keeps no column left of it for the link from 168, which
	// entries lays first: 216 would enter bent all the same, so the box stays as it is.
	crowded := &group{ //nolint:exhaustruct_v5 // what widen reads
		nodes: []gnode{{box: &graphBox{Name: "", Rows: nil, Tone: "", Shape: ""}, of: 0, kind: kindPlain, cl: -1, rank: 1, w: 128, h: 29, x: 210, y: 0, ins: []int{0, 1, 2}, outs: nil, written: 0}},
		links: []glink{
			{from: 9, to: 0, edge: 0, exit: 168, entry: 0, straight: false, track: 0},
			{from: 9, to: 0, edge: 1, exit: 216, entry: 0, straight: false, track: 0},
			{from: 9, to: 0, edge: 2, exit: 288, entry: 0, straight: false, track: 0},
		},
	}
	crowded.widen([]int{0}, 0, 0, 500)
	if n := crowded.nodes[0]; n.x != 210 || n.w != 128 {
		t.Errorf("widened where no more links enter straight: x %v w %v, want 210 128", n.x, n.w)
	}
}

// pathTurns is how many times the paths of l turn, all told.
func pathTurns(l *layout) int {
	n := 0
	for _, p := range l.paths {
		n += len(p.segs) - 1
	}
	return n
}

// boxIn is the area of the box named name in l.
func boxIn(l *layout, name string) rect {
	return l.names[name].r
}

// A graph's shapes: a chain runs straight, an edge back up a cycle points the way it is written, groups stand side
// by side as written, the languages lay out alike, a box widens for its edges, boxes of a rank stand nodeGap apart,
// a leaf added keeps the other ranks' order, and right is down turned.
func TestGraphShapes(t *testing.T) {
	t.Parallel()

	chain := laid(graphOf([]string{"a", "b", "c", "d"}, "a>b", "b>c", "c>d"))
	for _, p := range chain.paths {
		if len(p.segs) != 1 {
			t.Errorf("chain: %s turns: %v", p.what, p.segs)
		}
	}

	// Edges that can run straight do: every edge of the kept flow, and K3,3's edges straight down; of the two edges
	// of fan a reader once saw bend, c→hub runs straight and x→a turns twice at most; in the state example, the
	// edges of running, its box widened for them, and the long edge past it.
	for _, c := range []struct {
		file  string
		turns map[string]int // the most turns of the edges named; of every edge, none, when nil
	}{
		{"testdata/graph/flow.yaml", nil},
		{"testdata/graph/fan.yaml", map[string]int{"edge c→hub": 0, "edge x→a": 2}},
		{"examples/state.yaml", map[string]int{
			"edge stopped→running": 0, "edge running→outdated": 0, "edge outdated→running": 0, "edge running→stopped": 0,
			"edge outdated→stopped": 0,
		}},
	} {
		src, err := os.ReadFile(filepath.FromSlash(c.file))
		if err != nil {
			t.Fatal(err)
		}
		_, v, err := yamlpos.ParseMap(c.file, src)
		if err != nil {
			t.Fatal(err)
		}
		s, err := read(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range laid(s.Parts[0].Graph).paths {
			most, ok := c.turns[p.what]
			if c.turns != nil && !ok {
				continue
			}
			if len(p.segs)-1 > most {
				t.Errorf("%s: %s turns %d times, want %d at most: %v", c.file, p.what, len(p.segs)-1, most, p.segs)
			}
		}
	}
	// A point pushed off its column stands at a free column of its own, turning once in the channel above: ten
	// turns, not the twelve of entering it at x ≡ 6 and leaving it half a column on.
	pushed := graphOf([]string{"n0", "n1", "n2", "n3"}, "n1>n3", "n0>n2", "n1>n2", "n3>n0", "n2>n3", "n3>n2")
	for i, b := range []struct {
		shape string
		n     int
	}{{"pill", 8}, {"state", 5}, {"", 13}, {"hex", 3}} {
		pushed.Nodes[i].Box.Shape, pushed.Nodes[i].Box.Rows[0].Text["en"] = b.shape, strings.Repeat("w", b.n)
	}
	pushed.Edges[4].Label = Text{"en": strings.Repeat("w", 9)}
	if turns := pathTurns(laid(pushed)); turns != 10 {
		t.Errorf("a point pushed off its column: %d turns, want 10", turns)
	}
	// A box settle moves off its exit's column takes its exit again: six turns, not eight.
	moved := graphOf([]string{"n0", "n1", "n2"}, "n0>n1", "n2>n1", "n2>n0", "n1>n0", "n1>n0", "n2>n1")
	for i, b := range []struct {
		shape string
		n     int
	}{{"pill", 9}, {"state", 10}, {"state", 15}} {
		moved.Nodes[i].Box.Shape, moved.Nodes[i].Box.Rows[0].Text["en"] = b.shape, strings.Repeat("w", b.n)
	}
	movedTurns := 0
	for _, p := range laid(moved).paths {
		movedTurns += len(p.segs) - 1
	}
	if movedTurns > 6 {
		t.Errorf("a box moved by settle: %d turns, want 6 at most", movedTurns)
	}
	k33 := laid(graphOf([]string{"a", "b", "c", "x", "y", "z"},
		"a>x", "a>y", "a>z", "b>x", "b>y", "b>z", "c>x", "c>y", "c>z"))
	for _, k := range []int{0, 4, 8} {
		if p := k33.paths[k]; len(p.segs) != 1 {
			t.Errorf("K3,3: %s turns: %v", p.what, p.segs)
		}
	}

	cycle := laid(graphOf([]string{"a", "b", "c"}, "a>b", "b>c", "c>a"))
	back := cycle.paths[2].segs
	if a := boxIn(cycle, "a"); back[len(back)-1][3] != a.y+a.h {
		t.Errorf("cycle: c→a ends at %v, want at the bottom of a, %v", back[len(back)-1], a.y+a.h)
	}

	apart := laid(graphOf([]string{"a", "b", "c", "d"}, "a>b", "c>d"))
	if a, c := boxIn(apart, "a"), boxIn(apart, "c"); c.x < a.x+a.w+groupGap {
		t.Errorf("two groups: c at %v, not right of a (%v to %v)", c.x, a.x, a.x+a.w)
	}

	langs := graphOf([]string{"a", "b"}, "a>b")
	langs.Nodes[1].Box.Rows[0].Text = Text{"en": "Save", "ja": "データファイルと画像の写しを保存する"}
	en := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: []part{{Graph: langs}}, Example: ""}, //nolint:exhaustruct_v5 // one kind of part
		Options{Lang: "en", Icons: NoIcons{Err: nil}, Seq: 1})
	ja := newLayout(&spec{Width: 0, Height: 0, Title: Text{"en": "g"}, Parts: []part{{Graph: langs}}, Example: ""}, //nolint:exhaustruct_v5 // one kind of part
		Options{Lang: "ja", Icons: NoIcons{Err: nil}, Seq: 1})
	if en.inspect() != ja.inspect() {
		t.Errorf("languages: en\n%s\nja\n%s", en.inspect(), ja.inspect())
	}

	// Eleven edges at one box: five in, three out, and three back up a cycle, turned to leave it too.
	hub := laid(graphOf([]string{"a", "b", "c", "d", "e", "hub", "x", "y", "z"},
		"a>hub", "b>hub", "c>hub", "d>hub", "e>hub", "hub>x", "hub>y", "hub>z", "x>hub", "y>hub", "z>hub"))
	if len(hub.problems) > 0 {
		t.Errorf("eleven edges: %v", hub.problems)
	}
	if h := boxIn(hub, "hub"); h.w < 12*6+36 {
		t.Errorf("eleven edges: the box %v wide, want %v or more for six edges out", h.w, 12*6+36)
	}

	beside := laid(graphOf([]string{"r", "a", "b", "c"}, "r>a", "r>b", "r>c"))
	for _, pair := range [][2]string{{"a", "b"}, {"b", "c"}} {
		if l, r := boxIn(beside, pair[0]), boxIn(beside, pair[1]); r.x-(l.x+l.w) < nodeGap {
			t.Errorf("beside: %s and %s %v apart, want %v or more", pair[0], pair[1], r.x-(l.x+l.w), nodeGap)
		}
	}

	// A leaf added keeps the order of the other ranks.
	order := func(g *graph) []string {
		var out []string
		groups, err := grouped(g)
		if err != nil {
			t.Fatal(err)
		}
		for _, gr := range groups {
			for _, rank := range gr.ranks[:2] {
				names := []string{}
				for _, n := range rank {
					if b := gr.nodes[n].box; b != nil {
						names = append(names, b.Name)
					}
				}
				slices.SortFunc(names, func(a, b string) int {
					return int(gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == a })].x -
						gr.nodes[slices.IndexFunc(gr.nodes, func(n gnode) bool { return n.box != nil && n.box.Name == b })].x)
				})
				out = append(out, strings.Join(names, " "))
			}
		}
		return out
	}
	tree := graphOf([]string{"r", "a", "b", "c", "x", "y"}, "r>a", "r>b", "r>c", "a>x", "c>y")
	more := graphOf([]string{"r", "a", "b", "c", "x", "y", "w"}, "r>a", "r>b", "r>c", "a>x", "c>y", "b>w")
	if a, b := order(tree), order(more); !slices.Equal(a, b) {
		t.Errorf("a leaf added: ranks %v, then %v", a, b)
	}

	// Right runs the edges left to right, as down runs them top down.
	g := graphOf([]string{"a", "b", "c"}, "a>b", "a>c", "b>c")
	down := laid(g)
	turned := *g
	turned.Direction = "right"
	r := laid(&turned)
	for _, e := range g.Edges {
		if f, to := boxIn(down, e.From), boxIn(down, e.To); to.y < f.y+f.h {
			t.Errorf("down: %s→%s, %v not below %v", e.From, e.To, to, f)
		}
		if f, to := boxIn(r, e.From), boxIn(r, e.To); to.x < f.x+f.w {
			t.Errorf("right: %s→%s, %v not right of %v", e.From, e.To, to, f)
		}
	}
	if len(r.problems) > 0 {
		t.Errorf("right: %v", r.problems)
	}
}

// laidSpec is the diagram of src read and laid out in lang, and checked.
func laidSpec(t *testing.T, src string, lang i18n.Lang) *layout {
	t.Helper()
	_, v, err := yamlpos.ParseMap("s.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	s, err := read(v)
	if err != nil {
		t.Fatal(err)
	}
	l := newLayout(s, Options{Lang: lang, Icons: NoIcons{Err: nil}, Seq: 1})
	l.check()
	if l.failure != nil || len(l.problems) > 0 {
		t.Fatalf("%v %v", l.failure, l.problems)
	}
	return l
}

// labelled is g with label text on the edges ks.
func labelled(g *graph, text string, ks ...int) *graph {
	for _, k := range ks {
		g.Edges[k].Label = Text{"en": text}
	}
	return g
}

// A graph's labels stand on their edges, at the middle of the longest run where it is clear, on a background as one by
// hand. With no room on a run, room is made, its edges as straight as with no label. A label in another language takes
// the room of the widest. A label left of the boxes moves the graph right, its left at x, and a label of one group
// stands clear of the next.
func TestGraphLabels(t *testing.T) {
	t.Parallel()

	g2 := func(dir string) *graph {
		g := labelled(graphOf([]string{"write", "check", "show"}, "write>check", "check>show", "check>write"), "yes", 1)
		g.Edges[2].Label, g.Direction = Text{"en": "no"}, dir
		return g
	}
	for _, dir := range []string{"", "right"} {
		l := laid(g2(dir))
		for j, k := range []int{1, 2} {
			r, s := l.labels[j], longestSeg(l.paths[k].segs)
			along := runAxis(s)
			if !onRun(r, s) || r.lo(along)+r.size(along)/2 != (s[along]+s[along+2])/2 {
				t.Errorf("%q: label of %s at %v,%v, its longest run %v", dir, l.paths[k].what, r.x, r.y, s)
			}
		}
	}

	// Twenty edges side by side, the first labelled: room is made about it, and the graph moves right as far as the
	// label stands left of x.
	par := labelled(graphOf([]string{"a", "b"}, slices.Repeat([]string{"a>b"}, 20)...), "L0", 0)
	o, _ := readGraph(par, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // every name is a box's
	items, _, err := o.items()                                       //nolint:errcheck // the mistakes are told by laid
	if err != nil {
		t.Fatal(err)
	}
	e := items[2].p.Edge
	run := e.From.Point[0]
	if left := e.At[0] - labelWidth(e.Label, false)/2; e.beside || e.At[0] != run || left < par.X || left >= par.X+column {
		t.Errorf("parallel: label at %v, run at %v, x %v", e.At, run, par.X)
	}

	// Three edges between two boxes, the middle one labelled: no room between its neighbours, so room is made, and
	// every edge runs straight.
	three := labelled(graphOf([]string{"a", "b"}, "a>b", "a>b", "a>b"), "label", 1)
	l := laid(three)
	for _, p := range l.paths {
		if len(p.segs) != 1 {
			t.Errorf("three: %s turns: %v", p.what, p.segs)
		}
	}
	if !onRun(l.labels[0], l.paths[1].segs[0]) {
		t.Errorf("three: label at %v,%v, not on %v", l.labels[0].x, l.labels[0].y, l.paths[1].segs[0])
	}

	// Two groups: the first's label reaching right, the second past it.
	two := labelled(graphOf([]string{"a", "b", "c", "d"}, "a>b", "c>d"), strings.Repeat("w", 30), 0)
	l = laid(two)
	if c := boxIn(l, "c"); c.x <= l.labels[0].x+l.labels[0].w {
		t.Errorf("two groups: c at %v, the label of a→b reaching %v", c.x, l.labels[0].x+l.labels[0].w)
	}

	const src = `title: t
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - box: { name: write, rows: [Write the data] }
        - box: { name: check, shape: hex, rows: ["Checks pass?"] }
        - box: { name: show, rows: [Show the page] }
      edges:
        - { from: write, to: check }
        - { from: check, to: show, label: { en: "yes", ja: データファイルと画像の写しを保存する } }
        - { from: check, to: write, label: "no" }
  - edge: { from: [400, 20], to: [400, 200], label: by hand }
`
	en, ja := laidSpec(t, src, "en"), laidSpec(t, src, "ja")
	if fmt.Sprint(en.boxes, en.paths, en.labels) != fmt.Sprint(ja.boxes, ja.paths, ja.labels) {
		t.Errorf("en and ja lay out apart:\n%v\n%v", en.inspect(), ja.inspect())
	}
	bg := 0
	for _, el := range en.labelEls {
		switch {
		case el.Class == "label-bg":
			bg++
		case el.Kind == "text" && el.Text["en"] == "yes":
			if x := en.paths[1].segs[0][0]; el.Anchor != "middle" || el.X != x {
				t.Errorf("yes drawn at %v (%s), its run at %v", el.X, el.Anchor, x)
			}
		}
	}
	if bg != 3 {
		t.Errorf("%d labels on a background, want the graph's two and the one by hand", bg)
	}

	// A loop's label stands beside its line, on no background.
	loop := laid(labelled(graphOf([]string{"a"}, "a>a"), "again", 0))
	if slices.ContainsFunc(loop.labelEls, func(el elem) bool { return el.Class == "label-bg" }) {
		t.Errorf("a loop's label on a background: %v", loop.inspect())
	}
}

// longestSeg is the longest of segs, the first of those of one length.
func longestSeg(segs []seg) seg {
	best := segs[0]
	for _, s := range segs {
		if segLength(s) > segLength(best) {
			best = s
		}
	}
	return best
}

// A label stands on the longest run of its edge; with something at its middle it moves along the run, and with room
// made for it, it stands at the middle of the run as lengthened, not by the cut.
func TestGraphLabelOnLongest(t *testing.T) {
	t.Parallel()

	d := &drawing{
		right: false, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{
			0: {{0, 0}, {0, 60}, {200, 60}, {200, 80}},
		}, ends: map[int][2]int{0: {0, 0}}, back: map[int]bool{}, spots: map[int]spot{},
	}
	if err := d.place(0, 40); err != nil {
		t.Fatal(err)
	}
	if sp := d.spots[0]; sp.seg != 1 || sp.at != 80 {
		t.Errorf("on run %d from %v, want run 1 from 80", sp.seg, sp.at)
	}
	d.routes[1] = [][2]float64{{100, 0}, {100, 100}}
	delete(d.spots, 0)
	if err := d.place(0, 40); err != nil {
		t.Fatal(err)
	}
	if sp := d.spots[0]; sp.seg != 1 || sp.at == 80 {
		t.Errorf("with a run across its middle: on run %d from %v", sp.seg, sp.at)
	}

	// Another edge along the very run: the label does not stand on both.
	both := &drawing{
		right: false, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{
			0: {{0, 0}, {0, 200}}, 1: {{0, 0}, {0, 200}},
		}, ends: map[int][2]int{0: {0, 0}, 1: {0, 0}}, back: map[int]bool{}, spots: map[int]spot{},
	}
	if sp, ok := both.seek(0, 40); ok {
		t.Errorf("a label on a run another edge shares, at %v", sp.at)
	}
}

func TestGraphLabelMiddle(t *testing.T) {
	t.Parallel()

	g := labelled(graphOf([]string{"a", "b"}, "a>b"), "go", 0)
	l := laid(g)
	r, s := l.labels[0], l.paths[0].segs[0]
	if r.y+r.h/2 != (s[1]+s[3])/2 || s[3]-s[1] != labelH+2*labelEnds {
		t.Errorf("label at y %v on a run %v: want the run just long enough, the label at its middle", r.y, s)
	}
}

// crowd is a drawing down: a box at 0,0 48x30, edge 0 a short run out of its bottom at x 24, edge 2 a run beside it
// at x 12, and edge 1 a run across from x 25 at y, its label on it from x 26, on a box far off.
func crowd(y float64) *drawing {
	far := rect{x: 1000, y: 1000, w: 10, h: 10, what: "", at: ""} //nolint:exhaustruct_v5 // a box
	d := &drawing{right: false, boxes: map[int]rect{
		0: {x: 0, y: 0, w: 48, h: 30, what: "", at: ""}, //nolint:exhaustruct_v5 // a box
		1: far,
	}, shapes: map[int]string{0: "", 1: ""}, routes: map[int][][2]float64{
		0: {{24, 30}, {24, 60}}, 1: {{25, y}, {200, y}}, 2: {{12, -100}, {12, 300}},
	}, ends: map[int][2]int{0: {0, 1}, 1: {1, 1}, 2: {1, 1}}, back: map[int]bool{}, spots: map[int]spot{
		1: {edge: 1, seg: 0, beside: false, at: 26, w: 30},
	}}
	return d
}

// A label keeps clear of where an edge written by hand leaves a box of the graph, the middle of a side the graph's
// edges do not use; and where no run of its edge takes room for it, it is floe's failure.
func TestGraphLabelRoom(t *testing.T) {
	t.Parallel()

	box := rect{x: 0, y: 0, w: 48, h: 30, what: "", at: "", form: outline{hex: 0, radius: 0}} //nolint:exhaustruct_v5 // a box
	d := &drawing{
		right: false, boxes: map[int]rect{0: box}, shapes: map[int]string{0: ""},
		routes: map[int][][2]float64{0: {{60, 0}, {60, 40}}}, ends: map[int][2]int{0: {0, 0}}, back: map[int]bool{},
		spots: map[int]spot{},
	}
	if err := d.place(0, 20); err != nil {
		t.Fatal(err)
	}
	if r := d.rect(0); r.y <= box.y+box.h/2 && box.y+box.h/2 <= r.y+r.h {
		t.Errorf("label at %v,%v, across the middle of the box's right side at %v", r.x, r.y, box.y+box.h/2)
	}

	// A short run down out of a box, blocked left by a run beside it, and right by the label of a run across, which
	// starts just right of it at y (the cut a unit past the box is at 31): room is made past that label, which stays
	// with its run, whole, partly past the cut (y 28), or moves with it, partly short of the cut (y 35), or stands clear
	// of the cut (y 50). A label far across from where the new one goes, from x 150, leaves nothing in its way.
	for _, c := range []struct{ y, at, stays, moves float64 }{
		{28, 26, 6, 0}, {35, 26, 0, 5}, {50, 26, 0, 0}, {28, 150, 0, 0},
	} {
		d := crowd(c.y)
		d.spots[1] = spot{edge: 1, seg: 0, beside: false, at: c.at, w: 30}
		if stays, moves := d.crossed(1, 31, 25, 66); stays != c.stays || moves != c.moves {
			t.Errorf("y %v: the cut at 31 leaves %v of the label before the room, %v after; want %v, %v",
				c.y, stays, moves, c.stays, c.moves)
		}
	}
	for _, y := range []float64{28, 30} {
		d := crowd(y)
		if err := d.place(0, 30); err != nil {
			t.Fatalf("y %v: %v", y, err)
		}
		mine, theirs := d.rect(0), d.rect(1)
		if mine.overlaps(theirs) || !onRun(mine, segAt(d.routes[0], 0)) {
			t.Errorf("y %v: label at %v, the other at %v, the run %v", y, mine, theirs, d.routes[0])
		}
	}

	// A run down 2 right of a run, in the way of a label on it: the cut along the run falls between them, so that the
	// run in the way moves on with the rest, past the label.
	way := &drawing{right: false, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{
		0: {{24, 0}, {24, 300}}, 1: {{26, 0}, {26, 300}},
	}, ends: map[int][2]int{}, back: map[int]bool{}, spots: map[int]spot{}}
	sp := spot{edge: 0, seg: 0, beside: false, at: 100, w: 30}
	r := spotRect(segAt(way.routes[0], 0), sp)
	at, need := way.reach(r, 24, 1, 1)
	if at <= 24 || at >= 26 || need <= 0 {
		t.Fatalf("reach: cut at %v, %v on", at, need)
	}
	way.stretch(0, at, onGrid(need, column, math.Ceil))
	if x := way.routes[1][0][0]; x < r.x+r.w {
		t.Errorf("the run in the way at %v, the label to %v", x, r.x+r.w)
	}

	// A run down 2 left of a run: the cut falls between them, and the run and the label on it move on past it.
	back := &drawing{right: false, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{
		0: {{24, 0}, {24, 300}}, 1: {{22, 0}, {22, 300}},
	}, ends: map[int][2]int{}, back: map[int]bool{}, spots: map[int]spot{}}
	at, need = back.reach(spotRect(segAt(back.routes[0], 0), sp), 24, 1, -1)
	if at <= 22 || at >= 24 || need <= 0 {
		t.Fatalf("reach back: cut at %v, %v on", at, need)
	}
	back.stretch(0, at, onGrid(need, column, math.Ceil))
	if b := spotRect(segAt(back.routes[0], 0), sp); b.x <= 22 {
		t.Errorf("the label from %v, the run in the way at 22", b.x)
	}

	// A label above and left of the box: the group moves right and down by whole columns, keeping its grid, the
	// label's left and top within a column of 0.
	up := &drawing{
		right: false, boxes: map[int]rect{0: {x: 0, y: 0, w: 48, h: 30, what: "", at: ""}}, //nolint:exhaustruct_v5 // a box
		shapes: map[int]string{0: ""}, routes: map[int][][2]float64{0: {{12, 0}, {180, 0}}}, ends: map[int][2]int{0: {0, 0}},
		back: map[int]bool{}, spots: map[int]spot{0: {edge: 0, seg: 0, beside: false, at: -13, w: 30}}, loops: map[int]bool{},
	}
	up.stand(0, 0)
	lr := up.rect(0)
	if b := up.boxes[0]; math.Mod(b.x, column) != 0 || math.Mod(b.y, column) != 0 || lr.x < 0 || lr.x >= column ||
		lr.y < 0 || lr.y >= column {
		t.Errorf("stand: box at %v,%v, label at %v,%v", b.x, b.y, lr.x, lr.y)
	}

	// A run across a run too wide to go by, which pushing moves nowhere: the band opens past the cut at 1, and the label
	// stands at the middle of the run as it is then, not at the start of the band.
	band := &drawing{
		right: false, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{
			0: {{0, 0}, {100, 0}}, 1: {{50, -100}, {50, 100}},
		}, ends: map[int][2]int{0: {0, 0}, 1: {0, 0}}, back: map[int]bool{}, spots: map[int]spot{},
	}
	if err := band.place(0, 70); err != nil {
		t.Fatal(err)
	}
	if sp, ok := band.seek(0, 70); !ok || band.spots[0] != sp || sp.at != 63 {
		t.Errorf("the label from %v, seek finds %v %v, want from 63", band.spots[0].at, sp.at, ok)
	}

	// Placed again once the ends have settled, a level edge's label on a run too short stays: its boxes do not move.
	level := &drawing{
		right: false, boxes: map[int]rect{
			0: {x: 0, y: 0, w: 48, h: 20, what: "", at: ""},  //nolint:exhaustruct_v5 // a box
			1: {x: 58, y: 0, w: 48, h: 20, what: "", at: ""}, //nolint:exhaustruct_v5 // a box
		}, shapes: map[int]string{0: "", 1: ""}, routes: map[int][][2]float64{0: {{48, 10}, {58, 10}}},
		ends: map[int][2]int{0: {0, 1}}, back: map[int]bool{}, level: map[int][2]string{0: {"right", "left"}},
		spots: map[int]spot{0: {edge: 0, seg: 0, beside: false, at: 48, w: 10}},
	}
	level.reseat([]int{0})
	if x := level.boxes[1].x; x != 58 || level.spots[0].at != 48 {
		t.Errorf("box at %v, label from %v, want 58, 48", x, level.spots[0].at)
	}

	short := &drawing{
		right: false, boxes: map[int]rect{0: {x: 100, y: 0, w: 48, h: 60, what: "", at: ""}}, //nolint:exhaustruct_v5 // a box
		shapes: map[int]string{0: ""}, routes: map[int][][2]float64{0: {{10, 10}, {10, 20}}}, ends: map[int][2]int{0: {0, 0}},
		back: map[int]bool{}, spots: map[int]spot{},
	}
	if short.place(0, 20) == nil {
		t.Error("a label placed by a run too short for it, inside a box's span, where no cut makes room")
	}
}

// Loops, down and right, one or two on a box of each shape, labelled or not, lay out with no mistake, as graphFault
// asks (the other edges as with no loop, each loop turning 3 times, its labels a column out from the outer loop),
// on the side of the box the graph's edges do not use, and the graph's edges never refused as starting where they end.
func TestGraphLoops(t *testing.T) {
	t.Parallel()
	for _, direction := range []string{"", "right"} {
		for _, shape := range []string{"", "pill", "hex", "state"} {
			for _, n := range []int{1, 2} {
				for _, labelled := range []bool{false, true} {
					g := graphOf([]string{"idle", "run", "done"}, "idle>run", "run>done", "done>idle")
					g.Direction = direction
					g.Nodes[1].Box.Shape = shape
					for j := range n {
						e := graphEdge{From: "run", To: "run", Dashed: false, Bold: false, Tone: "", Start: nil, End: nil, Label: nil}
						if labelled {
							e.Label = Text{"en": strings.Repeat("w", 3+4*j)}
						}
						g.Edges = append(g.Edges, e)
					}
					name := fmt.Sprintf("%q %q %d labelled %v", direction, shape, n, labelled)
					if fault := graphFault(g); fault != "" {
						t.Fatalf("%s: %s", name, fault)
					}
					l := laid(g)
					box := l.names["run"].r
					for k := 3; k < len(g.Edges); k++ {
						for _, s := range l.paths[k].segs {
							if direction == "" && min(s[0], s[2]) < box.x+box.w-loopDepth ||
								direction == "right" && min(s[1], s[3]) < box.y+box.h-loopDepth {
								t.Errorf("%s: loop %v not on the box's loop side %v", name, l.paths[k].segs, box)
							}
						}
					}
				}
			}
		}
	}
}

// A loop's ends stand within loopDepth of its box's side however wide the box grows across after the loops are
// drawn, as room is made for others, so that they stay clear of the edges leaving the next side a column in: the
// corners of a box grown across round wider (a wide pill laid out right grows taller).
func TestGraphLoopEnds(t *testing.T) {
	t.Parallel()
	for _, right := range []bool{false, true} {
		for _, shape := range []string{"", "pill", "hex", "state"} {
			for _, n := range []int{1, 2, 3} {
				d := &drawing{
					right: right, boxes: map[int]rect{}, shapes: map[int]string{0: shape}, routes: map[int][][2]float64{},
					ends: map[int][2]int{}, back: map[int]bool{}, spots: map[int]spot{}, loops: map[int]bool{},
				}
				b := rect{x: 0, y: 0, w: 96, h: 29, what: "", at: ""} //nolint:exhaustruct_v5 // a box
				b.form = outlineOf(b.w, b.h, shape)
				d.boxes[0] = b
				ks := make([]int, n)
				for k := range ks {
					ks[k] = k
				}
				o := &graphOut{g: nil, at: "", groups: nil, passed: 0, loops: nil}
				for _, k := range ks {
					o.loops = append(o.loops, [2]int{k, 0})
				}
				o.g = &graph{X: 0, Y: 0, Direction: "", Nodes: nil, Edges: make([]graphEdge, n)}
				if err := o.drawLoops(d); err != nil {
					t.Fatal(err)
				}
				across, _ := d.axes()
				for _, grow := range []float64{0, 40, 400} {
					d.stretch(across, d.boxes[0].lo(across)+1, grow)
					g := d.boxes[0]
					for _, k := range ks {
						pts := d.routes[k]
						for _, p := range [][2]float64{pts[0], pts[3]} {
							if depth := g.lo(across) + g.size(across) - p[across]; depth > loopDepth+1e-9 {
								t.Errorf("right %v, %q, %d loops, grown %v: loop %d ends %v in from the side", right, shape, n, grow, k, depth)
							}
						}
					}
				}
			}
		}
	}
}

// A box grows once for its loops, as long as they need: by a label thousands of characters long (no cap on rounds
// of growing to reach), and no longer for the loops of a box of its rank, which grows with it.
func TestGraphLoopGrow(t *testing.T) {
	t.Parallel()
	for _, direction := range []string{"", "right"} {
		g := graphOf([]string{"a"}, "a>a")
		g.Direction = direction
		g.Edges[0].Label = Text{"en": strings.Repeat("w", 2000)}
		if fault := graphFault(g); fault != "" {
			t.Errorf("direction %q, a loop's label 2000 characters long: %s", direction, fault)
		}
	}
	loops := func(both bool) *graph {
		g := graphOf([]string{"s", "b", "c"}, "s>b", "s>c", "b>b", "b>b", "b>b")
		if both {
			g.Edges = append(g.Edges, graphOf(nil, "c>c", "c>c", "c>c").Edges...)
		}
		for k := 2; k < len(g.Edges); k++ {
			g.Edges[k].Label = Text{"en": []string{"one", "two", "three"}[(k-2)%3]}
		}
		return g
	}
	one, both := laid(loops(false)).names["b"].r, laid(loops(true)).names["b"].r
	if fault := graphFault(loops(true)); fault != "" {
		t.Fatal(fault)
	}
	if one.h != both.h {
		t.Errorf("box b %v tall with loops on c too, %v with loops on b alone", both.h, one.h)
	}
}

// reach makes room past where an edge written by hand leaves a box past the cut, from where it starts, though that
// is short of the run: the box past the run within the hand's length.
func TestGraphReachHands(t *testing.T) {
	t.Parallel()
	d := &drawing{
		right: false, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{0: {{90, 0}, {90, 200}}},
		ends: map[int][2]int{0: {0, 0}}, back: map[int]bool{}, spots: map[int]spot{}, loops: map[int]bool{},
	}
	d.boxes[0] = rect{x: 20, y: 0, w: 50, h: 20, what: "", at: ""}   //nolint:exhaustruct_v5 // a box
	d.boxes[1] = rect{x: 100, y: 80, w: 50, h: 40, what: "", at: ""} //nolint:exhaustruct_v5 // a box, its left hand from 76
	for i, b := range d.boxes {
		b.form = outlineOf(b.w, b.h, "")
		d.boxes[i] = b
	}
	r := rect{x: 91, y: 95, w: 40, h: 10, what: "", at: ""} //nolint:exhaustruct_v5 // a label beside the run
	at, need := d.reach(r, 90, 1, 1)
	d.stretch(0, at, need)
	if !free(r, nil, d.hands()) {
		t.Errorf("cut at %v, moved on %v: %v is not clear of the hands %v", at, need, r, d.hands())
	}

	// So short of the run, laid out right, a box's hand from its top reaching along the label: the run and the label
	// move past it.
	short := &drawing{
		right: true, boxes: map[int]rect{}, shapes: map[int]string{}, routes: map[int][][2]float64{0: {{90, 0}, {90, 200}}},
		ends: map[int][2]int{0: {0, 0}}, back: map[int]bool{}, spots: map[int]spot{}, loops: map[int]bool{},
	}
	short.boxes[0] = rect{x: 40, y: 100, w: 40, h: 20, what: "", at: ""} //nolint:exhaustruct_v5 // a box, its top hand at x 60
	sp := spot{edge: 0, seg: 0, beside: false, at: 80, w: 70}
	at, need = short.reach(spotRect(segAt(short.routes[0], 0), sp), 90, 1, -1)
	short.stretch(0, at, need)
	if r := spotRect(segAt(short.routes[0], 0), sp); !free(r, nil, short.hands()) {
		t.Errorf("cut at %v, moved on %v: %v is not clear of the hands %v", at, need, r, short.hands())
	}
}

// inGroup is g with the nodes named (boxes or groups, at its top) moved into a new group, where the first stood.
func inGroup(g *graph, name, label string, members ...string) *graph {
	var held []graphNode
	at := -1
	for i := 0; i < len(g.Nodes); i++ {
		n := g.Nodes[i]
		nm := ""
		if n.Box != nil {
			nm = n.Box.Name
		} else {
			nm = n.Group.Name
		}
		if !slices.Contains(members, nm) {
			continue
		}
		if at < 0 {
			at = i
		}
		held = append(held, n)
		g.Nodes = slices.Delete(g.Nodes, i, i+1)
		i--
	}
	var t Text
	if label != "" {
		t = Text{"en": label}
	}
	grp := graphNode{Box: nil, Group: &graphGroup{Name: name, Label: t, Accent: false, Nodes: held}}
	g.Nodes = slices.Insert(g.Nodes, at, grp)
	return g
}

// addGroups puts half the graphs' boxes at random into 1 to 3 groups, some nested, some labelled, and ends a tenth
// of their edges at a group instead, dropping those between a group and what it holds or among its ranks (told).
func addGroups(rnd *rand.Rand, g *graph) {
	if rnd.IntN(2) == 0 {
		return
	}
	var made []string
	for c := range 1 + rnd.IntN(3) {
		var top []string
		for _, n := range g.Nodes {
			if n.Box != nil || rnd.IntN(2) == 0 {
				if n.Box != nil {
					top = append(top, n.Box.Name)
				} else {
					top = append(top, n.Group.Name)
				}
			}
		}
		rnd.Shuffle(len(top), func(i, j int) { top[i], top[j] = top[j], top[i] })
		size := 1 + rnd.IntN(max(1, len(top)/3))
		if size >= len(top) {
			break
		}
		name, label := fmt.Sprintf("g%d", c), ""
		if rnd.IntN(2) == 0 {
			label = strings.Repeat("w", 1+rnd.IntN(12))
		}
		inGroup(g, name, label, top[:size]...)
		made = append(made, name)
	}
	for k := range g.Edges {
		if len(made) > 0 && g.Edges[k].From != g.Edges[k].To && rnd.IntN(10) == 0 {
			if rnd.IntN(2) == 0 {
				g.Edges[k].From = made[rnd.IntN(len(made))]
			} else {
				g.Edges[k].To = made[rnd.IntN(len(made))]
			}
		}
	}
	for {
		_, problems := readGraph(g, "#/parts/0/graph", (*drawing).settleEnds)
		var drop []int
		for _, p := range problems {
			if p.Key == "diagram_graph_inside" || p.Key == "diagram_graph_among" {
				k, err := strconv.Atoi(strings.Split(strings.TrimPrefix(p.At(), "#/parts/0/graph/edges/"), "/")[0])
				if err != nil {
					panic(err)
				}
				drop = append(drop, k)
			}
		}
		if len(drop) == 0 {
			return
		}
		slices.Sort(drop)
		for _, k := range slices.Backward(slices.Compact(drop)) {
			g.Edges = slices.Delete(g.Edges, k, k+1)
		}
	}
}

// inLanes wraps the nodes of g at its top, as written, into 2 to 4 lanes of them in a row (fewer if it has fewer),
// some labelled, its groups nested in them.
func inLanes(rnd *rand.Rand, g *graph) {
	n := min(len(g.Nodes), 2+rnd.IntN(3))
	cuts := make([]int, 0, n+1)
	cuts = append(cuts, 0, len(g.Nodes))
	for _, c := range rnd.Perm(len(g.Nodes) - 1)[:n-1] {
		cuts = append(cuts, c+1)
	}
	slices.Sort(cuts)
	nodes := g.Nodes
	g.Nodes = nil
	for i := range n {
		var t Text
		if rnd.IntN(2) == 0 {
			t = Text{"en": strings.Repeat("w", 1+rnd.IntN(12))}
		}
		g.Lanes = append(g.Lanes, graphGroup{Name: fmt.Sprintf("lane%d", i), Label: t, Accent: false, Nodes: nodes[cuts[i]:cuts[i+1]]})
	}
}

// inLaned makes the groups at the top of g, as written, its lanes.
func inLaned(g *graph) {
	for _, n := range g.Nodes {
		g.Lanes = append(g.Lanes, *n.Group)
	}
	g.Nodes = nil
}

// A graph's groups keep their boxes side by side in every rank, framed apart from all else: nested, side by side,
// boxes with no edge, edges in and out of them at any rank, round trips, loops, labels and a group of boxes beside.
func TestGraphGroups(t *testing.T) {
	t.Parallel()
	base := func() *graph {
		return graphOf([]string{"cli", "api", "store", "cache", "page", "z"}, "cli>api", "api>store", "api>cache", "store>page")
	}
	for _, direction := range []string{"", "right"} {
		for _, c := range []struct {
			name string
			g    func() *graph
		}{
			{"one", func() *graph { return inGroup(base(), "server", "Local server", "api", "store") }},
			{"nested", func() *graph {
				return inGroup(inGroup(base(), "data", "Data", "store", "cache"), "server", "Local server", "api", "data")
			}},
			{"side by side", func() *graph {
				return inGroup(inGroup(base(), "a", "A", "api"), "b", "", "store", "cache")
			}},
			{"no edges", func() *graph { return inGroup(base(), "lone", "Lone", "z", "page") }},
			{"in and out at any rank", func() *graph {
				g := inGroup(base(), "server", "Local server", "api", "store", "cache")
				g.Edges = append(g.Edges, graphOf(nil, "cli>store", "api>page", "cli>cache").Edges...)
				return g
			}},
			{"round trip", func() *graph {
				g := inGroup(base(), "server", "Local server", "api", "store")
				g.Edges = append(g.Edges, graphOf(nil, "store>api").Edges...)
				return g
			}},
			{"loop", func() *graph {
				g := inGroup(base(), "server", "Local server", "api", "store")
				g.Edges = append(g.Edges, graphOf(nil, "store>store", "api>api").Edges...)
				return g
			}},
			{"labels", func() *graph {
				g := inGroup(base(), "server", "Local server", "api", "store", "cache")
				for k := range g.Edges {
					g.Edges[k].Label = Text{"en": "label"}
				}
				return g
			}},
		} {
			g := c.g()
			g.Direction = direction
			if fault := graphFault(g); fault != "" {
				t.Errorf("%q %s: %s", direction, c.name, fault)
			}
		}
	}
}

// Lanes stand as written, nodeGap apart, each from end to end of the graph: headed or bare, a group nested at the
// first rank or the last, one with no edge, and lanes whose edges the order would have swapped. A group nested in a
// lane frames only its boxes' ranks.
func TestGraphLanes(t *testing.T) {
	t.Parallel()
	for _, direction := range []string{"", "right"} {
		for _, c := range []struct {
			name string
			g    func() *graph
		}{
			{"headed, bare and nested", func() *graph {
				g := graphOf([]string{"a", "b", "c", "d", "e", "f"}, "a>b", "b>c", "c>d", "d>e", "a>f")
				inGroup(inGroup(g, "top", "Top", "a"), "inner", "", "e")
				return inGroup(inGroup(inGroup(g, "A", "A", "top", "c"), "B", "", "b", "d"), "C", "C", "inner", "f")
			}},
			{"a lane with no edge", func() *graph {
				g := graphOf([]string{"a", "b", "c", "z"}, "a>b", "b>c")
				return inGroup(inGroup(inGroup(g, "X", "X", "a", "b"), "Y", "Y", "c"), "Z", "Z", "z")
			}},
			{"against the order", func() *graph {
				g := graphOf([]string{"a", "b", "c", "x", "y", "z"}, "a>b", "x>z", "y>c")
				return inGroup(inGroup(inGroup(g, "L1", "L1", "a", "b", "c"), "L2", "L2", "x"), "L3", "L3", "y", "z")
			}},
		} {
			g := c.g()
			g.Direction = direction
			inLaned(g)
			if fault := graphFault(g); fault != "" {
				t.Errorf("%q %s: %s", direction, c.name, fault)
				continue
			}
			along := 1
			if direction == "right" {
				along = 0
			}
			_, clusters, _ := flatten(g, "#")
			l := laid(g)
			long := 0.0
			for _, f := range l.frames {
				long = max(long, f.size(along))
			}
			for i, f := range l.frames {
				if nested := clusters[i].parent >= 0; nested == (f.size(along) == long) {
					t.Errorf("%q %s: frame %v, nested %v, %v long, the lanes %v", direction, c.name, f, nested, f.size(along), long)
				}
			}
		}
	}
}

// An edge needs no turn a group makes it take: one beside a frame runs straight down by it, the frame moved out of
// its way, and one between groups passes inside the group spanning its ranks, not round the frames. Making room
// for a point moves frames, but never widens one.
func TestGraphGroupStraight(t *testing.T) {
	t.Parallel()
	for _, direction := range []string{"", "right"} {
		g := graphOf([]string{"cli", "api", "store", "cache", "page", "disk"},
			"cli>api", "api>store", "api>cache", "store>page", "cli>server", "server>db", "cli>page", "server>cli")
		g.Nodes[0].Box.Rows[0].Text = Text{"en": "floe publish"} // as wide as the plan's picture
		g = inGroup(inGroup(g, "data", "Data", "store", "cache"), "server", "Local server", "api", "data")
		g = inGroup(g, "db", "Database", "disk")
		g.Direction = direction
		for _, p := range laid(g).paths {
			if p.what == "edge cli→page" && len(p.segs) != 1 {
				t.Errorf("%q: cli→page beside the frame turns: %v", direction, p.segs)
			}
		}

		// x in group a, at b's first rank; b spans four ranks: x→b3's points stand in b.
		between := inGroup(inGroup(graphOf([]string{"x", "b0", "b1", "b2", "b3"}, "b0>b1", "b1>b2", "b2>b3", "x>b3"),
			"a", "A", "x"), "b", "B", "b0", "b1", "b2", "b3")
		between.Direction = direction
		grs, err := grouped(between)
		if err != nil {
			t.Fatal(err)
		}
		points := 0
		for _, n := range grs[0].nodes {
			if n.isPoint() && n.of == 3 {
				points++
				if n.cl != 1 {
					t.Errorf("%q: a point of x→b3 at rank %d stands in group %d, not b", direction, n.rank, n.cl)
				}
			}
		}
		if points != 2 {
			t.Errorf("%q: x→b3 passes %d points, want 2", direction, points)
		}
	}

	// Every frame as wide as before room is made for the points, in random graphs and the two above.
	rnd := rand.New(rand.NewPCG(9, 10))     //nolint:gosec // G404: graphs to try, no secret
	groups := rand.New(rand.NewPCG(29, 10)) //nolint:gosec // G404: graphs to try, no secret
	for i := range 600 {
		direction := []string{"", "right"}[i%2]
		g := randomGraph(rnd, direction)
		addGroups(groups, g)
		if fault := wideningFault(g); fault != "" {
			t.Fatalf("graph %d %q: %s", i, direction, fault)
		}
	}
}

// wideningFault is a frame of g that making room for the points widens, or "".
func wideningFault(g *graph) string {
	right := g.Direction == "right"
	widths := func(gr *group) map[int]float64 {
		out := map[int]float64{}
		for _, n := range gr.nodes {
			switch n.kind {
			case kindLeft:
				out[n.cl] -= n.x
			case kindRight:
				out[n.cl] += n.x
			case kindPlain, kindHeading, kindIn, kindOut:
			}
		}
		return out
	}
	before, _ := readGraph(g, "#/parts/0/graph", (*drawing).settleEnds) //nolint:errcheck // its mistakes are graphFault's
	after, _ := readGraph(g, "#/parts/0/graph", (*drawing).settleEnds)  //nolint:errcheck // as above
	for k, gr := range before.groups {
		gr.split()
		gr.size(right)
		gr.order()
		gr.pack(nil)
		gr.pull()
		ga := after.groups[k]
		ga.place(right)
		// A group's marks stand in each of its ranks: its width counted once a rank.
		wb, wa := widths(gr), widths(ga)
		for c, w := range wb {
			if wa[c] > w {
				return fmt.Sprintf("group %d widens from %v to %v", c, w, wa[c])
			}
		}
	}
	return ""
}

// An edge to a group ends at its frame's top (from above) or leaves its bottom, each at a point of its own clear
// of the heading: from a box, to one, between groups with no edge between their boxes, into a nested group, back,
// and into a group whose second rank another edge from that box enters.
func TestGraphGroupEdges(t *testing.T) {
	t.Parallel()
	base := func(edges ...string) *graph {
		g := graphOf([]string{"cli", "api", "store", "cache", "page", "disk"},
			append([]string{"cli>api", "api>store", "api>cache"}, edges...)...)
		g = inGroup(inGroup(g, "data", "Data", "store", "cache"), "server", "Local server", "api", "data")
		return inGroup(g, "db", "Database", "disk")
	}
	for _, direction := range []string{"", "right"} {
		for _, c := range []struct {
			name  string
			edges []string
		}{
			{"in", []string{"cli>server"}},
			{"out", []string{"server>page"}},
			{"between groups", []string{"server>db"}},
			{"into a nested group", []string{"cli>data"}},
			{"back", []string{"cli>server", "server>cli"}},
			{"in, beside one to the second rank", []string{"cli>server", "cli>store", "cli>data"}},
			{"labelled", []string{"cli>server", "server>db", "data>page"}},
		} {
			g := base(c.edges...)
			g.Direction = direction
			for k := range g.Edges {
				if c.name == "labelled" {
					g.Edges[k].Label = Text{"en": "label"}
				}
			}
			if fault := graphFault(g); fault != "" {
				t.Errorf("%q %s: %s", direction, c.name, fault)
			}
		}
	}
}

// The hands of a box come in one order, its sides' as written: left then right laid out down, top then bottom laid
// out right; a side a loop or a level edge takes has none.
func TestGraphHandsOrder(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		right  bool
		looped bool
		taken  string
		want   []seg
	}{
		{false, false, "", []seg{{-4, 34.5, 20, 34.5}, {60, 34.5, 84, 34.5}}},
		{true, false, "", []seg{{40, -4, 40, 20}, {40, 49, 40, 73}}},
		{false, true, "", []seg{{-4, 34.5, 20, 34.5}}},
		{true, true, "", []seg{{40, -4, 40, 20}}},
		{false, false, "left", []seg{{60, 34.5, 84, 34.5}}},
		{true, false, "bottom", []seg{{40, -4, 40, 20}}},
	} {
		d := &drawing{ //nolint:exhaustruct_v5 // a box and its taken sides
			right: c.right, boxes: map[int]rect{0: {x: 20, y: 20, w: 40, h: 29, what: "", at: ""}}, //nolint:exhaustruct_v5 // a box
			taken: map[int]map[string]bool{0: {c.taken: c.taken != ""}},
		}
		if got := d.handsOf(0, c.looped); !slices.Equal(got, c.want) {
			t.Errorf("right %v, looped %v, %q taken: %v, want %v", c.right, c.looped, c.taken, got, c.want)
		}
	}
}

// The n edges at a box's side end at the points that part it into n+1 equal parts, in the order of what they reach,
// down and right and on every shape, whose outline they stay on. One edge runs down the boxes' middles, and a straight
// edge among more, which stays straight, is not held to its part.
func TestGraphBoxParts(t *testing.T) {
	t.Parallel()

	for _, direction := range []string{"", "right"} {
		for _, shape := range []string{"", "pill", "hex", "state"} {
			for _, n := range []int{1, 2, 3, 5, 8} {
				names := []string{"hub"}
				var edges []string
				for i := range n {
					names = append(names, fmt.Sprintf("n%d", i))
					edges = append(edges, fmt.Sprintf("hub>n%d", i), fmt.Sprintf("n%d>sink", i))
				}
				g := graphOf(append(names, "sink"), edges...)
				g.Direction = direction
				for _, b := range graphBoxes(g) {
					b.Shape, b.Rows[0].Text["en"] = shape, "box"
				}
				at := fmt.Sprintf("%q, %q, %d edges", direction, shape, n)
				l := laid(g)
				if fault := laidFault(g, l); fault != "" {
					t.Fatalf("%s: %s", at, fault)
				}
				axis := 0
				if direction == "right" {
					axis = 1
				}
				hub, sink := l.names["hub"].r, l.names["sink"].r
				for i := range n {
					from, to := l.paths[2*i].segs, l.paths[2*i+1].segs
					want := float64(i+1) / float64(n+1)
					if got := (from[0][axis] - hub.lo(axis)) / hub.size(axis); (n == 1 || len(from) > 1) && math.Abs(got-want) > 1e-6 {
						t.Errorf("%s: %s leaves %v along the side, want %v", at, l.paths[2*i].what, got, want)
					}
					if got := (to[len(to)-1][2+axis] - sink.lo(axis)) / sink.size(axis); (n == 1 || len(to) > 1) && math.Abs(got-want) > 1e-6 {
						t.Errorf("%s: %s ends %v along the side, want %v", at, l.paths[2*i+1].what, got, want)
					}
				}
			}
		}
	}
}

// A box's level edges stay where the ranks put them, its other ends parted round them.
func TestGraphBoxLevelStays(t *testing.T) {
	t.Parallel()

	g := graphOf([]string{"a", "b", "c", "d"}, "a>b", "b>a", "a>c", "a>d", "b>c", "b>d")
	g.Edges[0].Level, g.Edges[1].Level = true, true
	l, r := laid(g), routed(g)
	if fault := laidFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	for k := range 2 {
		if !slices.Equal(l.paths[k].segs, r.paths[k].segs) {
			t.Errorf("%s runs %v, %v before the ends moved", l.paths[k].what, l.paths[k].segs, r.paths[k].segs)
		}
	}
	if from, want := l.paths[2].segs[0], 20+64.0/3; math.Abs(from[0]-want) > 1e-6 {
		t.Errorf("%s leaves at %v, want %v: the other ends part the side", l.paths[2].what, from[0], want)
	}
}

// A straight edge holds its line, off its parts, and the other ends on its sides stand evenly between it and the sides'
// ends.
func TestGraphBoxSpread(t *testing.T) {
	t.Parallel()

	g := boxesOf(graphOf([]string{"a", "b", "c"}, "a>c", "b>c", "a>c"), 11, 6, 6)
	g.Direction = "right"
	want := `box "a" 20,26 128x64
box "b" 40,116 88x48
box "c" 184,20 88x72
edge a→c M148,44 H184
edge b→c M128,140 H160 V76 H184
edge a→c M148,67 H160 V60 H184
`
	if got := laid(g).inspect(); got != want {
		t.Errorf("got\n%swant\n%s", got, want)
	}
}

// A moved end stays where it is rather than run close beside another edge, along however short a stretch: a→d would leave
// at a's two thirds, x 62.67, and share 12 with b→c, 0.67 from it.
func TestGraphBoxKeepApart(t *testing.T) {
	t.Parallel()

	g := graphOf([]string{"a", "b", "c", "d"}, "a>b", "b>a", "a>c", "a>d", "b>c", "b>d")
	g.Edges[0].Level, g.Edges[1].Level = true, true
	want := `box "a" 20,20 64x29
box "b" 110,20 64x29
box "c" 20,97 64x29
box "d" 110,97 64x29
edge a→b M84,28.5 H110
edge b→a M110,40.5 H84
edge a→c M41.33,49 V97
edge a→d M68,49 V73 H131.33 V97
edge b→c M131.33,49 V61 H62 V97
edge b→d M152.67,49 V97
`
	if got := laid(g).inspect(); got != want {
		t.Errorf("got\n%swant\n%s", got, want)
	}
}

// Two runs a rounding apart are one line when drawn, and touch.
func TestRunsTouchRounded(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		s, q seg
		want bool
	}{
		{"on one line, a rounding apart", seg{0, 120, 50, 120}, seg{10, 119.99999999999998, 40, 119.99999999999998}, true},
		{"an end on a run, a rounding off", seg{0, 120, 50, 120}, seg{20, 119.99999999999998, 20, 200}, true},
		{"side by side", seg{0, 120, 50, 120}, seg{10, 121, 40, 121}, false},
		{"crossing", seg{0, 120, 50, 120}, seg{20, 100, 20, 200}, false},
		{"end to end on one line", seg{0, 120, 50, 120}, seg{50, 120, 90, 120}, true},
	} {
		if got := runsTouch(c.s, c.q); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// boxesOf is g with the boxes' words the given lengths in w's, so that their widths are as the layouts below want them.
func boxesOf(g *graph, lengths ...int) *graph {
	for i, n := range lengths {
		g.Nodes[i].Box.Rows[0].Text["en"] = strings.Repeat("w", n)
	}
	return g
}

// A straight edge whose point on the side with more ends the other side does not take runs on the point of the other.
func TestGraphBoxOtherPoint(t *testing.T) {
	t.Parallel()

	g := boxesOf(graphOf([]string{"a", "b", "c"}, "a>c", "a>b"), 18, 4, 1)
	want := `box "a" 20,20 184x29
box "b" 20,73 72x29
box "c" 116,73 48x29
edge a→c M142.67,49 V73
edge a→b M56,49 V73
`
	if got := laid(g).inspect(); got != want {
		t.Errorf("got\n%swant\n%s", got, want)
	}
}

// A labelled edge between boxes turns no more than once beyond its turns with no label: it may not slide on to its other
// end's point, and bends once there.
func TestGraphBoxLabelledStays(t *testing.T) {
	t.Parallel()

	g := boxesOf(graphOf([]string{"a", "b", "c"}, "a>c", "b>c", "b>c", "b>c"), 12, 6, 16)
	g.Direction = "right"
	g.Edges[1].Label, g.Edges[2].Label = Text{"en": "yes"}, Text{"en": "yes"}
	l := laid(g)
	if fault := laidFault(g, l); fault != "" {
		t.Fatal(fault)
	}
	if fault := turnFault(g, l); fault != "" {
		t.Error(fault)
	}
}

// An edge between a box and an icon does not take the box's point, which would move the icon for nothing and leave the
// next edge at the icon short of it.
func TestGraphBoxIconStays(t *testing.T) {
	t.Parallel()

	g := boxesOf(graphOf([]string{"a", "b"}, "a>b"), 14, 8)
	g.Nodes[1] = graphNode{Icon: iconNode(g.Nodes[1].Box)} //nolint:exhaustruct_v5 // one kind of node
	want := `box "a" 20,20 152x29
icon "b" 68,106 48x48
edge a→b M92,49 V106
label "wwwwwwww" 58.2,157 67.6x18
`
	if got := laid(g).inspect(); got != want {
		t.Errorf("got\n%swant\n%s", got, want)
	}
}

// addWiths gives 1 to 4 boxes of g, at random from rnd, a with of another box.
func addWiths(rnd *rand.Rand, g *graph) {
	boxes := graphBoxes(g)
	for range 1 + rnd.IntN(4) {
		a, b := boxes[rnd.IntN(len(boxes))], boxes[rnd.IntN(len(boxes))]
		if a != b {
			a.With = b.Name
		}
	}
}

// withTold reports whether g is told, and told only of what withs and level edges refuse.
func withTold(g *graph) bool {
	l := laid(g)
	for _, p := range l.problems {
		if p.Key != "diagram_graph_with_rank" && p.Key != "diagram_graph_among" && !strings.HasPrefix(p.Key, "diagram_graph_level_") {
			return false
		}
	}
	return l.failure == nil && len(l.problems) > 0
}

// withFault says which box of g stands in another rank than the box its with names, or "".
func withFault(g *graph) string {
	groups, err := grouped(g)
	if err != nil {
		return err.Error()
	}
	ranks := map[string]int{}
	for _, gr := range groups {
		for _, n := range gr.nodes {
			if n.box != nil {
				ranks[n.box.Name] = n.rank
			}
		}
	}
	for _, b := range graphBoxes(g) {
		if b.With != "" && ranks[b.Name] != ranks[b.With] {
			return fmt.Sprintf("%s stands in rank %d, its with %s in rank %d", b.Name, ranks[b.Name], b.With, ranks[b.With])
		}
	}
	return ""
}

// Graphs with boxes of a with at random, half with level edges too, with no group, groups and lanes: none fails, the
// boxes of a with stand in one rank, and at least least of them are laid out, those told of what with refuses skipped.
// It is a test of its own, so that its draws leave TestGraphRandom's alone.
func TestGraphWithRandom(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		direction string
		seed      uint64
		groups    bool
		lanes     bool
		least     int
	}{
		{"", 1, false, false, 88},
		{"right", 2, false, false, 90},
		{"", 3, true, false, 89},
		{"right", 4, true, false, 88},
		{"", 5, false, true, 85},
		{"right", 6, false, true, 76},
	} {
		t.Run(fmt.Sprintf("%s-%d", cmpOr(c.direction, "down"), c.seed), func(t *testing.T) {
			t.Parallel()

			rnd := rand.New(rand.NewPCG(c.seed, 7))    //nolint:gosec // G404: graphs to try, no secret
			groups := rand.New(rand.NewPCG(c.seed, 8)) //nolint:gosec // G404: graphs to try, no secret
			levels := rand.New(rand.NewPCG(c.seed, 9)) //nolint:gosec // G404: graphs to try, no secret
			laidOut := 0
			for i := range tries(300) {
				g := randomGraph(rnd, c.direction)
				if c.groups || c.lanes {
					addGroups(groups, g)
				}
				if c.lanes {
					inLanes(groups, g)
				}
				if i%2 == 0 {
					addLevels(levels, g)
				}
				addWiths(rnd, g)
				if withTold(g) {
					continue
				}
				laidOut++
				l := laid(g)
				if fault := withFault(g); fault != "" {
					t.Fatalf("seed %d, graph %d: %s", c.seed, i, fault)
				}
				if fault := laidFault(g, l); fault != "" {
					b, _ := json.Marshal(map[string]any{"title": "kept", "parts": []any{map[string]any{"graph": shrunk(g)}}}) //nolint:errcheck,errchkjson,musttag // the test's own
					t.Fatalf("seed %d, graph %d: %s; shrunk, to keep in testdata/graph:\n%s", c.seed, i, fault, b)
				}
			}
			t.Logf("seed %d: %d laid out", c.seed, laidOut)
			if !underRace && laidOut < c.least {
				t.Errorf("seed %d: %d graphs with withs laid out, fewer than %d", c.seed, laidOut, c.least)
			}
		})
	}
}

// A graph shrunk drops a with of the box it took off, and fails as it did.
func TestWithoutWith(t *testing.T) {
	t.Parallel()

	g := graphOf([]string{"a", "b"})
	g.Nodes[1].Box.With = "a"
	if got := withoutWith(g.Nodes, "a"); got[1].Box.With != "" || g.Nodes[1].Box.With != "a" {
		t.Errorf("with %q of the copy, %q of the graph", got[1].Box.With, g.Nodes[1].Box.With)
	}
}
