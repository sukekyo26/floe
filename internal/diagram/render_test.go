package diagram_test

import (
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/sukekyo26/floe/internal/diagram"
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

// Render writes what diagram.mjs writes for the same parts, byte for byte.
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
	a := parse(t, "a.yaml", strings.Replace(spec, "%s", "a", 1)+"  - edge: { d: 'M0,10 H90\"' }\n")
	b := parse(t, "b.yaml", strings.Replace(spec, "%s", "b", 1)+"  - raw: { svg: '<rect class=\"band\"/>' }\n")
	got, err := diagram.Render([]*diagram.Spec{a, b})
	if err != nil {
		t.Fatal(err)
	}
	// diagram.mjs wrote d as it was, so a stray quote broke the SVG; raw stays as it is.
	for _, w := range []string{
		`<path d="M0,10 H90&quot;" class="edge" marker-end="url(#a-arrow)"/>`,
		"</svg></div>\n<div class=\"dg-wrap\"",
		`<rect class="band"/>`,
	} {
		if !strings.Contains(got, w) {
			t.Errorf("Render = %s\nwant it to contain %s", got, w)
		}
	}
	if n := strings.Count(got, `<marker id="a-arrow"`); n != 1 {
		t.Errorf("Render defines the arrow %d times, want once", n)
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

// Every built-in example passes the checks, opens with a summary and is written in English.
func TestExamples(t *testing.T) {
	t.Parallel()

	examples, err := diagram.Examples()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(examples))
	for i, e := range examples {
		names[i] = e.Name
		s := parse(t, e.Name+".yaml", string(e.Spec))
		if _, err := diagram.Render([]*diagram.Spec{s}); err != nil {
			t.Errorf("example %s:\n%v", e.Name, err)
		}
		if e.Summary == "" || strings.HasPrefix(e.Summary, "#") {
			t.Errorf("example %s has no summary on its first line: %q", e.Name, e.Summary)
		}
		if i := strings.IndexFunc(string(e.Spec), func(r rune) bool { return r > unicode.MaxASCII }); i >= 0 {
			t.Errorf("example %s is not in plain English at byte %d", e.Name, i)
		}
	}
	want := "architecture class er flow git sequence state timeline venn"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("examples = %s, want %s", got, want)
	}
}
