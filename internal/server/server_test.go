package server_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
)

// env is a server on a free port over a store in a temporary directory.
type env struct {
	tmp    string
	store  *store.Store
	base   string
	origin string
	done   chan error
}

func newEnv(t *testing.T) *env {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v", ln.Addr())
	}
	port := addr.Port
	// link.Check refuses a home that does not exist, so the stand-in home is made.
	if err := os.Mkdir(filepath.Join(tmp, "home"), 0o700); err != nil {
		t.Fatal(err)
	}
	st := store.New(filepath.Join(tmp, "store"), filepath.Join(tmp, "home"), tmpl.NewLibrary(filepath.Join(tmp, "templates"), filepath.Join(tmp, "parts"), ""))
	e := &env{
		tmp: tmp, store: st, base: "http://127.0.0.1:" + strconv.Itoa(port),
		origin: "http://127.0.0.1:" + strconv.Itoa(port), done: make(chan error, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		e.done <- server.New(server.Config{Store: st, Port: port, Build: "b1"}).Serve(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-e.done; err != nil {
			t.Errorf("Serve: %v", err)
		}
	})
	return e
}

// The CSP of every page, and what a page drawn from a data file adds, up to the source it connects to.
const (
	sandbox = "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-forms allow-modals allow-downloads"
	cdns    = "https://cdnjs.cloudflare.com https://cdn.jsdelivr.net https://unpkg.com"
	cutOff  = "default-src 'none'; img-src data: blob:; media-src data: blob:; " +
		"style-src 'unsafe-inline' https://fonts.googleapis.com " + cdns + "; " +
		"font-src data: blob: https://fonts.gstatic.com " + cdns + "; script-src 'unsafe-inline' " + cdns + "; " +
		"form-action 'none'; base-uri 'none'; connect-src "
	cutOffMeta = `<meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="` + cutOff + `'none'">`
)

// page writes a data file for the report template titled title ("" for none), with html as its one
// part when there is some.
func (e *env) page(t *testing.T, rel, title, html string) string {
	t.Helper()

	var b strings.Builder
	b.WriteString("template: report\nlang: en\n")
	if title != "" {
		q, err := json.Marshal(title) // a JSON string is a YAML one too
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString("title: " + string(q) + "\n")
	}
	if html != "" {
		b.WriteString("sections:\n  - parts:\n      - html: |\n")
		for line := range strings.Lines(html) {
			b.WriteString("          " + line)
		}
		b.WriteString("\n")
	}
	return e.write(t, rel, b.String())
}

func (e *env) write(t *testing.T, rel, body string) string {
	t.Helper()

	path := filepath.Join(e.tmp, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	return path
}

func (e *env) publish(t *testing.T, file string, opts store.Options) {
	t.Helper()

	e.save(t, e.prepare, file, opts)
}

// prepare checks a publish of the data file file on disk from cwd as the CLI has the server check it,
// but in process: read by store.ReadUpload, its images read under its directory.
func (e *env) prepare(ctx context.Context, file string, opts store.Options, cwd string) (*store.Entry, error) {
	up, err := store.ReadUpload(ctx, file, cwd)
	if err != nil {
		return nil, err
	}
	up.Images = os.DirFS(up.Dir)
	return e.store.PrepareData(up, opts)
}

func (e *env) link(t *testing.T, file string, opts store.Options) {
	t.Helper()

	e.save(t, e.store.PrepareLink, file, opts)
}

func (e *env) save(t *testing.T, prepare func(context.Context, string, store.Options, string) (*store.Entry, error),
	file string, opts store.Options,
) {
	t.Helper()

	entry, err := prepare(t.Context(), file, opts, e.tmp)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.Save(entry); err != nil {
		t.Fatal(err)
	}
}

type resp struct {
	status int
	header http.Header
	body   string
}

var client = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Timeout:       5 * time.Second,
}

