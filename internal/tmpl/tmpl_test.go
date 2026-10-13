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

// builtins are the built-in templates alone.
var builtins = tmpl.NewLibrary("")

func render(t *testing.T, src string) string {
	t.Helper()
	d, err := builtins.Read("data.yaml", []byte(src), "", "")
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
	d, err := builtins.Read("example.yaml", src, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Template != "report" || d.Theme != "default" || d.Title != "Moving the team wiki" {
		t.Errorf("Read = %+v", d)
	}
	if d, err := builtins.Read("data.yaml", []byte("template: report\ntitle: ' T '\n"), "", ""); err != nil || d.Title != "T" {
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
		{"template: report\nlead: &a x\n", "data.yaml:2:7: anchors, aliases, merge keys (<<) and tags are not supported"},
		{"template: report\n<<: {lead: x}\n", "data.yaml:2:1: anchors, aliases, merge keys (<<) and tags are not supported"},
		{"template:\n", "data.yaml:1:1: template must be text"},
		{"template: report\n0x10: x\n", "data.yaml:2:1: a key is a name: write 0x10 as text, in quotes"},
		{"template: report\nsections:\n  - ~: x\n", "data.yaml:3:5: a key is a name: write ~ as text, in quotes"},
		{"template: report\nsections:\n  -\n", "data.yaml:3:3: sections[0]: write keys and values, not nothing"},
		{"template: report\nlead: .inf\n", "data.yaml:2:7: .inf is not a value a data file can hold"},
		{"template: report\nsections:\n  - heading: .NaN\n", "data.yaml:3:14: .NaN is not a value a data file can hold"},
		// The shared keys and the schema are reported together.
		{"template: report\ntheme: print\nleed: x\n", "data.yaml:2:8: template report has no theme \"print\"; use default\ndata.yaml:3:1: leed is not an item"},
		{"template: deck\n", `data.yaml:1:11: unknown template "deck"; use report`},
		{"template: report\ntheme: print\n", `data.yaml:2:8: template report has no theme "print"; use default`},
		{"template: report\ntitle: [a]\n", "data.yaml:2:8: title must be text"},
		{"template: report\ntitle: 'a\n", "data.yaml:2:8: this is not valid YAML: "},
	} {
		_, err := builtins.Read("data.yaml", []byte(c.src), "", "")
		var one *yamlpos.Error
		var all yamlpos.Problems
		if !errors.As(err, &one) && !errors.As(err, &all) || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("Read %q = %v, want %s", c.src, err, c.want)
		}
	}
	// An explicit key is found by its text after ?.
	if _, err := builtins.Read("data.yaml", []byte("template: report\n? lead\n: 3\n"), "", ""); err == nil || err.Error() != "data.yaml:3:3: lead: write text, not a number" {
		t.Errorf("Read of an explicit key = %v", err)
	}
	if _, err := builtins.Read("data.yaml", []byte("%YAML 1.2\n---\ntemplate: report\n"), "", ""); err != nil {
		t.Errorf("Read of a document after a %%YAML directive = %v", err)
	}
	// Data that passed its schema draws; a template that does not fit it is refused while drawing.
	d, err := builtins.Read("data.yaml", []byte("template: report\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	d.Values = map[string]any{"sections": "no"}
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
	_, err := builtins.Read("bad.yaml", []byte(src), "", "")
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

	names, err := builtins.Names()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
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

// A data file without its template is told so beside the other mistakes of the keys every template
// shares, and a theme: that is not text leaves the theme the default one.
func TestUnnamedBesideOtherMistakes(t *testing.T) {
	t.Parallel()

	_, err := tmpl.NewLibrary("").Read("d.yaml", []byte("title: 3\ntheme: [x]\n"), "", "")
	want := "d.yaml:1:1: name the template at the top level, like template: report (templates: report)\n" +
		"d.yaml:1:8: title must be text\nd.yaml:2:8: theme must be text"
	if err == nil || err.Error() != want {
		t.Errorf("Read = %v\nwant %s", err, want)
	}
	_, err = tmpl.NewLibrary("").Read("d.yaml", []byte("template: report\ntheme: [x]\n"), "", "")
	if want = "d.yaml:2:8: theme must be text"; err == nil || err.Error() != want {
		t.Errorf("Read of a theme not text = %v\nwant %s", err, want)
	}
}
