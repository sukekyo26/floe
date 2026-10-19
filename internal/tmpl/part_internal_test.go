package tmpl

import (
	"html/template"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
)

// palette are the CSS variables of a theme that parts and diagrams take their colours, fonts and width
// from, as docs/templates.md lists them.
var palette = []string{
	"--bg", "--surface", "--fg", "--muted", "--line", "--accent", "--accent-soft", "--good", "--good-soft",
	"--warn", "--warn-soft", "--bad", "--bad-soft", "--font", "--mono", "--text-width", "--code-bg", "--code-fg",
	"--code-prompt", "--code-add", "--code-del",
}

// The parts and diagrams take their colours, fonts and width from the palette alone, which the report
// theme defines whole. A part takes each with a value of its own for a theme without it: one taken
// without would not be warned of by floe template check.
func TestPalette(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(set.parts))
	for name := range set.parts {
		names = append(names, name)
	}
	vars, err := partVariables(names)
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range set.parts {
		if bare := regexp.MustCompile(`var\(\s*--[\w-]+\s*\)`).FindString(p.css); bare != "" {
			t.Errorf("the part %s takes %s without a value of its own", name, bare)
		}
		// A part's element names it, so that a page that draws it gets its style, and every name is a part's.
		if names := namedParts(p.html); p.pages != nil && !slices.Contains(names, name) {
			t.Errorf("the HTML of the part %s does not name it in a data-floe-part", name)
		}
		for _, other := range namedParts(p.html) {
			if _, ok := set.parts[other]; !ok {
				t.Errorf("the HTML of the part %s names %s, which is no part", name, other)
			}
		}
	}
	// compare is drawn in the table's look, so a theme for it is warned of the table's variables too, and
	// tabs of those of code, which a diff in it is drawn in the look of.
	if compare, cerr := partVariables([]string{"compare"}); cerr != nil || !slices.Contains(compare, "--line") {
		t.Errorf("the variables of compare = %v, %v", compare, cerr)
	}
	if tabs, terr := partVariables([]string{"tabs"}); terr != nil || !slices.Contains(tabs, "--code-bg") || !slices.Contains(tabs, "--code-add") {
		t.Errorf("the variables of tabs = %v, %v", tabs, terr)
	}
	themes, err := partsThemeVariables()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range slices.Concat(vars, diagram.Variables(), themes) {
		if !slices.Contains(palette, v) {
			t.Errorf("%s is not of the palette", v)
		}
	}
	css, err := templateFiles.ReadFile("templates/report/page.css")
	if err != nil {
		t.Fatal(err)
	}
	defined := definedVariables(css)
	for _, v := range palette {
		if !defined[v] {
			t.Errorf("the report theme leaves %s out", v)
		}
	}
	// A theme without a variable gets the report theme's light value, as docs/templates.md says.
	light, _, _ := strings.Cut(string(css), "@media")
	values := map[string]string{}
	for _, m := range regexp.MustCompile(`(--[\w-]+):\s*([^;]+);`).FindAllStringSubmatch(light, -1) {
		values[m[1]] = strings.TrimSpace(m[2])
	}
	for name, p := range set.parts {
		for _, m := range regexp.MustCompile(`var\(\s*(--[\w-]+)\s*,\s*([^)]*)\)`).FindAllStringSubmatch(p.css, -1) {
			if values[m[1]] != strings.TrimSpace(m[2]) {
				t.Errorf("the part %s takes %s with %s; the report theme's is %s", name, m[1], m[2], values[m[1]])
			}
		}
	}
}

