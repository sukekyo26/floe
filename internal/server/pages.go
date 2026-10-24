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
	"strings"
	"time"

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
	// Only the management page itself and VS Code may frame it, so another site cannot lay it under its
	// own buttons and have its delete clicked (clickjacking). A page's frame moved to the management page
	// shows it nested. (A page that frames it frames it in the sandbox it hands down, whose API calls are
	// refused.) VS Code's Simple Browser frames it in a webview of the workbench, and frame-ancestors
	// checks every ancestor: those two. An extension that could frame it so runs on the machine anyway.
	uiCSP = "frame-ancestors 'self' vscode-webview: vscode-file:"
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

// servePage answers /a/{slug}/{rest...}. The page URL itself is the frame (the management page with
// the page's header), whose iframe loads the page from ?raw; a missing page gets the frame with 404,
// which shows the way back. ?raw and the files under a link's directory are the page and its assets.
// LIMIT: no Range support. Add it if videos need seeking.
func (s *Server) servePage(w http.ResponseWriter, r *http.Request) {
	slug, rest := r.PathValue("slug"), r.PathValue("rest")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	m := s.cfg.Store.ReadMeta(slug)
	if rest == "" && !r.URL.Query().Has("raw") {
		status := http.StatusNotFound
		if m != nil {
			if _, ok := s.cfg.Store.File(slug, m, ""); ok {
				status = http.StatusOK
			}
		}
		sendUI(w, r, status)
		return
	}
	if m == nil {
		notFound(w)
		return
	}
	file, ok := s.cfg.Store.File(slug, m, rest)
	if !ok {
		notFound(w)
		return
	}
	// Assets are sandboxed too: an SVG or HTML opened directly must not run in this origin.
	h := w.Header()
	csp := pageCSP
	if m.IsDrawn() {
		// The reload script's stream only, under the name the page was opened by: the management API is
		// on the same port.
		csp += "; " + tmpl.CutOff("http://"+r.Host+"/api/events/")
	}
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	if m.IsDrawn() {
		page, version, err := s.cfg.Store.Draw(slug, m, file)
		if err != nil {
			// Saving the file right, or publishing it again, reloads the tab back to the page.
			page = mistakesPage(catalog(r), slug, m, file, err)
		}
		stream(w, htmlType, bytes.NewReader(page), servedScript(slug, "", version))
		return
	}
	// The version is taken before the content: a change in between costs one extra reload, not a miss.
	version := s.cfg.Store.Version(slug, m, file)
	f, err := os.Open(file) //nolint:gosec // G304: file passed store.File
	if err != nil {
		notFound(w)
		return
	}
	defer f.Close() //nolint:errcheck // read-only
	contentType := htmlType
	if rest != "" {
		contentType = types[strings.ToLower(filepath.Ext(file))]
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	tail := ""
	if contentType == htmlType {
		tail = servedScript(slug, rest, version)
	}
	stream(w, contentType, f, tail)
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	m := s.cfg.Store.ReadMeta(slug)
	if m == nil {
		notFound(w)
		return
	}
	file, ok := s.cfg.Store.File(slug, m, "")
	if !ok {
		notFound(w)
		return
	}
	var body io.Reader
	if m.IsDrawn() {
		page, _, err := s.cfg.Store.Draw(slug, m, file)
		switch {
		case errors.Is(err, store.ErrNotFound):
			notFound(w)
			return
		case err != nil:
			refuse(w, r, err)
			return
		}
		body = tmpl.WithCutOff(page)
	} else {
		// Linked HTML, as it is.
		f, err := os.Open(file) //nolint:gosec // G304: file passed store.File
		if err != nil {
			notFound(w)
			return
		}
		defer f.Close() //nolint:errcheck // read-only
		body = f
	}
	// The page linked or drawn, a page drawn from a data file still cut off. Nothing added on serving
	// (the reload script) goes in.
	w.Header().Set("Content-Disposition", `attachment; filename="`+slug+`.html"`)
	stream(w, htmlType, body, "")
}

// stream sends f as it is, with tail after it: a data file's page is drawn by floe, and a linked file
// is the user's own. A read error after the header has gone out can only cut the
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

// leaveFrame runs in a page inside the frame. Links to other sites open in a new tab: most sites
// refuse to be framed, and the sandbox forbids _top and _parent. Links to a frame URL of this server
// (/ and /a/<slug>/) tell the frame to move the whole tab, instead of nesting a frame in the iframe;
// but one to a part (#…) of this page's own frame URL is a link to a part of the page, which the
// browser follows in the page as it does #… alone: a move to another part adds to the history, one to
// the same part scrolls there again. It is written whole, as a <base> of the page would take #… elsewhere.
// In the sandbox location.origin is "null", so hosts are compared. A page can shadow open, URL, self
// and the like with top-level declarations, so everything but window, top and location comes through
// window; top cannot be replaced, as parent can (var parent = …).
// The part of the page it is at (#…) is told to the frame, which writes it into its own URL: when it
// loads, as Back may load it again at another, and on a move to another part.
// LIMIT: a part set by history.pushState or replaceState fires no hashchange and is not told; watch them if needed.
// LIMIT: a frame framed itself (VS Code's Simple Browser) is not the top and is told nothing; mend it if needed.
const leaveFrame = "if (top !== window) window.addEventListener('click', (e) => {" +
	"const a = e.target.closest?.('a[href]');" +
	"if (!a || e.defaultPrevented || e.button || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;" +
	"if (!['', '_self', '_parent', '_top'].includes(a.target)) return;" +
	"const u = new window.URL(a.href, location.href);" +
	"if (!/^https?:$/.test(u.protocol)) return;" +
	"if (u.host !== location.host) { e.preventDefault(); window.open(u.href, '_blank', 'noopener'); return; }" +
	"if (!/^\\/(a\\/[^/]+\\/?)?$/.test(u.pathname) || u.searchParams.has('raw')) return;" +
	"e.preventDefault();" +
	"if (u.hash && !u.search && u.pathname.replace(/\\/?$/, '/') === location.pathname) {" +
	"location.href = location.pathname + location.search + u.hash; return; }" +
	"top.postMessage({ artifactOpen: u.pathname + u.search + u.hash }, " +
	"location.protocol + '//' + location.host);});" +
	"if (top !== window) { const tell = () => top.postMessage(" +
	"{ artifactHash: location.hash, artifactPath: location.pathname }, location.protocol + '//' + location.host);" +
	"tell(); window.addEventListener('hashchange', tell); }"

// servedScript is appended to a served page only: it reloads the open tab when the page is
// republished (a link: when its file is saved), and handles links inside the frame. The version the
// page was served from goes into since, so a change before the stream connects is not missed.
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
		"es.addEventListener('reload', () => { es.close(); location.reload(); });" + leaveFrame + "}</script>\n"
}

