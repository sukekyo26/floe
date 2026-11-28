package store_test

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"floe/internal/drawhere"
	floepublish "floe/internal/publish"
	"floe/internal/store"
	"floe/internal/usererr"
)

// A page has one theme. A theme picked draws its last version in it, the versions before kept as they were drawn, and
// stays until a publish names another than the last one named, by its data's theme: or --theme: a publish that names
// the same, or none, keeps it. A theme that cannot be drawn writes nothing, and another file's page replacing it is new.
func TestPickTheme(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	if err := setTheme(s, "app"); err != nil {
		t.Fatal(err)
	}
	pick := func(theme string) {
		t.Helper()
		if err := s.PickTheme(t.Context(), dw, "a", theme); err != nil {
			t.Fatal(told(err, root))
		}
	}
	// n tells the publishes apart, so that each is a version.
	n := 0
	pub := func(theme string, o store.Options) {
		t.Helper()
		n++
		publish(t, s, dw, "a.yaml", strings.Replace(report(theme), "title: T", "title: T"+strconv.Itoa(n), 1), nil, o)
	}
	for _, step := range []struct {
		name, theme string
		do          func()
	}{
		{"new, in the theme for new pages", "app", func() { pub("", store.Options{}) }},
		{"picked", "ink", func() { pick("ink") }},
		{"published again", "ink", func() { pub("", store.Options{}) }},
		{"theme: written", "washi", func() { pub("washi", store.Options{}) }},
		{"picked over theme:", "ink", func() { pick("ink") }},
		{"the same theme: published again", "ink", func() { pub("washi", store.Options{}) }},
		{"theme: taken out", "ink", func() { pub("", store.Options{}) }},
		{"--theme", "app", func() { pub("", store.Options{Theme: new("app")}) }},
		{"picked over --theme", "washi", func() { pick("washi") }},
		{"the same --theme published again", "washi", func() { pub("", store.Options{Theme: new("app")}) }},
		{"rebuilt", "washi", func() {
			if err := s.Rebuild(t.Context(), dw); err != nil {
				t.Fatal(told(err, root))
			}
		}},
		{"a theme gone", "washi", func() {
			if err := s.PickTheme(t.Context(), dw, "a", "gone"); keyOf(err) != "tmpl_unknown_theme" {
				t.Errorf("a theme gone: told %s", told(err, root))
			}
		}},
	} {
		step.do()
		if m := s.ReadMeta("a"); m.Theme != step.theme || drawnIn(t, s, root, "a") != step.theme {
			t.Errorf("%s: theme %q, drawn in %q; want %q", step.name, m.Theme, drawnIn(t, s, root, "a"), step.theme)
		}
	}
	// The versions before the last are kept as they were drawn.
	versions := func() map[string]string {
		t.Helper()
		got := map[string]string{}
		entries, err := os.ReadDir(filepath.Join(root, "a", "versions"))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			got[e.Name()] = read(t, filepath.Join(root, "a", "versions", e.Name()))
		}
		return got
	}
	before := versions()
	pick("ink")
	if after := versions(); len(before) == 0 || !maps.Equal(after, before) {
		t.Errorf("versions before the last: %d, then %d, changed %v", len(before), len(after), !maps.Equal(after, before))
	}
	// Replaced by another project's file: new, in the theme for new pages.
	up := floepublish.Upload{File: "a.yaml", Source: []byte(report("")), Images: nil, Project: "q", Repo: "q", Place: "a.yaml"}
	e, err := floepublish.PrepareData(t.Context(), s, dw, up, store.Options{Force: true})
	if err != nil {
		t.Fatal(told(err, root))
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	if m := s.ReadMeta("a"); m.Theme != "app" || drawnIn(t, s, root, "a") != "app" {
		t.Errorf("replaced: theme %q, drawn in %q", m.Theme, drawnIn(t, s, root, "a"))
	}
	if err = s.PickTheme(t.Context(), dw, "none", "ink"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("no page: %v", err)
	}
}

// A publish whose page has its theme picked between the drawing and its Save is refused, nothing written, so that the
// theme picked is not written over.
func TestPublishAfterPick(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{})
	up := floepublish.Upload{File: "a.yaml", Source: []byte(report("washi")), Images: nil, Project: "p", Repo: "p", Place: "a.yaml"}
	e, err := floepublish.PrepareData(t.Context(), s, dw, up, store.Options{})
	if err != nil {
		t.Fatal(told(err, root))
	}
	if err = s.PickTheme(t.Context(), dw, "a", "ink"); err != nil {
		t.Fatal(told(err, root))
	}
	err = s.Save(e)
	var c *store.Conflict
	if !errors.As(err, &c) || keyOf(err) != "store_theme_changed_publish" {
		t.Errorf("saved after a pick: told %s", told(err, root))
	}
	if m := s.ReadMeta("a"); m.Theme != "ink" || drawnIn(t, s, root, "a") != "ink" {
		t.Errorf("written: theme %q, drawn in %q", m.Theme, drawnIn(t, s, root, "a"))
	}
}

