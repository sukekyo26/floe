package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
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
	return env{tmp: tmp, s: store.New(filepath.Join(tmp, "store"), filepath.Join(tmp, "home"), tmpl.NewLibrary("", "", "", ""))}
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

// publish prepares and saves the data file file from cwd (a directory under tmp, created when missing).
func (e env) publish(t *testing.T, file string, opts store.Options, cwd string) (*store.Entry, error) {
	t.Helper()

	return e.save(t, func(ctx context.Context, file string, opts store.Options, cwd string) (*store.Entry, error) {
		return prepare(ctx, e.s, file, opts, cwd)
	}, file, opts, cwd)
}

// prepare publishes the data file file on disk from cwd as the CLI does, but in process: read by
// store.ReadUpload, its images read under its directory.
func prepare(ctx context.Context, st *store.Store, file string, opts store.Options, cwd string) (*store.Entry, error) {
	up, err := store.ReadUpload(ctx, file, cwd)
	if err != nil {
		return nil, err
	}
	up.Images = os.DirFS(up.Dir)
	return st.PrepareData(up, opts)
}

// link prepares and saves a link to file from cwd, as publish does.
func (e env) link(t *testing.T, file string, opts store.Options, cwd string) (*store.Entry, error) {
	t.Helper()

	return e.save(t, e.s.PrepareLink, file, opts, cwd)
}