// do sends a request; target is sent as is (no cleaning), header pairs are name, value.
func (e *env) do(t *testing.T, method, target, body string, header ...string) resp {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, e.base+target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(header); i += 2 {
		if header[i] == "Host" {
			req.Host = header[i+1]
			continue
		}
		req.Header.Set(header[i], header[i+1])
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp{status: res.StatusCode, header: res.Header, body: string(b)}
}

func (e *env) get(t *testing.T, target string, header ...string) resp {
	t.Helper()

	return e.do(t, http.MethodGet, target, "", header...)
}

// write sends JSON from the management page's origin.
func (e *env) send(t *testing.T, method, target, body string) resp {
	t.Helper()

	return e.do(t, method, target, body, "Origin", e.origin, "Content-Type", "application/json")
}

// raw sends a request line over TCP and returns the status code.
func (e *env) raw(t *testing.T, target string) int {
	t.Helper()

	var d net.Dialer
	conn, err := d.DialContext(t.Context(), "tcp", strings.TrimPrefix(e.base, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	host := strings.TrimPrefix(e.base, "http://")
	if _, err = io.WriteString(conn, "GET "+target+" HTTP/1.1\r\nHost: "+host+"\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

func statuses(t *testing.T, e *env, targets ...string) string {
	t.Helper()

	codes := make([]string, 0, len(targets))
	for _, target := range targets {
		codes = append(codes, strconv.Itoa(e.get(t, target).status))
	}
	return strings.Join(codes, " ")
}

func TestManagementPageAndFrame(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, e.page(t, "untitled.yaml", "", "<p>untitled</p>"), store.Options{})

	for _, target := range []string{"/", "/a/untitled/"} {
		r := e.get(t, target)
		if r.status != http.StatusOK || !strings.Contains(r.body, "<title>floe</title>") ||
			r.header.Get("Content-Security-Policy") != "frame-ancestors 'self' vscode-webview: vscode-file:" {
			t.Errorf("GET %s = %d, CSP %q", target, r.status, r.header.Get("Content-Security-Policy"))
		}
		if !strings.Contains(r.body, `<html lang="en">`) {
			t.Errorf("GET %s without Accept-Language is not in English", target)
		}
		if r := e.get(t, target, "Accept-Language", "ja,en;q=0.5"); !strings.Contains(r.body, `<html lang="ja">`) ||
			!strings.Contains(r.body, "一覧へ戻る") {
			t.Errorf("GET %s for a Japanese reader is not in Japanese", target)
		}
	}
	if r := e.get(t, "/a/no-such/"); r.status != http.StatusNotFound || !strings.Contains(r.body, `id="notfound"`) ||
		r.header.Get("Content-Security-Policy") != "frame-ancestors 'self' vscode-webview: vscode-file:" {
		t.Errorf("an unknown page URL = %d, CSP %q, want the frame with 404", r.status, r.header.Get("Content-Security-Policy"))
	}
	if r := e.get(t, "/a/untitled"); r.status != http.StatusMovedPermanently || r.header.Get("Location") != "/a/untitled/" {
		t.Errorf("GET /a/untitled = %d %q", r.status, r.header.Get("Location"))
	}
	if got := statuses(t, e, "/a/Bad_Slug", "/a/Bad_Slug/", "/nope", "/api/nope"); got != "404 404 404 404" {
		t.Errorf("unknown routes = %s", got)
	}
	if r := e.do(t, http.MethodPost, "/", ""); r.status != http.StatusNotFound || r.body != "not found" {
		t.Errorf("POST / = %d %q, want 404", r.status, r.body)
	}
}

func TestHostAndRequestTarget(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	port := strings.TrimPrefix(e.base, "http://127.0.0.1:")
	if r := e.get(t, "/", "Host", "localhost:"+port); r.status != http.StatusOK {
		t.Errorf("Host localhost = %d", r.status)
	}
	if r := e.get(t, "/", "Host", "evil.example"); r.status != http.StatusForbidden {
		t.Errorf("a foreign Host = %d, want 403", r.status)
	}
	for _, target := range []string{"//", "http://["} {
		if got := e.raw(t, target); got != http.StatusBadRequest {
			t.Errorf("request target %q = %d, want 400", target, got)
		}
	}
	var h server.Health
	if err := json.Unmarshal([]byte(e.get(t, "/api/health").body), &h); err != nil || h.App != "floe" ||
		h.PID != os.Getpid() || h.Root != e.store.Root() || h.Build != "b1" {
		t.Errorf("health = %+v, %v", h, err)
	}
}

func TestStoredPage(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, e.page(t, "My Demo.yaml", "Demo Page", "<p>hi</p>"), store.Options{})
	m := e.store.ReadMeta("my-demo")
	file, _ := e.store.File("my-demo", m, "")
	stored, version, err := e.store.Draw("my-demo", m, file)
	if err != nil {
		t.Fatal(err)
	}

	r := e.get(t, "/a/my-demo/?raw")
	script := "\n<script>{const es = new window.EventSource(\"/api/events/my-demo?since=" + url.QueryEscape(version) + "\");"
	if !strings.HasPrefix(r.body, string(stored)) || !strings.HasPrefix(r.body[len(stored):], script) {
		t.Errorf("?raw should be the stored page then the reload script:\n%s", r.body)
	}
	if r.header.Get("Content-Security-Policy") != sandbox+"; "+cutOff+e.base+"/api/events/" ||
		r.header.Get("X-Content-Type-Options") != "nosniff" || r.header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("?raw headers = %v", r.header)
	}
	// The stream is allowed under the name the page was opened by.
	localhost := strings.Replace(e.base, "127.0.0.1", "localhost", 1)
	if got := e.get(t, "/a/my-demo/?raw", "Host", strings.TrimPrefix(localhost, "http://")).header.Get("Content-Security-Policy"); !strings.HasSuffix(got, " "+localhost+"/api/events/") {
		t.Errorf("?raw by localhost: CSP %q", got)
	}

	dl := e.get(t, "/api/artifacts/my-demo/download")
	want := strings.Replace(string(stored), "<!doctype html>", "<!doctype html>\n"+cutOffMeta, 1)
	if !strings.HasPrefix(string(stored), "<!doctype html>") || dl.body != want ||
		dl.header.Get("Content-Disposition") != `attachment; filename="my-demo.html"` {
		t.Errorf("download = %q, %v", dl.body, dl.header)
	}

	if got := statuses(t, e, "/api/artifacts/no-such/download", "/api/artifacts/Bad_Slug/download", "/a/my-demo/x.css",
		"/a/no-such/?raw"); got != "404 404 404 404" {
		t.Errorf("unknown downloads and a stored page's subpath = %s", got)
	}

	var list []store.Listed
	if err := json.Unmarshal([]byte(e.get(t, "/api/artifacts").body), &list); err != nil || len(list) != 1 || list[0].Title != "Demo Page" {
		t.Errorf("list = %+v, %v", list, err)
	}

	// A page stored as HTML (by the artifact skill) is not read: it has no data file to draw.
	e.write(t, "store/old/index.html", "<!doctype html><p>old</p>")
	e.write(t, "store/old/meta.json", `{"title":"Old","description":"","createdAt":"2026-01-01T00:00:00.000Z","updatedAt":"2026-01-01T00:00:00.000Z"}`)
	if r := e.get(t, "/a/old/?raw"); strings.Contains(r.body, "<p>old</p>") || !strings.Contains(r.body, "old/data.yaml") ||
		!strings.Contains(r.body, "floe rm old") {
		t.Errorf("a page stored as HTML = %s", r.body)
	}
	if got := e.get(t, "/api/artifacts/old/download").status; got != http.StatusNotFound {
		t.Errorf("a page stored as HTML: download %d", got)
	}
}

// events reads an event stream for up to wait and returns what came. It may run in a goroutine, so it
// reports failures with Error.
func (e *env) events(t *testing.T, target string, wait time.Duration) (http.Header, string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), wait)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.base+target, http.NoBody)
	if err != nil {
		t.Error(err)
		return nil, ""
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return nil, ""
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Error(err)
	}
	return res.Header, string(b)
}

