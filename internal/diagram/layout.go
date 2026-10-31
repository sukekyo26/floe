package diagram

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Text sizes and box metrics.
const (
	sizeMain  = 13
	sizeSub   = 12
	pad       = 16 // a box's inner margin on each side
	hexInset  = 18 // how far a hexagon's points stand in
	lineGap   = 17 // between the rows of a box
	tableHead = 30
	rowHeight = 22
	tableSep  = 9
	cornerR   = 8 // the corner radius of a box and a table
	labelH    = 18
	// canvasPad is the room right of and below what is drawn on a canvas whose width or height is left out: for
	// text drawn wider than its estimate, and for an edge's marker past its end.
	canvasPad = 20
)

// rect is the area a part takes, with the words that name it and where it is written (a JSON pointer).
type rect struct {
	x, y, w, h float64
	what, at   string
}

func (a rect) overlaps(b rect) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

func (a rect) inside(b rect) bool {
	return a.x >= b.x && a.y >= b.y && a.x+a.w <= b.x+b.w && a.y+a.h <= b.y+b.h
}

// seg is a straight run of a line: x1, y1, x2, y2.
type seg [4]float64

// path is an edge laid out: its runs, and the index in labels of its label, which it may run through; -1 for none.
type path struct {
	segs     []seg
	what, at string
	own      int
	crossing bool
}

// target is what an edge's end may name: a part's area, a table's row, or a dot's centre.
type target struct {
	r    rect
	kind string // box, table, row, icon, frame, bar, dot, circle
}

// layout is a spec laid out once, for every language its text is written in: the areas its checks compare and
// the elements it draws, so that the checks and the drawing read the same geometry.
type layout struct {
	s        *spec
	opts     Options
	id       string
	problems usererr.List
	failure  error
	names    map[string]target
	circles  map[string]circle
	boxes    []rect // edges may not run through these
	marks    []rect // may not overlap, but edges may reach their centre (dots)
	frames   []rect
	areas    []rect // may overlap (Venn circles)
	labels   []rect
	heads    []rect // frame headings, which edges may not cross
	strokes  []rect // the spans of edges and lines, which must stay on the canvas
	paths    []path
	// The elements of the layers, drawn bottom to top; grid lines below every other edge, last first.
	areaEls, gridEls, edgeEls, boxEls, labelEls []elem
	markers                                     []string // the marker kinds the edges use, in the order of first use
	iconDefs                                    []iconDef
	iconIDs                                     map[string]string // by name and size
	stacks                                      map[string]stack  // the labels placed by each region of a Venn diagram
	// The canvas: its width and height as written, or for one left out, +Inf while the parts are added and then
	// what is drawn reaches, so that each check and the drawing read the canvas they are told of.
	w, h float64
	// Labels placed by their region that had none of it in the canvas, told sticking out by checkPlacement, once
	// the canvas is known; only what and at are kept.
	strays []rect
}

func (l *layout) problem(at, key string, args ...any) {
	l.problems = append(l.problems, usererr.NewIn("", at, key, args...))
}

func pointer(i int) string {
	return "#/parts/" + strconv.Itoa(i)
}

// newLayout lays out s: names first, as edges and labels name parts written after them.
func newLayout(s *spec, opts Options) *layout {
	l := &layout{
		s: s, opts: opts, id: "", problems: nil, failure: nil, names: map[string]target{}, circles: map[string]circle{},
		boxes: nil, marks: nil, frames: nil, areas: nil, labels: nil, heads: nil, strokes: nil, paths: nil,
		areaEls: nil, gridEls: nil, edgeEls: nil, boxEls: nil, labelEls: nil, markers: nil, iconDefs: nil,
		iconIDs: map[string]string{}, stacks: map[string]stack{}, w: written(s.Width), h: written(s.Height), strays: nil,
	}
	b, _ := json.Marshal(s) //nolint:errcheck,errchkjson // read from JSON, it writes as JSON
	sum := sha256.Sum256(b)
	l.id = "dg" + strconv.Itoa(opts.Seq) + "-" + hex.EncodeToString(sum[:3])
	items := expand(s.Parts)
	for _, it := range items {
		l.name(it.at, it.p)
	}
	for _, p := range s.Parts {
		if lb := p.Label; lb != nil && lb.In != nil && lb.X == nil {
			st := l.stacks[regionKey(lb.In)]
			st.labels = append(st.labels, lb)
			l.stacks[regionKey(lb.In)] = st
		}
	}
	for _, it := range items {
		l.add(it.at, it.p)
	}
	if s.Width == 0 {
		l.w = l.reach(func(r rect) float64 { return r.x + r.w }) + canvasPad
	}
	if s.Height == 0 {
		l.h = l.reach(func(r rect) float64 { return r.y + r.h }) + canvasPad
	}
	return l
}

