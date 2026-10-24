package tmpl

import (
	"html/template"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/usererr"
)

// palette are the CSS variables default's palette defines, which the parts, diagrams, floe's base and
// the themes take their colours and faces from, as docs/templates.md lists them.
var palette = []string{
	"--bg", "--surface", "--fg", "--muted", "--line", "--accent", "--accent-soft", "--good", "--good-soft",
	"--warn", "--warn-soft", "--bad", "--bad-soft", "--font", "--display", "--mono", "--code-bg", "--code-fg",
	"--code-prompt", "--code-add", "--code-del", "--page-bg", "--rule", "--on-fill", "--shadow", "--text-width",
}

// The parts, diagrams, floe's base and the themes take from the page only what default's palette
// defines: every theme's palette goes over default's, so none of them goes without.
func TestPalette(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	under := set.palette
	defined := definedIn(under)
	for _, v := range palette {
		if !defined[v] {
			t.Errorf("default's palette leaves %s out", v)
		}
	}
	taken := map[string]string{"floe's base": baseCSS}
	for name, p := range set.parts {
		taken["the part "+name] = p.css
	}
	for name, theme := range set.themes {
		css := theme.base + strings.Join(theme.palettes, "")
		for _, part := range slices.Sorted(maps.Keys(theme.parts)) {
			css += theme.parts[part]
		}
		taken["the theme "+name] = css
	}
	for who, css := range taken {
		own := definedIn(css)
		for _, m := range usedVariable.FindAllStringSubmatch(css, -1) {
			if v := m[1]; !own[v] && !defined[v] {
				t.Errorf("%s takes %s, which default's palette does not define", who, v)
			}
		}
	}
	// default's palette is under every page, and a theme's sets its variables of its own names with it, so
	// nothing takes a value of its own for a variable. A diagram's style is the diagram package's, for pages
	// of any kind.
	delete(taken, "the part diagram")
	for who, css := range taken {
		for _, m := range regexp.MustCompile(`var\(\s*(--[\w-]+)\s*,`).FindAllStringSubmatch(css, -1) {
			t.Errorf("%s takes %s with a value of its own, which the palette gives", who, m[1])
		}
	}
	if set.strays != nil {
		t.Errorf("floe's parts and themes are warned of: %v", set.strays)
	}
	// A built-in template leaves the palette to the theme: a colour or a variable of it that it set would
	// hold over every theme's.
	pages, err := fs.Glob(templateFiles, "templates/*/page.css")
	if err != nil || pages == nil {
		t.Fatalf("the built-in templates' page.css = %v, %v", pages, err)
	}
	for _, name := range pages {
		src, readErr := templateFiles.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, v := range palette {
			if definedIn(string(src))[v] {
				t.Errorf("%s sets %s, which the theme's palette should", name, v)
			}
		}
		if w := colourWarnings(name, readCSS(src), true); w != nil {
			t.Errorf("%s writes colours: %v", name, w)
		}
	}
	// A diagram drawn into a page without the palette, by floe diagram render, takes default's light colours
	// (and faces of its own, for a page of any language).
	c, err := readPalette("palette.css", readCSS([]byte(under)))
	if err != nil {
		t.Fatal(err)
	}
	modes, err := floeModes()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`var\(\s*(--[\w-]+)\s*,\s*([^)]*)\)`).FindAllStringSubmatch(set.parts["diagram"].css, -1) {
		if _, colour := modes[m[1]]; colour && c.light[m[1]] != strings.TrimSpace(m[2]) {
			want := c.light[m[1]]
			t.Errorf("a diagram takes %s with %s; default's palette has %s", m[1], m[2], want)
		}
	}
	for name, p := range set.parts {
		// A part's element is its box, data-floe-part="<its name>", so that a page that draws it gets its
		// style; an element is the box of one part, and every name, and every look a part takes, is a part's.
		if p.pages != nil && !namesItself(namedParts(p.html), name) {
			t.Errorf("the HTML of the part %s is not its box, data-floe-part=%q", name, name)
		}
		for _, other := range namedParts(p.html) {
			if set.misnamed(other) != nameRight {
				t.Errorf("the HTML of the part %s names %s, which is no part, or data-floe-part=%q", name, other.name, other.value)
			}
		}
		for _, look := range p.looks {
			if set.parts[look.name] == nil {
				t.Errorf("the part %s takes the look of %s, which is no part", name, look.name)
			}
		}
	}
	// The boxes of floe's parts take their looks from part.yaml: the panel, through the table for risks.
	if looks := set.looksOf("risks"); !slices.Equal(looks, []string{"table", "panel"}) {
		t.Errorf("the looks of risks = %v", looks)
	}
	if !set.parts["panel"].isLook() || set.parts["panel"].css == "" {
		t.Errorf("the look panel = %+v", set.parts["panel"])
	}
}

