package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// A theme picked for a page draws it over the data's theme: and --theme, kept across publishes and rebuilds, dropped
// when another file's page replaces it; taking it off gives the page back to the data's theme:. The pick is written
// when the page drawn in it is the one it has, and not when it cannot be drawn.
func TestPickTheme(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	publish(t, s, dw, "a.yaml", report("washi"), nil, store.Options{})
	if err := s.PickTheme(dw, "a", "ink"); err != nil {
		t.Fatal(told(err, root))
	}
	at := func(when, picked, in string) {
		t.Helper()
		if m := s.ReadMeta("a"); m.PickedTheme != picked || drawnIn(t, s, root, "a") != in {
			t.Errorf("%s: picked %q, drawn in %q; want %q, %q", when, m.PickedTheme, drawnIn(t, s, root, "a"), picked, in)
		}
	}
	at("picked", "ink", "ink")
	publish(t, s, dw, "a.yaml", report("washi"), nil, store.Options{})
	at("published again", "ink", "ink")
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{Theme: ptr("app")})
	at("published with --theme", "ink", "ink")
	if err := s.Rebuild(dw); err != nil {
		t.Fatal(told(err, root))
	}
	at("rebuilt", "ink", "ink")
	// A theme that cannot be drawn: nothing written.
	err := s.PickTheme(dw, "a", "gone")
	if got := told(err, root); !strings.HasPrefix(got, `floe: there is no theme "gone"; use app, aurora,`) {
		t.Errorf("a theme gone: told %s", got)
	}
	at("a theme gone", "ink", "ink")
	if err = s.PickTheme(dw, "a", ""); err != nil {
		t.Fatal(told(err, root))
	}
	// Over the data's theme:, the --theme of the last publish
	at("taken off", "", "app")
	publish(t, s, dw, "a.yaml", report("washi"), nil, store.Options{})
	at("taken off, published", "", "washi")
	// The page drawn the same: the pick is written all the same.
	page := filepath.Join(root, "a", s.ReadMeta("a").Source, "page.html")
	before := read(t, page)
	if err = s.PickTheme(dw, "a", "washi"); err != nil {
		t.Fatal(told(err, root))
	}
	if read(t, filepath.Join(root, "a", s.ReadMeta("a").Source, "page.html")) != before {
		t.Error("drawn the same, the page changed")
	}
	at("drawn the same", "washi", "washi")
	// Replaced by another project's file: the pick goes.
	up := store.Upload{File: "a.yaml", Source: []byte(report("")), Images: nil, Project: "q"}
	e, err := s.PrepareData(dw, up, store.Options{Force: true})
	if err != nil {
		t.Fatal(told(err, root))
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	at("replaced", "", "")
	if err = s.PickTheme(dw, "none", "ink"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("no page: %v", err)
	}
}

// A page published while it is drawn in the theme picked is refused, nothing written.
func TestPickThemeMeanwhile(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	icons := &meanwhile{svg: "<g>v1</g>", then: nil}
	dw := picKit(t, "", "")
	dw.Icons = icons
	publish(t, s, dw, "d.yaml", iconPage, nil, store.Options{})
	icons.then = func() {
		publish(t, s, dw, "d.yaml", strings.Replace(iconPage, "title: T", "title: Again", 1), nil, store.Options{})
	}
	err := s.PickTheme(dw, "d", "ink")
	var conflict *store.Conflict
	if !errors.As(err, &conflict) || told(err, root) != "floe: page d has changed while it was drawn in the theme picked: pick it again" {
		t.Errorf("published meanwhile: told %s", told(err, root))
	}
	if m := s.ReadMeta("d"); m.PickedTheme != "" || m.Title != "Again" {
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
		e, err := s.PrepareLink(t.Context(), dw, path, store.Options{}, "", home)
		if err != nil {
			t.Fatal(told(err, root))
		}
		if err = s.Save(e); err != nil {
			t.Fatal(err)
		}
		slugs[filepath.Base(path)] = e.Slug
	}
	for _, name := range []string{"z.html", "proj"} {
		if err := s.PickTheme(dw, slugs[name], "ink"); told(err, root) != "floe: --theme picks the theme of a page drawn from a data file: a linked file is shown as it is" {
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
	if err = s.PickTheme(dw, data, "gone"); !strings.HasPrefix(told(err, root), `floe: there is no theme "gone"`) {
		t.Errorf("a theme gone: told %s", told(err, root))
	}
	if err = s.PickTheme(dw, data, "ink"); err != nil {
		t.Fatal(told(err, root))
	}
	m := s.ReadMeta(data)
	page, err := store.DrawLink(dw, data, m, m.Path, false)
	if err != nil || m.PickedTheme != "ink" || !strings.Contains(string(page), "--accent: "+accents["ink"]) {
		t.Errorf("a data file linked: %q, %v", m.PickedTheme, err)
	}
}
