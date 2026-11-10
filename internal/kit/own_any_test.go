package kit_test

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"floe/internal/doc"
	"floe/internal/i18n"
	"floe/internal/kit"
	"floe/internal/tmpl"
	"floe/internal/usererr"
)

const (
	quoteSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "required": ["text"], ` +
		`"additionalProperties": false, "properties": {"text": {"type": "string"}}}`
	boxSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", ` +
		`"properties": {"items": {"type": "array", "items": {"$ref": "floe:///parts/any.json"}}}}`
)

// ownParts is a Dir of parts of one's own: quote, box (whose items are any), the look frame and the shared shade,
// with files over them.
func ownParts(over map[string]string) kit.Dir {
	files := map[string]string{
		"quote/schema.json": quoteSchema, "quote/part.html": `<div data-floe-part="quote">{{.text}}</div>`, "quote/part.css": "",
		"box/schema.json": boxSchema, "box/part.html": `<div data-floe-part="box">{{range .items}}{{range .}}{{.}}{{end}}{{end}}</div>`,
		"box/part.css": "", "frame/part.css": "",
		"shade.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "enum": ["a"]}`,
	}
	for name, src := range over {
		files[name] = src
	}
	fsys := fstest.MapFS{}
	for name, src := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(src)}
	}
	return kit.Dir{FS: fsys, Name: "own/parts"}
}

// section is a report of one section holding the part written as YAML.
func section(part string) string {
	return "template: report\nlang: en\ntitle: T\nsections:\n  - heading: a\n    parts:\n      - " + part + "\n"
}

// valid tells what k's report says of data, "" for nothing.
func valid(t *testing.T, k *kit.Kit, data string) string {
	t.Helper()

	r, ok, err := k.Template("report")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("no report")
	}
	d, err := doc.Read("x.yaml", []byte(data), "")
	if err != nil {
		t.Fatal(told(err))
	}
	return told(r.Schema().Validate(d))
}

// One's own parts join any, so that a section of floe's templates takes them, a part of one's own holding any
// too; its looks and shared schemas do not, nor does stack (details and the like), and floe's Kit is left as it is.
func TestOwnPartInAny(t *testing.T) {
	t.Parallel()

	k, err := kit.Load(floe(t), kit.Sources{Parts: ownParts(nil)})
	if err != nil {
		t.Fatal(told(err))
	}
	for _, tt := range []struct{ name, part, want string }{
		{"quote", "quote: { text: a }", ""},
		{"quote in box", "box: { items: [ { quote: { text: a } } ] }", ""},
		{"a misspelt quote", "qoute: { text: a }", "x.yaml:7:9: additional properties 'qoute' not allowed"},
		{"the look", "frame: {}", "x.yaml:7:9: additional properties 'frame' not allowed"},
		{"the shared schema", "shade: a", "x.yaml:7:9: additional properties 'shade' not allowed"},
		{"quote in details", "details: { title: d, parts: [ { quote: { text: a } } ] }", "x.yaml:7:41: additional properties 'quote' not allowed"},
	} {
		if got := valid(t, k, section(tt.part)); got != tt.want {
			t.Errorf("%s: told %q, want %q", tt.name, got, tt.want)
		}
	}
	if got, want := valid(t, floe(t), section("quote: { text: a }")), "x.yaml:7:9: additional properties 'quote' not allowed"; got != want {
		t.Errorf("floe's report told %q, want %q", got, want)
	}
}

// floe's templates compiled again over one's own parts are found by Template alone: one's own of a name comes first,
// and the lists of one's own templates hold none of floe's.
func TestOwnPartRebuilt(t *testing.T) {
	t.Parallel()

	k, err := kit.Load(floe(t), kit.Sources{Parts: ownParts(nil)})
	if err != nil {
		t.Fatal(told(err))
	}
	if own := k.OwnTemplates(); len(own) != 0 {
		t.Errorf("OwnTemplates = %v, want none", own)
	}
	report := fstest.MapFS{
		"report/page.html":    {Data: []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body></body></html>`)},
		"report/schema.json":  {Data: []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`)},
		"report/example.yaml": {Data: []byte("template: report\nlang: en\ntitle: T\n")},
	}
	mine, err := kit.Load(floe(t), kit.Sources{Parts: ownParts(nil), Templates: kit.Dir{FS: report, Name: "own/templates"}})
	if err != nil {
		t.Fatal(told(err))
	}
	r, ok, err := mine.Template("report")
	if err != nil || !ok {
		t.Fatalf("report: %v, %v", ok, err)
	}
	if r.File("schema.json") != "own/templates/report/schema.json" {
		t.Errorf("report is %v, want one's own", r.File("schema.json"))
	}
}

// A mistake of one's own part is told at its file, never as floe's failure to compile its templates again; that
// failure is floe's own, its mistakes worded as text.
func TestOwnPartMistake(t *testing.T) {
	t.Parallel()

	_, err := kit.Load(floe(t), kit.Sources{Parts: ownParts(map[string]string{
		"quote/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"x": {"$ref": "floe:///parts/nope.json"}}}`,
	})})
	r := usererr.Tell(i18n.EnglishCatalog(), err)
	// It is told at box, which holds any, too (schema.go's LIMIT on a chain of $refs).
	if r.Failure != "" || len(r.Mistakes) == 0 || !strings.Contains(told(err), "own/parts/quote/schema.json: ") {
		t.Errorf("a mistake of quote told %q (failure %q)", told(err), r.Failure)
	}
	mistake := usererr.List{usererr.NewAt("floe/report.json", 1, 2, "schema_no_url", "x")}
	r = usererr.Tell(i18n.EnglishCatalog(), kit.FloeFailure("report", mistake))
	if r.Mistakes != nil || !strings.HasPrefix(r.Failure, "kit: floe's template report does not compile with your own parts: floe/report.json:1:2: ") {
		t.Errorf("floeFailure told %+v, want floe's failure with the mistake's place", r)
	}
}