func (e env) save(t *testing.T, prepareWith func(context.Context, string, store.Options, string) (*store.Entry, error),
	file string, opts store.Options, cwd string,
) (*store.Entry, error) {
	t.Helper()

	dir := filepath.Join(e.tmp, cwd)
	mkdir(t, dir)
	entry, err := prepareWith(t.Context(), file, opts, dir)
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

func (e env) mustLink(t *testing.T, file string, opts store.Options, cwd string) *store.Entry {
	t.Helper()

	entry, err := e.link(t, file, opts, cwd)
	if err != nil {
		t.Fatalf("link %s: %v", file, err)
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

// kept reads the copy of the data file of the stored page slug.
func (e env) kept(t *testing.T, slug string) string {
	t.Helper()

	return e.read(t, filepath.Join("store", slug, e.meta(t, slug).Source, "data.yaml"))
}

// drawn draws the stored page slug as it is served.
func (e env) drawn(t *testing.T, slug string) string {
	t.Helper()

	m := e.meta(t, slug)
	file, ok := e.s.File(slug, m, "")
	if !ok {
		t.Fatalf("no file to draw for %s", slug)
	}
	page, _, err := e.s.Draw(slug, m, file)
	if err != nil {
		t.Fatalf("draw %s: %v", slug, err)
	}
	return string(page)
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

	// A stage that collects its mistakes returns one alone in a List.
	if l, ok := errors.AsType[usererr.List](err); ok && len(l) == 1 {
		err = l[0]
	}
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
	if got := e.drawn(t, "my-demo"); !strings.Contains(got, "<h1>Demo Page</h1>") || !m.IsDrawn() ||
		e.kept(t, "my-demo") != e.read(t, "My Demo.yaml") {
		t.Errorf("the page is drawn as %q from %q, want the data file kept and drawn by report", got, e.kept(t, "my-demo"))
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
	if m := e.meta(t, "untitled"); m.Title != "untitled" || !strings.Contains(e.drawn(t, "untitled"), "<h1>untitled</h1>") {
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
		publish := e.mustLink
		if store.IsDataFile(tt.file) {
			publish = e.mustPublish
		}
		entry := publish(t, tt.file, store.Options{}, tt.cwd)
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
	entry := e.mustLink(t, file, store.Options{Slug: ptr("site"), Description: ptr("d")}, "elsewhere")
	m := e.meta(t, "site")
	if entry.Source != nil || m.Path != file || m.Project != "site" || m.PublishedFrom != "" || m.Title != "" {
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
	shown := e.mustLink(t, filepath.Join(e.tmp, "pub/shown.html"), store.Options{}, "proj")
	if shown.Slug != "shown" || shown.Meta.Path != filepath.Join(e.tmp, "real/page.html") {
		t.Errorf("a symlinked file: %s %s", shown.Slug, shown.Meta.Path)
	}

	// The same slug for another path is refused unless --force, which keeps nothing of the old link.
	if _, err := e.s.Edit("site", store.Edit{Favorite: ptr(true), Title: ptr("Pinned")}); err != nil {
		t.Fatal(err)
	}
	other := e.write(t, "site/sub/p.html", "<title>Sub</title>")
	_, err := e.link(t, other, store.Options{Slug: ptr("site")}, "proj")
	if ue := wantUserErr(t, err, "store_slug_linked", "site", file); !ue.Conflict {
		t.Errorf("the refusal is not a conflict for the management page: %+v", ue)
	}
	again := e.mustLink(t, file, store.Options{Slug: ptr("site")}, "proj")
	if m := e.meta(t, "site"); again.Prev == nil || m.Title != "Pinned" || !m.TitleEdited || !m.Favorite || m.Description != "d" {
		t.Errorf("re-registering the same path should keep the pinned title and the rest: %+v", m)
	}
	created := e.meta(t, "site").CreatedAt
	e.mustLink(t, other, store.Options{Slug: ptr("site"), Force: true}, "proj")
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
	if got := e.mustLink(t, site, store.Options{}, "proj").Slug; got != "site" {
		t.Fatalf("slug %q, want site", got)
	}
	if got := e.mustLink(t, other, store.Options{}, "proj").Slug; got != "site-2" {
		t.Errorf("an omitted slug another file uses: %q, want site-2", got)
	}
	if err := e.s.Remove("site-2"); err != nil {
		t.Fatal(err)
	}
	e.mustLink(t, other, store.Options{Slug: ptr("by-hand")}, "proj")
	if got := e.mustLink(t, other, store.Options{}, "proj").Slug; got != "by-hand" {
		t.Errorf("an omitted slug should reuse the entry of the same path: %q", got)
	}
	long := strings.Repeat("b", 63)
	e.mustLink(t, e.write(t, "l1/"+long+"/index.html", "1"), store.Options{}, "proj")
	if got := e.mustLink(t, e.write(t, "l2/"+long+"/index.html", "2"), store.Options{}, "proj").Slug; got != strings.Repeat("b", 62)+"-2" {
		t.Errorf("a numbered slug is cut to 64 characters: %q", got)
	}

	// A slug cannot switch between a stored page and a link.
	_, err := e.publish(t, e.page(t, "untitled.yaml", ""), store.Options{Slug: ptr("site")}, "proj")
	wantUserErr(t, err, "store_slug_is_link", "site")
	e.mustPublish(t, filepath.Join(e.tmp, "untitled.yaml"), store.Options{}, "proj")
	_, err = e.link(t, site, store.Options{Slug: ptr("untitled")}, "proj")
	wantUserErr(t, err, "store_slug_is_page", "untitled")

	// The link checks apply, by the real path.
	_, err = e.link(t, e.write(t, "home/top.html", "<p>t</p>"), store.Options{}, "proj")
	wantUserErr(t, err, "link_exposes_dir")
	// A symlink outside home that points into its hidden directory is judged by where it points.
	secret := e.write(t, "home/.secret/key.html", "<p>k</p>")
	mkdir(t, filepath.Join(e.tmp, "pub"))
	if err = os.Symlink(secret, filepath.Join(e.tmp, "pub/key.html")); err != nil {
		t.Fatal(err)
	}
	_, err = e.link(t, filepath.Join(e.tmp, "pub/key.html"), store.Options{}, "proj")
	wantUserErr(t, err, "link_hidden_home", secret)
	_, err = e.link(t, filepath.Join(e.tmp, "missing.html"), store.Options{}, "proj")
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
	e.mustLink(t, file, store.Options{}, "proj")

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

	file := e.write(t, "site/index.html", "<h1>L</h1>")
	e.mustLink(t, file, store.Options{}, "proj")
	if _, err := e.s.Edit("site", store.Edit{Title: ptr("Pinned")}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Edit("site", store.Edit{Title: ptr("")}); err != nil || e.meta(t, "site").Title != "" {
		t.Errorf("clearing a link's title hands it back to the file: %+v", e.meta(t, "site"))
	}
}

// Saves and edits of one slug at once take turns: each succeeds, and the store holds the page whole,
// meta.json naming the copy that is there.
func TestSaveAtOnce(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	doc := e.page(t, "race.yaml", "Race")
	mkdir(t, filepath.Join(e.tmp, "proj"))
	e.mustPublish(t, doc, store.Options{}, "proj")
	errs := make([]error, 16)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() {
			if i%2 == 1 {
				_, errs[i] = e.s.Edit("race", store.Edit{Description: ptr("d")})
				return
			}
			entry, err := prepare(t.Context(), e.s, doc, store.Options{}, filepath.Join(e.tmp, "proj"))
			if err == nil {
				err = e.s.Save(entry)
			}
			errs[i] = err
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatalf("saves at once: %v", err)
	}
	if page := e.drawn(t, "race"); !strings.Contains(page, "<h1>Race</h1>") {
		t.Errorf("the page after saves at once:\n%s", page)
	}
}

// A save whose write fails leaves the page it was to replace drawn as it was.
func TestSaveFailureKeepsPage(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("root writes into a directory it cannot write")
	}
	e := newEnv(t)
	doc := e.page(t, "kept.yaml", "Kept")
	e.mustPublish(t, doc, store.Options{}, "proj")
	source := filepath.Join(e.tmp, "store", "kept")
	if err := os.Chmod(source, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(source, 0o700); err != nil {
			t.Error(err)
		}
	})
	entry, err := prepare(t.Context(), e.s, e.page(t, "kept.yaml", "Changed"), store.Options{}, filepath.Join(e.tmp, "proj"))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.Save(entry); err == nil {
		t.Fatal("a save into a directory it cannot write succeeded")
	}
	if page := e.drawn(t, "kept"); !strings.Contains(page, "<h1>Kept</h1>") {
		t.Errorf("the page after a failed save:\n%s", page)
	}
}

// A save, an edit and a removal wait while the store is locked, as another of them holds it.
func TestSaveEditRemoveWait(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	doc := e.page(t, "wait.yaml", "Wait")
	e.mustPublish(t, doc, store.Options{}, "proj")
	entry, err := prepare(t.Context(), e.s, doc, store.Options{}, filepath.Join(e.tmp, "proj"))
	if err != nil {
		t.Fatal(err)
	}
	for _, act := range []func() error{
		func() error { return e.s.Save(entry) },
		func() error {
			_, err := e.s.Edit("wait", store.Edit{Description: ptr("d")})
			return err
		},
		func() error { return e.s.Remove("wait") },
	} {
		f, err := os.Open(filepath.Join(e.tmp, "store", ".lock"))
		if err != nil {
			t.Fatal(err)
		}
		if err = unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil { //nolint:gosec // G115: a descriptor fits an int
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- act() }()
		select {
		case err = <-done:
			t.Fatalf("it did not wait for the lock: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
		if err = <-done; err != nil {
			t.Fatal(err)
		}
	}
}

// Save checks a publish again against its slug as it is then. An edit made after Prepare stays, and
// the publish is refused when, after Prepare, another project published the slug or a page was
// renamed onto it.
func TestSaveChecksAgain(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	prepareIn := func(file, cwd string) *store.Entry {
		t.Helper()
		mkdir(t, filepath.Join(e.tmp, cwd))
		entry, err := prepare(t.Context(), e.s, file, store.Options{}, filepath.Join(e.tmp, cwd))
		if err != nil {
			t.Fatal(err)
		}
		return entry
	}

	doc := e.page(t, "kept.yaml", "Kept")
	e.mustPublish(t, doc, store.Options{}, "proj")
	entry := prepareIn(doc, "proj")
	if _, err := e.s.Edit("kept", store.Edit{Description: ptr("set meanwhile")}); err != nil {
		t.Fatal(err)
	}
	if err := e.s.Save(entry); err != nil || e.meta(t, "kept").Description != "set meanwhile" || entry.Prev == nil {
		t.Errorf("a save after an edit: %v, %+v", err, e.meta(t, "kept"))
	}

	race := e.page(t, "race.yaml", "Race")
	first, second := prepareIn(race, "one"), prepareIn(race, "two")
	if err := e.s.Save(first); err != nil {
		t.Fatal(err)
	}
	wantUserErr(t, e.s.Save(second), "store_slug_owned", "race", "one")

	e.mustPublish(t, e.page(t, "a.yaml", "A"), store.Options{}, "one")
	onto := prepareIn(e.page(t, "b.yaml", "B"), "two")
	if _, err := e.s.Edit("a", store.Edit{Slug: ptr("b")}); err != nil {
		t.Fatal(err)
	}
	wantUserErr(t, e.s.Save(onto), "store_slug_owned", "b", "one")
	if page := e.drawn(t, "b"); !strings.Contains(page, "<h1>A</h1>") {
		t.Errorf("the renamed page after a refused save:\n%s", page)
	}
}

// A store whose path holds what a glob takes for a pattern keeps one copy of a page all the same.
func TestStoreUnderPatternPath(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	root := filepath.Join(e.tmp, "[notes]", "store")
	st := store.New(root, filepath.Join(e.tmp, "home"), tmpl.NewLibrary("", "", "", ""))
	doc := e.page(t, "plan.yaml", "Plan")
	for range 2 {
		entry, err := prepare(t.Context(), st, doc, store.Options{}, e.tmp)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.Save(entry); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "plan"))
	if err != nil || len(entries) != 2 {
		t.Errorf("the page holds %v (%v), want one copy and meta.json", entries, err)
	}
}

// An edit or a removal in a store not made yet finds no page, and makes nothing.
func TestNoStore(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	root := filepath.Join(e.tmp, "none")
	st := store.New(root, filepath.Join(e.tmp, "home"), tmpl.NewLibrary("", "", "", ""))
	wantUserErr(t, st.Remove("x"), "store_no_page", "x")
	if _, err := st.Edit("x", store.Edit{Description: ptr("d")}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("an edit in no store = %v", err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a lookup made the store: %v", err)
	}
}

// A save that fails leaves no directory it made, which would keep a rename off the slug, and leaves
// one that was there, its meta.json unreadable.
func TestFailedSave(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	failing := func(file string) *store.Entry {
		t.Helper()
		entry, err := prepare(t.Context(), e.s, file, store.Options{}, e.tmp)
		if err != nil {
			t.Fatal(err)
		}
		// No file is named with a NUL.
		entry.Images = map[string][]byte{"bad\x00.png": {1}}
		return entry
	}
	if err := e.s.Save(failing(e.page(t, "fresh.yaml", "Fresh"))); err == nil {
		t.Fatal("a save of an image named with a NUL succeeded")
	}
	if _, err := os.Stat(filepath.Join(e.tmp, "store", "fresh")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed save of a new slug left its directory: %v", err)
	}

	e.mustPublish(t, e.page(t, "kept.yaml", "Kept"), store.Options{}, "")
	e.write(t, "store/kept/meta.json", "{")
	if err := e.s.Save(failing(e.page(t, "kept.yaml", "Kept"))); err == nil {
		t.Fatal("a save of an image named with a NUL succeeded")
	}
	if kept, err := filepath.Glob(filepath.Join(e.tmp, "store", "kept", "source-*", "data.yaml")); err != nil || len(kept) != 1 {
		t.Errorf("a failed save over an unreadable meta.json removed the copy: %v %v", kept, err)
	}
}

// A store made by a floe that took no lock is locked all the same: an edit finds its pages.
func TestStoreWithoutLockFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.mustPublish(t, e.page(t, "old.yaml", "Old"), store.Options{}, "proj")
	if err := os.Remove(filepath.Join(e.tmp, "store", ".lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Edit("old", store.Edit{Description: ptr("d")}); err != nil || e.meta(t, "old").Description != "d" {
		t.Errorf("an edit in a store without a lock file = %v", err)
	}
}

// A link without --slug takes its slug when it is saved: two linked at once take a slug each, and one
// whose entry was renamed since Prepare keeps the new slug.
func TestLinkSlugPickedOnSave(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	prepareIn := func(file string) *store.Entry {
		t.Helper()
		entry, err := e.s.PrepareLink(t.Context(), file, store.Options{}, e.tmp)
		if err != nil {
			t.Fatal(err)
		}
		return entry
	}
	one, two := prepareIn(e.write(t, "a/site/index.html", "<h1>A</h1>")), prepareIn(e.write(t, "b/site/index.html", "<h1>B</h1>"))
	for _, entry := range []*store.Entry{one, two} {
		if err := e.s.Save(entry); err != nil {
			t.Fatal(err)
		}
	}
	if one.Slug != "site" || two.Slug != "site-2" || e.meta(t, "site-2").Path != filepath.Join(e.tmp, "b", "site", "index.html") {
		t.Errorf("two links saved at once took %s and %s", one.Slug, two.Slug)
	}

	again := prepareIn(filepath.Join(e.tmp, "a", "site", "index.html"))
	if _, err := e.s.Edit("site", store.Edit{Slug: ptr("renamed")}); err != nil {
		t.Fatal(err)
	}
	if err := e.s.Save(again); err != nil || again.Slug != "renamed" || e.s.ReadMeta("site") != nil {
		t.Errorf("a link renamed since Prepare was saved as %s (%v)", again.Slug, err)
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.write(t, "site/index.html", "<h1>L</h1>")
	e.mustLink(t, file, store.Options{}, "proj")
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
	stored := &store.Meta{Source: "source-1", UpdatedAt: "2026-01-01T00:00:00.000Z"}
	if f, ok := e.s.File("p", stored, ""); !ok || f != filepath.Join(e.tmp, "store", "p", "source-1", "data.yaml") {
		t.Errorf("File of a stored page = %q, %v", f, ok)
	}
	if _, ok := e.s.File("p", stored, "x.css"); ok {
		t.Error("a stored page serves a subpath")
	}
	for _, source := range []string{"../q/source-1", "/etc", "source-1/..", "other"} {
		if f, ok := e.s.File("p", &store.Meta{Source: source}, ""); ok {
			t.Errorf("File of a page whose source is %q = %q", source, f)
		}
	}
	data := e.page(t, "store/p/source-1/data.yaml", "")
	if v := e.s.Version("p", stored, data); !strings.Contains(v, "+"+stored.UpdatedAt+"+") {
		t.Errorf("Version of a stored page = %q, want its publish time in it", v)
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
	if page, meta := e.drawn(t, "ja"), e.read(t, "store/ja/meta.json"); !strings.Contains(page, `<html lang="ja">`) ||
		strings.Contains(meta, `"lang"`) {
		t.Errorf("a page of lang: ja =\n%s\nmeta.json =\n%s", page, meta)
	}
	none := e.write(t, "none.yaml", "template: report\n")
	_, err := e.publish(t, none, store.Options{}, "repo")
	if err == nil || !strings.Contains(err.Error(), "none.yaml:1:1: name the language of the page") {
		t.Errorf("publish of data without lang: = %v", err)
	}

	// --theme picks a theme over the data's, which is then not used, and is refused for a link.
	printed := e.write(t, "printed.yaml", "template: report\ntheme: print\nlang: en\n")
	_, err = e.publish(t, printed, store.Options{}, "other")
	if err == nil || !strings.Contains(err.Error(), `there is no theme "print"`) {
		t.Errorf("publish of a theme there is none of = %v", err)
	}
	e.mustPublish(t, printed, store.Options{Theme: ptr("default")}, "other")
	if m := e.meta(t, "printed"); m.Theme != "default" {
		t.Errorf("--theme default over theme: print: theme %q", m.Theme)
	}
	listed := e.write(t, "listed.yaml", "template: report\ntheme: [dark]\nlang: en\n")
	e.mustPublish(t, listed, store.Options{Theme: ptr("default")}, "other")
	_, err = e.publish(t, frag, store.Options{Theme: ptr("print")}, "other")
	wantUserErr(t, err, "tmpl_unknown_theme", "print", "blueprint, brutal, default, editorial, ink, notebook, soft, swiss, washi")

	// A linked file is shown as it is: --theme is refused.
	site := e.write(t, "site/index.html", "<h1>A</h1>")
	_, err = e.link(t, site, store.Options{Theme: ptr("default")}, "repo")
	wantUserErr(t, err, "store_theme_link")
}

// A data file is drawn with the template it names, titled by its title: unless --title says
// otherwise, and the page records the theme given over the data's only.
func TestPublishDataFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.write(t, "plan.yaml", "template: report\ntitle: The plan\nsections:\n  - heading: Steps\n    parts:\n      - text: Do **it**.\nlang: ja\n")
	entry := e.mustPublish(t, file, store.Options{}, "")
	m := entry.Meta
	if entry.Slug != "plan" || m.Title != "The plan" || m.Theme != "" {
		t.Errorf("meta = %+v (slug %s)", m, entry.Slug)
	}
	page := e.drawn(t, "plan")
	for _, want := range []string{`<html lang="ja">`, "<h1>The plan</h1>", "<p>Do <strong>it</strong>.</p>"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page lacks %s:\n%s", want, page)
		}
	}
	e.mustPublish(t, file, store.Options{Title: ptr("Pinned")}, "")
	if page := e.drawn(t, "plan"); !strings.Contains(page, "<h1>Pinned</h1>") {
		t.Errorf("--title does not title the drawn page: %s", page)
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
	entry := e.mustLink(t, data, store.Options{Theme: ptr("default")}, "proj")
	m := entry.Meta
	if entry.Source != nil || m.Path != data || m.Theme != "default" || m.Title != "" {
		t.Errorf("the entry = %+v, source %d bytes", m, len(entry.Source))
	}
	if !m.IsDrawn() || e.s.Version("spec", &m, data) == "" {
		t.Errorf("IsDrawn = %v, Version = %q", m.IsDrawn(), e.s.Version("spec", &m, data))
	}
	if kept, globErr := filepath.Glob(filepath.Join(e.tmp, "store", "spec", "source-*")); globErr != nil || m.Source != "" || kept != nil {
		t.Errorf("a copy was stored for a link: %q, %v %v", m.Source, kept, globErr)
	}
	page, _, err := e.s.Draw("spec", &m, data)
	if err != nil || !strings.Contains(string(page), `<html lang="ja">`) || !strings.Contains(string(page), "<h1>Spec</h1>") {
		t.Errorf("Draw = %v:\n%s", err, page)
	}

	// Linked again, it reloads its tab: the version holds the publish time.
	again := e.mustLink(t, data, store.Options{}, "proj").Meta
	if e.s.Version("spec", &again, data) == e.s.Version("spec", &m, data) {
		t.Error("linking again in another theme left the version as it was")
	}
	// A slug that comes to link another file watches nothing of the one before.
	other := e.write(t, "proj/other.yaml", "template: report\nlang: en\n")
	if _, version, drawErr := e.s.Draw("spec", &m, other); drawErr != nil || version != e.s.Version("spec", &m, other) {
		t.Errorf("Draw of another file under the slug = %v at %q, watched at %q", drawErr, version, e.s.Version("spec", &m, other))
	}

	bad := e.write(t, "proj/bad.yaml", "template: report\nsections: 3\nlang: en\n")
	if _, err = e.link(t, bad, store.Options{}, "proj"); err == nil || !strings.Contains(err.Error(), "bad.yaml:2:11: sections:") {
		t.Errorf("linking a data file with mistakes = %v", err)
	}
}

// A linked data file that cannot be read is served at the version that watching it sees, so that its
// tab does not reload over and over; and one linked through a symlink is of the kind of its real file.
func TestLinkDataFileEdges(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	data := e.write(t, "proj/spec.yaml", "template: report\nlang: en\n")
	m := e.mustLink(t, data, store.Options{}, "proj").Meta
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
	if _, err = e.link(t, filepath.Join(e.tmp, "proj", "spec.html"), store.Options{Theme: ptr("default")}, "proj"); err != nil {
		t.Errorf("linking a data file through a symlink named .html, with --theme = %v", err)
	}

	bad := e.write(t, "proj/bad.yaml", "template: nosuch\nlang: en\n")
	if err = os.Symlink(bad, filepath.Join(e.tmp, "proj", "view.html")); err != nil {
		t.Fatal(err)
	}
	if _, err = e.link(t, filepath.Join(e.tmp, "proj", "view.html"), store.Options{}, "proj"); err == nil ||
		!strings.Contains(err.Error(), `unknown template "nosuch"`) {
		t.Errorf("linking a data file with mistakes through a symlink named .html = %v", err)
	}
}

// One data file linked under two slugs, drawn with two themes, is watched apart: drawing one does
// not move the version of the other.
func TestLinkDataFileTwice(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	dir := filepath.Join(e.tmp, "templates")
	if err := os.CopyFS(filepath.Join(dir, "sizes"), os.DirFS("../tmpl/testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	s := store.New(filepath.Join(e.tmp, "store"), filepath.Join(e.tmp, "home"), tmpl.NewLibrary(dir, "", "", ""))
	data := e.write(t, "proj/sizes.yaml", "template: sizes\nfiles: [{name: a, bytes: 1}]\nlang: en\n")
	link := func(slug, theme string) store.Meta {
		t.Helper()
		entry, err := s.PrepareLink(t.Context(), data, store.Options{Slug: ptr(slug), Theme: ptr(theme)},
			filepath.Join(e.tmp, "proj"))
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Save(entry); err != nil {
			t.Fatal(err)
		}
		return entry.Meta
	}
	a, b := link("a", "default"), link("b", "brutal")
	_, version, err := s.Draw("a", &a, data)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Draw("b", &b, data); err != nil {
		t.Fatal(err)
	}
	if now := s.Version("a", &a, data); now != version {
		t.Errorf("drawing b moved the version of a: %q, then %q", version, now)
	}

	// Linked again with the other theme, a is served at the version it is watched at, which waits for
	// the drawing: the tab reloads once, not twice.
	time.Sleep(5 * time.Millisecond) // another updatedAt, kept to the millisecond
	relinked := link("a", "brutal")
	if _, version, err = s.Draw("a", &relinked, data); err != nil || version != s.Version("a", &relinked, data) {
		t.Errorf("a linked again was served at %q, watched at %q (%v)", version, s.Version("a", &relinked, data), err)
	}
}

// A linked data file whose diagrams show icons not fetched yet is drawn again once they are: the fetch
// its mistake asks for reloads its tab.
func TestLinkDataFileIcons(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	icons := filepath.Join(e.tmp, "icons")
	s := store.New(filepath.Join(e.tmp, "store"), filepath.Join(e.tmp, "home"), tmpl.NewLibrary("", "", "", icons))
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

// A linked data file drawn with a part and a theme of one's own, kept elsewhere and linked into the
// directories of parts and of themes, themselves symlinks, is drawn again when a file of either is saved, or the link is
// pointed elsewhere or removed, and not when a file a link in the part leads out to is, which floe does
// not read.
func TestLinkDataFileParts(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	kept := filepath.Join(e.tmp, "kept", "note")
	e.write(t, "kept/note/schema.json", `{"type": "object", "properties": {"text": {"type": "string"}}}`)
	e.write(t, "kept/note/part.html", `<aside data-floe-part="note">{{.text}}</aside>`)
	css := e.write(t, "kept/note/part.css", ":scope { margin: 0; }\n")
	theme := e.write(t, "kept/mine/base.css", "blockquote { color: var(--fg); }\n")
	out := e.write(t, "elsewhere/notes.txt", "x")
	if err := os.Symlink(filepath.Dir(out), filepath.Join(kept, "elsewhere")); err != nil {
		t.Fatal(err)
	}
	under, themes := filepath.Join(e.tmp, "real-parts"), filepath.Join(e.tmp, "real-themes")
	for _, dir := range []string{under, themes} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for link, to := range map[string]string{
		filepath.Join(under, "note"): kept, filepath.Join(themes, "mine"): filepath.Dir(theme),
		filepath.Join(e.tmp, "parts"): under, filepath.Join(e.tmp, "themes"): themes,
	} {
		if err := os.Symlink(to, link); err != nil {
			t.Fatal(err)
		}
	}
	parts := filepath.Join(e.tmp, "parts")
	s := store.New(filepath.Join(e.tmp, "store"), filepath.Join(e.tmp, "home"), tmpl.NewLibrary("", parts, filepath.Join(e.tmp, "themes"), ""))
	data := e.write(t, "proj/spec.yaml", "template: report\nlang: en\ntheme: mine\nsections:\n  - parts:\n      - note: { text: Mine }\n")
	m := store.Meta{Path: data}
	page, version, err := s.Draw("spec", &m, data)
	if err != nil || !strings.Contains(string(page), `<aside data-floe-part="note">Mine</aside>`) {
		t.Fatalf("Draw with the note of one's own = %v:\n%s", err, page)
	}
	if v := s.Version("spec", &m, data); v != version {
		t.Fatalf("the version moved from %q to %q with nothing changed", version, v)
	}
	later := time.Now().Add(time.Hour)
	if err = os.Chtimes(out, later, later); err != nil {
		t.Fatal(err)
	}
	if v := s.Version("spec", &m, data); v != version {
		t.Errorf("the version moved from %q to %q once a file out of the part is saved", version, v)
	}
	for i, file := range []string{css, theme} {
		later := time.Now().Add(time.Duration(i+1) * time.Minute)
		if err = os.Chtimes(file, later, later); err != nil {
			t.Fatal(err)
		}
		if v := s.Version("spec", &m, data); v == version {
			t.Errorf("the version stays %q once %s is saved", v, file)
		}
		version = s.Version("spec", &m, data)
	}
	// The part's link pointed at another copy of it, then removed.
	link := filepath.Join(under, "note")
	e.write(t, "kept2/note/schema.json", `{"type": "object", "properties": {"text": {"type": "string"}}}`)
	e.write(t, "kept2/note/part.html", `<aside data-floe-part="note">{{.text}}</aside>`)
	e.write(t, "kept2/note/part.css", ":scope { margin: 0; }\n")
	for _, step := range []struct {
		what   string
		change func() error
	}{
		{"pointed elsewhere", func() error {
			if rmErr := os.Remove(link); rmErr != nil {
				return rmErr
			}
			return os.Symlink(filepath.Join(e.tmp, "kept2", "note"), link)
		}},
		{"removed", func() error { return os.Remove(link) }},
	} {
		if err = step.change(); err != nil {
			t.Fatal(err)
		}
		if v := s.Version("spec", &m, data); v == version {
			t.Errorf("the version stays %q once the part's link is %s", v, step.what)
		}
		version = s.Version("spec", &m, data)
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

// PrepareData reads nothing from disk: a data file that names images is asked for them first, then
// checked with those sent, which the entry keeps. A publish its slug, its owner or its schema refuses
// is refused before they are asked for, and so is a blank title.
func TestPrepareData(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	s := store.New(filepath.Join(e.tmp, "store"), filepath.Join(e.tmp, "home"), tmpl.NewLibrary("../tmpl/testdata", "", "", ""))
	png, err := os.ReadFile("../tmpl/testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("template: sizes\nlang: en\nfiles: [{ name: a, bytes: 1 }]\nchart: img/chart.png\n")
	up := store.Upload{File: "plans/sizes.yaml", Dir: "/plans", Source: src, Images: nil, Project: "proj"}
	var need *store.NeedImages
	if _, err = s.PrepareData(up, store.Options{}); !errors.As(err, &need) || !slices.Equal(need.Names, []string{"img/chart.png"}) {
		t.Fatalf("PrepareData without images = %v, want img/chart.png asked for", err)
	}
	// The mistakes of the schema come first: what names an image is not looked at until it passes.
	bad := up
	bad.Source = []byte("template: sizes\nlang: en\nfiles: 3\nchart: img/chart.png\n")
	if _, err = s.PrepareData(bad, store.Options{}); err == nil || errors.As(err, &need) || !strings.Contains(err.Error(), "plans/sizes.yaml:3:8") {
		t.Errorf("PrepareData of data the schema refuses = %v, want its mistake before images are asked for", err)
	}
	bad.Source = []byte("template: sizes\ntheme: nosuch\nlang: en\nfiles: []\nchart: img/chart.png\n")
	if _, err = s.PrepareData(bad, store.Options{}); err == nil || errors.As(err, &need) || !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("PrepareData of data naming no theme of its template = %v, want its mistake before images are asked for", err)
	}
	if _, err = s.PrepareData(up, store.Options{Title: ptr(" ")}); err == nil || !strings.Contains(err.Error(), "--title is empty") {
		t.Errorf("PrepareData with a blank title = %v, want it refused", err)
	}
	// Every option given empty is told at once.
	if _, err = (store.Options{Title: ptr(" "), Theme: ptr("")}).Normalized(); err == nil ||
		!strings.HasPrefix(err.Error(), "--title is empty") || !strings.Contains(err.Error(), "\n--theme is empty") {
		t.Errorf("Normalized with a blank title and theme = %v, want both", err)
	}
	// The command line sends the options it normalized, which the server's normalizing leaves as they are.
	given := store.Options{Slug: ptr(" plan "), Title: ptr(" A "), Description: ptr(" d "), Theme: ptr(" ink ")}
	once, err := given.Normalized()
	if twice, err2 := once.Normalized(); err != nil || err2 != nil || !reflect.DeepEqual(once, twice) || *once.Slug != "plan" {
		t.Errorf("Normalized twice = %+v, then %+v (%v, %v), want alike", once, twice, err, err2)
	}
	// What is checked of the file is told with what is checked of the options.
	notData := up
	notData.File = "plans/page.html"
	if _, err = s.PrepareData(notData, store.Options{Title: ptr(" ")}); err == nil || strings.Count(err.Error(), "\n") != 1 ||
		!strings.Contains(err.Error(), "--title is empty") {
		t.Errorf("PrepareData of no data file with a blank title = %v, want both", err)
	}
	if _, err = s.PrepareLink(t.Context(), "nope.html", store.Options{Title: ptr(" ")}, e.tmp); err == nil ||
		strings.Count(err.Error(), "\n") != 1 || !strings.Contains(err.Error(), "--title is empty") {
		t.Errorf("PrepareLink of no file with a blank title = %v, want both", err)
	}
	up.Images = fstest.MapFS{}
	want := `plans/sizes.yaml:4:8: chart: there is no image "img/chart.png" under /plans`
	if _, err = s.PrepareData(up, store.Options{}); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("PrepareData with none of its images = %v, want %s", err, want)
	}
	up.Images = fstest.MapFS{"img/chart.png": {Data: png}}
	entry, err := s.PrepareData(up, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Slug != "sizes" || entry.Meta.PublishedFrom != "proj" || !bytes.Equal(entry.Images["img/chart.png"], png) {
		t.Errorf("entry = %s from %q, %d images", entry.Slug, entry.Meta.PublishedFrom, len(entry.Images))
	}
	// So is a page of another project.
	if err = s.Save(entry); err != nil {
		t.Fatal(err)
	}
	up.Images, up.Project = nil, "other"
	if _, err = s.PrepareData(up, store.Options{}); err == nil || errors.As(err, &need) || !strings.Contains(err.Error(), "proj") {
		t.Errorf("PrepareData of a page of another project = %v, want it refused before images are asked for", err)
	}
	up.File = "plans/page.html"
	if _, err = s.PrepareData(up, store.Options{}); err == nil || !strings.Contains(err.Error(), "page.html") {
		t.Errorf("PrepareData of HTML = %v, want it refused", err)
	}
}
