package tmpl

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
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
		if p.pages != nil && p.classes == nil {
			t.Errorf("the style of the part %s names no floe- class, so no page gets it", name)
		}
		// A part draws in another's look by naming its classes in its HTML, not in its style, which would
		// bring its style to a page of the other's.
		for other, q := range set.parts {
			if c := slices.IndexFunc(p.classes, func(c string) bool { return slices.Contains(q.classes, c) }); other != name && c >= 0 {
				t.Errorf("the styles of the parts %s and %s both name %s", name, other, p.classes[c])
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
	for _, v := range append(vars, diagram.Variables()...) {
		if !slices.Contains(palette, v) {
			t.Errorf("%s is not of the palette", v)
		}
	}
	css, err := builtin.ReadFile("builtin/report/page.html")
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

// The parts' style goes before the template's own, which then wins over it: in the <head>, else before
// the first <style> or <link> of a page without one, else at its end.
func TestWithStyles(t *testing.T) {
	t.Parallel()

	for page, want := range map[string]string{
		"<head><meta charset=utf-8><style>a{}</style></head><p>x</p>": "<head><meta charset=utf-8>S<style>a{}</style></head><p>x</p>",
		"<head><title>t</title></HEAD><style>b{}</style>":             "<head><title>t</title>S</HEAD><style>b{}</style>",
		"<p>x</p><link rel=stylesheet href=data:,>":                   "<p>x</p>S<link rel=stylesheet href=data:,>",
		"<p>x</p>": "<p>x</p>S",
		`<head><script>if (s.startsWith("<!--")) x()</script><style>f{}</style></head>`:                                       `<head><script>if (s.startsWith("<!--")) x()</script>S<style>f{}</style></head>`,
		"<head><!-- no <script> here --><style>g{}</style></head><body><script>y()</script>":                                  "<head><!-- no <script> here -->S<style>g{}</style></head><body><script>y()</script>",
		"<head><!-- styles before </head> --><style>d{}</style></head>":                                                       "<head><!-- styles before </head> -->S<style>d{}</style></head>",
		"<head><!--><style>e{}</style><!-- x --></head>":                                                                      "<head><!-->S<style>e{}</style><!-- x --></head>",
		"<head><noscript><link rel=stylesheet></noscript><!-- <style> --><script>'<style>'</script><style>c{}</style></head>": "<head><noscript><link rel=stylesheet></noscript><!-- <style> --><script>'<style>'</script>S<style>c{}</style></head>",
	} {
		if got := string(withStyles([]byte(page), "S")); got != want {
			t.Errorf("withStyles(%q) = %q, want %q", page, got, want)
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

// Every word a part draws by label has a message, which the languages without it take from English.
func TestPartLabels(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	// The words a part draws from its data, label $mark, are the values of the schema's enum.
	dynamic := map[string][]string{"matrix": {"yes", "part", "no"}, "risks": {"low", "mid", "high"}}
	english := i18n.EnglishCatalog()
	for name, p := range set.parts {
		keys := dynamic[name]
		for _, m := range regexp.MustCompile(`label "(\w+)"`).FindAllStringSubmatch(p.html, -1) {
			keys = append(keys, m[1])
		}
		if strings.Contains(p.html, "label $") != (dynamic[name] != nil) {
			t.Errorf("the part %s draws a word from its data, which TestPartLabels does not know", name)
		}
		for _, k := range keys {
			if key := "part_" + name + "_" + k; english.Msg(key) == key {
				t.Errorf("no message for %s", key)
			}
		}
	}
}