// written is a canvas's size as written, or +Inf for one left out (0), so that nothing is held within it yet.
func written(size float64) float64 {
	if size == 0 {
		return math.Inf(1)
	}
	return size
}

// reach is the farthest end of what is drawn, told by end, and 0 when all of it is before 0.
// LIMIT: raw SVG is not measured (that would read SVG as a browser does), so it may be cut off right of or below
// what else is drawn; write width and height with it.
// LIMIT: text is as wide as its estimate; text drawn over canvasPad wider (a run of bold Ws) at the end is cut off.
// LIMIT: an end that is not finite (a circle of radius 1e308) makes the canvas +Inf, which no check refuses, as
// with place's 1e299; refuse it if diagrams that large come to matter.
func (l *layout) reach(end func(rect) float64) float64 {
	far := 0.0
	for _, r := range slices.Concat(l.boxes, l.marks, l.labels, l.frames, l.heads, l.areas, l.strokes) {
		far = max(far, end(r))
	}
	return far
}

// name keeps the name of the part at at, refusing one used before.
func (l *layout) name(at string, p part) {
	keep := func(name string, t target) bool {
		if name == "" {
			return false
		}
		if _, ok := l.names[name]; ok {
			l.problem(at, "diagram_name_used", name)
			return false
		}
		l.names[name] = t
		return true
	}
	switch {
	case p.Box != nil:
		keep(p.Box.Name, target{r: boxRect(p.Box, at), kind: "box"})
	case p.Table != nil:
		r, rows := tableRects(p.Table, at)
		keep(p.Table.Name, target{r: r, kind: "table"})
		for j, row := range p.Table.Rows {
			keep(row.Name, target{r: rows[j], kind: "row"})
		}
	case p.Icon != nil:
		size := iconSize(p.Icon)
		keep(p.Icon.Name, target{r: rect{p.Icon.X, p.Icon.Y, size, size, "", at}, kind: "icon"})
	case p.Frame != nil:
		keep(p.Frame.Name, target{r: rect{p.Frame.X, p.Frame.Y, p.Frame.W, p.Frame.H, "", at}, kind: "frame"})
	case p.Bar != nil:
		keep(p.Bar.Name, target{r: barRect(p.Bar, at), kind: "bar"})
	case p.Dot != nil:
		r := dotR(p.Dot)
		keep(p.Dot.Name, target{r: rect{p.Dot.CX - r, p.Dot.CY - r, r * 2, r * 2, "", at}, kind: "dot"})
	case p.Circle != nil:
		// No area: no edge ends at it.
		if keep(p.Circle.Name, target{r: rect{x: 0, y: 0, w: 0, h: 0, what: "", at: at}, kind: "circle"}) {
			l.circles[p.Circle.Name] = *p.Circle
		}
	}
}

func (l *layout) add(at string, p part) {
	switch {
	case p.Frame != nil:
		l.addFrame(p.Frame, at)
	case p.Icon != nil:
		l.addIcon(p.Icon, at)
	case p.Box != nil:
		l.addBox(p.Box, at)
	case p.Table != nil:
		l.addTable(p.Table, at)
	case p.Edge != nil:
		l.addEdge(p.Edge, at)
	case p.Dot != nil:
		l.addDot(p.Dot, at)
	case p.Bar != nil:
		l.addBar(p.Bar, at)
	case p.Axis != nil:
		l.addAxis(p.Axis, at)
	case p.Circle != nil:
		l.addCircle(p.Circle, at)
	case p.Label != nil:
		l.addLabel(p.Label, at)
	case p.Line != nil:
		n := p.Line
		who := fmt.Sprintf("line %s,%s %s,%s", num(n.X1), num(n.Y1), num(n.X2), num(n.Y2))
		l.strokes = append(l.strokes, span([]seg{{n.X1, n.Y1, n.X2, n.Y2}}, who, at))
		l.edgeEls = append(l.edgeEls, lineEl(n.X1, n.Y1, n.X2, n.Y2, cmpOr(n.Kind, "line")))
	case p.Raw != nil:
		l.addRaw(p.Raw, at)
	}
}

