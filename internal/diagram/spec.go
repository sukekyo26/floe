// Package diagram reads a diagram spec: a YAML mapping of a canvas and the parts placed on it at
// absolute coordinates. A mistake in the spec's shape stops at the first one, with its line and
// column.
package diagram

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/sukekyo26/floe/internal/usererr"
)

// Spec is one diagram, read from File where Pos is.
type Spec struct {
	Pos
	File          string
	ID            string
	Width, Height float64
	Title         string
	Parts         []Part
}

// Part is one of Frame, Box, Table, Icon, Edge, Dot, Bar, Axis, Circle, Label, Line and Raw.
type Part interface {
	At() Pos
}

// Frame is a box around other parts, with its label inside the top left corner.
type Frame struct {
	Pos
	X, Y, W, H float64
	Label      string
	Accent     bool
	Icon       string // an icon in the corner, the AWS group frame style
	Square     bool
}

// Box is a box of centred rows of text.
type Box struct {
	Pos
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
	Pos
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
	Pos
	X, Y  float64
	Name  string
	Label string
	Size  float64
}

// Edge is a right-angled line through the absolute M, H, V and L commands of D.
type Edge struct {
	Pos
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
	Pos
	CX, CY, R float64
	Tone      string
	Hollow    bool
}

// Bar is a horizontal span (a timeline period), with its label to the right.
type Bar struct {
	Pos
	X1, X2, Y float64
	Tone      string
	Label     string
}

// Axis is a time axis with labelled ticks, and grid lines up to Grid when it is set.
type Axis struct {
	Pos
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
	Pos
	CX, CY, R float64
	Tone      string
}

// Label is text; Anchor is middle, start or end.
type Label struct {
	Pos
	Text   string
	X, Y   float64
	Anchor string
	Plain  bool // no background, for text on a coloured area
	Strong bool
}

// Line is a line without arrows; Kind is line, lifeline or divider.
type Line struct {
	Pos
	X1, Y1, X2, Y2 float64
	Kind           string
}

// Raw is SVG that no part can draw, written out as it is and left out of the checks.
type Raw struct {
	Pos
	SVG string
}

var (
	idRe        = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	tones       = []string{"accent", "good", "warn", "bad"}
	shapes      = []string{"rect", "pill", "hex", "state"}
	rowStyles   = []string{"main", "sub", "main code", "sub code"}
	textAnchors = []string{"middle", "start", "end"}
	lineKinds   = []string{"line", "lifeline", "divider"}
	// Tokens of an edge's d as diagram.mjs splits them; anything else (commas, spaces) separates.
	pathTokenRe = regexp.MustCompile(`[A-Za-z]|-?(?:\d+\.?\d*|\.\d+)(?:e[-+]?\d+)?`)
)

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
	doc, err := parser.ParseBytes(src, 0)
	if err != nil {
		var se *yaml.SyntaxError
		if errors.As(err, &se) && se.Token != nil {
			at := Pos{Line: se.Token.Position.Line, Column: se.Token.Position.Column}
			return nil, &Error{File: file, Pos: at, Err: usererr.New("diagram_yaml", se.Message)}
		}
		return nil, &Error{File: file, Pos: Pos{Line: 1, Column: 1}, Err: usererr.New("diagram_yaml", err.Error())}
	}
	if len(doc.Docs) > 1 {
		at := doc.Docs[1].Start.Position // the --- that starts the second document
		return nil, &Error{File: file, Pos: Pos{Line: at.Line, Column: at.Column}, Err: usererr.New("diagram_documents")}
	}
	if len(doc.Docs) == 0 || doc.Docs[0].Body == nil {
		return nil, &Error{File: file, Pos: Pos{Line: 1, Column: 1}, Err: usererr.New("diagram_empty")}
	}
	return FromNode(file, doc.Docs[0].Body)
}

