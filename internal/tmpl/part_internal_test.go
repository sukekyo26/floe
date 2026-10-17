package tmpl

import (
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
	"--warn", "--warn-soft", "--bad", "--bad-soft", "--font", "--mono", "--text-width",
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
	}
	for _, v := range append(vars, diagram.Variables()...) {
		if !slices.Contains(palette, v) {
			t.Errorf("%s is not of the palette", v)
		}
	}
	css, err := builtin.ReadFile("builtin/report/themes/default.css")
	if err != nil {
		t.Fatal(err)
	}
	defined := definedVariables(css)
	for _, v := range palette {
		if !defined[v] {
			t.Errorf("the report theme leaves %s out", v)
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