func (l *layout) addDot(d *dot, at string) {
	r := dotR(d)
	l.marks = append(l.marks, rect{d.CX - r, d.CY - r, r * 2, r * 2, fmt.Sprintf("dot %s,%s", num(d.CX), num(d.CY)), at})
	cls := "dot" + tone(d.Tone)
	if d.Hollow {
		cls += " hollow"
	}
	l.boxEls = append(l.boxEls, elem{Kind: "circle", X: d.CX, Y: d.CY, R: r, Class: cls})
}

func (l *layout) addCircle(c *circle, at string) {
	who := fmt.Sprintf("circle %s,%s", num(c.CX), num(c.CY))
	if c.Name != "" {
		who = "circle " + strconv.Quote(c.Name)
	}
	l.areas = append(l.areas, rect{c.CX - c.R, c.CY - c.R, c.R * 2, c.R * 2, who, at})
	l.areaEls = append(l.areaEls, elem{Kind: "circle", X: c.CX, Y: c.CY, R: c.R, Class: "venn" + toneOr(c.Tone, "accent")})
}

func (l *layout) addRaw(r *raw, at string) {
	svg, err := checkRaw(r.SVG)
	var bad rawElements
	switch {
	case errors.As(err, &bad):
		// LIMIT: told at the start of the value, as x/net/html keeps no place of an element; tell its line in a
		// block if long raw SVG comes to need it.
		for _, name := range bad {
			l.problem(at+"/raw/svg", "diagram_raw_element", name)
		}
	case err != nil:
		l.failure = err
	default:
		l.edgeEls = append(l.edgeEls, elem{Kind: "raw", Raw: svg})
	}
}

func tone(t string) string {
	if t == "" {
		return ""
	}
	return " " + t
}

func toneOr(t, def string) string {
	return tone(cmpOr(t, def))
}

func cmpOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// widest is the language of t whose text is widest, and its width: a full-width character (U+2E80 and above) is
// size wide, any other 0.6 of it, or 0.62 in code. Widths are counted in fiftieths of size, so that text as wide
// compares equal: English is kept against text only as wide, and of the others the first by tag.
// LIMIT: an estimate of no font, as floe draws with the system's. Measure the text if one comes to miss.
func widest(t Text, size float64, code bool) (string, float64) {
	narrow := 30
	if code {
		narrow = 31
	}
	units := func(s string) int {
		n := 0
		for _, c := range s {
			if c >= 0x2e80 {
				n += 50
			} else {
				n += narrow
			}
		}
		return n
	}
	lang, most := "en", units(t["en"])
	for _, k := range slices.Sorted(maps.Keys(t)) {
		if n := units(t[k]); n > most {
			lang, most = k, n
		}
	}
	return lang, float64(most) * size / 50
}

func textWidth(t Text, size float64, code bool) float64 {
	_, w := widest(t, size, code)
	return w
}

// what names a part in a mistake: by its name, else by its English text.
func what(kind, name string, text Text) string {
	if name != "" {
		return kind + " " + strconv.Quote(name)
	}
	return kind + " " + strconv.Quote(text["en"])
}

// fit refuses text that is wider than room, telling the width to write: the widest of its languages.
func (l *layout) fit(at, who string, t Text, size float64, code bool, room, written float64) {
	if lang, tw := widest(t, size, code); tw > room {
		// LIMIT: the language's mark is made here, so no catalog picks its brackets; move it if one needs its own.
		in := ""
		if lang != "en" {
			in = " (" + lang + ")"
		}
		l.problem(at, "diagram_too_narrow", who, t[lang], in, num(math.Ceil(written+tw-room)))
	}
}

func ceil8(v float64) float64 {
	return math.Ceil(v/8) * 8
}

func rowSize(style string, first bool) (float64, bool) {
	if style == "" && first || strings.HasPrefix(style, "main") {
		return sizeMain, strings.HasSuffix(style, "code")
	}
	return sizeSub, strings.HasSuffix(style, "code")
}

