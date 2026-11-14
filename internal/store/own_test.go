package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"floe/internal/store"
	"floe/internal/usererr"
)

const ownSrc = "lang: en\ntemplate: pic\n"

// put publishes src as file from project p, at place, the slug given or none (nil).
func put(t *testing.T, s *store.Store, dw store.Drawing, file, place, src string, o store.Options) (*store.Entry, error) {
	t.Helper()

	up := store.Upload{File: file, Source: []byte(src), Images: map[string][]byte{}, Project: "p", Repo: "p", Place: place}
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

// Renamed between Prepare and Save, the page is written under its new slug, that of a file whose name makes no slug too,
// and when another file took the old slug meanwhile.
func TestPublishRenamedInBetween(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		file, slug string
		taken      bool
	}{{"plan.yaml", "plan", false}, {"日本語.yaml", "notes", false}, {"plan.yaml", "plan", true}} {
		t.Run(c.slug+strconv.FormatBool(c.taken), func(t *testing.T) {
			t.Parallel()

			s, root := newStore(t)
			dw := picKit(t, "", "")
			if _, err := put(t, s, dw, c.file, c.file, ownSrc, store.Options{Slug: ptr(c.slug)}); err != nil {
				t.Fatal(told(err, root))
			}
			up := store.Upload{File: c.file, Source: []byte(ownSrc + "title: two\n"), Images: map[string][]byte{}, Project: "p", Repo: "p", Place: c.file}
			e, err := s.PrepareData(dw, up, store.Options{})
			if err != nil {
				t.Fatal(told(err, root))
			}
			rename(t, s, c.slug, c.slug+"-b")
			if c.taken {
				if _, err = put(t, s, dw, "x.yaml", "docs/x.yaml", ownSrc, store.Options{Slug: ptr(c.slug)}); err != nil {
					t.Fatal(told(err, root))
				}
			}
			if err = s.Save(e); err != nil || e.Slug != c.slug+"-b" {
				t.Fatalf("saved to %s, %v; want %s-b", e.Slug, told(err, root), c.slug)
			}
			if m := s.ReadMeta(c.slug + "-b"); len(m.Versions) != 2 || (s.ReadMeta(c.slug) != nil) != c.taken {
				t.Errorf("%s-b has %d versions, %s %v; want 2, none", c.slug, len(m.Versions), c.slug, s.ReadMeta(c.slug))
			}
		})
	}
}

