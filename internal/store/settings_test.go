package store_test

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/kit"
	"floe/internal/store"
	"floe/internal/usererr"
)

// floeDrawing is floe's own templates, parts and themes, none of one's own.
func floeDrawing(t *testing.T) store.Drawing {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	return store.Drawing{Kit: k, Icons: nil, Err: nil}
}

// report is a page of floe's report, in theme when it names one.
func report(theme string) string {
	src := "lang: en\ntemplate: report\ntitle: T\nsections:\n  - heading: h\n    parts:\n      - text: x\n"
	if theme != "" {
		src = "theme: " + theme + "\n" + src
	}
	return src
}

// accents are the accent colours of themes' palettes, which a page drawn in one holds and no other does.
var accents = map[string]string{"ink": "#2b3a67", "washi": "#264c78", "app": "#c2185b"}

// drawnIn is the theme of accents the stored page slug of s is drawn in, "" for none of them (default).
func drawnIn(t *testing.T, s *store.Store, root, slug string) string {
	t.Helper()

	m := s.ReadMeta(slug)
	page, err := os.ReadFile(filepath.Join(root, slug, m.Source, "page.html"))
	if err != nil {
		t.Fatal(err)
	}
	in := ""
	for theme, accent := range accents {
		if strings.Contains(string(page), "--accent: "+accent) {
			if in != "" {
				t.Fatalf("%s is drawn in %s and %s", slug, in, theme)
			}
			in = theme
		}
	}
	return in
}

// settings.json is none chosen until written; what encoding/json/v2 refuses in it is its words at the file, and
// choosing again writes it whole, mending it.
func TestSettings(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	if set, err := s.ReadSettings(); err != nil || set.Theme != "" {
		t.Errorf("none written: %+v, %v", set, err)
	}
	if err := s.SetTheme("ink"); err != nil {
		t.Fatal(err)
	}
	if set, err := s.ReadSettings(); err != nil || set.Theme != "ink" {
		t.Errorf("written: %+v, %v", set, err)
	}
	file := filepath.Join(root, "settings.json")
	for _, broken := range []string{`{"theme": 1}`, `{"theme": "a", "theme": "b"}`, `{"theme": "a", "x": 1}`, `{`, ``} {
		if err := os.WriteFile(file, []byte(broken), 0o600); err != nil {
			t.Fatal(err)
		}
		var set store.Settings
		v2 := jsonv2.Unmarshal([]byte(broken), &set, jsonv2.RejectUnknownMembers(true))
		_, err := s.ReadSettings()
		if v2 == nil || told(err, root) != "S/settings.json: "+v2.Error() {
			t.Errorf("%s: told %s, v2 %v", broken, told(err, root), v2)
		}
		if err := s.SetTheme("washi"); err != nil {
			t.Fatal(err)
		}
		if set, err := s.ReadSettings(); err != nil || set.Theme != "washi" {
			t.Errorf("mended %s: %+v, %v", broken, set, err)
		}
	}
}

// A page drawn from a data file takes the default theme chosen when it is first stored, under the data's theme:,
// and keeps it: the default changed, published again, or changed between its drawing and its Save. A page replacing
// another file's (--force) is new; a default gone refuses only a new page that would be drawn in it.
func TestPublishCreatedTheme(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	publish(t, s, dw, "none.yaml", report(""), nil, store.Options{})
	if m := s.ReadMeta("none"); m.CreatedTheme != "" || drawnIn(t, s, root, "none") != "" {
		t.Errorf("none chosen: created %q, drawn in %q", m.CreatedTheme, drawnIn(t, s, root, "none"))
	}
	if err := s.SetTheme("ink"); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{})
	publish(t, s, dw, "own.yaml", report("app"), nil, store.Options{})
	publish(t, s, dw, "none.yaml", report(""), nil, store.Options{})
	if err := s.SetTheme("washi"); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{})
	publish(t, s, dw, "b.yaml", report(""), nil, store.Options{})
	// Drawn, then the default changed, then saved: kept as drawn.
	e, err := s.PrepareData(dw, store.Upload{File: "c.yaml", Source: []byte(report("")), Images: nil, Project: "p", Repo: "p", Place: "c.yaml"}, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetTheme("ink"); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	// Replacing another project's page with --force: new, in the default of now.
	up := store.Upload{File: "b.yaml", Source: []byte(report("")), Images: nil, Project: "q", Repo: "q", Place: "b.yaml"}
	if e, err = s.PrepareData(dw, up, store.Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	for slug, want := range map[string][2]string{
		"none": {"", ""}, "a": {"ink", "ink"}, "own": {"ink", "app"}, "c": {"washi", "washi"}, "b": {"ink", "ink"},
	} {
		if m := s.ReadMeta(slug); m.CreatedTheme != want[0] || drawnIn(t, s, root, slug) != want[1] {
			t.Errorf("%s: created %q, drawn in %q; want %q, %q", slug, m.CreatedTheme, drawnIn(t, s, root, slug), want[0], want[1])
		}
	}
	if err = s.SetTheme("gone"); err != nil {
		t.Fatal(err)
	}
	_, err = s.PrepareData(dw, store.Upload{File: "d.yaml", Source: []byte(report("")), Images: nil, Project: "p", Repo: "p", Place: "d.yaml"}, store.Options{})
	if got := told(err, root); !strings.HasPrefix(got, `floe: there is no theme "gone"; use app, aurora,`) {
		t.Errorf("a new page in a default gone: told %s", got)
	}
	publish(t, s, dw, "d.yaml", report("washi"), nil, store.Options{})
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{})
	if err = s.Rebuild(dw); err != nil {
		t.Errorf("Rebuild: %s", told(err, root))
	}
}

