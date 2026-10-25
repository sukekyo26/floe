package diagram

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Text sizes and box metrics of diagram.mjs.
const (
	sizeMain  = 13
	sizeSub   = 12
	pad       = 16 // a box's inner margin on each side
	lineGap   = 17 // between the rows of a box
	tableHead = 30
	tableRow  = 22
	tableSep  = 9
	cornerR   = 8 // the corner radius of a box and a table
)

// rect is the area a part takes, with the words that name it in messages.
type rect struct {
	x, y, w, h float64
	what       *usererr.Error
	at         yamlpos.Pos
}

func (a rect) overlaps(b rect) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

func (a rect) inside(b rect) bool {
	return a.x >= b.x && a.y >= b.y && a.x+a.w <= b.x+b.w && a.y+a.h <= b.y+b.h
}

type path struct {
	edge Edge
	own  int // the index in labels of the edge's label, which it may run through; -1 without one
}

// layout takes the parts in order, as diagram.mjs does, and gathers both what the checks compare and
// the SVG of each part, so that the checks and the drawing read the same geometry.
type layout struct {
	spec     *Spec
	icons    *icons.Store
	err      error        // the first failure of the icons, which no user can fix
	unplaced usererr.List // the icons' mistakes, of no place: not fetched, no directory, an index or icon unread
	boxes    []rect       // edges may not run through these
	marks    []rect       // may not overlap, but edges may reach their centre (dots)
	frames   []rect
	areas    []rect // may overlap (Venn circles)
	labels   []rect
	heads    []rect // frame headings, which edges may not cross
	strokes  []rect // the spans of edges and lines, which must stay on the canvas
	paths    []path
	problems usererr.List
	seen     map[string]bool
	// The SVG of the layers, drawn bottom to top.
	areaSVG, edgeSVG, boxSVG, labelSVG []markup
	// Axis grid lines go below every other edge, last first: diagram.mjs unshifts them.
	grids   []markup
	markers []string // the marker kinds the edges use, in the order of first use
	// Each icon at each size is defined once (in <defs>) and placed with <use>, however often it shows.
	iconDefs []markup
	iconIDs  map[string]string // by name and size
}

// newLayout lays out s. A failure of the icons stops it with an error, as no mistake of s does.
func newLayout(s *Spec, store *icons.Store) (*layout, error) {
	l := &layout{
		spec: s, icons: store, err: nil, boxes: nil, marks: nil, frames: nil, areas: nil, labels: nil, heads: nil,
		strokes: nil, paths: nil, problems: nil, unplaced: nil, seen: map[string]bool{},
		areaSVG: nil, edgeSVG: nil, boxSVG: nil, labelSVG: nil, grids: nil, markers: nil,
		iconDefs: nil, iconIDs: map[string]string{},
	}
	for _, p := range s.Parts {
		l.add(p)
	}
	if l.err != nil {
		return nil, l.err
	}
	return l, nil
}

func (l *layout) problem(at yamlpos.Pos, key string, args ...any) {
	e := yamlpos.New(l.spec.File, at, key, args...)
	if key := e.Error(); !l.seen[key] {
		l.seen[key] = true
		l.problems = append(l.problems, e)
	}
}

