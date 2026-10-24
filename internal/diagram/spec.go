// Package diagram reads a diagram spec: a YAML mapping of a canvas and the parts placed on it at
// absolute coordinates. Every mistake in the spec's shape is told at once, one to a value, with its line
// and column.
package diagram

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Spec is one diagram, read from File where Pos is.
type Spec struct {
	yamlpos.Pos
	File          string
	ID            string
	Width, Height float64
	Title         string
	Parts         []Part
}

// Part is one of Frame, Box, Table, Icon, Edge, Dot, Bar, Axis, Circle, Label, Line and Raw.
type Part interface {
	At() yamlpos.Pos
}

// Frame is a box around other parts, with its label inside the top left corner.
type Frame struct {
	yamlpos.Pos
	X, Y, W, H float64
	Label      string
	Accent     bool
	Icon       string // an icon in the corner, the AWS group frame style
	Square     bool
}

// Box is a box of centred rows of text.
type Box struct {
	yamlpos.Pos
	X, Y, W, H float64
	Rows       []Row
	Tone       string
	Shape      string
}

// Row is a line of text in a box; Style is main, sub, main code or sub code.
type Row struct {
	Text, Style string
}

// Table is a heading over rows of a name and a type (an ER entity, a class).
type Table struct {
	yamlpos.Pos
	X, Y, W float64
	Title   string
	Rows    []TableRow
	Tone    string
}

// TableRow is a name and a type, or a separator line.
type TableRow struct {
	Name, Type string
	Sep        bool
}

// Icon is an icon of an icon set (aws/…) with its top left at X, Y.
type Icon struct {
	yamlpos.Pos
	X, Y  float64
	Name  string
	Label string
	Size  float64
}

// Edge is a right-angled line through the absolute M, H, V and L commands of D.
type Edge struct {
	yamlpos.Pos
	D        string
	Segs     [][4]float64 // x1, y1, x2, y2 of each straight run of D
	Label    string
	LabelAt  *[2]float64 // where Label goes, from the key at
	Dashed   bool
	Bold     bool
	Tone     string
	Start    string // a marker name, or none
	End      string
	Crossing bool // allows this edge to cross another
}

// Dot is a circle that edges may reach the centre of (a commit, a milestone, a start state).
type Dot struct {
	yamlpos.Pos
	CX, CY, R float64
	Tone      string
	Hollow    bool
}

// Bar is a horizontal span (a timeline period), with its label to the right.
type Bar struct {
	yamlpos.Pos
	X1, X2, Y float64
	Tone      string
	Label     string
}

// Axis is a time axis with labelled ticks, and grid lines up to Grid when it is set.
type Axis struct {
	yamlpos.Pos
	X1, X2, Y float64
	Ticks     []Tick
	Grid      *float64
}

// Tick is a mark on an axis.
type Tick struct {
	X    float64
	Text string
}

// Circle is a translucent circle that may overlap others (a Venn diagram).
type Circle struct {
	yamlpos.Pos
	CX, CY, R float64
	Tone      string
}

// Label is text; Anchor is middle, start or end.
type Label struct {
	yamlpos.Pos
	Text   string
	X, Y   float64
	Anchor string
	Plain  bool // no background, for text on a coloured area
	Strong bool
}

// Line is a line without arrows; Kind is line, lifeline or divider.
type Line struct {
	yamlpos.Pos
	X1, Y1, X2, Y2 float64
	Kind           string
}

// Raw is SVG that no part can draw, written out as it is and left out of the checks.
type Raw struct {
	yamlpos.Pos
	SVG string
}

var (
	idRe        = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	tones       = []string{"accent", "good", "warn", "bad"}
	shapes      = []string{"rect", "pill", "hex", "state"}
	rowStyles   = []string{"main", "sub", "main code", "sub code"}
	textAnchors = []string{"middle", "start", "end"}
	lineKinds   = []string{"line", "lifeline", "divider"}
	// A number of SVG path data: a sign, digits with or without a point, and an exponent.
	pathNumberRe = regexp.MustCompile(`^[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?`)
)

// pathSpace is the white space of SVG path data.
const pathSpace = " \t\r\n\f"

