package diagram_test

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
)

func parse(t *testing.T, file, src string) *diagram.Spec {
	t.Helper()
	s, err := diagram.Parse(file, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return s
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
// (TestRenderMarkerIDs).
func TestRenderMatchesDiagramMJS(t *testing.T) {
	t.Parallel()

	s := parse(t, "every-part.yaml", readFile(t, "testdata/every-part.yaml"))
	got, err := diagram.Render([]*diagram.Spec{s})
	if err != nil {
		t.Fatal(err)
	}
	if want := readFile(t, "testdata/every-part.html"); got != want {
		t.Errorf("Render\n got: %s\nwant: %s", got, want)
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	const spec = "id: %s\nwidth: 100\nheight: 50\ntitle: t\nparts:\n"
	a := parse(t, "a.yaml", strings.Replace(spec, "%s", "a", 1)+"  - edge: { d: 'M0,10 H90' }\n")
	b := parse(t, "b.yaml", strings.Replace(spec, "%s", "b", 1)+"  - raw: { svg: '<rect class=\"band\"/>' }\n")
	got, err := diagram.Render([]*diagram.Spec{a, b})
	if err != nil {
		t.Fatal(err)
	}
	// Two diagrams in one fragment; raw stays as it is.
	for _, w := range []string{
		`<path d="M0,10 H90" class="edge" marker-end="url(#a_arrow)"/>`,
		"</svg></div>\n<div class=\"dg-wrap\"",
		`<rect class="band"/>`,
	} {
		if !strings.Contains(got, w) {
			t.Errorf("Render = %s\nwant it to contain %s", got, w)
		}
	}
	if n := strings.Count(got, `<marker id="a_arrow"`); n != 1 {
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
	got, err := diagram.Render([]*diagram.Spec{a, b})
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
	_, err := diagram.Render([]*diagram.Spec{a, b})
	want := `a.yaml:6:5: dot at 0,0 sticks out of the 100x50 canvas
b.yaml:2:1: diagram id "same" is already used in a.yaml:1; give each diagram its own id`
	if got := errString(err); got != want {
		t.Errorf("Render\n got:\n%s\nwant:\n%s", got, want)
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
	if got, want := strings.Join(names, " "), "architecture class er flow git sequence state timeline venn"; got != want {
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
			if _, err := diagram.Render([]*diagram.Spec{s}); err != nil {
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
