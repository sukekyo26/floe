package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

func ptr[T any](v T) *T {
	return &v
}

// env is a store in a temporary directory, outside any git repository, with a home of its own.
type env struct {
	tmp string
	s   *store.Store
}

func newEnv(t *testing.T) env {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mkdir(t, filepath.Join(tmp, "home"))
	return env{tmp: tmp, s: store.New(filepath.Join(tmp, "store"), filepath.Join(tmp, "home"), tmpl.NewLibrary("", ""))}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
}

func (e env) write(t *testing.T, rel, body string) string {
	t.Helper()

	path := filepath.Join(e.tmp, rel)
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// publish prepares and saves file from cwd (a directory under tmp, created when missing).
func (e env) publish(t *testing.T, file string, opts store.Options, cwd string) (*store.Entry, error) {
	t.Helper()

	dir := filepath.Join(e.tmp, cwd)
	mkdir(t, dir)
	entry, err := e.s.Prepare(t.Context(), file, opts, dir)
	if err != nil {
		return nil, err
	}
	if err := e.s.Save(entry); err != nil {
		t.Fatal(err)
	}
	return entry, nil
}

func (e env) mustPublish(t *testing.T, file string, opts store.Options, cwd string) *store.Entry {
	t.Helper()

	entry, err := e.publish(t, file, opts, cwd)
	if err != nil {
		t.Fatalf("publish %s: %v", file, err)
	}
	return entry
}

func (e env) meta(t *testing.T, slug string) *store.Meta {
	t.Helper()

	m := e.s.ReadMeta(slug)
	if m == nil {
		t.Fatalf("no meta.json for %s", slug)
	}
	return m
}

func (e env) read(t *testing.T, rel string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(e.tmp, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// page writes a data file for the report template, titled title ("" for none).
func (e env) page(t *testing.T, rel, title string) string {
	t.Helper()

	src := "template: report\nlang: en\n"
	if title != "" {
		src += "title: " + title + "\n"
	}
	return e.write(t, rel, src)
}

// wantUserErr fails unless err is a usererr with the message key and, when given, args; it returns a
// copy of it.
func wantUserErr(t *testing.T, err error, key string, args ...any) usererr.Error {
	t.Helper()

	var ue *usererr.Error
	if !errors.As(err, &ue) || ue.Key != key || len(args) > 0 && !slices.Equal(ue.Args, args) {
		t.Fatalf("err = %#v, want a usererr %s%v", err, key, args)
	}
	return *ue
}

func TestPublishStoredPage(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	doc := e.page(t, "My Demo.yaml", "Demo Page")
	first := e.mustPublish(t, doc, store.Options{Description: ptr("a demo")}, "proj")
	m := e.meta(t, "my-demo")
	if first.Slug != "my-demo" || first.Prev != nil || m.Title != "Demo Page" || m.Description != "a demo" ||
		m.Project != "proj" || m.PublishedFrom != "proj" || m.Path != "" || m.CreatedAt != m.UpdatedAt {
		t.Fatalf("first publish: %+v, %+v", first, m)
	}
	if got := e.read(t, "store/my-demo/index.html"); !strings.Contains(got, "<h1>Demo Page</h1>") || m.Template != "report" || !m.StrictCSP ||
		!strings.Contains(e.read(t, "store/my-demo/meta.json"), `"strictCsp": true`) {
		t.Errorf("the page is stored as %q (template %q, strictCsp %v), want it drawn by report and cut off", got, m.Template, m.StrictCSP)
	}

	again := e.mustPublish(t, doc, store.Options{Slug: ptr("my-demo")}, "proj")
	if again.Prev == nil || again.Prev.Title != "Demo Page" {
		t.Errorf("a republish does not report what it overwrites: %+v", again.Prev)
	}
	if m2 := e.meta(t, "my-demo"); m2.Description != "a demo" || m2.CreatedAt != m.CreatedAt {
		t.Errorf("a republish lost the description or createdAt: %+v", m2)
	}
	e.mustPublish(t, doc, store.Options{Title: ptr("Renamed"), Description: ptr("")}, "proj")
	if m3 := e.meta(t, "my-demo"); m3.Title != "Renamed" || m3.Description != "" {
		t.Errorf("--title and an empty --description: %+v", m3)
	}
}

// A stored page is drawn from a data file: HTML is refused unless linked, and a data file without a
// title: is titled by its slug.
func TestPublishNeedsDataFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	frag := e.write(t, "frag.html", "<h1>A</h1>")
	_, err := e.publish(t, frag, store.Options{}, "proj")
	wantUserErr(t, err, "store_not_data", frag)
	// The refusal comes before reading, so a missing HTML file is pointed to --link too.
	missing := filepath.Join(e.tmp, "missing.html")
	_, err = e.publish(t, missing, store.Options{}, "proj")
	wantUserErr(t, err, "store_not_data", missing)
	e.mustPublish(t, e.page(t, "untitled.yaml", ""), store.Options{}, "proj")
	if m := e.meta(t, "untitled"); m.Title != "untitled" || !strings.Contains(e.read(t, "store/untitled/index.html"), "<h1>untitled</h1>") {
		t.Errorf("a data file without a title: is titled %q", m.Title)
	}
	// Removing title: from the data file gives the slug back, as clearing the title does.
	e.mustPublish(t, e.page(t, "untitled.yaml", "Once"), store.Options{}, "proj")
	e.mustPublish(t, e.page(t, "untitled.yaml", ""), store.Options{}, "proj")
	if m := e.meta(t, "untitled"); m.Title != "untitled" || m.DataTitle != "" {
		t.Errorf("a data file whose title: is removed: %+v", m)
	}
}

func TestPublishNames(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	long := strings.Repeat("a", 63)
	tests := []struct {
		file, cwd, want string
	}{
		{long + " b.yaml", "proj", long},
		{"Report_2026.YML", "proj", "report-2026"},
		{"docs/index.html", "proj", "docs"},
		{"index.html", "proj/rel", "rel"},
	}
	// index.html, which only a link takes, is named after its directory.
	for _, tt := range tests {
		e.write(t, filepath.Join(tt.cwd, tt.file), "template: report\nlang: en\n")
		entry := e.mustPublish(t, tt.file, store.Options{Link: !store.IsDataFile(tt.file)}, tt.cwd)
		if entry.Slug != tt.want {
			t.Errorf("publish %s from %s: slug %q, want %q", tt.file, tt.cwd, entry.Slug, tt.want)
		}
	}

	_, err := e.publish(t, filepath.Join(e.tmp, "proj", "Report_2026.YML"), store.Options{Slug: ptr("../x")}, "proj")
	ue := wantUserErr(t, err, "store_invalid_slug", "../x")
	if ja := ue.Localize(i18n.New("ja")); !strings.Contains(ja, "URL 名「../x」は使えません") {
		t.Errorf("ja = %q", ja)
	}
	_, err = e.publish(t, filepath.Join(e.tmp, "nope.yaml"), store.Options{}, "proj")
	wantUserErr(t, err, "file_cannot_read", filepath.Join(e.tmp, "nope.yaml"), "no such file or directory")
	if _, err := os.Stat(filepath.Join(e.tmp, "store", "nope")); !errors.Is(err, os.ErrNotExist) {
		t.Error("a failed publish left something in the store")
	}
}

func TestPublishOwner(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	doc := e.page(t, "My Demo.yaml", "Demo Page")
	e.mustPublish(t, doc, store.Options{Description: ptr("repo page")}, "repo")
	if _, err := e.s.Edit("my-demo", store.Edit{Favorite: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	before := e.meta(t, "my-demo")

	_, err := e.publish(t, doc, store.Options{}, "other")
	wantUserErr(t, err, "store_slug_owned", "my-demo", "repo")
	if m := e.meta(t, "my-demo"); *m != *before {
		t.Errorf("a refused publish changed the page: %+v", m)
	}

	e.mustPublish(t, doc, store.Options{Force: true}, "other")
	m := e.meta(t, "my-demo")
	if m.PublishedFrom != "other" || m.Project != "other" || m.Favorite || m.Description != "" || m.CreatedAt == before.CreatedAt {
		t.Errorf("--force did not start the page afresh: %+v", m)
	}

	// A page from before publishedFrom is owned by its project, unless that was edited on the page.
	old := &store.Meta{Title: "t", Project: "legacy", CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
	if err = e.s.WriteMeta("my-demo", old); err != nil {
		t.Fatal(err)
	}
	_, err = e.publish(t, doc, store.Options{}, "other")
	wantUserErr(t, err, "store_slug_owned", "my-demo", "legacy")
	old.ProjectEdited = true
	if err = e.s.WriteMeta("my-demo", old); err != nil {
		t.Fatal(err)
	}
	e.mustPublish(t, doc, store.Options{}, "other")
}

func TestPublishKeepsEdits(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	doc := e.page(t, "My Demo.yaml", "Demo Page")
	e.mustPublish(t, doc, store.Options{}, "repo")
	if _, err := e.s.Edit("my-demo", store.Edit{Favorite: ptr(true), Title: ptr("Mine"), Project: ptr("p")}); err != nil {
		t.Fatal(err)
	}
	e.mustPublish(t, doc, store.Options{}, "repo")
	if m := e.meta(t, "my-demo"); !m.Favorite || m.Title != "Mine" || m.Project != "p" || m.PublishedFrom != "repo" {
		t.Errorf("a republish lost the edits: %+v", m)
	}
	e.mustPublish(t, doc, store.Options{Title: ptr("Agent")}, "repo")
	if m := e.meta(t, "my-demo"); m.Title != "Agent" || m.TitleEdited || m.Project != "p" {
		t.Errorf("--title should win and unpin the title, keeping the project: %+v", m)
	}
	if _, err := e.s.Edit("my-demo", store.Edit{Favorite: ptr(false), Title: ptr(""), Project: ptr("")}); err != nil {
		t.Fatal(err)
	}
	e.mustPublish(t, doc, store.Options{}, "repo")
	if m := e.meta(t, "my-demo"); m.Title != "Demo Page" || m.Project != "repo" || m.Favorite || m.TitleEdited || m.ProjectEdited {
		t.Errorf("cleared edits should hand the title and project back: %+v", m)
	}
}

func TestPublishLink(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.write(t, "site/index.html", "<h1>Linked v1</h1>")
	entry := e.mustPublish(t, file, store.Options{Link: true, Slug: ptr("site"), Description: ptr("d")}, "elsewhere")
	m := e.meta(t, "site")
	if entry.Page != nil || m.Path != file || m.Project != "site" || m.PublishedFrom != "" || m.Title != "" {
		t.Errorf("a link: %+v", m)
	}
	if des, err := os.ReadDir(filepath.Join(e.tmp, "store", "site")); err != nil || len(des) != 1 {
		t.Errorf("a link stores %d files (%v), want meta.json alone", len(des), err)
	}

	// A file reached through a symlink is stored by its real path and named after the link.
	e.write(t, "real/page.html", "<h1>R</h1>")
	mkdir(t, filepath.Join(e.tmp, "pub"))
	if err := os.Symlink(filepath.Join(e.tmp, "real/page.html"), filepath.Join(e.tmp, "pub/shown.html")); err != nil {
		t.Fatal(err)
	}
	shown := e.mustPublish(t, filepath.Join(e.tmp, "pub/shown.html"), store.Options{Link: true}, "proj")
	if shown.Slug != "shown" || shown.Meta.Path != filepath.Join(e.tmp, "real/page.html") {
		t.Errorf("a symlinked file: %s %s", shown.Slug, shown.Meta.Path)
	}

	// The same slug for another path is refused unless --force, which keeps nothing of the old link.
	if _, err := e.s.Edit("site", store.Edit{Favorite: ptr(true), Title: ptr("Pinned")}); err != nil {
		t.Fatal(err)
	}
	other := e.write(t, "site/sub/p.html", "<title>Sub</title>")
	_, err := e.publish(t, other, store.Options{Link: true, Slug: ptr("site")}, "proj")
	if ue := wantUserErr(t, err, "store_slug_linked", "site", file); !ue.Conflict {
		t.Errorf("the refusal is not a conflict for the management page: %+v", ue)
	}
	again := e.mustPublish(t, file, store.Options{Link: true, Slug: ptr("site")}, "proj")
	if m := e.meta(t, "site"); again.Prev == nil || m.Title != "Pinned" || !m.TitleEdited || !m.Favorite || m.Description != "d" {
		t.Errorf("re-registering the same path should keep the pinned title and the rest: %+v", m)
	}
	created := e.meta(t, "site").CreatedAt
	e.mustPublish(t, other, store.Options{Link: true, Slug: ptr("site"), Force: true}, "proj")
	if m := e.meta(t, "site"); m.Path != other || m.Title != "" || m.TitleEdited || m.Favorite || m.Description != "" ||
		m.CreatedAt == created {
		t.Errorf("--force should replace the link afresh: %+v", m)
	}
}

func TestPublishLinkSlugs(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	site := e.write(t, "site/index.html", "<p>site</p>")
	other := e.write(t, "other/site/index.html", "<p>other</p>")
	if got := e.mustPublish(t, site, store.Options{Link: true}, "proj").Slug; got != "site" {
		t.Fatalf("slug %q, want site", got)
	}
	if got := e.mustPublish(t, other, store.Options{Link: true}, "proj").Slug; got != "site-2" {
		t.Errorf("an omitted slug another file uses: %q, want site-2", got)
	}
	if err := e.s.Remove("site-2"); err != nil {
		t.Fatal(err)
	}
	e.mustPublish(t, other, store.Options{Link: true, Slug: ptr("by-hand")}, "proj")
	if got := e.mustPublish(t, other, store.Options{Link: true}, "proj").Slug; got != "by-hand" {
		t.Errorf("an omitted slug should reuse the entry of the same path: %q", got)
	}
	long := strings.Repeat("b", 63)
	e.mustPublish(t, e.write(t, "l1/"+long+"/index.html", "1"), store.Options{Link: true}, "proj")
	if got := e.mustPublish(t, e.write(t, "l2/"+long+"/index.html", "2"), store.Options{Link: true}, "proj").Slug; got != strings.Repeat("b", 62)+"-2" {
		t.Errorf("a numbered slug is cut to 64 characters: %q", got)
	}

	// A slug cannot switch between a stored page and a link.
	_, err := e.publish(t, e.page(t, "untitled.yaml", ""), store.Options{Slug: ptr("site")}, "proj")
	wantUserErr(t, err, "store_slug_is_link", "site")
	e.mustPublish(t, filepath.Join(e.tmp, "untitled.yaml"), store.Options{}, "proj")
	_, err = e.publish(t, site, store.Options{Link: true, Slug: ptr("untitled")}, "proj")
	wantUserErr(t, err, "store_slug_is_page", "untitled")

	// The link checks apply, by the real path.
	_, err = e.publish(t, e.write(t, "home/top.html", "<p>t</p>"), store.Options{Link: true}, "proj")
	wantUserErr(t, err, "link_exposes_dir")
	// A symlink outside home that points into its hidden directory is judged by where it points.
	secret := e.write(t, "home/.secret/key.html", "<p>k</p>")
	mkdir(t, filepath.Join(e.tmp, "pub"))
	if err = os.Symlink(secret, filepath.Join(e.tmp, "pub/key.html")); err != nil {
		t.Fatal(err)
	}
	_, err = e.publish(t, filepath.Join(e.tmp, "pub/key.html"), store.Options{Link: true}, "proj")
	wantUserErr(t, err, "link_hidden_home", secret)
	_, err = e.publish(t, filepath.Join(e.tmp, "missing.html"), store.Options{Link: true}, "proj")
	wantUserErr(t, err, "file_cannot_read")
}

func TestList(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	if list, err := e.s.List(); err != nil || len(list) != 0 {
		t.Fatalf("List of a missing store = %v, %v", list, err)
	}
	for slug, updated := range map[string]string{"older": "2026-01-01T00:00:00.000Z", "newer": "2026-02-01T00:00:00.000Z"} {
		meta := &store.Meta{Title: slug, CreatedAt: updated, UpdatedAt: updated}
		mkdir(t, filepath.Join(e.tmp, "store", slug))
		if err := e.s.WriteMeta(slug, meta); err != nil {
			t.Fatal(err)
		}
	}
	e.write(t, "store/bad-json/meta.json", "{bad")
	e.write(t, "store/bad-time/meta.json", `{"title":"t","updatedAt":12345}`)
	e.write(t, "store/no-time/meta.json", `{"title":"t"}`)
	e.write(t, "store/null/meta.json", "null")
	e.write(t, "store/Bad_Slug/meta.json", `{"title":"t","updatedAt":"2027-01-01T00:00:00.000Z"}`)
	mkdir(t, filepath.Join(e.tmp, "store", "no-meta"))
	file := e.write(t, "site/index.html", "<h1>Linked v1</h1>")
	e.mustPublish(t, file, store.Options{Link: true}, "proj")

	slugs := func() string {
		list, err := e.s.List()
		if err != nil {
			t.Fatal(err)
		}
		s := make([]string, 0, len(list))
		for _, l := range list {
			s = append(s, l.Slug)
		}
		return strings.Join(s, " ")
	}
	if got := slugs(); got != "site newer older" {
		t.Errorf("List = %q, want newest first without the broken entries", got)
	}

	linked := func() store.Listed {
		list, err := e.s.List()
		if err != nil {
			t.Fatal(err)
		}
		return list[0]
	}
	if l := linked(); l.Title != "Linked v1" || l.Missing == nil || *l.Missing || l.Path != file {
		t.Errorf("a link is listed with the file's title: %+v", l)
	}
	// The title is cached per version of the file: a rewrite of the same size is picked up.
	e.write(t, "site/index.html", "<h1>Linked v2</h1>")
	if l := linked(); l.Title != "Linked v2" {
		t.Errorf("a same-size rewrite: title %q", l.Title)
	}
	if err := os.Rename(file, file+".gone"); err != nil {
		t.Fatal(err)
	}
	if l := linked(); l.Missing == nil || !*l.Missing || l.Title != "site" || l.UpdatedAt != e.meta(t, "site").UpdatedAt {
		t.Errorf("a gone file is listed as missing under its slug: %+v", l)
	}
}

func TestWriteMetaFormat(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	mkdir(t, filepath.Join(e.tmp, "store", "x"))
	m := &store.Meta{Title: "<A & B>", Description: "", Favorite: true, CreatedAt: "c", UpdatedAt: "u"}
	if err := e.s.WriteMeta("x", m); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"title\": \"<A & B>\",\n  \"description\": \"\",\n  \"favorite\": true,\n" +
		"  \"createdAt\": \"c\",\n  \"updatedAt\": \"u\"\n}\n"
	if got := e.read(t, "store/x/meta.json"); got != want {
		t.Errorf("meta.json =\n%s\nwant\n%s", got, want)
	}
	if err := e.s.WriteMeta("missing-dir", m); err == nil {
		t.Error("WriteMeta into a missing directory succeeded")
	}
}

func TestEdit(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.mustPublish(t, e.page(t, "My Demo.yaml", "Demo Page"), store.Options{}, "repo")
	e.mustPublish(t, e.page(t, "untitled.yaml", ""), store.Options{}, "repo")
	before := e.meta(t, "my-demo")

	if _, err := e.s.Edit("my-demo", store.Edit{Description: ptr("  edited  ")}); err != nil {
		t.Fatal(err)
	}
	if m := e.meta(t, "my-demo"); m.Description != "edited" || m.UpdatedAt != before.UpdatedAt {
		t.Errorf("Edit description: %+v", m)
	}
	to, err := e.s.Edit("my-demo", store.Edit{Slug: ptr(" renamed "), Title: ptr("T"), Project: ptr("p")})
	if m := e.meta(t, "renamed"); err != nil || to != "renamed" || m.Title != "T" || !m.TitleEdited || m.Project != "p" || !m.ProjectEdited {
		t.Errorf("Edit rename: %q %v %+v", to, err, m)
	}
	if e.s.ReadMeta("my-demo") != nil {
		t.Error("the old slug is left after a rename")
	}
	_, err = e.s.Edit("renamed", store.Edit{Slug: ptr("untitled")})
	if ue := wantUserErr(t, err, "store_slug_taken", "untitled"); !ue.Conflict {
		t.Error("a slug in use is not a conflict")
	}
	_, err = e.s.Edit("renamed", store.Edit{Slug: ptr("Bad Slug")})
	wantUserErr(t, err, "store_invalid_slug", "Bad Slug")
	_, err = e.s.Edit("renamed", store.Edit{Project: ptr(" / ")})
	wantUserErr(t, err, "store_project_slash")
	if _, err := e.s.Edit("no-such", store.Edit{}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Edit of an unknown slug = %v", err)
	}

	if _, err := e.s.Edit("renamed", store.Edit{Title: ptr(""), Project: ptr(""), Favorite: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	raw := e.read(t, "store/renamed/meta.json")
	if m := e.meta(t, "renamed"); m.Title != "Demo Page" || strings.Contains(raw, "project") || strings.Contains(raw, "favorite") ||
		strings.Contains(raw, "Edited") {
		t.Errorf("clearing restores the page's title and drops the keys:\n%s", raw)
	}

	// A drawn page's <title> is the title it was published under, so clearing a pinned title goes back
	// to the data file's title:, or to the slug (a title of spaces names none).
	for _, tt := range []struct{ file, title, want string }{
		{"pinned.yaml", "title: Data Title\n", "Data Title"},
		{"bare.yaml", "", "bare"},
		{"spaces.yaml", "title: '  '\n", "spaces"},
	} {
		slug := strings.TrimSuffix(tt.file, ".yaml")
		file := e.write(t, tt.file, "template: report\nlang: en\n"+tt.title)
		e.mustPublish(t, file, store.Options{}, "repo")
		if _, err := e.s.Edit(slug, store.Edit{Title: ptr("Pinned")}); err != nil {
			t.Fatal(err)
		}
		e.mustPublish(t, file, store.Options{}, "repo")
		if _, err := e.s.Edit(slug, store.Edit{Title: ptr(" ")}); err != nil || e.meta(t, slug).Title != tt.want {
			t.Errorf("clearing the pinned title of %s: %v %+v", tt.file, err, e.meta(t, slug))
		}
	}
	// Clearing the title while renaming falls back to the new slug.
	if _, err := e.s.Edit("bare", store.Edit{Title: ptr(""), Slug: ptr("bare-2")}); err != nil || e.meta(t, "bare-2").Title != "bare-2" {
		t.Errorf("clearing the title while renaming: %v %+v", err, e.meta(t, "bare-2"))
	}

	// A page stored as HTML before pages were drawn takes its own <title> back, or the slug.
	old := store.Meta{Title: "Pinned", TitleEdited: true, CreatedAt: before.CreatedAt, UpdatedAt: before.UpdatedAt}
	e.write(t, "store/old/index.html", "<title>Old</title>")
	if err := e.s.WriteMeta("old", &old); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Edit("old", store.Edit{Title: ptr("")}); err != nil || e.meta(t, "old").Title != "Old" {
		t.Errorf("clearing the title of an HTML page: %v %+v", err, e.meta(t, "old"))
	}
	if err := os.Remove(filepath.Join(e.tmp, "store/old/index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Edit("old", store.Edit{Title: ptr(" "), Slug: ptr("older")}); err != nil || e.meta(t, "older").Title != "older" {
		t.Errorf("clearing the title of an unreadable page falls back to the new slug: %v %+v", err, e.meta(t, "older"))
	}

	file := e.write(t, "site/index.html", "<h1>L</h1>")
	e.mustPublish(t, file, store.Options{Link: true}, "proj")
	if _, err := e.s.Edit("site", store.Edit{Title: ptr("Pinned")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Edit("site", store.Edit{Title: ptr("")}); err != nil || e.meta(t, "site").Title != "" {
		t.Errorf("clearing a link's title hands it back to the file: %+v", e.meta(t, "site"))
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.write(t, "site/index.html", "<h1>L</h1>")
	e.mustPublish(t, file, store.Options{Link: true}, "proj")
	if err := e.s.Remove("site"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("rm of a link removed the file itself: %v", err)
	}
	wantUserErr(t, e.s.Remove("site"), "store_no_page", "site")
	wantUserErr(t, e.s.Remove("../x"), "store_no_page", "../x")
}

func TestFileAndVersion(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	stored := &store.Meta{UpdatedAt: "2026-01-01T00:00:00.000Z"}
	if f, ok := e.s.File("p", stored, ""); !ok || f != filepath.Join(e.tmp, "store", "p", "index.html") {
		t.Errorf("File of a stored page = %q, %v", f, ok)
	}
	if _, ok := e.s.File("p", stored, "x.css"); ok {
		t.Error("a stored page serves a subpath")
	}
	if v := e.s.Version("p", stored, ""); v != stored.UpdatedAt {
		t.Errorf("Version of a stored page = %q", v)
	}
	file := e.write(t, "site/index.html", "x")
	mtime := time.Date(2026, 3, 4, 5, 6, 7, 890_123_456, time.UTC)
	if err := os.Chtimes(file, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	linked := &store.Meta{Path: file}
	if v := e.s.Version("p", linked, file); v != "2026-03-04T05:06:07.890Z" {
		t.Errorf("Version of a link = %q", v)
	}
	if v := e.s.Version("p", linked, file+".gone"); v != "" {
		t.Errorf("Version of a gone link = %q", v)
	}
	if store.Now() == "" || !store.ValidSlug("a-1") || store.ValidSlug("-a") || e.s.Root() != filepath.Join(e.tmp, "store") {
		t.Error("Now, ValidSlug or Root")
	}
}

func TestListedJSON(t *testing.T) {
	t.Parallel()

	missing := false
	b, err := json.Marshal(store.Listed{Slug: "s", Meta: store.Meta{Path: "/p", UpdatedAt: "u"}, Missing: &missing})
	if err != nil || string(b) != `{"slug":"s","path":"/p","description":"","createdAt":"","updatedAt":"u","missing":false}` {
		t.Errorf("Listed JSON = %s, %v", b, err)
	}
}

// A page is in the language of its data's lang:, which meta.json does not keep; data without one is
// refused.
func TestPublishLang(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	frag := e.page(t, "frag.yaml", "")
	ja := e.write(t, "ja.yaml", "template: report\nlang: ja\n")
	e.mustPublish(t, ja, store.Options{}, "repo")
	if page, meta := e.read(t, "store/ja/index.html"), e.read(t, "store/ja/meta.json"); !strings.Contains(page, `<html lang="ja">`) ||
		strings.Contains(meta, `"lang"`) {
		t.Errorf("a page of lang: ja =\n%s\nmeta.json =\n%s", page, meta)
	}
	none := e.write(t, "none.yaml", "template: report\n")
	_, err := e.publish(t, none, store.Options{}, "repo")
	if err == nil || !strings.Contains(err.Error(), "none.yaml:1:1: name the language of the page") {
		t.Errorf("publish of data without lang: = %v", err)
	}

	// --theme picks a theme of the template over the data's, which is then not used, and is refused
	// for a link.
	printed := e.write(t, "printed.yaml", "template: report\ntheme: print\nlang: en\n")
	_, err = e.publish(t, printed, store.Options{}, "other")
	if err == nil || !strings.Contains(err.Error(), `template report has no theme "print"`) {
		t.Errorf("publish of a theme the template does not have = %v", err)
	}
	e.mustPublish(t, printed, store.Options{Theme: ptr("default")}, "other")
	if m := e.meta(t, "printed"); m.Theme != "default" {
		t.Errorf("--theme default over theme: print: theme %q", m.Theme)
	}
	listed := e.write(t, "listed.yaml", "template: report\ntheme: [dark]\nlang: en\n")
	e.mustPublish(t, listed, store.Options{Theme: ptr("default")}, "other")
	_, err = e.publish(t, frag, store.Options{Theme: ptr("print")}, "other")
	wantUserErr(t, err, "tmpl_unknown_theme", "print", "report", "default")

	// A linked file is shown as it is: --theme is refused.
	site := e.write(t, "site/index.html", "<h1>A</h1>")
	_, err = e.publish(t, site, store.Options{Link: true, Theme: ptr("default")}, "repo")
	wantUserErr(t, err, "store_theme_link")
}

// A data file is drawn with the template it names, titled by its title: unless --title says
// otherwise, and the page records the template and theme.
func TestPublishDataFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.write(t, "plan.yaml", "template: report\ntitle: The plan\nsections:\n  - heading: Steps\n    parts:\n      - text: Do **it**.\nlang: ja\n")
	entry := e.mustPublish(t, file, store.Options{}, "")
	m := entry.Meta
	if entry.Slug != "plan" || m.Title != "The plan" || m.Template != "report" || m.Theme != "default" {
		t.Errorf("meta = %+v (slug %s)", m, entry.Slug)
	}
	b, err := os.ReadFile(filepath.Join(e.tmp, "store", "plan", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<html lang="ja">`, "<h1>The plan</h1>", "<p>Do <strong>it</strong>.</p>"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the page lacks %s:\n%s", want, b)
		}
	}
	if entry := e.mustPublish(t, file, store.Options{Title: ptr("Pinned")}, ""); !strings.Contains(string(entry.Page), "<h1>Pinned</h1>") {
		t.Errorf("--title does not title the drawn page: %s", entry.Page)
	}
	bad := e.write(t, "bad.yml", "template: deck\nlang: en\n")
	if _, err := e.publish(t, bad, store.Options{}, ""); err == nil || err.Error() != bad+`:1:11: unknown template "deck"; use report` {
		t.Errorf("publish of a data file naming no template there = %v", err)
	}
}

// A data file is linked as an HTML file is, checked as a stored page is, with the theme given over its
// own: no page is stored, as it is drawn each time it is opened, in its lang:.
func TestLinkDataFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	data := e.write(t, "proj/spec.yaml", "template: report\ntitle: Spec\nlang: ja\n")
	entry := e.mustPublish(t, data, store.Options{Link: true, Theme: ptr("default")}, "proj")
	m := entry.Meta
	if entry.Page != nil || m.Path != data || m.Theme != "default" || m.Template != "" || m.Title != "" ||
		m.StrictCSP {
		t.Errorf("the entry = %+v, page %d bytes", m, len(entry.Page))
	}
	if !m.IsDrawn() || e.s.Version("spec", &m, data) == "" {
		t.Errorf("IsDrawn = %v, Version = %q", m.IsDrawn(), e.s.Version("spec", &m, data))
	}
	if _, err := os.Stat(filepath.Join(e.tmp, "store", "spec", "index.html")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a page was stored for a link: %v", err)
	}
	page, _, err := e.s.Draw("spec", &m, data)
	if err != nil || !strings.Contains(string(page), `<html lang="ja">`) || !strings.Contains(string(page), "<h1>Spec</h1>") {
		t.Errorf("Draw = %v:\n%s", err, page)
	}

	// Linked again in another theme, it reloads its tab.
	again := e.mustPublish(t, data, store.Options{Link: true}, "proj").Meta
	if e.s.Version("spec", &again, data) == e.s.Version("spec", &m, data) {
		t.Error("linking again in another theme left the version as it was")
	}
	// A slug that comes to link another file watches nothing of the one before.
	other := e.write(t, "proj/other.yaml", "template: report\nlang: en\n")
	if _, version, drawErr := e.s.Draw("spec", &m, other); drawErr != nil || version != e.s.Version("spec", &m, other) {
		t.Errorf("Draw of another file under the slug = %v at %q, watched at %q", drawErr, version, e.s.Version("spec", &m, other))
	}

	bad := e.write(t, "proj/bad.yaml", "template: report\nsections: 3\nlang: en\n")
	if _, err = e.publish(t, bad, store.Options{Link: true}, "proj"); err == nil || !strings.Contains(err.Error(), "bad.yaml:2:11: sections:") {
		t.Errorf("linking a data file with mistakes = %v", err)
	}
}

// A linked data file that cannot be read is served at the version that watching it sees, so that its
// tab does not reload over and over; and one linked through a symlink is of the kind of its real file.
func TestLinkDataFileEdges(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	data := e.write(t, "proj/spec.yaml", "template: report\nlang: en\n")
	m := e.mustPublish(t, data, store.Options{Link: true}, "proj").Meta
	if err := os.Chmod(data, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(data, 0o600); err != nil {
			t.Error(err)
		}
	})
	_, version, err := e.s.Draw("spec", &m, data)
	if !errors.Is(err, store.ErrNotFound) || version != e.s.Version("spec", &m, data) {
		t.Errorf("Draw of a file that cannot be read = %v at %q, watched at %q", err, version, e.s.Version("spec", &m, data))
	}
	// Made readable again, it reloads its tab.
	if err = os.Chmod(data, 0o600); err != nil {
		t.Fatal(err)
	}
	if again := e.s.Version("spec", &m, data); again == version {
		t.Errorf("the version of the file made readable again stays %q", again)
	}

	// A symlink named .html to a data file takes what a data file does.
	if err = os.Symlink(data, filepath.Join(e.tmp, "proj", "spec.html")); err != nil {
		t.Fatal(err)
	}
	if _, err = e.publish(t, filepath.Join(e.tmp, "proj", "spec.html"), store.Options{Link: true, Theme: ptr("default")}, "proj"); err != nil {
		t.Errorf("linking a data file through a symlink named .html, with --theme = %v", err)
	}

	bad := e.write(t, "proj/bad.yaml", "template: nosuch\nlang: en\n")
	if err = os.Symlink(bad, filepath.Join(e.tmp, "proj", "view.html")); err != nil {
		t.Fatal(err)
	}
	if _, err = e.publish(t, filepath.Join(e.tmp, "proj", "view.html"), store.Options{Link: true}, "proj"); err == nil ||
		!strings.Contains(err.Error(), `unknown template "nosuch"`) {
		t.Errorf("linking a data file with mistakes through a symlink named .html = %v", err)
	}
}

// One data file linked under two slugs, drawn with the templates of their publishes, is watched apart:
// drawing one does not move the version of the other.
func TestLinkDataFileTwice(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	dirs := map[string]string{"a": filepath.Join(e.tmp, "a"), "b": filepath.Join(e.tmp, "b")}
	for _, dir := range dirs {
		if err := os.CopyFS(filepath.Join(dir, "sizes"), os.DirFS("../tmpl/testdata/sizes")); err != nil {
			t.Fatal(err)
		}
	}
	e.write(t, "b/sizes/extra.css", "")
	data := e.write(t, "proj/sizes.yaml", "template: sizes\nfiles: [{name: a, bytes: 1}]\nlang: en\n")
	metas := map[string]store.Meta{}
	for slug, dir := range dirs {
		s := store.New(filepath.Join(e.tmp, "store"), filepath.Join(e.tmp, "home"), tmpl.NewLibrary(dir, ""))
		entry, err := s.Prepare(t.Context(), data, store.Options{Link: true, Slug: ptr(slug)}, filepath.Join(e.tmp, "proj"))
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Save(entry); err != nil {
			t.Fatal(err)
		}
		if entry.Meta.Templates != dir {
			t.Errorf("the templates of %s = %q, want %q", slug, entry.Meta.Templates, dir)
		}
		metas[slug] = entry.Meta
	}
	a, b := metas["a"], metas["b"]
	_, version, err := e.s.Draw("a", &a, data)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.s.Draw("b", &b, data); err != nil {
		t.Fatal(err)
	}
	if now := e.s.Version("a", &a, data); now != version {
		t.Errorf("drawing b moved the version of a: %q, then %q", version, now)
	}

	// Linked again with the other templates, a is served at the version it is watched at, which waits
	// for the drawing: the tab reloads once, not twice.
	time.Sleep(5 * time.Millisecond) // another updatedAt, kept to the millisecond
	s := store.New(filepath.Join(e.tmp, "store"), filepath.Join(e.tmp, "home"), tmpl.NewLibrary(dirs["b"], ""))
	entry, err := s.Prepare(t.Context(), data, store.Options{Link: true, Slug: ptr("a")}, filepath.Join(e.tmp, "proj"))
	if err != nil {
		t.Fatal(err)
	}
	relinked := entry.Meta
	if _, version, err = e.s.Draw("a", &relinked, data); err != nil || version != e.s.Version("a", &relinked, data) {
		t.Errorf("a linked again was served at %q, watched at %q (%v)", version, e.s.Version("a", &relinked, data), err)
	}
}

// A linked data file whose diagrams show icons not fetched yet is drawn again once they are: the fetch
// its mistake asks for reloads its tab.
func TestLinkDataFileIcons(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	icons := filepath.Join(e.tmp, "icons")
	s := store.New(filepath.Join(e.tmp, "store"), filepath.Join(e.tmp, "home"), tmpl.NewLibrary("", icons))
	data := e.write(t, "proj/spec.yaml", "template: report\nsections:\n  - parts:\n      - diagram:\n          id: a\n"+
		"          width: 100\n          height: 50\n          title: t\n          parts:\n"+
		"            - icon: { x: 10, y: 5, name: aws/aws-lambda, size: 24 }\nlang: en\n")
	m := store.Meta{Path: data}
	_, version, err := s.Draw("spec", &m, data)
	if err == nil || !strings.Contains(err.Error(), "floe icons fetch aws") {
		t.Fatalf("Draw without the icons = %v", err)
	}
	if v := s.Version("spec", &m, data); v != version {
		t.Fatalf("the version moved from %q to %q with nothing changed", version, v)
	}
	if err = os.CopyFS(icons, os.DirFS("../diagram/testdata/icons")); err != nil {
		t.Fatal(err)
	}
	if v := s.Version("spec", &m, data); v == version {
		t.Errorf("the version stays %q once the icons are fetched", v)
	}
	if page, _, err := s.Draw("spec", &m, data); err != nil || !strings.Contains(string(page), `<use href="#a_icon-`) {
		t.Errorf("Draw with the icons = %v", err)
	}
}

// WriteAtomic gives the file the mode asked less the umask, as os.WriteFile does, writing over the file
// there and leaving nothing beside it.
func TestWriteAtomic(t *testing.T) { //nolint:paralleltest // sets the umask, which is the process's
	defer unix.Umask(unix.Umask(0o022))
	dir := t.TempDir()
	path := filepath.Join(dir, "page.html")
	for _, c := range []struct {
		umask int
		want  os.FileMode
	}{{0o022, 0o644}, {0o077, 0o600}} {
		unix.Umask(c.umask)
		if err := store.WriteAtomic(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(path)
		entries, _ := os.ReadDir(dir) //nolint:errcheck // counted below
		if err != nil || fi.Mode().Perm() != c.want || len(entries) != 1 {
			t.Errorf("under umask %o: %v, %v, %d files; want %v", c.umask, fi.Mode(), err, len(entries), c.want)
		}
	}
}