// palette.css holds the palette's variables alone, set on :root and then for dark mode, without
// !important; a colour set for light mode alone is warned of.
func TestCheckPalette(t *testing.T) {
	t.Parallel()

	modes, err := floeModes()
	if err != nil {
		t.Fatal(err)
	}
	dark := "\n@media (prefers-color-scheme: dark) { :root { --bg: #000; --page-bg: none; color-scheme: dark; } }"
	// A palette always dark sets every colour default's turns dark, on :root alone; one that sets them
	// all for dark mode too but --bg leaves that one light among dark ones.
	var always, allButBg strings.Builder
	for _, name := range slices.Sorted(maps.Keys(modes)) {
		if modes[name] {
			always.WriteString(name + ": black; ")
			if name != "--bg" {
				allButBg.WriteString(name + ": black; ")
			}
		}
	}
	for css, want := range map[string]string{
		":root { " + always.String() + "}": "",
		":root { " + always.String() + "}\n@media (prefers-color-scheme: dark) { :root { " + allButBg.String() + "} }": "p.css: palette.css sets --bg for light mode alone",
		":root { --bg: light-dark(#fff, #000); --fg: light-dark(#000, #fff); }":                                        "",
		":root { --bg: #fff; }" + dark: "",
		":root { --bg: #fff; --page-bg: url(data:image/svg+xml;base64,PHN2Zz4=) repeat; }" + dark:                "",
		":root { --bg: #fff; --page-bg: url(data:image/svg+xml,<svg><style>p{fill:red}</style></svg>); }" + dark: "",
		":root { --radius: 4px; --text-width: 60rem; color-scheme: light dark; }":                                "",
		":root { color-scheme: light; }": "p.css: palette.css sets color-scheme for light mode alone",
		":root { --bg: #fff; --font: \"a;b\", serif; color-scheme: light; } /* body { } */" + dark: "",
		"@media (prefers-color-scheme: dark) { :root { --bg: #000; } }\n:root { --bg: #fff; }":     "p.css:2: write this :root before",
		":root {\n  --bg: #fff !important;\n}":                                                     "p.css:2: palette.css takes no !important",
		":root { --bg: #fff; }\nbody { margin: 0; }":                                               "p.css:2: palette.css holds only",
		"--bg: #fff;": "p.css:1: palette.css holds only",
		"@media (prefers-color-scheme: dark) {\n  --bg: #000;\n}": "p.css:2: palette.css holds only",
		":root {\n  .x { color: red; }\n}":                        "p.css:2: palette.css holds only",
		":root { color: red; }":                                   "p.css:1: palette.css holds only",
		"@media print { :root { --bg: #fff; } }":                  "p.css:1: palette.css holds only",
		":root { --bg: #fff; --code-bg: #222; --font: serif; --page-bg: var(--bg); --code-fg: #eee; }": "p.css: palette.css sets --bg, --code-bg for light mode alone",
		":root { --a: url(x(y); } } body { background: red }":                                          "p.css:1: this } closes no block",
		":root { Color-Scheme: light dark; --l: \"!important\"; --x: {a:b}; }":                         "",
		":root { --x: {a:b}; --bg: #fff; }":                                                            "p.css: palette.css sets --bg for light mode alone",
		":root { --a: f(]; color: red); }":                                                             "",
		"\n{ --x: 1; }\n\n":                                                                            "p.css:2: palette.css holds only",
	} {
		s := readCSS([]byte(css))
		err := s.refuse("p.css", false)
		var warnings []*usererr.Error
		if err == nil {
			warnings, err = checkPalette("p.css", s, nil)
		}
		got := ""
		switch {
		case err != nil:
			got = err.Error()
		case warnings != nil:
			got = warnings[0].Error()
		}
		if want == "" && got != "" || !strings.HasPrefix(got, want) {
			t.Errorf("checkPalette(%q) = %q, want %q", css, got, want)
		}
	}
}

