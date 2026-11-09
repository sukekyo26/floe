package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

const ownSrc = "lang: en\ntemplate: pic\n"

// put publishes src as file from project p, at place, the slug given or none (nil).
func put(t *testing.T, s *store.Store, dw store.Drawing, file, place, src string, o store.Options) (*store.Entry, error) {
	t.Helper()

	up := store.Upload{File: file, Source: []byte(src), Images: map[string][]byte{}, Project: "p", Place: place}
	e, err := s.PrepareData(dw, up, o)
	if err != nil {
		return nil, err
	}
	return e, s.Save(e)
}

func rename(t *testing.T, s *store.Store, from, to string) {
	t.Helper()

	if _, err := s.Edit(from, store.Edit{Slug: &to}); err != nil { //nolint:exhaustruct // the slug alone
		t.Fatal(err)
	}
}

// A page renamed on the management page is found by its file published again without a slug: it gets the version,
// comes back from the archive, and no page is made at the old slug.
func TestPublishFindsRenamedPage(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	if _, err := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc, store.Options{}); err != nil {
		t.Fatal(told(err, root))
	}
	rename(t, s, "plan", "plan-b")
	if _, err := s.Edit("plan-b", store.Edit{Archived: ptr(true)}); err != nil { //nolint:exhaustruct // archived alone
		t.Fatal(err)
	}
	e, err := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc+"title: two\n", store.Options{})
	if err != nil || e.Slug != "plan-b" {
		t.Fatalf("published to %v, %v; want plan-b", e, told(err, root))
	}
	m := s.ReadMeta("plan-b")
	if len(m.Versions) != 2 || m.Archived {
		t.Errorf("plan-b: %d versions, archived %v; want 2, false", len(m.Versions), m.Archived)
	}
	if s.ReadMeta("plan") != nil {
		t.Error("a page was made at plan")
	}
}

// Renamed between Prepare and Save, the page is written under its new slug.
func TestPublishRenamedInBetween(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	if _, err := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc, store.Options{}); err != nil {
		t.Fatal(told(err, root))
	}
	up := store.Upload{File: "plan.yaml", Source: []byte(ownSrc + "title: two\n"), Images: map[string][]byte{}, Project: "p", Place: "plan.yaml"}
	e, err := s.PrepareData(dw, up, store.Options{})
	if err != nil {
		t.Fatal(told(err, root))
	}
	rename(t, s, "plan", "plan-b")
	if err = s.Save(e); err != nil || e.Slug != "plan-b" {
		t.Fatalf("saved to %s, %v; want plan-b", e.Slug, told(err, root))
	}
	if m := s.ReadMeta("plan-b"); len(m.Versions) != 2 || s.ReadMeta("plan") != nil {
		t.Errorf("plan-b has %d versions, plan %v; want 2, none", len(m.Versions), s.ReadMeta("plan"))
	}
}

