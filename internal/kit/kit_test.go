package kit_test

import (
	"bytes"
	"fmt"
	"html/template"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
)

func told(err error) string {
	if err == nil {
		return ""
	}
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

func lines(s ...string) string { return strings.Join(s, "\n") }

func floe(t *testing.T) *kit.Kit {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(told(err))
	}
	return k
}

// floe's own read without a mistake, once a process: 30 parts, 5 looks, 11 themes and 7 templates.
func TestFloe(t *testing.T) {
	t.Parallel()

	k := floe(t)
	if again := floe(t); again != k {
		t.Error("Floe read floe's own twice")
	}
	parts, looks, themes, templates := k.Names()
	if len(parts) != 30 || !slices.Equal(looks, []string{"band", "cards", "numbered", "panel", "rows"}) || len(themes) != 11 ||
		!slices.Equal(templates, []string{"adr", "design", "handoff", "postmortem", "report", "runbook", "slides"}) {
		t.Errorf("Names = %v, %v, %v, %v", parts, looks, themes, templates)
	}
	for _, name := range parts {
		p, _ := k.Part(name)
		if page(t, p, "en") == nil || p.CSS() == nil {
			t.Errorf("the part %s has no page or no CSS", name)
		}
	}
	for _, name := range []string{"progress", "table", "terminal", "tree"} {
		if p, _ := k.Part(name); p.Checks() == nil {
			t.Errorf("the part %s has no checks", name)
		}
	}
	if p, _ := k.Part("table"); !slices.Equal(p.Looks(), []string{"panel"}) {
		t.Errorf("table takes %v", p.Looks())
	}
	if th, _ := k.Theme("default"); th.Palette() == nil || th.CSS("table") == nil {
		t.Error("the default theme has no palette or no table.css")
	}
	if len(k.BaseCSS()) == 0 {
		t.Error("no base.css")
	}
}

// The files floe part show and template show print are copies: Floe's kit, shared by the process, stays.
func TestEntriesCopied(t *testing.T) {
	t.Parallel()

	k := floe(t)
	for _, entries := range []func() []kit.Entry{k.Parts, k.Templates, k.Themes} {
		first := entries()
		i := slices.IndexFunc(first, func(e kit.Entry) bool { return len(e.Files) > 0 })
		was := first[i].Files[0].Text[0]
		first[i].Files[0].Text[0] = 'X'
		if again := entries()[i].Files[0].Text[0]; again != was {
			t.Errorf("%s: writing the text returned changed the kit: %q", first[i].Name, again)
		}
	}
}

// Each part, look, shared schema, template and theme shows the files it read, in the order show prints them, an
// empty file too.
func TestEntryFiles(t *testing.T) {
	t.Parallel()

	k := floe(t)
	names := func(entries []kit.Entry, name string) string {
		i := slices.IndexFunc(entries, func(e kit.Entry) bool { return e.Name == name })
		files := make([]string, 0, len(entries[i].Files))
		for _, f := range entries[i].Files {
			files = append(files, f.Name)
		}
		return strings.Join(files, ", ")
	}
	for _, tt := range []struct {
		entries    func() []kit.Entry
		name, want string
	}{
		{k.Parts, "code", "schema.json, part.html, part.css"},
		{k.Parts, "bars", "schema.json, part.yaml, part.html, part.css"},
		{k.Parts, "panel", "part.yaml, part.css"},
		{k.Parts, "tone", "tone.json"},
		{k.Templates, "design", "schema.json, computed.yaml, example.yaml, page.html, page.css, words.yaml"},
		{k.Themes, "ink", "palette.css, base.css, cards.css, cell.css, code.css, note.css, panel.css, table.css"},
	} {
		if got := names(tt.entries(), tt.name); got != tt.want {
			t.Errorf("%s shows %s, want %s", tt.name, got, tt.want)
		}
	}

	own, err := kit.Load(k, kit.Sources{Parts: kit.Dir{FS: fstest.MapFS{
		"mine/schema.json": {Data: []byte(partSchema)},
		"mine/checks.yaml": {Data: nil},
		"mine/part.html":   {Data: []byte(`<div data-floe-part="mine">{{.t}}</div>`)},
		"mine/part.css":    {Data: []byte{}},
	}, Name: "own/parts"}})
	if err != nil {
		t.Fatal(told(err))
	}
	if got := names(own.Parts(), "mine"); got != "schema.json, checks.yaml, part.html, part.css" {
		t.Errorf("mine shows %s, want its empty checks.yaml too", got)
	}
}