// A part's part.css and a theme's files but palette.css take their colours from the palette: a colour
// written as a value is warned of, and a variable a theme's file sets. A colour's name counts only in a
// property that takes a colour; a mask's colours, transparent, currentColor and mixes of variables pass.
func TestColourWarnings(t *testing.T) {
	t.Parallel()

	for css, want := range map[string]string{
		":scope { color: #fff; }":                                         "c.css:1: #fff is a colour",
		":scope {\n  box-shadow: 0 1px 2px rgb(0 0 0 / 0.1);\n}":          "c.css:2: rgb(0 0 0 / 0.1) is a colour",
		":scope { background: linear-gradient(OKLCH(50% 0.1 20), red); }": "c.css:1: OKLCH(50% 0.1 20) is a colour",
		":scope { border: 1px solid White; }":                             "c.css:1: White is a colour",
		":scope { --x: #123; }":                                           "c.css:1: #123 is a colour",
		":scope { color: var(--fg, #000); }":                              "c.css:1: #000 is a colour",
		":scope { color: Canvas; }":                                       "c.css:1: Canvas is a colour",
		"@media (hover) { :scope:hover { outline-color: tomato; } }":      "c.css:1: tomato is a colour",
		"color: #abc;": "c.css:1: #abc is a colour",
		":scope { filter: drop-shadow(0 0 2px black); }":                                              "c.css:1: black is a colour",
		":scope { -webkit-box-shadow: 0 0 1px red; }":                                                 "c.css:1: red is a colour",
		":scope { -webkit-tap-highlight-color: red; }":                                                "c.css:1: red is a colour",
		":scope { text-emphasis: dot red; }":                                                          "c.css:1: red is a colour",
		":scope { color: rgb(from #fff r g b / 50%); }":                                               "c.css:1: #fff is a colour",
		":scope { backdrop-filter: drop-shadow(0 0 4px black); }":                                     "c.css:1: black is a colour",
		":scope { color: rgb(from var(--accent) r g b / 50%); }":                                      "",
		":scope { font-family: Arial Black, serif; white-space: nowrap; }":                            "",
		":scope { content: \"#fff red\"; grid-area: red; }":                                           "",
		":scope { --x: white; }":                                                                      "c.css:1: white is a colour",
		":scope { color: WindowText; }":                                                               "c.css:1: WindowText is a colour",
		":scope { -ms-filter: drop-shadow(0 0 1px red); }":                                            "c.css:1: red is a colour",
		"@property --c { syntax: '<color>'; initial-value: white; inherits: false; }":                 "c.css:1: white is a colour",
		":scope { --face: \"Arial Black\"; transition-property: background; }":                        "",
		":scope { color: var(--fg); background: color-mix(in srgb, var(--accent) 8%, transparent); }": "",
		":scope { border-color: currentColor transparent; }":                                          "",
		":scope { mask: linear-gradient(#000, transparent); -webkit-mask-image: url(\"#a\"); }":       "",
	} {
		got := ""
		if w := colourWarnings("c.css", readCSS([]byte(css)), true); w != nil {
			got = w[0].Error()
		}
		if want == "" && got != "" || !strings.HasPrefix(got, want) {
			t.Errorf("colourWarnings(%q) = %q, want %q", css, got, want)
		}
	}
	w := colourWarnings("base.css", readCSS([]byte(":root {\n  --x: var(--fg); --y: 1px;\n}\n.a { color: var(--x); }\n--z: 0;\n--e: ;")), false)
	if len(w) != 4 || !strings.HasPrefix(w[2].Error(), "base.css:5: --z is set") || !strings.HasPrefix(w[3].Error(), "base.css:6: --e is set") || !strings.HasPrefix(w[0].Error(), "base.css:2: --x is set") || !strings.HasPrefix(w[1].Error(), "base.css:2: --y is set") {
		t.Errorf("colourWarnings of a theme's file = %v, want --x and --y told", w)
	}
}