func (l *layout) add(p Part) {
	switch p := p.(type) {
	case Frame:
		l.addFrame(p)
	case Icon:
		l.addIcon(p)
	case Box:
		l.addBox(p)
	case Table:
		l.addTable(p)
	case Edge:
		l.addEdge(p)
	case Dot:
		what := usererr.New("diagram_what_dot", formatNum(p.CX), formatNum(p.CY))
		l.marks = append(l.marks, rect{p.CX - p.R, p.CY - p.R, p.R * 2, p.R * 2, what, p.Pos})
		cls := "dot" + tone(p.Tone)
		if p.Hollow {
			cls += " hollow"
		}
		l.boxSVG = append(l.boxSVG, svgf(`<circle cx="%v" cy="%v" r="%v" class="%v"/>`, p.CX, p.CY, p.R, cls))
	case Bar:
		l.addBar(p)
	case Axis:
		l.addAxis(p)
	case Circle:
		what := usererr.New("diagram_what_circle", formatNum(p.CX), formatNum(p.CY))
		l.areas = append(l.areas, rect{p.CX - p.R, p.CY - p.R, p.R * 2, p.R * 2, what, p.Pos})
		l.areaSVG = append(l.areaSVG,
			svgf(`<circle cx="%v" cy="%v" r="%v" class="%v"/>`, p.CX, p.CY, p.R, "venn"+tone(p.Tone)))
	case Label:
		l.label(p.Pos, p.Text, p.X, p.Y, p.Anchor, p.Plain, p.Strong)
	case Line:
		what := usererr.New("diagram_what_line", formatNum(p.X1), formatNum(p.Y1), formatNum(p.X2), formatNum(p.Y2))
		l.strokes = append(l.strokes, span([][4]float64{{p.X1, p.Y1, p.X2, p.Y2}}, what, p.Pos))
		l.edgeSVG = append(l.edgeSVG,
			line(p.X1, p.Y1, p.X2, p.Y2, p.Kind))
	case Raw:
		l.edgeSVG = append(l.edgeSVG, markup(p.SVG))
	}
}

// tone is the class that a tone adds.
func tone(t string) string {
	if t == "" {
		return ""
	}
	return " " + t
}

// addFrame keeps the frame's heading too: inside the top left corner, or right of the corner icon.
func (l *layout) addFrame(f Frame) {
	what := usererr.New("diagram_what_frame", f.Label)
	l.frames = append(l.frames, rect{f.X, f.Y, f.W, f.H, what, f.Pos})
	hx, hy, size := f.X+12, f.Y+18, float64(sizeSub)
	if f.Icon != "" {
		hx, hy, size = f.X+40, f.Y+15, sizeMain
	}
	if f.Label != "" {
		head := usererr.New("diagram_what_heading", f.Label)
		l.heads = append(l.heads, rect{hx, hy - 9, textWidth(f.Label, size, false), 18, head, f.Pos})
		// The heading keeps the margin on its right that it has on its left.
		l.fit(f.Pos, what, f.Label, size, false, f.W-(hx-f.X)-12)
	}
	rx, cls := 10.0, "frame"
	if f.Icon != "" || f.Square {
		rx = 0
	}
	if f.Accent {
		cls += " accent"
	}
	var corner markup
	if f.Icon != "" {
		cls += " grouped"
		// The icon is a part like a box: nothing may overlap it or run through it. It is checked from
		// the frame's edge, as the 1px of it outside the frame only covers the frame's line.
		l.boxes = append(l.boxes, rect{f.X, f.Y, 31, 31, usererr.New("diagram_what_frame_icon", f.Label), f.Pos})
		// The frame's line (1.2 wide) is centred on its edge, so the icon moved out by 1 covers its
		// outer half (0.6) too.
		corner = l.image(f.Pos, f.Icon, f.X-1, f.Y-1, 32, what)
	}
	box := svgf(`<g class="%v"><rect x="%v" y="%v" width="%v" height="%v" rx="%v"/>`, cls, f.X, f.Y, f.W, f.H, rx)
	head := svgf(`<text x="%v" y="%v" text-anchor="start">%v</text></g>`, hx, hy, f.Label)
	l.areaSVG = append(l.areaSVG, box+corner+head)
}

func (l *layout) addIcon(i Icon) {
	what := usererr.New("diagram_what_icon", i.Name)
	l.boxes = append(l.boxes, rect{i.X, i.Y, i.Size, i.Size, what, i.Pos})
	l.boxSVG = append(l.boxSVG, l.image(i.Pos, i.Name, i.X, i.Y, i.Size, what))
	if i.Label != "" {
		l.label(i.Pos, i.Label, i.X+i.Size/2, i.Y+i.Size+16, "middle", false, false)
	}
}

