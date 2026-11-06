package diagram_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

var update = flag.Bool("update", false, "write the golden files of the examples anew")

// drawer is a drawer of the schema of floe's part diagram, as draw compiles it.
func drawer(t *testing.T) *diagram.Drawer {
	t.Helper()
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	c, err := schema.CompilePart(diagram.URL, k.Resources())
	if err != nil {
		t.Fatal(err)
	}
	return diagram.New(c)
}

// iconsOf are the icons of a set kept in the test: aws/aws-lambda alone.
type iconsOf map[string][]byte

func (s iconsOf) Lookup(name string) ([]byte, error) {
	set, _, _ := strings.Cut(name, "/")
	if set != "aws" {
		return nil, icons.ErrNoSet
	}
	if b, ok := s[name]; ok {
		return b, nil
	}
	return nil, icons.ErrNoIcon
}

var lambda = iconsOf{"aws/aws-lambda": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)}

// draw reads src, a spec in s.yaml, and draws it with how, its mistakes told at their lines and columns.
func draw(t *testing.T, src string, how func(d *diagram.Drawer, v any) (string, error)) (string, string) {
	t.Helper()
	root, v, err := yamlpos.ParseMap("s.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := how(drawer(t), doc.Ints(v))
	if err = yamlpos.Place(err, "s.yaml", root, nil); err != nil {
		return "", usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
	}
	return out, ""
}

// toldInJapanese is what drawing src tells, in the Japanese catalog.
func toldInJapanese(t *testing.T, src string) string {
	t.Helper()
	root, v, err := yamlpos.ParseMap("s.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	_, err = render("ja")(drawer(t), doc.Ints(v))
	return usererr.Tell(i18n.New("ja"), yamlpos.Place(err, "s.yaml", root, nil)).Render("floe: ")
}

func render(lang string) func(d *diagram.Drawer, v any) (string, error) {
	return func(d *diagram.Drawer, v any) (string, error) {
		svg, err := d.Draw(v, diagram.Options{Lang: i18n.Lang(lang), Icons: lambda, Seq: 1})
		return string(svg), err
	}
}

// inspect is the lines of the parts Drawer.Inspect tells, without the line of the diagram.
func inspect(d *diagram.Drawer, v any) (string, error) {
	got, err := d.Inspect(v, diagram.Options{Lang: "en", Icons: lambda, Seq: 1})
	_, parts, _ := strings.Cut(got, "\n")
	return parts, err
}

func golden(t *testing.T, file, got string) {
	t.Helper()
	path := filepath.Join("testdata", file)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs from what is drawn now; run just golden and read the change", file)
	}
}

// Each example is drawn in English and in Japanese, and inspected, as the golden files hold; the icon part's
// icons are the test's.
func TestExamples(t *testing.T) {
	t.Parallel()

	examples, err := diagram.Examples()
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != 15 {
		t.Errorf("%d examples, want 15", len(examples))
	}
	store := iconsOf{}
	for _, name := range []string{"aws/aws-lambda", "aws/amazon-bedrock", "aws/group/aws-cloud-logo", "aws/group/region", "aws/res/client", "aws/res/users", "aws/amazon-route-53", "aws/amazon-cloudfront", "aws/amazon-simple-storage-service", "aws/group/virtual-private-cloud-vpc", "aws/group/public-subnet", "aws/group/private-subnet", "aws/res/elastic-load-balancing-application-load-balancer", "aws/amazon-elastic-container-service", "aws/amazon-rds", "aws/amazon-simple-queue-service"} {
		store[name] = []byte(`<svg xmlns="http://www.w3.org/2000/svg"><title>` + name + `</title></svg>`)
	}
	for _, e := range examples {
		for _, lang := range []string{"en", "ja"} {
			got, mistakes := draw(t, string(e.Spec), func(d *diagram.Drawer, v any) (string, error) {
				svg, err := d.Draw(v, diagram.Options{Lang: i18n.Lang(lang), Icons: store, Seq: 1})
				return string(svg), err
			})
			if mistakes != "" {
				t.Errorf("%s in %s: %s", e.Name, lang, mistakes)
				continue
			}
			golden(t, e.Name+"."+lang+".html", got+"\n")
		}
		got, mistakes := draw(t, string(e.Spec), func(d *diagram.Drawer, v any) (string, error) {
			return d.Inspect(v, diagram.Options{Lang: "en", Icons: store, Seq: 1})
		})
		if mistakes != "" {
			t.Errorf("%s inspected: %s", e.Name, mistakes)
			continue
		}
		golden(t, e.Name+".inspect", got)
	}
}

const head = "width: 400\nheight: 200\ntitle: T\nparts:\n"

// rawWords are the words of a mistake of raw SVG, after the element it holds.
const rawWords = "which a diagram may not hold: leave it out, and draw the looks with SVG's shapes and attributes (fill= and the like)"

// A spec of the wrong shape is told in jsonschema's words, at its line and column; nothing is laid out.
func TestShapeMistakes(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		parts, want string
	}{
		{"  - boxx: {}", "s.yaml:5:5: additional properties 'boxx' not allowed"},
		{"  - {}", "s.yaml:5:5: minProperties: got 0, want 1"},
		{"  - box: { x: 1, y: 1, rows: [a], tone: blue }", "s.yaml:5:41: value must be one of 'accent', 'good', 'warn', 'bad', 'muted'"},
		{"  - box: { y: 1, rows: [a] }", "s.yaml:5:12: missing property 'x'"},
		{"  - box: { x: 1, y: 1, rows: [{ ja: あ }] }", "s.yaml:5:33: missing property 'en'"},
		{"  - box: { x: 1, y: 1, rows: [a], w: 0 }", "s.yaml:5:38: minimum: got 0, want 1"},
		{"  - edge: { from: a, to: b, d: M1 }", "s.yaml:5:13: additional properties 'd' not allowed"},
		{"  - edge: { from: a, to: b, fromSide: up }", "s.yaml:5:39: value must be one of 'top', 'right', 'bottom', 'left'"},
		{"  - label: { text: a }", "s.yaml:5:14: missing properties 'x', 'y'"},
		{"  - label: { text: a, x: 1 }", "s.yaml:5:14: missing property 'y'"},
		{"  - circle: { name: c, cx: 1, cy: 1, r: 1 }\n  - label: { text: a, x: 1, in: [c] }", "s.yaml:6:14: properties 'y' required, if 'x' exists"},
		{"  - edge: { from: [1, 2, 3], to: b }", "s.yaml:5:19: maxItems: got 3, want 2"},
		// A point of via is either shape, each told in the library's words of its own: oneOf would say none matched.
		{"  - edge: { from: a, to: b, via: [{}] }", "s.yaml:5:35: minProperties: got 0, want 1"},
		{"  - edge: { from: a, to: b, via: [{ z: 1 }] }", "s.yaml:5:37: additional properties 'z' not allowed"},
		{"  - edge: { from: a, to: b, via: [[1]] }", "s.yaml:5:35: minItems: got 1, want 2"},
		{"  - edge: { from: a, to: b, via: [[1, 2, 3]] }", "s.yaml:5:35: maxItems: got 3, want 2"},
		{"  - edge: { from: a, to: b, via: [140] }", "s.yaml:5:35: got number, want array or object"},
		{"  - edge: { from: a, to: b, via: [{ x: a }] }", "s.yaml:5:40: got string, want number"},
		{"  - axis: { x1: 1, x2: 9, y: 1, ticks: [], labels: Between }", "s.yaml:5:52: value must be one of 'at', 'between'"},
		{"  - box: { x: 1, y: 1, rows: [a], zz: 1, aa: 1, mm: 1 }", "s.yaml:5:12: additional properties 'aa', 'mm', 'zz' not allowed"},
		// A label placed by its region takes no anchor; one at x and y does.
		{"  - circle: { name: c, cx: 100, cy: 100, r: 80 }\n  - label: { text: a, in: [c], anchor: start }", "s.yaml:6:14: properties 'x' required, if 'anchor' exists"},
		{"  - circle: { name: c, cx: 100, cy: 100, r: 80 }\n  - label: { text: a, in: [c], x: 100, anchor: start }", "s.yaml:6:14: properties 'y' required, if 'x' exists"},
		{"  - circle: { name: c, cx: 100, cy: 100, r: 80 }\n  - label: { text: a, in: [c], x: 100, y: 100, anchor: start }", ""},
		{"  - label: { text: a, x: 100, y: 100, anchor: end }", ""},
	} {
		// The library's map order does not change the words, however often they are told.
		for range 5 {
			if _, got := draw(t, head+c.parts+"\n", render("en")); got != c.want {
				t.Errorf("%s\n  told %q\n  want %q", c.parts, got, c.want)
				break
			}
		}
	}
	// A value no spec, given to Draw unchecked (by a part.html of one's own), is a mistake, not floe's failure.
	for _, v := range []any{42, map[string]any{"width": 1}, nil} {
		_, err := drawer(t).Draw(v, diagram.Options{Lang: "en", Icons: lambda, Seq: 1})
		if r := usererr.Tell(i18n.EnglishCatalog(), err); r.Failure != "" || len(r.Mistakes) == 0 {
			t.Errorf("Draw(%v) = %+v, want mistakes", v, r)
		}
	}
}

