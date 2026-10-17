package tmpl

import (
	"regexp"
	"slices"
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
	} {
		if got := string(withStyles([]byte(page), "S")); got != want {
			t.Errorf("withStyles(%q) = %q, want %q", page, got, want)
		}
	}
}
