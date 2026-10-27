package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sukekyo26/floe/internal/draw"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

const (
	htmlType = "text/html; charset=utf-8"
	// Pages run in an opaque origin, so their scripts cannot call the management API.
	// allow-popups-to-escape-sandbox: a site opened in a new tab does not inherit the sandbox (it has no
	// opener, so it cannot reach the API either).
	pageCSP = "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-forms allow-modals allow-downloads"
)

// types are the files served from a link's directory. The registered file itself is HTML whatever its
// extension.
var types = map[string]string{
	".html":  htmlType,
	".htm":   htmlType,
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".json":  "application/json",
	".map":   "application/json",
	".txt":   "text/plain; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".ttf":   "font/ttf",
	".wasm":  "application/wasm",
}

// errNoPage is a page, or a file under a link, that is not there to serve.
var errNoPage = errors.New("no such page")

// version is what an open tab of slug, of the file rest under a link ("" for the page), compares to tell that it
// changed, without drawing: this server's id with, for a stored page, its copy (a new one each publish and
// rebuild); for a linked file, its mark; for a linked data file, the generation of the drawing, its mark and its
// meta.json's updatedAt (a link made again, another theme). m is slug's meta.json as it was read for the content
// served with it, so that both are of one publish; "" when there is no page.
// LIMIT: each open tab reads meta.json and stats a file every second (version is all it does). Fold the versions
// into the server's watching if tabs come to be hundreds.
// LIMIT: a linked data file's images, and the files under a linked HTML file, are not in its version: changing
// only them reloads no tab (the user chose so). Save the data file again, or reload the tab.
func (s *Server) version(slug string, m *store.Meta, rest string) string {
	if m == nil {
		return ""
	}
	if !m.IsLink() {
		return s.id + "." + m.Source
	}
	file, ok := s.cfg.Store.File(slug, m, rest)
	if !ok {
		return ""
	}
	if rest == "" && files.IsDataFile(m.Path) {
		_, gen := s.now()
		return s.id + "." + strconv.Itoa(gen) + "." + files.Mark(file) + "." + m.UpdatedAt
	}
	return s.id + "." + files.Mark(file)
}

// content is what slug, whose meta.json is m, serves for rest ("" for the page itself): a stored page as it was
// drawn, a linked file as it is, a linked data file drawn now. A linked data file with mistakes (or a drawing
// that cannot be read) is its mistakes, its body the page of them in the language of c. drawn tells a page floe
// drew, whose CSP cuts it off. errNoPage for none.
// LIMIT: no Range support. Add it if videos need seeking.
func (s *Server) content(c *i18n.Catalog, slug string, m *store.Meta, rest string) (served, error) {
	none := served{body: nil, typ: "", drawn: false}
	file, ok := s.cfg.Store.File(slug, m, rest)
	if !ok {
		return none, errNoPage
	}
	if m.IsLink() && rest == "" && files.IsDataFile(m.Path) {
		page, err := s.drawLink(slug, m, file)
		if err != nil {
			return served{body: io.NopCloser(bytes.NewReader(mistakesPage(c, slug, err))), typ: htmlType, drawn: true}, err
		}
		return served{body: io.NopCloser(bytes.NewReader(page)), typ: htmlType, drawn: true}, nil
	}
	f, err := os.Open(file) //nolint:gosec // G304: file passed store.File
	if err != nil {
		return none, errNoPage
	}
	typ := htmlType
	if rest != "" {
		typ = types[strings.ToLower(filepath.Ext(file))]
		if typ == "" {
			typ = "application/octet-stream"
		}
	}
	return served{body: f, typ: typ, drawn: !m.IsLink()}, nil
}

// served is what content serves: body, of the media type typ, drawn by floe or not.
type served struct {
	body  io.ReadCloser
	typ   string
	drawn bool
}