// The layout is made once for every language: a mistake of it is told once, the parts named by their names or
// their English, and the width a box needs is that of its widest language.
func TestLayoutMistakes(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		parts, want string
	}{
		{
			"  - box: { x: 20, y: 20, rows: [{ en: A, ja: あ }] }\n  - box: { name: b, x: 30, y: 30, rows: [{ en: B, ja: い }] }",
			`s.yaml:6:5: box "あ" overlaps box "b"`,
		},
		{"  - box: { name: a, x: 20, y: 20, rows: [A] }\n  - box: { name: a, x: 200, y: 20, rows: [B] }", `s.yaml:6:5: the name "a" is given before: give each part, row and node its own name`},
		{"  - edge: { from: chek, to: [10, 10] }", `s.yaml:5:5: edge: no part, row or node named "chek"`},
		{
			"  - circle: { name: c, cx: 100, cy: 100, r: 50 }\n  - edge: { from: c, to: [300, 10] }",
			`s.yaml:6:5: edge: "c" is a circle, which no edge can end at: end it at a box, a table or its named row, an icon, a frame, a bar, a dot or a point`,
		},
		{
			"  - edge: { from: [10, 10], fromSide: top, to: [300, 10] }",
			`s.yaml:5:5: edge: [10,10] has no side top: write the side of a box, a table, an icon, a frame or a bar, left or right of a row, and none for a dot or a point`,
		},
		{
			"  - table: { x: 20, y: 20, title: T, rows: [{ name: r, cells: [a] }] }\n  - edge: { from: r, fromSide: top, to: [300, 10] }",
			`s.yaml:6:5: edge: "r" has no side top: write the side of a box, a table, an icon, a frame or a bar, left or right of a row, and none for a dot or a point`,
		},
		{
			"  - edge: { from: [10, 100], to: [390, 100] }\n  - edge: { from: [200, 10], to: [200, 190] }",
			"s.yaml:6:5: edge [10,100]→[390,100] crosses edge [200,10]→[200,190] at 200,100; move one of them, or add crossing: true to one if it cannot be avoided",
		},
		{"  - label: { text: hi, x: 500, y: 20 }", `s.yaml:5:5: label "hi" sticks out of the 400x200 canvas`},
		{
			"  - circle: { name: a, cx: 100, cy: 100, r: 60 }\n  - label: { text: a long label here, x: 100, y: 100, in: [a] }",
			`s.yaml:6:5: label "a long label here" is not all inside a and outside the other circles: move it, or leave out x and y for floe to place it`,
		},
		{
			"  - circle: { name: a, cx: 100, cy: 100, r: 20 }\n  - label: { text: a label far too wide for it, in: [a] }",
			`s.yaml:6:5: label "a label far too wide for it" finds no room inside a and outside the other circles: make that area larger, or the text shorter`,
		},
		{"  - raw: { svg: '<rect/></svg><p>x</p>' }", "s.yaml:5:17: raw SVG holds <p>, " + rawWords},
		{"  - raw: { svg: '<foreignObject><div/></foreignObject>' }", "s.yaml:5:17: raw SVG holds <foreignObject>, " + rawWords},
		{"  - raw: { svg: '<style>rect{}</style>' }", "s.yaml:5:17: raw SVG holds <style>, " + rawWords},
		// The elements no input of a page may write, in whatever case, under others, and past an </svg> too.
		{"  - raw: { svg: '<noscript><circle r=\"3\"/></noscript>' }", "s.yaml:5:17: raw SVG holds <noscript>, " + rawWords},
		{"  - raw: { svg: '<base href=\"https://x\"/>' }", "s.yaml:5:17: raw SVG holds <base>, " + rawWords},
		{"  - raw: { svg: '<NoScript></NoScript>' }", "s.yaml:5:17: raw SVG holds <noscript>, " + rawWords},
		{"  - raw: { svg: '<g><g><noscript/></g></g>' }", "s.yaml:5:17: raw SVG holds <noscript>, " + rawWords},
		{"  - raw: { svg: '<BASE/>' }", "s.yaml:5:17: raw SVG holds <base>, " + rawWords},
		{"  - raw: { svg: '<circle r=\"3\"/></svg><noscript>x</noscript>' }", "s.yaml:5:17: raw SVG holds <noscript>, " + rawWords},
		{"  - raw: { svg: '<desc><noscript>x</noscript></desc>' }", "s.yaml:5:17: raw SVG holds <noscript>, " + rawWords},
		// Every element raw SVG may not hold is told, at one place and so sorted by its words, but none inside one
		// told.
		{"  - raw: { svg: '<noscript/><base href=\"x\"/><style/>' }", "s.yaml:5:17: raw SVG holds <base>, " + rawWords + "\ns.yaml:5:17: raw SVG holds <noscript>, " + rawWords + "\ns.yaml:5:17: raw SVG holds <style>, " + rawWords},
		{"  - raw: { svg: '<g><circle r=\"1\"/><g><style/></g><base/></g>' }", "s.yaml:5:17: raw SVG holds <base>, " + rawWords + "\ns.yaml:5:17: raw SVG holds <style>, " + rawWords},
		{"  - raw: { svg: '<g><script>x</script></g><rect/></svg><p><b>x</b></p>' }", "s.yaml:5:17: raw SVG holds <p>, " + rawWords + "\ns.yaml:5:17: raw SVG holds <script>, " + rawWords},
		// Attributes are kept as written, and a name in text, a comment or an attribute is no element.
		{"  - raw: { svg: '<circle onload=\"f()\" r=\"3\"/>' }", ""},
		{"  - raw: { svg: '<a href=\"javascript:f()\"><text>x</text></a>' }", ""},
		{"  - raw: { svg: '<g><title>t</title><desc>d</desc><circle r=\"3\"/></g>' }", ""},
		{"  - raw: { svg: '<text>noscript base</text>' }", ""},
		{"  - raw: { svg: '<g class=\"noscript\"><!-- <noscript> --></g>' }", ""},
		// A mistake of parts written by hand is told at the one written last, whatever order the check compares them in;
		// at the first of those of one part.
		{
			"  - dot: { cx: 110, cy: 100 }\n  - label: { text: Dark mode, x: 122, y: 100, anchor: start }\n" +
				"  - dot: { cx: 110, cy: 112 }\n  - label: { text: Share a note, x: 122, y: 112, anchor: start }",
			"s.yaml:7:5: dot 103,93 14x14 overlaps dot 103,105 14x14\ns.yaml:8:5: label \"Dark mode\" overlaps label \"Share a note\"",
		},
		{"  - label: { text: a note, x: 150, y: 100 }\n  - box: { name: a, x: 120, y: 80, w: 80, h: 40, rows: [a] }", `s.yaml:6:5: box "a" overlaps label "a note"`},
		{"  - box: { name: a, x: 120, y: 80, w: 80, h: 40, rows: [a] }\n  - edge: { from: [20, 100], to: [300, 100] }", `s.yaml:6:5: edge [20,100]→[300,100] runs through box "a"; move one of them, or go round it by via`},
		{"  - edge: { from: [20, 100], to: [300, 100] }\n  - box: { name: a, x: 120, y: 80, w: 80, h: 40, rows: [a] }", `s.yaml:6:5: edge [20,100]→[300,100] runs through box "a"; move one of them, or go round it by via`},
		{"  - frame: { x: 10, y: 10, w: 200, h: 180, label: F }\n  - box: { name: a, x: 180, y: 80, w: 80, h: 40, rows: [a] }", `s.yaml:6:5: box "a" straddles the edge of frame "F" at x 210, spanning 180 to 260`},
		{"  - box: { name: a, x: 180, y: 80, w: 80, h: 40, rows: [a] }\n  - frame: { x: 10, y: 10, w: 200, h: 180, label: F }", `s.yaml:6:5: box "a" straddles the edge of frame "F" at x 210, spanning 180 to 260`},
		{
			"  - box: { name: a, x: 20, y: 20, w: 100, h: 40, rows: [a] }\n  - box: { name: b, x: 200, y: 20, w: 100, h: 40, rows: [b] }\n" +
				"  - box: { name: c, x: 60, y: 30, w: 200, h: 30, rows: [c] }",
			"s.yaml:7:5: box \"a\" overlaps box \"c\"\ns.yaml:7:5: box \"b\" overlaps box \"c\"",
		},
		{
			"  - box: { name: a, x: 120, y: 60, w: 80, h: 40, rows: [a] }\n  - edge: { from: [20, 106], to: [300, 106], label: a long edge label here }",
			`s.yaml:6:5: box "a" overlaps label "a long edge label here"`,
		},
		// Two parts named in the same words, told at one place, are told once (LIMIT in tell).
		{
			"  - box: { x: 20, y: 20, w: 60, h: 40, rows: [S] }\n  - box: { x: 90, y: 20, w: 60, h: 40, rows: [S] }\n  - label: { text: across both, x: 85, y: 40 }",
			`s.yaml:7:5: box "S" overlaps label "across both"`,
		},
		// A part with no name is named by its text in the language it is laid out as wide as (by code's widths in a code
		// row), else by where it stands.
		{
			"  - label: { text: { en: Decide what to build, ja: \"作るものと作らないものを、使う人と話して決める\" }, x: 16, y: 44, anchor: start }\n" +
				"  - bar: { x1: 200, x2: 280, y: 44 }",
			`s.yaml:6:5: bar 200,37 80x14 overlaps label "作るものと作らないものを、使う人と話して決める"`,
		},
		{
			"  - frame: { x: 10, y: 10, w: 250, h: 150 }\n  - frame: { x: 300, y: 100, w: 150, h: 40 }\n  - box: { x: 0, y: 140, w: 80, h: 40, rows: [A] }",
			"s.yaml:6:5: frame 300,100 150x40 sticks out of the 400x200 canvas\ns.yaml:7:5: box \"A\" straddles the edge of frame 10,10 250x150 at x 10, spanning 0 to 80\ns.yaml:7:5: box \"A\" straddles the edge of frame 10,10 250x150 at y 160, spanning 140 to 180",
		},
		// A label's text is empty in no language, wherever a key takes one; rows and a table's title may be.
		{"  - box: { name: a, x: 10, y: 30, w: 60, h: 30, rows: [a] }\n  - edge: { from: a, to: [200, 45], label: \"\" }", "s.yaml:6:44: minLength: got 0, want 1"},
		{"  - box: { name: a, x: 10, y: 30, w: 60, h: 30, rows: [a] }\n  - edge: { from: a, to: [200, 45], label: { en: \"\", ja: あ } }", "s.yaml:6:50: minLength: got 0, want 1"},
		{"  - edge: { from: [10, 45], to: [200, 45], label: { en: a, ja: \"\" } }", "s.yaml:5:64: minLength: got 0, want 1"},
		{"  - bar: { x1: 10, x2: 100, y: 85, label: \"\" }", "s.yaml:5:43: minLength: got 0, want 1"},
		{"  - icon: { icon: aws/amazon-ec2, x: 10, y: 10, label: \"\" }", "s.yaml:5:56: minLength: got 0, want 1"},
		{"  - frame: { x: 100, y: 100, w: 100, h: 60, label: \"\" }", "s.yaml:5:52: minLength: got 0, want 1"},
		{"  - label: { text: \"\", x: 100, y: 100 }", "s.yaml:5:20: minLength: got 0, want 1"},
		{"  - circle: { name: A, cx: 150, cy: 100, r: 60 }\n  - label: { text: \"\", in: [A] }", "s.yaml:6:20: minLength: got 0, want 1"},
		{"  - graph: { x: 20, y: 20, nodes: [{ box: { name: a, rows: [a] } }], edges: [{ from: a, to: a, label: \"\" }] }", "s.yaml:5:103: minLength: got 0, want 1"},
		{"  - graph: { x: 20, y: 20, nodes: [{ group: { name: g, label: \"\", nodes: [{ box: { name: a, rows: [a] } }] } }] }", "s.yaml:5:63: minLength: got 0, want 1"},
		{"  - box: { x: 10, y: 30, rows: [\"\"] }\n  - box: { x: 100, y: 30, rows: [[\"\", sub]] }\n  - table: { x: 200, y: 10, title: \"\", rows: [[\"\", \"\"]] }", ""},
		// Accepted, as the LIMITs of label and addAxis say: text of spaces, and a tick empty in one language, named by the other.
		{"  - edge: { from: [10, 45], to: [200, 45], label: \" \" }", ""},
		{"  - axis: { x1: 20, x2: 300, y: 100, ticks: [[160, { en: \"\", ja: 月 }]] }\n  - box: { name: b, x: 150, y: 100, w: 60, h: 30, rows: [b] }", `s.yaml:6:5: box "b" overlaps label "月"`},
		// A tick of no text in any language is a mark alone: nothing overlaps it or sticks out (an edge through it: f13).
		{"  - axis: { x1: 20, x2: 300, y: 100, ticks: [[20, \"\"], [160, \"\"]] }\n  - box: { name: a, x: 150, y: 100, w: 60, h: 30, rows: [a] }", ""},
		{"  - axis: { x1: 20, x2: 300, y: 100, ticks: [[20, { en: \"\", ja: \"\" }], [160, { en: \"\", ja: \"\" }]] }\n  - box: { name: a, x: 150, y: 100, w: 60, h: 30, rows: [a] }", ""},
		{"  - axis: { x1: 10, x2: 396, y: 100, ticks: [[10, \"\"], [396, \"\"]] }", ""},
		// A tick's text between ticks is checked as at one.
		{"  - axis: { x1: 20, x2: 380, y: 100, ticks: [[20, Mon], [140, \"\"]], labels: between }\n  - box: { name: a, x: 60, y: 105, w: 40, h: 30, rows: [[a, sub]] }", `s.yaml:6:5: box "a" overlaps label "Mon"`},
		{"  - box: { x: 20, y: 20, rows: [[{ en: \"あいう\", ja: \"abcde\" }, main code]] }\n  - box: { name: b, x: 30, y: 30, w: 80, h: 30, rows: [b] }", `s.yaml:6:5: box "abcde" overlaps box "b"`},
		{
			"  - circle: { name: A, cx: 150, cy: 100, r: 20 }\n  - label: { text: a much longer label than fits, in: [A] }",
			"s.yaml:6:5: label \"a much longer label than fits\" finds no room inside A and outside the other circles: make that area larger, or the text shorter",
		},
		{"  - circle: { cx: 390, cy: 100, r: 20 }", "s.yaml:5:5: circle 370,80 40x40 sticks out of the 400x200 canvas"},
		// A label at x and y whose corner is on the circle's edge is in, wherever the circle stands; one 0.001 out is not.
		{"  - circle: { name: A, cx: 100.3, cy: 100, r: 50 }\n  - label: { text: Mon, in: [A], x: 114.5, y: 131, plain: true }", ""},
		{"  - circle: { name: A, cx: 117.3, cy: 100, r: 50 }\n  - label: { text: Mon, in: [A], x: 131.5, y: 131, plain: true }", ""},
		{
			"  - circle: { name: A, cx: 100.3, cy: 100, r: 50 }\n  - label: { text: Mon, in: [A], x: 114.5, y: 131.00125, plain: true }",
			"s.yaml:6:5: label \"Mon\" is not all inside A and outside the other circles: move it, or leave out x and y for floe to place it",
		},
		{"  - dot: { name: d, cx: 50, cy: 50 }\n  - box: { name: b, x: 40, y: 40, w: 60, h: 30, rows: [b] }", `s.yaml:6:5: box "b" overlaps dot "d"`},
		// Each check of the placement on its own.
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - box: { x: 80, y: 40, w: 60, h: 40, rows: [A] }", `s.yaml:6:5: box "A" straddles the edge of frame "F" at x 110, spanning 80 to 140`},
		{"  - box: { name: b, x: 150, y: 80, w: 60, h: 40, rows: [B] }\n  - edge: { from: [100, 100], to: [300, 100] }", `s.yaml:6:5: edge [100,100]→[300,100] runs through box "b"; move one of them, or go round it by via`},
		// A label part written at an x and y straddles a frame as a box does, by its box; inside or outside one, or
		// across frames that cross, as a box would.
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - label: { text: across, x: 100, y: 50 }", `s.yaml:6:5: label "across" straddles the edge of frame "F" at x 110, spanning 73.4 to 126.6`},
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - label: { text: in, x: 50, y: 50 }", ""},
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - label: { text: out, x: 200, y: 50 }", ""},
		{
			"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - frame: { x: 120, y: 10, w: 100, h: 100, label: G }\n  - frame: { x: 5, y: 60, w: 230, h: 40, square: true }\n" +
				"  - label: { text: a heading across both, x: 20, y: 80, anchor: start }",
			"s.yaml:8:5: label \"a heading across both\" straddles the edge of frame \"F\" at x 110, spanning 15 to 176.2\n" +
				"s.yaml:8:5: label \"a heading across both\" straddles the edge of frame \"G\" at x 120, spanning 15 to 176.2",
		},
		{"  - circle: { name: c, cx: 100, cy: 100, r: 90 }\n  - frame: { x: 10, y: 10, w: 100, h: 180, label: F }\n  - label: { text: across, x: 110, y: 100, in: [c] }", `s.yaml:7:5: label "across" straddles the edge of frame "F" at x 110, spanning 83.4 to 136.6`},
		// One out of its region is told so alone.
		{
			"  - circle: { name: c, cx: 100, cy: 100, r: 90 }\n  - frame: { x: 10, y: 10, w: 100, h: 180, label: F }\n  - label: { text: across, x: 110, y: 20, in: [c] }",
			`s.yaml:7:5: label "across" is not all inside c and outside the other circles: move it, or leave out x and y for floe to place it`,
		},
		// An edge's label written at an x and y, as a label part.
		{
			"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - edge: { from: [20, 150], to: [200, 150], label: a long note, at: [100, 50] }",
			`s.yaml:6:5: label "a long note" straddles the edge of frame "F" at x 110, spanning 55.4 to 144.6`,
		},
		// Of a part that may not straddle a frame itself, as a box: an icon's, by hand or in a graph, and a bar's; told at
		// the last written, the frame after the part at the frame's line.
		{
			"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - icon: { name: a, x: 56, y: 50, icon: aws/aws-lambda, label: Lambda function }",
			`s.yaml:6:5: label "Lambda function" straddles the edge of frame "f" at y 110, spanning 101 to 119`,
		},
		{
			"  - icon: { name: a, x: 56, y: 50, icon: aws/aws-lambda, label: Lambda function }\n  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }",
			`s.yaml:6:5: label "Lambda function" straddles the edge of frame "f" at y 110, spanning 101 to 119`,
		},
		{"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - icon: { name: a, x: 56, y: 41, icon: aws/aws-lambda, label: Lambda function }", ""},
		{"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - icon: { name: a, x: 200, y: 50, icon: aws/aws-lambda, label: Lambda function }", ""},
		{
			"  - frame: { name: f, x: 0, y: 0, w: 300, h: 80 }\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - icon: { name: l, icon: aws/aws-lambda, label: Lambda function name }",
			`s.yaml:5:5: label "Lambda function name" straddles the edge of frame "f" at y 80, spanning 71 to 89`,
		},
		{
			"  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - group: { name: g, label: G, nodes: [{ icon: { name: l, icon: aws/aws-lambda, label: A very long Lambda function name } }] }",
			"",
		},
		{
			"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - bar: { x1: 20, x2: 80, y: 50, label: a long bar label }",
			`s.yaml:6:5: label "a long bar label" straddles the edge of frame "F" at x 110, spanning 85 to 210.2`,
		},
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - bar: { x1: 20, x2: 40, y: 50, label: ok }", ""},
		// Once for each edge crossed: a corner, a box wider than the frame; one that only touches an edge is not told.
		{
			"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - box: { name: a, x: 100, y: 90, w: 80, h: 40, rows: [a] }",
			`s.yaml:6:5: box "a" straddles the edge of frame "f" at x 140, spanning 100 to 180` + "\n" +
				`s.yaml:6:5: box "a" straddles the edge of frame "f" at y 110, spanning 90 to 130`,
		},
		{
			"  - frame: { name: f, x: 20, y: 50, w: 120, h: 90 }\n  - box: { name: a, x: 10, y: 80, w: 200, h: 40, rows: [a] }",
			`s.yaml:6:5: box "a" straddles the edge of frame "f" at x 140, spanning 10 to 210` + "\n" +
				`s.yaml:6:5: box "a" straddles the edge of frame "f" at x 20, spanning 10 to 210`,
		},
		{"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - box: { name: a, x: 140, y: 40, w: 80, h: 40, rows: [a] }", ""},
		// A dot and a bar, of no x and y to write, spanned as any part.
		{
			"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - dot: { name: a, cx: 140, cy: 60 }",
			`s.yaml:6:5: dot "a" straddles the edge of frame "f" at x 140, spanning 133 to 147`,
		},
		{
			"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - bar: { x1: 100, x2: 200, y: 60 }",
			`s.yaml:6:5: bar 100,53 100x14 straddles the edge of frame "f" at x 140, spanning 100 to 200`,
		},
		// Not of those: an edge's in its middle, an axis's, a graph edge's, a label part's in its region (LIMIT in
		// checkPlacement): not compared.
		{"  - frame: { name: f, x: 20, y: 20, w: 120, h: 90 }\n  - axis: { x1: 10, x2: 300, y: 60, ticks: [[135, a tick label]], labels: at }", ""},
		{"  - circle: { name: c, cx: 100, cy: 100, r: 90 }\n  - frame: { x: 10, y: 10, w: 100, h: 180, label: F }\n  - label: { text: placed across the edge, in: [c] }", ""},
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - frame: { x: 150, y: 10, w: 100, h: 100, label: G }\n  - edge: { from: [20, 60], to: [240, 60], label: an edge label }", ""},
		{
			"  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n" +
				"      edges:\n        - { from: a, to: b, label: a graph label }\n  - frame: { x: 120, y: 5, w: 200, h: 150, label: F }",
			"",
		},
		{"  - label: { text: L, x: 200, y: 100 }\n  - edge: { from: [100, 100], to: [300, 100] }", `s.yaml:6:5: edge [100,100]→[300,100] runs through label "L"; move one of them, or go round it by via`},
		{"  - box: { x: 20, y: 20, w: 80, h: 20, rows: [A, B] }", `s.yaml:5:5: box "A": its text needs a height of 46px or more; the box has 20px`},
		{"  - box: { x: 20, y: -5, w: 60, h: 40, rows: [A] }", `s.yaml:5:5: box "A" sticks out of the 400x200 canvas`},
		{"  - box: { x: 20, y: 180, w: 60, h: 40, rows: [A] }", `s.yaml:5:5: box "A" sticks out of the 400x200 canvas`},
		{"  - edge: { from: [10, 10], to: [10, 10] }", "s.yaml:5:5: edge [10,10]→[10,10] starts where it ends: end it at another part or point"},
		{
			"  - box: { name: a, x: 100, y: 20, w: 80, h: 29, rows: [a] }\n  - edge: { from: a, fromSide: left, to: a, toSide: left }",
			"s.yaml:6:5: edge \"a\"→\"a\" starts where it ends: end it at another part or point",
		},
		{
			"  - circle: { name: c, cx: 100, cy: 100, r: 60 }\n  - circle: { name: c, cx: 300, cy: 100, r: 5 }\n  - label: { text: x, in: [c] }",
			`s.yaml:6:5: the name "c" is given before: give each part, row and node its own name`,
		},
		{"  - icon: { icon: aws/amazon-ec2, x: 10, y: 10 }", `s.yaml:5:5: icon "aws/amazon-ec2": no icon "aws/amazon-ec2" (search with: floe icons search <word>)`},
	} {
		if _, got := draw(t, head+c.parts+"\n", render("en")); got != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, got, c.want)
		}
	}
	// A label keeps out of every other circle, one over the middle of its box or along an edge too, and is placed
	// where none is: in the middle of the circles' boxes of one room.
	venn := "width: 300\nheight: 300\ntitle: T\nparts:\n  - circle: { name: a, cx: 150, cy: 150, r: 140 }\n"
	region := `s.yaml:7:5: label "only in a" is not all inside a and outside the other circles: move it, or leave out x and y for floe to place it`
	at := "  - label: { text: only in a, in: [a], x: 150, y: 150 }"
	for _, c := range []struct{ parts, want string }{
		{"  - circle: { name: b, cx: 150, cy: 150, r: 8 }\n" + at, region},
		{"  - circle: { name: b, cx: 150, cy: 165, r: 8 }\n" + at, region},
		{"  - circle: { name: b, cx: 105, cy: 150, r: 8 }\n" + at, region},
		{"  - circle: { name: b, cx: 195, cy: 150, r: 8 }\n" + at, region},
		{"  - circle: { name: c, cx: 230, cy: 150, r: 60 }\n  - label: { text: a and c, in: [a, c], x: 230, y: 150 }", ""},
		{
			"  - circle: { name: c, cx: 230, cy: 150, r: 60 }\n  - label: { text: a and c, in: [a, c], x: 200, y: 150 }",
			`s.yaml:7:5: label "a and c" is not all inside a, c and outside the other circles: move it, or leave out x and y for floe to place it`,
		},
		{
			"  - circle: { name: b, cx: 150, cy: 162, r: 5 }\n  - circle: { name: c, cx: 150, cy: 138, r: 5 }\n" + at,
			strings.Replace(region, "7:5", "8:5", 1),
		},
		{
			"  - circle: { name: b, cx: 150, cy: 150, r: 60 }\n  - label: { text: a fairly long label for the ring, in: [a] }",
			`s.yaml:7:5: label "a fairly long label for the ring" finds no room inside a and outside the other circles: make that area larger, or the text shorter`,
		},
		{"  - circle: { name: b, cx: 150, cy: 200, r: 8 }\n" + at, ""},
		{"  - circle: { name: b, cx: 150, cy: 168, r: 9 }\n" + at, ""},
	} {
		if _, got := draw(t, venn+c.parts+"\n", render("en")); got != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, got, c.want)
		}
	}
	for parts, want := range map[string]string{
		"  - circle: { name: b, cx: 150, cy: 150, r: 6 }\n  - label: { text: a rather long label text here ok, in: [a] }": `label "a rather long label text here ok" 29.8,113 240.4x18 in a`,
		"  - circle: { name: b, cx: 150, cy: 150, r: 8 }\n  - label: { text: only in a, in: [a] }":                        `label "only in a" 112.6,73 74.8x18 in a`,
	} {
		if got, mistakes := draw(t, venn+parts+"\n", inspect); mistakes != "" || !strings.Contains(got, want) {
			t.Errorf("%s\n  placed %s %s\n  want %s", parts, got, mistakes, want)
		}
	}
	// The icons' own mistakes (none fetched) are of the whole output, told once at no place however many icons need them.
	missing := usererr.NewWhole("icons_not_fetched", "AWS", "/x/aws", "aws", "14MB", "aws.amazon.com")
	none := failing{missing}
	src := head + "  - icon: { icon: aws/a, x: 10, y: 10 }\n  - icon: { icon: aws/b, x: 100, y: 10 }\n"
	root, v, _ := yamlpos.ParseMap("s.yaml", []byte(src)) //nolint:errcheck // read just above
	_, err := drawer(t).Draw(doc.Ints(v), diagram.Options{Lang: "en", Icons: none, Seq: 1})
	r := usererr.Tell(i18n.EnglishCatalog(), yamlpos.Place(err, "s.yaml", root, nil))
	if len(r.Mistakes) != 1 || r.Mistakes[0].File != "" {
		t.Errorf("icons not fetched = %q, want it once, at no place", r.Render("floe: "))
	}
}

