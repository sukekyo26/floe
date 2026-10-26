package kit_test

import (
	"fmt"
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

// floe's own read without a mistake, once a process: 29 parts, 2 looks, 9 themes and report.
func TestFloe(t *testing.T) {
	t.Parallel()

	k := floe(t)
	if again := floe(t); again != k {
		t.Error("Floe read floe's own twice")
	}
	parts, looks, themes, templates := k.Names()
	if len(parts) != 29 || !slices.Equal(looks, []string{"panel", "rows"}) || len(themes) != 9 ||
		!slices.Equal(templates, []string{"report"}) {
		t.Errorf("Names = %v, %v, %v, %v", parts, looks, themes, templates)
	}
	for _, name := range parts {
		p, _ := k.Part(name)
		if p.Page("en") == nil || p.CSS() == nil {
			t.Errorf("the part %s has no page or no CSS", name)
		}
	}
	for _, name := range []string{"links", "matrix", "progress", "table", "terminal", "tree"} {
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

// The example of report reads, validates and passes its checks: floe's own checks run on floe's own data.
func TestFloeReport(t *testing.T) {
	t.Parallel()

	r, ok := floe(t).Template("report")
	if !ok || r.Schema() == nil || r.Checks() == nil || r.Page("en") == nil || r.Example() == nil {
		t.Fatalf("report = %+v", r)
	}
	d, err := doc.Read("example.yaml", r.Example())
	if err != nil {
		t.Fatal(told(err))
	}
	if err := usererr.Collect(r.Schema().Validate(d), r.Checks().Run(d, nil)); err != nil {
		t.Error(told(err))
	}
}

// A part's words follow the page's language down its subtags to English.
func TestPartWords(t *testing.T) {
	t.Parallel()

	p, _ := floe(t).Part("matrix")
	for lang, want := range map[string]string{"ja-JP": "できる", "en": "Yes", "fr": "Yes", "JA": "できる"} {
		var b strings.Builder
		data := map[string]any{"columns": []any{"a"}, "rows": []any{map[string]any{"what": "x", "cells": []any{"yes"}}}}
		if err := p.Page(lang).Execute(&b, data); err != nil || !strings.Contains(b.String(), want) {
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
			"parts/p/part.yaml:1:8: write looks: as a list of the names of parts or looks, like looks: [panel]",
		},
		{"part.yaml, together", part(map[string]string{"part.yaml": "looks: [panel, [x]]\nbogus: 1\ndescription: x\n"}), lines(
			"parts/p/part.yaml:1:16: write looks: as a list of the names of parts or looks, like looks: [panel]",
			"parts/p/part.yaml:2:1: bogus is not a key of part.yaml; write looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:",
			"parts/p/part.yaml:3:1: a part's description is its schema.json's description; part.yaml's description: is a look alone's, which has no schema",
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
			"parts/p/part.html: template: part.html:1: bad character U+007D '}'",
		},
		{"words.yaml, together", part(map[string]string{"words.yaml": "en:\n  a: A\n  n: 3\nEN:\n  a: B\nja: x\nnot-a-tag_: {}\n"}), lines(
			"parts/p/words.yaml:3:6: en.n: write text, in quotes where YAML would read a number, true, false or null",
			"parts/p/words.yaml:4:1: EN is the language en again: a tag's case does not tell languages apart",
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
		{"", "en", `templates/t/page.html: there is no words.yaml to draw "zz" from: put one beside it, and write the word under en:`},
		{"en: {a: A}\n", "en", `templates/t/page.html: the words of en in words.yaml have no "zz"`},
		{"en: {a: A}\nja: {zz: ず}\n", "en", `templates/t/page.html: the words of en in words.yaml have no "zz", which those of ja have: add it to en`},
		{"ja: {zz: ず}\n", "en", `templates/t/page.html: words.yaml has no words in the page's language nor in en to draw "zz" in, but in ja: add en, or the page's language`},
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
		err = tm.Page(tt.lang).Execute(&strings.Builder{}, nil)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: %v, want %s", tt.words, err, tt.want)
		}
	}
}

// Parts and looks share their names: a shared schema of a part's name is refused, of floe's too, and so is
// one's own of another kind than floe's of its name, which floe's parts and schemas name.
func TestNames(t *testing.T) {
	t.Parallel()

	k := floe(t)
	mine, err := kit.Read(k, kit.Sources{Parts: kit.Dir{FS: fstest.MapFS{
		"table/schema.json": {Data: []byte(`{"type": "object"}`)}, "table/part.html": {Data: []byte("x")},
		"panel/part.css": {Data: []byte("a {}")},
	}, Name: "own/parts"}})
	if err != nil {
		t.Fatal(told(err))
	}
	if mine.Resources()["floe:///parts/table.json"].File != "own/parts/table/schema.json" {
		t.Error("one's own part table does not take the place of floe's")
	}
	if parts, looks, _, _ := mine.Names(); !slices.Contains(parts, "table") || !slices.Contains(looks, "panel") {
		t.Errorf("Names = %v, %v", parts, looks)
	}
	twice := "own/parts/p.json: p names both own/parts/p and own/parts/p.json: give one of them another name"
	for _, tt := range []struct {
		parts fstest.MapFS
		want  string
	}{
		{fstest.MapFS{"p/part.css": {Data: []byte("")}, "p.json": {Data: []byte("{}")}}, twice},
		{fstest.MapFS{"table.json": {Data: []byte("{}")}}, "own/parts/table.json: table names both parts/table and own/parts/table.json: give one of them another name"},
		{fstest.MapFS{"any/part.css": {Data: []byte("")}}, "own/parts/any: any names both parts/any.json and own/parts/any: give one of them another name"},
		{fstest.MapFS{"table/part.css": {Data: []byte("")}}, "own/parts/table: table is the name of the part parts/table, which its parts and schemas name as a part: give this look another name"},
		{
			fstest.MapFS{"panel/schema.json": {Data: []byte(`{"type": "object"}`)}, "panel/part.html": {Data: []byte("x")}},
			"own/parts/panel: panel is the name of the look parts/panel, which its parts take with looks:: give this part another name",
		},
	} {
		_, err := kit.Read(k, kit.Sources{Parts: kit.Dir{FS: tt.parts, Name: "own/parts"}})
		if got := told(err); got != tt.want {
			t.Errorf("told\n%s\nwant\n%s", got, tt.want)
		}
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
		Themes:    kit.Dir{FS: fstest.MapFS{"t/tabel.css": {Data: []byte("")}, "t/base.css": {Data: []byte("")}}, Name: "own/themes"},
		Templates: kit.Dir{FS: fstest.MapFS{"t/pgae.html": {Data: []byte("")}, "t/computed.yml": {Data: []byte("")}}, Name: "own/templates"},
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