// FromNode reads a spec from a node of YAML already parsed, such as a diagram inside a data file,
// so that its errors point at the lines and columns of that file.
func FromNode(file string, n ast.Node) (*Spec, error) {
	a := &anchors{found: nil}
	ast.Walk(a, n)
	if a.found != nil {
		return nil, &Error{File: file, Pos: posOf(a.found), Err: usererr.New("diagram_no_anchors")}
	}
	f, err := newFields(file, "diagram", posOf(n), n)
	if err != nil {
		return nil, err
	}
	spec := &Spec{
		Pos: f.at, File: file, ID: f.str("id"), Width: f.size("width"), Height: f.size("height"),
		Title: f.optStr("title"), Parts: nil,
	}
	items := f.list("parts")
	if f.lookup("parts") == nil {
		f.missing("parts")
	}
	if err := f.done(); err != nil {
		return nil, err
	}
	if !idRe.MatchString(spec.ID) {
		return nil, f.errAt(posOf(f.lookup("id")), "diagram_bad_id", spec.ID)
	}
	if strings.TrimSpace(spec.Title) == "" {
		at := f.at
		if n := f.lookup("title"); n != nil {
			at = posOf(n)
		}
		return nil, f.errAt(at, "diagram_no_title", spec.ID)
	}
	for _, item := range items {
		p, err := decodePart(file, item)
		if err != nil {
			return nil, err
		}
		spec.Parts = append(spec.Parts, p)
	}
	return spec, nil
}

// decodePart reads one part: a mapping of one key, the kind, holding the part's keys.
func decodePart(file string, n ast.Node) (Part, error) {
	m, ok := n.(*ast.MappingNode)
	if !ok || len(m.Values) != 1 {
		return nil, &Error{File: file, Pos: posOf(n), Err: usererr.New("diagram_part_shape")}
	}
	kind := m.Values[0].Key.String()
	i := slices.IndexFunc(decoders, func(d decoder) bool { return d.kind == kind })
	if i < 0 {
		kinds := make([]string, 0, len(decoders))
		for _, d := range decoders {
			kinds = append(kinds, d.kind)
		}
		return nil, &Error{
			File: file, Pos: posOf(m.Values[0].Key),
			Err: usererr.New("diagram_unknown_part", kind, strings.Join(kinds, ", ")),
		}
	}
	f, err := newFields(file, kind, posOf(m.Values[0].Key), m.Values[0].Value)
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
				return nil
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
			return nil
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
	if f.err != nil {
		return e
	}
	segs, err := segments(e.D)
	if err != nil {
		f.fail(f.lookup("d"), err.Key, err.Args...)
		return e
	}
	e.Segs = segs
	if e.Label != "" && e.LabelAt == nil {
		f.fail(f.lookup("label"), "diagram_edge_no_at", e.D)
	}
	return e
}

// segments splits d into straight runs, as diagram.mjs does: only the absolute M, H, V and L.
func segments(d string) ([][4]float64, *usererr.Error) {
	tokens := pathTokenRe.FindAllString(d, -1)
	var segs [][4]float64
	var x, y float64
	started := false
	for i := 0; i < len(tokens); {
		cmd := tokens[i]
		i++
		num := func() (float64, *usererr.Error) {
			if i >= len(tokens) {
				return 0, usererr.New("diagram_edge_number", d, cmd)
			}
			v, err := strconv.ParseFloat(tokens[i], 64)
			i++
			if err != nil {
				return 0, usererr.New("diagram_edge_number", d, cmd)
			}
			return v, nil
		}
		nx, ny := x, y
		var err *usererr.Error
		switch cmd {
		case "M", "L":
			if nx, err = num(); err == nil {
				ny, err = num()
			}
		case "H":
			nx, err = num()
		case "V":
			ny, err = num()
		default:
			return nil, usererr.New("diagram_edge_command", d, cmd)
		}
		if err != nil {
			return nil, err
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
			return nil
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