// definedIn returns the CSS variables css defines (--fg: #000), by name.
func definedIn(css string) map[string]bool {
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?:^|[^\w-])(--[\w-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}
	return defined
}

// A page's style goes before its </head>, and into no page without one. A tag in a comment, a script or a noscript is
// not one.
func TestWithStyles(t *testing.T) {
	t.Parallel()

	for page, want := range map[string]string{
		"<head><meta charset=utf-8></head><p>x</p>":                           "<head><meta charset=utf-8>S</head><p>x</p>",
		"<head><title>t</title></HEAD><p>x</p>":                               "<head><title>t</title>S</HEAD><p>x</p>",
		"<p>x</p><!-- </head> -->":                                            "",
		`<head><script>if (s.endsWith("</head>")) x()</script></head>`:        `<head><script>if (s.endsWith("</head>")) x()</script>S</head>`,
		"<head><!-- styles before </head> --></head>":                         "<head><!-- styles before </head> -->S</head>",
		"<head><!--></head>":                                                  "<head><!-->S</head>",
		"<head><noscript></head></noscript></head><body><script>y()</script>": "<head><noscript></head></noscript>S</head><body><script>y()</script>",
	} {
		got, ok := withStyles([]byte(page), tagsIn(page, false), "S")
		if string(got) != want || ok != (want != "") {
			t.Errorf("withStyles(%q) = %q, %v, want %q", page, got, ok, want)
		}
	}
}

// tagsOf finds the first <style> and <link> as an HTML parser would, case and all, past those in
// comments, scripts and noscripts, and past elements of other names that start so.
func TestTagsOf(t *testing.T) {
	t.Parallel()

	page := "<!-- <style> --><script>'<style><link>'</script><noscript><style></noscript><link-card></link-card><style-box>" +
		"<script-x><link></script-x><LINK rel=x><Style>a{}</Style>"
	want := tags{head: -1, style: strings.Index(page, "<Style>"), link: strings.Index(page, "<link></script-x>")}
	if got := tagsOf(tagsIn(page, false)); got != want {
		t.Errorf("tagsOf(%q) = %+v, want %+v", page, got, want)
	}
	if got := tagsOf(tagsIn("<p>x</p>", false)); got != (tags{head: -1, style: -1, link: -1}) {
		t.Errorf("tagsOf of a page without them = %+v", got)
	}
}

// A data-floe-part is an attribute when a tag holds it, past a > in a quoted value or in a template's
// action before it, and not in a value or in text.
func TestNamedPartsInTags(t *testing.T) {
	t.Parallel()

	for html, want := range map[string]bool{
		`<a title="a > b" data-floe-part="table">x</a>`:                                         false,
		`<div {{if ne (printf "%T" .v) "<nil>"}}class="x"{{end}} data-floe-part="table"></div>`: false,
		`<div class="{{or .k "a>b"}}" data-floe-part='table'>`:                                  false,
		`<p title="see data-floe-part=table">x</p>`:                                             true,
		`<p>write data-floe-part=table</p>`:                                                     true,
		`<script>'<b data-floe-part="table">'</script>`:                                         true,
		`<!-- <b data-floe-part="table"> -->`:                                                   true,
		`<div/data-floe-part="table">`:                                                          false,
	} {
		if named := namedParts(html); len(named) != 1 || named[0].name != "table" || named[0].text != want {
			t.Errorf("namedParts(%q) = %+v, want table, text %v", html, named, want)
		}
	}
	// HTML that is drawn, or the data's, holds no actions: {{ in a value is text, which hides no tag.
	html := `<a title="{{">x</a><div data-floe-part="a b"></div><style>p{}</style>`
	if named := namedIn(html, tagsIn(html, false)); len(named) != 2 || named[0].text || !named[0].notOne() {
		t.Errorf("namedIn(%q) = %+v, want an attribute of two names", html, named)
	}
	if at := tagsOf(tagsIn(html, false)); at.style != strings.Index(html, "<style>") {
		t.Errorf("tagsOf(%q) = %+v", html, at)
	}
	// A textarea's text, as a title's, runs to its end tag, past a quote: tags in it are text, and those
	// after it tags.
	for html, want := range map[string]bool{
		`<textarea><a title="</textarea><div data-floe-part="panel">x</div>"></a></textarea>`: false,
		`<textarea><div data-floe-part="panel">x</div></textarea>`:                            true,
		`<title>a</title><div data-floe-part="panel">`:                                        false,
	} {
		if named := namedIn(html, tagsIn(html, false)); len(named) != 1 || named[0].text != want {
			t.Errorf("namedIn(%q) = %+v, want text %v", html, named, want)
		}
	}
	// A template's action is no markup of the page's, out of a tag too.
	for html, want := range map[string]bool{
		`{{/* <div data-floe-part="panel"> */}}`:                true,
		`<p>{{ "<!--" }}</p><div data-floe-part="panel"></div>`: false,
	} {
		if named := namedParts(html); len(named) != 1 || named[0].text != want {
			t.Errorf("namedParts(%q) = %+v, want text %v", html, named, want)
		}
	}
	if at := tagsOf(tagsIn(`{{/* no <style> here */}}<p>x</p>`, true)); at.style != -1 {
		t.Errorf("tagsOf of a <style> in a template's comment = %+v", at)
	}
}

