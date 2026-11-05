package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const tooDeep = "html: open stack of elements exceeds 512 nodes\n"

// bent is what a page holds under 503 <div> that the parser takes, and written, takes no more.
var bent = strings.Repeat("<div>", 503) + "<form><i><nobr><h2></form><div><li><button><h1>"

// deepPage is a directory with d.yaml, of the template deep of the body given, and the environment that finds it.
func deepPage(t *testing.T, body string) (string, map[string]string) {
	t.Helper()
	dir := write(t, map[string]string{
		"d.yaml":                      "template: deep\nlang: en\ntitle: E\n",
		"templates/deep/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"templates/deep/example.yaml": "template: deep\nlang: en\ntitle: E\n",
		"templates/deep/page.html": `<!doctype html><html lang="{{.Lang}}"><head><meta name="viewport" ` +
			`content="width=device-width, initial-scale=1"><title>{{.Title}}</title></head><body>` + body + "</body></html>",
	})
	return dir, map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
}

// The parser counts the elements open, <html> and <body> among them: 510 in a page.html go in, 511 do not, and the
// mistake is the library's, in its words, at the file alone, as what went so deep has no one place.
func TestPageTooDeep(t *testing.T) {
	t.Parallel()

	dir, env := deepPage(t, strings.Repeat("<div>", 510))
	if code, _, errs := runIn(dir, env, "render", "d.yaml"); code != 0 || errs != "" {
		t.Errorf("render of 510 = %d, %q", code, errs)
	}
	dir, env = deepPage(t, strings.Repeat("<div>", 511))
	for _, args := range [][]string{{"render", "d.yaml"}, {"render", "--check", "d.yaml"}, {"check", "d.yaml"}} {
		if code, _, errs := runIn(dir, env, args...); code != 1 || errs != "d.yaml: "+tooDeep {
			t.Errorf("%v = %d, %q", args, code, errs)
		}
	}
	example := filepath.Join(dir, "templates", "deep", "example.yaml")
	if code, _, errs := runIn(dir, env, "template", "check"); code != 1 || errs != example+": "+tooDeep {
		t.Errorf("template check = %d, %q", code, errs)
	}

	quotes := "template: report\nlang: en\ntitle: A\nsections:\n  - heading: A\n    parts:\n      - text: \"" +
		strings.Repeat("> ", 600) + "x\"\n"
	dir = write(t, map[string]string{"q.yaml": quotes})
	if code, _, errs := runIn(dir, nil, "render", "q.yaml"); code != 1 || errs != "q.yaml: "+tooDeep {
		t.Errorf("render of a deep quote = %d, %q", code, errs)
	}
}

// What the library will not write is the page's mistake, not floe's failure.
func TestPageNotWritten(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{
		"v.yaml": "template: report\nlang: en\ntitle: A\nsections:\n  - heading: A\n    parts:\n" +
			"      - html: \"<svg><col>x</col></svg>\"\n",
	})
	if code, _, errs := runIn(dir, nil, "render", "v.yaml"); code != 1 ||
		errs != "v.yaml: html: void element <col> has child nodes\n" {
		t.Errorf("render = %d, %q", code, errs)
	}
}

// render writes the tree check checked, so that a page check takes is written, its CSP first in its head, though
// written and read again it goes past the parser's depth.
func TestRenderDoesNotReadAgain(t *testing.T) {
	t.Parallel()

	dir, env := deepPage(t, bent)
	if code, _, errs := runIn(dir, env, "check", "d.yaml"); code != 0 || errs != "" {
		t.Errorf("check = %d, %q", code, errs)
	}
	if code, _, errs := runIn(dir, env, "render", "d.yaml"); code != 0 || errs != "" {
		t.Errorf("render = %d, %q", code, errs)
	}
	page, err := os.ReadFile(filepath.Join(dir, "d.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(page, []byte(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"/>`+
		`<meta http-equiv="Content-Security-Policy"`)) {
		t.Errorf("d.html starts %q", page[:min(len(page), 120)])
	}
	if _, err := html.Parse(bytes.NewReader(page)); err == nil || err.Error()+"\n" != tooDeep {
		t.Errorf("read again: %v; if the parser takes such a page now, this test no longer holds such a page", err)
	}
}

// A raw SVG the library will not read or write is told at the value, in the library's words.
func TestRawNotRead(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ svg, want string }{
		{strings.Repeat("<g>", 511), ""},
		{strings.Repeat("<g>", 512), "spec.yaml:6:12: " + tooDeep},
		{"<col>x</col>", "spec.yaml:6:12: html: void element <col> has child nodes\n"},
	} {
		dir := write(t, map[string]string{
			"spec.yaml": "title: t\nwidth: 10\nheight: 10\nparts:\n  - raw:\n      svg: \"" + c.svg + "\"\n",
		})
		if _, _, errs := runIn(dir, nil, "diagram", "render", "--no-assets", "spec.yaml"); errs != c.want {
			t.Errorf("%.20s: %q, want %q", c.svg, errs, c.want)
		}
	}
}