// inset is how much narrower than a box its text must be: its margins, and a hexagon's points.
func inset(b *box) float64 {
	if b.Shape == "hex" {
		return pad*2 + hexInset*2
	}
	return pad * 2
}

func boxRect(b *box, at string) rect {
	w := b.W
	if w == 0 {
		widest := 0.0
		for i, r := range b.Rows {
			size, code := rowSize(r.Style, i == 0)
			widest = max(widest, textWidth(r.Text, size, code))
		}
		w = ceil8(widest + inset(b))
	}
	h := b.H
	if h == 0 {
		h = float64(len(b.Rows)*lineGap + 12)
	}
	x := b.X
	if b.Origin == "centre" {
		x -= w / 2
	}
	return rect{x, b.Y, w, h, what("box", b.Name, b.Rows[0].Text), at}
}

func (l *layout) addBox(b *box, at string) {
	r := boxRect(b, at)
	kids := make([]elem, 0, len(b.Rows))
	for i, row := range b.Rows {
		size, code := rowSize(row.Style, i == 0)
		if b.W != 0 {
			l.fit(at, r.what, row.Text, size, code, b.W-inset(b), b.W)
		}
		style := row.Style
		if style == "" {
			style = map[bool]string{true: "main", false: "sub"}[i == 0]
		}
		y := r.y + r.h/2 + (float64(i)-float64(len(b.Rows)-1)/2)*lineGap
		kids = append(kids, elem{Kind: "text", X: r.x + r.w/2, Y: y, Anchor: "middle", Class: style, Text: row.Text})
	}
	if need := len(b.Rows)*lineGap + 12; float64(need) > r.h {
		l.problem(at, "diagram_too_short", r.what, need, num(r.h))
	}
	l.boxes = append(l.boxes, r)
	kids = append([]elem{shape(r, b.Shape)}, kids...)
	l.boxEls = append(l.boxEls, elem{Kind: "g", Class: "box" + tone(b.Tone), Kids: kids})
}

// shape draws a box: a rounded rectangle (rect), a pill, a hexagon for a decision (hex) or a state.
func shape(r rect, kind string) elem {
	x, y, w, h := r.x, r.y, r.w, r.h
	radius := float64(cornerR)
	switch kind {
	case "hex":
		return elem{Kind: "path", D: fmt.Sprintf("M%s,%s H%s L%s,%s L%s,%s H%s L%s,%s Z",
			num(x+hexInset), num(y), num(x+w-hexInset), num(x+w), num(y+h/2), num(x+w-hexInset), num(y+h),
			num(x+hexInset), num(x), num(y+h/2))}
	case "pill":
		radius = h / 2
	case "state":
		radius = math.Min(26, h/2)
	}
	return elem{Kind: "rect", X: x, Y: y, W: w, H: h, RX: radius}
}

func tableWidth(t *table) float64 {
	if t.W != 0 {
		return t.W
	}
	widest := textWidth(t.Title, sizeMain, false) + pad*2
	for _, row := range t.Rows {
		widest = max(widest, textWidth(rowText(row), sizeSub, true)+24)
	}
	return ceil8(widest)
}

// rowText is row as drawn in each language it is written in; none for a separator.
func rowText(row tableRow) Text {
	if row.Sep {
		return Text{}
	}
	t := row.Cells[0]
	if len(row.Cells) == 2 {
		// Each language a cell is written in, both cells in it as drawn.
		t = Text{}
		for _, c := range row.Cells {
			for lang := range c {
				t[lang] = row.Cells[0].in(i18n.Lang(lang)) + "  " + row.Cells[1].in(i18n.Lang(lang))
			}
		}
	}
	return t
}

// tableRects are the area of t and of each of its rows.
func tableRects(t *table, at string) (rect, []rect) {
	w := tableWidth(t)
	x := t.X
	if t.Origin == "centre" {
		x -= w / 2
	}
	rows := make([]rect, len(t.Rows))
	top, h := t.Y+tableHead, float64(tableHead)
	for i, row := range t.Rows {
		if row.Sep {
			top += tableSep
			h += tableSep
			continue
		}
		rows[i] = rect{x, top, w, rowHeight, "row " + strconv.Quote(row.Name), at}
		top += rowHeight
		h += rowHeight
	}
	if len(t.Rows) > 0 {
		h += 6
	}
	return rect{x, t.Y, w, h, what("table", t.Name, t.Title), at}, rows
}

