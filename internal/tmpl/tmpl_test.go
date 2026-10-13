package tmpl_test

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

func render(t *testing.T, src string) string {
	t.Helper()
	d, err := tmpl.Read("data.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.Render("A title & more", "ja")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The report's example draws: Markdown text, HTML as it is, and the header from the data.
func TestReportExample(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("builtin/report/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := tmpl.Read("example.yaml", src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Template != "report" || d.Theme != "default" || d.Title != "Moving the team wiki" {
		t.Errorf("Read = %+v", d)
	}
	if d, err := tmpl.Read("data.yaml", []byte("template: report\ntitle: ' T '\n")); err != nil || d.Title != "T" {
		t.Errorf("Read of a title with spaces = %+v, %v", d, err)
	}
	got := render(t, string(src))
	for _, want := range []string{
		`<html lang="ja">`,
		`<title>A title &amp; more</title>`,
		`<h1>A title &amp; more</h1>`,
		`<p class="eyebrow">Plan · 2030-04</p>`,
		`<h2>Steps</h2>`,
		`<li>Freeze edits on the old wiki on <strong>Friday evening</strong>.</li>`,
		`<div class="table-wrap"><table>`,
		`<th>Likelihood</th>`,
		`<p class="note warn">Anyone editing during the freeze loses their changes.</p>`,
		`--accent: #1d6670;`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the example lacks %s:\n%s", want, got)
		}
	}
}

// Items the schema declares may be left out, and only html takes HTML: Markdown leaves it out.
func TestReportParts(t *testing.T) {
	t.Parallel()

	got := render(t, "template: report\nsections:\n  - parts:\n      - text: 'a <script>x()</script> <table> `</table>` **b**'\n  - heading: h\n")
	if strings.Contains(got, "<script>x()") || !strings.Contains(got, "<strong>b</strong>") || strings.Contains(got, `<div class="table-wrap">`) {
		t.Errorf("Markdown with raw HTML = %s", got)
	}
	if strings.Contains(got, `class="eyebrow"`) || strings.Contains(got, `class="lead"`) {
		t.Errorf("an eyebrow or a lead is drawn without one: %s", got)
	}
}

// Each refusal of Read points at the line and column it is about.
func TestReadRefusals(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ src, want string }{
		{"title: t\n", "data.yaml:1:1: name the template at the top level, like template: report (templates: report)"},
		{"", "data.yaml:1:1: name the template at the top level"},
		{"template: ''\n", "data.yaml:1:11: name the template at the top level"},
		{"- template: report\n", "data.yaml:1:1: write the top level as keys and values"},
		{"template: report\n---\ntitle: t\n", "data.yaml:2:1: a data file holds one YAML document"},
		{"template: report\n...\nlead: x\n", "data.yaml:3:1: a data file holds one YAML document"},
		{"template: report\ntemplate: report\n", `data.yaml:2:1: this is not valid YAML: mapping key "template" already defined`},
		{"template: report\n---\ntitle: 'a\n", "data.yaml:3:8: this is not valid YAML: "},
		{"template: report\nlead: &a x\n", "data.yaml:2:7: anchors, aliases and tags are not supported"},
		{"template: deck\n", `data.yaml:1:11: unknown template "deck"; use report`},
		{"template: report\ntheme: print\n", `data.yaml:2:8: template report has no theme "print"; use default`},
		{"template: report\ntitle: [a]\n", "data.yaml:2:8: title must be text"},
		{"template: report\ntitle: 'a\n", "data.yaml:2:8: this is not valid YAML: "},
	} {
		_, err := tmpl.Read("data.yaml", []byte(c.src))
		var at *yamlpos.Error
		if !errors.As(err, &at) || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("Read %q = %v, want %s", c.src, err, c.want)
		}
	}
	if _, err := tmpl.Read("data.yaml", []byte("%YAML 1.2\n---\ntemplate: report\n")); err != nil {
		t.Errorf("Read of a document after a %%YAML directive = %v", err)
	}
	// Data that passed its schema draws; a template that does not fit it is refused while drawing.
	d := &tmpl.Data{File: "data.yaml", Template: "report", Theme: "default", Title: "", Values: map[string]any{"sections": "no"}}
	if _, err := d.Render("t", "en"); err == nil || !strings.HasPrefix(err.Error(), "data.yaml does not fit the template report: ") {
		t.Errorf("Render of data of the wrong shape = %v", err)
	}
}

// Every mistake against the schema is reported at once, in the order of the file, each at its line
// and column, worded in the reader's language.
func TestSchemaMistakes(t *testing.T) {
	t.Parallel()

	src := `template: report
title: Bad
leed: a typo
sections:
  - heading: 1
    parts:
      - txt: hello
      - text: a
        html: <b>b</b>
      - {}
  - parts: nope
`
	_, err := tmpl.Read("bad.yaml", []byte(src))
	var p yamlpos.Problems
	if !errors.As(err, &p) {
		t.Fatalf("Read = %v, want problems", err)
	}
	want := []string{
		"bad.yaml:3:1: leed is not an item of the template report (a typo?); use eyebrow, lead, sections, template, theme, title",
		"bad.yaml:5:14: sections[0].heading: write text, not a number",
		"bad.yaml:7:9: sections[0].parts[0].txt is not an item of the template report (a typo?); use html, text",
		"bad.yaml:8:9: sections[0].parts[1]: the number of keys must be at most 1",
		"bad.yaml:10:9: sections[0].parts[2]: the number of keys must be at least 1",
		"bad.yaml:11:12: sections[1].parts: write a list, not text",
	}
	if got := p.Error(); got != strings.Join(want, "\n") {
		t.Errorf("the mistakes =\n%s\nwant\n%s", got, strings.Join(want, "\n"))
	}
	if ja := p.Localize(i18n.New("ja")); !strings.Contains(ja, "bad.yaml:11:12: sections[1].parts: 文字列ではなくリストで書いてください") {
		t.Errorf("the mistakes in Japanese:\n%s", ja)
	}
}

// Every built-in template has a page, a schema, an example that draws, and a default theme.
func TestBuiltins(t *testing.T) {
	t.Parallel()

	for _, name := range tmpl.Names() {
		for _, f := range []string{"page.html", "schema.json", "example.yaml", "themes/default.css"} {
			if _, err := fs.Stat(os.DirFS("builtin/"+name), f); err != nil {
				t.Errorf("template %s: %v", name, err)
			}
		}
		src, err := os.ReadFile("builtin/" + name + "/example.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if render(t, string(src)) == "" {
			t.Errorf("the example of %s draws nothing", name)
		}
	}
}
