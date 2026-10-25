package diagram_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/icons"
)

// standIn holds a few icons under the official names, each a plain square.
var standIn = icons.New("testdata/icons")

func parse(t *testing.T, file, src string) *diagram.Spec {
	t.Helper()
	s, err := diagram.Parse(file, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// render draws specs with diagram.Render, the diagrams one after another.
func render(specs []*diagram.Spec, store *icons.Store) (string, error) {
	out, err := diagram.Render(specs, store)
	return strings.Join(out, "\n"), err
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Render writes what diagram.mjs writes for the same parts, byte for byte, but for the marker ids
// (TestRenderMarkerIDs) and icons. diagram.mjs wrote an icon's data URI wherever it showed; Render
// defines each once and places it with <use>, so icons.html is what Render writes, checked by eye.
func TestRenderMatchesDiagramMJS(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"every-part", "icons"} {
		s := parse(t, name+".yaml", readFile(t, "testdata/"+name+".yaml"))
		got, err := render([]*diagram.Spec{s}, standIn)
		if err != nil {
			t.Fatal(err)
		}
		if want := readFile(t, "testdata/"+name+".html"); got != want {
			t.Errorf("Render %s\n got: %s\nwant: %s", name, got, want)
		}
	}
}

// Each icon at each size is defined once, however often it shows.
func TestRenderIconDefs(t *testing.T) {
	t.Parallel()

	got, err := render([]*diagram.Spec{parse(t, "icons.yaml", readFile(t, "testdata/icons.yaml"))}, standIn)
	if err != nil {
		t.Fatal(err)
	}
	// Two corner icons, the client twice, and Bedrock at two sizes.
	if defs, uses := strings.Count(got, "<image "), strings.Count(got, "<use "); defs != 5 || uses != 6 {
		t.Errorf("Render defines %d icons and places %d, want 5 and 6", defs, uses)
	}
	if uses := regexp.MustCompile(`<use href="#ic_icon-[0-9a-f]{12}"`).FindAllString(got, -1); len(slices.Compact(slices.Sorted(slices.Values(uses)))) != 5 {
		t.Errorf("Render places %v, want 5 icons", uses)
	}

	// The ids come from the images, so diagrams of one id rendered apart do not take each other's icons.
	const spec = "id: d\nwidth: 100\nheight: 100\ntitle: t\nparts:\n  - icon: { x: 10, y: 10, name: %s }\n"
	def := regexp.MustCompile(`<image id="([^"]+)"`)
	ids := make([]string, 0, 2)
	for _, name := range []string{"aws/aws-lambda", "aws/amazon-bedrock"} {
		got, err := render([]*diagram.Spec{parse(t, "d.yaml", strings.Replace(spec, "%s", name, 1))}, standIn)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, def.FindStringSubmatch(got)[1])
	}
	if ids[0] == ids[1] {
		t.Errorf("two icons in diagrams of one id are both %s", ids[0])
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	const spec = "id: %s\nwidth: 100\nheight: 50\ntitle: t\nparts:\n"
	a := parse(t, "a.yaml", strings.Replace(spec, "%s", "a", 1)+"  - edge: { d: 'M0,10 H90' }\n")
	b := parse(t, "b.yaml", strings.Replace(spec, "%s", "b", 1)+"  - raw: { svg: '<rect class=\"band\"/>' }\n")
	out, err := diagram.Render([]*diagram.Spec{a, b}, standIn)
	if err != nil {
		t.Fatal(err)
	}
	// A fragment for each diagram, in order; raw stays as it is.
	if len(out) != 2 {
		t.Fatalf("Render = %q, want 2 diagrams", out)
	}
	for i, w := range []string{
		`<path d="M0,10 H90" class="edge" marker-end="url(#a_arrow)"/>`,
		`<rect class="band"/>`,
	} {
		if !strings.HasPrefix(out[i], `<div class="dg-wrap"`) || !strings.Contains(out[i], w) {
			t.Errorf("Render drew diagram %d as %s\nwant it to contain %s", i, out[i], w)
		}
	}
	if n := strings.Count(out[0], `<marker id="a_arrow"`); n != 1 {
		t.Errorf("Render defines the arrow %d times, want once", n)
	}
}

// diagram.mjs joined the diagram id and the marker kind with "-", so "a" with zero-one and "a-zero"
// with one both defined a-zero-one, and one diagram drew the other's marker. An id has no "_".
func TestRenderMarkerIDs(t *testing.T) {
	t.Parallel()

	const spec = "width: 100\nheight: 50\ntitle: t\nparts:\n"
	a := parse(t, "a.yaml", "id: a\n"+spec+"  - edge: { d: 'M0,10 H90', end: zero-one }\n")
	b := parse(t, "b.yaml", "id: a-zero\n"+spec+"  - edge: { d: 'M0,10 H90', end: one }\n")
	got, err := render([]*diagram.Spec{a, b}, standIn)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`<marker id="a_zero-one"`, `url(#a_zero-one)`, `<marker id="a-zero_one"`, `url(#a-zero_one)`} {
		if strings.Count(got, w) != 1 {
			t.Errorf("Render = %s\nwant %s once", got, w)
		}
	}
}

func TestRenderProblems(t *testing.T) {
	t.Parallel()

	const spec = "id: same\nwidth: 100\nheight: 50\ntitle: t\nparts:\n"
	a := parse(t, "a.yaml", spec+"  - dot: { cx: 0, cy: 0 }\n")
	b := parse(t, "b.yaml", "\n"+spec+"  - dot: { cx: 50, cy: 25 }\n")
	_, err := render([]*diagram.Spec{a, b}, standIn)
	want := `a.yaml:6:5: dot at 0,0 sticks out of the 100x50 canvas
b.yaml:2:1: diagram id "same" is already used in a.yaml:1; give each diagram its own id`
	if got := errString(err); got != want {
		t.Errorf("Render\n got:\n%s\nwant:\n%s", got, want)
	}
}

// The mistakes of a spec are in the order of its lines, whatever check found them.
func TestRenderProblemsInLineOrder(t *testing.T) {
	t.Parallel()

	o := parse(t, "o.yaml", "id: o\nwidth: 100\nheight: 50\ntitle: O\nparts:\n"+
		"  - dot: { cx: 0, cy: 25 }\n  - box: { x: 80, y: 10, w: 40, h: 30, rows: [A] }\n")
	_, err := render([]*diagram.Spec{o}, standIn)
	want := `o.yaml:6:5: dot at 0,25 sticks out of the 100x50 canvas
o.yaml:7:5: box "A" sticks out of the 100x50 canvas`
	if got := errString(err); got != want {
		t.Errorf("Render\n got:\n%s\nwant:\n%s", got, want)
	}
}

// Icons not fetched leave the diagrams that show them undrawn, told once after the mistakes of the
// others, and every other diagram is checked and drawn.
func TestRenderIconsNotFetched(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	const head = "width: 100\nheight: 100\ntitle: t\nparts:\n"
	icon := "  - icon: { x: 10, y: 10, name: aws/aws-lambda }\n"
	i1 := parse(t, "i1.yaml", "id: i1\n"+head+icon)
	o := parse(t, "o.yaml", "id: o\n"+head+"  - dot: { cx: 0, cy: 25 }\n")
	i2 := parse(t, "i2.yaml", "id: i2\n"+head+icon)
	ok := parse(t, "ok.yaml", "id: ok\n"+head+"  - dot: { cx: 50, cy: 25 }\n")
	out, err := diagram.Render([]*diagram.Spec{i1, o, i2, ok}, icons.New(dir))
	want := "o.yaml:6:5: dot at 0,25 sticks out of the 100x100 canvas\n" +
		"AWS icons are not in " + filepath.Join(dir, "aws") + `; run "floe icons fetch aws" (downloads about 14MB from aws.amazon.com) and try again`
	if got := errString(err); got != want {
		t.Errorf("Render\n got:\n%s\nwant:\n%s", got, want)
	}
	if len(out) != 4 || out[0] != "" || out[1] != "" || out[2] != "" || out[3] == "" {
		t.Errorf("Render drew %q, want the last diagram alone", out)
	}
}

// Every built-in example passes the checks and opens with a summary, in every language with a
// catalog. English holds every kind and is written in plain English; a language with examples of its
// own translates every one, and a language without them falls back to English.
func TestExamples(t *testing.T) {
	t.Parallel()

	english, err := diagram.Examples(i18n.English)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(english))
	for i, e := range english {
		names[i] = e.Name
		if i := strings.IndexFunc(string(e.Spec), func(r rune) bool { return r > unicode.MaxASCII }); i >= 0 {
			t.Errorf("example %s is not in plain English at byte %d", e.Name, i)
		}
	}
	if got, want := strings.Join(names, " "), "architecture aws class er flow git sequence state timeline venn"; got != want {
		t.Errorf("examples = %s, want %s", got, want)
	}
	for _, lang := range i18n.Supported() {
		examples, err := diagram.Examples(lang)
		if err != nil {
			t.Fatal(err)
		}
		_, err = os.Stat("examples/" + string(lang))
		translated := lang != i18n.English && err == nil
		for i, e := range examples {
			s := parse(t, string(lang)+"/"+e.Name+".yaml", string(e.Spec))
			if _, err := render([]*diagram.Spec{s}, standIn); err != nil {
				t.Errorf("example %s in %s:\n%v", e.Name, lang, err)
			}
			if e.Summary == "" || strings.HasPrefix(e.Summary, "#") {
				t.Errorf("example %s in %s has no summary on its first line: %q", e.Name, lang, e.Summary)
			}
			if translated && string(e.Spec) == string(english[i].Spec) {
				t.Errorf("example %s is not translated into %s", e.Name, lang)
			}
		}
	}
	// A language without examples of its own, or a region of one that has them, falls back.
	for lang, want := range map[i18n.Lang]i18n.Lang{"pt-br": i18n.English, "ja-jp": "ja"} {
		got, err := diagram.Examples(lang)
		if err != nil {
			t.Fatal(err)
		}
		in, err := diagram.Examples(want)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, in) {
			t.Errorf("Examples(%s) is not in %s", lang, want)
		}
	}
}
