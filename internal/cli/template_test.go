package cli_test

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/version"
)

// floe template new copies a template to edit, its example naming the copy; list and show tell of it.
func TestTemplateCommands(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	if r := e.run(t, "", "template", "list"); r.code != 0 || !strings.HasPrefix(r.stdout, "report\tbuilt-in\t\tA page of sections") {
		t.Fatalf("template list = %+v", r)
	}

	spec := filepath.Join(e.tmp, "templates", "spec")
	if r := e.run(t, "", "template", "new", "spec", "--from", "report"); r.code != 0 || r.stdout != "made the template spec in "+spec+", a copy of report\n" {
		t.Fatalf("template new = %+v", r)
	}
	release := strings.SplitN(version.Version, ".", 3)
	floe, err := os.ReadFile(filepath.Join(spec, "template.yaml"))
	if err != nil || string(floe) != "floe: "+release[0]+"."+release[1]+"\n" {
		t.Errorf("template.yaml = %q, %v", floe, err)
	}
	if r := e.run(t, "", "check", filepath.Join(spec, "example.yaml")); r.code != 0 {
		t.Errorf("check of the copy's example = %+v", r)
	}
	if r := e.run(t, "", "template", "check", spec); r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("template check of the copy = %+v", r)
	}
	// Warnings leave the exit code 0; mistakes, at their lines and columns, make it 1.
	if err = os.Remove(filepath.Join(spec, "template.yaml")); err != nil {
		t.Fatal(err)
	}
	if r := e.run(t, "", "template", "check", spec); r.code != 0 || !strings.HasPrefix(r.stderr, "floe: "+spec+" has no template.yaml") {
		t.Errorf("template check without template.yaml = %+v", r)
	}
	computed := filepath.Join(spec, "computed.yaml")
	if err = os.WriteFile(computed, []byte("total: nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := e.run(t, "", "template", "check", spec); r.code != 1 || !strings.Contains(r.stderr, "\n"+computed+":1:8: nope cannot be read as an expression: unknown name nope\n") {
		t.Errorf("template check of a broken template = %+v", r)
	}
	if r := e.run(t, "", "template", "check", spec, spec+"/."); r.code != 1 || strings.Count(r.stderr, "nope cannot be read") != 1 {
		t.Errorf("template check of a template named twice = %+v", r)
	}
	if err = os.Remove(computed); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(spec, "template.yaml"), floe, 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	// A part of one's own that cannot be used is told once, however many templates are checked.
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	if err = os.MkdirAll(filepath.Join(e.tmp, "parts", "base"), 0o700); err != nil {
		t.Fatal(err)
	}
	if r := e.run(t, "", "template", "check", spec, spec); r.code != 0 || strings.Count(r.stderr, "the part base cannot be used") != 1 {
		t.Errorf("template check of two templates with a part that cannot be used = %+v", r)
	}
	delete(e.vars, "FLOE_PARTS_DIR")
	if r := e.run(t, "", "template", "list"); r.code != 0 || !strings.Contains(r.stdout, "\nspec\t"+spec+"\t"+release[0]+"."+release[1]+".0\tA page of sections") {
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
	r := e.run(t, "", "template", "list")
	if r.code != 1 || !strings.Contains(r.stdout, "report\tbuilt-in\t") || !strings.Contains(r.stdout, "\tTwo lines. A page of sections") ||
		!strings.Contains(r.stderr, "cannot read "+filepath.Join(e.tmp, "templates", "drafts", "schema.json")) || !strings.Contains(r.stderr, "memo") ||
		strings.Count(r.stdout, "\n") != 2 {
		t.Errorf("template list with broken templates = %+v", r)
	}

	// A copy of a template of one's own keeps its template.yaml as written, and names itself.
	kept := "# kept\nfloe: " + release[0] + "." + release[1] + ".0\n"
	e.write(t, "templates/spec/template.yaml", kept)
	if r := e.run(t, "", "template", "new", "spec2", "--from", "spec"); r.code != 0 {
		t.Fatalf("template new from one's own = %+v", r)
	}
	if b, err := os.ReadFile(filepath.Join(e.tmp, "templates", "spec2", "template.yaml")); err != nil || string(b) != kept {
		t.Errorf("the copy's template.yaml = %q, %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(e.tmp, "templates", "spec2", "example.yaml")); err != nil || !strings.Contains(string(b), "\ntemplate: spec2\n") {
		t.Errorf("the copy's example = %s, %v", b, err)
	}

	// A template.yaml that does not read stops the copy, which leaves nothing at its name.
	e.write(t, "templates/spec/template.yaml", "floe: x\n")
	if r := e.run(t, "", "template", "new", "spec4", "--from", "spec"); r.code != 1 || !strings.Contains(r.stderr, "template.yaml:1:7") {
		t.Errorf("template new from a broken template.yaml = %+v", r)
	}
	if _, err := os.Stat(filepath.Join(e.tmp, "templates", "spec4")); !os.IsNotExist(err) {
		t.Errorf("a failed copy left spec4: %v", err)
	}

	// A copy of a template of one's own without template.yaml needs no floe either.
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

// floe template new where the directory of templates is a file is told so by its setting, not that the
// template is there.
func TestTemplateNewUnderAFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_TEMPLATES_DIR"] = e.write(t, "templates", "")
	want(t, "template new where the templates are a file", e.run(t, "", "template", "new", "spec", "--from", "report"), 1, "",
		"floe: FLOE_TEMPLATES_DIR, "+e.vars["FLOE_TEMPLATES_DIR"]+", is not a directory; set FLOE_TEMPLATES_DIR to a directory")
}

// floe template show tells where a template is, its items with what they take, and its example.
func TestTemplateShow(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	r := e.run(t, "", "template", "show", "report")
	for _, want := range []string{
		"report: A page of sections",
		"\nwhere: built-in\nthemes: blueprint, brutal, default, editorial, ink, notebook, soft, swiss, washi\n\nitems:\n",
		"\n  eyebrow: text — A short line above the title",
		"\n  lang: text, required — The page's language, a BCP 47 tag such as en, ja or pt-BR\n",
		"\n  template: text, one of report, required — The template that draws the page\n",
		"\n  sections: a list of keys and values\n    heading: text\n    parts: a list of keys and values — One key naming the kind of the part\n",
		"\n      html: text, format html — HTML that goes in as it is\n",
		"\nexample.yaml:\n# A report",
		"\ntemplate: report\n",
	} {
		if r.code != 0 || !strings.Contains(r.stdout, want) {
			t.Errorf("template show lacks %q:\n%s%s", want, r.stdout, r.stderr)
		}
	}

	// A list that may be null says so, and a description keeps to its line.
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	e.write(t, "templates/memo/schema.json", `{"properties": {"tags": {"type": ["array", "null"], "items": {"type": "string"},
  "description": "Two\nlines"}, "gone": null}}`)
	if r := e.run(t, "", "template", "show", "memo"); !strings.Contains(r.stdout, "\n  gone\n  lang: text, required — The page's language, a BCP 47 tag such as en, ja or pt-BR\n  tags: a list of text or nothing — Two lines\n") {
		t.Errorf("template show of a list that may be null:\n%s%s", r.stdout, r.stderr)
	}
}

// The images of a data file are read from its directory, a relative one under the working directory,
// by floe check and floe publish alike, and go into the page.
func TestDataImages(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	if err := os.CopyFS(filepath.Join(e.tmp, "templates", "sizes"), os.DirFS("../tmpl/testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	png, err := os.ReadFile("../tmpl/testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	e.write(t, "work/report/chart.png", string(png))
	e.write(t, "work/report/sizes.yaml", "template: sizes\nfiles: [{name: a, bytes: 1}]\nchart: chart.png\nlang: en\n")
	if r := e.run(t, "work", "check", "report/sizes.yaml"); r.code != 0 {
		t.Errorf("check = %+v", r)
	}
	if r := e.run(t, "work", "publish", "report/sizes.yaml"); r.code != 0 {
		t.Fatalf("publish = %+v", r)
	}
	if kept, globErr := filepath.Glob(filepath.Join(e.vars["FLOE_DIR"], "sizes", "source-*", "chart.png")); globErr != nil || len(kept) != 1 {
		t.Errorf("the published page keeps images %v (%v), want chart.png", kept, globErr)
	}
	e.write(t, "work/report/sizes.yaml", "template: sizes\nfiles: [{name: a, bytes: 1}]\nchart: ../chart.png\nlang: en\n")
	if r := e.run(t, "work", "check", "report/sizes.yaml"); r.code != 1 || !strings.Contains(r.stderr, `report/sizes.yaml:3:8: chart: "../chart.png" is not a file under`) {
		t.Errorf("check of an image out of the directory = %+v", r)
	}
}

// A linked data file is drawn with the server's templates of one's own, a relative directory under the
// working directory of the publish that started it. A publish that means other templates is refused
// before it stores anything, rather than linking a page the server would draw with others.
func TestLinkDataFileOfOwnTemplate(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	for _, dir := range []string{"templates", "others"} {
		if err := os.CopyFS(filepath.Join(e.tmp, dir, "sizes"), os.DirFS("../tmpl/testdata/sizes")); err != nil {
			t.Fatal(err)
		}
	}
	e.write(t, "work/a.yaml", "template: sizes\nfiles: [{name: a, bytes: 1536}]\nlang: en\n")
	e.write(t, "work/b.yaml", "template: sizes\nfiles: [{name: a, bytes: 1536}]\nlang: en\n")
	e.vars["FLOE_TEMPLATES_DIR"] = "../templates"
	if r := e.run(t, "work", "publish", "a.yaml", "--link"); r.code != 0 {
		t.Fatalf("publish --link a = %+v", r)
	}
	if page := e.fetch(t, "/a/a/?raw"); !strings.Contains(page, "<td>1.5 KiB</td>") {
		t.Errorf("the linked page a:\n%s", page)
	}
	e.vars["FLOE_TEMPLATES_DIR"] = "../others"
	want(t, "publish --link with other templates", e.run(t, "work", "publish", "b.yaml", "--link"), 1, "",
		"floe: the server on port "+e.port+" draws pages with the templates of "+filepath.Join(e.tmp, "templates"))
	if _, err := os.Stat(filepath.Join(e.vars["FLOE_DIR"], "b")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the refused link was stored: %v", err)
	}
}

// fetch returns the body the server answers path with.
func (e *env) fetch(t *testing.T, path string) string {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, e.base()+path, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The server a publish starts draws the diagrams of a linked data file with the icons of a relative
// FLOE_ICONS_DIR under the working directory of that publish, as its check did.
func TestLinkDataFileOfRelativeIcons(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	if err := os.CopyFS(filepath.Join(e.tmp, "work", "icons"), os.DirFS("../diagram/testdata/icons")); err != nil {
		t.Fatal(err)
	}
	e.write(t, "work/a.yaml", "template: report\nsections:\n  - parts:\n      - diagram:\n          id: a\n"+
		"          width: 100\n          height: 50\n          title: t\n          parts:\n"+
		"            - icon: { x: 10, y: 5, name: aws/aws-lambda, size: 24 }\nlang: en\n")
	e.vars["FLOE_ICONS_DIR"] = "icons"
	if r := e.run(t, "work", "publish", "a.yaml", "--link"); r.code != 0 {
		t.Fatalf("publish --link = %+v", r)
	}
	if page := e.fetch(t, "/a/a/?raw"); !strings.Contains(page, `<use href="#a_icon-`) {
		t.Errorf("the linked page:\n%s", page)
	}
}

// floe template check tells the mistakes of a part of one's own once, though the parts that take its look
// are each warned of; floe's that hold it stay usable.
func TestTemplateCheckBrokenPartOnce(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	e.write(t, "parts/code/schema.json", `{"type": "object"}`)
	e.write(t, "parts/code/part.html", `<div data-floe-part="code"></div>`)
	e.write(t, "parts/code/part.css", "}\n")
	// A template that holds none of the parts code leaves out.
	e.write(t, "templates/spec/template.yaml", "floe: "+strings.Join(strings.SplitN(version.Version, ".", 3)[:2], ".")+"\n")
	e.write(t, "templates/spec/schema.json", `{"type": "object", "properties": {"lead": {"type": "string"}}, "additionalProperties": false}`)
	e.write(t, "templates/spec/page.html", "<!doctype html><html><head><title>{{.Title}}</title></head><body>{{.Data.lead}}</body></html>")
	e.write(t, "templates/spec/example.yaml", "template: spec\nlang: en\nlead: x\n")
	spec := filepath.Join(e.tmp, "templates", "spec")
	r := e.run(t, "", "template", "check", spec)
	css := filepath.Join(e.tmp, "parts", "code", "part.css") + ":1: this } closes no block"
	if r.code != 0 || strings.Count(r.stderr, css) != 1 || !strings.Contains(r.stderr, "the part diff cannot be used") ||
		strings.Contains(r.stderr, "the part stack cannot be used") {
		t.Errorf("template check = %+v, want %s once", r, css)
	}
}

// A part of one's own that cannot be used stops only the data files that take it, in a check of several and
// in a publish, which the server checks as the command line does.
func TestUnusablePartWhereTakenOnly(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	e.write(t, "parts/links/schema.json", `{"type": "array", "items": {"type": "object", "properties": {"u": {"pattern": "(["}}}}`)
	e.write(t, "parts/links/part.html", `<ul data-floe-part="links"></ul>`)
	e.write(t, "parts/links/part.css", "")
	page := "template: report\nlang: en\ntitle: t\nsections:\n  - heading: h\n    parts:\n"
	none := e.write(t, "none.yaml", page+"      - text: hi\n")
	takes := e.write(t, "takes.yaml", page+"      - links: [{ title: a, url: /a }]\n")
	at := takes + ":7:16: the part links cannot be used\n  " + filepath.Join(e.tmp, "parts", "links", "schema.json")
	if r := e.run(t, "", "check", none, takes); r.code != 1 || !strings.HasPrefix(r.stderr, at) || strings.Contains(r.stderr, "none.yaml") {
		t.Errorf("check of both = %+v, want %s alone", r, at)
	}
	want(t, "publish of the one that does not take it", e.run(t, "", "publish", none), 0, e.base()+"/a/none/\n", "")
	if r := e.run(t, "", "publish", takes); r.code != 1 || !strings.HasPrefix(r.stderr, at) {
		t.Errorf("publish of the one that takes it = %+v, want %s", r, at)
	}
}

// A directory of parts whose name no $ref can name, as %zz, which is no URL, stops no page.
func TestPartNameNoURL(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	if err := os.MkdirAll(filepath.Join(e.tmp, "parts", "%zz"), 0o750); err != nil {
		t.Fatal(err)
	}
	page := e.write(t, "page.yaml", "template: report\nlang: en\ntitle: t\nsections:\n  - heading: h\n    parts:\n      - text: hi\n")
	want(t, "check", e.run(t, "", "check", page), 0, "", "")
}

// floe template show and floe part show tell an item a part that cannot be used draws as such.
func TestShowUnusablePart(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	for _, name := range []string{"links", "note"} {
		e.write(t, "parts/"+name+"/schema.json", `{"type": 1}`)
		e.write(t, "parts/"+name+"/part.html", `<div data-floe-part="`+name+`"></div>`)
		e.write(t, "parts/"+name+"/part.css", "")
	}
	for _, c := range []struct{ args, line string }{
		{"template show report", "\n      links: the part links cannot be used\n"},
		{"part show stack", "\n  note: the part note cannot be used\n"},
	} {
		if r := e.run(t, "", strings.Fields(c.args)...); r.code != 0 || !strings.Contains(r.stdout, c.line) {
			t.Errorf("%s = %+v, want %q", c.args, r, c.line)
		}
	}
}

// A table of Markdown takes the look of the table part: with a table of one's own that cannot be used,
// the item of Markdown is refused where it is written, the first time with why, in each file.
func TestUnusableTableInMarkdown(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	e.write(t, "parts/table/schema.json", `{"type": 1}`)
	e.write(t, "parts/table/part.html", `<div data-floe-part="table"></div>`)
	e.write(t, "parts/table/part.css", "")
	page := "template: report\nlang: en\ntitle: t\nsections:\n  - heading: h\n    parts:\n"
	table := func(indent string) string {
		return indent + "| a | b |\n" + indent + "|---|---|\n" + indent + "| 1 | 2 |\n"
	}
	a := e.write(t, "a.yaml", page+"      - text: |\n"+table("          ")+"      - note:\n          text: |\n"+table("            "))
	b := e.write(t, "b.yaml", page+"      - text: hi\n      - text: |\n"+table("          "))
	// HTML of the data that names it is told the same way, so that the one after it is told as above.
	c := e.write(t, "c.yaml", page+"      - html: '<div data-floe-part=\"table\"></div>'\n      - text: |\n"+table("          "))
	stderr := a + ":7:15: the part table cannot be used\n" +
		"  " + filepath.Join(e.tmp, "parts", "table", "schema.json") + ": #/type: JSON Schema does not take this: 'anyOf' failed\n" +
		a + ":12:11: the part table cannot be used, as told above\n" +
		b + ":8:15: the part table cannot be used, as told above\n"
	want(t, "check", e.run(t, "", "check", a, b), 1, "", stderr)
	stderr = c + ":7:15: the part table cannot be used\n" +
		"  " + filepath.Join(e.tmp, "parts", "table", "schema.json") + ": #/type: JSON Schema does not take this: 'anyOf' failed\n" +
		c + ":8:15: the part table cannot be used, as told above\n"
	want(t, "check of HTML and Markdown", e.run(t, "", "check", c), 1, "", stderr)
}
