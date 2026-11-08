package browsertest_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// lookFragmentRoot pastes the fragment of floe diagram render, a diagram of each tone and every built-in example,
// into pages of one's own, their style in the head before it. In a page without a palette, the diagram is painted as in a page of floe's default theme, its toolbar
// too, and in a shadow root as well. The page outside the diagrams keeps what it draws: its palette in a layer of its
// own, its color-scheme of a meta, the variables it reads with a fallback. A page's colours written on the
// diagrams' roots win, before the fragment or after it.
func lookFragmentRoot(t *testing.T) { //nolint:paralleltest,thelper // a subtest of TestLookRule; the browser tests share one Chrome
	s := newSite(t)
	b := s.b
	// The diagrams: one of each tone, then every built-in example but the AWS ones, which draw every kind of part
	// between them.
	specs := []string{s.write(t, "tones.yaml", tonesSpec)}
	var data strings.Builder
	data.WriteString("template: report\nlang: en\ntitle: Diagrams\nsections:\n  - parts:\n      - diagram:\n")
	for line := range strings.Lines(tonesSpec) {
		data.WriteString("          " + line)
	}
	for line := range strings.Lines(s.floe(t, "diagram", "example")) {
		kind, _, _ := strings.Cut(line, "\t")
		if strings.HasPrefix(kind, "aws") { // their icons are fetched, which a test does not
			continue
		}
		specs = append(specs, s.write(t, kind+".yaml", "example: "+kind+"\n"))
		data.WriteString("      - diagram: { example: " + kind + " }\n")
	}
	fragment := s.floe(t, append([]string{"diagram", "render"}, specs...)...)
	s.floe(t, "publish", s.write(t, "diagrams.yaml", data.String()), "--slug", "diagrams")
	b.navigate("/a/diagrams/?raw")
	b.until("the toolbars", "document.querySelectorAll('.floe-zoom-tools').length === "+strconv.Itoa(len(specs)))
	if got := b.eval(unpainted); got != "" {
		t.Errorf("shapes part.css leaves unpainted in floe's page:\n%s", got)
	}
	if got := b.eval(rawOff); got != "" {
		t.Errorf("raw SVG not in the palette's colours it reads, in floe's page:\n%s", got)
	}
	want := b.eval(diagramColours("document"))

	dir := t.TempDir()
	open := func(name, head, body string) {
		t.Helper()
		file := filepath.Join(dir, name)
		page := "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><title>" + name + "</title>" + head +
			"</head>\n<body>\n" + body + "\n</body></html>\n"
		if err := os.WriteFile(file, []byte(page), 0o600); err != nil {
			t.Fatal(err)
		}
		b.navigate("file://" + file)
	}

	open("bare.html", "", fragment)
	b.until("the toolbars", "document.querySelectorAll('.floe-zoom-tools').length === "+strconv.Itoa(len(specs)))
	if got := b.eval(diagramColours("document")); got != want {
		t.Errorf("in a page without a palette:\n%s\nwant, as in floe's page:\n%s", got, want)
	}

	open("shadow.html", "", `<div id="h"></div><script>document.getElementById('h').attachShadow({mode: 'open'}).innerHTML = `+
		js(fragment)+`;</script>`)
	inShadow := b.eval(diagramColours("document.getElementById('h').shadowRoot"))
	var wantShadow []string
	for line := range strings.Lines(want) {
		if !strings.HasPrefix(line, ".floe-zoom-tools") && !strings.HasPrefix(line, ".floe-zoom-hint") { // the zoom script finds no diagram in a shadow root
			wantShadow = append(wantShadow, strings.TrimSuffix(line, "\n"))
		}
	}
	if w := strings.Join(wantShadow, "\n"); inShadow != w {
		t.Errorf("in a shadow root:\n%s\nwant:\n%s", inShadow, w)
	}

	own := `<meta name="color-scheme" content="light dark"><style>
@layer host { :root { --bg: rgb(10, 20, 30); --accent: rgb(200, 0, 100); } }
.probe { color: var(--accent, rgb(1, 1, 1)); background: var(--bg); width: var(--text-width, 33px);
  border-top: var(--rule, 3px dotted rgb(2, 2, 2)); box-shadow: var(--shadow, 0 0 0 1px rgb(3, 3, 3)); }
</style>`
	open("alone.html", own, `<p class="probe">p</p>`)
	alone := b.eval(outside)
	open("around.html", own, `<p class="probe">p</p>`+fragment)
	b.until("the toolbar", "document.querySelector('.floe-zoom-tools')")
	if got := b.eval(outside); got != alone {
		t.Errorf("outside the diagram: %s, want %s, as without it", got, alone)
	}

	const accent = "rgb(1, 2, 3)"
	rule := "<style>.floe-zoom-tools, .floe-zoom { --accent: " + accent + "; }</style>"
	stroke := "getComputedStyle(document.querySelector('.dg .box[data-floe-tone=accent] rect')).stroke"
	for _, page := range []struct{ name, head, body string }{
		{"before.html", rule, fragment},
		{"after.html", "", fragment + rule},
	} {
		open(page.name, page.head, page.body)
		if got := b.eval(stroke); got != accent {
			t.Errorf("%s: the accent box's stroke %s, want the page's %s", page.name, got, accent)
		}
	}
}
