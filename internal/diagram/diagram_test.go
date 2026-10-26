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
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

var update = flag.Bool("update", false, "write the golden files of the examples anew")

func drawer(t *testing.T) *diagram.Drawer {
	t.Helper()
	d, err := diagram.Own()
	if err != nil {
		t.Fatal(err)
	}
	return d
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
	root, err := yamlpos.Parse("s.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err = yamlpos.Decode("s.yaml", root, &v); err != nil {
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
	if len(examples) != 10 {
		t.Errorf("%d examples, want 10", len(examples))
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

// A spec of the wrong shape is told in jsonschema's words, at its line and column; nothing is laid out.
func TestShapeMistakes(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		parts, want string
	}{
		{"  - boxx: {}", "s.yaml:5:5: additional properties 'boxx' not allowed"},
		{"  - {}", "s.yaml:5:5: minProperties: got 0, want 1"},
		{"  - box: { x: 1, y: 1, rows: [a], tone: blue }", "s.yaml:5:41: value must be one of 'accent', 'good', 'warn', 'bad'"},
		{"  - box: { y: 1, rows: [a] }", "s.yaml:5:12: missing property 'x'"},
		{"  - box: { x: 1, y: 1, rows: [{ ja: あ }] }", "s.yaml:5:33: missing property 'en'"},
		{"  - box: { x: 1, y: 1, rows: [a], w: 0 }", "s.yaml:5:38: minimum: got 0, want 1"},
		{"  - edge: { from: a, to: b, d: M1 }", "s.yaml:5:13: additional properties 'd' not allowed"},
		{"  - edge: { from: a, to: b, fromSide: up }", "s.yaml:5:39: value must be one of 'top', 'right', 'bottom', 'left'"},
		{"  - label: { text: a }", "s.yaml:5:14: missing properties 'x', 'y'"},
	} {
		if _, got := draw(t, head+c.parts+"\n", render("en")); got != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, got, c.want)
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
		{"  - box: { x: 20, y: 20, w: 40, rows: [{ en: Short, ja: ずっと長い見出しの文字 }] }", `s.yaml:5:5: box "Short": "Short" needs w: 175 or more`},
		{"  - box: { name: a, x: 20, y: 20, rows: [A] }\n  - box: { name: a, x: 200, y: 20, rows: [B] }", `s.yaml:6:5: the name "a" is given before: give each part and row its own name`},
		{"  - edge: { from: chek, to: [10, 10] }", `s.yaml:5:5: edge: no part or row named "chek"`},
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
		{"  - raw: { svg: '<rect/></svg><p>x</p>' }", "s.yaml:5:5: raw SVG holds <p>, which it may not: write SVG's elements alone, without style, script or foreignObject"},
		{"  - raw: { svg: '<foreignObject><div/></foreignObject>' }", "s.yaml:5:5: raw SVG holds <foreignObject>, which it may not: write SVG's elements alone, without style, script or foreignObject"},
		{"  - raw: { svg: '<style>rect{}</style>' }", "s.yaml:5:5: raw SVG holds <style>, which it may not: write SVG's elements alone, without style, script or foreignObject"},
		{"  - icon: { icon: aws/amazon-ec2, x: 10, y: 10 }", `s.yaml:5:5: icon "aws/amazon-ec2": no icon "aws/amazon-ec2" (search with: floe icons search <word>)`},
	} {
		if _, got := draw(t, head+c.parts+"\n", render("en")); got != c.want {
			t.Errorf("%s\n  told %q\n  want %q", c.parts, got, c.want)
		}
	}
	// The icons' own mistakes (none fetched) keep their places, told once however many icons need them.
	missing := usererr.New("icons_not_fetched", "AWS", "/x/aws", "aws", "14MB", "aws.amazon.com")
	none := failing{missing}
	src := head + "  - icon: { icon: aws/a, x: 10, y: 10 }\n  - icon: { icon: aws/b, x: 100, y: 10 }\n"
	root, _ := yamlpos.Parse("s.yaml", []byte(src)) //nolint:errcheck // read just above
	var v any
	_ = yamlpos.Decode("s.yaml", root, &v) //nolint:errcheck // as above
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
		{"  - box: { x: 100, y: 10, origin: centre, rows: [abc] }", `<rect x="72" y="10" width="56" height="29" rx="8"/>`},
	} {
		got, mistakes := draw(t, head+c.parts+"\n", render("en"))
		if mistakes != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s\n  drew %s %s\n  want %s", c.parts, got, mistakes, c.want)
		}
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
}
