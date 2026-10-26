package kit_test

import (
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
	partFiles = "a part holds schema.json, part.html and part.css, and a look alone part.css without the other two (and part.yaml if it takes looks); add what it lacks, or remove what a look has no use for"
	braceTold = "this } closes no block: floe puts this CSS in a layer, which it would close, leaving the rules after it out of the layer and over the parts' styles"
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
		{"a look with a part.html", with(map[string]string{"lk/part.css": "a{}", "lk/part.html": "x"}), nil, nil, "own/parts/lk: " + partFiles},
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
			"own/parts/p/part.yaml:1:16: looks names nope, which is no part or look: name one floe or your own parts hold",
		},
		{
			"a look taking itself", with(map[string]string{"lk/part.css": "a{}", "lk/part.yaml": "looks: [lk]\n"}), nil, nil,
			"own/parts/lk/part.yaml:1:9: the part lk takes its own look (lk → lk), which floe would follow forever; fix its part.yaml",
		},
		{
			"a cycle through floe's", with(map[string]string{"panel/part.css": "a{}", "panel/part.yaml": "looks: [rows]\n"}), nil, nil,
			"own/parts/panel/part.yaml:1:9: the part panel takes its own look (panel → rows → panel), which floe would follow forever; fix its part.yaml",
		},
		// The files in the order they come: palette.css, base.css, then the parts' files.
		{"a theme", part, map[string]string{"t/palette.css": ":root { --a: 1 }\nbody {}\n", "t/base.css": "}", "t/p.css": "@import \"x\";"}, nil, lines(
			"own/themes/t/palette.css:2:1: palette.css holds only the palette's variables, :root { --name: value; }, and its color-scheme, and no other rule, @media neither: write other rules in base.css or a part's file; for a dark page, make a theme whose :root sets dark colours and color-scheme: dark",
			"own/themes/t/base.css:1:1: "+braceTold,
			"own/themes/t/p.css:1:1: floe puts this CSS in a layer, where a browser reads no @import: load the stylesheet with a <link> in page.html",
		)},
		{"a template without its files", part, nil, map[string]string{"t/page.css": "}"}, lines(
			"own/templates/t: page.html is missing: a template has page.html and schema.json",
			"own/templates/t: schema.json is missing: a template has page.html and schema.json",
			"own/templates/t/page.css:1:1: "+braceTold,
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
