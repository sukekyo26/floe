// Package server serves the pages and the API on 127.0.0.1: the pages stored as they were drawn when published,
// linked files as they are, and linked data files drawn each time they are opened.
package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/ui"
	"github.com/sukekyo26/floe/internal/usererr"
)

// App is the name /api/health answers with, so that a client can tell floe from another program.
const App = "floe"

// Config is what a server serves.
type Config struct {
	Store *store.Store
	Port  int
	// Build identifies the running binary; a publish from another build restarts the server.
	Build string
	// Drawing reads what pages are drawn with: one's own parts, themes and templates over floe's, and the icons.
	// The server reads it once, and again when Parts, Themes, Templates or Icons change.
	Drawing func() (store.Drawing, error)
	// Templates, Parts and Themes are the directories of one's own, and Icons that of the icon sets ("" for none):
	// what the server draws with, which /api/health tells and the server watches.
	Templates, Parts, Themes, Icons string
	// Home is the user's home directory, which bounds what may be linked.
	Home string
}

// Server handles the requests.
type Server struct {
	cfg     Config
	hosts   map[string]bool
	origins map[string]bool
	// every is how often the drawing is watched and an event stream compares a page's version.
	every time.Duration
	// id is this server's, new each start: in each version, so that a tab open across a restart reloads.
	id string

	// looking lets one look run at a time (look).
	looking sync.Mutex
	mu      sync.Mutex
	drawing store.Drawing
	gen     int    // how many times the drawing has been read
	mark    string // what the directories it was read from were then
	// fresh is whether the drawing is of mark: false before it is read, and after floe's failure, to read it again.
	fresh bool
}

// New returns a server for cfg, the drawing read.
func New(cfg Config) *Server {
	s := &Server{
		cfg: cfg, hosts: map[string]bool{}, origins: map[string]bool{}, every: time.Second, id: rand.Text()[:8],
		looking: sync.Mutex{}, mu: sync.Mutex{}, drawing: store.Drawing{Kit: nil, Icons: nil, Err: nil}, gen: 0, mark: "", fresh: false,
	}
	// Requests under another name are refused against DNS rebinding.
	for _, h := range []string{"localhost", "127.0.0.1"} {
		host := net.JoinHostPort(h, strconv.Itoa(cfg.Port))
		s.hosts[host] = true
		s.origins["http://"+host] = true
	}
	s.look()
	return s
}

// look reads the drawing again when the directories it is read from changed since it was read, or was never read.
// Their mark is taken before the drawing is read: a change while it is read is told by the next look. One runs at a
// time: a request's look waits for the one running, so that the drawing it takes after is of no older a mark than
// its own. Without that, the watch's look of the mark before a save could write its drawing over the request's, and
// the request draw with the old one (the lock is not for the writes, which are under mu).
func (s *Server) look() {
	s.looking.Lock()
	defer s.looking.Unlock()
	m := s.marks()
	s.mu.Lock()
	same := s.fresh && m == s.mark
	s.mu.Unlock()
	if same {
		return
	}
	d, err := s.cfg.Drawing()
	if err != nil {
		d = store.Drawing{Kit: nil, Icons: nil, Err: err}
	}
	// A failure of floe's (too many files open) is not of what it read: read it again next time.
	var mistakes usererr.List
	fresh := err == nil || mistakes.Add(err) == nil
	s.mu.Lock()
	s.drawing, s.mark, s.fresh = d, m, fresh
	s.gen++
	s.mu.Unlock()
}

// now returns the drawing and its generation, taken together: the drawing of the last look. A request that draws
// to store uses latest; now only draws that the watch mends after (a page drawn when opened).
func (s *Server) now() (store.Drawing, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.drawing, s.gen
}

// latest is the drawing of what the directories hold now: looked at first, for a request that draws to store,
// which the watch would mend only after it is stored.
// LIMIT: the wait for a running look does not see the request's context end, so that a request gone waits behind
// the watch's read; watch the context too if cfg.Drawing comes to be slow.
func (s *Server) latest() store.Drawing {
	s.look()
	d, _ := s.now()
	return d
}

// Handler returns the routes behind the request checks.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", serveUI)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/artifacts", s.list)
	mux.HandleFunc("POST /api/artifacts", s.addLink)
	mux.HandleFunc("PUT /api/artifacts/{slug}", s.save)
	mux.HandleFunc("PATCH /api/artifacts/{slug}", s.edit)
	mux.HandleFunc("DELETE /api/artifacts/{slug}", s.remove)
	mux.HandleFunc("POST /api/rebuild", s.rebuild)
	mux.HandleFunc("GET /api/artifacts/{slug}/download", s.download)
	mux.HandleFunc("GET /api/events/{slug}", s.watch)
	mux.HandleFunc("GET /api/events/{slug}/{rest...}", s.watch)
	mux.HandleFunc("GET /a/{slug}", s.redirect)
	// /a/{slug}/ and /a/{slug}/{rest...} would both match /a/x/, so one route takes both.
	mux.HandleFunc("GET /a/{slug}/{rest...}", s.servePage)
	// Everything else, other methods included, is 404 rather than ServeMux's 405.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { notFound(w) })
	return s.check(mux)
}

