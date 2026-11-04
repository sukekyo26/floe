package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/url"
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

// errNoPage is a page, or a file under a link, that is not there to serve.
var errNoPage = errors.New("no such page")

// version is what an open tab of slug, of the file rest under a link ("" for the page), compares to tell that it
// changed, without drawing: this server's id with, for a stored page, its copy (a new one each publish and
// rebuild), or of its version v (versionParam, 0 for none) the store's mark of it; for a linked file, its mark; for a
// linked data file, the generation of the drawing, its mark and its meta.json's updatedAt (a link made again, another
// theme). m is slug's meta.json as it was read for the content served with it, so that both are of one publish; ""
// when there is no page, or no version v.
// LIMIT: each tab shown reads meta.json and stats a file every second it asks (version is all it does). Fold the
// versions into the server's watching if tabs come to be hundreds.
// LIMIT: a linked data file's images, and the files under a linked HTML file, are not in its version: changing
// only them reloads no tab (the user chose so). Save the data file again, or reload the tab.
func (s *Server) version(slug string, m *store.Meta, rest string, v int) string {
	if m == nil {
		return ""
	}
	if v > 0 {
		if mark := s.cfg.Store.VersionMark(slug, m, v); mark != "" {
			return s.id + "." + mark
		}
		return ""
	}
	if !m.IsLink() {
		return s.id + "." + m.Source
	}
	file, ok := s.cfg.Store.File(slug, m, rest)
	if !ok {
		return ""
	}
	if rest == "" && m.IsData() {
		_, gen := s.now()
		return s.id + "." + strconv.Itoa(gen) + "." + files.Mark(file) + "." + m.UpdatedAt
	}
	return s.id + "." + files.Mark(file)
}