func TestLiveReload(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	file := e.page(t, "My Demo.yaml", "Demo Page", "")
	e.publish(t, file, store.Options{})
	m := e.store.ReadMeta("my-demo")
	stored, _ := e.store.File("my-demo", m, "")
	now := url.QueryEscape(e.store.Version("my-demo", m, stored))

	h, body := e.events(t, "/api/events/my-demo?since=stale", 3*time.Second)
	if h.Get("Access-Control-Allow-Origin") != "null" || body != "retry: 1000\n\nevent: reload\ndata: changed\n\n" {
		t.Errorf("a stale version: %v %q", h, body)
	}
	if _, body := e.events(t, "/api/events/my-demo?since="+now, 1500*time.Millisecond); strings.Contains(body, "reload") {
		t.Errorf("the current version got %q", body)
	}

	got := make(chan string, 1)
	go func() {
		_, body := e.events(t, "/api/events/my-demo?since="+now, 4*time.Second)
		got <- body
	}()
	time.Sleep(300 * time.Millisecond)
	time.Sleep(2 * time.Millisecond) // a republish in the same millisecond would keep the version
	e.publish(t, file, store.Options{})
	if body := <-got; !strings.Contains(body, "event: reload") {
		t.Errorf("republishing did not notify the open stream: %q", body)
	}
	if _, body := e.events(t, "/api/events/no-such?since=", time.Second); strings.Contains(body, "reload") {
		t.Errorf("an unknown page with an empty version reloads: %q", body)
	}
	if r := e.get(t, "/api/events/Bad_Slug"); r.status != http.StatusNotFound {
		t.Errorf("events of a bad slug = %d", r.status)
	}
}