func (l *layout) addTable(t *table, at string) {
	r, rows := tableRects(t, at)
	if t.W != 0 {
		l.fit(at, r.what, t.Title, sizeMain, false, t.W-pad*2, t.W)
	}
	x, y, w, h, rad := r.x, r.y, r.w, r.h, float64(cornerR)
	kids := []elem{{Kind: "rect", X: x, Y: y, W: w, H: h, RX: rad, Class: "bg"}, {
		Kind: "path", Class: "head",
		D: fmt.Sprintf("M%s,%s V%s Q%s,%s %s,%s H%s Q%s,%s %s,%s V%s Z", num(x), num(y+tableHead), num(y+rad), num(x), num(y),
			num(x+rad), num(y), num(x+w-rad), num(x+w), num(y), num(x+w), num(y+rad), num(y+tableHead)),
	}}
	if len(t.Rows) > 0 {
		kids = append(kids, lineEl(x, y+tableHead, x+w, y+tableHead, "sep"))
	}
	kids = append(kids, elem{Kind: "text", X: x + w/2, Y: y + tableHead/2, Anchor: "middle", Class: "main", Text: t.Title})
	top := y + tableHead
	for i, row := range t.Rows {
		if row.Sep {
			kids = append(kids, lineEl(x, top+tableSep/2.0, x+w, top+tableSep/2.0, "sep"))
			top += tableSep
			continue
		}
		if t.W != 0 {
			l.fit(at, r.what, rowText(row), sizeSub, true, w-24, w)
		}
		rowY := rows[i].y + rowHeight/2
		kids = append(kids, elem{Kind: "text", X: x + 12, Y: rowY, Anchor: "start", Class: "cell code", Text: row.Cells[0]})
		if len(row.Cells) == 2 {
			kids = append(kids, elem{Kind: "text", X: x + w - 12, Y: rowY, Anchor: "end", Class: "type", Text: row.Cells[1]})
		}
		top += rowHeight
	}
	kids = append(kids, elem{Kind: "rect", X: x, Y: y, W: w, H: h, RX: rad, Class: "outline"})
	l.boxes = append(l.boxes, r)
	l.boxEls = append(l.boxEls, elem{Kind: "g", Class: "table" + tone(t.Tone), Kids: kids})
}

func iconSize(i *icon) float64 {
	if i.Size == 0 {
		return 48
	}
	return i.Size
}

func dotR(d *dot) float64 {
	if d.R == 0 {
		return 7
	}
	return d.R
}

func barRect(b *bar, at string) rect {
	name := b.Label["en"]
	if name == "" {
		name = num(b.X1) + "-" + num(b.X2)
	}
	w := math.Max(math.Abs(b.X2-b.X1), 4)
	return rect{math.Min(b.X1, b.X2), b.Y - 7, w, 14, what("bar", b.Name, Text{"en": name}), at}
}

// addFrame keeps the frame's heading too: inside the top left corner, or right of the corner icon.
func (l *layout) addFrame(f *frame, at string) {
	who := what("frame", f.Name, f.Label)
	l.frames = append(l.frames, rect{f.X, f.Y, f.W, f.H, who, at})
	hx, hy, size := f.X+12, f.Y+18, float64(sizeSub)
	if f.Icon != "" {
		hx, hy, size = f.X+40, f.Y+15, sizeMain
	}
	if f.Label != nil {
		l.heads = append(l.heads, rect{hx, hy - 9, textWidth(f.Label, size, false), labelH, "the heading of " + who, at})
		// The heading keeps the margin on its right that it has on its left.
		l.fit(at, who, f.Label, size, false, f.W-(hx-f.X)-12, f.W)
	}
	rx, cls := 10.0, "frame"
	if f.Icon != "" || f.Square {
		rx = 0
	}
	if f.Accent {
		cls += " accent"
	}
	kids := []elem{{Kind: "rect", X: f.X, Y: f.Y, W: f.W, H: f.H, RX: rx}}
	if f.Icon != "" {
		cls += " grouped"
		// The icon is a part like a box: nothing may overlap it or run through it. It is checked from the frame's
		// edge, as the 1px of it outside the frame only covers the frame's line (1.2 wide, centred on its edge).
		l.boxes = append(l.boxes, rect{f.X, f.Y, 31, 31, "the icon of " + who, at})
		if use, ok := l.image(at, f.Icon, f.X-1, f.Y-1, 32, "the icon of "+who); ok {
			kids = append(kids, use)
		}
	}
	if f.Label != nil {
		kids = append(kids, elem{Kind: "text", X: hx, Y: hy, Anchor: "start", Text: f.Label})
	}
	l.areaEls = append(l.areaEls, elem{Kind: "g", Class: cls, Kids: kids})
}

