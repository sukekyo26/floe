package server_test

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/store"
)

// A linked data file is drawn each time it is opened, with its images, in its language: none of its
// directory is served, a mistake shows the list of mistakes instead of the page, and saving its data,
// an image or a file of its template reloads the open tab.
func TestDrawnLink(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	if err := os.CopyFS(filepath.Join(e.tmp, "templates", "sizes"), os.DirFS("../tmpl/testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	png, err := os.ReadFile("../tmpl/testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	chart := e.write(t, "work/spec/chart.png", string(png))
	const right = "template: sizes\ntitle: Sizes\nfiles: [{name: a, bytes: 1536}]\nchart: chart.png\nlang: ja\n"
	data := e.write(t, "work/spec/sizes.yaml", right)
	e.publish(t, data, store.Options{Link: true, Slug: nil, Title: nil, Description: nil, Theme: nil, Force: false})

	r := e.get(t, "/a/sizes/?raw")
	for _, want := range []string{`<html lang="ja">`, "<td>1.5 KiB</td>", `<img src="data:image/png;base64,`, "/api/events/sizes?since="} {
		if r.status != http.StatusOK || !strings.Contains(r.body, want) {
			t.Fatalf("the page = %d, without %s:\n%s", r.status, want, r.body)
		}
	}
	if got := r.header.Get("Content-Security-Policy"); got != sandbox+"; "+cutOff+e.base+"/api/events/" {
		t.Errorf("the page's CSP = %q", got)
	}
	if got := e.get(t, "/a/sizes/chart.png").status; got != http.StatusNotFound {
		t.Errorf("a file beside the data file = %d, want it not served", got)
	}
	if d := e.get(t, "/api/artifacts/sizes/download"); d.status != http.StatusOK || !strings.Contains(d.body, "<td>1.5 KiB</td>") ||
		strings.Contains(d.body, "EventSource") || !strings.HasPrefix(d.body, "<!doctype html>\n"+cutOffMeta) {
		t.Errorf("the download = %d:\n%s", d.status, d.body)
	}
	if l := e.get(t, "/api/artifacts"); !strings.Contains(l.body, `"title":"Sizes"`) {
		t.Errorf("the list = %s", l.body)
	}

	// Each source reloads the tab: the image, a file of the template, and the data file itself.
	since := regexp.MustCompile(`since=([^"]+)`).FindStringSubmatch(r.body)[1]
	for _, file := range []string{chart, filepath.Join(e.tmp, "templates", "sizes", "themes", "default.css"), data} {
		later := time.Now().Add(time.Minute)
		if err = os.Chtimes(file, later, later); err != nil {
			t.Fatal(err)
		}
		if _, body := e.events(t, "/api/events/sizes?since="+since, 3*time.Second); !strings.Contains(body, "event: reload") {
			t.Errorf("saving %s did not reload the tab: %q", file, body)
		}
		since = url.QueryEscape(e.store.Version("sizes", e.store.ReadMeta("sizes"), data))
	}

	// A file of the template whose name starts with a dot (.git) is no file of it.
	before := e.store.Version("sizes", e.store.ReadMeta("sizes"), data)
	e.write(t, "templates/sizes/.git/HEAD", "ref")
	if after := e.store.Version("sizes", e.store.ReadMeta("sizes"), data); after != before {
		t.Errorf("a file under .git of the template changed the version: %s, then %s", before, after)
	}

	// A mistake shows the mistakes, in the reader's language, and is still listed as there.
	e.write(t, "work/spec/sizes.yaml", strings.Replace(right, "bytes: 1536", "bytes: lots", 1))
	r = e.get(t, "/a/sizes/?raw", "Accept-Language", "ja")
	if r.status != http.StatusOK || !strings.Contains(r.body, "データファイルに誤りがあります") ||
		!strings.Contains(r.body, "sizes.yaml:3:26: files[0].bytes:") || !strings.Contains(r.body, "/api/events/sizes?since=") ||
		r.header.Get("Content-Security-Policy") != sandbox+"; "+cutOff+e.base+"/api/events/" {
		t.Errorf("the page of a data file with mistakes = %d:\n%s", r.status, r.body)
	}
	if l := e.get(t, "/api/artifacts"); strings.Contains(l.body, `"missing":true`) {
		t.Errorf("a data file with mistakes is listed as missing: %s", l.body)
	}
	if d := e.get(t, "/api/artifacts/sizes/download", "Accept-Language", "ja"); d.status != http.StatusConflict ||
		!strings.Contains(d.body, "sizes.yaml:3:26: files[0].bytes: 文字列ではなく整数で書いてください") {
		t.Errorf("the download of a data file with mistakes = %d %s", d.status, d.body)
	}
	// One that cannot be read is not there to download.
	if err = os.Chmod(data, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(data, 0o600); err != nil {
			t.Error(err)
		}
	})
	if d := e.get(t, "/api/artifacts/sizes/download"); d.status != http.StatusNotFound {
		t.Errorf("the download of a data file that cannot be read = %d", d.status)
	}
}

// A stored data file is drawn each time it is opened from the copy publish keeps of it and of its
// images: the original may go, publishing again replaces the copy whole, and saving the copy reloads
// the tab.
func TestDrawnStored(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	if err := os.CopyFS(filepath.Join(e.tmp, "templates", "sizes"), os.DirFS("../tmpl/testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	png, err := os.ReadFile("../tmpl/testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	e.write(t, "work/spec/gone/old.png", string(png))
	e.write(t, "work/spec/img/chart.png", string(png))
	const right = "template: sizes\ntitle: Sizes\nfiles: [{name: a, bytes: 1536}]\nchart: img/chart.png\nlang: ja\n"
	data := e.write(t, "work/spec/sizes.yaml", strings.Replace(right, "img/chart.png", "gone/old.png", 1))
	e.publish(t, data, store.Options{})
	// What a write stopped halfway left goes on the next publish.
	e.write(t, "store/sizes/source/.tmp-stray", "x")
	e.write(t, "work/spec/sizes.yaml", right)
	e.publish(t, data, store.Options{})
	if err = os.RemoveAll(filepath.Join(e.tmp, "work")); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(e.store.Root(), "sizes", "source")
	if b, readErr := os.ReadFile(filepath.Join(source, "data.yaml")); readErr != nil || string(b) != right {
		t.Errorf("the copy of the data file = %q, %v", b, readErr)
	}
	if b, readErr := os.ReadFile(filepath.Join(source, "img", "chart.png")); readErr != nil || string(b) != string(png) {
		t.Errorf("the copy of the image = %v", readErr)
	}
	for _, gone := range []string{"source/gone", "source/.tmp-stray", "index.html"} {
		if _, statErr := os.Stat(filepath.Join(e.store.Root(), "sizes", gone)); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("%s is in the store: %v", gone, statErr)
		}
	}
	r := e.get(t, "/a/sizes/?raw")
	for _, want := range []string{"<td>1.5 KiB</td>", `<img src="data:image/png;base64,`, "/api/events/sizes?since="} {
		if r.status != http.StatusOK || !strings.Contains(r.body, want) {
			t.Fatalf("the page = %d, without %s:\n%s", r.status, want, r.body)
		}
	}
	if got := statuses(t, e, "/a/sizes/data.yaml", "/a/sizes/source/data.yaml"); got != "404 404" {
		t.Errorf("the copy is served: %s", got)
	}
	if d := e.get(t, "/api/artifacts/sizes/download"); !strings.HasPrefix(d.body, "<!doctype html>\n"+cutOffMeta) {
		t.Errorf("the download = %d:\n%s", d.status, d.body)
	}

	// Saving the copy reloads the tab, and a mistake in it shows the mistakes.
	since := regexp.MustCompile(`since=([^"]+)`).FindStringSubmatch(r.body)[1]
	e.write(t, "store/sizes/source/data.yaml", strings.Replace(right, "bytes: 1536", "bytes: lots", 1))
	later := time.Now().Add(time.Minute)
	if err = os.Chtimes(filepath.Join(source, "data.yaml"), later, later); err != nil {
		t.Fatal(err)
	}
	if _, body := e.events(t, "/api/events/sizes?since="+since, 3*time.Second); !strings.Contains(body, "event: reload") {
		t.Errorf("saving the copy did not reload the tab: %q", body)
	}
	if r := e.get(t, "/a/sizes/?raw"); !strings.Contains(r.body, "source/data.yaml:3:26: files[0].bytes:") ||
		!strings.Contains(r.body, "publish it again") || !strings.Contains(r.body, "is in "+source+".") {
		t.Errorf("the page of a copy with mistakes:\n%s", r.body)
	}

	// An image under a directory named as the copy of the data file is refused at its line, in any case:
	// macOS ignores it.
	e.write(t, "clash/Data.yaml/c.png", string(png))
	clash := e.write(t, "clash/sizes.yaml", strings.Replace(right, "img/chart.png", "Data.yaml/c.png", 1))
	if _, prepErr := e.store.Prepare(t.Context(), clash, store.Options{Slug: ptr("sizes")}, e.tmp); prepErr == nil ||
		!strings.Contains(prepErr.Error(), `sizes.yaml:4:8: chart: "Data.yaml/c.png" is under a directory named data.yaml`) {
		t.Errorf("publish of an image under Data.yaml/ = %v", prepErr)
	}

	// An image whose path turns from a directory into a file, and back, is published each time.
	e.write(t, "flip/x.png/a.png", string(png))
	flip := e.write(t, "flip/sizes.yaml", strings.Replace(right, "img/chart.png", "x.png/a.png", 1))
	e.publish(t, flip, store.Options{Slug: ptr("flip")})
	if err = os.Rename(filepath.Join(e.tmp, "flip", "x.png"), filepath.Join(e.tmp, "flip", "x.png.dir")); err != nil {
		t.Fatal(err)
	}
	e.write(t, "flip/x.png", string(png))
	e.write(t, "flip/sizes.yaml", strings.Replace(right, "img/chart.png", "x.png", 1))
	e.publish(t, flip, store.Options{Slug: ptr("flip")})
	if err = os.Remove(filepath.Join(e.tmp, "flip", "x.png")); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(filepath.Join(e.tmp, "flip", "x.png.dir"), filepath.Join(e.tmp, "flip", "x.png")); err != nil {
		t.Fatal(err)
	}
	e.write(t, "flip/sizes.yaml", strings.Replace(right, "img/chart.png", "x.png/a.png", 1))
	e.publish(t, flip, store.Options{Slug: ptr("flip")})
	if r := e.get(t, "/a/flip/?raw"); !strings.Contains(r.body, `<img src="data:image/png;base64,`) {
		t.Errorf("the page after the image's path turned twice:\n%s", r.body)
	}

	// Renamed on the management page, it is drawn under its new slug.
	e.write(t, "store/sizes/source/data.yaml", right)
	if _, err := e.store.Edit("sizes", store.Edit{Slug: ptr("moved")}); err != nil {
		t.Fatal(err)
	}
	if r := e.get(t, "/a/moved/?raw"); !strings.Contains(r.body, "<td>1.5 KiB</td>") {
		t.Errorf("the renamed page:\n%s", r.body)
	}
}

// The management page links a data file as floe publish --link does, refusing one with mistakes in the
// reader's language.
func TestAddDataLink(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	bad := e.write(t, "work/bad.yaml", "title: 3\n")
	r := e.do(t, http.MethodPost, "/api/artifacts", `{"path": "`+bad+`"}`, "Origin", e.origin, "Content-Type",
		"application/json", "Accept-Language", "ja")
	if r.status != http.StatusBadRequest || !strings.Contains(r.body, "bad.yaml:1:1: 最上位に template: report") {
		t.Errorf("linking a data file with mistakes = %d %s", r.status, r.body)
	}
	good := e.page(t, "work/good.yaml", "Good", "")
	r = e.do(t, http.MethodPost, "/api/artifacts", `{"path": "`+good+`"}`, "Origin", e.origin, "Content-Type",
		"application/json")
	if r.status != http.StatusCreated || !strings.Contains(e.get(t, "/a/good/?raw").body, "<h1>Good</h1>") {
		t.Errorf("linking a data file = %d %s", r.status, r.body)
	}
}