func TestEdit(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, e.page(t, "My Demo.yaml", "Demo Page", ""), store.Options{})
	e.publish(t, e.page(t, "untitled.yaml", "", "<p>u</p>"), store.Options{})
	before := e.store.ReadMeta("my-demo")
	patch := func(target, body string) int { return e.send(t, http.MethodPatch, target, body).status }

	r := e.do(t, http.MethodPatch, "/api/artifacts/my-demo", `{"description":"x"}`, "Origin", "http://evil.example",
		"Content-Type", "application/json")
	if r.status != http.StatusForbidden {
		t.Errorf("PATCH from another origin = %d", r.status)
	}
	if got := patch("/api/artifacts/my-demo", `{"description":"  edited  "}`); got != http.StatusOK {
		t.Fatalf("PATCH description = %d", got)
	}
	if m := e.store.ReadMeta("my-demo"); m.Description != "edited" || m.UpdatedAt != before.UpdatedAt {
		t.Errorf("PATCH description: %+v", m)
	}
	codes := []int{
		patch("/api/artifacts/no-such", `{"description":"x"}`),
		patch("/api/artifacts/Bad_Slug", `{"description":"x"}`),
		patch("/api/artifacts/my-demo", `{"description":1}`),
		patch("/api/artifacts/my-demo", `{"title":null}`),
		patch("/api/artifacts/my-demo", `true`),
		patch("/api/artifacts/my-demo", `["x"]`),
		patch("/api/artifacts/my-demo", `null`),
		patch("/api/artifacts/my-demo", `{bad`),
		patch("/api/artifacts/my-demo", `{"favorite":"yes"}`),
		patch("/api/artifacts/my-demo", `{"slug":"untitled"}`),
		patch("/api/artifacts/my-demo", `{"slug":"Bad Slug"}`),
		patch("/api/artifacts/my-demo", `{"project":"/"}`),
		e.do(t, http.MethodPatch, "/api/artifacts/my-demo", `{}`, "Origin", e.origin, "Content-Type", "text/plain").status,
		patch("/api/artifacts/my-demo", `{"description":"`+strings.Repeat("x", 70000)+`"}`),
	}
	if got := toString(codes); got != "404 404 400 400 400 400 400 400 400 409 400 400 415 413" {
		t.Errorf("PATCH refusals = %s", got)
	}
	if r := e.send(t, http.MethodPatch, "/api/artifacts/my-demo", `{"slug":"renamed","title":"T","project":"p","favorite":true}`); r.body != `{"slug":"renamed"}` {
		t.Errorf("PATCH rename = %d %s", r.status, r.body)
	}
	if m := e.store.ReadMeta("renamed"); m == nil || m.Title != "T" || m.Project != "p" || !m.Favorite {
		t.Errorf("after the rename: %+v", m)
	}
	if r := e.send(t, http.MethodPatch, "/api/artifacts/my-demo", `"x"`); r.body != "cannot read the request: send a JSON object" {
		t.Errorf("a JSON string = %q", r.body)
	}
	ja := e.do(t, http.MethodPatch, "/api/artifacts/my-demo", `"x"`,
		"Origin", e.origin, "Content-Type", "application/json", "Accept-Language", "ja-JP,ja;q=0.9,en;q=0.8")
	if ja.body != "送られた内容を読めませんでした（JSON のオブジェクトで送ってください）" {
		t.Errorf("a JSON string to a Japanese reader = %q", ja.body)
	}
}