// image places the icon of name (aws/…). An icon is an
// <image>, not inline <svg>: the official icons' ids collide between icons. An icon not there is a mistake
// of the spec; icons not fetched or not read are a mistake of no place, and a failure that is no mistake
// stops the drawing.
func (l *layout) image(at yamlpos.Pos, name string, x, y, size float64, what *usererr.Error) markup {
	icon, names, err := l.icons.Lookup(name)
	if errors.Is(err, icons.ErrNoSet) {
		l.problem(at, "diagram_icon_set", what, name, strings.Join(icons.Sets(), ", "))
		return ""
	}
	if err != nil {
		if failed := l.unplaced.Add(err); failed != nil && l.err == nil {
			l.err = failed
		}
		return ""
	}
	if icon == "" {
		if len(names) == 0 {
			l.problem(at, "diagram_icon_unknown", what, name)
		} else {
			l.problem(at, "diagram_icon_close_names", what, name, strings.Join(names, ", "))
		}
		return ""
	}
	return svgf(`<use href="#%v" x="%v" y="%v"/>`, l.iconDef(name, icon, size), x, y)
}

// iconDef returns the id of the <image> of href at size in <defs>, defining it on first use. It is
// found again by name and size, which are short; the id joins the
// diagram's id and a hash of the image, so that diagrams of one id rendered apart and pasted into
// one page still find their own icons.
func (l *layout) iconDef(name, href string, size float64) string {
	key := name + " " + formatNum(size)
	if id, ok := l.iconIDs[key]; ok {
		return id
	}
	sum := sha256.Sum256([]byte(href + " " + formatNum(size)))
	id := l.spec.ID + "_icon-" + hex.EncodeToString(sum[:6])
	l.iconIDs[key] = id
	l.iconDefs = append(l.iconDefs, svgf(`<image id="%v" href="%v" width="%v" height="%v"/>`, id, href, size, size))
	return id
}

// label places text at x, y as diagram.mjs does: 18px tall, with 5px around it. It returns the
// label's index in labels.
func (l *layout) label(at yamlpos.Pos, text string, x, y float64, anchor string, plain, strong bool) int {
	size, cls := float64(sizeSub), "label"
	if strong {
		size, cls = sizeMain, "label strong"
	}
	w := textWidth(text, size, false) + 10
	x0 := x - w/2
	switch anchor {
	case "start":
		x0 = x - 5
	case "end":
		x0 = x - w + 5
	}
	l.labels = append(l.labels, rect{x0, y - 9, w, 18, usererr.New("diagram_what_label", text), at})
	bg := markup("")
	if !plain {
		bg = svgf(`<rect x="%v" y="%v" width="%v" height="18" class="label-bg"/>`, x0, y-9, w)
	}
	l.labelSVG = append(l.labelSVG,
		svgf(`%v<text x="%v" y="%v" text-anchor="%v" class="%v">%v</text>`, bg, x, y, anchor, cls, text))
	return len(l.labels) - 1
}

func (l *layout) addBox(b Box) {
	what := usererr.New("diagram_what_box", b.Rows[0].Text)
	room := b.W
	if b.Shape == "hex" {
		room -= 36
	}
	room -= pad * 2
	var text strings.Builder
	for i, r := range b.Rows {
		size := float64(sizeMain)
		if strings.HasPrefix(r.Style, "sub") {
			size = sizeSub
		}
		l.fit(b.Pos, what, r.Text, size, strings.HasSuffix(r.Style, "code"), room)
		y := b.Y + b.H/2 + (float64(i)-float64(len(b.Rows)-1)/2)*lineGap
		text.WriteString(string(
			svgf(`<text x="%v" y="%v" text-anchor="middle" class="%v">%v</text>`, b.X+b.W/2, y, r.Style, r.Text)))
	}
	if need := len(b.Rows)*lineGap + 12; float64(need) > b.H {
		l.problem(b.Pos, "diagram_too_short", what, need, formatNum(b.H))
	}
	l.boxes = append(l.boxes, rect{b.X, b.Y, b.W, b.H, what, b.Pos})
	l.boxSVG = append(l.boxSVG, svgf(`<g class="%v">%v%v</g>`, "box"+tone(b.Tone), shape(b), markup(text.String())))
}