// A page's style goes before its </head>, else at its end. A tag in a comment, a script or a noscript is
// not one.
func TestWithStyles(t *testing.T) {
	t.Parallel()

	for page, want := range map[string]string{
		"<head><meta charset=utf-8></head><p>x</p>":                           "<head><meta charset=utf-8>S</head><p>x</p>",
		"<head><title>t</title></HEAD><p>x</p>":                               "<head><title>t</title>S</HEAD><p>x</p>",
		"<p>x</p>":                                                            "<p>x</p>S",
		`<head><script>if (s.endsWith("</head>")) x()</script></head>`:        `<head><script>if (s.endsWith("</head>")) x()</script>S</head>`,
		"<head><!-- styles before </head> --></head>":                         "<head><!-- styles before </head> -->S</head>",
		"<head><!--></head>":                                                  "<head><!-->S</head>",
		"<head><noscript></head></noscript></head><body><script>y()</script>": "<head><noscript></head></noscript>S</head><body><script>y()</script>",
	} {
		if got := string(withStyles([]byte(page), "S")); got != want {
			t.Errorf("withStyles(%q) = %q, want %q", page, got, want)
		}
	}
}

// tagsOf finds the first <style> and <link> as an HTML parser would, case and all, past those in
// comments, scripts and noscripts.
func TestTagsOf(t *testing.T) {
	t.Parallel()

	page := "<!-- <style> --><script>'<style><link>'</script><noscript><style></noscript><LINK rel=x><Style>a{}</Style>"
	want := tags{head: -1, style: strings.Index(page, "<Style>"), link: strings.Index(page, "<LINK")}
	if got := tagsOf([]byte(page)); got != want {
		t.Errorf("tagsOf(%q) = %+v, want %+v", page, got, want)
	}
	if got := tagsOf([]byte("<p>x</p>")); got != (tags{head: -1, style: -1, link: -1}) {
		t.Errorf("tagsOf of a page without them = %+v", got)
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

// A mistake in a check's expression is told before one in its list, which the data may not reach.
func TestCheckExprFirst(t *testing.T) {
	t.Parallel()

	list, err := readChecks("checks.yaml", []byte("- for: title\n  expr: 'item +'\n  message: m\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	problems, err := list[0].check("data.yaml", map[string]any{"title": "t"}, nil)
	if problems != nil || err == nil || !strings.Contains(err.Error(), "item +") {
		t.Errorf("check = %v, %v, want the mistake in the expression", problems, err)
	}
}

// A part holds parts, which it draws as the data file drew them, but no item of a format of floe's,
// which floe looks for only outside parts.
func TestPartsDrawnWhole(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	var drawnWhole func(s *schema) bool
	drawnWhole = func(s *schema) bool {
		if s == nil {
			return false
		}
		if slices.Contains(formats, s.Format) || drawnWhole(s.Items) {
			return true
		}
		return slices.ContainsFunc(slices.Collect(maps.Values(s.Properties)), drawnWhole)
	}
	for name, p := range set.parts {
		if drawnWhole(p.schema.Items) || slices.ContainsFunc(slices.Collect(maps.Values(p.schema.Properties)), drawnWhole) {
			t.Errorf("the part %s holds an item of a format", name)
		}
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
	if _, err := w.word("w.yaml", "en")("c"); err == nil || err.Error() != `w.yaml has no word "c"` {
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
	if w, err := readWords("w.yaml", nil); w != nil || err != nil {
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
		`{{if .x}}{{with .y}}{{word "b"}}{{end}}{{end}}`:    `p.html names the words b, which w.yaml does not have`,
		`{{range .x}}{{word "a"}}{{word .}}{{end}}`:         "",
		`{{define "d"}}{{word "c"}}{{end}}{{template "d"}}`: `p.html names the words c, which w.yaml does not have`,
		`{{"d" | word}}`:   `p.html names the words d, which w.yaml does not have`,
		`{{(word "e").x}}`: `p.html names the words e, which w.yaml does not have`,
	} {
		page, err := template.New("p").Funcs(template.FuncMap{"word": zh.word("w.yaml", "en")}).Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		if err = zh.checkWords(page, "p.html", "w.yaml"); (err == nil) != (want == "") || err != nil && err.Error() != want {
			t.Errorf("checkWords(%s) = %v, want %q", src, err, want)
		}
	}
}