// Without a slug, a publish writes to its file's page: the one at its name if the file's, else the first of the file's
// by slug, else its name. Pages of other repositories, other places and of no place are not the file's.
func TestOwnSlugOrder(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		// pages are published first, each as slug from place, of repository p unless other
		pages []struct{ slug, place, repo string }
		file  string
		want  string // the slug written to, "" for the name refused as no slug
	}{
		{"its name's", []struct{ slug, place, repo string }{{"aa", "plan.yaml", "p"}, {"plan", "plan.yaml", "p"}}, "plan.yaml", "plan"},
		{"the first of its own", []struct{ slug, place, repo string }{{"plan", "docs/plan.yaml", "p"}, {"plan-c", "plan.yaml", "p"}, {"plan-b", "plan.yaml", "p"}}, "plan.yaml", "plan-b"},
		{"another repository's", []struct{ slug, place, repo string }{{"other", "plan.yaml", "q"}}, "plan.yaml", "plan"},
		{"another place's", []struct{ slug, place, repo string }{{"other", "docs/plan.yaml", "p"}}, "plan.yaml", "plan"},
		{"of no place", []struct{ slug, place, repo string }{{"other", "", "p"}}, "plan.yaml", "plan"},
		{"a name of no slug, its page", []struct{ slug, place, repo string }{{"notes", "日本語.yaml", "p"}}, "日本語.yaml", "notes"},
		{"a name of no slug, no page", nil, "日本語.yaml", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, root := newStore(t)
			dw := picKit(t, "", "")
			for _, p := range c.pages {
				up := store.Upload{File: "x.yaml", Source: []byte(ownSrc), Images: map[string][]byte{}, Project: "p", Repo: p.repo, Place: p.place}
				e, err := s.PrepareData(dw, up, store.Options{Slug: ptr(p.slug)})
				if err == nil {
					err = s.Save(e)
				}
				if err != nil {
					t.Fatal(told(err, root))
				}
			}
			e, err := put(t, s, dw, c.file, c.file, ownSrc, store.Options{})
			if c.want == "" {
				if u, ok := errors.AsType[*usererr.Error](err); !ok || u.Key != "store_slug_unnamed" || u.File() != c.file {
					t.Errorf("refused %v, want store_slug_unnamed at %s", err, c.file)
				}
				return
			}
			if err != nil || e.Slug != c.want {
				t.Errorf("wrote %v, %s; want %s", e, told(err, root), c.want)
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
		if u, ok := errors.AsType[*usererr.Error](err); !ok || u.File() != root {
			t.Errorf("refused %v, want the store unread", err)
		}
	}
}

// A link whose name makes no slug, and no page of its, is refused at its path; a slug given empty is a slug's mistake.
func TestOwnSlugUnnamed(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	dir := filepath.Join(home, "資料")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	html := filepath.Join(dir, "index.html")
	if err := os.WriteFile(html, []byte("<title>p</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	dw := picKit(t, "", "")
	_, err := s.PrepareLink(t.Context(), dw, html, store.Options{}, "", filepath.Dir(home))
	if u, ok := errors.AsType[*usererr.Error](err); !ok || u.Key != "store_slug_unnamed" || u.File() != html {
		t.Errorf("link refused %v, want store_slug_unnamed at %s", err, html)
	}
	_, err = put(t, s, dw, "日本語.yaml", "日本語.yaml", ownSrc, store.Options{Slug: ptr("")})
	if keyOf(err) != "store_invalid_slug" {
		t.Errorf("empty slug refused %v, want store_invalid_slug", err)
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
		{"made of the slug", ownSrc, store.Options{}, store.Edit{}, "plan-b"},                    //nolint:exhaustruct // none
		{"title:", ownSrc + "title: Plan\n", store.Options{}, store.Edit{}, "Plan"},              //nolint:exhaustruct // none
		{"--title", ownSrc, store.Options{Title: ptr("Mine")}, store.Edit{}, "Mine"},             //nolint:exhaustruct // none
		{"pinned", ownSrc, store.Options{}, store.Edit{Title: ptr("plan")}, "plan"},              //nolint:exhaustruct // none
		{"given with the rename", ownSrc, store.Options{}, store.Edit{Title: ptr("New")}, "New"}, //nolint:exhaustruct // none
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

// A stored page is its data file's from whatever project it is published: by the file's repository and place, else its
// real path alone, which a page published before keys were the file's alone holds with its project. Published again
// from another project, the page is written, renamed or not, and stays in its project; a new page is in the project
// published from, not the file's repository's. A page that kept no file is compared by the project published from,
// once, and by the file's key from then on.
func TestSameFileAnyProject(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	plans := filepath.Join(filepath.Dir(root), "plans", "db.yaml")
	pub := func(project, repo, place string, o store.Options) string {
		t.Helper()
		up := store.Upload{File: "db.yaml", Source: []byte(ownSrc), Images: map[string][]byte{}, Project: project, Repo: repo, Place: place}
		e, err := s.PrepareData(dw, up, o)
		if err != nil {
			t.Fatalf("from %s: %s", project, told(err, root))
		}
		if err = s.Save(e); err != nil {
			t.Fatal(err)
		}
		return e.Slug
	}
	pages := func() []string {
		t.Helper()
		list, err := s.List("")
		if err != nil {
			t.Fatal(err)
		}
		slugs := make([]string, 0, len(list))
		for _, a := range list {
			slugs = append(slugs, a.Slug)
		}
		slices.Sort(slugs)
		return slugs
	}
	check := func(what, slug, project string, want []string) {
		t.Helper()
		if m := s.ReadMeta(slug); m == nil || m.Project != project {
			t.Errorf("%s: %s in %+v, want project %s", what, slug, m, project)
		}
		if got := pages(); !slices.Equal(got, want) {
			t.Errorf("%s: pages %v, want %v", what, got, want)
		}
	}

	// A plans path: a file outside a repository.
	pub("a", "", plans, store.Options{})
	if got := pub("b", "", plans, store.Options{}); got != "db" {
		t.Errorf("from another project: wrote %s, want db", got)
	}
	check("from another project", "db", "a", []string{"db"})
	rename(t, s, "db", "wiki")
	if got := pub("b", "", plans, store.Options{}); got != "wiki" {
		t.Errorf("renamed, from another project: wrote %s, want wiki", got)
	}
	check("renamed, from another project", "wiki", "a", []string{"wiki"})

	// A path in a repository, the same from outside it.
	if got := pub("r", "r", "docs/x.yaml", store.Options{Slug: ptr("x")}); got != "x" {
		t.Fatalf("wrote %s, want x", got)
	}
	rename(t, s, "x", "xr")
	if got := pub("a", "r", "docs/x.yaml", store.Options{}); got != "xr" {
		t.Errorf("renamed, from outside the repository: wrote %s, want xr", got)
	}
	check("renamed, from outside the repository", "xr", "r", []string{"wiki", "xr"})

	// A plans path kept with the project published from, before keys were the file's alone, renamed.
	old := filepath.Join(filepath.Dir(root), "plans", "old.yaml")
	pub("floe", "floe", old, store.Options{Slug: ptr("old")})
	rename(t, s, "old", "older")
	if got := pub("b", "", old, store.Options{}); got != "older" {
		t.Errorf("a plans path kept with its project, renamed: wrote %s, want older", got)
	}

	// A new page is in the project published from.
	pub("w", "r", "docs/new.yaml", store.Options{Slug: ptr("new")})
	if m := s.ReadMeta("new"); m.Project != "w" || m.PublishedFrom != "r" {
		t.Errorf("a new page: project %q, file's repository %q, want w and r", m.Project, m.PublishedFrom)
	}

	// A page that kept no file: compared by the project published from, then keyed.
	pub("p", "p", "", store.Options{Slug: ptr("nofile")})
	up := store.Upload{File: "n.yaml", Source: []byte(ownSrc), Images: map[string][]byte{}, Project: "q", Repo: "", Place: plans + "2"}
	if _, err := s.PrepareData(dw, up, store.Options{Slug: ptr("nofile")}); err == nil {
		t.Error("a page that kept no file, from another project: no refusal")
	}
	pub("p", "", plans+"2", store.Options{Slug: ptr("nofile")})
	if m := s.ReadMeta("nofile"); m.PublishedFrom != "" || m.PublishedFile != plans+"2" {
		t.Errorf("a page that kept no file, published again: keyed %q %q", m.PublishedFrom, m.PublishedFile)
	}
	pub("q", "", plans+"2", store.Options{Slug: ptr("nofile")})
}