// A part's style comes after that of the looks it takes, which it adjusts, given or not, and of the
// parts it holds that are given; the others keep the order of their names.
func TestInOrder(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	got := set.inOrder([]string{"findings", "compare", "details", "stack", "compare"})
	if want := []string{"panel", "table", "compare", "stack", "details", "findings"}; !slices.Equal(got, want) {
		t.Errorf("inOrder = %v, want %v", got, want)
	}
}

// any.json offers every part floe draws, but cell and stack, which only other parts hold, and Markdown
// and HTML: a part added to floe and not to any.json is no part of report's.
func TestAnyOffersEveryPart(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"html", "text"}
	for name, p := range set.parts {
		if !p.isLook() && name != "cell" && name != "stack" {
			want = append(want, name)
		}
	}
	slices.Sort(want)
	offered := set.shapes[partsURL+"any.json"].keys
	if got := slices.Sorted(maps.Keys(offered)); !slices.Equal(got, want) {
		t.Errorf("any.json offers %v, want %v", got, want)
	}
	for name, s := range offered {
		if s.part != set.parts[name] {
			t.Errorf("any.json's %s draws by %s", name, s.ref)
		}
	}
}

// floe's parts and the schemas they share hold no mistake floe refuses of a part of one's own (a format of
// floe's, through a schema the parts share too, a $ref floe does not follow to or cannot read), whatever
// order they are read in: floe's are read without the checks that wait for every part to be read.
func TestFloePartsHaveNoMistakes(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	for url, n := range set.notes {
		if mistakes, err := set.mistakes(n); mistakes != nil || err != nil {
			t.Errorf("%s: %v, %v", url, mistakes, err)
		}
	}
}

