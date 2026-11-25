package store_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	floepublish "floe/internal/publish"
	"floe/internal/store"
	"floe/internal/usererr"
)

var update = flag.Bool("update", false, "write testdata/owners.golden anew")

// ownerCase is a file as a publish names it: a data file of a project, with its key, or a link, by its file's name. A
// place that begins with <tmp> is a real path.
type ownerCase struct {
	name, project, repo, place, link string
}

// owners are the pages there at the slug, none first, and publishers the publishes sent to it.
var (
	owners = []ownerCase{
		{"none", "", "", "", ""},
		{"a file of r", "p", "r", "docs/a.yaml", ""},
		{"a file of o", "p", "o", "docs/a.yaml", ""},
		{"a real path", "p", "", "<tmp>/a.yaml", ""},
		{"a real path, with its project", "p", "proj", "<tmp>/a.yaml", ""},
		{"no file, from p", "p", "p", "", ""},
		{"no file, from none", "", "", "", ""},
		{"a link", "", "", "", "l1"},
	}
	publishers = []ownerCase{
		{"a file of r", "q", "r", "docs/a.yaml", ""},
		{"another of r", "q", "r", "docs/b.yaml", ""},
		{"a file of x at the place", "q", "x", "docs/a.yaml", ""},
		{"a file of x at the place, from p", "p", "x", "docs/a.yaml", ""},
		{"a real path", "q", "", "<tmp>/a.yaml", ""},
		{"another real path", "q", "", "<tmp>/b.yaml", ""},
		{"no file, from p", "p", "p", "", ""},
		{"no file, from q", "q", "q", "", ""},
		{"the link", "", "", "", "l1"},
		{"another link", "", "", "", "l2"},
	}
)

// ownerStore is a store and the files that the publishes of owners and publishers are of.
type ownerStore struct {
	s         *store.Store
	dw        store.Drawing
	home, tmp string
}

// put prepares and saves c at the slug s.
func (o ownerStore) put(t *testing.T, c ownerCase, force bool) error {
	t.Helper()

	var e *store.Entry
	var err error
	opts := store.Options{Slug: new("s"), Force: force} //nolint:exhaustruct // the rest not given
	if c.link != "" {
		e, err = floepublish.PrepareLink(t.Context(), o.s, o.dw, filepath.Join(o.home, "proj", c.link+".html"), opts, "", o.home)
	} else {
		place := c.place
		if rest, ok := strings.CutPrefix(place, "<tmp>/"); ok {
			place = filepath.Join(o.tmp, filepath.FromSlash(rest))
		}
		e, err = floepublish.PrepareData(t.Context(), o.s, o.dw, floepublish.Upload{
			File: "a.yaml", Source: []byte(ownSrc), Images: map[string][]byte{}, Project: c.project, Repo: c.repo, Place: place,
		}, opts)
	}
	if err != nil {
		return err
	}
	return o.s.Save(e)
}

// answered is what o answers the publish c over the page there, which holds a tag to tell that it goes on: the key of
// the refusal, or what came of the page and the key of its file.
func (o ownerStore) answered(t *testing.T, there bool, c ownerCase, force bool) string {
	t.Helper()

	if err := o.put(t, c, force); err != nil {
		u, ok := errors.AsType[*usererr.Error](err)
		if !ok {
			t.Fatalf("%s: %v", c.name, err)
		}
		return u.Key
	}
	m := o.s.ReadMeta("s")
	how := "new"
	if there {
		how = "replaced"
		if len(m.Tags) > 0 {
			how = "goes on"
		}
	}
	shown := strings.ReplaceAll(filepath.ToSlash(m.PublishedFile), filepath.ToSlash(o.tmp), "<tmp>")
	return fmt.Sprintf("%s; link %t, from %q, file %q, project %q", how, m.IsLink(), m.PublishedFrom, shown, m.Project)
}

// What a publish at the slug of another page comes to, for each kind of page there and of publish, with and without
// --force: what it answers must not change (testdata/owners.golden). The publishes name their slug, so that OwnSlug is
// not what picks it (TestOwnSlugOrder, TestOwnSlugLinkRenamed).
func TestOwnerAnswers(t *testing.T) {
	t.Parallel()

	dw := picKit(t, "", "")
	var out strings.Builder
	for _, there := range owners {
		for _, c := range publishers {
			for _, force := range []bool{false, true} {
				s, root := newStore(t)
				o := ownerStore{s: s, dw: dw, home: filepath.Dir(root), tmp: filepath.Dir(root)}
				writeLinks(t, o.home)
				if there.name != "none" {
					if err := o.put(t, there, false); err != nil {
						t.Fatalf("%s: %v", there.name, err)
					}
					//nolint:exhaustruct // the tag alone
					if _, err := s.Edit("s", store.Edit{Tags: &store.TagsEdit{Add: []string{"mark"}, Remove: nil}}); err != nil {
						t.Fatal(err)
					}
				}
				forced := ""
				if force {
					forced = " --force"
				}
				fmt.Fprintf(&out, "%s | %s%s: %s\n", there.name, c.name, forced, o.answered(t, there.name != "none", c, force))
			}
		}
	}
	golden := filepath.Join("testdata", "owners.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(out.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v; run just golden", err)
	}
	if got := out.String(); got != string(want) {
		for i, line := range strings.Split(got, "\n") {
			if old := strings.Split(string(want), "\n"); i >= len(old) || old[i] != line {
				t.Errorf("answered %s; testdata/owners.golden has %q", line, cut(old, i))
			}
		}
	}
}

func cut(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
}