type decoder struct {
	kind   string
	decode func(f *fields) Part
}

var decoders = []decoder{
	{"frame", decodeFrame},
	{"box", decodeBox},
	{"table", decodeTable},
	{"icon", decodeIcon},
	{"edge", decodeEdge},
	{"dot", decodeDot},
	{"bar", decodeBar},
	{"axis", decodeAxis},
	{"circle", decodeCircle},
	{"label", decodeLabel},
	{"line", decodeLine},
	{"raw", decodeRaw},
}

// Parse reads the spec in src; file names it in errors.
func Parse(file string, src []byte) (*Spec, error) {
	body, err := yamlpos.Parse(file, src, "diagram_documents")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return nil, yamlpos.New(file, yamlpos.Pos{Line: 1, Column: 1}, "diagram_empty")
	}
	if err != nil {
		return nil, err //nolint:wrapcheck // a mistake at its line and column
	}
	return FromNode(file, body)
}

// FromNode reads a spec from a node of YAML already parsed, such as a diagram inside a data file,
// so that its errors point at the lines and columns of that file.
func FromNode(file string, n ast.Node) (*Spec, error) {
	// Its shape is told with its anchors and tags, of what they hold: the wrappers are Anchors' to tell.
	// Nothing else is read: a value is not written where an alias's position points.
	var f *fields
	var shape error
	if u := yamlpos.Unwrap(n); yamlpos.Written(u, u != n) {
		f, shape = newFields(file, "diagram", yamlpos.Of(n), u)
	}
	if err := usererr.Collect(yamlpos.Anchors(file, n), shape); err != nil {
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns
	}
	spec := &Spec{
		Pos: f.at, File: file, ID: f.str("id"), Width: f.size("width"), Height: f.size("height"),
		Title: f.optStr("title"), Parts: nil,
	}
	items := f.list("parts")
	if f.lookup("parts") == nil {
		f.missing("parts")
	}
	// No title is told as a key not given is; one of spaces alone always. It names the diagram by its id,
	// once read as text, whether or not it can be one.
	noTitle, named := "diagram_no_title_unnamed", []any{}
	if f.ok("id") {
		noTitle, named = "diagram_no_title", []any{spec.ID}
		if !idRe.MatchString(spec.ID) {
			f.fail(f.lookup("id"), "diagram_bad_id", spec.ID)
		}
	}
	if n := f.lookup("title"); n == nil {
		f.absent(f.at, noTitle, named...)
	} else if strings.TrimSpace(spec.Title) == "" {
		f.fail(n, noTitle, named...)
	}
	var found usererr.List
	if failed := found.Add(f.done()); failed != nil {
		return nil, failed //nolint:wrapcheck // already says what failed
	}
	// Each part on its own: a part told wrong is left out, and the others read.
	for _, item := range items {
		p, err := decodePart(file, item)
		if failed := found.Add(err); failed != nil {
			return nil, failed //nolint:wrapcheck // already says what failed
		}
		if err == nil {
			spec.Parts = append(spec.Parts, p)
		}
	}
	if len(found) > 0 {
		return nil, found.Sorted()
	}
	return spec, nil
}

// decodePart reads one part: a mapping of one key, the kind, holding the part's keys.
func decodePart(file string, n ast.Node) (Part, error) {
	m, ok := n.(*ast.MappingNode)
	if !ok || len(m.Values) != 1 {
		return nil, yamlpos.New(file, yamlpos.Of(n), "diagram_part_shape")
	}
	kind := m.Values[0].Key.String()
	i := slices.IndexFunc(decoders, func(d decoder) bool { return d.kind == kind })
	if i < 0 {
		kinds := make([]string, 0, len(decoders))
		for _, d := range decoders {
			kinds = append(kinds, d.kind)
		}
		return nil, yamlpos.New(file, yamlpos.Of(m.Values[0].Key), "diagram_unknown_part", kind, strings.Join(kinds, ", "))
	}
	f, err := newFields(file, kind, yamlpos.Of(m.Values[0].Key), m.Values[0].Value)
	if err != nil {
		return nil, err
	}
	p := decoders[i].decode(f)
	return p, f.done()
}

