package server_test

import (
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
	const right = "template: sizes\ntitle: Sizes\nfiles: [{name: a, bytes: 1536}]\nchart: chart.png\n"
	data := e.write(t, "work/spec/sizes.yaml", right)
	ja := "ja"
	e.publish(t, data, store.Options{Link: true, Lang: &ja, Slug: nil, Title: nil, Description: nil, Theme: nil, Force: false})

	r := e.get(t, "/a/sizes/?raw")
	for _, want := range []string{`<html lang="ja">`, "<td>1.5 KiB</td>", `<img src="data:image/png;base64,`, "/api/events/sizes?since="} {
		if r.status != http.StatusOK || !strings.Contains(r.body, want) {
			t.Fatalf("the page = %d, without %s:\n%s", r.status, want, r.body)
		}
	}
	if got := e.get(t, "/a/sizes/chart.png").status; got != http.StatusNotFound {
		t.Errorf("a file beside the data file = %d, want it not served", got)
	}
	if d := e.get(t, "/api/artifacts/sizes/download"); d.status != http.StatusOK || !strings.Contains(d.body, "<td>1.5 KiB</td>") || strings.Contains(d.body, "EventSource") {
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
		since = url.QueryEscape(e.store.Version(e.store.ReadMeta("sizes"), data))
	}

	// A mistake shows the mistakes, in the reader's language, and is still listed as there.
	e.write(t, "work/spec/sizes.yaml", strings.Replace(right, "bytes: 1536", "bytes: lots", 1))
	r = e.get(t, "/a/sizes/?raw", "Accept-Language", "ja")
	if r.status != http.StatusOK || !strings.Contains(r.body, "データファイルに誤りがあります") ||
		!strings.Contains(r.body, "sizes.yaml:3:26: files[0].bytes:") || !strings.Contains(r.body, "/api/events/sizes?since=") {
		t.Errorf("the page of a data file with mistakes = %d:\n%s", r.status, r.body)
	}
	if l := e.get(t, "/api/artifacts"); strings.Contains(l.body, `"missing":true`) {
		t.Errorf("a data file with mistakes is listed as missing: %s", l.body)
	}
	if d := e.get(t, "/api/artifacts/sizes/download"); d.status != http.StatusConflict {
		t.Errorf("the download of a data file with mistakes = %d", d.status)
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
