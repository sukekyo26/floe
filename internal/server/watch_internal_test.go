package server

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/store"
)

// watched is a server whose own templates are under tpl (a symlink to the directory dir when link), with one
// template mine.
func watched(t *testing.T, link bool) (*Server, string) {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tmp, "dir")
	tpl := dir
	if link {
		tpl = filepath.Join(tmp, "tpl")
		if err := os.Symlink(dir, tpl); err != nil {
			t.Fatal(err)
		}
	}
	for name, text := range map[string]string{
		"mine/page.html":    "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>{{.Data.n}}</body></html>\n",
		"mine/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"mine/example.yaml": "lang: en\ntemplate: mine\nn: 1\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	drawing := func() (store.Drawing, error) {
		floe, err := kit.Floe()
		if err != nil {
			return store.Drawing{Kit: nil, Icons: nil, Err: nil}, err
		}
		none := kit.Dir{FS: nil, Name: ""}
		k, err := kit.Load(floe, kit.Sources{Parts: none, Themes: none, Templates: kit.Dir{FS: os.DirFS(tpl), Name: tpl}})
		return store.Drawing{Kit: k, Icons: nil, Err: nil}, err
	}
	s := New(Config{
		Store: store.New(filepath.Join(tmp, "store")), Port: 7325, Build: "test", Drawing: drawing, Templates: tpl,
		Parts: "", Themes: "", Icons: "", Home: tmp,
	})
	return s, dir
}

func write(t *testing.T, path, text string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The drawing is read again when what it is read from changes, however it is changed, and only then: not for a
// name kit leaves alone. A root that is a symlink is watched inside.
func TestLook(t *testing.T) {
	t.Parallel()

	for _, link := range []bool{false, true} {
		s, dir := watched(t, link)
		gen := func() int { _, g := s.now(); return g }
		page := filepath.Join(dir, "mine", "page.html")
		steps := []struct {
			what    string
			change  func()
			changed bool
		}{
			{"nothing", func() {}, false},
			{"a .swp written", func() { write(t, filepath.Join(dir, "mine", ".page.html.swp"), "x") }, false},
			{"page.html written in place", func() { write(t, page, "<p>{{.Data.n}}</p>\n") }, true},
			{"its mtime set back", func() {
				old := time.Unix(1, 0)
				if err := os.Chtimes(page, old, old); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"page.html replaced by a rename", func() {
				write(t, page+".new", "<p>{{.Data.n}}!</p>\n")
				if err := os.Rename(page+".new", page); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"a file added", func() { write(t, filepath.Join(dir, "mine", "page.css"), "p {}") }, true},
		}
		for _, step := range steps {
			before := gen()
			step.change()
			s.look()
			if got := gen() != before; got != step.changed {
				t.Errorf("link %v, %s: read again %v, want %v", link, step.what, got, step.changed)
			}
		}
	}
}

// A drawing that cannot be read is what every drawing refuses with, until it is mended.
func TestLookBroken(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	write(t, filepath.Join(dir, "mine", "schema.json"), "{")
	s.look()
	if d, _ := s.now(); d.Err == nil {
		t.Fatal("a broken template is read")
	}
	write(t, filepath.Join(dir, "mine", "schema.json"), `{"type": "object"}`)
	s.look()
	if d, _ := s.now(); d.Err != nil || d.Kit == nil {
		t.Errorf("mended, still %v", d.Err)
	}
}

// A version changes with a publish, a linked data file saved, the drawing read again and a restart; not with
// nothing.
func TestVersion(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	home := filepath.Dir(dir)
	data := filepath.Join(home, "proj", "d.yaml")
	if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, data, "lang: en\ntemplate: mine\nn: 1\n")
	d, _ := s.now()
	e, err := s.cfg.Store.PrepareLink(t.Context(), d, data, store.Options{}, "", home) //nolint:exhaustruct // none given
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cfg.Store.Save(e); err != nil {
		t.Fatal(err)
	}
	v := s.version("d", "")
	if s.version("d", "") != v {
		t.Error("a version changed with nothing")
	}
	write(t, data, "lang: en\ntemplate: mine\nn: 2\n")
	if s.version("d", "") == v {
		t.Error("the data file saved, the version is the same")
	}
	v = s.version("d", "")
	write(t, filepath.Join(dir, "mine", "page.html"), "<p>{{.Data.n}}</p>\n")
	s.look()
	if s.version("d", "") == v {
		t.Error("the drawing read again, the version is the same")
	}
	again, _ := watched(t, false)
	if again.id == s.id {
		t.Error("two servers of one id")
	}
}

// Serving ends with its context, the watching of the drawing with it.
func TestServeEnds(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	s.every = time.Millisecond
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Serve did not end")
	}
}