// A settings.json that cannot be read refuses a new page drawn from a data file, not one there already, nor a link of
// HTML or of a folder, which take no theme.
func TestSettingsBrokenLinks(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{})
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.PrepareData(dw, store.Upload{File: "b.yaml", Source: []byte(report("")), Images: nil, Project: "p", Repo: "p", Place: "b.yaml"}, store.Options{})
	if got := told(err, root); !strings.HasPrefix(got, "S/settings.json: ") {
		t.Errorf("a new page: told %s", got)
	}
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{})
	home := filepath.Dir(root)
	dir := filepath.Join(home, "proj", "site")
	if err = os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	html := filepath.Join(dir, "z.html")
	if err = os.WriteFile(html, []byte("<title>z</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{html, dir} {
		linked, linkErr := s.PrepareLink(t.Context(), dw, path, store.Options{}, "", home)
		if linkErr != nil {
			t.Fatalf("link %s: %s", path, told(linkErr, root))
		}
		if err = s.Save(linked); err != nil {
			t.Fatal(err)
		}
		if linked.Meta.CreatedTheme != "" {
			t.Errorf("link %s took %q", path, linked.Meta.CreatedTheme)
		}
	}
	// A data file linked is drawn from a data file: it reads the default as a stored page does.
	data := filepath.Join(home, "proj", "page.yaml")
	if err = os.WriteFile(data, []byte(report("")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PrepareLink(t.Context(), dw, data, store.Options{}, "", home); !strings.HasPrefix(told(err, root), "S/settings.json: ") {
		t.Errorf("a data file linked: told %s", told(err, root))
	}
	if err = s.SetTheme("ink"); err != nil {
		t.Fatal(err)
	}
	e, err := s.PrepareLink(t.Context(), dw, data, store.Options{}, "", home)
	if err != nil {
		t.Fatal(told(err, root))
	}
	if e.Meta.CreatedTheme != "ink" {
		t.Errorf("a data file linked took %q", e.Meta.CreatedTheme)
	}
}

// A publish to another project's page is told its drawing's mistakes before the owner, and the owner's refusal (409)
// with the page's warnings, as before a page took the default theme.
func TestPublishOrderKept(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := floeDrawing(t)
	publish(t, s, dw, "a.yaml", report(""), nil, store.Options{})
	_, err := s.PrepareData(dw, store.Upload{File: "a.yaml", Source: []byte("title: a: b\n"), Images: nil, Project: "q", Repo: "q", Place: "a.yaml"}, store.Options{})
	if got := told(err, root); got != "a.yaml:1:8: mapping value is not allowed in this context" {
		t.Errorf("a mistake of the data first: told %s", got)
	}
	fenced := strings.Replace(report(""), "text: x", "text: \"```\\nx\\n```\"", 1)
	_, err = s.PrepareData(dw, store.Upload{File: "a.yaml", Source: []byte(fenced), Images: nil, Project: "q", Repo: "q", Place: "a.yaml"}, store.Options{})
	var conflict *store.Conflict
	report := usererr.Tell(i18n.EnglishCatalog(), err)
	if !errors.As(err, &conflict) || len(report.Warnings) != 1 {
		t.Errorf("the owner with the warnings: %+v, %v", report, err)
	}
}