// writeLinks writes the HTML files linked under home.
func writeLinks(t *testing.T, home string) {
	t.Helper()

	dir := filepath.Join(home, "proj")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"l1", "l2"} {
		if err := os.WriteFile(filepath.Join(dir, name+".html"), []byte("<title>x</title>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// A page published again with --force from its own file goes on in the theme picked for it, whatever project it is
// from: the themes are of the owner's file, as the meta.json kept is. A link whose slug another file takes over is
// drawn in the default of now.
func TestThemesOwner(t *testing.T) {
	t.Parallel()

	for _, c := range []ownerCase{
		{"a file of r", "p", "r", "docs/a.yaml", ""},
		{"a real path", "p", "", "<tmp>/a.yaml", ""},
		{"no file, from p", "p", "p", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, root := newStore(t)
			dw := floeDrawing(t)
			tmp := filepath.Dir(root)
			pub := func(project, repo, place string, force bool) {
				t.Helper()
				if rest, ok := strings.CutPrefix(place, "<tmp>/"); ok {
					place = filepath.Join(tmp, rest)
				}
				up := floepublish.Upload{File: "a.yaml", Source: []byte(report("")), Images: map[string][]byte{}, Project: project, Repo: repo, Place: place}
				e, err := floepublish.PrepareData(t.Context(), s, dw, up, store.Options{Slug: new("s"), Force: force}) //nolint:exhaustruct // the rest not given
				if err == nil {
					err = s.Save(e)
				}
				if err != nil {
					t.Fatal(told(err, root))
				}
			}
			pub(c.project, c.repo, c.place, false)
			if err := s.PickTheme(t.Context(), dw, "s", "ink"); err != nil {
				t.Fatal(told(err, root))
			}
			// Another project, but for the page that kept no file: that one is compared by the project.
			project, repo := "q", c.repo
			if c.place == "" {
				project, repo = "p", "x"
			}
			pub(project, repo, c.place, true)
			if m := s.ReadMeta("s"); m.PickedTheme != "ink" || drawnIn(t, s, root, "s") != "ink" {
				t.Errorf("published again: picked %q, drawn in %q; want ink", m.PickedTheme, drawnIn(t, s, root, "s"))
			}
		})
	}

	s, root := newStore(t)
	dw := floeDrawing(t)
	home := filepath.Dir(root)
	link := func(name string, force bool) {
		t.Helper()
		file := filepath.Join(home, "proj", name)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(report("")), 0o600); err != nil {
			t.Fatal(err)
		}
		e, err := floepublish.PrepareLink(t.Context(), s, dw, file, store.Options{Slug: new("s"), Force: force}, "", home) //nolint:exhaustruct // the rest not given
		if err == nil {
			err = s.Save(e)
		}
		if err != nil {
			t.Fatal(told(err, root))
		}
	}
	if err := setTheme(s, "ink"); err != nil {
		t.Fatal(err)
	}
	link("a.yaml", false)
	if err := setTheme(s, "washi"); err != nil {
		t.Fatal(err)
	}
	link("a.yaml", true)
	if m := s.ReadMeta("s"); m.CreatedTheme != "ink" {
		t.Errorf("the same file linked again with --force: created %q, want ink", m.CreatedTheme)
	}
	link("b.yaml", true)
	if m := s.ReadMeta("s"); m.CreatedTheme != "washi" {
		t.Errorf("another file linked with --force: created %q, want washi", m.CreatedTheme)
	}
}

// A link renamed is found again by its file without a slug, and one that another page of its file joins between Prepare
// and Save is saved where it was found.
func TestOwnSlugLinkRenamed(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	home := filepath.Dir(root)
	html := filepath.Join(home, "page.html")
	if err := os.WriteFile(html, []byte("<title>p</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepare := func(slug *string) *store.Entry {
		t.Helper()
		e, err := floepublish.PrepareLink(t.Context(), s, dw, html, store.Options{Slug: slug}, "", filepath.Dir(home)) //nolint:exhaustruct // the rest not given
		if err != nil {
			t.Fatal(told(err, root))
		}
		return e
	}
	save := func(e *store.Entry) {
		t.Helper()
		if err := s.Save(e); err != nil {
			t.Fatal(told(err, root))
		}
	}
	save(prepare(nil))
	rename(t, s, "page", "notes")
	if e := prepare(nil); e.Slug != "notes" {
		t.Errorf("renamed, linked again at %s, want notes", e.Slug)
	}
	// Renamed between Prepare and Save, the link is written where it went.
	e := prepare(nil)
	rename(t, s, "notes", "moved")
	if save(e); e.Slug != "moved" || s.ReadMeta("notes") != nil {
		t.Errorf("saved to %s, notes %v; want moved alone", e.Slug, s.ReadMeta("notes"))
	}

	// A data file's page stays the one found, though another of its pages comes before it meanwhile.
	if _, err := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc, store.Options{Slug: new("plan-b")}); err != nil { //nolint:exhaustruct // the rest not given
		t.Fatal(told(err, root))
	}
	if _, err := put(t, s, dw, "other.yaml", "other.yaml", ownSrc, store.Options{Slug: new("plan")}); err != nil { //nolint:exhaustruct // the rest not given
		t.Fatal(told(err, root))
	}
	up := floepublish.Upload{File: "plan.yaml", Source: []byte(ownSrc + "title: two\n"), Images: map[string][]byte{}, Project: "p", Repo: "p", Place: "plan.yaml"}
	found, err := floepublish.PrepareData(t.Context(), s, dw, up, store.Options{}) //nolint:exhaustruct // none given
	if err != nil {
		t.Fatal(told(err, root))
	}
	if _, err = put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc, store.Options{Slug: new("plan-a")}); err != nil { //nolint:exhaustruct // the rest not given
		t.Fatal(told(err, root))
	}
	if save(found); found.Slug != "plan-b" {
		t.Errorf("saved to %s, want plan-b", found.Slug)
	}
}