// The example of report reads, validates and passes its checks: floe's own checks run on floe's own data.
func TestFloeReport(t *testing.T) {
	t.Parallel()

	r, ok := floe(t).Template("report")
	// report has no checks.yaml: a fence in a text part is warned of as it is drawn.
	if !ok || r.Schema() == nil || r.Checks() != nil || page(t, r, "en") == nil || r.Example() == nil {
		t.Fatalf("report = %+v", r)
	}
	d, err := doc.Read("example.yaml", r.Example(), "")
	if err != nil {
		t.Fatal(told(err))
	}
	if err := r.Schema().Validate(d); err != nil {
		t.Error(told(err))
	}
}

// A part's words follow the page's language, as i18n.Tag reads it, down its subtags to English.
func TestPartWords(t *testing.T) {
	t.Parallel()

	p, _ := floe(t).Part("fields")
	for lang, want := range map[string]string{"ja-jp": ">必須<", "en": ">required<", "fr": ">required<", "ja": ">必須<"} {
		var b strings.Builder
		// Every key of the schema, as Walk hands an item, left out ones nil.
		data := []any{map[string]any{"name": "x", "required": true, "type": nil, "default": nil, "text": nil}}
		if err := page(t, p, lang).Execute(&b, data); err != nil || !strings.Contains(b.String(), want) {
			t.Errorf("%s: %v, %q holds no %s", lang, err, b.String(), want)
		}
	}
}

const partSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"t": {"$ref": "floe:///parts/table.json"}}}`

// One's own parts and templates read over floe's take floe's looks and parts, and leave floe's Kit as it is,
// read at once.
func TestReadOver(t *testing.T) {
	t.Parallel()

	k := floe(t)
	before := names(k)
	own := kit.Sources{
		Parts: kit.Dir{FS: fstest.MapFS{
			"mine/schema.json": {Data: []byte(partSchema)},
			"mine/part.html":   {Data: []byte(`<div data-floe-part="mine">{{.t}}</div>`)},
			"mine/part.css":    {Data: []byte("\xef\xbb\xbfdiv { margin: 0 }\n")},
			"mine/part.yaml":   {Data: []byte("looks: [panel]\n")},
		}, Name: "own/parts"},
		Themes: kit.Dir{FS: fstest.MapFS{
			"dark/palette.css": {Data: []byte(":root { --ground: black; }\n")},
			"dark/mine.css":    {Data: []byte("div { color: white }\n")},
			"dark/stray.css":   {Data: []byte("div { color: red }\n")},
		}, Name: "own/themes"},
		Templates: kit.Dir{FS: fstest.MapFS{
			"memo/page.html":   {Data: []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body></body></html>`)},
			"memo/schema.json": {Data: []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"m": {"$ref": "floe:///parts/mine.json"}}}`)},
		}, Name: "own/templates"},
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			mine, err := kit.Read(k, own)
			if err != nil {
				t.Error(told(err))
				return
			}
			p, ok := mine.Part("mine")
			th, _ := mine.Theme("dark")
			if _, floes := mine.Part("table"); !ok || !floes || !slices.Equal(p.Looks(), []string{"panel"}) ||
				string(p.CSS()) != "div { margin: 0 }\n" || th.CSS("mine") == nil || !slices.Equal(th.Unread(), []string{"stray.css"}) {
				t.Errorf("read over floe's: %+v %+v", p, th)
			}
		})
	}
	wg.Wait()
	if after := names(k); after != before {
		t.Errorf("Read changed floe's Kit: %s, then %s", before, after)
	}
}

func names(k *kit.Kit) string {
	parts, looks, themes, templates := k.Names()
	return strings.Join(slices.Concat(parts, looks, themes, templates), " ")
}

// Reading tells every mistake of the files it reads together, Sorted, and gives no Kit.
func TestReadMistakes(t *testing.T) {
	t.Parallel()

	const page = `<div data-floe-part="p">{{.t}}</div>`
	part := func(files map[string]string) fstest.MapFS {
		fsys := fstest.MapFS{"p/schema.json": {Data: []byte(`{"type": "object"}`)}, "p/part.html": {Data: []byte(page)}, "p/part.css": {Data: []byte("")}}
		for name, src := range files {
			fsys["p/"+name] = &fstest.MapFile{Data: []byte(src)}
		}
		return fsys
	}
	for _, tt := range []struct {
		name  string
		parts fstest.MapFS
		want  string
	}{
		{
			"looks not a list", part(map[string]string{"part.yaml": "looks: panel\n"}),
			"parts/p/part.yaml:1:8: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
		},
		{"part.yaml, together", part(map[string]string{"part.yaml": "looks: [panel, [x]]\nbogus: 1\ndescription: x\n"}), lines(
			"parts/p/part.yaml:1:16: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
			"parts/p/part.yaml:2:1: bogus is not a key of part.yaml; write looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:",
			"parts/p/part.yaml:3:1: a part's description is its schema.json's description; part.yaml's description: is a look alone's, which has no schema",
		)},
		{"looks not text", part(map[string]string{"part.yaml": "looks: [404, 1.10, true, false, ~, '']\n"}), lines(
			"parts/p/part.yaml:1:9: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
			"parts/p/part.yaml:1:14: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
			"parts/p/part.yaml:1:20: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
			"parts/p/part.yaml:1:26: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
			"parts/p/part.yaml:1:33: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
			"parts/p/part.yaml:1:36: write looks: as a list of the names of parts or looks, each as text (in quotes where YAML would read a number, true, false or null), like looks: [panel]",
		)},
		{
			"part.yaml not keys and values", part(map[string]string{"part.yaml": "- a\n"}),
			"parts/p/part.yaml:1:1: write part.yaml as keys and values: looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:",
		},
		{
			"a look's description",
			fstest.MapFS{"lk/part.css": {Data: []byte("")}, "lk/part.yaml": {Data: []byte("description: [1]\n")}},
			"parts/lk/part.yaml:1:14: write description: as text, what the look is",
		},
		{
			"a Go template", part(map[string]string{"part.html": "<div>{{.t}\n"}),
			"parts/p/part.html:1: bad character U+007D '}'",
		},
		{"words.yaml, together", part(map[string]string{"words.yaml": "en:\n  a: A\n  n: 3\nEN:\n  a: B\nja: x\nnot-a-tag_: {}\n"}), lines(
			"parts/p/words.yaml:3:6: en.n: write text, in quotes where YAML would read a number, true, false or null",
			"parts/p/words.yaml:4:1: EN and en are one language, as a tag's case, or _ for -, does not tell languages apart: keep one",
			"parts/p/words.yaml:6:5: ja: write the names of the words and their text",
		)},
		{
			"words.yaml not keys and values", part(map[string]string{"words.yaml": "- a\n"}),
			"parts/p/words.yaml:1:1: write words.yaml as language tags, each the names of the words and their text",
		},
		{
			"checks.yaml", part(map[string]string{"checks.yaml": "- expr: doc.a <\n  message: m\n"}),
			"parts/p/checks.yaml:1:15: unexpected token EOF",
		},
		{"words.yaml and part.yaml at once", part(map[string]string{"words.yaml": "- a\n", "part.yaml": "- a\n"}), lines(
			"parts/p/part.yaml:1:1: write part.yaml as keys and values: looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:",
			"parts/p/words.yaml:1:1: write words.yaml as language tags, each the names of the words and their text",
		)},
	} {
		k, err := kit.Read(nil, kit.Sources{Parts: kit.Dir{FS: tt.parts, Name: "parts"}})
		if got := told(err); got != tt.want || k != nil {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// A word a page names that its language does not have stops the drawing, telling the language it looked in
// and those that have it.
func TestWordUnknown(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ words, lang, want string }{
		{"", "en", `templates/t/page.html: words.yaml has no word "zz" in en`},
		{"# none yet\n", "en", `templates/t/page.html: words.yaml has no word "zz" in en`},
		{"en: {a: A}\n", "en", `templates/t/page.html: words.yaml has no word "zz" in en`},
		{"en: {a: A}\nja: {zz: ず}\n", "en", `templates/t/page.html: words.yaml has no word "zz" in en`},
		{"ja: {zz: ず}\n", "en", `templates/t/page.html: words.yaml has no word "zz" in en`},
		{"en: {a: A}\nja: {a: あ}\n", "ja-JP", `templates/t/page.html: words.yaml has no word "zz" in ja`},
	} {
		fsys := fstest.MapFS{"t/page.html": {Data: []byte(`{{word "zz"}}`)}}
		if tt.words != "" {
			fsys["t/words.yaml"] = &fstest.MapFile{Data: []byte(tt.words)}
		}
		k, err := kit.Read(nil, kit.Sources{Templates: kit.Dir{FS: fsys, Name: "templates"}})
		if err != nil {
			t.Fatal(told(err))
		}
		tm, _ := k.Template("t")
		err = page(t, tm, tt.lang).Execute(&strings.Builder{}, nil)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: %v, want %s", tt.words, err, tt.want)
		}
	}
}

// A word is its text as YAML reads it: a | block keeps its line break at the end, a >- block leaves it out.
func TestWordsAsWritten(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"t/page.html":  {Data: []byte(`[{{word "a"}}][{{word "b"}}]`)},
		"t/words.yaml": {Data: []byte("en:\n  a: |\n    x\n  b: >-\n    y\n    z\n")},
	}
	k, err := kit.Read(nil, kit.Sources{Templates: kit.Dir{FS: fsys, Name: "templates"}})
	if err != nil {
		t.Fatal(told(err))
	}
	tm, _ := k.Template("t")
	var b strings.Builder
	if err := page(t, tm, "en").Execute(&b, nil); err != nil || b.String() != "[x\n][y z]" {
		t.Errorf("drew %q, %v; want %q", b.String(), err, "[x\n][y z]")
	}
}

// Parts, looks and shared schemas share their names: one's own of a name floe's take is refused, of any kind,
// as floe's templates and parts would go on with floe's; so are two of one's own of a name, a name floe does
// not take, and a name of a theme's file. A theme of floe's name is refused too; a template of one hides floe's.
func TestNames(t *testing.T) {
	t.Parallel()

	k := floe(t)
	for _, tt := range []struct {
		parts fstest.MapFS
		want  string
	}{
		{
			fstest.MapFS{"p/part.css": {Data: []byte("")}, "p.json": {Data: []byte("{}")}},
			"own/parts/p.json: p names both own/parts/p and own/parts/p.json: give one of them another name",
		},
		{
			fstest.MapFS{"table.json": {Data: []byte("{}")}},
			"own/parts/table.json: table is the name of floe's parts/table: give yours another name",
		},
		{
			fstest.MapFS{"any/part.css": {Data: []byte("")}},
			"own/parts/any: any is the name of floe's parts/any.json: give yours another name",
		},
		{
			fstest.MapFS{"table/schema.json": {Data: []byte(`{"type": "object"}`)}, "table/part.html": {Data: []byte("x")}},
			"own/parts/table: table is the name of floe's parts/table: give yours another name",
		},
		{
			fstest.MapFS{"diagram/schema.json": {Data: []byte(`{"type": "object"}`)}, "diagram/part.html": {Data: []byte("x")}},
			"own/parts/diagram: diagram is the name of floe's parts/diagram: give yours another name",
		},
		{
			fstest.MapFS{"panel/part.css": {Data: []byte("")}},
			"own/parts/panel: panel is the name of floe's parts/panel: give yours another name",
		},
		{
			fstest.MapFS{"base/part.css": {Data: []byte("")}, "palette/part.css": {Data: []byte("")}},
			lines(
				"own/parts/base: base names a file of every theme (base.css), which a theme's CSS for this part would be read as: give it another name",
				"own/parts/palette: palette names a file of every theme (palette.css), which a theme's CSS for this part would be read as: give it another name",
			),
		},
		// A shared schema has no CSS a theme gives.
		{fstest.MapFS{"base.json": {Data: []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema"}`)}}, ""},
		{
			fstest.MapFS{"Note/part.css": {Data: []byte("")}, "-x/part.css": {Data: []byte("")}, "a b.json": {Data: []byte("{}")}, "my-note_2/part.css": {Data: []byte("")}},
			lines(
				"own/parts/-x: floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it",
				"own/parts/Note: floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it",
				"own/parts/a b.json: floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it",
			),
		},
	} {
		_, err := kit.Read(k, kit.Sources{Parts: kit.Dir{FS: tt.parts, Name: "own/parts"}})
		if got := told(err); got != tt.want {
			t.Errorf("told\n%s\nwant\n%s", got, tt.want)
		}
	}
	_, err := kit.Read(k, kit.Sources{
		Themes:    kit.Dir{FS: fstest.MapFS{"Dark/base.css": {Data: []byte("")}, "ink/base.css": {Data: []byte("")}}, Name: "own/themes"},
		Templates: kit.Dir{FS: fstest.MapFS{"Report/page.html": {Data: []byte("")}, "report/page.html": {Data: []byte("x")}}, Name: "own/templates"},
	})
	want := lines(
		"own/themes/Dark: floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it",
		"own/themes/ink: ink is the name of floe's themes/ink: give yours another name",
		"own/templates/Report: floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it",
	)
	if got := told(err); got != want {
		t.Errorf("themes and templates told\n%s\nwant\n%s", got, want)
	}
}