func toString(codes []int) string {
	s := make([]string, 0, len(codes))
	for _, c := range codes {
		s = append(s, strconv.Itoa(c))
	}
	return strings.Join(s, " ")
}

func TestDelete(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, e.page(t, "My Demo.yaml", "Demo Page", ""), store.Options{})
	port := strings.TrimPrefix(e.base, "http://127.0.0.1:")
	del := func(slug, origin string) int {
		return e.do(t, http.MethodDelete, "/api/artifacts/"+slug, "", "Origin", origin).status
	}
	got := toString([]int{
		del("my-demo", "http://evil.example"), del("my-demo", "null"), del("my-demo", "http://localhost:"+port),
		del("my-demo", "http://localhost:"+port), del("Bad_Slug", e.origin),
	})
	if got != "403 403 204 404 404" {
		t.Errorf("DELETE = %s", got)
	}
}

// TestLinkedFileSwappedForSymlink replaces a registered file with a symlink after registering it: the
// link is served from its real path only, so the page and its download are 404 and the list shows it
// as missing, titled by its slug.
func TestLinkedFileSwappedForSymlink(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	page := e.write(t, "swap/page.html", "<title>Real</title><p>real</p>")
	other := e.write(t, "swap/other.html", "<title>Other</title><p>other</p>")
	e.link(t, page, store.Options{Slug: ptr("swap")})
	if err := os.Remove(page); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, page); err != nil {
		t.Fatal(err)
	}
	if got := statuses(t, e, "/a/swap/", "/a/swap/?raw", "/api/artifacts/swap/download"); got != "404 404 404" {
		t.Errorf("a link swapped for a symlink = %s", got)
	}
	var list []store.Listed
	if err := json.Unmarshal([]byte(e.get(t, "/api/artifacts").body), &list); err != nil || len(list) != 1 ||
		list[0].Missing == nil || !*list[0].Missing || list[0].Title != "swap" {
		t.Errorf("the list after the swap = %+v, %v", list, err)
	}
}