func (l *layout) addIcon(i *icon, at string) {
	size := iconSize(i)
	who := what("icon", i.Name, Text{"en": i.Icon})
	l.boxes = append(l.boxes, rect{i.X, i.Y, size, size, who, at})
	if use, ok := l.image(at, i.Icon, i.X, i.Y, size, who); ok {
		l.boxEls = append(l.boxEls, use)
	}
	if i.Label != nil {
		l.label(at, i.Label, i.X+size/2, i.Y+size+16, "middle", false, false)
	}
}

// iconDef is an icon at a size, defined once and placed with <use>: the official icons' ids collide between
// icons, so an icon is an <image>, not inline <svg>.
type iconDef struct {
	ID   string
	Href template.URL
	Size float64
}

// image places the icon of name. A name its set does not hold is a mistake of the spec; icons not fetched or
// not read are mistakes of their own place, and a failure that is no mistake stops the drawing.
func (l *layout) image(at, name string, x, y, size float64, who string) (elem, bool) {
	// An icon placed again is looked up once.
	key := name + " " + num(size)
	if id, ok := l.iconIDs[key]; ok {
		return elem{Kind: "use", Ref: id, X: x, Y: y}, true
	}
	b, err := l.opts.Icons.Lookup(name)
	switch {
	case errors.Is(err, icons.ErrNoSet):
		l.problem(at, "diagram_icon_set", who, name, strings.Join(icons.Sets(), ", "))
		return elem{}, false //nolint:exhaustruct // none
	case errors.Is(err, icons.ErrNoIcon):
		l.problem(at, "diagram_icon_unknown", who, name)
		return elem{}, false //nolint:exhaustruct // none
	case err != nil:
		if failed := l.problems.Add(err); failed != nil && l.failure == nil {
			l.failure = failed
		}
		return elem{}, false //nolint:exhaustruct // none
	}
	id := l.id + "_icon-" + strconv.Itoa(len(l.iconDefs))
	l.iconIDs[key] = id
	// A data URI of the SVG floe fetched itself (icons.Lookup reads only the set's own files).
	href := template.URL("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(b)) //nolint:gosec // see above
	l.iconDefs = append(l.iconDefs, iconDef{ID: id, Href: href, Size: size})
	return elem{Kind: "use", Ref: id, X: x, Y: y}, true
}

// label places text at x, y: 18px tall, with 5px around it. It returns the label's index in labels.
func (l *layout) label(at string, t Text, x, y float64, anchor string, plain, strong bool) int {
	cls := "label"
	if strong {
		cls = "label strong"
	}
	w := labelWidth(t, strong)
	x0 := x - w/2
	switch anchor {
	case "start":
		x0 = x - 5
	case "end":
		x0 = x - w + 5
	}
	l.labels = append(l.labels, rect{x0, y - 9, w, labelH, what("label", "", t), at})
	if !plain {
		l.labelEls = append(l.labelEls, elem{Kind: "rect", X: x0, Y: y - 9, W: w, H: labelH, Class: "label-bg"})
	}
	l.labelEls = append(l.labelEls, elem{Kind: "text", X: x, Y: y, Anchor: anchor, Class: cls, Text: t})
	return len(l.labels) - 1
}

// labelWidth is how wide a label of t is, with 5px around its text.
func labelWidth(t Text, strong bool) float64 {
	if strong {
		return textWidth(t, sizeMain, false) + 10
	}
	return textWidth(t, sizeSub, false) + 10
}

func (l *layout) addBar(b *bar, at string) {
	r := barRect(b, at)
	l.boxes = append(l.boxes, r)
	l.boxEls = append(l.boxEls, elem{Kind: "rect", X: r.x, Y: r.y, W: r.w, H: r.h, RX: 3, Class: "bar" + tone(b.Tone)})
	if b.Label != nil {
		l.label(at, b.Label, r.x+r.w+10, b.Y, "start", false, false)
	}
}

func (l *layout) addAxis(a *axis, at string) {
	runs := []seg{{a.X1, a.Y, a.X2, a.Y}}
	l.edgeEls = append(l.edgeEls, lineEl(a.X1, a.Y, a.X2, a.Y, "axis"))
	for _, t := range a.Ticks {
		if a.Grid != nil {
			runs = append(runs, seg{t.X, *a.Grid, t.X, a.Y})
			l.gridEls = append(l.gridEls, lineEl(t.X, *a.Grid, t.X, a.Y, "grid"))
		}
		runs = append(runs, seg{t.X, a.Y, t.X, a.Y + 5})
		l.edgeEls = append(l.edgeEls, lineEl(t.X, a.Y, t.X, a.Y+5, "axis"))
		l.label(at, t.Text, t.X, a.Y+17, "middle", true, false)
	}
	l.strokes = append(l.strokes, span(runs, fmt.Sprintf("axis %s-%s at %s", num(a.X1), num(a.X2), num(a.Y)), at))
}

func lineEl(x1, y1, x2, y2 float64, cls string) elem {
	return elem{Kind: "line", X: x1, Y: y1, X2: x2, Y2: y2, Class: cls}
}

// span is the rect that straight runs take, for the canvas check alone: a line, a lifeline or an edge may cross
// parts, but not leave the canvas.
func span(runs []seg, who, at string) rect {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, s := range runs {
		x0, x1 = min(x0, s[0], s[2]), max(x1, s[0], s[2])
		y0, y1 = min(y0, s[1], s[3]), max(y1, s[1], s[3])
	}
	return rect{x0, y0, x1 - x0, y1 - y0, who, at}
}

// num writes v for SVG and messages, to a hundredth: a sum of estimates (text widths) would show its float error.
func num(v float64) string {
	v = math.Round(v*100) / 100
	if v == 0 {
		return "0" // and -0
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// marker is an end of an edge, drawn with its right end on the end of the edge, turning round at the start.
type marker struct {
	Kind, ViewBox string
	Body          template.HTML
	W, H, RX, RY  float64
}

var markers = []marker{
	{"arrow", "0 0 10 10", `<path d="M0,0 L10,5 L0,10 z" class="mk-fill"/>`, 10, 10, 9, 5},
	{"triangle", "0 0 14 14", `<path d="M1,1 L13,7 L1,13 z" class="mk-hollow"/>`, 14, 14, 13, 7},
	{"diamond", "0 0 18 10", `<path d="M1,5 L9,1 L17,5 L9,9 z" class="mk-fill"/>`, 18, 10, 17, 5},
	{"odiamond", "0 0 18 10", `<path d="M1,5 L9,1 L17,5 L9,9 z" class="mk-hollow"/>`, 18, 10, 17, 5},
	{"one", "0 0 14 14", `<path d="M6,1 V13 M10,1 V13" class="mk-line"/>`, 14, 14, 14, 7},
	{"many", "0 0 14 14", `<path d="M0,7 L14,1 M0,7 L14,7 M0,7 L14,13" class="mk-line"/>`, 14, 14, 14, 7},
	{
		"zero-one", "0 0 22 14",
		`<circle cx="6" cy="7" r="4" class="mk-hollow"/><path d="M15,1 V13" class="mk-line"/>`, 22, 14, 22, 7,
	},
	{
		"zero-many", "0 0 24 14",
		`<circle cx="5" cy="7" r="4" class="mk-hollow"/><path d="M10,7 L24,1 M10,7 L24,7 M10,7 L24,13" class="mk-line"/>`,
		24, 14, 24, 7,
	},
}

// marker is the id of the marker kind, kept for the drawing, or "" for none.
func (l *layout) marker(kind string) string {
	if kind == "none" {
		return ""
	}
	if !slices.Contains(l.markers, kind) {
		l.markers = append(l.markers, kind)
	}
	return l.id + "_" + kind
}