// shape draws a box: a rounded rectangle (rect), a pill, a hexagon for a decision (hex) or a state.
func shape(b Box) markup {
	x, y, w, h := b.X, b.Y, b.W, b.H
	r := float64(cornerR)
	switch b.Shape {
	case "hex":
		return svgf(`<path d="M%v,%v H%v L%v,%v L%v,%v H%v L%v,%v Z"/>`,
			x+18, y, x+w-18, x+w, y+h/2, x+w-18, y+h, x+18, x, y+h/2)
	case "pill":
		r = h / 2
	case "state":
		r = math.Min(26, h/2)
	}
	return svgf(`<rect x="%v" y="%v" width="%v" height="%v" rx="%v"/>`, x, y, w, h, r)
}

func (l *layout) addTable(t Table) {
	what := usererr.New("diagram_what_table", t.Title)
	l.fit(t.Pos, what, t.Title, sizeMain, false, t.W-pad*2)
	x, y, w, r := t.X, t.Y, t.W, float64(cornerR)
	var body strings.Builder
	top, h := y+tableHead, float64(tableHead)
	for _, row := range t.Rows {
		if row.Sep {
			body.WriteString(string(
				line(x, top+tableSep/2.0, x+w, top+tableSep/2.0, "sep")))
			top += tableSep
			h += tableSep
			continue
		}
		l.fit(t.Pos, what, row.Name+"  "+row.Type, sizeSub, true, w-24)
		body.WriteString(string(
			svgf(`<text x="%v" y="%v" text-anchor="start" class="cell code">%v</text>`, x+12, top+tableRow/2, row.Name)))
		if row.Type != "" {
			body.WriteString(string(
				svgf(`<text x="%v" y="%v" text-anchor="end" class="type">%v</text>`, x+w-12, top+tableRow/2, row.Type)))
		}
		top += tableRow
		h += tableRow
	}
	if len(t.Rows) > 0 {
		h += 6
	}
	l.boxes = append(l.boxes, rect{x, y, w, h, what, t.Pos})
	head := svgf(`<path d="M%v,%v V%v Q%v,%v %v,%v H%v Q%v,%v %v,%v V%v Z" class="head"/>`,
		x, y+tableHead, y+r, x, y, x+r, y, x+w-r, x+w, y, x+w, y+r, y+tableHead)
	sep := markup("")
	if len(t.Rows) > 0 {
		sep = line(x, y+tableHead, x+w, y+tableHead, "sep")
	}
	l.boxSVG = append(l.boxSVG, svgf(`<g class="%v"><rect x="%v" y="%v" width="%v" height="%v" rx="%v" class="bg"/>%v%v`+
		`<text x="%v" y="%v" text-anchor="middle" class="main">%v</text>%v`+
		`<rect x="%v" y="%v" width="%v" height="%v" rx="%v" class="outline"/></g>`,
		"table"+tone(t.Tone), x, y, w, h, r, head, sep, x+w/2, y+tableHead/2, t.Title, markup(body.String()), x, y, w, h, r))
}

func (l *layout) fit(at yamlpos.Pos, what *usererr.Error, text string, size float64, code bool, room float64) {
	if tw := textWidth(text, size, code); tw > room {
		l.problem(at, "diagram_too_narrow", what, text, int(math.Ceil(tw-room)))
	}
}