// drawLink draws the linked data file file of slug, whose meta.json is m, with the drawing of now: in its
// theme, under slug for a page of no title, its images under its real directory.
func (s *Server) drawLink(slug string, m *store.Meta, file string) ([]byte, error) {
	drawing, _ := s.now()
	if drawing.Err != nil {
		return nil, drawing.Err
	}
	src, err := os.ReadFile(file) //nolint:gosec // G304: file passed store.File
	if err != nil {
		return nil, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	images := files.NewRoot(filepath.Dir(file))
	defer images.Close() //nolint:errcheck // only read from
	o := tmpl.Options{Theme: m.Theme, Untitled: slug, Images: images, ImagesDir: "", Icons: drawing.Icons}
	return tmpl.Draw(drawing.Kit, file, src, o) //nolint:wrapcheck // its mistakes, or what failed
}

// servePage answers /a/{slug}/{rest...}: the page, or a file under a link. Its version is taken from the
// meta.json its content is served by, before the content: a change in between costs one extra reload, not a miss.
// A stored page whose copy a publish removed after its meta.json was read is read once more.
func (s *Server) servePage(w http.ResponseWriter, r *http.Request) {
	slug, rest := r.PathValue("slug"), r.PathValue("rest")
	var version string
	var got served
	err := errNoPage
	for try := 0; try < 2 && errors.Is(err, errNoPage); try++ {
		m := s.cfg.Store.ReadMeta(slug)
		if m == nil {
			break
		}
		version = s.version(slug, m, rest)
		got, err = s.content(catalog(r), slug, m, rest)
	}
	if errors.Is(err, errNoPage) {
		notFound(w)
		return
	}
	defer got.body.Close() //nolint:errcheck // read-only
	// Assets are sandboxed too: an SVG or HTML opened directly must not run in this origin.
	h := w.Header()
	csp := pageCSP
	if got.drawn {
		// The reload script's stream only, under the name the page was opened by: the management API is
		// on the same port.
		csp += "; " + draw.CutOff("http://"+r.Host+"/api/events/")
	}
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	tail := ""
	if got.typ == htmlType {
		tail = servedScript(slug, rest, version)
	}
	stream(w, got.typ, got.body, tail)
}

// download answers the page of slug as a file, as it was linked or drawn, a page floe drew with its CSP in it
// (draw.CutOffPage), as no header goes with a file: never the page of a linked data file's mistakes, which are
// refused.
func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	m := s.cfg.Store.ReadMeta(slug)
	if m == nil {
		notFound(w)
		return
	}
	got, err := s.content(catalog(r), slug, m, "")
	switch {
	case errors.Is(err, errNoPage):
		notFound(w)
		return
	case err != nil:
		_ = got.body.Close() //nolint:errcheck // read-only
		refuse(w, r, err)
		return
	}
	defer got.body.Close() //nolint:errcheck // read-only
	body := io.Reader(got.body)
	if got.drawn {
		page, err := io.ReadAll(got.body)
		if err == nil {
			page, err = draw.CutOffPage(page)
		}
		if err != nil {
			refuse(w, r, err)
			return
		}
		body = bytes.NewReader(page)
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+slug+`.html"`)
	stream(w, htmlType, body, "")
}

// stream sends f as it is, with tail after it. A read error after the header has gone out can only cut the
// response short.
func stream(w http.ResponseWriter, contentType string, f io.Reader, tail string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, f); err != nil {
		return
	}
	//nolint:errcheck,gosec // the client hung up; G705: tail is the script floe builds
	_, _ = io.WriteString(w, tail)
}

// servedScript is appended to a served page only: it reloads the open tab when the page changes (a publish, a
// rebuild, a linked file saved). The version the page was served from goes into since, so a change before the
// stream connects is not missed.
func servedScript(slug, rest, version string) string {
	u := "/api/events/" + slug
	if rest != "" {
		parts := strings.Split(rest, "/")
		for i, p := range parts {
			parts[i] = url.PathEscape(p)
		}
		u += "/" + strings.Join(parts, "/")
	}
	lit, _ := json.Marshal(u + "?since=" + url.QueryEscape(version)) //nolint:errcheck // a string always encodes
	// The stream is closed before reloading; a slow reload would otherwise get the event again.
	return "\n<script>{const es = new window.EventSource(" + string(lit) + ");" +
		"es.addEventListener('reload', () => { es.close(); location.reload(); });}</script>\n"
}

// watch is the event stream of a page. Pages subscribe from the sandbox (Origin: null), so CORS opens
// to null only; any sandboxed iframe of another site can claim null too, so the event says nothing but
// "changed". It compares versions, and draws nothing.
func (s *Server) watch(w http.ResponseWriter, r *http.Request) {
	slug, rest := r.PathValue("slug"), r.PathValue("rest")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	since := r.URL.Query().Get("since")
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("Access-Control-Allow-Origin", "null")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	if _, err := io.WriteString(w, "retry: 1000\n\n"); err != nil || rc.Flush() != nil {
		return
	}
	ticker := time.NewTicker(s.every)
	defer ticker.Stop()
	for {
		if s.version(slug, s.cfg.Store.ReadMeta(slug), rest) != since {
			_, _ = io.WriteString(w, "event: reload\ndata: changed\n\n") //nolint:errcheck // closing anyway
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

// mistakes is the page shown for a linked data file with mistakes, in the reader's language.
var mistakes = template.Must(template.New("mistakes").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><title>{{.Slug}}</title>
<style>
:root { --fg: #1f2328; --bg: #fff; --bad: #b42318; }
body { margin: 2rem; font: 15px/1.6 system-ui, sans-serif; color: var(--fg); background: var(--bg); }
h1 { font-size: 1.25rem; color: var(--bad); }
pre { white-space: pre-wrap; overflow-wrap: anywhere; font: 13px/1.6 ui-monospace, monospace; }
</style></head>
<body><h1>{{.Head}}</h1><p>{{.Lead}}</p><pre>{{.Mistakes}}</pre></body>
</html>
`))

// mistakesPage words err, the mistakes of the linked data file of slug, in the language of c: they are put right
// by saving it.
func mistakesPage(c *i18n.Catalog, slug string, err error) []byte {
	var b bytes.Buffer
	//nolint:errcheck // a buffer takes every write, and the fields are text
	_ = mistakes.Execute(&b, map[string]string{
		"Lang": string(c.Lang()), "Slug": slug, "Head": c.Msg("server_mistakes_head"),
		"Lead": c.Msg("server_mistakes_lead"), "Mistakes": usererr.Tell(c, err).Render(""),
	})
	return b.Bytes()
}
