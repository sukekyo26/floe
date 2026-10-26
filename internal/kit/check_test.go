package kit_test

import (
	"html/template"
	"io"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/kit"
)

// floe's own pass every check: the test that stands for checking them at every run (judgement 1).
func TestCheckFloe(t *testing.T) {
	t.Parallel()

	if err := kit.Check(floe(t)); err != nil {
		t.Error(told(err))
	}
}

const (
	partFiles  = "a part holds schema.json, part.html and part.css, and a look alone part.css without the other two (and part.yaml if it takes looks); add what it lacks, or remove what a look has no use for"
	lookUnread = "floe reads no file but part.css and part.yaml in a look, a directory without a schema.json: remove it, or add the schema.json to make it a part"
	braceTold  = "this } closes no block: floe puts this CSS in a layer, which it would close, leaving the rules after it out of the layer and over the parts' styles"
)

// mapFS makes the files of a directory, name to text.
func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, src := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(src)}
	}
	return fsys
}

// Check tells the mistakes of what a Kit read itself, over floe's, all together; floe's own are not told
// again.
func TestCheckMistakes(t *testing.T) {
	t.Parallel()

	part := map[string]string{
		"p/schema.json": `{"type": "object"}`, "p/part.html": `<div data-floe-part="p"></div>`, "p/part.css": "div { margin: 0 }\n",
	}
	with := func(more map[string]string, without ...string) map[string]string {
		files := map[string]string{}
		for k, v := range part {
			files[k] = v
		}
		for k, v := range more {
			files[k] = v
		}
		for _, k := range without {
			delete(files, k)
		}
		return files
	}
	for _, tt := range []struct {
		name                    string
		parts, themes, template map[string]string
		want                    string
	}{
		{"right", part, nil, nil, ""},
		{"no part.css", with(nil, "p/part.css"), nil, nil, "own/parts/p: " + partFiles},
		{"a look with a part.html", with(map[string]string{"lk/part.css": "a{}", "lk/part.html": "x", "lk/checks.yaml": ""}), nil, nil, lines(
			"own/parts/lk/checks.yaml: "+lookUnread, "own/parts/lk/part.html: "+lookUnread)},
		{
			"a file a part does not read", with(map[string]string{"p/notes.txt": ""}), nil, nil,
			"own/parts/p/notes.txt: floe reads no file of this name in a part: a part holds schema.json, part.html and part.css, and part.yaml, checks.yaml and words.yaml if it likes; remove it",
		},
		{
			"a file a theme does not read", part,
			map[string]string{"t/tabel.css": "a{}"},
			nil,
			"own/themes/t/tabel.css: floe reads no file of this name in a theme: a theme holds base.css, palette.css and a <part>.css for each part or look it styles; rename it after one, or remove it",
		},
		{"part.css", with(map[string]string{"p/part.css": "}\n@import \"x\";\n"}), nil, nil, lines(
			"own/parts/p/part.css:1:1: "+braceTold,
			"own/parts/p/part.css:2:1: floe puts this CSS in a layer, where a browser reads no @import: load the stylesheet with a <link> in page.html",
		)},
		{
			"a part's schema with floe's format", with(map[string]string{"p/schema.json": `{"type": "object", "properties": {"h": {"type": "string", "format": "html"}}}`}), nil, nil,
			`own/parts/p/schema.json: #/properties/h: a part's schema takes no "format": "html": its part.html draws its items, which the checks of a format do not reach; declare the item in the template's schema instead, or, for Markdown, declare "type": "string" alone and draw it with {{markdown .<name>}}`,
		},
		{
			"escaping", with(map[string]string{"p/part.html": `<div data-floe-part="p"><a href="{{.t}}>x</a></div>`}), nil, nil,
			"own/parts/p/part.html: ends in a non-text context: {stateURL delimDoubleQuote urlPartPreQuery jsCtxRegexp [] attrURL elementNone <nil>}",
		},
		{
			"an unknown look", with(map[string]string{"p/part.yaml": "looks: [panel, nope]\n"}), nil, nil,
			"own/parts/p/part.yaml:1:16: looks names \"nope\", which is no part or look: name one floe or your own parts hold",
		},
		{
			"looks named as text", with(map[string]string{"p/part.yaml": "looks:\n  - \"404\"\n  - |\n    panel\n"}), nil, nil,
			lines(
				"own/parts/p/part.yaml:2:5: looks names \"404\", which is no part or look: name one floe or your own parts hold",
				"own/parts/p/part.yaml:3:5: looks names \"panel\\n\", which is no part or look: name one floe or your own parts hold",
			),
		},
		{
			"a look taking itself", with(map[string]string{"lk/part.css": "a{}", "lk/part.yaml": "looks: [lk]\n"}), nil, nil,
			"own/parts/lk/part.yaml:1:9: the look lk takes itself (lk → lk), which floe would follow forever; fix its part.yaml",
		},
		{
			"a part's cycle", with(map[string]string{"p/part.yaml": "looks: [zl]\n", "zl/part.css": "a{}", "zl/part.yaml": "looks: [p]\n"}), nil, nil,
			"own/parts/p/part.yaml:1:9: the part p takes its own look (p → zl → p), which floe would follow forever; fix its part.yaml",
		},
		// The files in the order they come: palette.css, base.css, then the parts' files.
		{"a theme", part, map[string]string{"t/palette.css": ":root { --a: 1 }\n}\n", "t/base.css": "}", "t/p.css": "@import \"x\";"}, nil, lines(
			"own/themes/t/palette.css:2:1: "+braceTold,
			"own/themes/t/base.css:1:1: "+braceTold,
			"own/themes/t/p.css:1:1: floe puts this CSS in a layer, where a browser reads no @import: load the stylesheet with a <link> in page.html",
		)},
		{"a template without its files", part, nil, map[string]string{"t/page.css": "}", "t/pgae.html": ""}, lines(
			"own/templates/t: example.yaml is missing: a template has page.html, schema.json and example.yaml",
			"own/templates/t: page.html is missing: a template has page.html, schema.json and example.yaml",
			"own/templates/t: schema.json is missing: a template has page.html, schema.json and example.yaml",
			"own/templates/t/page.css:1:1: "+braceTold,
			"own/templates/t/pgae.html: floe reads no file of this name in a template: a template holds page.html, schema.json and example.yaml, and page.css, computed.yaml, checks.yaml, words.yaml and assets/ if it likes; rename it after one, or remove it",
		)},
	} {
		src := kit.Sources{Parts: kit.Dir{FS: mapFS(tt.parts), Name: "own/parts"}}
		if tt.themes != nil {
			src.Themes = kit.Dir{FS: mapFS(tt.themes), Name: "own/themes"}
		}
		if tt.template != nil {
			src.Templates = kit.Dir{FS: mapFS(tt.template), Name: "own/templates"}
		}
		k, err := kit.Read(floe(t), src)
		if err != nil {
			t.Fatalf("%s: %s", tt.name, told(err))
		}
		if got := told(kit.Check(k)); got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// Drawing with floe's own, from the copies of their pages, leaves them as read: they check after, the escaping
// of each page too.
func TestCheckAfterDrawing(t *testing.T) {
	t.Parallel()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	parts, _, _, templates := k.Names()
	pages := make([]interface {
		Page(lang string) (*template.Template, error)
	}, 0, len(parts)+len(templates))
	for _, name := range parts {
		p, _ := k.Part(name)
		pages = append(pages, p)
	}
	for _, name := range templates {
		tm, _ := k.Template(name)
		pages = append(pages, tm)
	}
	for _, p := range pages {
		page, err := p.Page("en")
		if err != nil {
			t.Fatal(err)
		}
		// Its drawing of nothing may fail: what is asked is that it ran, which escapes it.
		_ = page.Execute(io.Discard, nil) //nolint:errcheck // see above
	}
	if err := kit.Check(k); err != nil {
		t.Errorf("Check after drawing: %v", err)
	}
}