// content is what slug, whose meta.json is m, serves for rest ("" for the page itself): a stored page as it was
// drawn (of its version v, which version told is there, when v is not 0), a linked file as it is, a linked data file
// drawn now. A linked data file with mistakes (or a drawing
// that cannot be read) is its mistakes, its body the page of them in the language of c, drawn with what drawing
// returns. drawn tells a page floe drew, whose CSP cuts it off. errNoPage for none.
// LIMIT: no Range support. Add it if videos need seeking.
func (s *Server) content(
	c *i18n.Catalog, slug string, m *store.Meta, rest string, v int, drawing func() store.Drawing,
) (served, error) {
	none := served{body: nil, typ: "", drawn: false}
	if v > 0 {
		page, ok := s.cfg.Store.VersionPage(slug, m, v)
		if !ok {
			return none, errNoPage
		}
		return served{body: io.NopCloser(bytes.NewReader(page)), typ: htmlType, drawn: true}, nil
	}
	file, ok := s.cfg.Store.File(slug, m, rest)
	if !ok {
		return none, errNoPage
	}
	if rest == "" && m.IsData() {
		page, err := drawLink(slug, m, file, drawing())
		if err != nil {
			return served{body: io.NopCloser(bytes.NewReader(mistakesPage(c, slug, err))), typ: htmlType, drawn: true}, err
		}
		return served{body: io.NopCloser(bytes.NewReader(page)), typ: htmlType, drawn: true}, nil
	}
	f, _, err := files.Open(file)
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

// versionParam is the version ?v= of q asks of the page whose meta.json is m, at rest: 0 for none, and false for one
// that cannot be a version's number. Only a stored page's own URL (rest "", which serves the page with ?raw only)
// reads it: under a link, ?v= is a file's own (style.css?v=3, against a cache), and a slug of no page asks none.
func versionParam(m *store.Meta, rest string, q url.Values) (int, bool) {
	if m == nil || m.IsLink() || rest != "" || !q.Has("v") {
		return 0, true
	}
	raw := q.Get("v")
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || strconv.Itoa(n) != raw {
		return 0, false
	}
	return n, true
}

// served is what content serves: body, of the media type typ, drawn by floe or not.
type served struct {
	body  io.ReadCloser
	typ   string
	drawn bool
}

// drawLink draws the linked data file file of slug, whose meta.json is m, with drawing: in its theme, under slug
// for a page of no title, its images under its real directory.
func drawLink(slug string, m *store.Meta, file string, drawing store.Drawing) ([]byte, error) {
	if drawing.Err != nil {
		return nil, drawing.Err
	}
	src, _, err := files.Read(file)
	if err != nil {
		return nil, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	images := files.NewRoot(filepath.Dir(file))
	defer images.Close() //nolint:errcheck // only read from
	o := tmpl.Options{Theme: m.Theme, Untitled: slug, Images: images, ImagesDir: "", Icons: drawing.Icons}
	// Its warnings are the writer's, whom publish and check tell; one who opens the page cannot mend them.
	page, _, err := tmpl.Draw(drawing.Kit, file, src, o)
	return page, err //nolint:wrapcheck // its mistakes, or what failed
}

// servePage answers /a/{slug}/{rest...}. The page URL itself is the frame (the management page with the page's
// header), whose iframe loads the page from ?raw; a missing page gets the frame with 404, which shows the way back.
// A folder's page at file is framed at ?file=, its iframe loading file under the folder; a folder's URL itself
// leads to the list with the folder open.
// ?raw and the files under a link's directory are the page and its files, ?raw&v=<n> a stored page's version n. Its
// version, and of a stored page the version it tells its frame it served, are taken from the meta.json
// its content is served by, before the content: a change in between costs one extra reload, not a miss. A stored
// page whose copy a publish removed after its meta.json was read is read once more.
func (s *Server) servePage(w http.ResponseWriter, r *http.Request) {
	slug, rest := r.PathValue("slug"), r.PathValue("rest")
	q := r.URL.Query()
	if rest == "" && !q.Has("raw") {
		s.frame(w, r, slug)
		return
	}
	var version string
	var read *Read
	var got served
	var v int
	err := errNoPage
	for try := 0; try < 2 && errors.Is(err, errNoPage); try++ {
		m := s.cfg.Store.ReadMeta(slug)
		var ok bool
		if v, ok = versionParam(m, rest, q); m == nil || !ok {
			break
		}
		version = s.version(slug, m, rest, v)
		if v > 0 && version == "" {
			break
		}
		read = nil
		if rest == "" {
			if mark, ok := s.cfg.Store.ReadMark(slug, m, v); ok {
				read = &Read{N: mark.N, At: mark.At}
			}
		}
		// Of the last look, as the version is: one changed since reloads the tab.
		got, err = s.content(catalog(r), slug, m, rest, v, func() store.Drawing { d, _ := s.now(); return d })
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
		// The reload script's asking only, under the name the page was opened by: the management API is
		// on the same port.
		csp += "; " + draw.CutOff("http://"+r.Host+"/api/changed/")
	}
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	tail := ""
	if got.typ == htmlType {
		tail = servedScript(slug, rest, version, v, read, got.drawn)
	}
	stream(w, got.typ, got.body, tail)
}

// frame answers the frame of slug, 404 when there is no page to frame. A folder has no page of its own: its URL is the
// list with it open, where publish's URL leads.
func (s *Server) frame(w http.ResponseWriter, r *http.Request, slug string) {
	status := http.StatusNotFound
	if m := s.cfg.Store.ReadMeta(slug); m != nil {
		file := r.URL.Query().Get("file")
		if m.Folder && file == "" {
			to := "/?kind=folder&open=" + url.QueryEscape(slug)
			if m.Archived {
				to += "&view=archived"
			}
			http.Redirect(w, r, to, http.StatusSeeOther)
			return
		}
		if _, ok := s.cfg.Store.File(slug, m, file); ok && (file == "" || m.Folder) {
			status = http.StatusOK
		}
	}
	sendUI(w, r, status, slug)
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
	// A file kept is not mended by the watch: of what the directories hold now.
	got, err := s.content(catalog(r), slug, m, "", 0, s.latest)
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

// leaveFrame runs in a page inside the frame. Links to other sites open in a new tab: most sites
// refuse to be framed, and the sandbox forbids _top and _parent. Links to a frame URL of this server
// (/ and /a/<slug>/) tell the frame to move the whole tab, instead of nesting a frame in the iframe;
// but one to a part (#…, # alone too) of this page's own frame URL is a link to a part of the page, which the
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
	"if (u.href.includes('#') && !u.search && u.pathname.replace(/\\/?$/, '/') === location.pathname) {" +
	"location.href = location.pathname + location.search + (u.hash || '#'); return; }" +
	"top.postMessage({ artifactOpen: u.pathname + u.search + u.hash }, " +
	"location.protocol + '//' + location.host);});" +
	"if (top !== window) { const tell = () => top.postMessage(" +
	"{ artifactHash: location.hash, artifactPath: location.pathname }, location.protocol + '//' + location.host);" +
	"tell(); window.addEventListener('hashchange', tell); }"

// tellRead tells the frame the version of a stored page it was served, read as soon as it is loaded. It goes to parent,
// the frame the iframe is in even when that is framed itself (VS Code's Simple Browser), of this origin only, so that a
// site that frames ?raw is told nothing. The frame takes it from its own iframe, at the URL it opened.
// LIMIT: a page counts as read as soon as it is loaded, read or not. Wait for a look at it if that comes to mislead.
// LIMIT: a page that replaces window.parent tells nothing, and a page framed can claim any version of its own, or a
// value of the right shape the server refuses (n: 1e30), which shows its frame a mistake each time: only its own unread
// and its own frame suffer from what the sandbox lets it do.
func tellRead(read *Read) string {
	if read == nil {
		return ""
	}
	// Marshal writes no < or >: it stays within the script.
	lit, _ := json.Marshal(read) //nolint:errcheck // a number and a string always encode
	return "if (window.parent !== window) window.parent.postMessage({ artifactRead: " + string(lit) +
		", artifactPath: location.pathname }, location.protocol + '//' + location.host);"
}

// tellKeys hands the frame the keys pressed in the page that move on or open a modal, so that they work with the focus
// in it: no key that changes an entry, and not Escape, which would keep a dialog of the page from closing. A page floe
// did not draw hands over Ctrl+K (⌘K) alone, as it may use the letters itself without preventDefault (a coverage
// report's j and k). Nothing is taken until the frame says it takes them, of this origin, so that a page in another
// iframe keeps the browser's / and Ctrl+K. It listens on window, after the page's own. Shift is not looked at, as ? is
// typed with it on most keyboards; Ctrl+Shift+K has the key K. A field is the one to type in; a checkbox and a radio
// (the part tabs) are not.
// LIMIT: a field inside a shadow root is seen as its host, and hands the keys over.
// LIMIT: a page floe drew with a script of its own that takes these keys without preventDefault does both.
func tellKeys(drawn bool) string {
	letters := "[]"
	if drawn {
		letters = "['j', 'k', 'h', '?', '/']"
	}
	return "{const here = location.protocol + '//' + location.host; let wanted = false;" +
		"window.addEventListener('message', (e) => { if (e.source === window.parent && e.origin === here " +
		"&& e.data?.artifactKeysWanted === true) wanted = true; });" +
		"window.addEventListener('keydown', (e) => {" +
		"if (!wanted || e.defaultPrevented || e.isComposing || e.keyCode === 229 || e.altKey) return;" +
		"const on = e.target;" +
		"if (on.closest?.('input:not([type=\"checkbox\"], [type=\"radio\"]), textarea, select') " +
		"|| on.isContentEditable || window.document.designMode === 'on') return;" +
		"const key = e.ctrlKey || e.metaKey ? (e.key === 'k' ? 'Ctrl+K' : '') : " +
		"(" + letters + ".includes(e.key) ? e.key : '');" +
		"if (!key) return; e.preventDefault(); window.parent.postMessage({ artifactKey: key }, here); });}"
}

// askEvery is how often a page shown asks whether it changed: about as often as the drawing is watched, and set
// apart from it so that the page's asking does not follow a test's shorter watching.
const askEvery = time.Second

// servedScript is appended to a served page only: it reloads the open tab when the page changes (a publish, a
// rebuild, a linked file saved), and handles links inside the frame. The page holds no connection to the server,
// as Chrome opens six at most to one server and an open tab holding one would make the next pages wait: while it is
// shown, it asks every askEvery whether the version it was served from (since) is still the page's, each ask cut off
// after 5 s so that one left hanging (a connection half open after sleep) does not stop the asking, and once more
// when it is shown again. A tab not shown asks nothing, and reloads when shown.
// A version v of a stored page (0 for none) asks of that version. read is what it tells the frame it served (tellRead).
// drawn tells a page floe drew (tellKeys).
// LIMIT: that a tab not shown asks nothing is not checked in a browser: headless Chrome showed no tab hidden. Were it
// broken, a tab not shown asks every second, holding nothing.
func servedScript(slug, rest, version string, v int, read *Read, drawn bool) string {
	u := "/api/changed/" + slug
	if rest != "" {
		parts := strings.Split(rest, "/")
		for i, p := range parts {
			parts[i] = url.PathEscape(p)
		}
		u += "/" + strings.Join(parts, "/")
	}
	u += "?since=" + url.QueryEscape(version)
	if v > 0 {
		u += "&v=" + strconv.Itoa(v)
	}
	lit, _ := json.Marshal(u) //nolint:errcheck // a string always encodes
	// What a page could shadow is read through window, as in leaveFrame. busy stays set once reloading.
	return "\n<script>{const u = " + string(lit) + "; let busy = false;" +
		"const look = async () => { if (busy || window.document.hidden) return; busy = true;" +
		"try { const r = await window.fetch(u, { cache: 'no-store', signal: window.AbortSignal.timeout(5000) });" +
		"if ((await r.text()) === 'changed') { location.reload(); return; } } catch {}" +
		"busy = false; };" +
		"window.setInterval(look, " + strconv.FormatInt(askEvery.Milliseconds(), 10) + ");" +
		"window.document.addEventListener('visibilitychange', look); look();" +
		leaveFrame + tellRead(read) + tellKeys(drawn) + "}</script>\n"
}

// changed answers whether the page of slug (of rest under a link) is no longer of the version since: "changed" or
// "same". Pages ask from the sandbox (Origin null), so CORS opens to null only; any sandboxed iframe of another site
// can claim null too, so it says nothing but that. No page is "changed" whatever since is: a version holds this
// server's id, new each start, so "same" comes only to a page served, and whether a slug is there is not told. It
// compares versions, and draws nothing.
func (s *Server) changed(w http.ResponseWriter, r *http.Request) {
	slug, rest := r.PathValue("slug"), r.PathValue("rest")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	answer := "same"
	q := r.URL.Query()
	m := s.cfg.Store.ReadMeta(slug)
	if v, ok := versionParam(m, rest, q); !ok {
		answer = "changed"
	} else if version := s.version(slug, m, rest, v); version == "" || version != q.Get("since") {
		answer = "changed"
	}
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Access-Control-Allow-Origin", "null")
	_, _ = io.WriteString(w, answer) //nolint:errcheck // the page asks again
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