// Without a slug, a publish writes to its file's page: the one at its name if the file's, else the first of the file's
// by slug, else its name. Pages of other projects, other places and of no place are not the file's.
func TestOwnSlugOrder(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		// pages are published first, each as slug from place, of project p unless other
		pages []struct{ slug, place, project string }
		file  string
		want  string // the slug written to, or the refusal told
	}{
		{"its name's", []struct{ slug, place, project string }{{"aa", "plan.yaml", "p"}, {"plan", "plan.yaml", "p"}}, "plan.yaml", "plan"},
		{"the first of its own", []struct{ slug, place, project string }{{"plan", "docs/plan.yaml", "p"}, {"plan-c", "plan.yaml", "p"}, {"plan-b", "plan.yaml", "p"}}, "plan.yaml", "plan-b"},
		{"another project's", []struct{ slug, place, project string }{{"other", "plan.yaml", "q"}}, "plan.yaml", "plan"},
		{"another place's", []struct{ slug, place, project string }{{"other", "docs/plan.yaml", "p"}}, "plan.yaml", "plan"},
		{"of no place", []struct{ slug, place, project string }{{"other", "", "p"}}, "plan.yaml", "plan"},
		{"a name of no slug, its page", []struct{ slug, place, project string }{{"notes", "日本語.yaml", "p"}}, "日本語.yaml", "notes"},
		{"a name of no slug, no page", nil, "日本語.yaml", `floe: invalid slug "": use 1 to 64 lowercase letters, digits and hyphens, starting and ending with a letter or digit`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, root := newStore(t)
			dw := picKit(t, "", "")
			for _, p := range c.pages {
				up := store.Upload{File: "x.yaml", Source: []byte(ownSrc), Images: map[string][]byte{}, Project: p.project, Place: p.place}
				e, err := s.PrepareData(dw, up, store.Options{Slug: ptr(p.slug)})
				if err == nil {
					err = s.Save(e)
				}
				if err != nil {
					t.Fatal(told(err, root))
				}
			}
			e, err := put(t, s, dw, c.file, c.file, ownSrc, store.Options{})
			got := told(err, root)
			if err == nil {
				got = e.Slug
			}
			if got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

// A store whose list cannot be read stops a publish without a slug, of a data file or a link, rather than take the name.
func TestOwnSlugUnreadable(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 { // root reads what a mode keeps
		t.Skip("root reads a directory of mode 0")
	}
	s, root := newStore(t)
	dw := picKit(t, "", "")
	if _, err := put(t, s, dw, "other.yaml", "other.yaml", ownSrc, store.Options{}); err != nil {
		t.Fatal(told(err, root))
	}
	if err := os.Chmod(root, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) }) //nolint:errcheck // for the temporary directory's removal
	html := filepath.Join(filepath.Dir(root), "a.html")
	if err := os.WriteFile(html, []byte("<title>a</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, dataErr := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc, store.Options{})
	_, linkErr := s.PrepareLink(t.Context(), dw, html, store.Options{}, "", filepath.Dir(filepath.Dir(root)))
	for _, err := range []error{dataErr, linkErr} {
		if want := "S: permission denied"; told(err, root) != want {
			t.Errorf("told %s, want %s", told(err, root), want)
		}
	}
}

// A link without a slug takes its file's entry at its name before another of its, earlier by slug.
func TestOwnSlugLink(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	html := filepath.Join(home, "page.html")
	if err := os.WriteFile(html, []byte("<title>p</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	dw := picKit(t, "", "")
	for _, slug := range []*string{nil, ptr("aa")} {
		e, err := s.PrepareLink(t.Context(), dw, html, store.Options{Slug: slug}, "", filepath.Dir(home))
		if err == nil {
			err = s.Save(e)
		}
		if err != nil {
			t.Fatal(told(err, root))
		}
	}
	e, err := s.PrepareLink(t.Context(), dw, html, store.Options{}, "", filepath.Dir(home))
	if err != nil || e.Slug != "page" {
		t.Errorf("picked %v, %s; want page", e, told(err, root))
	}
}

// --force without a slug replaces another file's page at the name only when the file has no page of its own.
func TestPublishForceWithoutSlug(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	if _, err := put(t, s, dw, "plan.yaml", "docs/plan.yaml", ownSrc, store.Options{}); err != nil {
		t.Fatal(told(err, root))
	}
	if _, err := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc, store.Options{Slug: ptr("plan-b")}); err != nil {
		t.Fatal(told(err, root))
	}
	e, err := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc+"title: two\n", store.Options{Force: true})
	if err != nil || e.Slug != "plan-b" || s.ReadMeta("plan").PublishedFile != "docs/plan.yaml" {
		t.Errorf("wrote %v, %s; want plan-b, plan kept", e, told(err, root))
	}
	e, err = put(t, s, dw, "plan.yaml", "notes/plan.yaml", ownSrc, store.Options{Force: true})
	if err != nil || e.Slug != "plan" || s.ReadMeta("plan").PublishedFile != "notes/plan.yaml" {
		t.Errorf("of no page: wrote %v, %s; want plan replaced", e, told(err, root))
	}
}

// A rename follows a title made of the slug, and no other.
func TestEditRenameTitle(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name, src string
		o         store.Options
		edit      store.Edit
		want      string
	}{
		{"made of the slug", ownSrc, store.Options{}, store.Edit{}, "plan-b"},                                             //nolint:exhaustruct // none
		{"title:", ownSrc + "title: Plan\n", store.Options{}, store.Edit{}, "Plan"},                                       //nolint:exhaustruct // none
		{"--title", ownSrc, store.Options{Title: ptr("Mine")}, store.Edit{}, "Mine"},                                      //nolint:exhaustruct // none
		{"pinned", ownSrc, store.Options{}, store.Edit{Title: ptr("plan")}, "plan"},                                       //nolint:exhaustruct // none
		{"given with the rename", ownSrc, store.Options{}, store.Edit{Title: ptr("New")}, "New"},                          //nolint:exhaustruct // none
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, root := newStore(t)
			dw := picKit(t, "", "")
			if _, err := put(t, s, dw, "plan.yaml", "plan.yaml", c.src, c.o); err != nil {
				t.Fatal(told(err, root))
			}
			if c.name == "pinned" {
				if _, err := s.Edit("plan", c.edit); err != nil {
					t.Fatal(err)
				}
				c.edit = store.Edit{} //nolint:exhaustruct // none
			}
			c.edit.Slug = ptr("plan-b")
			if _, err := s.Edit("plan", c.edit); err != nil {
				t.Fatal(err)
			}
			if got := s.ReadMeta("plan-b").Title; got != c.want {
				t.Errorf("title %q, want %q", got, c.want)
			}
		})
	}
}