// A diagram's title is empty in no language; one of spaces alone passes, as draw's LIMIT says.
func TestTitleNotEmpty(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ title, want string }{
		{`""`, "s.yaml:1:8: minLength: got 0, want 1"},
		{`{ en: "", ja: あ }`, "s.yaml:1:14: minLength: got 0, want 1"},
		{`{ en: a, ja: "" }`, "s.yaml:1:21: minLength: got 0, want 1"},
		{`" "`, ""},
		{"T", ""},
	} {
		src := "title: " + c.title + "\nwidth: 200\nheight: 80\nparts:\n  - box: { x: 10, y: 10, rows: [a] }\n"
		if _, got := draw(t, src, render("en")); got != c.want {
			t.Errorf("title %s:\n  told %q\n  want %q", c.title, got, c.want)
		}
	}
}

type failing struct{ err error }

func (f failing) Lookup(string) ([]byte, error) {
	return nil, f.err
}

// Text too wide is told in the language whose text is widest, English when another is only as wide: widths that
// are equal compare equal, wherever the text is and whatever its size.
func TestTooNarrow(t *testing.T) {
	t.Parallel()

	codeEn, codeJa := strings.Repeat("A", 50), "あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほま"
	for _, c := range []struct {
		parts, want string
	}{
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: Short, ja: ずっと長い見出しの文字 }] }", `s.yaml:5:5: box "ずっと長い見出しの文字": "ずっと長い見出しの文字" (ja) needs w: 175 or more`},
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: { en: F, ja: とても長い日本語の見出し } }", `s.yaml:5:5: frame "とても長い日本語の見出し": "とても長い日本語の見出し" (ja) needs w: 168 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: { en: T, ja: とても長い日本語の題 }, rows: [a] }", `s.yaml:5:5: table "とても長い日本語の題": "とても長い日本語の題" (ja) needs w: 162 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[{ en: id, ja: とても長い識別子 }, int]] }", `s.yaml:5:5: table "T": "とても長い識別子  int" (ja) needs w: 157 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[id, { en: int, ja: とても長い型の名前 }]] }", `s.yaml:5:5: table "T": "id  とても長い型の名前" (ja) needs w: 162 or more`},
		// The name and the type in other languages: each language's row as drawn, falling back to English.
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[{ en: id, ja: とても長い識別子 }, { en: int, ko: 아주 긴 정수 형식 }]] }", `s.yaml:5:5: table "T": "id  아주 긴 정수 형식" (ko) needs w: 160 or more`},
		// A row is measured as drawn: the name and the two spaces after it in code, the type not; the name and the
		// type of one language, so the long Japanese name and the long English type are not added up.
		{"  - table: { x: 20, y: 20, w: 200, title: T, rows: [[id, \"character varying(255)\"]] }", `s.yaml:5:5: table "T": "id  character varying(255)" needs w: 213 or more`},
		{"  - table: { x: 20, y: 20, w: 216, title: T, rows: [[id, \"character varying(255)\"]] }", ""},
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[{ en: a, ja: とても長い名前 }, { en: a very long type name, ja: 型 }]] }", `s.yaml:5:5: table "T": "a  a very long type name" needs w: 198 or more`},
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: A very long row, ja: 短い }] }", `s.yaml:5:5: box "A very long row": "A very long row" needs w: 157 or more`},
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: AB, ja: あ, ko: 아주 긴 한국어 줄 }] }", `s.yaml:5:5: box "아주 긴 한국어 줄": "아주 긴 한국어 줄" (ko) needs w: 148 or more`},
		// Two languages other than English as wide: the first by tag.
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: A, ja: あいう, ko: 가나다 }] }", `s.yaml:5:5: box "あいう": "あいう" (ja) needs w: 71 or more`},
		// As wide as English, which is told: in the main row, the sub row, mixed widths and code.
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: ABCDEFGHIJKLMNOPQRSTUVWXY, ja: あいうえおかきくけこさしすせそた }] }", `s.yaml:5:5: box "ABCDEFGHIJKLMNOPQRSTUVWXY": "ABCDEFGHIJKLMNOPQRSTUVWXY" needs w: 240 or more`},
		{"  - box: { x: 20, y: 20, w: 40, rows: [[{ en: ABCDEFGHIJ, ja: あいうえおか }, sub]] }", `s.yaml:5:5: box "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 104 or more`},
		{"  - box: { x: 20, y: 20, w: 48, h: 60, rows: [A, { en: ABCDEFGHIJKLMNO, ja: あいうえおかきくけ }] }", `s.yaml:5:5: box "A": "ABCDEFGHIJKLMNO" needs w: 140 or more`},
		{"  - box: { x: 20, y: 20, w: 40, rows: [[{ en: ABCDEFGHIJ, ja: ABCDEあいう }, sub]] }", `s.yaml:5:5: box "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 104 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[{ en: " + codeEn + ", ja: " + codeJa + " }]] }", `s.yaml:5:5: table "T": "` + codeEn + `" needs w: 396 or more`},
		// A separator has no text, in a table sized by its rows too.
		{"  - table: { x: 20, y: 20, title: T, rows: [a, \"---\", b] }", ""},
		// U+2E80, the first CJK radical, is full-width.
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: AB, ja: \"\u2e80\u2e80\" }] }", "s.yaml:5:5: box \"\u2e80\u2e80\": \"\u2e80\u2e80\" (ja) needs w: 58 or more"},
		// Text exactly as wide as the room fits.
		{"  - box: { x: 20, y: 20, w: 104, h: 60, rows: [A, ABCDEFGHIJ] }", ""},
	} {
		if _, got := draw(t, head+c.parts+"\n", render("en")); got != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, got, c.want)
		}
	}
	if got, want := i18n.New("ja").Msg("diagram_too_narrow", `frame "F"`, "見出し", " (ja)", "192"), `frame "F": 「見出し」 (ja)には w: 192 以上が要ります`; got != want {
		t.Errorf("ja = %q, want %q", got, want)
	}
}

// An edge past the drawn shape of a box, by a hexagon's point or a rounded corner, does not run through it; 4 in
// from there, it does, told as before.
func TestEdgesPastShapes(t *testing.T) {
	t.Parallel()

	shapes := "width: 520\nheight: 500\ntitle: T\nparts:\n" +
		"  - box: { name: h, x: 100, y: 20, w: 200, h: 52, shape: hex, rows: [Decide] }\n" +
		"  - edge: { from: [95, 15], to: [108, 28], straight: true }\n" +
		"  - edge: { from: [310, 10], to: [296, 30], straight: true }\n" +
		"  - box: { name: s, x: 100, y: 110, w: 200, h: 80, shape: state, rows: [Running] }\n" +
		"  - edge: { from: [60, 105], to: [106, 118], straight: true }\n" +
		"  - box: { name: p, x: 100, y: 220, w: 200, h: 52, shape: pill, rows: [Start] }\n" +
		"  - edge: { from: [60, 215], to: [106, 228], straight: true }\n" +
		"  - box: { name: r, x: 100, y: 300, w: 200, h: 52, rows: [Save] }\n" +
		"  - edge: { from: [90, 290], to: [102, 302], straight: true }\n" +
		"  - table: { name: t, x: 100, y: 380, w: 200, title: T, rows: [[a]] }\n" +
		"  - edge: { from: [90, 370], to: [102, 382], straight: true }\n" +
		"  - bar: { name: b, x1: 100, x2: 300, y: 470 }\n" +
		"  - edge: { from: [90, 455], to: [101.3, 464.3], straight: true }\n" +
		"  - box: { name: tall, x: 400, y: 20, w: 60, h: 120, shape: pill, rows: [T] }\n" +
		"  - edge: { from: [390, 15], to: [405, 30], straight: true }\n" +
		"  - edge: { from: [130, 21], to: [270, 21], straight: true }\n" +
		"  - edge: { from: [60, 20], to: [100, 46], straight: true }\n"
	if _, got := draw(t, shapes, inspect); got != "" {
		t.Errorf("edges past the shapes = %q, want none of them told", got)
	}
	inward := strings.NewReplacer("to: [108, 28]", "to: [112, 32]", "to: [296, 30]", "to: [290, 34]",
		"to: [106, 118]", "to: [110, 121]", "to: [106, 228]", "to: [110, 231]", "to: [102, 302]", "to: [106, 306]",
		"to: [102, 382]", "to: [106, 386]", "to: [101.3, 464.3]", "to: [103, 466]", "to: [405, 30]", "to: [415, 60]",
		"from: [130, 21], to: [270, 21]", "from: [60, 46], to: [340, 46]").Replace(shapes)
	want := strings.Join([]string{
		`s.yaml:6:5: edge [95,15]→[112,32] runs through box "h"; move one of them, or go round it by via`,
		`s.yaml:7:5: edge [310,10]→[290,34] runs through box "h"; move one of them, or go round it by via`,
		`s.yaml:9:5: edge [60,105]→[110,121] runs through box "s"; move one of them, or go round it by via`,
		`s.yaml:11:5: edge [60,215]→[110,231] runs through box "p"; move one of them, or go round it by via`,
		`s.yaml:13:5: edge [90,290]→[106,306] runs through box "r"; move one of them, or go round it by via`,
		`s.yaml:15:5: edge [90,370]→[106,386] runs through table "t"; move one of them, or go round it by via`,
		`s.yaml:17:5: edge [90,455]→[103,466] runs through bar "b"; move one of them, or go round it by via`,
		`s.yaml:19:5: edge [390,15]→[415,60] runs through box "tall"; move one of them, or go round it by via`,
		`s.yaml:20:5: edge [60,46]→[340,46] runs through box "h"; move one of them, or go round it by via`,
	}, "\n")
	if _, got := draw(t, inward, inspect); got != want {
		t.Errorf("edges 4 in = %q, want %q", got, want)
	}
}

// An icon's bottom side is under its label, whichever way the edge runs and however wide or big they are; without a
// label it is the icon's. A side left out is chosen by the icon with its label. The label is in the way of any other
// run still. The last cases are what the rule costs and lets by (the LIMITs of onSide), kept here so that changing any
// of them is a choice.
func TestEdgesIconLabel(t *testing.T) {
	t.Parallel()
	const (
		head = "width: 800\nheight: 400\ntitle: T\nparts:\n"
		a    = "  - icon: { name: a, x: 100, y: 20, icon: aws/aws-lambda, label: a }\n"
		end  = "  - box: { name: b, x: 100, y: 160, w: 48, h: 30, rows: [b] }\n"
	)
	for _, c := range []struct {
		name, parts, want, refused string
	}{
		{name: "down", parts: a + end + "  - edge: { from: a, to: b }", want: "edge \"a\"→\"b\" M124,89 V160"},
		{name: "up", parts: a + end + "  - edge: { from: b, to: a }", want: "edge \"b\"→\"a\" M124,160 V89"},
		{name: "written", parts: a + end + "  - edge: { from: a, fromSide: bottom, to: b, toSide: top }", want: "edge \"a\"→\"b\" M124,89 V160"},
		{
			name:  "no label",
			parts: "  - icon: { name: a, x: 100, y: 20, icon: aws/aws-lambda }\n" + end + "  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M124,68 V160",
		},
		{
			name:  "wide label",
			parts: "  - icon: { name: a, x: 100, y: 20, icon: aws/aws-lambda, label: A long name of an icon }\n" + end + "  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M124,89 V160",
		},
		{
			name:  "size",
			parts: "  - icon: { name: a, x: 100, y: 20, size: 80, icon: aws/aws-lambda, label: a }\n" + "  - box: { name: b, x: 116, y: 180, w: 48, h: 30, rows: [b] }\n  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M140,121 V180",
		},
		{
			name:    "another edge at the label",
			parts:   a + "  - box: { name: c, x: 20, y: 70, w: 44, h: 30, rows: [c] }\n  - box: { name: d, x: 180, y: 70, w: 44, h: 30, rows: [d] }\n  - edge: { from: c, to: d }",
			refused: `edge "c"→"d" runs through label "a"; move one of them, or go round it by via`,
		},
		{
			name:    "a via between the icon and its label",
			parts:   a + "  - box: { name: b, x: 220, y: 100, w: 48, h: 30, rows: [b] }\n  - edge: { from: a, to: b, fromSide: bottom, toSide: left, via: [[124, 70], [190, 70], [190, 115]] }",
			refused: `edge "a"→"b" runs through label "a"; move one of them, or go round it by via`,
		},
		{
			name:  "no side, the other's centre as far across as down from the icon with its label",
			parts: a + "  - box: { name: b, x: 160, y: 70.5, w: 60, h: 100, rows: [b] }\n  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M148,44 H190 V70.5",
		},
		{
			name:  "a via level with the label",
			parts: a + "  - box: { name: b, x: 300, y: 80, w: 48, h: 30, rows: [b] }\n  - edge: { from: b, fromSide: top, to: a, via: [[324, 70], [200, 70], [200, 80]] }",
			want:  "edge \"b\"→\"a\" M324,80 V70 H200 V80 H174 V44 H148",
		},
		{
			name:    "no side, further down, the other's top beside the label",
			parts:   a + "  - box: { name: b, x: 160, y: 71, w: 60, h: 100, rows: [b] }\n  - edge: { from: a, to: b }",
			refused: `edge "a"→"b" runs through label "a"; move one of them, or go round it by via`,
		},
		{
			name:    "the same, a side floe chooses moved by the label's place",
			parts:   a + "  - box: { name: b, x: 160, y: 72, w: 60, h: 100, rows: [b] }\n  - edge: { from: a, to: b }",
			refused: `edge "a"→"b" runs through box "b"; move one of them, or go round it by via`,
		},
		{
			name:  "the same, further off, the bottom side",
			parts: a + "  - box: { name: b, x: 160, y: 106, w: 60, h: 30, rows: [b] }\n  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M124,89 V97.5 H190 V106",
		},
		{
			name:  "above, the right side",
			parts: "  - icon: { name: a, x: 100, y: 200, icon: aws/aws-lambda, label: a }\n  - box: { name: b, x: 160, y: 154, w: 60, h: 30, rows: [b] }\n  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M148,224 H154 V169 H160",
		},
		{
			name:  "another edge across the gap",
			parts: a + "  - box: { name: c, x: 20, y: 55, w: 44, h: 30, rows: [c] }\n  - box: { name: d, x: 180, y: 55, w: 44, h: 30, rows: [d] }\n  - edge: { from: c, to: d }",
			want:  "edge \"c\"→\"d\" M64,70 H180",
		},
		{
			name:  "the same, a side written",
			parts: a + "  - box: { name: b, x: 160, y: 71, w: 60, h: 100, rows: [b] }\n  - edge: { from: a, fromSide: right, to: b }",
			want:  "edge \"a\"→\"b\" M148,44 H190 V71",
		},
		{
			name:    "the sides written, the other's side above the label's bottom",
			parts:   a + "  - box: { name: b, x: 300, y: 55, w: 60, h: 30, rows: [b] }\n  - edge: { from: a, fromSide: bottom, to: b, toSide: left }",
			refused: `edge "a"→"b" runs through label "a"; move one of them, or go round it by via`,
		},
		{
			name:    "the same, the other way",
			parts:   a + "  - box: { name: b, x: 300, y: 55, w: 60, h: 30, rows: [b] }\n  - edge: { from: b, fromSide: left, to: a, toSide: bottom }",
			refused: `edge "b"→"a" runs through label "a"; move one of them, or go round it by via`,
		},
		{
			name:  "the other's top 2 above the label's bottom, the run in its edge",
			parts: a + "  - box: { name: b, x: 160, y: 87, w: 60, h: 100, rows: [b] }\n  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M124,89 V88 H190 V87",
		},
		{
			name:  "the other's top at the label's bottom",
			parts: a + "  - box: { name: b, x: 160, y: 89, w: 60, h: 100, rows: [b] }\n  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M124,89 H190",
		},
		{
			name:  "along the label's bottom",
			parts: a + "  - box: { name: b, x: 300, y: 74, w: 60, h: 30, rows: [b] }\n  - edge: { from: a, fromSide: bottom, to: b, toSide: left }",
			want:  "edge \"a\"→\"b\" M124,89 H300",
		},
		{
			name: "another edge along the label's bottom",
			parts: a + "  - box: { name: c, x: 20, y: 74, w: 44, h: 30, rows: [c] }\n  - box: { name: d, x: 200, y: 74, w: 44, h: 30, rows: [d] }\n" +
				"  - box: { name: e, x: 100, y: 200, w: 48, h: 30, rows: [e] }\n  - edge: { from: c, to: d }\n  - edge: { from: a, to: e }",
			want: "edge \"a\"→\"e\" M124,89 V200",
		},
		{
			name:  "a part 1 below the label",
			parts: a + "  - box: { name: b, x: 100, y: 90, w: 48, h: 30, rows: [b] }\n  - edge: { from: a, to: b }",
			want:  "edge \"a\"→\"b\" M124,89 V90",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, mistakes := draw(t, head+c.parts+"\n", inspect)
			if c.refused != "" {
				if !strings.Contains(mistakes, c.refused) {
					t.Errorf("told %q, want %q", mistakes, c.refused)
				}
				return
			}
			if mistakes != "" || !strings.Contains(got, c.want) {
				t.Errorf("printed %s, told %q, want %q", got, mistakes, c.want)
			}
		})
	}
}

// A straight edge joins its points as written, a diagonal too; others turn at right angles.
func TestEdges(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		parts, want string
	}{
		{"  - edge: { from: [10, 10], via: [[80, 70]], to: [150, 10], straight: true, end: none }", `d="M10,10 L80,70 L150,10"`},
		{"  - edge: { from: [10, 10], to: [150, 70], end: none }", `d="M10,10 H150 V70"`},
		{
			"  - box: { name: a, x: 10, y: 10, w: 60, h: 40, rows: [a] }\n  - box: { name: b, x: 200, y: 100, w: 60, h: 40, rows: [b] }\n  - edge: { from: a, to: b }",
			`d="M70,30 H135 V120 H200"`,
		},
		{
			"  - box: { name: a, x: 10, y: 10, w: 60, h: 40, rows: [a] }\n  - box: { name: b, x: 200, y: 100, w: 60, h: 40, rows: [b] }\n  - edge: { from: a, fromSide: bottom, to: b, toSide: left }",
			`d="M40,50 V120 H200"`,
		},
		{
			"  - dot: { name: d, cx: 40, cy: 40 }\n  - box: { name: b, x: 200, y: 150, w: 60, h: 40, rows: [b] }\n  - edge: { from: d, to: b, toSide: left }",
			`d="M40,47 V170 H200"`,
		},
		{
			"  - box: { name: a, x: 100, y: 20, w: 80, h: 29, rows: [a] }\n  - box: { name: b, x: 100, y: 120, w: 80, h: 29, rows: [b] }\n  - edge: { from: a, fromSide: left, to: b, toSide: left }",
			`d="M100,34.5 H80 V134.5 H100"`,
		},
		{"  - box: { x: 100, y: 10, origin: centre, rows: [abc] }", `<rect x="68" y="10" width="64" height="29" rx="8"/>`},
	} {
		got, mistakes := draw(t, head+c.parts+"\n", render("en"))
		if mistakes != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s\n  drew %s %q\n  want %q", c.parts, got, mistakes, c.want)
		}
	}
}

// An end named without a side, its via next to it outside it, takes the side towards that via, or when the via is off
// both ways, the side across the way the edge came; the last run meets such a side, and a side written with the via
// outside it, square, with two turns halfway when one turn would run back. The rest take the side facing the other
// end: a via on the end or in it, a via off both ways at a from end or on a straight edge, a point, a dot.
// An end a graph or a tree laid out, with no side written, keeps off the sides their edges end at.
func TestEdgesHandSides(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		spec, want string
	}{
		// graph box: right, not the top a→b ends at
		{"width: 200\nheight: 160\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n      edges:\n        - { from: a, to: b }\n  - edge: { from: [100, 0], via: [[100, 60]], to: b }\n", `d="M100,0 V60 V87.5 H68"`},
		// laid out right: the bottom, not the left
		{"width: 260\nheight: 160\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      direction: right\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n      edges:\n        - { from: a, to: b }\n  - edge: { from: [0, 100], via: [[60, 100]], to: b }\n", `d="M0,100 H60 H116 V68"`},
		// a box in a lane
		{"width: 400\nheight: 220\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      lanes:\n        - { name: l1, label: L1, nodes: [{ box: { name: a, rows: [a] } }] }\n        - { name: l2, label: L2, nodes: [{ box: { name: b, rows: [b] } }] }\n      edges:\n        - { from: a, to: b }\n  - edge: { from: [300, 0], via: [[300, 100]], to: b }\n", `d="M300,0 V100 V129.5 H176"`},
		// a tree's child: right, not its top
		{"width: 260\nheight: 200\ntitle: T\nparts:\n  - tree:\n      x: 90\n      y: 20\n      root: { name: r, rows: [r], children: [{ name: c, rows: [c] }] }\n  - edge: { from: [200, 0], via: [[200, 80]], to: c }\n", `d="M200,0 V80 V103.5 H114"`},
		// a tree's root keeps its top
		{"width: 260\nheight: 200\ntitle: T\nparts:\n  - tree:\n      x: 44\n      y: 50\n      root: { name: r, rows: [r], children: [{ name: c, rows: [c] }] }\n  - edge: { from: [200, 0], via: [[200, 40]], to: r }\n", `d="M200,0 V40 H44 V50"`},
		// without via, from the other end
		{"width: 260\nheight: 240\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 100\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n      edges:\n        - { from: a, to: b }\n  - box: { name: h, x: 60, y: 10, w: 60, h: 30, rows: [h] }\n  - edge: { from: h, to: b }\n", `d="M90,40 V167.5 H68"`},
		// a group's frame: right, not its top
		{"width: 300\nheight: 220\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - group: { name: g, label: G, nodes: [{ box: { name: b, rows: [b] } }] }\n      edges:\n        - { from: a, to: g }\n  - edge: { from: [200, 0], via: [[200, 60]], to: g }\n", `d="M200,0 V60 V120.5 H134"`},
		// two of the graph's edges end at b's top
		{"width: 300\nheight: 200\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: c, rows: [c] }\n        - box: { name: b, rows: [b] }\n      edges:\n        - { from: a, to: b }\n        - { from: c, to: b }\n  - edge: { from: [250, 0], via: [[250, 60]], to: b }\n", `d="M250,0 V60 V87.5 H116"`},
		// a group's frame in the first rank keeps its top, which no edge of the graph ends at
		{"width: 300\nheight: 240\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 40\n      nodes:\n        - group: { name: g, label: G, nodes: [{ box: { name: b, rows: [b] } }] }\n        - box: { name: a, rows: [a] }\n      edges:\n        - { from: g, to: a }\n  - edge: { from: [250, 0], via: [[250, 20]], to: g }\n", `d="M250,0 V20 H68 V40"`},
		// a lane's outer side, which no edge of the graph crosses, is left
		{"width: 400\nheight: 220\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      lanes:\n        - { name: l1, label: L1, nodes: [{ box: { name: a, rows: [a] } }] }\n        - { name: l2, label: L2, nodes: [{ box: { name: b, rows: [b] } }] }\n      edges:\n        - { from: a, to: b }\n  - edge: { from: [0, 200], via: [[10, 200]], to: l1 }\n", `d="M0,200 H10 V88 H20"`},
		// across first: right, though the top is left too
		{"width: 300\nheight: 320\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n      edges:\n        - { from: a, to: b }\n  - edge: { from: a, via: [[100, 5]], to: [200, 300] }\n", `d="M68,34.5 H100 V5 H200 V300"`},
		// a leaf keeps its bottom
		{"width: 260\nheight: 320\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n      edges:\n        - { from: a, to: b }\n  - edge: { from: b, via: [[100, 140]], to: [100, 300] }\n", `d="M44,102 V140 H100 V300"`},
		// no side left but the level edge's: the top kept (LIMIT in clear)
		{"width: 400\nheight: 320\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: c, rows: [c] }\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n        - box: { name: d, rows: [d] }\n      edges:\n        - { from: c, to: a }\n        - { from: a, to: d }\n        - { from: a, to: b, level: true }\n  - edge: { from: [80, 0], via: [[80, 60]], to: a }\n", `d="M80,0 V60 H44 V73"`},
		// a's bottom, where a→d leaves, is not left though c→a ends at its top: the level edge's side kept, and told.
		{
			"width: 400\nheight: 320\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: c, rows: [c] }\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n        - box: { name: d, rows: [d] }\n      edges:\n        - { from: c, to: a }\n        - { from: a, to: d }\n        - { from: a, to: b, level: true }\n  - edge: { from: [200, 140], via: [[80, 140]], to: a }\n",
			"s.yaml:17:5: edge \"a\"→\"b\" crosses edge [200,140]→\"a\" at 80,94; move one of them, or add crossing: true to one if it cannot be avoided",
		},
		// No side left with the via outside it: the side kept, crossing the loop, is told.
		{
			"width: 260\nheight: 200\ntitle: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - box: { name: a, rows: [a] }\n        - box: { name: b, rows: [b] }\n      edges:\n        - { from: a, to: b }\n        - { from: b, to: b }\n  - edge: { from: [200, 0], via: [[200, 100]], to: b }\n",
			"s.yaml:14:5: edge \"b\"→\"b\" crosses edge [200,0]→\"b\" at 92,88; move one of them, or add crossing: true to one if it cannot be avoided",
		},
	} {
		got, mistakes := draw(t, c.spec, render("en"))
		if mistakes != c.want && (mistakes != "" || !strings.Contains(got, c.want)) {
			t.Errorf("%s\n  drew %s %s\n  want %s", c.spec, got, mistakes, c.want)
		}
	}
}

// A point of via with one axis takes the other from the point before, the first from the centre of the edge's start.
func TestEdgeViaAxis(t *testing.T) {
	t.Parallel()

	const head = "width: 500\nheight: 320\ntitle: T\nparts:\n  - box: { name: a, x: 20, y: 20, w: 60, h: 40, rows: [a] }\n" +
		"  - box: { name: b, x: 200, y: 200, w: 60, h: 40, rows: [b] }\n  - box: { name: c, x: 20, y: 200, w: 60, h: 40, rows: [c] }\n"
	const row = "  - table: { x: 300, y: 20, w: 100, title: T, rows: [{ name: r, cells: [x, y] }] }\n"
	const dot = "  - dot: { name: d, cx: 150, cy: 40 }\n"
	for _, c := range []struct {
		parts, want string
	}{
		{"  - edge: { from: a, to: b, fromSide: right, toSide: left, via: [{ x: 140 }] }", "edge \"a\"→\"b\" M80,40 H140 V220 H200"},
		{"  - edge: { from: a, to: b, via: [{ y: 120 }] }", "edge \"a\"→\"b\" M50,60 V120 H230 V200"},
		{"  - edge: { from: a, to: b, fromSide: right, toSide: right, via: [{ x: 300 }] }", "edge \"a\"→\"b\" M80,40 H300 V220 H260"},
		{"  - edge: { from: a, to: b, via: [{ x: 140 }, { y: 150 }, { x: 180 }] }", "edge \"a\"→\"b\" M80,40 H140 V150 H180 V220 H200"},
		{"  - edge: { from: [20, 150], to: b, via: [{ x: 100 }] }", "edge [20,150]→\"b\" M20,150 H100 V220 H200"},
		{dot + "  - edge: { from: d, to: b, via: [{ y: 120 }] }", "edge \"d\"→\"b\" M150,40 V120 H230 V200"},
		{row + "  - edge: { from: r, to: [20, 100], via: [{ x: 450 }] }", "edge \"r\"→[20,100] M400,61 H450 V100 H20"},
		{"  - edge: { from: a, to: b, straight: true, via: [{ x: 140 }] }", "edge \"a\"→\"b\" M80,40 H140 L200,220"},
		{"  - edge: { from: a, to: b, via: [{ x: 140, y: 100 }] }", "edge \"a\"→\"b\" M80,40 H140 V100 H230 V200"},
		{"  - edge: { from: c, to: [10, 10], via: [{ x: 0 }] }", "edge \"c\"→[10,10] M20,220 H0 V10 H10"},
		{"  - edge: { from: b, to: [10, 10], via: [{ y: 0 }] }", "edge \"b\"→[10,10] M230,200 V0 H10 V10"},
		// The centre of an icon is of its image, not with its label.
		{"  - icon: { name: i, icon: aws/aws-lambda, x: 20, y: 100, label: L }\n  - edge: { from: i, to: b, fromSide: right, via: [{ x: 150 }] }", "edge \"i\"→\"b\" M68,124 H150 V220 H200"},
		{"  - bar: { name: s, x1: 300, x2: 400, y: 40 }\n  - edge: { from: s, to: b, via: [{ y: 120 }] }", "edge \"s\"→\"b\" M350,47 V120 H230 V200"},
		{"  - frame: { name: f, x: 300, y: 20, w: 100, h: 100, label: F }\n  - edge: { from: f, to: b, fromSide: bottom, via: [{ y: 150 }] }", "edge \"f\"→\"b\" M350,120 V150 H230 V200"},

		// The first point written on the axis the edge leaves by. A point on the side, as a first point left to the side
		// would be, runs along the box's edge and is not told.
		{"  - edge: { from: a, to: b, fromSide: top, toSide: left, via: [[150, 20]] }", "edge \"a\"→\"b\" M50,20 H150 V220 H200"},
		{"  - edge: { from: a, to: b, fromSide: top, toSide: left, via: [{ y: 10 }, { x: 150 }] }", "edge \"a\"→\"b\" M50,20 V10 H150 V220 H200"},
		{"  - edge: { from: a, to: b, fromSide: right, toSide: top, via: [{ x: 150 }] }", "edge \"a\"→\"b\" M80,40 H150 H230 V200"},

		// What may not be left out of via, as the docs tell it: the edge runs another way, and nothing is told.
		// The first point of an edge with no side written.
		{"  - edge: { from: a, to: b, toSide: right, via: [[50, 10], [300, 10], [300, 220]] }", "edge \"a\"→\"b\" M50,20 V10 H300 V220 H260"},
		{"  - edge: { from: a, to: b, toSide: right, via: [[300, 10]] }", "edge \"a\"→\"b\" M80,40 H300 V10 H280 V220 H260"},
		{"  - edge: { from: a, to: b, fromSide: top, toSide: right, via: [[300, 10]] }", "edge \"a\"→\"b\" M50,20 V10 H300 V220 H260"},
		{"  - edge: { from: a, to: b, toSide: right, via: [{ y: 10 }, { x: 300 }] }", "edge \"a\"→\"b\" M50,20 V10 H300 V220 H260"},
		// The first point of an edge leaving a point or a dot up or down.
		{"  - edge: { from: [150, 40], to: b, toSide: top, via: [[150, 120], [230, 120]] }", "edge [150,40]→\"b\" M150,40 V120 H230 V200"},
		{"  - edge: { from: [150, 40], to: b, toSide: top, via: [[230, 120]] }", "edge [150,40]→\"b\" M150,40 H230 V120 V200"},
		{"  - edge: { from: [150, 40], to: b, toSide: top, via: [{ y: 120 }, { x: 230 }] }", "edge [150,40]→\"b\" M150,40 V120 H230 V200"},
		{dot + "  - edge: { from: d, to: b, toSide: top, via: [[230, 120]] }", "edge \"d\"→\"b\" M150,40 H230 V120 V200"},
		{dot + "  - edge: { from: d, to: b, toSide: top, via: [{ y: 120 }, { x: 230 }] }", "edge \"d\"→\"b\" M150,40 V120 H230 V200"},
		// A turn the other way than the edge came.
		{
			"  - edge: { from: a, to: b, fromSide: right, toSide: top, via: [[130, 100], [300, 100], [300, 150]] }",
			"edge \"a\"→\"b\" M80,40 H130 V100 H300 V150 H230 V200",
		},
		{
			"  - edge: { from: a, to: b, fromSide: right, toSide: top, via: [[130, 100], [300, 150]] }",
			"edge \"a\"→\"b\" M80,40 H130 V100 V150 H300 V175 H230 V200",
		},
		// The last point: with no via the edge shares the side.
		{
			"  - edge: { from: a, to: b, toSide: left, via: [{ x: 140 }] }\n  - edge: { from: c, to: b }",
			"edge \"a\"→\"b\" M80,40 H140 V220 H200\nedge \"c\"→\"b\" M80,220 H200",
		},
		{
			"  - edge: { from: a, to: b, toSide: left }\n  - edge: { from: c, to: b }",
			"edge \"a\"→\"b\" M80,40 H140 V213.33 H200\nedge \"c\"→\"b\" M80,226.67 H200",
		},
	} {
		out, mistakes := draw(t, head+c.parts+"\n", inspect)
		if got := edgeLines(out); mistakes != "" || got != c.want {
			t.Errorf("%s\n  drew %q %q\n  want %q", c.parts, got, mistakes, c.want)
		}
	}

	// A first point on the other axis, or where a part stands, is told as any via is; a name unknown too.
	for _, c := range []struct {
		parts, want string
	}{
		{"  - edge: { from: a, to: b, fromSide: top, toSide: left, via: [{ x: 150 }] }", `s.yaml:8:5: edge "a"→"b" runs through box "a"; move one of them, or go round it by via`},
		{"  - edge: { from: a, to: b, fromSide: right, toSide: top, via: [{ y: 150 }] }", `s.yaml:8:5: edge "a"→"b" runs through box "a"; move one of them, or go round it by via`},
		{"  - edge: { from: a, to: b, via: [{ x: 50 }] }", "s.yaml:8:5: edge \"a\"→\"b\" runs through box \"a\"; move one of them, or go round it by via\ns.yaml:8:5: edge \"a\"→\"b\" runs through box \"c\"; move one of them, or go round it by via"},
		{"  - edge: { from: nobody, to: b, via: [{ x: 50 }] }", `s.yaml:8:5: edge: no part, row or node named "nobody"`},
	} {
		if _, got := draw(t, head+c.parts+"\n", inspect); got != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, got, c.want)
		}
	}

	// Moving what the edge goes to: one axis leaves the way chosen, a point copying the other axis bends.
	moved := func(y int, via string) string {
		src := "width: 900\nheight: 700\ntitle: T\nparts:\n  - box: { name: api, x: 616, y: 626, w: 48, h: 40, rows: [a] }\n" +
			"  - box: { name: writer, x: 790, y: " + strconv.Itoa(y) + ", w: 48, h: 40, rows: [w] }\n" +
			"  - edge: { from: api, to: writer, fromSide: right, toSide: left, via: [" + via + "] }\n"
		got, mistakes := draw(t, src, inspect)
		if mistakes != "" {
			t.Fatalf("%s: %s", via, mistakes)
		}
		return got
	}
	for _, c := range []struct {
		y         int
		via, want string
	}{
		{366, "{ x: 710 }", "edge \"api\"→\"writer\" M664,646 H710 V386 H790"},
		{342, "{ x: 710 }", "edge \"api\"→\"writer\" M664,646 H710 V362 H790"},
		{366, "[710, 374]", "edge \"api\"→\"writer\" M664,646 H710 V374 H750 V386 H790"},
		{342, "[710, 374]", "edge \"api\"→\"writer\" M664,646 H710 V374 V362 H790"},
	} {
		if got := edgeLines(moved(c.y, c.via)); got != c.want {
			t.Errorf("writer at %d by %s\n  drew %q\n  want %q", c.y, c.via, got, c.want)
		}
	}

	// The id made from the spec: [x, y] as it was before a point had axes, { x, y } as [x, y], and one axis as itself.
	idOf := func(via string) string {
		got, mistakes := draw(t, head+"  - edge: { from: a, to: b, via: ["+via+"] }\n", render("en"))
		if mistakes != "" {
			t.Fatalf("%s: %s", via, mistakes)
		}
		return regexp.MustCompile(`dg1-[0-9a-f]+`).FindString(got)
	}
	if got := idOf("[140, 100]"); got != "dg1-12b45c" {
		t.Errorf("id of [x, y] = %q, want it as before", got)
	}
	if got := idOf("{ x: 140, y: 100 }"); got != "dg1-12b45c" {
		t.Errorf("id of { x, y } = %q, want that of [x, y]", got)
	}
	// The same way, written otherwise.
	if x, y := idOf("{ x: 140 }"), idOf("{ y: 100 }"); x == y || x == idOf("[140, 40]") || y == idOf("[50, 100]") {
		t.Errorf("ids of one axis = %q, %q, want them of what is written", x, y)
	}
}

// edgeLines is what inspect tells of edges alone, a line each.
func edgeLines(got string) string {
	var lines []string
	for line := range strings.Lines(got) {
		if strings.HasPrefix(line, "edge ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}
	return strings.Join(lines, "\n")
}

func TestEdgesVia(t *testing.T) {
	t.Parallel()

	const head = "width: 560\nheight: 340\ntitle: T\nparts:\n"
	const c01 = "  - box: { name: c01, x: 60, y: 234, w: 88, h: 36, rows: [[render, main code]] }\n"
	const ab = "  - box: { name: a, x: 40, y: 40, w: 100, h: 40, rows: [a] }\n  - box: { name: b, x: 300, y: 140, w: 100, h: 40, rows: [b] }\n"
	const page = "  - table: { x: 300, y: 40, w: 200, title: Page, rows: [{ name: slug, cells: [slug, PK] }, [title, text]] }\n"
	for _, c := range []struct {
		parts, want string
	}{
		// Sides taken from the via, and met square.
		{c01 + "  - edge: { from: [44, 154], via: [[44, 252]], to: c01 }", `d="M44,154 V252 H60"`},
		{c01 + "  - edge: { from: [10, 20], via: [[10, 240]], to: c01 }", `d="M10,20 V240 V252 H60"`},
		{c01 + "  - edge: { from: [0, 150], via: [[90, 150]], to: c01 }", `d="M0,150 H90 H104 V234"`},
		{c01 + "  - edge: { from: [10, 200], via: [[30, 200]], to: c01 }", `d="M10,200 H30 V252 H60"`},
		{c01 + "  - edge: { from: [10, 100], via: [[10, 200]], to: c01 }", `d="M10,100 V200 H104 V234"`},
		{c01 + "  - edge: { from: [0, 150], via: [[200, 150]], to: c01 }", `d="M0,150 H200 V252 H148"`},
		{c01 + "  - edge: { from: [10, 20], via: [[104, 20], [200, 20], [200, 252]], to: c01 }", `d="M10,20 H104 H200 V252 H148"`},
		{c01 + "  - edge: { from: [0, 150], via: [[120, 150]], to: c01 }", `d="M0,150 H120 V192 H104 V234"`},
		{c01 + "  - edge: { from: [10, 100], via: [[10, 262]], to: c01 }", `d="M10,100 V262 H35 V252 H60"`},
		{c01 + "  - edge: { from: [10, 100], via: [[10, 200]], to: c01, toSide: top }", `d="M10,100 V200 H104 V234"`},
		{c01 + "  - edge: { from: [10, 100], via: [[10, 100]], to: c01 }", `d="M10,100 H10 V252 H60"`},
		{ab + "  - edge: { from: a, via: [[90, 160], [200, 160]], to: b }", `d="M90,80 V160 H200 H300"`},
		{
			"  - box: { name: a, x: 40, y: 40, w: 100, h: 40, rows: [a] }\n  - box: { name: b, x: 300, y: 200, w: 100, h: 40, rows: [b] }\n" +
				"  - edge: { from: a, via: [[90, 220]], to: b, straight: true }",
			`d="M90,80 V220 H300"`,
		},
		{
			page + "  - box: { name: a, x: 40, y: 180, w: 100, h: 40, rows: [a] }\n  - edge: { from: a, via: [[530, 200], [530, 81]], to: slug }",
			`d="M140,200 H530 V81 H500"`,
		},
		{
			"  - table: { name: t, x: 300, y: 60, w: 200, title: Page, rows: [[slug, PK], [title, text]] }\n" +
				"  - edge: { from: [10, 20], via: [[310, 20]], to: t }",
			`d="M10,20 H310 H400 V60"`,
		},
		// Sides facing the other end, sides written with the via not outside them, points and dots: through goes on.
		// A via level with the side, come to going away from it, runs back along that run (LIMIT in enter).
		{c01 + "  - edge: { from: [50, 300], via: [[50, 252], [20, 252]], to: c01 }", `d="M50,300 V252 H20 H60"`},
		{page + "  - edge: { from: [530, 160], via: [[530, 120]], to: slug }", `d="M530,160 V120 V81 H500"`},
		{c01 + "  - edge: { from: [300, 100], via: [[200, 100]], to: c01, toSide: left }", `d="M300,100 H200 H60 V252"`},
		{c01 + "  - edge: { from: [60, 20], via: [[60, 100]], to: c01, toSide: left }", `d="M60,20 V100 V252"`},
		{c01 + "  - edge: { from: [0, 100], via: [[60, 100]], to: c01 }", `d="M0,100 H60 H104 V234"`},
		{c01 + "  - edge: { from: [10, 240], via: [[60, 240]], to: c01 }", `d="M10,240 H60 V252"`},
		{c01 + "  - edge: { from: [80, 100], via: [[80, 234]], to: c01 }", `d="M80,100 V234 H104"`},
		{c01 + "  - edge: { from: [10, 200], via: [[30, 200], [30, 200]], to: c01 }", `d="M10,200 H30 H30 V252 H60"`},
		{"  - edge: { from: [0, 0], via: [[50, 0]], to: [100, 100] }", `d="M0,0 H50 H100 V100"`},
		{"  - dot: { name: d, cx: 300, cy: 100 }\n  - edge: { from: [10, 20], via: [[300, 20]], to: d }", `d="M10,20 H300 V92"`},
		// At a via, the turn towards the next point off line takes the other way first when the way the edge came would run
		// back along it.
		{"  - box: { name: a, x: 40, y: 40, w: 100, h: 40, rows: [a] }\n  - edge: { from: a, via: [[200, 300]], to: [400, 100] }", `d="M140,60 H200 V300 H400 V100"`},
		{"  - box: { name: a, x: 40, y: 40, w: 100, h: 40, rows: [a] }\n  - edge: { from: a, via: [[200, 20]], to: [100, 0] }", `d="M90,40 V20 H200 V0 H100"`},
		{"  - box: { name: a, x: 40, y: 40, w: 100, h: 40, rows: [a] }\n  - edge: { from: a, via: [[200, 20], [300, 20]], to: [250, 100] }", `d="M140,60 H200 V20 H300 V100 H250"`},
		{"  - box: { name: a, x: 40, y: 40, w: 100, h: 40, rows: [a] }\n  - edge: { from: a, via: [[200, 20]], to: [180, 100] }", `d="M140,60 H200 V20 H180 V100"`},
		{"  - table: { x: 40, y: 40, w: 100, title: T, rows: [{ name: r, cells: [x, y] }] }\n  - edge: { from: r, via: [[200, 20]], to: [300, 100] }", `d="M140,81 H200 V20 H300 V100"`},
		{"  - box: { name: a, x: 40, y: 40, w: 100, h: 40, rows: [a] }\n  - edge: { from: a, via: [[200, 120]], to: [300, 180] }", `d="M140,60 H200 V120 V180 H300"`},
		{
			"  - box: { name: b, x: 300, y: 100, w: 100, h: 40, rows: [b] }\n  - edge: { from: [10, 20], via: [[200, 300]], to: b, straight: true }",
			`d="M10,20 L200,300 L300,120"`,
		},
	} {
		got, mistakes := draw(t, head+c.parts+"\n", render("en"))
		if mistakes != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s\n  drew %s %q\n  want %q", c.parts, got, mistakes, c.want)
		}
	}

	// A via in a frame leaves the side facing the other end, and the edge runs back through a: a side with the via not
	// outside it is met the way the edge was going (LIMIT in enter), b's top too. A from end with the via behind the side
	// facing the other end starts through a (LIMIT in facing). Turning the other way first at a via runs through k
	// (LIMIT in through). Both ends unknown are told.
	for _, c := range []struct {
		parts, want string
	}{
		{
			"  - frame: { name: f, x: 200, y: 40, w: 280, h: 200, label: F }\n  - box: { name: a, x: 260, y: 120, w: 100, h: 40, rows: [a] }\n" +
				"  - edge: { from: a, via: [[420, 140], [420, 200]], to: f }",
			`s.yaml:7:5: edge "a"→"f" runs through box "a"; move one of them, or go round it by via`,
		},
		{
			"  - box: { name: b, x: 300, y: 100, w: 100, h: 40, rows: [b] }\n  - edge: { from: [450, 300], via: [[500, 300]], to: b, toSide: top }",
			`s.yaml:6:5: edge [450,300]→"b" runs through box "b"; move one of them, or go round it by via`,
		},
		{
			"  - box: { name: a, x: 40, y: 140, w: 100, h: 40, rows: [a] }\n  - edge: { from: a, via: [[200, 220]], to: [210, 0] }",
			`s.yaml:6:5: edge "a"→[210,0] runs through box "a"; move one of them, or go round it by via`,
		},
		{
			"  - box: { name: k, x: 180, y: 30, w: 40, h: 40, rows: [[k, sub]] }\n  - edge: { from: [0, 0], via: [[200, 0]], to: [100, 100] }",
			`s.yaml:6:5: edge [0,0]→[100,100] runs through box "k"; move one of them, or go round it by via`,
		},
		{
			"  - edge: { from: nope1, via: [[30, 200]], to: nope2 }",
			"s.yaml:5:5: edge: no part, row or node named \"nope1\"\ns.yaml:5:5: edge: no part, row or node named \"nope2\"",
		},
	} {
		if _, mistakes := draw(t, head+c.parts+"\n", render("en")); mistakes != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, mistakes, c.want)
		}
	}
}

// A label with in stands on a circle's colour and has no box, placed by floe or at x and y, unless it says plain:
// false; one free of in has its box unless it says plain.
func TestVennLabelsBare(t *testing.T) {
	t.Parallel()
	src := "width: 300\nheight: 300\ntitle: T\nparts:\n  - circle: { name: a, cx: 150, cy: 150, r: 100 }\n" +
		"  - label: { text: placed, in: [a] }\n  - label: { text: put, in: [a], x: 150, y: 200 }\n" +
		"  - label: { text: boxed, in: [a], x: 150, y: 100, plain: false }\n" +
		"  - label: { text: free, x: 150, y: 280 }\n  - label: { text: bare, x: 40, y: 20, plain: true }\n"
	got, mistakes := draw(t, src, render("en"))
	if mistakes != "" || strings.Count(got, `class="label-bg"`) != 2 {
		t.Errorf("drew %s %s\n  want two label-bg, boxed's and free's", got, mistakes)
	}
}

// Placing the labels of a region tries a bounded number of points within the canvas, however large the circle,
// on a grid the canvas cuts but does not move; a region all outside the canvas sticks out.
func TestVennBounded(t *testing.T) {
	t.Parallel()

	src := head + "  - circle: { name: c, cx: 200, cy: 100, r: 40000 }\n  - label: { text: x, in: [c] }\n"
	if _, got := draw(t, src, render("en")); !strings.Contains(got, "sticks out") {
		t.Errorf("a huge circle = %q, want it to stick out, told at once", got)
	}
	src = "width: 400\nheight: 300\ntitle: T\nparts:\n  - circle: { name: a, cx: 90, cy: 150, r: 101 }\n" +
		"  - circle: { name: b, cx: 250, cy: 150, r: 101 }\n  - label: { text: xxxx, in: [a, b] }\n"
	// The circles stick out themselves; the label finds its room in the canvas.
	if _, got := draw(t, src, render("en")); strings.Contains(got, `label "xxxx"`) {
		t.Errorf("a region in the canvas of circles out of it = %q, want the label placed", got)
	}
	src = head + "  - circle: { name: c, cx: 900, cy: 100, r: 50 }\n  - label: { text: x, in: [c] }\n"
	if _, got := draw(t, src, render("en")); !strings.Contains(got, `s.yaml:6:5: label "x" sticks out of the 400x200 canvas`) {
		t.Errorf("a region out of the canvas = %q, want the label to stick out", got)
	}
	// The region is where the circles of in all are, in no order of in.
	two := head + "  - circle: { name: a, cx: 300, cy: 100, r: 120 }\n  - circle: { name: b, cx: 500, cy: 100, r: 120 }\n"
	_, ab := draw(t, two+"  - label: { text: x, in: [a, b] }\n", render("en"))
	_, ba := draw(t, two+"  - label: { text: x, in: [b, a] }\n", render("en"))
	if ab != ba || !strings.Contains(ab, `label "x" sticks out`) {
		t.Errorf("in: [a, b] = %q and in: [b, a] = %q, want one, the label sticking out", ab, ba)
	}
	apart := head + "  - circle: { name: a, cx: 60, cy: 100, r: 40 }\n  - circle: { name: b, cx: 300, cy: 100, r: 40 }\n" +
		"  - label: { text: x, in: [a, b] }\n"
	if _, got := draw(t, apart, render("en")); !strings.Contains(got, `label "x" finds no room`) {
		t.Errorf("circles apart = %q, want no room", got)
	}
	// Far from 0, a label is placed.
	far := "width: 1000000\nheight: 300\ntitle: T\nparts:\n" +
		"  - circle: { name: a, cx: 999000, cy: 150, r: 100 }\n  - label: { text: x, in: [a] }\n"
	if got, mistakes := draw(t, far, inspect); mistakes != "" || !strings.Contains(got, `label "x" 998991.4,141`) {
		t.Errorf("a region far from 0 = %s %s, want the label placed", got, mistakes)
	}
	// A region that only touches the canvas has no room in it and sticks out; one with some of the canvas looks for
	// room there.
	for _, tt := range []struct{ circle, want string }{
		{"cx: -100.5, cy: 100, r: 100.5", `s.yaml:6:5: label "x" sticks out of the 400x200 canvas`},
		{"cx: 0.5, cy: 100, r: 1.5", `s.yaml:6:5: label "x" finds no room inside c`},
	} {
		src := head + "  - circle: { name: c, " + tt.circle + " }\n  - label: { text: x, in: [c] }\n"
		if _, got := draw(t, src, render("en")); !strings.Contains(got, tt.want) {
			t.Errorf("a circle of %s = %q, want %q", tt.circle, got, tt.want)
		}
	}
}

// A width or height left out reaches as far as what is drawn, and 20 more; one written stays. The canvas still
// starts at 0, 0, and what cannot be placed in it is told by the canvas reached.
func TestCanvasLeftOut(t *testing.T) {
	t.Parallel()

	// Written at the ends of the bounds, a box reaches a canvas that may be written as it is, and no more.
	far := "title: T\nparts:\n  - box: { x: 1000000, y: 1000000, w: 1000000, h: 1000000, rows: [A] }\n"
	for _, c := range []struct{ size, want, mistakes string }{
		{"", `viewBox="0 0 2000020 2000020"`, ""},
		{"width: 2000020\nheight: 2000020\n", `viewBox="0 0 2000020 2000020"`, ""},
		{"width: 10000001\n", "", "s.yaml:1:8: maximum: got 1.0000001×10⁰⁷, want 1×10⁰⁷"},
	} {
		svg, mistakes := draw(t, c.size+far, render("en"))
		if mistakes != c.mistakes || !strings.Contains(svg, c.want) {
			t.Errorf("a box at the bounds, %q: mistakes %q, want %q, the canvas %s", c.size, mistakes, c.mistakes, c.want)
		}
	}

	// The label, right of and below the box, is what reaches farthest.
	parts := "title: T\nparts:\n  - box: { x: 20, y: 20, w: 100, h: 40, rows: [A] }\n" +
		"  - label: { text: far, x: 300, y: 100, anchor: start }\n"
	for _, c := range []struct{ size, want string }{
		{"", `viewBox="0 0 346.6 129"`},
		{"width: 400\n", `viewBox="0 0 400 129"`},
		{"height: 300\n", `viewBox="0 0 346.6 300"`},
	} {
		svg, mistakes := draw(t, c.size+parts, render("en"))
		if mistakes != "" || !strings.Contains(svg, c.want) {
			t.Errorf("a canvas of %q = %s %s, want %s", c.size, svg, mistakes, c.want)
		}
	}

	// Each kind of what is drawn reaches as far as it goes, past the box.
	box := "title: T\nparts:\n  - box: { x: 20, y: 20, w: 100, h: 40, rows: [A] }\n"
	for _, c := range []struct{ part, want string }{
		{"edge: { from: [20, 100], via: [[300, 100], [300, 120]], to: [20, 120], straight: true }", `viewBox="0 0 320 140"`},
		{"line: { x1: 20, y1: 150, x2: 400, y2: 150 }", `viewBox="0 0 420 170"`},
		// The axis reaches across, its tick's text down.
		{"axis: { x1: 20, x2: 500, y: 100, ticks: [[20, a]] }", `viewBox="0 0 520 146"`},
		// A tick of no text reaches no further than its mark.
		{"axis: { x1: 20, x2: 300, y: 100, ticks: [[20, \"\"], [300, \"\"]] }", `viewBox="0 0 320 125"`},
		{"dot: { cx: 50, cy: 300 }", `viewBox="0 0 140 327"`},
		// The frame reaches across, its heading below its height.
		{"frame: { x: 140, y: 100, w: 200, h: 20, label: Store }", `viewBox="0 0 360 147"`},
	} {
		svg, mistakes := draw(t, box+"  - "+c.part+"\n", render("en"))
		if mistakes != "" || !strings.Contains(svg, c.want) {
			t.Errorf("a canvas reached by %s = %s %s, want %s", c.part, svg, mistakes, c.want)
		}
	}

	// A region's labels go where they go on a canvas written.
	venn := "  - circle: { name: a, cx: 120, cy: 120, r: 100 }\n  - circle: { name: b, cx: 240, cy: 120, r: 100 }\n" +
		"  - label: { text: both, in: [a, b] }\n  - label: { text: a alone, in: [a] }\n"
	written, mistakes := draw(t, "width: 400\nheight: 300\ntitle: T\nparts:\n"+venn, inspect)
	left, mistakes2 := draw(t, "title: T\nparts:\n"+venn, inspect)
	if mistakes != "" || mistakes2 != "" || left != written {
		t.Errorf("a Venn diagram of no canvas = %s %s, want it as on one written: %s %s", left, mistakes2, written, mistakes)
	}

	for _, c := range []struct{ parts, want string }{
		{"  - box: { x: -10, y: 20, rows: [Write] }\n", `s.yaml:3:5: box "Write" sticks out of the 90x69 canvas`},
		{"  - box: { x: -200, y: -100, rows: [Write] }\n", `s.yaml:3:5: box "Write" sticks out of the 20x20 canvas`},
		{
			"  - circle: { name: a, cx: -200, cy: 50, r: 50 }\n  - label: { text: x, in: [a] }\n",
			"s.yaml:3:5: circle \"a\" sticks out of the 20x120 canvas\ns.yaml:4:5: label \"x\" sticks out of the 20x120 canvas",
		},
		// Coordinates and sizes stand within a million either way, the schema refusing past it; at a million, the
		// layout checks the part as any other.
		{
			"  - box: { x: 20, y: 20, w: 100, h: 40, rows: [A] }\n  - circle: { name: c, cx: 1.0e+308, cy: -1000001, r: 1000001 }\n",
			"s.yaml:4:28: maximum: got 1×10³⁰⁸, want 1×10⁰⁶\ns.yaml:4:42: minimum: got -1.000001×10⁰⁶, want -1×10⁰⁶\n" +
				"s.yaml:4:55: maximum: got 1.000001×10⁰⁶, want 1×10⁰⁶",
		},
		{
			"  - box: { x: 20, y: 20, w: 100, h: 40, rows: [A] }\n  - circle: { name: c, cx: -1000000, cy: 50, r: 1000000 }\n",
			`s.yaml:4:5: circle "c" sticks out of the 140x1000070 canvas`,
		},
	} {
		_, got := draw(t, "title: T\nparts:\n"+c.parts, render("en"))
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.parts, got, c.want)
		}
	}
}

// A label placed by its region keeps out of every other circle, one of no name too.
func TestVennUnnamed(t *testing.T) {
	t.Parallel()

	src := "width: 400\nheight: 300\ntitle: T\nparts:\n  - circle: { name: a, cx: 150, cy: 150, r: 100 }\n" +
		"  - circle: { cx: 150, cy: 150, r: 60 }\n  - label: { text: x, in: [a] }\n"
	got, mistakes := draw(t, src, inspect)
	if mistakes != "" || strings.Contains(got, `label "x" 141.4,141`) {
		t.Errorf("a label beside a circle of no name = %s %s, want it outside that circle", got, mistakes)
	}
}

// A diagram's ids hold its number among the diagrams drawn into a page, so that one drawn twice holds others.
func TestIDs(t *testing.T) {
	t.Parallel()

	src := head + "  - edge: { from: [10, 10], to: [150, 10] }\n"
	ids := map[string]bool{}
	for _, seq := range []int{1, 2} {
		got, _ := draw(t, src, func(d *diagram.Drawer, v any) (string, error) {
			svg, err := d.Draw(v, diagram.Options{Lang: "en", Icons: lambda, Seq: seq})
			return string(svg), err
		})
		_, rest, _ := strings.Cut(got, `<marker id="`)
		id, _, _ := strings.Cut(rest, `"`)
		ids[id] = true
	}
	if len(ids) != 2 {
		t.Errorf("ids of one spec drawn twice = %v, want two", ids)
	}
}