// site links a directory with assets, dot paths, an escaping symlink and FIFOs.
func site(t *testing.T, e *env) string {
	t.Helper()

	index := e.write(t, "site/index.html", `<h1>Linked v1</h1><link rel="stylesheet" href="s.css">`+"\n")
	e.write(t, "site/s.css", "p{}")
	e.write(t, "site/sub/p.html", "<title>Sub</title><p>sub</p>\n")
	e.write(t, "site/a b#.html", "<title>Odd</title>")
	e.write(t, "site/.git/config", "secret")
	e.write(t, "site/sub/.git/config", "secret")
	e.write(t, "outside.txt", "outside")
	if err := os.Symlink(filepath.Join(e.tmp, "outside.txt"), filepath.Join(e.tmp, "site/escape.txt")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"site/pipe.txt", "site/pipe.html"} {
		if err := syscall.Mkfifo(filepath.Join(e.tmp, p), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e.link(t, index, store.Options{Slug: ptr("site")})
	return index
}

func ptr[T any](v T) *T {
	return &v
}

func TestLinkedFile(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	index := site(t, e)

	e.write(t, "site/index.html", "<h1>Linked v2</h1>\n")
	// Linked HTML is another tool's, which may load from elsewhere: it is not cut off.
	if r := e.get(t, "/a/site/?raw"); !strings.HasPrefix(r.body, "<h1>Linked v2</h1>\n\n<script>") ||
		r.header.Get("Content-Security-Policy") != sandbox {
		t.Errorf("a link is read on every request and served as it is: %v\n%s", r.header, r.body)
	}
	if got := e.get(t, "/api/artifacts/site/download").body; got != "<h1>Linked v2</h1>\n" {
		t.Errorf("a link's download = %q", got)
	}
	r := e.get(t, "/a/site/s.css")
	if r.header.Get("Content-Type") != "text/css; charset=utf-8" || r.header.Get("Content-Security-Policy") != sandbox ||
		r.header.Get("X-Content-Type-Options") != "nosniff" || r.body != "p{}" {
		t.Errorf("an asset: %v %q", r.header, r.body)
	}
	if r := e.get(t, "/a/site/sub/p.html"); !strings.Contains(r.body, "<title>Sub</title>") ||
		!strings.Contains(r.body, `new window.EventSource("/api/events/site/sub/p.html?since=`) || strings.Contains(r.body, "<title>floe</title>") {
		t.Errorf("an HTML page under the link: %s", r.body)
	}
	if r := e.get(t, "/a/site/a%20b%23.html"); !strings.Contains(r.body, `EventSource("/api/events/site/a%20b%23.html?since=`) {
		t.Errorf("an odd file name in the events URL: %s", r.body)
	}
	got := statuses(t, e, "/a/site/.git/config", "/a/site/sub%2F.git/config", "/a/site/sub%2f.git%2fconfig",
		"/a/site/%2e%2e/outside.txt", "/a/site/..%2Foutside.txt", "/a/site/escape.txt", "/a/site/sub/", "/a/site/sub",
		"/a/site/sub/../s.css", "/a/site//s.css", "/a/site/pipe.txt", "/a/site/nope.css")
	if got != strings.TrimSpace(strings.Repeat("404 ", 12)) {
		t.Errorf("paths that must not be served = %s", got)
	}
	if got := statuses(t, e, "/api/health"); got != "200" {
		t.Errorf("health after the FIFO = %s", got)
	}

	// Saving the linked file notifies its open pages.
	since := url.QueryEscape(e.store.Version("site", e.store.ReadMeta("site"), index))
	ch := make(chan string, 1)
	go func() {
		_, body := e.events(t, "/api/events/site?since="+since, 4*time.Second)
		ch <- body
	}()
	time.Sleep(300 * time.Millisecond)
	e.write(t, "site/index.html", "<h1>Linked v3</h1>\n")
	if err := os.Chtimes(index, time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if body := <-ch; !strings.Contains(body, "event: reload") {
		t.Errorf("saving the file did not notify: %q", body)
	}

	if dl := e.get(t, "/api/artifacts/site/download"); dl.body != "<h1>Linked v3</h1>\n" {
		t.Errorf("download of a link: %s", dl.body)
	}

	if err := os.Rename(index, index+".gone"); err != nil {
		t.Fatal(err)
	}
	if got := statuses(t, e, "/a/site/", "/a/site/?raw", "/api/artifacts/site/download"); got != "404 404 404" {
		t.Errorf("a gone link = %s", got)
	}
}

func TestLinkedDocumentBytes(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	doc := e.write(t, "doc/index.html", "<!doctype html><title>Doc</title><p>\xff</p>\n")
	e.link(t, doc, store.Options{Slug: ptr("doc-link")})
	if dl := e.get(t, "/api/artifacts/doc-link/download"); dl.body != "<!doctype html><title>Doc</title><p>\xff</p>\n" {
		t.Errorf("download of a linked document = %q", dl.body)
	}

	big := "<!doctype html><title>Big</title><p>" + strings.Repeat("x", 1<<20) + "\xff"
	e.write(t, "doc/index.html", big)
	if r := e.get(t, "/a/doc-link/?raw"); !strings.HasPrefix(r.body, big) ||
		!strings.HasPrefix(r.body[len(big):], "\n<script>{const es = new window.EventSource(") {
		t.Error("a large document is not streamed byte for byte with the reloader appended")
	}

	// A fragment is served as it is too, bytes that are not UTF-8 and all.
	frag := `<svg viewBox="0 0 1 1"><title>Chart</title></svg>` + "\n<h1>Frag</h1><p>\xff</p>\n"
	e.write(t, "doc/index.html", frag)
	if dl := e.get(t, "/api/artifacts/doc-link/download"); dl.body != frag {
		t.Errorf("download of a linked fragment = %q", dl.body)
	}
}

// TestHangUpClosesTheFile reads only the start of large responses and checks that the server closes
// the file. It counts this process's descriptors, so it needs /proc.
func TestHangUpClosesTheFile(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skip("no /proc")
	}
	e := newEnv(t)
	doc := e.write(t, "abort/index.html", "<!doctype html><title>Abort</title><p>"+strings.Repeat("x", 8<<20))
	e.link(t, doc, store.Options{Slug: ptr("abort")})
	for range 10 {
		for _, target := range []string{"/a/abort/?raw", "/api/artifacts/abort/download"} {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, e.base+target, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(res.Body, make([]byte, 10)); err != nil {
				t.Fatal(err)
			}
			_ = res.Body.Close() //nolint:errcheck // hanging up is the point
		}
	}
	open := -1
	for range 50 {
		open = countOpen(t, filepath.Dir(doc))
		if open == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if open != 0 {
		t.Errorf("%d descriptors of the file are left open", open)
	}
}

func countOpen(t *testing.T, dir string) int {
	t.Helper()

	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, fd := range fds {
		if target, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name())); err == nil && strings.HasPrefix(target, dir+"/") {
			n++
		}
	}
	return n
}

func TestAddLink(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	index := site(t, e)
	if err := e.store.Remove("site"); err != nil {
		t.Fatal(err)
	}
	post := func(body string, header ...string) resp {
		return e.do(t, http.MethodPost, "/api/artifacts", body, header...)
	}
	js := "Content-Type"
	body := `{"path":"` + index + `"}`
	got := toString([]int{
		post(body, "Origin", "http://evil.example", js, "application/json").status,
		post(body, "Origin", "null", js, "application/json").status,
		post(body, "Origin", e.origin, js, "text/plain").status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"site/index.html"}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+filepath.Join(e.tmp, "outside.txt")+`"}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+filepath.Join(e.tmp, "site/pipe.html")+`"}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":1}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{bad`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `"x"`).status,
	})
	if got != "403 403 415 400 400 400 400 400 400" {
		t.Errorf("refused adds = %s", got)
	}

	r := e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+index+`","description":"from ui"}`)
	if m := e.store.ReadMeta("site"); r.status != http.StatusCreated || r.body != `{"slug":"site","title":"","prev":null}` || m == nil ||
		m.Path != index || m.Description != "from ui" {
		t.Errorf("add = %d %s, %+v", r.status, r.body, m)
	}
	sub := filepath.Join(e.tmp, "site/sub/p.html")
	r = e.do(t, http.MethodPost, "/api/artifacts", `{"path":"`+sub+`","slug":"site"}`,
		"Origin", e.origin, "Content-Type", "application/json", "Accept-Language", "ja")
	if r.status != http.StatusConflict || !strings.Contains(r.body, "URL 名「site」は "+index+" のリンクで使われています") {
		t.Errorf("a URL name of another file = %d %s", r.status, r.body)
	}
	if r := e.send(t, http.MethodPost, "/api/artifacts", body); r.status != http.StatusOK {
		t.Errorf("re-adding the same path = %d %s", r.status, r.body)
	}
	if list, err := e.store.List(); err != nil || len(list) != 1 {
		t.Errorf("re-adding the same path made %d entries (%v), want it updated in place", len(list), err)
	}
	if r := e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+sub+`"}`); r.body != `{"slug":"p","title":"","prev":null}` {
		t.Errorf("an omitted slug = %s", r.body)
	}

	// A field sent is given, trimmed, empty or not, as floe publish --link sends them: an empty slug is
	// refused, a blank description clears it, and force replaces another file's link.
	got = toString([]int{
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+sub+`","slug":""}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+sub+`","title":3}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+sub+`","force":"yes"}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+index+`","description":" "}`).status,
		e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+sub+`","slug":" site ","title":"Sub","force":true}`).status,
	})
	if m := e.store.ReadMeta("site"); got != "400 400 400 200 200" || m == nil || m.Path != sub || m.Title != "Sub" {
		t.Errorf("fields sent = %s, %+v", got, m)
	}
	if m := e.store.ReadMeta("p"); m == nil {
		t.Error("the link p is gone")
	}
	// The path is taken as sent: a file name may start with a space.
	spaced := e.write(t, "site/ spaced.html", "<p>s</p>")
	if r := e.send(t, http.MethodPost, "/api/artifacts", `{"path":"`+spaced+`","slug":"spaced"}`); r.status != http.StatusCreated ||
		e.store.ReadMeta("spaced") == nil || e.store.ReadMeta("spaced").Path != spaced {
		t.Errorf("a path with a space = %d %s", r.status, r.body)
	}
}