// check refuses what no route should see. A malformed request target (//) is a client error. A path
// that is not clean (.., // or an encoded dot segment after decoding) is 404 here, because ServeMux
// would redirect it to its cleaned form: path safety is link.Resolve's job, not the router's.
func (s *Server) check(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hosts[r.Host] {
			send(w, http.StatusForbidden, "forbidden host")
			return
		}
		if strings.HasPrefix(r.RequestURI, "//") || !strings.HasPrefix(r.URL.Path, "/") {
			send(w, http.StatusBadRequest, "bad request")
			return
		}
		if r.URL.Path != cleanPath(r.URL.Path) {
			notFound(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cleanPath(p string) string {
	clean := path.Clean(p)
	if strings.HasSuffix(p, "/") && clean != "/" {
		clean += "/"
	}
	return clean
}

// protocolText is the text of a refusal of the protocol, fixed and in plain English (forbidden origin, not
// found), which neither the management page nor the CLI sends when right. The user's mistakes and floe's
// failures go through refuse instead: a text made at run time takes a conversion to be sent with send.
type protocolText string

const textType = "text/plain; charset=utf-8"

// send answers body as text: a refusal of the protocol, or an empty body. It is the one way in for a refusal
// as text; refuse and sendJSON's failure write theirs with sendBytes.
func send(w http.ResponseWriter, status int, body protocolText) {
	sendBytes(w, status, textType, []byte(body))
}

func sendBytes(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	//nolint:errcheck,gosec // the client hung up; G705: a page is served in the sandbox (pageCSP)
	_, _ = w.Write(body)
}

func sendJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		// Not through refuse, which answers with sendJSON.
		sendBytes(w, http.StatusInternalServerError, textType, []byte(err.Error()))
		return
	}
	sendBytes(w, status, "application/json", b)
}

func notFound(w http.ResponseWriter) {
	send(w, http.StatusNotFound, "not found")
}

func serveUI(w http.ResponseWriter, r *http.Request) {
	sendUI(w, r, http.StatusOK)
}

// sendUI answers with the management page in the reader's language.
func sendUI(w http.ResponseWriter, r *http.Request, status int) {
	page, err := ui.Page(catalog(r))
	if err != nil {
		refuse(w, r, err)
		return
	}
	w.Header().Set("Content-Security-Policy", uiCSP)
	sendBytes(w, status, htmlType, page)
}

// catalog is the language of the reader of a response, from Accept-Language.
func catalog(r *http.Request) *i18n.Catalog {
	return i18n.New(i18n.FromAcceptLanguage(r.Header.Get("Accept-Language")))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	sendJSON(w, http.StatusOK, Health{
		App: App, PID: os.Getpid(), Root: s.cfg.Store.Root(), Build: s.cfg.Build, Templates: s.cfg.Templates,
		Parts: s.cfg.Parts, Themes: s.cfg.Themes, Icons: s.cfg.Icons,
	})
}

// Health is the answer of /api/health.
type Health struct {
	App   string `json:"app"`
	PID   int    `json:"pid"`
	Root  string `json:"root"`
	Build string `json:"build"`
	// Templates, Parts, Themes and Icons are the directories of templates, parts and themes of one's own and
	// of icons it draws pages with.
	Templates string `json:"templates"`
	Parts     string `json:"parts"`
	Themes    string `json:"themes"`
	Icons     string `json:"icons"`
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	list, err := s.cfg.Store.List()
	if err != nil {
		refuse(w, r, err)
		return
	}
	sendJSON(w, http.StatusOK, list)
}

func (*Server) redirect(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !store.ValidSlug(slug) {
		notFound(w)
		return
	}
	// The query kept: /a/x?raw is the page, not its frame.
	to := "/a/" + slug + "/"
	if r.URL.RawQuery != "" {
		to += "?" + r.URL.RawQuery
	}
	w.Header().Set("Location", to)
	send(w, http.StatusMovedPermanently, "")
}

// Serve serves on ln until ctx is done, watching the drawing every s.every until then.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	// The watching ends however serving does.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go s.watchDrawing(ctx)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	if err := srv.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// watchDrawing looks at the drawing every s.every until ctx is done.
func (s *Server) watchDrawing(ctx context.Context) {
	ticker := time.NewTicker(s.every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.look()
		}
	}
}