// Raw SVG not closed is written closed, as read.
func TestRawClosed(t *testing.T) {
	t.Parallel()

	got, mistakes := draw(t, head+"  - raw: { svg: '<g><rect x=\"1\">' }\n", render("en"))
	if want := `<g><rect x="1"></rect></g>`; mistakes != "" || !strings.Contains(got, want) {
		t.Errorf("raw = %s %s, want %s", got, mistakes, want)
	}
}

// Text is drawn in the language asked for, its parents, then English.
func TestLanguages(t *testing.T) {
	t.Parallel()

	src := head + "  - box: { x: 10, y: 10, rows: [{ en: Yes, ja: はい }] }\n"
	for lang, want := range map[string]string{"en": ">Yes<", "ja": ">はい<", "ja-jp": ">はい<", "fr": ">Yes<"} {
		if got, _ := draw(t, src, render(lang)); !strings.Contains(got, want) {
			t.Errorf("%s: %s, want %s", lang, got, want)
		}
	}
	if _, err := os.Stat("testdata"); errors.Is(err, os.ErrNotExist) {
		t.Fatal("testdata is missing")
	}
}

// Inspect tells each edge's path and the region a label is in.
func TestInspect(t *testing.T) {
	t.Parallel()

	src := head + "  - circle: { name: a, cx: 100, cy: 100, r: 80 }\n  - label: { text: x, in: [a] }\n  - edge: { from: [200, 10], to: [300, 10] }\n"
	got, mistakes := draw(t, src, inspect)
	for _, want := range []string{"edge [200,10]→[300,10] M200,10 H300", `label "x"`, "in a"} {
		if mistakes != "" || !strings.Contains(got, want) {
			t.Errorf("inspect = %s %s, want %s", got, mistakes, want)
		}
	}
	// A frame with no name nor label is named by where it stands, its icon so too, each once; a label by its widest text.
	got, mistakes = draw(t, head+"  - frame: { x: 10, y: 10, w: 250, h: 150, icon: aws/aws-lambda }\n"+
		"  - label: { text: { en: Done, ja: \"完了した\" }, x: 100, y: 100 }\n", inspect)
	if want := "frame 10,10 250x150\nthe icon of frame 10,10 250x150 10,10 31x31\nlabel \"完了した\" 71,91 58x18\n"; mistakes != "" || got != want {
		t.Errorf("inspect = %q %s, want %q", got, mistakes, want)
	}
	// A table's width is its rows' as drawn: the type is not in code.
	got, mistakes = draw(t, head+"  - table: { x: 20, y: 20, title: T, rows: [[id, \"character varying(255)\"], [name, text]] }\n", inspect)
	if want := `table "T" 20,20 216x80`; mistakes != "" || !strings.HasPrefix(got, want+"\n") {
		t.Errorf("inspect = %q %s, want it to begin %q", got, mistakes, want)
	}
	// A label whose centre is on a circle's edge is in no circle, wherever the circle stands.
	for _, c := range []struct{ cx, x, want string }{{"100.2", "130.2", "114.4"}, {"100.7", "130.7", "114.9"}} {
		got, mistakes = draw(t, head+"  - circle: { name: A, cx: "+c.cx+", cy: 100, r: 50 }\n  - label: { text: Mon, x: "+c.x+", y: 140, plain: true }\n", inspect)
		if want := `label "Mon" ` + c.want + ",131 31.6x18\n"; mistakes != "" || !strings.HasSuffix(got, want) {
			t.Errorf("a circle at %s: inspect = %q %s, want it to end %q", c.cx, got, mistakes, want)
		}
	}
	// A label floe places in a region of circles symmetric about a line stands on the line, wherever they stand.
	for _, c := range []struct{ a, b, c, want string }{{"300", "460", "380", "342.6"}, {"377.7", "537.7", "457.7", "420.3"}, {"455.55", "615.55", "535.55", "498.15"}} {
		src := "width: 1200\nheight: 520\ntitle: T\nparts:\n  - circle: { name: a, cx: " + c.a + ", cy: 200, r: 170 }\n" +
			"  - circle: { name: b, cx: " + c.b + ", cy: 200, r: 170 }\n  - circle: { name: c, cx: " + c.c + ", cy: 335, r: 170 }\n" +
			"  - label: { text: \"cli, i18n\", in: [a, b, c] }\n"
		got, mistakes = draw(t, src, inspect)
		_, label, _ := strings.Cut(got, "\nlabel ")
		if want := `"cli, i18n" ` + c.want + ",218.5 74.8x18 in a, b, c\n"; mistakes != "" || label != want {
			t.Errorf("circles at %s: inspect = %q %s, want its label %q", c.a, got, mistakes, want)
		}
	}
	// A heading stands where an edge is refused for running through it, right of a frame's icon, and in the corner of a
	// frame without one; a heading's own line follows the frames, and a frame of no label has none.
	frame := "  - frame: { name: f, x: 20, y: 20, w: 300, h: 150, label: AWS Cloud, icon: aws/aws-lambda }\n" +
		"  - frame: { name: g, x: 20, y: 180, w: 100, h: 20, label: Az, square: true }\n"
	got, mistakes = draw(t, head+frame, inspect)
	if want := "frame \"f\" 20,20 300x150\nframe \"g\" 20,180 100x20\nthe heading of frame \"f\" 60,26 74.88x18\n" +
		"the heading of frame \"g\" 32,189 15.36x18\nthe icon of frame \"f\" 20,20 31x31\n"; mistakes != "" || got != want {
		t.Errorf("inspect = %q %s, want %q", got, mistakes, want)
	}
	if _, mistakes = draw(t, head+frame+"  - edge: { from: [133, 10], to: [133, 120] }\n", inspect); !strings.Contains(mistakes, `runs through the heading of frame "f"`) {
		t.Errorf("an edge in the heading told = %q", mistakes)
	}
	if _, mistakes = draw(t, head+frame+"  - edge: { from: [135, 10], to: [135, 120] }\n", inspect); mistakes != "" {
		t.Errorf("an edge past the heading told = %q", mistakes)
	}
	// The line of the diagram: its title in the widest language, its canvas as written or as far as what is drawn.
	lined := func(d *diagram.Drawer, v any) (string, error) {
		return d.Inspect(v, diagram.Options{Lang: "en", Icons: lambda, Seq: 1})
	}
	titled := "title: { en: A, ja: あいうえお }\nparts:\n  - box: { name: a, x: 10, y: 10, rows: [A] }\n"
	got, mistakes = draw(t, titled, lined)
	if want := "diagram \"あいうえお\" 78x59\nbox \"a\" 10,10 48x29\n"; mistakes != "" || got != want {
		t.Errorf("inspect = %q %s, want %q", got, mistakes, want)
	}
	if got, mistakes = draw(t, head+"  - box: { name: a, x: 10, y: 10, rows: [A] }\n", lined); mistakes != "" || !strings.HasPrefix(got, "diagram \"T\" 400x200\n") {
		t.Errorf("inspect = %q %s, want the canvas written", got, mistakes)
	}
	// What the checks refuse is told with the mistakes, and laid out as a spec with a mistake is (newLayout): a guide.
	// Nothing is drawn of it.
	bent := "title: S\nparts:\n  - box: { name: a, x: 20, y: 20, w: 100, h: 60, rows: [A] }\n" +
		"  - box: { name: b, x: 200, y: 40, w: 100, h: 60, rows: [B] }\n  - edge: { from: a, to: b }\n"
	if got, mistakes = draw(t, bent, inspect); mistakes != "" || !strings.Contains(got, "edge \"a\"→\"b\" M120,60 H200\n") {
		t.Errorf("inspect = %q %s, want the edge straight", got, mistakes)
	}
	refused := bent + "  - label: { text: x, x: 30, y: 30 }\n"
	var told string
	_, mistakes = draw(t, refused, func(d *diagram.Drawer, v any) (string, error) {
		var err error
		told, err = inspect(d, v)
		return told, err
	})
	if want := `box "a" overlaps label "x"`; !strings.Contains(mistakes, want) || !strings.Contains(told, "edge \"a\"→\"b\" M120,50 H160 V70 H200\n") {
		t.Errorf("inspect of a mistake = %q %q, want %s and the edge bent", told, mistakes, want)
	}
	if got, mistakes = draw(t, refused, render("en")); got != "" || mistakes == "" {
		t.Errorf("draw of a mistake = %q %q, want nothing drawn", got, mistakes)
	}
	// A bar is drawn from the smaller of its x, its label right of it, whichever is written first.
	got, mistakes = draw(t, head+"  - bar: { x1: 200, x2: 20, y: 50, label: back }\n", inspect)
	for _, want := range []string{`bar "back" 20,43 180x14`, `label "back" 205,41 38.8x18`} {
		if mistakes != "" || !strings.Contains(got, want) {
			t.Errorf("inspect = %s %s, want %s", got, mistakes, want)
		}
	}
}