func (l *layout) addEdge(e Edge) {
	cls := "edge"
	if e.Dashed {
		cls += " dashed"
	}
	if e.Bold {
		cls += " bold"
	}
	cls += tone(e.Tone)
	l.edgeSVG = append(l.edgeSVG,
		svgf(`<path d="%v" class="%v"%v%v/>`, e.D, cls, l.marker(e.Start, "start"), l.marker(e.End, "end")))
	p := path{edge: e, own: -1}
	if e.Label != "" {
		p.own = l.label(e.Pos, e.Label, e.LabelAt[0], e.LabelAt[1], "middle", false, false)
	}
	l.paths = append(l.paths, p)
	if len(e.Segs) > 0 {
		l.strokes = append(l.strokes, span(e.Segs, usererr.New("diagram_what_edge", e.D), e.Pos))
	}
}

// span is the rect that straight runs take, for the canvas check alone: a line, a lifeline or an edge
// may cross parts, but not leave the canvas.
func span(runs [][4]float64, what *usererr.Error, at yamlpos.Pos) rect {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, s := range runs {
		x0, x1 = min(x0, s[0], s[2]), max(x1, s[0], s[2])
		y0, y1 = min(y0, s[1], s[3]), max(y1, s[1], s[3])
	}
	return rect{x0, y0, x1 - x0, y1 - y0, what, at}
}

// marker is the attribute that puts the marker kind at where (start or end) of an edge.
func (l *layout) marker(kind, where string) markup {
	if kind == "none" {
		return ""
	}
	if !slices.Contains(l.markers, kind) {
		l.markers = append(l.markers, kind)
	}
	return svgf(` marker-%v="url(#%v_%v)"`, where, l.spec.ID, kind)
}

func (l *layout) addBar(b Bar) {
	name := b.Label
	if name == "" {
		name = formatNum(b.X1) + "-" + formatNum(b.X2)
	}
	w := math.Max(b.X2-b.X1, 4)
	l.boxes = append(l.boxes, rect{b.X1, b.Y - 7, w, 14, usererr.New("diagram_what_bar", name), b.Pos})
	l.boxSVG = append(l.boxSVG,
		svgf(`<rect x="%v" y="%v" width="%v" height="14" rx="3" class="%v"/>`, b.X1, b.Y-7, w, "bar"+tone(b.Tone)))
	if b.Label != "" {
		l.label(b.Pos, b.Label, b.X1+w+10, b.Y, "start", false, false)
	}
}

func (l *layout) addAxis(a Axis) {
	runs := [][4]float64{{a.X1, a.Y, a.X2, a.Y}}
	l.edgeSVG = append(l.edgeSVG, line(a.X1, a.Y, a.X2, a.Y, "axis"))
	for _, t := range a.Ticks {
		if a.Grid != nil {
			runs = append(runs, [4]float64{t.X, *a.Grid, t.X, a.Y})
			l.grids = append(l.grids, line(t.X, *a.Grid, t.X, a.Y, "grid"))
		}
		runs = append(runs, [4]float64{t.X, a.Y, t.X, a.Y + 5})
		l.edgeSVG = append(l.edgeSVG, line(t.X, a.Y, t.X, a.Y+5, "axis"))
		l.label(a.Pos, t.Text, t.X, a.Y+17, "middle", true, false)
	}
	what := usererr.New("diagram_what_axis", formatNum(a.X1), formatNum(a.X2), formatNum(a.Y))
	l.strokes = append(l.strokes, span(runs, what, a.Pos))
}

// line draws a straight line of the class cls.
func line(x1, y1, x2, y2 float64, cls string) markup {
	return svgf(`<line x1="%v" y1="%v" x2="%v" y2="%v" class="%v"/>`, x1, y1, x2, y2, cls)
}

// textWidth estimates the width of s: a full-width character (above U+2E80) is size wide, any other
// 0.6 of it, or 0.62 in code.
// LIMIT: an estimate, as diagram.mjs makes it. Measure the text instead if a font change makes it miss.
func textWidth(s string, size float64, code bool) float64 {
	narrow := size * 0.6
	if code {
		narrow = size * 0.62
	}
	w := 0.0
	for _, c := range s {
		if c > 0x2e80 {
			w += size
		} else {
			w += narrow
		}
	}
	return w
}