// keyOf is the key of the one mistake err holds, alone or in a list, "" for none.
func keyOf(err error) string {
	var e *usererr.Error
	var l usererr.List
	switch {
	case errors.As(err, &e):
		return e.Key
	case errors.As(err, &l) && len(l) == 1:
		return l[0].Key
	}
	return ""
}

// A page published, or a link put on another file, while it is drawn in the theme picked is refused, nothing written.
func TestPickThemeMeanwhile(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	icons := &meanwhile{svg: "<g>v1</g>", then: nil}
	dw := picKit(t, "", "")
	dw.Icons, dw.Draw = icons, drawhere.Here(dw.Kit, icons)
	publish(t, s, dw, "d.yaml", iconPage, nil, store.Options{})
	icons.then = func() {
		publish(t, s, dw, "d.yaml", strings.Replace(iconPage, "title: T", "title: Again", 1), nil, store.Options{})
	}
	err := s.PickTheme(t.Context(), dw, "d", "ink")
	var conflict *store.Conflict
	if !errors.As(err, &conflict) || keyOf(err) != "store_theme_changed" {
		t.Errorf("published meanwhile: told %s", told(err, root))
	}
	if m := s.ReadMeta("d"); m.Theme == "ink" || m.Title != "Again" {
		t.Errorf("written: %+v", m)
	}
	// A link put on another file meanwhile, which has no copy to tell it
	home := filepath.Dir(root)
	dir := filepath.Join(home, "proj")
	if err = os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	link := func(name string, o store.Options) {
		t.Helper()
		path := filepath.Join(dir, name)
		if werr := os.WriteFile(path, []byte(iconPage), 0o600); werr != nil {
			t.Fatal(werr)
		}
		e, lerr := floepublish.PrepareLink(t.Context(), s, dw, path, o, "", home)
		if lerr != nil {
			t.Fatal(told(lerr, root))
		}
		if lerr = s.Save(e); lerr != nil {
			t.Fatal(lerr)
		}
	}
	icons.then = nil
	link("a.yaml", store.Options{Slug: new("l")})
	icons.then = func() { link("b.yaml", store.Options{Slug: new("l"), Force: true}) }
	err = s.PickTheme(t.Context(), dw, "l", "ink")
	if !errors.As(err, &conflict) {
		t.Errorf("linked to another file meanwhile: told %s", told(err, root))
	}
	if m := s.ReadMeta("l"); m.Theme == "ink" || filepath.Base(m.Path) != "b.yaml" {
		t.Errorf("written: %+v", m)
	}
}

// A theme is picked for a page drawn from a data file, linked too, drawn to be checked and the pick alone written; not
// for a linked HTML file or folder.
func TestPickThemeLinks(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	home := filepath.Dir(root)
	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{"z.html": "<title>z</title>", "page.yaml": report("")} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	slugs := map[string]string{}
	for _, path := range []string{filepath.Join(dir, "z.html"), filepath.Join(dir, "page.yaml"), dir} {
		e, err := floepublish.PrepareLink(t.Context(), s, dw, path, store.Options{}, "", home)
		if err != nil {
			t.Fatal(told(err, root))
		}
		if err = s.Save(e); err != nil {
			t.Fatal(err)
		}
		slugs[filepath.Base(path)] = e.Slug
	}
	for _, name := range []string{"z.html", "proj"} {
		if err := s.PickTheme(t.Context(), dw, slugs[name], "ink"); keyOf(err) != "store_theme_link" {
			t.Errorf("%s: told %s", name, told(err, root))
		}
	}
	list, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range list {
		if want := l.Slug == slugs["page.yaml"]; l.Themed != want {
			t.Errorf("%s listed themed %v", l.Slug, l.Themed)
		}
	}
	data := slugs["page.yaml"]
	if err = s.PickTheme(t.Context(), dw, data, "gone"); keyOf(err) != "tmpl_unknown_theme" {
		t.Errorf("a theme gone: told %s", told(err, root))
	}
	if err = s.PickTheme(t.Context(), dw, data, "ink"); err != nil {
		t.Fatal(told(err, root))
	}
	drawn := func(when, want string) {
		t.Helper()
		m := s.ReadMeta(data)
		page, drawErr := store.DrawLink(t.Context(), dw, data, m, m.Path, false)
		if drawErr != nil || m.Theme != "ink" || !strings.Contains(string(page), "--accent: "+accents[want]) {
			t.Errorf("%s: theme %q, not drawn in %s, %v", when, m.Theme, want, drawErr)
		}
	}
	drawn("picked", "ink")
	// The file names another theme: drawn in it when opened, nothing written.
	if err = os.WriteFile(filepath.Join(dir, "page.yaml"), []byte(report("washi")), 0o600); err != nil {
		t.Fatal(err)
	}
	drawn("the file's theme: changed", "washi")
	// Picked again: the file's theme: as it is now is the one named.
	if err = s.PickTheme(t.Context(), dw, data, "ink"); err != nil {
		t.Fatal(told(err, root))
	}
	drawn("picked again", "ink")
}