// watch is the event stream of a page. Pages subscribe from the sandbox (Origin: null), so CORS opens
// to null only; any sandboxed iframe of another site can claim null too, so the event says nothing but
// "changed".
// LIMIT: each stream stats meta.json and the file every second. Switch to fsnotify if dozens of tabs
// stay open.
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
	ticker := time.NewTicker(s.reloadEvery)
	defer ticker.Stop()
	for {
		if s.version(slug, rest) != since {
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

func (s *Server) version(slug, rest string) string {
	m := s.cfg.Store.ReadMeta(slug)
	if m == nil {
		return ""
	}
	file, _ := s.cfg.Store.File(slug, m, rest)
	return s.cfg.Store.Version(slug, m, file)
}

// mistakes is the page shown for a data file with mistakes, in the reader's language.
var mistakes = template.Must(template.New("mistakes").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head><meta charset="utf-8"><title>{{.Slug}}</title>
<style>
:root { color-scheme: light dark; --fg: #1f2328; --bg: #fff; --bad: #b42318; }
@media (prefers-color-scheme: dark) { :root { --fg: #e6edf3; --bg: #0d1117; --bad: #ff7b72; } }
body { margin: 2rem; font: 15px/1.6 system-ui, sans-serif; color: var(--fg); background: var(--bg); }
h1 { font-size: 1.25rem; color: var(--bad); }
pre { white-space: pre-wrap; overflow-wrap: anywhere; font: 13px/1.6 ui-monospace, monospace; }
</style></head>
<body><h1>{{.Head}}</h1><p>{{.Lead}}</p><pre>{{.Mistakes}}</pre></body>
</html>
`))

// mistakesPage words the mistakes of file, the data file of slug, whose meta.json is m: a linked one
// is put right by saving it, a stored one, a copy, by publishing the data file again.
func mistakesPage(c *i18n.Catalog, slug string, m *store.Meta, file string, err error) []byte {
	text := usererr.Tell(c, err).Render("")
	lead := c.Msg("server_mistakes_lead")
	switch {
	case m.IsLink():
	case errors.Is(err, store.ErrNotFound):
		lead = c.Msg("server_mistakes_lead_no_copy", slug)
	default:
		lead = c.Msg("server_mistakes_lead_stored", filepath.Dir(file))
	}
	var b bytes.Buffer
	//nolint:errcheck // a buffer takes every write, and the fields are text
	_ = mistakes.Execute(&b, map[string]string{
		"Lang": string(c.Lang()), "Slug": slug, "Head": c.Msg("server_mistakes_head"),
		"Lead": lead, "Mistakes": text,
	})
	return b.Bytes()
}