func decodeFrame(f *fields) Part {
	return Frame{
		Pos: f.at, X: f.num("x"), Y: f.num("y"), W: f.size("w"), H: f.size("h"),
		Label: f.optStr("label"), Accent: f.flag("accent"), Icon: f.optStr("icon"), Square: f.flag("square"),
	}
}

func decodeBox(f *fields) Part {
	return Box{
		Pos: f.at, X: f.num("x"), Y: f.num("y"), W: f.size("w"), H: f.size("h"),
		Rows: boxRows(f), Tone: f.oneOf("tone", "", tones...), Shape: f.oneOf("shape", "rect", shapes...),
	}
}

// boxRows reads rows: one text, or a list of a text or [text, style]. Without a style the first row
// is main and the rest are sub.
func boxRows(f *fields) []Row {
	n := f.get("rows")
	if n == nil {
		f.missing("rows")
		return nil
	}
	items := []ast.Node{n}
	if s, ok := n.(*ast.SequenceNode); ok {
		items = s.Values
	}
	if len(items) == 0 {
		f.fail(n, "diagram_empty_list", f.kind, "rows")
	}
	rows := make([]Row, 0, len(items))
	for i, item := range items {
		style := "main"
		if i > 0 {
			style = "sub"
		}
		if s, ok := item.(*ast.SequenceNode); ok {
			if len(s.Values) != 2 {
				f.fail(item, "diagram_box_row", f.kind)
				continue
			}
			item = s.Values[0]
			style = f.text("rows", s.Values[1])
			f.allow(s.Values[1], "row style", style, rowStyles)
		}
		rows = append(rows, Row{Text: f.text("rows", item), Style: style})
	}
	return rows
}

func decodeTable(f *fields) Part {
	return Table{
		Pos: f.at, X: f.num("x"), Y: f.num("y"), W: f.size("w"), Title: f.str("title"),
		Rows: tableRows(f), Tone: f.oneOf("tone", "", tones...),
	}
}

