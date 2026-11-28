package store_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"

	floepublish "floe/internal/publish"
	"floe/internal/store"
)

// A page written before a page had one theme is moved to the theme it was drawn in, the picked one over --theme over
// its data's theme: over the default it took when first stored, and the one --theme or theme: named; a page of now,
// a link to HTML and a second run are left as they are.
func TestMoveThemes(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	home := filepath.Dir(root)
	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{"z.html": "<title>z</title>", "l.yaml": report("washi")} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		e, err := floepublish.PrepareLink(t.Context(), s, dw, path, store.Options{Slug: new(name[:1])}, "", home)
		if err != nil {
			t.Fatal(told(err, root))
		}
		if err = s.Save(e); err != nil {
			t.Fatal(err)
		}
	}
	for _, slug := range []string{"picked", "flag", "written", "created", "none", "now"} {
		theme := ""
		if slug == "picked" || slug == "written" {
			theme = "washi"
		}
		publish(t, s, dw, slug+".yaml", report(theme), nil, store.Options{})
	}
	// before writes the meta.json of slug as floe wrote it before: theme the --theme of its last publish, with
	// pickedTheme and createdTheme.
	before := func(slug string, keys map[string]string) {
		t.Helper()
		file := filepath.Join(root, slug, "meta.json")
		var m map[string]any
		if err := json.Unmarshal([]byte(read(t, file)), &m); err != nil {
			t.Fatal(err)
		}
		delete(m, "theme")
		delete(m, "givenTheme")
		for k, v := range keys {
			m[k] = v
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(file, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before("picked", map[string]string{"pickedTheme": "ink", "createdTheme": "app"})
	before("flag", map[string]string{"theme": "app", "createdTheme": "ink"})
	before("written", map[string]string{"createdTheme": "ink"})
	before("created", map[string]string{"createdTheme": "ink"})
	before("none", map[string]string{})
	before("l", map[string]string{"pickedTheme": "ink"})
	metas := func() map[string]string {
		t.Helper()
		got := map[string]string{}
		for _, slug := range []string{"picked", "flag", "written", "created", "none", "now", "l", "z"} {
			got[slug] = read(t, filepath.Join(root, slug, "meta.json"))
		}
		return got
	}
	was := metas()
	if err := s.MoveThemes(); err != nil {
		t.Fatal(told(err, root))
	}
	moved := metas()
	for slug, want := range map[string][2]string{
		"picked": {"ink", "washi"}, "flag": {"app", "app"}, "written": {"washi", "washi"}, "created": {"ink", ""},
		"none": {"default", ""}, "l": {"ink", "washi"},
	} {
		var m map[string]any
		if err := json.Unmarshal([]byte(moved[slug]), &m); err != nil {
			t.Fatal(err)
		}
		given := ""
		if g, ok := m["givenTheme"].(string); ok {
			given = g
		}
		_, picked := m["pickedTheme"]
		_, created := m["createdTheme"]
		if m["theme"] != want[0] || given != want[1] || picked || created {
			t.Errorf("%s moved to %v, want theme %q given %q", slug, m, want[0], want[1])
		}
	}
	for _, slug := range []string{"now", "z"} {
		if moved[slug] != was[slug] {
			t.Errorf("%s written:\n%s", slug, moved[slug])
		}
	}
	if err := s.MoveThemes(); err != nil {
		t.Fatal(told(err, root))
	}
	if again := metas(); !maps.Equal(again, moved) {
		t.Error("a second run wrote again")
	}
}
