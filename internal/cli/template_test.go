package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// floe template new copies a template to edit, its example naming the copy; list and show tell of it.
func TestTemplateCommands(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	if r := e.run(t, "", "template", "list"); r.code != 0 || !strings.HasPrefix(r.stdout, "report\tbuilt-in\tdefault\t\tA page of sections") {
		t.Fatalf("template list = %+v", r)
	}

	spec := filepath.Join(e.tmp, "templates", "spec")
	if r := e.run(t, "", "template", "new", "spec", "--from", "report"); r.code != 0 || r.stdout != "made the template spec in "+spec+", a copy of report\n" {
		t.Fatalf("template new = %+v", r)
	}
	floe, err := os.ReadFile(filepath.Join(spec, "template.yaml"))
	if err != nil || string(floe) != "floe: 0.0\n" {
		t.Errorf("template.yaml = %q, %v", floe, err)
	}
	if r := e.run(t, "", "check", filepath.Join(spec, "example.yaml")); r.code != 0 {
		t.Errorf("check of the copy's example = %+v", r)
	}
	if r := e.run(t, "", "template", "list"); r.code != 0 || !strings.Contains(r.stdout, "\nspec\t"+spec+"\tdefault\t0.0.0\tA page of sections") {
		t.Errorf("template list with a copy = %+v", r)
	}
	if fi, serr := os.Stat(spec); serr != nil || fi.Mode().Perm() != 0o755 {
		t.Errorf("the copy's mode = %v, %v", fi, serr)
	}

	// A broken template is reported after the others, which it does not hide, and a description keeps
	// to its column.
	e.write(t, "templates/drafts/page.html", "")
	schema := filepath.Join(spec, "schema.json")
	src, err := os.ReadFile(schema)
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G703: the test's own directory
	if err := os.WriteFile(schema, []byte(strings.Replace(string(src), `"description": "A page`, `"description": "Two\nlines.\tA page`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(e.tmp, "gone"), filepath.Join(e.tmp, "templates", "memo")); err != nil {
		t.Fatal(err)
	}
	e.vars["FLOE_LANG"] = "ja"
	r := e.run(t, "", "template", "list")
	if r.code != 1 || !strings.Contains(r.stdout, "report\tbuilt-in\t") || !strings.Contains(r.stdout, "\tTwo lines. A page of sections") ||
		!strings.Contains(r.stderr, "schema.json of the template drafts") || !strings.Contains(r.stderr, "memo") ||
		strings.Count(r.stdout, "\n") != 2 {
		t.Errorf("template list with broken templates = %+v", r)
	}
	delete(e.vars, "FLOE_LANG")

	// A copy of a template of one's own without template.yaml needs no floe either.
	if r := e.run(t, "", "template", "new", "spec2", "--from", "spec"); r.code != 0 {
		t.Fatalf("template new from one's own = %+v", r)
	}
	if err := os.Remove(filepath.Join(e.tmp, "templates", "spec", "template.yaml")); err != nil {
		t.Fatal(err)
	}
	if r := e.run(t, "", "template", "new", "spec3", "--from", "spec"); r.code != 0 {
		t.Fatalf("template new from one's own without template.yaml = %+v", r)
	}
	if _, err := os.Stat(filepath.Join(e.tmp, "templates", "spec3", "template.yaml")); !os.IsNotExist(err) {
		t.Errorf("a copy of a template without template.yaml has one: %v", err)
	}

	for _, tt := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"new", "spec", "--from", "report"}, 1, "floe: spec is there already (" + spec + "); pick another name, or remove it\n"},
		{[]string{"new", "Spec", "--from", "report"}, 1, "floe: Spec cannot name a template"},
		{[]string{"new", "memo", "--from", "nope"}, 1, `floe: unknown template "nope"; use drafts, memo, report, spec, spec2, spec3`},
		{[]string{"new", "memo"}, 2, "floe: name the template to copy with --from, like --from report"},
		{[]string{"show", "nope"}, 1, `floe: unknown template "nope"`},
	} {
		if r := e.run(t, "", append([]string{"template"}, tt.args...)...); r.code != tt.code || !strings.Contains(r.stderr, tt.want) {
			t.Errorf("template %v = %+v, want %d and %s", tt.args, r, tt.code, tt.want)
		}
	}
}

// floe template new under a file where the directory of templates would be says what failed, not that
// the template is there.
func TestTemplateNewUnderAFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_TEMPLATES_DIR"] = e.write(t, "templates", "")
	if r := e.run(t, "", "template", "new", "spec", "--from", "report"); r.code != 1 || strings.Contains(r.stderr, "is there already") ||
		!strings.Contains(r.stderr, "make the template spec") {
		t.Errorf("template new under a file = %+v", r)
	}
}

// floe template show tells where a template is, its items with what they take, and its example.
func TestTemplateShow(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	r := e.run(t, "", "template", "show", "report")
	for _, want := range []string{
		"report: A page of sections",
		"\nwhere: built-in\nthemes: default\n\nitems:\n",
		"\n  eyebrow: text — A short line above the title",
		"\n  sections: a list of keys and values\n    heading: text\n    parts: a list of keys and values — One key naming the kind of the part\n",
		"\n      html: text, format html — HTML that goes in as it is\n",
		"\nexample.yaml:\n# A report",
		"\ntemplate: report\n",
	} {
		if r.code != 0 || !strings.Contains(r.stdout, want) {
			t.Errorf("template show lacks %q:\n%s%s", want, r.stdout, r.stderr)
		}
	}
	e.vars["FLOE_LANG"] = "ja"
	if r := e.run(t, "", "template", "show", "report"); !strings.Contains(r.stdout, "      html: 文字列、形式 html — HTML that goes in as it is\n") {
		t.Errorf("template show in Japanese:\n%s", r.stdout)
	}
	delete(e.vars, "FLOE_LANG")

	// A list that may be null says so, and a description keeps to its line.
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	e.write(t, "templates/memo/schema.json", `{"properties": {"tags": {"type": ["array", "null"], "items": {"type": "string"},
  "description": "Two\nlines"}}}`)
	e.write(t, "templates/memo/themes/default.css", "")
	if r := e.run(t, "", "template", "show", "memo"); !strings.Contains(r.stdout, "\n  tags: a list of text or nothing — Two lines\n") {
		t.Errorf("template show of a list that may be null:\n%s%s", r.stdout, r.stderr)
	}
}
