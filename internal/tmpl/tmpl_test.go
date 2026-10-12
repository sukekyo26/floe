package tmpl_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
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
	got := render(t, string(src))
	for _, want := range []string{
		`<html lang="ja">`,
		`<title>A title &amp; more</title>`,
		`<h1>A title &amp; more</h1>`,
		`<p class="eyebrow">Plan · 2030-04</p>`,
		`<h2>Steps</h2>`,
		`<li>Freeze edits on the old wiki on <strong>Friday evening</strong>.</li>`,
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

	got := render(t, "template: report\nsections:\n  - parts:\n      - text: 'a <script>x()</script> **b**'\n  - heading: h\n")
	if strings.Contains(got, "<script>x()") || !strings.Contains(got, "<strong>b</strong>") {
		t.Errorf("Markdown with raw HTML = %s", got)
	}
	if strings.Contains(got, `class="eyebrow"`) || strings.Contains(got, `class="lead"`) {
		t.Errorf("an eyebrow or a lead is drawn without one: %s", got)
	}
}

func TestReadRefusals(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ src, want string }{
		{"title: t\n", "data.yaml: name the template at the top, like template: report (templates: report)"},
		{"", "data.yaml: name the template at the top"},
		{"template: deck\n", `data.yaml: unknown template "deck"; use report`},
		{"template: report\ntheme: print\n", `data.yaml: template report has no theme "print"; use default`},
		{"template: report\ntitle: [a]\n", "data.yaml: title must be text"},
		{"template: report\ntitle: 'a\n", "data.yaml is not valid YAML: "},
	} {
		_, err := tmpl.Read("data.yaml", []byte(c.src))
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("Read %q = %v, want %s", c.src, err, c.want)
		}
	}
	d, err := tmpl.Read("data.yaml", []byte("template: report\nsections: no\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Render("t", "en"); err == nil || !strings.HasPrefix(err.Error(), "data.yaml does not fit the template report: ") {
		t.Errorf("Render of data of the wrong shape = %v", err)
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
