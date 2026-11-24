package kit_test

import (
	"io/fs"
	"maps"
	"testing"

	"floe/internal/kit"
)

// listing is an FS that counts in listed how often each directory, under the name it is told by, is listed.
type listing struct {
	fs.FS
	under  string
	listed map[string]int
}

func (l listing) ReadDir(name string) ([]fs.DirEntry, error) {
	l.listed[l.under+"/"+name]++
	return fs.ReadDir(l.FS, name) //nolint:wrapcheck // the FS's
}

// A read of one's own lists each directory once: on Windows every listing looks at the path from the drive's root.
func TestEachDirectoryListedOnce(t *testing.T) {
	t.Parallel()

	got := map[string]int{}
	dir := func(under string, files map[string]string) kit.Dir {
		return kit.Dir{FS: listing{FS: mapFS(files), under: under, listed: got}, Name: under}
	}
	const svg = `<svg xmlns="http://www.w3.org/2000/svg"/>`
	_, err := kit.Load(floe(t), kit.Sources{
		Parts: dir("parts", map[string]string{
			"mine/schema.json": `{"type": "object"}`, "mine/part.html": `<div data-floe-part="mine"></div>`,
			"mine/part.css": "div { margin: 0 }\n",
			"look/part.css": "div { margin: 0 }\n", "shared.json": `{"type": "string"}`,
		}),
		Themes: dir("themes", map[string]string{"night/palette.css": ":root { --ink: #eee }\n"}),
		Templates: dir("templates", map[string]string{
			"memo2/schema.json":    `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
			"memo2/page.html":      "<!doctype html><html><head><title>t</title></head><body></body></html>",
			"memo2/example.yaml":   "lang: en\ntemplate: memo2\n",
			"memo2/assets/a.svg":   svg,
			"memo2/assets/b/b.svg": svg,
		}),
	})
	// A mistake would stop the reading before its last listings.
	if err != nil {
		t.Fatal(told(err))
	}
	want := map[string]int{
		"parts/.": 1, "parts/mine": 1, "parts/look": 1, "themes/.": 1, "themes/night": 1,
		"templates/.": 1, "templates/memo2": 1, "templates/memo2/assets": 1, "templates/memo2/assets/b": 1,
	}
	if !maps.Equal(got, want) {
		t.Errorf("directories listed = %v; want %v", got, want)
	}
}