// A tick's text is at the tick, or with labels between, halfway to the nearest tick right of it, else to x2, however
// the ticks are written; a tick of no text in any language is a mark alone.
func TestAxisTickText(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ axis, want string }{
		{`ticks: [[20, { en: "", ja: "月" }], [140, ""]]`, `label "月" 9,108 22x18` + "\n"},
		{`ticks: [[260, Wed], [20, Mon], [140, Tue]], labels: between`, "label \"Wed\" 304.2,108 31.6x18\nlabel \"Mon\" 64.2,108 31.6x18\nlabel \"Tue\" 184.2,108 31.6x18\n"},
		{`ticks: [[20, Mon], [20, ""], [140, ""]], labels: between`, `label "Mon" 64.2,108 31.6x18` + "\n"},
		{`ticks: [[20, Mon], [380, End]], labels: between`, "label \"Mon\" 184.2,108 31.6x18\nlabel \"End\" 364.2,108 31.6x18\n"},
		{`ticks: [[400, A]], labels: between`, `label "A" 381.4,108 17.2x18` + "\n"},
		{`ticks: [[20, Mon], [140, Tue]], labels: at`, "label \"Mon\" 4.2,108 31.6x18\nlabel \"Tue\" 124.2,108 31.6x18\n"},
	} {
		src := "width: 420\nheight: 150\ntitle: T\nparts:\n  - axis: { x1: 20, x2: 380, y: 100, " + c.axis + " }\n"
		if got, mistakes := draw(t, src, inspect); mistakes != "" || got != c.want {
			t.Errorf("%s\n  placed %q %s\n  want %q", c.axis, got, mistakes, c.want)
		}
	}
}