// PUT /api/artifacts/{slug} stores the data file the CLI sends: it asks for the images the data names,
// tells the mistakes at their lines and columns as JSON to a reader that takes it, and says what it
// replaced.
func TestSave(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	if err := os.CopyFS(filepath.Join(e.tmp, "templates", "sizes"), os.DirFS("../tmpl/testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	png, err := os.ReadFile("../tmpl/testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("template: sizes\ntitle: Sizes\nlang: en\nfiles: [{ name: a, bytes: 1 }]\nchart: img/chart.png\n")
	put := func(slug string, body map[string]any, header ...string) resp {
		b, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return e.do(t, http.MethodPut, "/api/artifacts/"+slug, string(b),
			append([]string{"Origin", e.origin, "Content-Type", "application/json", "Accept", "application/json"}, header...)...)
	}
	upload := func(images map[string][]byte, project string) map[string]any {
		return map[string]any{"file": "plans/sizes.yaml", "dir": "/plans", "source": src, "images": images, "project": project}
	}

	got := toString([]int{
		e.do(t, http.MethodPut, "/api/artifacts/sizes", `{}`, "Origin", "null", "Content-Type", "application/json").status,
		e.do(t, http.MethodPut, "/api/artifacts/sizes", `{}`, "Origin", e.origin, "Content-Type", "text/plain").status,
		put("Not%20a%20slug", upload(nil, "proj")).status,
		put("sizes", map[string]any{"file": "plans/sizes.yaml", "dir": "/plans", "source": src, "extra": 1}).status,
		put("sizes", map[string]any{"file": "plans/sizes.yaml", "source": src}).status,
		put("sizes", upload(map[string][]byte{"../a.png": png}, "proj")).status,
	})
	if got != "403 415 400 400 400 400" {
		t.Errorf("refused publishes = %s", got)
	}

	if r := put("sizes", upload(nil, "proj")); r.status != http.StatusUnprocessableEntity || r.body != `{"images":["img/chart.png"]}` {
		t.Errorf("a publish without its images = %d %s", r.status, r.body)
	}
	r := put("sizes", upload(map[string][]byte{}, "proj"), "Accept-Language", "ja")
	if r.status != http.StatusBadRequest || !strings.Contains(r.body, `"mistakes":true`) ||
		!strings.Contains(r.body, `plans/sizes.yaml:5:8: chart: /plans の下に画像「img/chart.png」がありません`) {
		t.Errorf("a publish missing its image = %d %s", r.status, r.body)
	}
	images := map[string][]byte{"img/chart.png": png}
	if r = put("sizes", upload(images, "proj")); r.status != http.StatusCreated ||
		r.body != `{"slug":"sizes","title":"Sizes","prev":null}` {
		t.Errorf("a new publish = %d %s", r.status, r.body)
	}
	first := e.store.ReadMeta("sizes")
	if r = put("sizes", upload(images, "proj")); r.status != http.StatusOK ||
		r.body != `{"slug":"sizes","title":"Sizes","prev":{"title":"Sizes","updatedAt":"`+first.UpdatedAt+`"}}` {
		t.Errorf("a publish again = %d %s", r.status, r.body)
	}
	if !strings.Contains(e.get(t, "/a/sizes/?raw").body, `<img src="data:image/png;base64,`) {
		t.Error("the stored page does not show its image")
	}
	// A refusal that is not a data file's mistake, and one to a reader that does not take JSON.
	if r = put("sizes", upload(nil, "other")); r.status != http.StatusConflict || !strings.Contains(r.body, `"mistakes":false`) {
		t.Errorf("a publish over another project's page = %d %s", r.status, r.body)
	}
	b, err := json.Marshal(upload(nil, "other"))
	if err != nil {
		t.Fatal(err)
	}
	if r = e.send(t, http.MethodPut, "/api/artifacts/sizes", string(b)); r.status != http.StatusConflict ||
		!strings.HasPrefix(r.body, `slug "sizes" was published from project proj`) {
		t.Errorf("a refusal as text = %d %s", r.status, r.body)
	}
	// The options are trimmed, as those of every publish are.
	withTitle := upload(images, "proj")
	withTitle["title"] = " Plan "
	if r = put("sizes", withTitle); r.status != http.StatusOK || !strings.Contains(r.body, `"title":"Plan"`) {
		t.Errorf("a publish with a title to trim = %d %s", r.status, r.body)
	}
}
