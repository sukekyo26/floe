package diagram_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
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

func render(lang string) func(d *diagram.Drawer, v any) (string, error) {
	return func(d *diagram.Drawer, v any) (string, error) {
		svg, err := d.Draw(v, diagram.Options{Lang: i18n.Lang(lang), Icons: lambda, Seq: 1})
		return string(svg), err
	}
}

func inspect(d *diagram.Drawer, v any) (string, error) {
	return d.Inspect(v, diagram.Options{Lang: "en", Icons: lambda, Seq: 1})
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
	if len(examples) != 14 {
		t.Errorf("%d examples, want 14", len(examples))
	}
	store := iconsOf{}
	for _, name := range []string{"aws/aws-lambda", "aws/amazon-bedrock", "aws/group/aws-cloud-logo", "aws/group/region", "aws/res/client"} {
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
			`s.yaml:5:5: box "A" overlaps box "b"`,
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
			"s.yaml:5:5: edge [10,100]→[390,100] crosses edge [200,10]→[200,190] at 200,100; move one of them, or add crossing: true to one if it cannot be avoided",
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
		// Each check of the placement on its own.
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: F }\n  - box: { x: 80, y: 40, w: 60, h: 40, rows: [A] }", `s.yaml:6:5: box "A" straddles the edge of frame "F"`},
		{"  - box: { name: b, x: 150, y: 80, w: 60, h: 40, rows: [B] }\n  - edge: { from: [100, 100], to: [300, 100] }", `s.yaml:6:5: edge [100,100]→[300,100] runs through box "b"`},
		{"  - label: { text: L, x: 200, y: 100 }\n  - edge: { from: [100, 100], to: [300, 100] }", `s.yaml:6:5: edge [100,100]→[300,100] runs through label "L"`},
		{"  - box: { x: 20, y: 20, w: 80, h: 20, rows: [A, B] }", `s.yaml:5:5: box "A": its text needs a height of 46px or more; the box has 20px`},
		{"  - box: { x: 20, y: -5, w: 60, h: 40, rows: [A] }", `s.yaml:5:5: box "A" sticks out of the 400x200 canvas`},
		{"  - box: { x: 20, y: 180, w: 60, h: 40, rows: [A] }", `s.yaml:5:5: box "A" sticks out of the 400x200 canvas`},
		{"  - edge: { from: [10, 10], to: [10, 10] }", "s.yaml:5:5: edge [10,10]→[10,10] starts where it ends: end it at another part or point"},
		{
			"  - box: { name: a, x: 100, y: 20, w: 80, h: 29, rows: [a] }\n  - edge: { from: a, fromSide: left, to: a, toSide: left }",
			"s.yaml:6:5: edge a→a starts where it ends: end it at another part or point",
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
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: Short, ja: ずっと長い見出しの文字 }] }", `s.yaml:5:5: box "Short": "ずっと長い見出しの文字" (ja) needs w: 175 or more`},
		{"  - frame: { x: 10, y: 10, w: 100, h: 100, label: { en: F, ja: とても長い日本語の見出し } }", `s.yaml:5:5: frame "F": "とても長い日本語の見出し" (ja) needs w: 168 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: { en: T, ja: とても長い日本語の題 }, rows: [a] }", `s.yaml:5:5: table "T": "とても長い日本語の題" (ja) needs w: 162 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[{ en: id, ja: とても長い識別子 }, int]] }", `s.yaml:5:5: table "T": "とても長い識別子  int" (ja) needs w: 158 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[id, { en: int, ja: とても長い型の名前 }]] }", `s.yaml:5:5: table "T": "id  とても長い型の名前" (ja) needs w: 162 or more`},
		// The name and the type in other languages: each language's row as drawn, falling back to English.
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[{ en: id, ja: とても長い識別子 }, { en: int, ko: 아주 긴 정수 형식 }]] }", `s.yaml:5:5: table "T": "id  아주 긴 정수 형식" (ko) needs w: 161 or more`},
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: A very long row, ja: 短い }] }", `s.yaml:5:5: box "A very long row": "A very long row" needs w: 149 or more`},
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: AB, ja: あ, ko: 아주 긴 한국어 줄 }] }", `s.yaml:5:5: box "AB": "아주 긴 한국어 줄" (ko) needs w: 147 or more`},
		// Two languages other than English as wide: the first by tag.
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: A, ja: あいう, ko: 가나다 }] }", `s.yaml:5:5: box "A": "あいう" (ja) needs w: 71 or more`},
		// As wide as English, which is told: in the main row, the sub row, mixed widths and code.
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: ABCDEFGHIJ, ja: あいうえおか }] }", `s.yaml:5:5: box "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 110 or more`},
		{"  - box: { x: 20, y: 20, w: 40, h: 60, rows: [A, { en: ABCDEFGHIJKLMNO, ja: あいうえおかきくけ }] }", `s.yaml:5:5: box "A": "ABCDEFGHIJKLMNO" needs w: 140 or more`},
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: ABCDEFGHIJ, ja: ABCDEあいう }] }", `s.yaml:5:5: box "ABCDEFGHIJ": "ABCDEFGHIJ" needs w: 110 or more`},
		{"  - table: { x: 20, y: 20, w: 80, title: T, rows: [[{ en: " + codeEn + ", ja: " + codeJa + " }]] }", `s.yaml:5:5: table "T": "` + codeEn + `" needs w: 396 or more`},
		// A separator has no text, in a table sized by its rows too.
		{"  - table: { x: 20, y: 20, title: T, rows: [a, \"---\", b] }", ""},
		// U+2E80, the first CJK radical, is full-width.
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: AB, ja: \"\u2e80\u2e80\" }] }", "s.yaml:5:5: box \"AB\": \"\u2e80\u2e80\" (ja) needs w: 58 or more"},
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
		`s.yaml:6:5: edge [95,15]→[112,32] runs through box "h"`,
		`s.yaml:7:5: edge [310,10]→[290,34] runs through box "h"`,
		`s.yaml:9:5: edge [60,105]→[110,121] runs through box "s"`,
		`s.yaml:11:5: edge [60,215]→[110,231] runs through box "p"`,
		`s.yaml:13:5: edge [90,290]→[106,306] runs through box "r"`,
		`s.yaml:15:5: edge [90,370]→[106,386] runs through table "t"`,
		`s.yaml:17:5: edge [90,455]→[103,466] runs through bar "b"`,
		`s.yaml:19:5: edge [390,15]→[415,60] runs through box "tall"`,
		`s.yaml:20:5: edge [60,46]→[340,46] runs through box "h"`,
	}, "\n")
	if _, got := draw(t, inward, inspect); got != want {
		t.Errorf("edges 4 in = %q, want %q", got, want)
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
			`d="M40,40 V170 H200"`,
		},
		{
			"  - box: { name: a, x: 100, y: 20, w: 80, h: 29, rows: [a] }\n  - box: { name: b, x: 100, y: 120, w: 80, h: 29, rows: [b] }\n  - edge: { from: a, fromSide: left, to: b, toSide: left }",
			`d="M100,34.5 H80 V134.5 H100"`,
		},
		{"  - box: { x: 100, y: 10, origin: centre, rows: [abc] }", `<rect x="72" y="10" width="56" height="29" rx="8"/>`},
	} {
		got, mistakes := draw(t, head+c.parts+"\n", render("en"))
		if mistakes != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s\n  drew %s %s\n  want %s", c.parts, got, mistakes, c.want)
		}
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
		"  - circle: { name: b, cx: 250, cy: 150, r: 101 }\n  - label: { text: xxxx, in: [a, b], plain: true }\n"
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
	far := "width: 100000000000000000000.0\nheight: 300\ntitle: T\nparts:\n" +
		"  - circle: { name: a, cx: 50000000000000000000.0, cy: 150, r: 100 }\n  - label: { text: x, in: [a] }\n"
	if got, mistakes := draw(t, far, inspect); mistakes != "" || !strings.Contains(got, `label "x" 50000000000000000000,141`) {
		t.Errorf("a region far from 0 = %s %s, want the label placed", got, mistakes)
	}
	// A huge radius ends, its circle sticking out. A region that only touches the canvas has no room in it and
	// sticks out; one with some of the canvas looks for room there.
	for _, tt := range []struct{ circle, want string }{
		{"cx: 200, cy: 100, r: 9.683899191965114e+299", `s.yaml:5:5: circle "c" sticks out of the 400x200 canvas`},
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
		{"  - box: { x: -10, y: 20, rows: [Write] }\n", `s.yaml:3:5: box "Write" sticks out of the 82x69 canvas`},
		{"  - box: { x: -200, y: -100, rows: [Write] }\n", `s.yaml:3:5: box "Write" sticks out of the 20x20 canvas`},
		{
			"  - circle: { name: a, cx: -200, cy: 50, r: 50 }\n  - label: { text: x, in: [a] }\n",
			"s.yaml:3:5: circle \"a\" sticks out of the 20x120 canvas\ns.yaml:4:5: label \"x\" sticks out of the 20x120 canvas",
		},
		// An end that is not finite is left out of the canvas, which the part then sticks out of.
		{
			"  - box: { x: 20, y: 20, w: 100, h: 40, rows: [A] }\n  - circle: { name: c, cx: -1.0e+308, cy: 50, r: 1.0e+308 }\n",
			`s.yaml:4:5: circle "c" sticks out of the 140x80 canvas`,
		},
		{
			"  - box: { x: 20, y: 20, w: 100, h: 40, rows: [A] }\n  - circle: { name: c, cx: 1.0e+308, cy: 1.0e+308, r: 1.0e+308 }\n",
			`s.yaml:4:5: circle "c" sticks out of the 140x80 canvas`,
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
	// A bar is drawn from the smaller of its x, its label right of it, whichever is written first.
	got, mistakes = draw(t, head+"  - bar: { x1: 200, x2: 20, y: 50, label: back }\n", inspect)
	for _, want := range []string{`bar "back" 20,43 180x14`, `label "back" 205,41 38.8x18`} {
		if mistakes != "" || !strings.Contains(got, want) {
			t.Errorf("inspect = %s %s, want %s", got, mistakes, want)
		}
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
