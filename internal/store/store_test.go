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

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
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
	return env{tmp: tmp, s: store.New(filepath.Join(tmp, "store"), filepath.Join(tmp, "home"))}
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

	src := "template: report\n"
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
	if got := e.read(t, "store/my-demo/index.html"); !strings.Contains(got, "<h1>Demo Page</h1>") || m.Template != "report" {
		t.Errorf("the page is stored as %q (template %q), want it drawn by report", got, m.Template)
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
		e.write(t, filepath.Join(tt.cwd, tt.file), "template: report\n")
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
		file := e.write(t, tt.file, "template: report\n"+tt.title)
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
	if v := store.Version(stored, ""); v != stored.UpdatedAt {
		t.Errorf("Version of a stored page = %q", v)
	}
	file := e.write(t, "site/index.html", "x")
	mtime := time.Date(2026, 3, 4, 5, 6, 7, 890_123_456, time.UTC)
	if err := os.Chtimes(file, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	linked := &store.Meta{Path: file}
	if v := store.Version(linked, file); v != "2026-03-04T05:06:07.890Z" {
		t.Errorf("Version of a link = %q", v)
	}
	if v := store.Version(linked, file+".gone"); v != "" {
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

func TestPublishLang(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	frag := e.page(t, "frag.yaml", "")
	e.mustPublish(t, frag, store.Options{}, "repo")
	if m := e.meta(t, "frag"); m.Lang != "" || !strings.Contains(e.read(t, "store/frag/index.html"), `<html lang="en">`) {
		t.Errorf("a page without --lang: lang %q", m.Lang)
	}
	e.mustPublish(t, frag, store.Options{Lang: ptr("ja")}, "repo")
	e.mustPublish(t, frag, store.Options{}, "repo")
	if m := e.meta(t, "frag"); m.Lang != "ja" || !strings.Contains(e.read(t, "store/frag/index.html"), `<html lang="ja">`) {
		t.Errorf("a republish without --lang should keep ja: lang %q", m.Lang)
	}
	e.mustPublish(t, frag, store.Options{Lang: ptr("pt-BR")}, "repo")
	if m := e.meta(t, "frag"); m.Lang != "pt-BR" {
		t.Errorf("--lang pt-BR: lang %q", m.Lang)
	}
	e.mustPublish(t, frag, store.Options{Force: true}, "other")
	if m := e.meta(t, "frag"); m.Lang != "" {
		t.Errorf("--force from another project should start afresh: lang %q", m.Lang)
	}
	for _, bad := range []string{"", "x", "ja_JP", "en-", `ja"><script>`} {
		_, err := e.publish(t, frag, store.Options{Lang: ptr(bad)}, "other")
		wantUserErr(t, err, "lang_invalid", bad)
	}

	// A linked file is shown as it is: --lang is refused, and a language left from before is dropped.
	site := e.write(t, "site/index.html", "<h1>A</h1>")
	_, err := e.publish(t, site, store.Options{Link: true, Lang: ptr("ja")}, "repo")
	wantUserErr(t, err, "store_lang_link")
	e.mustPublish(t, site, store.Options{Link: true}, "repo")
	m := e.meta(t, "site")
	m.Lang = "ja"
	if err := e.s.WriteMeta("site", m); err != nil {
		t.Fatal(err)
	}
	e.mustPublish(t, site, store.Options{Link: true}, "repo")
	if m := e.meta(t, "site"); m.Lang != "" {
		t.Errorf("a relinked file keeps lang %q", m.Lang)
	}
}

// A data file is drawn with the template it names, titled by its title: unless --title says
// otherwise, and the page records the template and theme.
func TestPublishDataFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.write(t, "plan.yaml", "template: report\ntitle: The plan\nsections:\n  - heading: Steps\n    parts:\n      - text: Do **it**.\n")
	entry := e.mustPublish(t, file, store.Options{Lang: ptr("ja")}, "")
	m := entry.Meta
	if entry.Slug != "plan" || m.Title != "The plan" || m.Template != "report" || m.Theme != "default" || m.Lang != "ja" {
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
	bad := e.write(t, "bad.yml", "template: deck\n")
	if _, err := e.publish(t, bad, store.Options{}, ""); err == nil || err.Error() != bad+`: unknown template "deck"; use report` {
		t.Errorf("publish of a data file naming no template there = %v", err)
	}
}