// withSchema returns the names of floe's templates with a schema: those a Kit with parts of its own compiles again.
func withSchema(t *testing.T) []string {
	t.Helper()

	var names []string
	_, _, _, templates := floe(t).Names()
	for _, name := range templates {
		tm, _, err := floe(t).Template(name)
		if err != nil {
			t.Fatal(err)
		}
		if tm.Schema() != nil {
			names = append(names, name)
		}
	}
	return names
}

// ownKit returns floe's Kit with the parts of ownParts over it.
func ownKit(t *testing.T) *kit.Kit {
	t.Helper()

	k, err := kit.Load(floe(t), kit.Sources{Parts: ownParts(nil)})
	if err != nil {
		t.Fatal(told(err))
	}
	return k
}

// Every template of floe's with a schema compiles again with one's own parts: Load no longer compiles them, so that
// a failure of floe's to would show here first.
func TestOwnPartEveryTemplate(t *testing.T) {
	t.Parallel()

	k := ownKit(t)
	for _, name := range withSchema(t) {
		again, ok, err := k.Template(name)
		if err != nil || !ok {
			t.Fatalf("%s: %v, %v", name, ok, err)
		}
		base, _, err := floe(t).Template(name)
		if err != nil {
			t.Fatal(err)
		}
		if again.Schema() == base.Schema() {
			t.Errorf("%s is floe's own, not compiled again", name)
		}
	}
}

// A template is compiled again when first taken, not when the Kit is read, and once.
func TestOwnPartCompiledWhenTaken(t *testing.T) {
	t.Parallel()

	k := ownKit(t)
	if n := kit.Compiled(k); n != 0 {
		t.Errorf("compiled %d when read, want 0", n)
	}
	for range 2 {
		if _, _, err := k.Template("report"); err != nil {
			t.Fatal(err)
		}
		if n := kit.Compiled(k); n != 1 {
			t.Errorf("compiled %d once report was taken, want 1", n)
		}
	}
}

// Goroutines taking every template of one Kit at once each get the same one, compiled once.
func TestOwnPartTakenOnce(t *testing.T) {
	t.Parallel()

	k := ownKit(t)
	names := withSchema(t)
	const takers = 8
	got := make([][]*kit.Template, takers)
	var wg sync.WaitGroup
	for i := range takers {
		wg.Go(func() {
			for _, name := range names {
				tm, _, err := k.Template(name)
				if err != nil {
					t.Error(err)
				}
				got[i] = append(got[i], tm)
			}
		})
	}
	wg.Wait()
	for i := range takers {
		if !slices.Equal(got[i], got[0]) {
			t.Errorf("taker %d got other templates than taker 0", i)
		}
	}
	if n := kit.Compiled(k); n != len(names) {
		t.Errorf("compiled %d, want %d", n, len(names))
	}
}

// floe's failure to compile a template again stops what draws with it, as floe's failure, not a mistake of the data;
// the other templates draw.
func TestOwnPartRebuildFails(t *testing.T) {
	t.Parallel()

	k := ownKit(t)
	kit.FailRebuilt(k, "report")
	_, _, err := tmpl.Draw(k, "x.yaml", []byte(section("quote: { text: a }")), tmpl.Options{})
	r := usererr.Tell(i18n.EnglishCatalog(), err)
	if r.Mistakes != nil || !strings.HasPrefix(r.Failure, "kit: floe's template report does not compile with your own parts: ") {
		t.Errorf("report told %+v, want floe's failure", r)
	}
	adr, _, err := k.Template("adr")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := tmpl.Draw(k, "adr.yaml", adr.Example(), tmpl.Options{}); err != nil {
		t.Errorf("adr: %s", told(err))
	}
}

// CompileAgain, which floe part check runs, compiles every template again and tells floe's first failure; with no
// part of one's own, it compiles none.
func TestOwnPartCompileAgain(t *testing.T) {
	t.Parallel()

	k := ownKit(t)
	if err := k.CompileAgain(); err != nil || kit.Compiled(k) != len(withSchema(t)) {
		t.Errorf("CompileAgain: %v, compiled %d of %d", err, kit.Compiled(k), len(withSchema(t)))
	}
	broken := ownKit(t)
	kit.FailRebuilt(broken, "report")
	if r := usererr.Tell(i18n.EnglishCatalog(), broken.CompileAgain()); !strings.HasPrefix(r.Failure, "kit: floe's template report ") {
		t.Errorf("CompileAgain with report broken told %+v, want floe's failure", r)
	}
	none, err := kit.Load(floe(t), kit.Sources{})
	if err != nil {
		t.Fatal(told(err))
	}
	if err := none.CompileAgain(); err != nil || kit.Compiled(none) != 0 {
		t.Errorf("CompileAgain with no part of one's own: %v, compiled %d", err, kit.Compiled(none))
	}
}
