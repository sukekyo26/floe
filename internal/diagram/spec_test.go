package diagram_test

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
)

func ptr[T any](v T) *T { return &v }

func TestParseEveryPart(t *testing.T) {
	t.Parallel()

	src := `id: all-parts
width: 640
height: 400.5
title: 部品の一覧
parts:
  - frame: { x: 16, y: 16, w: 608, h: 368, label: VPC, icon: aws/vpc, square: true }
  - box:
      x: 40
      y: 72
      w: 200
      h: 56
      tone: accent
      shape: hex
      rows: [[ALB, main code], 公開の入口, [補足, sub]]
  - box: { x: 400, y: 72, w: 200, h: 56, rows: API }
  - table: { x: 40, y: 200, w: 200, title: users, rows: [[id, PK], ---, [name], email] }
  - icon: { x: 300, y: 200, name: aws/amazon-ec2, label: EC2 }
  - edge: { d: "M240,100 H400", label: HTTPS, at: [320, 100], start: one, end: none, crossing: true }
  - dot: { cx: 10, cy: 20 }
  - bar: { x1: 1.5, x2: 3, y: 4, tone: good, label: 2026 }
  - axis: { x1: 0, x2: 100, y: 390, ticks: [[0, 10/06], [50, true]], grid: 0 }
  - axis: { x1: 0, x2: 100, y: 390, ticks: [] }
  - circle: { cx: 100, cy: 100, r: 40 }
  - label: { text: 1.0, x: 5, y: 6, anchor: start, plain: true }
  - line: { x1: 0, y1: 0, x2: 10, y2: 0 }
  - raw:
      svg: |
        <rect class="band"/>
`
	got, err := diagram.Parse("all.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	at := func(line, col int) diagram.Pos { return diagram.Pos{Line: line, Column: col} }
	want := &diagram.Spec{
		Pos: at(1, 1), File: "all.yaml", ID: "all-parts", Width: 640, Height: 400.5, Title: "部品の一覧",
		Parts: []diagram.Part{
			diagram.Frame{Pos: at(6, 5), X: 16, Y: 16, W: 608, H: 368, Label: "VPC", Icon: "aws/vpc", Square: true},
			diagram.Box{
				Pos: at(7, 5), X: 40, Y: 72, W: 200, H: 56, Tone: "accent", Shape: "hex",
				Rows: []diagram.Row{{"ALB", "main code"}, {"公開の入口", "sub"}, {"補足", "sub"}},
			},
			diagram.Box{Pos: at(15, 5), X: 400, Y: 72, W: 200, H: 56, Shape: "rect", Rows: []diagram.Row{{"API", "main"}}},
			diagram.Table{
				Pos: at(16, 5), X: 40, Y: 200, W: 200, Title: "users",
				Rows: []diagram.TableRow{{"id", "PK", false}, {"---", "", true}, {"name", "", false}, {"email", "", false}},
			},
			diagram.Icon{Pos: at(17, 5), X: 300, Y: 200, Name: "aws/amazon-ec2", Label: "EC2", Size: 48},
			diagram.Edge{
				Pos: at(18, 5), D: "M240,100 H400", Segs: [][4]float64{{240, 100, 400, 100}},
				Label: "HTTPS", LabelAt: &[2]float64{320, 100}, Start: "one", End: "none", Crossing: true,
			},
			diagram.Dot{Pos: at(19, 5), CX: 10, CY: 20, R: 7},
			diagram.Bar{Pos: at(20, 5), X1: 1.5, X2: 3, Y: 4, Tone: "good", Label: "2026"},
			diagram.Axis{
				Pos: at(21, 5), X1: 0, X2: 100, Y: 390,
				Ticks: []diagram.Tick{{0, "10/06"}, {50, "true"}}, Grid: ptr(0.0),
			},
			diagram.Axis{Pos: at(22, 5), X1: 0, X2: 100, Y: 390, Ticks: []diagram.Tick{}},
			diagram.Circle{Pos: at(23, 5), CX: 100, CY: 100, R: 40, Tone: "accent"},
			diagram.Label{Pos: at(24, 5), Text: "1.0", X: 5, Y: 6, Anchor: "start", Plain: true},
			diagram.Line{Pos: at(25, 5), X2: 10, Kind: "line"},
			diagram.Raw{Pos: at(26, 5), SVG: "<rect class=\"band\"/>\n"},
		},
	}
	if p := got.Parts[1].At(); p != at(7, 5) {
		t.Errorf("At() = %v, want the line and column of box", p)
	}
	if len(got.Parts) != len(want.Parts) {
		t.Fatalf("got %d parts, want %d", len(got.Parts), len(want.Parts))
	}
	for i := range want.Parts {
		if !reflect.DeepEqual(got.Parts[i], want.Parts[i]) {
			t.Errorf("part %d:\n got %#v\nwant %#v", i, got.Parts[i], want.Parts[i])
		}
	}
	got.Parts, want.Parts = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("spec = %#v, want %#v", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	const head = "id: d\nwidth: 100\nheight: 100\ntitle: T\nparts:\n"
	tests := []struct {
		name, src, want string
	}{
		{"YAML syntax", "id: a\nid: b\n", `s.yaml:2:1: this is not valid YAML: mapping key "id" already defined at [1:1]`},
		{"empty", "", "s.yaml:1:1: the spec is empty; write a diagram with id, width, height, title and parts"},
		{"two documents", head + "---\nid: e\n", "s.yaml:6:1: a spec holds one diagram; put each --- separated document in a file of its own"},
		{"anchor", head + "  - dot: { cx: &x 1, cy: *x }\n", "s.yaml:6:16: anchors, aliases and tags are not supported; write the value out"},
		{"not a mapping", "- a\n", "s.yaml:1:1: diagram: write a mapping of keys and values"},
		{"unknown top key", head + "  []\nwidht: 3\n", `s.yaml:7:1: diagram: unknown key "widht"; use id, width, height, title, parts`},
		{"missing key", "id: d\nheight: 1\ntitle: T\nparts: []\n", "s.yaml:1:1: diagram: give width"},
		{"bad id", "id: Arch\nwidth: 1\nheight: 1\ntitle: T\nparts: []\n", `s.yaml:1:5: diagram id "Arch": use lowercase letters, digits and hyphens`},
		{"no title", "id: d\nwidth: 1\nheight: 1\nparts: []\n", `s.yaml:1:1: diagram "d": give a title (read aloud by screen readers)`},
		{"blank title", "id: d\nwidth: 1\nheight: 1\ntitle: ' '\nparts: []\n", `s.yaml:4:8: diagram "d": give a title (read aloud by screen readers)`},
		{"parts not a list", "id: d\nwidth: 1\nheight: 1\ntitle: T\nparts: 3\n", "s.yaml:5:8: diagram: parts must be a list"},
		{"part of two keys", head + "  - { dot: {}, box: {} }\n", "s.yaml:6:7: write each part as one key naming its kind, like - box: {x: 40, y: 72, ...}"},
		{"unknown part", head + "  - arrow: {}\n", `s.yaml:6:5: unknown part "arrow"; use frame, box, table, icon, edge, dot, bar, axis, circle, label, line, raw`},
		{"part without keys", head + "  - dot:\n", "s.yaml:6:5: dot: write a mapping of keys and values"},
		{"unknown key before a missing one", head + "  - box: { x: 1, y: 1, wdth: 1, h: 1, rows: a }\n", `s.yaml:6:24: box: unknown key "wdth"; use x, y, w, h, rows, tone, shape`},
		{"missing part key", head + "  - dot: { cx: 1 }\n", "s.yaml:6:5: dot: give cy"},
		{"not a number", head + "  - dot: { cx: 1, cy: '2' }\n", "s.yaml:6:23: dot: cy must be a number"},
		{"not text", head + "  - label: { text: [a], x: 1, y: 1 }\n", "s.yaml:6:20: label: text must be text"},
		{"not a bool", head + "  - dot: { cx: 1, cy: 2, hollow: yes }\n", "s.yaml:6:34: dot: hollow must be true or false"},
		{"unknown tone", head + "  - dot: { cx: 1, cy: 2, tone: red }\n", `s.yaml:6:32: dot: unknown tone "red"; use accent, good, warn, bad`},
		{"unknown marker", head + "  - edge: { d: 'M0,0 H1', end: dot }\n", `s.yaml:6:32: edge: unknown end "dot"; use none, arrow, triangle, diamond, odiamond, one, many, zero-one, zero-many`},
		{"no rows", head + "  - box: { x: 1, y: 1, w: 1, h: 1, rows: [] }\n", "s.yaml:6:42: box: give at least one item in rows"},
		{"row of three", head + "  - box: { x: 1, y: 1, w: 1, h: 1, rows: [[a, sub, x]] }\n", "s.yaml:6:43: box: write each row as text or [text, style]"},
		{"row style", head + "  - box: { x: 1, y: 1, w: 1, h: 1, rows: [[a, code]] }\n", `s.yaml:6:47: box: unknown row style "code"; use main, sub, main code, sub code`},
		{"table row", head + "  - table: { x: 1, y: 1, w: 1, title: t, rows: [[]] }\n", "s.yaml:6:49: table: write each row as a name, [name] or [name, type], or --- for a separator"},
		{"tick", head + "  - axis: { x1: 1, x2: 2, y: 3, ticks: [5] }\n", "s.yaml:6:41: axis: write each tick as [x, text], like [160, 10/06]"},
		{"pair", head + "  - edge: { d: 'M0,0 H1', label: a, at: [1] }\n", "s.yaml:6:41: edge: at must be a pair of numbers, like [320, 100]"},
		{"edge command", head + "  - edge: { d: 'M0,0 C1' }\n", `s.yaml:6:16: edge "M0,0 C1": use absolute M, H, V and L only (got C)`},
		{"edge start", head + "  - edge: { d: H10 }\n", `s.yaml:6:16: edge "H10": start with M`},
		{"edge number", head + "  - edge: { d: M10 }\n", `s.yaml:6:16: edge "M10": expected a number after M`},
		{"edge stray character", head + "  - edge: { d: 'M0,10 H90\" V200' }\n", `s.yaml:6:16: edge "M0,10 H90" V200": unexpected """; write only M, H, V, L, numbers, spaces and commas`},
		{"edge comma after a command", head + "  - edge: { d: 'M,10 H90' }\n", `s.yaml:6:16: edge "M,10 H90": expected a number after M`},
		{"edge doubled comma", head + "  - edge: { d: 'M0,0 H10,,V20' }\n", `s.yaml:6:16: edge "M0,0 H10,,V20": unexpected ","; write only M, H, V, L, numbers, spaces and commas`},
		{"edge relative command", head + "  - edge: { d: 'M0,0 h10' }\n", `s.yaml:6:16: edge "M0,0 h10": use absolute M, H, V and L only (got h)`},
		{"edge number out of range", head + "  - edge: { d: 'M1e999,0' }\n", `s.yaml:6:16: edge "M1e999,0": expected a number after M`},
		{"number out of range", head + "  - dot: { cx: 1e999, cy: 0 }\n", "s.yaml:6:16: dot: cx must be a number"},
		{"zero width", "id: d\nwidth: 0\nheight: 1\ntitle: T\nparts: []\n", "s.yaml:2:8: diagram: width must be greater than 0"},
		{"negative box", head + "  - box: { x: 1, y: 1, w: -1, h: 1, rows: a }\n", "s.yaml:6:27: box: w must be greater than 0"},
		{"zero radius", head + "  - dot: { cx: 1, cy: 1, r: 0 }\n", "s.yaml:6:29: dot: r must be greater than 0"},
		{"zero icon", head + "  - icon: { x: 1, y: 1, name: aws/x, size: 0 }\n", "s.yaml:6:44: icon: size must be greater than 0"},
		{"missing parts", "id: d\nwidth: 1\nheight: 1\ntitle: T\n", "s.yaml:1:1: diagram: give parts"},
		{"missing rows", head + "  - box: { x: 1, y: 1, w: 1, h: 1 }\n", "s.yaml:6:5: box: give rows"},
		{"missing ticks", head + "  - axis: { x1: 1, x2: 2, y: 3 }\n", "s.yaml:6:5: axis: give ticks"},
		{"edge label without at", head + "  - edge: { d: 'M0,0 H1', label: a }\n", `s.yaml:6:34: edge "M0,0 H1" has a label but no at: [x, y]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := diagram.Parse("s.yaml", []byte(tt.src))
			if err == nil {
				t.Fatal("Parse succeeded")
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("error\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestErrorLocalize(t *testing.T) {
	t.Parallel()

	_, err := diagram.Parse("s.yaml", []byte("id: d\nheight: 1\ntitle: T\nparts: []\n"))
	var l i18n.Localizer
	if !errors.As(err, &l) {
		t.Fatalf("error %v is not localized", err)
	}
	if got, want := l.Localize(i18n.New("ja")), "s.yaml:1:1: diagram: width を指定してください"; got != want {
		t.Errorf("Localize(ja) = %q, want %q", got, want)
	}
}

// A diagram inside a data file reports its mistakes at the lines and columns of that file.
func TestFromNode(t *testing.T) {
	t.Parallel()

	src := "title: Report\nsections:\n  - diagram:\n      id: d\n      width: 100\n      height: 50\n      title: T\n      parts:\n        - dot: { cx: 1, cy: 2, r: big }\n"
	file, err := parser.ParseBytes([]byte(src), 0)
	if err != nil {
		t.Fatal(err)
	}
	path, err := yaml.PathString("$.sections[0].diagram")
	if err != nil {
		t.Fatal(err)
	}
	n, err := path.FilterNode(file.Docs[0].Body)
	if err != nil {
		t.Fatal(err)
	}
	_, err = diagram.FromNode("report.yaml", n)
	if got, want := errString(err), "report.yaml:9:35: dot: r must be a number"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// An edge's d takes what SVG path data allows for M, H, V and L, as a browser draws it.
func TestEdgePathForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d    string
		want [][4]float64
	}{
		{"M1E2,0 H+200", [][4]float64{{100, 0, 200, 0}}},
		{"M0-5L.5.5", [][4]float64{{0, -5, .5, .5}}},
		{"\fM0 , 0\tV1e1\n", [][4]float64{{0, 0, 0, 10}}},
		{"M0,0 L1.,2 H3", [][4]float64{{0, 0, 1, 2}, {1, 2, 3, 2}}},
	}
	for _, tt := range tests {
		src := "id: d\nwidth: 9\nheight: 9\ntitle: T\nparts:\n  - edge: { d: " + strconv.Quote(tt.d) + " }\n"
		s, err := diagram.Parse("s.yaml", []byte(src))
		if err != nil {
			t.Errorf("d %q: %v", tt.d, err)
			continue
		}
		e, ok := s.Parts[0].(diagram.Edge)
		if !ok || !reflect.DeepEqual(e.Segs, tt.want) {
			t.Errorf("d %q: runs %v, want %v", tt.d, e.Segs, tt.want)
		}
	}
}