// An edge is drawn to a named dot's outline, the tip of its marker there, towards the first point off its centre, both
// ends measured as routed; it stays at the centre when the run left cannot keep its marker (or is within the radius),
// between two dots the one run cannot keep room for, and at a dot's centre written as a point. It is checked and
// labelled as routed.
func TestEdgesAtDots(t *testing.T) {
	t.Parallel()

	const (
		a = "  - box: { name: a, x: 20, y: 20, w: 100, h: 40, rows: [a] }\n"
		b = "  - box: { name: b, x: 200, y: 20, w: 100, h: 40, rows: [b] }\n"
		r = "  - dot: { name: r, cx: 240, cy: 140 }\n"
		d = "  - dot: { name: d, cx: 300, cy: 100 }\n"
		s = "  - dot: { name: s, cx: 40, cy: 140 }\n"
	)
	for _, c := range []struct{ name, parts, want string }{
		{"square, the arrow's tip at the outline", a + r + "  - edge: { from: a, fromSide: right, to: r }", `d="M120,40 H240 V132"`},
		{"straight", a + r + "  - edge: { from: a, to: r, via: [[180, 100]], straight: true }", `d="M120,40 L180,100 L233.34,135.56"`},
		{"from a dot, no marker", s + b + "  - edge: { from: s, to: b }", `d="M40,133 V40 H200"`},
		{"from a dot, an arrow", s + b + "  - edge: { from: s, to: b, start: arrow }", `d="M40,132 V40 H200"`},
		{"via on the centre", a + d + "  - edge: { from: a, via: [[300, 20], [300, 100]], to: d }", `d="M120,40 H300 V20 V92"`},
		{"first via on the centre", s + b + "  - edge: { from: s, via: [[40, 140], [40, 40]], to: b }", `d="M40,133 V40 H200"`},
		{"both dots", "  - dot: { name: s, cx: 40, cy: 100 }\n" + d + "  - edge: { from: s, to: d, start: arrow }", `d="M48,100 H292"`},
		{"a run that cannot keep the arrow", a + d + "  - edge: { from: d, via: [[300, 88], [200, 88]], to: a, start: arrow }", `d="M300,100 V88`},
		{"a run that keeps no marker", a + d + "  - edge: { from: d, via: [[300, 88], [200, 88]], to: a }", `d="M300,93 V88`},
		{"within the radius", "  - dot: { name: r, cx: 240, cy: 45 }\n  - edge: { from: [20, 40], to: r }", `d="M20,40 H240 V45"`},
		{"via in line within the radius", a + d + "  - edge: { from: a, via: [[300, 40], [300, 95]], to: d }", `d="M120,40 H300 V95 V100"`},
		{"dots that touch", "  - dot: { name: s, cx: 40, cy: 50 }\n  - dot: { name: t, cx: 54, cy: 50 }\n  - edge: { from: s, to: t }", `d="M40,50 H54"`},
		{"a run too short for both", "  - dot: { name: s, cx: 40, cy: 50 }\n  - dot: { name: t, cx: 55, cy: 50 }\n  - edge: { from: s, to: t }", `d="M40,50 H55"`},
		{"a point at the centre", d + "  - edge: { from: [20, 20], via: [[300, 20]], to: [300, 100] }", `d="M20,20 H300 V100"`},
		{
			"checked as routed", d + "  - edge: { from: [100, 103], to: [340, 103] }\n  - edge: { from: [300, 150], to: d }",
			"s.yaml:7:5: edge [100,103]→[340,103] crosses edge [300,150]→\"d\" at 300,103; move one of them, or add crossing: true to one if it cannot be avoided",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, mistakes := draw(t, "width: 360\nheight: 200\ntitle: T\nparts:\n"+c.parts+"\n", render("en"))
			if mistakes != c.want && (mistakes != "" || !strings.Contains(got, c.want)) {
				t.Errorf("%s\n  drew %s %q\n  want %q", c.parts, got, mistakes, c.want)
			}
		})
	}
	// The label stands where it would at the centre.
	got, mistakes := draw(t, "width: 360\nheight: 200\ntitle: T\nparts:\n  - dot: { name: u, cx: 150, cy: 155 }\n"+
		"  - edge: { from: [50, 50], to: u, label: go }\n", inspect)
	if want := `label "go" 137.8,93.5 24.4x18`; mistakes != "" || !strings.Contains(got, want) {
		t.Errorf("inspect = %s %s, want %s", got, mistakes, want)
	}
}

// counted counts the lookups of icons.
type counted struct {
	iconsOf
	n *int
}

func (c counted) Lookup(name string) ([]byte, error) {
	*c.n++
	return c.iconsOf.Lookup(name)
}

// An icon placed again, at the same size, is looked up once.
func TestIconLookedUpOnce(t *testing.T) {
	t.Parallel()

	n := 0
	src := "width: 200\nheight: 120\ntitle: T\nparts:\n" +
		"  - icon: { x: 10, y: 10, icon: aws/aws-lambda, label: A }\n" +
		"  - icon: { x: 100, y: 10, icon: aws/aws-lambda, label: B }\n"
	_, mistakes := draw(t, src, func(d *diagram.Drawer, v any) (string, error) {
		svg, err := d.Draw(v, diagram.Options{Lang: "en", Icons: counted{lambda, &n}, Seq: 1})
		return string(svg), err
	})
	if mistakes != "" || n != 1 {
		t.Errorf("an icon placed twice: %d lookups, %s; want 1", n, mistakes)
	}
}