// A description beside a $ref is ended as a sentence of its script before the part's.
func TestJoinDescriptions(t *testing.T) {
	t.Parallel()

	for _, c := range [][3]string{
		{"Risks", "A table", "Risks. A table"},
		{"Risks:", "A table", "Risks: A table"},
		{"起こりうる問題", "A table", "起こりうる問題。A table"},
		{"問題（四半期）", "A table", "問題（四半期）A table"},
		{"", "A table", "A table"},
		{"Risks", "", "Risks"},
	} {
		if got := joinDescriptions(c[0], c[1]); got != c[2] {
			t.Errorf("joinDescriptions(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// A mistake in a check's expression is told when the template's checks.yaml is read, before any data,
// which may not reach its list.
func TestCheckExprFirst(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "checks.yaml"), []byte("- for: title\n  expr: 'item +'\n  message: m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLibrary("", "", "", "").template("k", dir, dir).readChecks(); err == nil || !strings.Contains(err.Error(), "item +") {
		t.Errorf("readChecks = %v, want the mistake in the expression", err)
	}
}

// floe's parts' checks.yaml name paths their schemas declare, as those of one's own are checked.
func TestFloePartChecksDeclared(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	c, err := set.compiler(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for name, p := range set.parts {
		if len(p.checks) == 0 {
			continue
		}
		s, err := c.Compile(partsURL + name + ".json")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := checksDeclared(p.checks, s, true); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		checked++
	}
	if checked < 6 {
		t.Errorf("checked the checks.yaml of %d parts, want every one of floe's, 6 or more", checked)
	}
}

// Every expression of a part's checks.yaml that cannot be read is told, not the first alone.
func TestPartChecksAllMistakes(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{"checks.yaml": {Data: []byte("- expr: 'item +'\n  message: m\n- expr: 'itme.x'\n  message: m\n")}} //nolint:exhaustruct // the file's text
	_, err := readPartChecks(files, "parts/p")
	if err == nil || !strings.Contains(err.Error(), "parts/p/checks.yaml:1:9: item +") || !strings.Contains(err.Error(), "parts/p/checks.yaml:3:9: itme.x") {
		t.Errorf("readPartChecks of two broken expressions = %v, want both", err)
	}
}

// Every word a part draws has its text in the part's words.yaml, and every word there is drawn: named as
// written (word "risk"), or from the data (word $level), one of the values of the schema's enum.
func TestPartWords(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	fromData := map[string][]string{"matrix": {"yes", "part", "no"}, "risks": {"low", "mid", "high"}}
	for name, p := range set.parts {
		if p.pages == nil {
			continue
		}
		// Its HTML parsed anew: the parts' own pages are escaped, in place, by the tests drawing them.
		page, err := template.New(name).Funcs(funcs).Funcs(partFuncs).
			Funcs(template.FuncMap{"word": p.words.word("", fallbackLang)}).Parse(p.html)
		if err != nil {
			t.Fatal(err)
		}
		words := callsOf(page, "word")
		if words.other != (fromData[name] != nil) {
			t.Errorf("the part %s names a word otherwise than as written, which TestPartWords does not know", name)
		}
		drawn := slices.Concat(words.names, fromData[name])
		for _, n := range drawn {
			if _, ok := p.words[fallbackLang][n]; !ok {
				t.Errorf("the part %s has no word %s", name, n)
			}
		}
		for n := range p.words[fallbackLang] {
			if !slices.Contains(drawn, n) {
				t.Errorf("the part %s does not draw its word %s", name, n)
			}
		}
	}
}

// A part's words are read with their mistakes at their lines and columns, and a page takes the words of
// its language, else of the language without its last subtag, one at a time, else of en.
func TestReadWords(t *testing.T) {
	t.Parallel()

	w, err := readWords("w.yaml", []byte("en: { a: A, b: B }\nJA: { a: あ, b: い }\n"))
	if err != nil {
		t.Fatal(err)
	}
	for tag, want := range map[string]string{"ja": "ja", "ja-JP": "ja", "en-GB": "en", "fr": "en", "JA_jp.UTF-8": "ja"} {
		if got := w.lang(tag); got != want {
			t.Errorf("lang(%s) = %s, want %s", tag, got, want)
		}
	}
	if got, err := w.word("w.yaml", "ja")("a"); got != "あ" || err != nil {
		t.Errorf("word a in ja = %q, %v", got, err)
	}
	if _, err := w.word("w.yaml", "en")("c"); err == nil || err.Error() != `w.yaml: words.yaml has no word "c"` {
		t.Errorf("word c = %v", err)
	}
	for src, want := range map[string]string{
		"ja: { a: あ }\n":                                    "w.yaml:1:1: words.yaml has no en",
		"en: { a: A }\nja: { b: い }\n":                      "w.yaml:2:1: ja leaves out a, which en has\nw.yaml:2:1: ja has b, which en does not",
		"en: { a: A }\nj_p: { a: あ }\n":                     "w.yaml:2:1: j_p is not a language tag",
		"en: { a: [A] }\n":                                  "w.yaml:1:10: en.a: write text",
		"en: A\n":                                           "w.yaml:1:5: en: write the names of the words",
		"- en\n":                                            "w.yaml:1:1: write words.yaml as language tags",
		"en: &x { a: A }\nja: *x\n":                         "w.yaml:1:5: anchors, aliases",
		"en: { a: A }\nEN: { a: B }\n":                      "w.yaml:2:1: EN is the language en again",
		"en: { a: A }\npt-BR: { b: B }\n":                   "w.yaml:2:1: pt-BR leaves out a, which en has\nw.yaml:2:1: pt-BR has b, which en does not",
		"en: { a: A }\n---\nja: { a: あ }\n":                 "w.yaml:2:1: words.yaml holds one YAML document",
		"en: { a: A, b: B }\nj_p: { a: あ }\nja: { a: あ }\n": "w.yaml:2:1: j_p is not a language tag such as en, ja or pt-BR\nw.yaml:3:1: ja leaves out b, which en has",
		"en: { one: 1 }\n":                                  "w.yaml:1:12: en.one: write text, in quotes where YAML would read a number",
		"en: { a: A }\nja:\n":                               "w.yaml:2:1: ja: write the names",
		"en: { a: A }\nja_JP: { a: 1 }\n":                   "w.yaml:2:1: ja_JP is not a language tag such as en, ja or pt-BR\nw.yaml:2:13: ja_JP.a: write text",
	} {
		if _, err := readWords("w.yaml", []byte(src)); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("readWords(%q) = %v, want %s", src, err, want)
		}
	}
	if w, err := readWords("w.yaml", []byte("en:\n  a: |\n    A\n")); err != nil || w["en"]["a"] != "A" {
		t.Errorf("readWords of a block = %v, %v", w, err)
	}
	// A language not read is not compared with en, nor en with it: its own mistake alone is told.
	for src, want := range map[string]string{
		"en: { a: A }\nja: x\n": "w.yaml:2:5: ja: write the names of the words and their text",
		"en: A\nja: { a: x }\n": "w.yaml:1:5: en: write the names of the words and their text",
	} {
		if _, err := readWords("w.yaml", []byte(src)); err == nil || err.Error() != want {
			t.Errorf("readWords(%q) = %v, want %s", src, err, want)
		}
	}
	if w, err := readWords("w.yaml", nil); !slices.Equal(slices.Collect(maps.Keys(w)), []string{fallbackLang}) || w[fallbackLang] != nil || err != nil {
		t.Errorf("readWords of nothing = %v, %v", w, err)
	}
	// A language of more subtags is looked up a subtag at a time.
	zh := words{"en": {"a": "A"}, "zh-hant": {"a": "甲"}}
	if got := zh.lang("zh-Hant-TW"); got != "zh-hant" {
		t.Errorf("lang(zh-Hant-TW) = %s", got)
	}
	// A word named as written that the part has none of is refused as it is read, in any branch; one named
	// from the data is left to the drawing.
	for src, want := range map[string]string{
		`{{if .x}}{{with .y}}{{word "b"}}{{end}}{{end}}`:    `p.html: it names the words b, which words.yaml does not have`,
		`{{range .x}}{{word "a"}}{{word .}}{{end}}`:         "",
		`{{define "d"}}{{word "c"}}{{end}}{{template "d"}}`: `p.html: it names the words c, which words.yaml does not have`,
		`{{"d" | word}}`:   `p.html: it names the words d, which words.yaml does not have`,
		`{{(word "e").x}}`: `p.html: it names the words e, which words.yaml does not have`,
	} {
		page, err := template.New("p").Funcs(template.FuncMap{"word": zh.word("w.yaml", "en")}).Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		if err = zh.checkWords(page, "p.html"); (err == nil) != (want == "") || err != nil && err.Error() != want {
			t.Errorf("checkWords(%s) = %v, want %q", src, err, want)
		}
	}
}

// No CSS closes its <style>: each </style, of any case, is written <\/style, and the <style>s floe
// writes are all that end.
func TestPageStyleEnd(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	css := "/* </Style><b>x</b> */ a { background: url(data:image/svg+xml,<svg><style>p{}</style></svg>); }"
	style, err := set.pageStyle("<body></body>", nil, []byte(css), "default")
	if err != nil {
		t.Fatal(err)
	}
	if ends, sheets := strings.Count(strings.ToLower(style), "</style"), strings.Count(style, "<style>\n"); ends != sheets ||
		!strings.Contains(style, `/* <\/Style><b>x</b> */`) || !strings.Contains(style, `p{}<\/style></svg>`) {
		t.Errorf("pageStyle = %s, %d ends of %d <style>s", style, ends, sheets)
	}
}