// tableRows reads rows: each a name, [name] or [name, type], or --- for a separator.
func tableRows(f *fields) []TableRow {
	items := f.list("rows")
	rows := make([]TableRow, 0, len(items))
	for _, item := range items {
		row := TableRow{Name: "", Type: "", Sep: false}
		s, ok := item.(*ast.SequenceNode)
		switch {
		case !ok:
			row.Name = f.text("rows", item)
			row.Sep = row.Name == "---"
		case len(s.Values) == 1 || len(s.Values) == 2:
			row.Name = f.text("rows", s.Values[0])
			if len(s.Values) == 2 {
				row.Type = f.text("rows", s.Values[1])
			}
		default:
			f.fail(item, "diagram_table_row", f.kind)
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func decodeIcon(f *fields) Part {
	return Icon{
		Pos: f.at, X: f.num("x"), Y: f.num("y"), Name: f.str("name"),
		Label: f.optStr("label"), Size: f.optSize("size", 48),
	}
}

func decodeEdge(f *fields) Part {
	e := Edge{
		Pos: f.at, D: f.str("d"), Segs: nil, Label: f.optStr("label"), LabelAt: f.pair("at"),
		Dashed: f.flag("dashed"), Bold: f.flag("bold"), Tone: f.oneOf("tone", "", tones...),
		Start: f.oneOf("start", "none", markerKinds...), End: f.oneOf("end", "arrow", markerKinds...),
		Crossing: f.flag("crossing"),
	}
	// Read of d, which names the edge in the mistakes.
	if !f.ok("d") {
		return e
	}
	segs, err := segments(e.D)
	if err != nil {
		f.fail(f.lookup("d"), err.Key, err.Args...)
	}
	e.Segs = segs
	// An at given wrong is told as such.
	if e.Label != "" && f.lookup("at") == nil {
		f.absent(yamlpos.Of(f.lookup("label")), "diagram_edge_no_at", e.D)
	}
	return e
}

// segments splits d into straight runs. It reads the grammar of SVG path data for the absolute M, H, V
// and L only: a browser stops drawing at the first character the grammar does not allow, and the
// checks must read only what is drawn.
func segments(d string) ([][4]float64, *usererr.Error) {
	var segs [][4]float64
	var x, y float64
	started := false
	rest := strings.TrimLeft(d, pathSpace)
	for rest != "" {
		r, size := utf8.DecodeRuneInString(rest)
		cmd := string(r)
		switch {
		case strings.ContainsRune("MHVL", r):
		case r < utf8.RuneSelf && unicode.IsLetter(r):
			return nil, usererr.New("diagram_edge_command", d, cmd)
		default:
			return nil, usererr.New("diagram_edge_char", d, cmd)
		}
		n := 2
		if cmd == "H" || cmd == "V" {
			n = 1
		}
		v, after, ok := pathNumbers(rest[size:], n)
		if !ok {
			return nil, usererr.New("diagram_edge_number", d, cmd)
		}
		rest = after
		nx, ny := v[0], v[1]
		switch cmd {
		case "H":
			ny = y
		case "V":
			nx, ny = x, v[0]
		}
		if cmd != "M" {
			if !started {
				return nil, usererr.New("diagram_edge_start", d)
			}
			segs = append(segs, [4]float64{x, y, nx, ny})
		}
		x, y, started = nx, ny, true
	}
	return segs, nil
}

// pathNumbers reads the n numbers after a command, separated by a comma, spaces or both, and the
// spaces after them.
func pathNumbers(s string, n int) (v [2]float64, rest string, ok bool) {
	rest = strings.TrimLeft(s, pathSpace)
	for k := range n {
		if k > 0 {
			rest = strings.TrimLeft(strings.TrimPrefix(strings.TrimLeft(rest, pathSpace), ","), pathSpace)
		}
		m := pathNumberRe.FindString(rest)
		f, err := strconv.ParseFloat(m, 64)
		if m == "" || err != nil {
			return v, rest, false
		}
		v[k], rest = f, rest[len(m):]
	}
	return v, strings.TrimLeft(rest, pathSpace), true
}

func decodeDot(f *fields) Part {
	return Dot{
		Pos: f.at, CX: f.num("cx"), CY: f.num("cy"), R: f.optSize("r", 7),
		Tone: f.oneOf("tone", "", tones...), Hollow: f.flag("hollow"),
	}
}

func decodeBar(f *fields) Part {
	return Bar{
		Pos: f.at, X1: f.num("x1"), X2: f.num("x2"), Y: f.num("y"),
		Tone: f.oneOf("tone", "", tones...), Label: f.optStr("label"),
	}
}

func decodeAxis(f *fields) Part {
	return Axis{
		Pos: f.at, X1: f.num("x1"), X2: f.num("x2"), Y: f.num("y"),
		Ticks: ticks(f), Grid: f.optNumPtr("grid"),
	}
}

// ticks reads ticks: each [x, text].
func ticks(f *fields) []Tick {
	items := f.list("ticks")
	if f.lookup("ticks") == nil {
		f.missing("ticks")
	}
	out := make([]Tick, 0, len(items))
	for _, item := range items {
		pair, ok := item.(*ast.SequenceNode)
		if !ok || len(pair.Values) != 2 {
			f.fail(item, "diagram_tick", f.kind)
			continue
		}
		out = append(out, Tick{X: f.number("ticks", pair.Values[0]), Text: f.text("ticks", pair.Values[1])})
	}
	return out
}

func decodeCircle(f *fields) Part {
	return Circle{
		Pos: f.at, CX: f.num("cx"), CY: f.num("cy"), R: f.size("r"),
		Tone: f.oneOf("tone", "accent", tones...),
	}
}

func decodeLabel(f *fields) Part {
	return Label{
		Pos: f.at, Text: f.str("text"), X: f.num("x"), Y: f.num("y"),
		Anchor: f.oneOf("anchor", "middle", textAnchors...), Plain: f.flag("plain"), Strong: f.flag("strong"),
	}
}

func decodeLine(f *fields) Part {
	return Line{
		Pos: f.at, X1: f.num("x1"), Y1: f.num("y1"), X2: f.num("x2"), Y2: f.num("y2"),
		Kind: f.oneOf("kind", "line", lineKinds...),
	}
}

func decodeRaw(f *fields) Part {
	return Raw{Pos: f.at, SVG: f.str("svg")}
}