// The runs floe places (between facing sides, round sides of one way, the two turns halfway into a side) keep
// ruleGap off the lines of axes and line parts along them, where they move with no new mistake.
func TestEdgesOffRules(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ src, want string }{
		// g1: a run between facing sides, on a grid line, moves to the nearer place, the smaller of two as near.
		{`width: 500
height: 140
title: g1
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
`, `edge "decide"→"design" M296,44 H308 V80 H328`},
		// g2: a run round two sides of one way moves outward only; a line written after the edge is kept off too.
		{`width: 500
height: 160
title: g2
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 20, y: 100, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: right, to: b, toSide: right }
  - line: { x1: 100, y1: 0, x2: 100, y2: 160 }
`, `edge "a"→"b" M80,35 H104 V115 H80`},
		// g3: a run across on an axis with no ticks moves above it.
		{`width: 300
height: 200
title: g3
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 160, y: 140, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: bottom, to: b, toSide: top }
  - axis: { x1: 10, x2: 290, y: 95, ticks: [] }
`, `edge "a"→"b" M50,50 V91 H190 V140`},
		// g4: a run between facing sides on a line part.
		{`width: 300
height: 160
title: g4
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 120, y: 100, w: 60, h: 30, rows: [b] }
  - line: { x1: 100, y1: 0, x2: 100, y2: 160 }
  - edge: { from: a, fromSide: right, to: b, toSide: left }
`, `edge "a"→"b" M80,35 H96 V115 H120`},
		// e1: no room between the ends inside their markers: it stays.
		{`width: 300
height: 160
title: e1
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 88, y: 100, w: 60, h: 30, rows: [b] }
  - line: { x1: 84, y1: 0, x2: 84, y2: 160 }
  - edge: { from: a, fromSide: right, to: b, toSide: left }
`, `edge "a"→"b" M80,35 H84 V115 H88`},
		// e2: outward is off the canvas written: it stays.
		{`width: 102
height: 160
title: e2
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 20, y: 100, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: right, to: b, toSide: right }
  - line: { x1: 100, y1: 0, x2: 100, y2: 160 }
`, `edge "a"→"b" M80,35 H100 V115 H80`},
		// e3: an axis with ticks is kept off past them below, so the run moves above.
		{`width: 300
height: 200
title: e3
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 160, y: 150, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: bottom, to: b, toSide: top }
  - axis: { x1: 10, x2: 290, y: 100, ticks: [[10, "0"], [150, "1"], [290, "2"]] }
`, `edge "a"→"b" M50,50 V96 H190 V150`},
		// e4: two edges out of one part sharing the run move as one.
		{`width: 500
height: 230
title: e4
parts:
  - axis: { x1: 200, x2: 424, y: 200, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - bar: { name: api, x1: 328, x2: 408, y: 140 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - edge: { from: decide, fromSide: right, to: api, toSide: left }
`, `edge "decide"→"design" M296,44 H308 V80 H328
edge "decide"→"api" M296,44 H308 V140 H328`},
		// e4b: the same out of a side of a box, the ends apart on it.
		{`width: 500
height: 230
title: e4b
parts:
  - axis: { x1: 200, x2: 424, y: 200, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - box: { name: decide, x: 200, y: 29, w: 96, h: 30, rows: [d] }
  - box: { name: design, x: 328, y: 65, w: 80, h: 30, rows: [d] }
  - box: { name: api, x: 328, y: 125, w: 80, h: 30, rows: [a] }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - edge: { from: decide, fromSide: right, to: api, toSide: left }
`, `edge "decide"→"design" M296,39 H308 V80 H328
edge "decide"→"api" M296,49 H308 V140 H328`},
		// e4c: an edge out of a side and an edge into it move as one.
		{`width: 500
height: 230
title: e4c
parts:
  - axis: { x1: 200, x2: 424, y: 200, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - box: { name: decide, x: 200, y: 29, w: 96, h: 30, rows: [d] }
  - box: { name: design, x: 328, y: 65, w: 80, h: 30, rows: [d] }
  - box: { name: api, x: 328, y: 125, w: 80, h: 30, rows: [a] }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - edge: { from: api, fromSide: left, to: decide, toSide: right }
`, `edge "decide"→"design" M296,39 H308 V80 H328
edge "api"→"decide" M328,140 H308 V49 H296`},
		// e5: both move right when a part holds the left for one of them.
		{`width: 500
height: 230
title: e5
parts:
  - axis: { x1: 200, x2: 424, y: 200, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - bar: { name: api, x1: 328, x2: 408, y: 140 }
  - box: { name: x, x: 270, y: 100, w: 40, h: 30, rows: [[x, sub]] }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - edge: { from: decide, fromSide: right, to: api, toSide: left }
`, `edge "decide"→"design" M296,44 H316 V80 H328
edge "decide"→"api" M296,44 H316 V140 H328`},
		// e6: the label stands on the run that would move: it stays.
		{`width: 500
height: 140
title: e6
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left, label: next }
`, `edge "decide"→"design" M296,44 H312 V80 H328`},
		// f1: a part's edge along the place nearer holds it.
		{`width: 500
height: 140
title: f1
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - box: { name: k, x: 268, y: 56, w: 40, h: 30, rows: [[k, sub]] }
`, `edge "decide"→"design" M296,44 H316 V80 H328`},
		// f2: another edge's run along the place nearer holds it.
		{`width: 500
height: 140
title: f2
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - edge: { from: [308, 52], to: [308, 76], straight: true, end: none }
`, `edge "decide"→"design" M296,44 H316 V80 H328
edge [308,52]→[308,76] M308,52 V76`},
		// f3: moving it changes the run the label stands on: it stays.
		{`width: 400
height: 160
title: f3
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 300, y: 100, w: 60, h: 30, rows: [b] }
  - line: { x1: 190, y1: 0, x2: 190, y2: 160 }
  - edge: { from: a, fromSide: right, to: b, toSide: left, label: next }
`, `edge "a"→"b" M80,35 H190 V115 H300`},
		// f4: the run made longer would lie along a line part: it stays.
		{`width: 500
height: 160
title: f4
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 20, y: 100, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: right, to: b, toSide: right }
  - line: { x1: 100, y1: 0, x2: 100, y2: 160 }
  - line: { x1: 101, y1: 35, x2: 200, y2: 35 }
`, `edge "a"→"b" M80,35 H100 V115 H80`},
		// f5: an axis with no ticks is kept off by ruleGap below.
		{`width: 300
height: 200
title: f5
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 160, y: 144, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: bottom, to: b, toSide: top }
  - axis: { x1: 10, x2: 290, y: 95, ticks: [] }
`, `edge "a"→"b" M50,50 V99 H190 V144`},
		// f6: with ticks, past them below.
		{`width: 300
height: 200
title: f6
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 160, y: 144, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: bottom, to: b, toSide: top }
  - axis: { x1: 10, x2: 290, y: 95, ticks: [[10, ""], [290, ""]] }
`, `edge "a"→"b" M50,50 V91 H190 V144`},
		// f7: a canvas whose size is left out holds anything below and right.
		{`title: f7
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
`, `edge "decide"→"design" M296,44 H308 V80 H328`},
		// h1: the two turns halfway into a side, after via, move as well.
		{`width: 300
height: 200
title: h1
parts:
  - box: { name: b, x: 200, y: 100, w: 60, h: 30, rows: [b] }
  - line: { x1: 185, y1: 0, x2: 185, y2: 200 }
  - edge: { from: [170, 150], via: [[170, 60]], to: b, toSide: left }
`, `edge [170,150]→"b" M170,150 V60 H181 V115 H200`},
		// h2: a run ruleGap off a line stays.
		{`width: 500
height: 140
title: h2
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [316, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
`, `edge "decide"→"design" M296,44 H312 V80 H328`},
		// h3: a slanting line is no rule.
		{`width: 300
height: 160
title: h3
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 120, y: 100, w: 60, h: 30, rows: [b] }
  - line: { x1: 60, y1: 0, x2: 140, y2: 160 }
  - edge: { from: a, fromSide: right, to: b, toSide: left }
`, `edge "a"→"b" M80,35 H100 V115 H120`},
		// f8: a start's marker keeps the run off that end too: no room, it stays.
		{`width: 300
height: 160
title: f8
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 104, y: 100, w: 60, h: 30, rows: [b] }
  - line: { x1: 92, y1: 0, x2: 92, y2: 160 }
  - edge: { from: a, fromSide: right, to: b, toSide: left, start: arrow }
`, `edge "a"→"b" M80,35 H92 V115 H104`},
		// f8b: with no marker at the start, the run moves nearer it.
		{`width: 300
height: 160
title: f8b
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 104, y: 100, w: 60, h: 30, rows: [b] }
  - line: { x1: 92, y1: 0, x2: 92, y2: 160 }
  - edge: { from: a, fromSide: right, to: b, toSide: left }
`, `edge "a"→"b" M80,35 H88 V115 H104`},
		// f9: the place nearer runs through a label: it moves the other way.
		{`width: 500
height: 140
title: f9
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - label: { text: x, x: 306, y: 62, anchor: end }
`, `edge "decide"→"design" M296,44 H316 V80 H328`},
		// f10: the place nearer crosses another edge: it moves the other way.
		{`width: 500
height: 140
title: f10
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - edge: { from: [302, 62], to: [310, 62], end: none }
`, `edge "decide"→"design" M296,44 H316 V80 H328
edge [302,62]→[310,62] M302,62 H310`},
		// f12: a tick of no text is no label to keep off: the run moves to the nearer place.
		{`width: 320
height: 230
title: f12
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 220, y: 174, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: bottom, to: b, toSide: top }
  - axis: { x1: 10, x2: 300, y: 95, ticks: [[150, ""]] }
  - line: { x1: 10, y1: 112, x2: 300, y2: 112 }
`, `edge "a"→"b" M50,50 V108 H250 V174`},
		// f13: an edge runs through a tick of no text, no label.
		{`width: 400
height: 200
title: f13
parts:
  - box: { name: a, x: 20, y: 20, w: 60, h: 30, rows: [a] }
  - box: { name: b, x: 140, y: 150, w: 60, h: 30, rows: [b] }
  - edge: { from: a, fromSide: bottom, to: b, toSide: top }
  - axis: { x1: 10, x2: 390, y: 95, ticks: [[50, ""], [170, ""]] }
`, `edge "a"→"b" M50,50 V104 H170 V150`},
		// f11: the place nearer runs over a dot: it moves the other way.
		{`width: 500
height: 140
title: f11
parts:
  - axis: { x1: 200, x2: 424, y: 110, ticks: [[200, ""], [312, ""], [424, ""]], grid: 20 }
  - bar: { name: decide, x1: 200, x2: 296, y: 44 }
  - bar: { name: design, x1: 328, x2: 408, y: 80 }
  - edge: { from: decide, fromSide: right, to: design, toSide: left }
  - dot: { cx: 304, cy: 62 }
`, `edge "decide"→"design" M296,44 H316 V80 H328`},
	} {
		got, mistakes := draw(t, c.src, inspect)
		var edges []string
		for line := range strings.Lines(got) {
			if strings.HasPrefix(line, "edge ") {
				edges = append(edges, strings.TrimSuffix(line, "\n"))
			}
		}
		if mistakes != "" || strings.Join(edges, "\n") != c.want {
			t.Errorf("%s\n  drew %q %s\n  want %q", c.src, edges, mistakes, c.want)
		}
	}
}

// Text is measured in the class it is drawn in: bold wider than regular, code by its own, at the size part.css gives.
func TestFaces(t *testing.T) {
	t.Parallel()

	const head = "width: 800\nheight: 400\ntitle: T\nparts:\n"
	for _, c := range []struct{ name, parts, want string }{
		{"a table's title", "  - table: { x: 10, y: 10, title: ABCDEFGHIJ, rows: [[id, int]] }\n", `table "ABCDEFGHIJ" 10,10 120x58` + "\n"},
		{"a box's first row", "  - box: { x: 10, y: 10, rows: [ABCDEFGHIJ] }\n", `box "ABCDEFGHIJ" 10,10 120x29` + "\n"},
		{"main", "  - box: { x: 10, y: 10, rows: [[ABCDEFGHIJ, main]] }\n", `box "ABCDEFGHIJ" 10,10 120x29` + "\n"},
		{"sub", "  - box: { x: 10, y: 10, rows: [[ABCDEFGHIJ, sub]] }\n", `box "ABCDEFGHIJ" 10,10 104x29` + "\n"},
		{"a second row", "  - box: { x: 10, y: 10, rows: [A, ABCDEFGHIJ] }\n", `box "A" 10,10 104x46` + "\n"},
		{"main code, at the size of code", "  - box: { x: 10, y: 10, rows: [[render, main code]] }\n", `box "render" 10,10 80x29` + "\n"},
		{"sub code", "  - box: { x: 10, y: 10, rows: [[ABCDEFGHIJ, sub code]] }\n", `box "ABCDEFGHIJ" 10,10 112x29` + "\n"},
		{"full-width bold, as regular", "  - box: { x: 10, y: 10, rows: [注文の履歴テーブル] }\n", `box "注文の履歴テーブル" 10,10 152x29` + "\n"},
		{"a bold label", "  - label: { x: 100, y: 50, text: ABCDEFGHIJ, strong: true }\n", `label "ABCDEFGHIJ" 53.4,41 93.2x18` + "\n"},
		{"a label", "  - label: { x: 100, y: 50, text: ABCDEFGHIJ }\n", `label "ABCDEFGHIJ" 59,41 82x18` + "\n"},
	} {
		if got, told := draw(t, head+c.parts, inspect); got != c.want || told != "" {
			t.Errorf("%s:\n  inspect %q %q\n  want %q", c.name, got, told, c.want)
		}
	}
	for _, c := range []struct{ name, parts, want string }{
		{
			"the room of a frame's heading",
			"  - frame: { x: 10, y: 10, w: 300, h: 150, label: Developer machine here }\n  - box: { x: 182, y: 16, w: 100, rows: [x] }\n",
			`s.yaml:6:5: box "x" overlaps the heading of frame "Developer machine here"`,
		},
		{
			"the room of the heading of a frame with an icon",
			"  - frame: { x: 10, y: 10, w: 340, h: 150, icon: aws/aws-lambda, label: Developer machine here }\n  - box: { x: 226, y: 16, w: 100, rows: [x] }\n",
			`s.yaml:6:5: box "x" overlaps the heading of frame "Developer machine here"`,
		},
		{"a frame's heading", "  - frame: { x: 10, y: 10, w: 60, h: 60, label: ABCDEFGHIJ }\n", `s.yaml:5:5: frame "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 101 or more`},
		{"the heading of a frame with an icon", "  - frame: { x: 10, y: 10, w: 60, h: 60, icon: aws/aws-lambda, label: ABCDEFGHIJ }\n", `s.yaml:5:5: frame "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 136 or more`},
		{"a table's title, by a written w", "  - table: { x: 10, y: 10, w: 100, title: ABCDEFGHIJ, rows: [[id, int]] }\n", `s.yaml:5:5: table "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 116 or more`},
		{"a row, by a written w", "  - box: { x: 10, y: 10, w: 60, rows: [ABCDEFGHIJ] }\n", `s.yaml:5:5: box "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 116 or more`},
		{"main code, by a written w", "  - box: { x: 10, y: 10, w: 60, rows: [[render, main code]] }\n", `s.yaml:5:5: box "render": "render" needs w: 77 or more`},
	} {
		if _, told := draw(t, head+c.parts, inspect); told != c.want {
			t.Errorf("%s:\n  told %q\n  want %q", c.name, told, c.want)
		}
	}

	// Just past a frame's heading as it is drawn, at 12px, a part stands clear.
	const past = "  - frame: { x: 10, y: 10, w: 300, h: 150, label: Developer machine here }\n  - box: { x: 192, y: 16, w: 100, rows: [x] }\n"
	if _, told := draw(t, head+past, inspect); told != "" {
		t.Errorf("past a frame's heading: told %q", told)
	}

	// A group's frame is as wide as its heading is drawn: both where it is kept room and where it is checked.
	const group = "  - graph:\n      x: 20\n      y: 20\n      nodes:\n        - group:\n            name: g\n" +
		"            label: " + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" + "\n            nodes:\n              - box: { name: a, rows: [[a, sub]] }\n"
	if got, told := draw(t, head+group, inspect); !strings.HasPrefix(got, `frame "g" 20,20 258x`) || told != "" {
		t.Errorf("a group's heading:\n  inspect %q %q\n  want the frame 258 wide", got, told)
	}

	// A lane's heading is measured bold where a level edge is checked against it: regular, this one would not reach it.
	const lanes = "title: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      direction: right\n      lanes:\n" +
		"        - { name: u, label: U, nodes: [{ box: { name: a, rows: [Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa] } }, { box: { name: z, rows: [Zzzzzzzzzzzzzzzzzzzzzzzzzzzzzz] } }] }\n" +
		"        - { name: v, label: nnnnnnnnnnnnnnnnnnnnnnn, nodes: [{ box: { name: m, rows: [M] } }] }\n" +
		"        - { name: e, label: E, nodes: [{ box: { name: b, rows: [B] } }] }\n" +
		"      edges:\n        - { from: z, to: m }\n        - { from: a, to: b, level: true }\n"
	want := `s.yaml:13:36: edge "a"→"b": it runs through the heading of lane "v", which reaches into the first rank laid out right; drop level, or shorten the heading`
	if _, told := draw(t, lanes, inspect); told != want {
		t.Errorf("a lane's heading:\n  told %q\n  want %q", told, want)
	}
}

// A part is named in the language it is measured widest in, so in the class it is drawn in: bold, eight narrow
// characters are wider than five full-width (256 to 250), and regular narrower (240 to 250).
func TestFacesNamed(t *testing.T) {
	t.Parallel()

	const (
		head = "width: 600\nheight: 400\ntitle: T\nparts:\n"
		text = "{ en: The page, ja: ページ本体 }"
	)
	for _, c := range []struct{ name, parts, want string }{
		{"a box's first row", "  - box: { x: 10, y: 10, rows: [" + text + "] }\n", `box "The page"`},
		{"a box's row in sub", "  - box: { x: 10, y: 10, rows: [[" + text + ", sub]] }\n", `box "ページ本体"`},
		{"a table's title", "  - table: { x: 10, y: 10, title: " + text + ", rows: [[id, int]] }\n", `table "The page"`},
		{"a frame's heading", "  - frame: { x: 10, y: 10, w: 300, h: 100, label: " + text + " }\n", `frame "The page"`},
		{"a frame's heading by its icon", "  - frame: { x: 10, y: 10, w: 300, h: 100, icon: aws/aws-lambda, label: " + text + " }\n", `frame "The page"`},
		{"a tree's node", "  - tree: { x: 200, y: 20, root: { rows: [" + text + "], children: [{ rows: [[k, sub]] }] } }\n", `edge "The page"→"k"`},
		{"a bold label", "  - label: { x: 100, y: 50, text: " + text + ", strong: true }\n", `label "The page"`},
		{"a label", "  - label: { x: 100, y: 50, text: " + text + " }\n", `label "ページ本体"`},
		{"a bold label placed by in", "  - circle: { name: A, cx: 150, cy: 150, r: 88 }\n  - label: { text: " + text + ", in: [A], strong: true }\n", `label "The page"`},
	} {
		if got, told := draw(t, head+c.parts, inspect); !strings.Contains(got, c.want) || told != "" {
			t.Errorf("%s:\n  inspect %q %q\n  want %s", c.name, got, told, c.want)
		}
	}
	// A bold label in that finds no room is named as it is measured too.
	const small = "  - circle: { name: A, cx: 150, cy: 150, r: 10 }\n  - label: { text: " + text + ", in: [A], strong: true }\n"
	want := `s.yaml:6:5: label "The page" finds no room inside A and outside the other circles: make that area larger, or the text shorter`
	if _, told := draw(t, head+small, inspect); told != want {
		t.Errorf("no room:\n  told %q\n  want %q", told, want)
	}
}

