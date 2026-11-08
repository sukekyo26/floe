package browsertest_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fragments publishes the diagrams of each tone and every built-in example but the AWS ones (their icons are fetched,
// which a test does not) in a page of floe's, opens it once its toolbars are drawn, and returns the fragment floe
// diagram render writes of them and how many diagrams it holds.
func fragments(t *testing.T, s *site) (string, int) {
	t.Helper()

	specs := []string{s.write(t, "tones.yaml", tonesSpec)}
	var data strings.Builder
	data.WriteString("template: report\nlang: en\ntitle: Diagrams\nsections:\n  - parts:\n      - diagram:\n")
	for line := range strings.Lines(tonesSpec) {
		data.WriteString("          " + line)
	}
	for line := range strings.Lines(s.floe(t, "diagram", "example")) {
		kind, _, _ := strings.Cut(line, "\t")
		if strings.HasPrefix(kind, "aws") {
			continue
		}
		specs = append(specs, s.write(t, kind+".yaml", "example: "+kind+"\n"))
		data.WriteString("      - diagram: { example: " + kind + " }\n")
	}
	fragment := s.floe(t, append([]string{"diagram", "render"}, specs...)...)
	s.floe(t, "publish", s.write(t, "diagrams.yaml", data.String()), "--slug", "diagrams")
	s.b.navigate("/a/diagrams/?raw")
	s.b.until("the toolbars", "document.querySelectorAll('.floe-zoom-tools').length === "+strconv.Itoa(len(specs)))
	return fragment, len(specs)
}

// openOwn opens a page of one's own named name, of head and body, from a directory of the test's.
func openOwn(t *testing.T, s *site, name, head, body string) {
	t.Helper()

	file := filepath.Join(t.TempDir(), name)
	page := "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><title>" + name + "</title>" + head +
		"</head>\n<body>\n" + body + "\n</body></html>\n"
	if err := os.WriteFile(file, []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	s.b.navigate("file://" + file)
}

// coloursApart returns {seen, amiss} of got against want, lines of diagramColours: each line that differs is amiss at
// diagram, told as page.
func coloursApart(page, got, want string) measured {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	m := measured{seen: len(w)}
	for i := range max(len(g), len(w)) {
		a, b := "none", "none"
		if i < len(g) {
			a = g[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			m.amiss = append(m.amiss, amiss{Page: page, At: "diagram", Text: a + ", in floe's page " + b})
		}
	}
	return m
}

const lookFragmentAloneFix = "paint a diagram by the palette and the tones its root holds, and read no theme's variable in it"

// lookFragmentAlone pastes the fragment of floe diagram render of every diagram (fragments) into a page of one's own
// without a palette, and into a shadow root, as diagramColours reads them: the diagrams, their toolbars too, are
// painted as in a page of floe's default theme (but the toolbars in a shadow root, where the zoom script finds no
// diagram).
func lookFragmentAlone(t *testing.T, s *site) measured {
	t.Helper()

	fragment, n := fragments(t, s)
	want := s.b.eval(diagramColours("document"))
	openOwn(t, s, "bare.html", "", fragment)
	s.b.until("the toolbars", "document.querySelectorAll('.floe-zoom-tools').length === "+strconv.Itoa(n))
	m := coloursApart("a page without a palette", s.b.eval(diagramColours("document")), want)
	openOwn(t, s, "shadow.html", "", `<div id="h"></div><script>document.getElementById('h').attachShadow({mode: 'open'}).innerHTML = `+
		js(fragment)+`;</script>`)
	var wantShadow []string
	for line := range strings.Lines(want) {
		if !strings.HasPrefix(line, ".floe-zoom-tools") && !strings.HasPrefix(line, ".floe-zoom-hint") {
			wantShadow = append(wantShadow, strings.TrimSuffix(line, "\n"))
		}
	}
	m.add(coloursApart("a shadow root", s.b.eval(diagramColours("document.getElementById('h').shadowRoot")), strings.Join(wantShadow, "\n")))
	return m
}

// brokenFragmentAlone draws a diagram whose raw SVG fills by a variable of floe's default theme's base.css, which a
// diagram's root does not hold: drawn in a page of floe's, then as a fragment in a page without a palette.
func brokenFragmentAlone(t *testing.T, s *site) (string, measured) {
	t.Helper()

	spec := "width: 200\nheight: 100\ntitle: Raw\nparts:\n  - raw: { svg: '<rect x=\"10\" y=\"10\" width=\"50\" height=\"50\" fill=\"var(--default-panel)\"/>' }\n"
	var data strings.Builder
	data.WriteString("template: report\nlang: en\ntitle: D\nsections:\n  - parts:\n      - diagram:\n")
	for line := range strings.Lines(spec) {
		data.WriteString("          " + line)
	}
	s.floe(t, "publish", s.write(t, "d.yaml", data.String()), "--slug", "broken")
	s.b.navigate("/a/broken/?raw")
	s.b.until("the toolbar", "document.querySelector('.floe-zoom-tools')")
	want := s.b.eval(diagramColours("document"))
	openOwn(t, s, "bare.html", "", s.floe(t, "diagram", "render", s.write(t, "raw.yaml", spec)))
	s.b.until("the toolbar", "document.querySelector('.floe-zoom-tools')")
	return "diagram", coloursApart("broken", s.b.eval(diagramColours("document")), want)
}