// NameMistake refuses the name of a part to make as Load refuses the part's directory, over the kit of one's
// own: beside a shared schema of one's own of its name too, which Load tells at the schema.
func TestNameMistake(t *testing.T) {
	t.Parallel()

	shr := fstest.MapFS{"shr.json": {Data: []byte("{}")}}
	k, err := kit.Load(floe(t), kit.Sources{Parts: kit.Dir{FS: shr, Name: "own/parts"}})
	if err != nil {
		t.Fatal(told(err))
	}
	for _, tt := range []struct{ name, want string }{
		{"mine", ""},
		{"a-b_c", ""},
		{"Bad", "own/parts/Bad: floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it"},
		{"base", "own/parts/base: base names a file of every theme (base.css), which a theme's CSS for this part would be read as: give it another name"},
		{"palette", "own/parts/palette: palette names a file of every theme (palette.css), which a theme's CSS for this part would be read as: give it another name"},
		{"note", "own/parts/note: note is the name of floe's parts/note: give yours another name"},
		{"any", "own/parts/any: any is the name of floe's parts/any.json: give yours another name"},
		{"shr", "own/parts/shr: shr names both own/parts/shr and own/parts/shr.json: give one of them another name"},
	} {
		if got := told(k.NameMistake("own/parts", kit.KindPart, tt.name)); got != tt.want {
			t.Errorf("NameMistake(%s) told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
		if tt.name == "shr" {
			continue
		}
		fsys := maps.Clone(shr)
		fsys[tt.name+"/part.css"] = &fstest.MapFile{Data: []byte("")}
		if _, err := kit.Load(floe(t), kit.Sources{Parts: kit.Dir{FS: fsys, Name: "own/parts"}}); told(err) != tt.want {
			t.Errorf("Load of %s told\n%s\nwant\n%s", tt.name, told(err), tt.want)
		}
	}
	if got := told(k.NameMistake("own/templates", kit.KindTemplate, "report")); got != "" {
		t.Errorf("NameMistake of a template of floe's name told %s; it hides floe's", got)
	}
}

// A key written twice in a part's schema.json or a schema the parts share is told at that file, as read.
func TestKeysTwiceOwn(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"p/schema.json": {Data: []byte(`{"type": "object", "type": "object"}`)},
		"p/part.html":   {Data: []byte(`<div data-floe-part="p"></div>`)},
		"p/part.css":    {Data: []byte("")},
		"shr.json":      {Data: []byte("{\n  \"enum\": [1],\n  \"enum\": [2]\n}")},
	}
	_, err := kit.Load(floe(t), kit.Sources{Parts: kit.Dir{FS: fsys, Name: "own/parts"}})
	tail := " is in this object already: only the last would be read; keep one"
	want := lines(
		`own/parts/p/schema.json:1:20: the key "type"`+tail,
		`own/parts/shr.json:3:3: the key "enum"`+tail,
	)
	if got := told(err); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// Read keeps the names of the files it does not read, for the check of what a directory holds.
func TestUnread(t *testing.T) {
	t.Parallel()

	k, err := kit.Read(floe(t), kit.Sources{
		Parts: kit.Dir{FS: fstest.MapFS{
			"lk/part.css": {Data: []byte("")}, "lk/part.html": {Data: []byte("x")}, "lk/checks.yaml": {Data: []byte("")},
			"p/schema.json": {Data: []byte(`{"type": "object"}`)}, "p/part.html": {Data: []byte("x")}, "p/notes.txt": {Data: []byte("")},
		}, Name: "own/parts"},
		Themes: kit.Dir{FS: fstest.MapFS{"t/tabel.css": {Data: []byte("")}, "t/base.css": {Data: []byte("")}}, Name: "own/themes"},
		Templates: kit.Dir{FS: fstest.MapFS{
			"t/pgae.html": {Data: []byte("")}, "t/computed.yml": {Data: []byte("")}, "t/computed.yaml": {Data: []byte("")},
		}, Name: "own/templates"},
	})
	if err != nil {
		t.Fatal(told(err))
	}
	l, _ := k.Look("lk")
	p, _ := k.Part("p")
	th, _ := k.Theme("t")
	tm, _ := k.Template("t")
	got := fmt.Sprint(l.Unread(), p.Unread(), th.Unread(), tm.Unread())
	if got != "[checks.yaml part.html] [notes.txt] [tabel.css] [computed.yml pgae.html]" {
		t.Errorf("Unread = %s", got)
	}
}

// A shared schema's $id, which changes how the templates compiled with it read, is told as it is read, and no
// template is compiled past it: compiling would tell a JSON pointer not found, into the schema it names.
func TestReadSharedID(t *testing.T) {
	t.Parallel()

	_, err := kit.Read(floe(t), kit.Sources{
		Parts: kit.Dir{FS: fstest.MapFS{"myshared.json": {Data: []byte(
			`{"properties": {"y": {"$ref": "note.json#/properties/title"}}, "$defs": {"n": {"$id": "note.json", "type": "string"}}}`,
		)}}, Name: "own/parts"},
		Templates: kit.Dir{FS: fstest.MapFS{
			"t/page.html":   {Data: []byte("x")},
			"t/schema.json": {Data: []byte(`{"properties": {"x": {"$ref": "floe:///parts/myshared.json"}}}`)},
		}, Name: "own/templates"},
	})
	want := "own/parts/myshared.json: #/$defs/n: this $id, note.json, has no scheme, which a part or a schema the parts share may not have, as it may land under floe:///parts/ and take the place of another: write an absolute URL of your own, like https://example.com/item.json, or leave $id out"
	if got := told(err); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// page returns the copy of the page of p, a part or a template, to draw in lang.
func page(t *testing.T, p interface {
	Page(lang string) (*template.Template, error)
}, lang string,
) *template.Template {
	t.Helper()
	c, err := p.Page(lang)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// A $ref of one's own to no part is told at the file that holds it, a part's schema.json or a shared schema,
// not at a template compiled with it (S5); and when a part could not be read, none is compiled, as a $ref to it
// would be told as to no part.
func TestCompileParts(t *testing.T) {
	t.Parallel()

	const gone = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": "floe:///parts/gone.json"}`
	template := fstest.MapFS{
		"memo/page.html": {Data: []byte("x")},
		"memo/schema.json": {Data: []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", ` +
			`"properties": {"m": {"$ref": "floe:///parts/mine.json"}, "s": {"$ref": "floe:///parts/sh.json"}}}`)},
	}
	mine := func(schema string) fstest.MapFS {
		return fstest.MapFS{
			"mine/schema.json": {Data: []byte(schema)}, "mine/part.html": {Data: []byte("x")}, "mine/part.css": {Data: []byte("")},
			"sh.json": {Data: []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "string"}`)},
		}
	}
	for _, tt := range []struct {
		name  string
		parts fstest.MapFS
		want  string
	}{
		{"a part", mine(gone), "own/parts/mine/schema.json: floe:///parts/gone.json is not a part; refer to one as floe:///parts/<name>.json"},
		{"a shared schema", func() fstest.MapFS {
			p := mine(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "string"}`)
			p["sh.json"] = &fstest.MapFile{Data: []byte(gone)}
			return p
		}(), "own/parts/sh.json: floe:///parts/gone.json is not a part; refer to one as floe:///parts/<name>.json"},
		{"a part that could not be read", func() fstest.MapFS {
			p := mine(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": "floe:///parts/bad.json"}`)
			p["bad/schema.json"] = &fstest.MapFile{Data: []byte("{")}
			p["bad/part.html"] = &fstest.MapFile{Data: []byte("x")}
			return p
		}(), "own/parts/bad/schema.json:1:2: unexpected EOF"},
	} {
		_, err := kit.Read(floe(t), kit.Sources{
			Parts: kit.Dir{FS: tt.parts, Name: "own/parts"}, Templates: kit.Dir{FS: template, Name: "own/templates"},
		})
		if got := told(err); got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// A shared schema of one's own is checked from its root, whether a schema refers to it or not, and its mistakes
// are told in its file: one it refers to is checked from its own root, not in the file referring to it. Read keeps
// it compiled, as a part list shows its description.
func TestCheckShared(t *testing.T) {
	t.Parallel()

	const head = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "description": "Mine", `
	inAnyOf := head + `"properties": {"m": {"anyOf": [{"$ref": "floe:///parts/note.json"}]}}}`
	// Read, which does not check, keeps the schema compiled: Load checks what it keeps.
	k, err := kit.Read(floe(t), kit.Sources{Parts: kit.Dir{FS: fstest.MapFS{"sh.json": {Data: []byte(inAnyOf)}}, Name: "own/parts"}})
	if err != nil {
		t.Fatal(told(err))
	}
	about := "(none)"
	if i := slices.IndexFunc(k.Parts(), func(e kit.Entry) bool { return e.Name == "sh" }); i >= 0 {
		about = k.Parts()[i].About
	}
	if about != "Mine" {
		t.Fatalf("the about of sh = %q, want Mine", about)
	}
	for _, tt := range []struct {
		name  string
		parts fstest.MapFS
		want  string
	}{
		{"referred to by none", fstest.MapFS{"sh.json": {Data: []byte(inAnyOf)}}, "own/parts/sh.json: #/properties/m/anyOf/0: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"referred to by a shared schema", fstest.MapFS{
			"sh.json":  {Data: []byte(head + `"properties": {"t": {"$ref": "floe:///parts/sh2.json"}}}`)},
			"sh2.json": {Data: []byte(inAnyOf)},
		}, "own/parts/sh2.json: #/properties/m/anyOf/0: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"right", fstest.MapFS{"sh.json": {Data: []byte(head + `"properties": {"m": {"$ref": "floe:///parts/note.json"}}}`)}}, ""},
	} {
		if _, err := kit.Load(floe(t), kit.Sources{Parts: kit.Dir{FS: tt.parts, Name: "own/parts"}}); told(err) != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, told(err), tt.want)
		}
	}
}

// A byte order mark past a file's start is refused in every file of one's own that Read reads as YAML or JSON:
// Read leaves the one at the start to the reader of its kind, which leaves it out once. A part's HTML is kept
// without it, as its places are told in it, and the files a template shows, but the example it draws, which the
// reading of a data file leaves it out of.
func TestReadMarks(t *testing.T) {
	t.Parallel()

	const mark = "\xef\xbb\xbf"
	bomAt := func(file string) string { return file + ":1:1: " + i18n.EnglishCatalog().Msg("text_bom") }
	part := func(name, src string) kit.Sources {
		fsys := fstest.MapFS{
			"mine/schema.json": {Data: []byte(partSchema)},
			"mine/part.html":   {Data: []byte(`<div data-floe-part="mine">{{.t}}</div>`)},
		}
		fsys[name] = &fstest.MapFile{Data: []byte(src)}
		return kit.Sources{Parts: kit.Dir{FS: fsys, Name: "own/parts"}}
	}
	const tmplSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`
	template := func(name, src string) kit.Sources {
		fsys := fstest.MapFS{
			"memo/page.html":   {Data: []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body></body></html>`)},
			"memo/schema.json": {Data: []byte(tmplSchema)},
		}
		fsys[name] = &fstest.MapFile{Data: []byte(src)}
		return kit.Sources{Templates: kit.Dir{FS: fsys, Name: "own/templates"}}
	}
	for _, tt := range []struct {
		name string
		own  kit.Sources
		want string
	}{
		{"a part's schema.json", part("mine/schema.json", mark+mark+partSchema), bomAt("own/parts/mine/schema.json")},
		{"a part's part.yaml", part("mine/part.yaml", mark+mark+"looks: [panel]\n"), bomAt("own/parts/mine/part.yaml")},
		{"a part's checks.yaml", part("mine/checks.yaml", mark+mark+"[]\n"), bomAt("own/parts/mine/checks.yaml")},
		{"a part's words.yaml", part("mine/words.yaml", mark+mark+"en: {}\n"), bomAt("own/parts/mine/words.yaml")},
		{"a shared schema", part("one.json", mark+mark+`{"type": "string"}`), bomAt("own/parts/one.json")},
		{"a template's schema.json", template("memo/schema.json", mark+mark+tmplSchema), bomAt("own/templates/memo/schema.json")},
		{"a template's computed.yaml", template("memo/computed.yaml", mark+mark+"{}\n"), bomAt("own/templates/memo/computed.yaml")},
	} {
		if _, err := kit.Read(floe(t), tt.own); told(err) != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, told(err), tt.want)
		}
	}
	own := part("mine/part.html", mark+`<div data-floe-part="mine">{{.t}}</div>`)
	own.Templates = template("memo/example.yaml", mark+"lang: en\n").Templates
	own.Themes = kit.Dir{FS: fstest.MapFS{"mine/palette.css": {Data: []byte(mark + ":root {}\n")}}, Name: "own/themes"}
	k, err := kit.Read(floe(t), own)
	if err != nil {
		t.Fatal(told(err))
	}
	p, _ := k.Part("mine")
	m, _ := k.Template("memo")
	if bytes.HasPrefix(p.PageSource(), []byte(mark)) || !bytes.HasPrefix(m.Example(), []byte(mark)) {
		t.Errorf("part.html %q, example.yaml %q: want the one without its mark, the other as written", p.PageSource(), m.Example())
	}
	for _, e := range slices.Concat(k.Parts(), k.Templates(), k.Themes()) {
		for _, f := range e.Files {
			if bytes.Contains(f.Text, []byte(mark)) {
				t.Errorf("%s shows %s with a mark: %q", e.Name, f.Name, f.Text)
			}
		}
	}
}