// The mistakes of an icon of a graph and of a group's icon, told as those of the part icon, and those of the schema.
func TestGraphIconMistakes(t *testing.T) {
	t.Parallel()

	const (
		head = "title: t\nparts:\n"
		node = "  - graph: { x: 0, y: 0, nodes: [ { "
		tail = " } ] }\n"
	)
	for _, c := range []struct{ parts, want string }{
		{node + "icon: { name: a, icon: aws/nope, label: A }" + tail, `s.yaml:3:45: icon "a": no icon "aws/nope" (search with: floe icons search <word>)`},
		{node + "icon: { name: a, icon: nope, label: A }" + tail, `s.yaml:3:45: icon "a": "nope" needs its icon set in front (aws), like aws/amazon-ec2 (search with: floe icons search <word>)`},
		{node + "icon: { name: a, icon: aws/aws-lambda, tone: warn }" + tail, "s.yaml:3:45: additional properties 'tone' not allowed"},
		{node + "icon: { icon: aws/aws-lambda }" + tail, "s.yaml:3:45: missing property 'name'"},
		{node + `icon: { name: a, icon: "" }` + tail, "s.yaml:3:60: minLength: got 0, want 1"},
		{node + "group: { name: g, icon: aws/nope, nodes: [ { box: { name: a, rows: [A] } } ] }" + tail, `s.yaml:3:46: the icon of frame "g": no icon "aws/nope" (search with: floe icons search <word>)`},
		{node + `group: { name: g, icon: "", nodes: [ { box: { name: a, rows: [A] } } ] }` + tail, "s.yaml:3:61: minLength: got 0, want 1"},
		{"  - box: { x: 0, y: 0, name: a, icon: aws/aws-lambda, rows: [A] }\n", "s.yaml:3:12: additional properties 'icon' not allowed"},
	} {
		_, mistakes := draw(t, head+c.parts, drawWith("en", lambda))
		if mistakes != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, mistakes, c.want)
		}
	}
	// What is written by hand over a graph's icon, or over its frame's, is told where it is written.
	over := head + `  - graph: { x: 20, y: 20, nodes: [ { group: { name: g, icon: aws/aws-lambda, nodes: [ { icon: { name: a, icon: aws/aws-lambda } } ] } } ] }
  - box: { x: 20, y: 20, w: 20, h: 20, rows: [B] }
`
	if _, mistakes := draw(t, over, drawWith("en", lambda)); !strings.HasPrefix(mistakes, "s.yaml:4:5: ") {
		t.Errorf("a box over a frame's icon: told %q, want it at the box", mistakes)
	}
}

// The top of each box and icon of what inspect tells, by name: of one height, the same top is the same rank.
func topsOf(t *testing.T, got string) map[string]string {
	t.Helper()
	tops := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^(?:box|icon) "([^"]+)" [-\d.]+,([-\d.]+) `).FindAllStringSubmatch(got, -1) {
		tops[m[1]] = m[2]
	}
	return tops
}

// with stands a box or an icon in the rank of the box it names, and takes the edges from boxes into it out of the
// ranks alone: whatever the order written, up or down as the ranks fall.
func TestGraphWith(t *testing.T) {
	t.Parallel()

	const top = "title: T\nparts:\n  - graph:\n      x: 20\n      y: 20\n      nodes:\n"
	b := func(name, with string) string {
		if with != "" {
			with = ", with: " + with
		}
		return "        - box: { name: " + name + with + ", rows: [" + strings.ToUpper(name) + "] }\n"
	}
	spec := func(nodes, edges string) string {
		if edges == "" {
			return top + nodes + "      edges: []\n"
		}
		return top + nodes + "      edges:\n" + edges
	}
	e := func(from, to string) string { return "        - { from: " + from + ", to: " + to + " }\n" }
	above := func(t *testing.T, tops map[string]string, hi, lo string) {
		t.Helper()
		h, err := strconv.ParseFloat(tops[hi], 64)
		if err != nil {
			t.Fatalf("%s: %v", hi, err)
		}
		l, err := strconv.ParseFloat(tops[lo], 64)
		if err != nil {
			t.Fatalf("%s: %v", lo, err)
		}
		if h >= l {
			t.Errorf("%s stands at %s, %s at %s, want it above", hi, tops[hi], lo, tops[lo])
		}
	}
	level := func(t *testing.T, tops map[string]string, a, c string) {
		t.Helper()
		if tops[a] != tops[c] {
			t.Errorf("%s stands at %s, %s at %s, want them in one rank", a, tops[a], c, tops[c])
		}
	}
	laid := func(t *testing.T, src string) (string, map[string]string) {
		t.Helper()
		got, told := draw(t, src, inspect)
		if told != "" {
			t.Fatalf("%s: told %s", src, told)
		}
		return got, topsOf(t, got)
	}

	t.Run("whatever the order written", func(t *testing.T) {
		t.Parallel()
		x, alb, nat, api := b("x", ""), b("alb", ""), b("nat", "alb"), b("api", "")
		for _, c := range []struct{ nodes, edges string }{
			{x + alb + nat + api, e("x", "api") + e("alb", "api") + e("api", "nat")},
			{alb + nat + x + api, e("x", "api") + e("alb", "api") + e("api", "nat")},
			{x + alb + nat + api, e("alb", "api") + e("x", "api") + e("api", "nat")},
		} {
			_, tops := laid(t, spec(c.nodes, c.edges))
			level(t, tops, "alb", "nat")
			above(t, tops, "alb", "api")
		}
	})
	t.Run("an edge up is drawn as written", func(t *testing.T) {
		t.Parallel()
		got, tops := laid(t, spec(b("lb", "")+b("api", "")+b("nat", "lb"), e("lb", "api")+e("api", "nat")))
		level(t, tops, "lb", "nat")
		above(t, tops, "nat", "api")
		if want := "edge \"api\"→\"nat\" M116,85 V49\n"; !strings.Contains(got, want) {
			t.Errorf("inspect = %s, want %s", got, want)
		}
		// Its arrow at nat, the end written, not at api.
		svg, told := draw(t, spec(b("lb", "")+b("api", "")+b("nat", "lb"), e("lb", "api")+e("api", "nat")), render("en"))
		if told != "" || !strings.Contains(svg, `d="M116,85 V49"`) {
			t.Errorf("draw = %s %s, want the edge from api up to nat", svg, told)
		}
	})
	t.Run("an icon", func(t *testing.T) {
		t.Parallel()
		icon := func(name, with string) string {
			if with != "" {
				with = ", with: " + with
			}
			return "        - icon: { name: " + name + with + ", icon: aws/aws-lambda, label: " + name + " }\n"
		}
		_, tops := laid(t, spec(icon("u", "")+icon("v", "")+icon("w", "u"), e("u", "v")+e("v", "w")))
		level(t, tops, "u", "w")
		above(t, tops, "u", "v")
		_, told := draw(t, spec(icon("a", "nobody"), ""), inspect)
		if want := "s.yaml:7:34: with: no box or icon of this graph named \"nobody\""; told != want {
			t.Errorf("told %q, want %q", told, want)
		}
	})
	t.Run("in another group", func(t *testing.T) {
		t.Parallel()
		g := func(name, with, in string) string {
			if with != "" {
				with = ", with: " + with
			}
			return "        - group: { name: " + name + ", label: " + strings.ToUpper(name) + ", nodes: [ { box: { name: " + in + with +
				", rows: [" + strings.ToUpper(in) + "] } } ] }\n"
		}
		_, tops := laid(t, spec(b("u", "")+g("g", "b", "a")+g("h", "", "b"), e("u", "b")))
		level(t, tops, "a", "b")
		_, none := laid(t, spec(b("u", "")+g("g", "", "a")+g("h", "", "b"), e("u", "b")))
		above(t, none, "a", "b")
	})
	t.Run("joining what no edge joins", func(t *testing.T) {
		t.Parallel()
		_, tops := laid(t, spec(b("a", "")+b("b", "")+b("c", "b")+b("d", ""), e("a", "b")+e("c", "d")))
		level(t, tops, "b", "c")
		above(t, tops, "a", "c")
		above(t, tops, "c", "d")
	})
	t.Run("the order within one group of boxes", func(t *testing.T) {
		t.Parallel()
		xs := func(got string) string {
			found := regexp.MustCompile(`(?m)^box "([^"]+)" ([-\d.]+),20 `).FindAllStringSubmatch(got, -1)
			order := make([]string, 0, len(found))
			for _, m := range found {
				order = append(order, fmt.Sprintf("%s@%s", m[1], m[2]))
			}
			return strings.Join(order, " ")
		}
		got, _ := laid(t, spec(b("a", "")+b("m", "")+b("b", "a")+b("z", ""), e("a", "z")+e("m", "z")))
		if want := "a@20 m@92 b@164"; xs(got) != want {
			t.Errorf("order = %s, want %s", xs(got), want)
		}
		// With a level edge in the group of boxes, those of a row stand together: not those of a with, m still between.
		got, _ = laid(t, spec(b("a", "")+b("m", "")+b("b", "a")+b("z", "")+b("p", "")+b("q", ""),
			e("a", "z")+e("m", "z")+e("z", "p")+"        - { from: p, to: q, level: true }\n"))
		if want := "a@26 m@98 b@170"; xs(got) != want {
			t.Errorf("order = %s, want %s", xs(got), want)
		}
		// LIMIT of joined: with no edge, m is a group of boxes of its own, past those a with joins.
		got, _ = laid(t, spec(b("a", "")+b("m", "")+b("b", "a"), ""))
		if want := "a@20 m@164 b@92"; xs(got) != want {
			t.Errorf("order = %s, want %s", xs(got), want)
		}
	})
	t.Run("what still decides ranks", func(t *testing.T) {
		t.Parallel()
		group := "        - group: { name: g, label: G, nodes: [ { box: { name: i, rows: [I] } } ] }\n"
		// An edge from a group: the with's other box goes down too.
		nodes := b("z", "") + group
		_, tops := laid(t, spec(b("alb", "")+b("nat", "alb")+nodes, e("z", "i")+e("g", "nat")))
		level(t, tops, "alb", "nat")
		above(t, tops, "i", "alb")
		_, none := laid(t, spec(b("alb", "")+b("nat", "")+nodes, e("z", "i")+e("g", "nat")))
		above(t, none, "alb", "nat")
		// An edge into a level edge's other box, the same.
		_, tops = laid(t, spec(b("y", "")+b("alb", "")+b("nat", "alb")+b("x", ""),
			"        - { from: nat, to: x, level: true }\n"+e("y", "x")))
		level(t, tops, "alb", "x")
		above(t, tops, "y", "alb")
		// An edge out of it: what it reaches stands below.
		_, tops = laid(t, spec(b("alb", "")+b("nat", "alb")+b("x", ""), e("nat", "x")))
		above(t, tops, "nat", "x")
		// And the cycle such an edge makes is turned as written.
		_, tops = laid(t, spec(b("x", "")+b("alb", "")+b("nat", "alb"), e("nat", "x")+e("x", "alb")))
		level(t, tops, "alb", "nat")
		above(t, tops, "x", "alb")
	})
	t.Run("what is refused", func(t *testing.T) {
		t.Parallel()
		const rank = "with stands its two ends in one rank, where a graph's edge cannot run; drop with, or write the edge by hand beside the graph"
		group := "        - group: { name: g, label: G, nodes: [ { box: { name: b, rows: [B] } } ] }\n"
		for _, c := range []struct{ name, src, want string }{
			{"a name of nothing", spec(b("a", "nobody"), ""), `s.yaml:7:33: with: no box or icon of this graph named "nobody"`},
			{
				"a name of nothing, an edge into it deciding ranks", spec(b("a", "")+b("b", "nobody"), e("a", "b")),
				`s.yaml:8:33: with: no box or icon of this graph named "nobody"`,
			},
			{"a group", spec(b("a", "g")+group, ""), `s.yaml:7:33: with: no box or icon of this graph named "g"`},
			{"an edge within one rank", spec(b("a", "b")+b("b", ""), e("a", "b")), "s.yaml:10:13: edge \"a\"→\"b\": " + rank},
			{
				"an edge within one rank, across groups", spec(
					"        - group: { name: h, label: H, nodes: [ { box: { name: a, with: b, rows: [A] } } ] }\n"+group, e("a", "b")),
				"s.yaml:10:13: edge \"a\"→\"b\": " + rank,
			},
			{"an edge into one rank by level and with", spec(b("a", "")+b("b", "c")+b("c", ""),
				"        - { from: a, to: b, level: true }\n"+e("a", "c")), "s.yaml:12:13: edge \"a\"→\"c\": " + rank},
			{
				"an edge within one rank by level alone", spec(b("a", "")+b("b", "a"),
					"        - { from: a, to: b, level: true }\n"+e("a", "b")),
				"s.yaml:11:13: edge \"a\"→\"b\": \"a\" and \"b\" stand in one rank by level edges, so this edge cannot run from one rank to another; end it elsewhere, or drop level from the edges joining them",
			},
			{"an edge that falls in one rank", spec(b("y", "")+b("alb", "")+b("nat", "alb")+b("x", ""),
				"        - { from: nat, to: x, level: true }\n"+e("y", "nat")), "s.yaml:13:13: edge \"y\"→\"nat\": " + rank},
			{"a with of itself", spec(b("a", "")+b("b", "b"), e("a", "b")), "s.yaml:10:13: edge \"a\"→\"b\": " + rank},
			{"a ring of with", spec(b("x", "")+b("a", "b")+b("b", "a"), e("x", "a")), "s.yaml:11:13: edge \"x\"→\"a\": " + rank},
			{"a ring holding the edge", spec(b("a", "b")+b("b", "c")+b("c", "a"), e("a", "c")), "s.yaml:11:13: edge \"a\"→\"c\": " + rank},
			{
				"a box a with stands among a group of ranks",
				spec(b("nat", "alb")+
					"        - group: { name: vpc, label: VPC, nodes: [ { box: { name: alb, rows: [ALB] } }, { box: { name: api, rows: [API] } } ] }\n",
					e("alb", "api")+e("vpc", "nat")),
				`s.yaml:11:28: edge "vpc"→"nat": "nat" can stand neither above group "vpc" nor below it, as "alb" stands no lower than it and "alb" no higher; end the edge at a box in "vpc"`,
			},
			{
				"a box a with stands between a level edge's boxes",
				`title: T
parts:
  - graph:
      x: 20
      y: 20
      lanes:
        - { name: l0, label: L0, nodes: [ { box: { name: p, rows: [P] } } ] }
        - { name: l1, label: L1, nodes: [ { box: { name: u, rows: [U] } }, { box: { name: m, with: p, rows: [M] } } ] }
        - { name: l2, label: L2, nodes: [ { box: { name: q, rows: [Q] } } ] }
      edges:
        - { from: u, to: q }
        - { from: p, to: q, level: true }
`,
				`s.yaml:12:36: edge "p"→"q": box "m" stands between its boxes in their rank; drop level, or move it out of the way`,
			},
			{
				"a box a with stands among a group", spec(b("e", "a")+
					"        - group: { name: g, label: G, nodes: [ { box: { name: a, rows: [A] } } ] }\n", e("e", "g")),
				`s.yaml:10:19: edge "e"→"g": "e" can stand neither above group "g" nor below it, as "a" stands no lower than it and "a" no higher; end the edge at a box in "g"`,
			},
		} {
			if _, told := draw(t, c.src, inspect); told != c.want {
				t.Errorf("%s: told\n%swant\n%s", c.name, told, c.want)
			}
		}
		// Written by hand beside the graph, as the words say, each is drawn.
		for _, nodes := range []string{
			b("a", "b") + b("b", ""),
			"        - group: { name: h, label: H, nodes: [ { box: { name: a, with: b, rows: [A] } } ] }\n" + group,
		} {
			if _, told := draw(t, spec(nodes, "")+"  - edge: { from: a, to: b }\n", inspect); told != "" {
				t.Errorf("by hand: told %s", told)
			}
		}
		// A ring out of which an edge runs is one rank and the rank below.
		_, tops := laid(t, spec(b("a", "b")+b("b", "a")+b("c", ""), e("a", "c")))
		level(t, tops, "a", "b")
		above(t, tops, "a", "c")
	})
	t.Run("a gateway in the rank of the balancer", func(t *testing.T) {
		t.Parallel()
		src := `title: A gateway in the rank of the balancer
parts:
  - graph:
      x: 20
      y: 20
      nodes:
        - icon: { name: users, icon: aws/aws-lambda, label: Users }
        - box: { name: pay, with: users, rows: [Payments, an outside API] }
        - group:
            name: vpc
            label: VPC
            nodes:
              - icon: { name: alb, icon: aws/aws-lambda, label: ALB }
              - icon: { name: api, icon: aws/aws-lambda, label: API }
              - group:
                  name: public
                  label: Public
                  nodes:
                    - icon: { name: nat, with: alb, icon: aws/aws-lambda, label: NAT }
      edges:
        - { from: users, to: alb }
        - { from: alb, to: api }
        - { from: api, to: nat, dashed: true }
        - { from: nat, to: pay, dashed: true }
`
		_, tops := laid(t, src)
		above(t, tops, "users", "alb") // pay is another height: above alb as users is
		above(t, tops, "pay", "alb")
		level(t, tops, "alb", "nat")
		above(t, tops, "pay", "nat")
		above(t, tops, "nat", "api")
	})
}
