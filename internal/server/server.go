// Package server serves the management page, the pages and the API on 127.0.0.1.
package server

import (
	"context"
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
)

// App is the name /api/health answers with, so that a client can tell floe from another program.
const App = "floe"

// Config is what a server serves and how long it stays without requests.
type Config struct {
	Store *store.Store
	Port  int
	// Build identifies the running binary; a publish from another build restarts the server.
	Build string
	Idle  time.Duration
}

// Server handles the requests.
type Server struct {
	cfg     Config
	hosts   map[string]bool
	origins map[string]bool
	// reloadEvery is how often an event stream compares the version.
	reloadEvery time.Duration
}

// New returns a server for cfg.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, hosts: map[string]bool{}, origins: map[string]bool{}, reloadEvery: time.Second}
	// Requests under another name are refused against DNS rebinding.
	for _, h := range []string{"localhost", "127.0.0.1"} {
		host := net.JoinHostPort(h, strconv.Itoa(cfg.Port))
		s.hosts[host] = true
		s.origins["http://"+host] = true
	}
	return s
}

// Handler returns the routes behind the request checks.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.serveUI)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/artifacts", s.list)
	mux.HandleFunc("POST /api/artifacts", s.addLink)
	mux.HandleFunc("PATCH /api/artifacts/{slug}", s.edit)
	mux.HandleFunc("DELETE /api/artifacts/{slug}", s.remove)
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

func send(w http.ResponseWriter, status int, body string) {
	sendBytes(w, status, "text/plain; charset=utf-8", []byte(body))
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
		send(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendBytes(w, status, "application/json", b)
}

func notFound(w http.ResponseWriter) {
	send(w, http.StatusNotFound, "not found")
}

func (*Server) serveUI(w http.ResponseWriter, r *http.Request) {
	sendUI(w, r, http.StatusOK)
}

// sendUI answers with the management page in the reader's language.
func sendUI(w http.ResponseWriter, r *http.Request, status int) {
	page, err := ui.Page(catalog(r))
	if err != nil {
		send(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendBytes(w, status, htmlType, page)
}

// catalog is the language of the reader of a response, from Accept-Language.
func catalog(r *http.Request) *i18n.Catalog {
	return i18n.New(i18n.FromAcceptLanguage(r.Header.Get("Accept-Language")))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	sendJSON(w, http.StatusOK, Health{App: App, PID: os.Getpid(), Root: s.cfg.Store.Root(), Build: s.cfg.Build})
}

// Health is the answer of /api/health.
type Health struct {
	App   string `json:"app"`
	PID   int    `json:"pid"`
	Root  string `json:"root"`
	Build string `json:"build"`
}

func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	list, err := s.cfg.Store.List()
	if err != nil {
		send(w, http.StatusInternalServerError, err.Error())
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
	w.Header().Set("Location", "/a/"+slug+"/")
	send(w, http.StatusMovedPermanently, "")
}

// Serve serves on ln until ctx is done or no request has been in flight for the idle time; an open
// page holds its event stream, and a shown management page polls the list, so neither lets it stop.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	idle := newIdle(s.cfg.Idle)
	srv := &http.Server{
		Handler:           idle.wrap(s.Handler()),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	case <-idle.done:
	}
	if err := srv.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// idle closes done once no request has been in flight for d.
type idle struct {
	d      time.Duration
	mu     sync.Mutex
	active int
	timer  *time.Timer
	done   chan struct{}
	once   sync.Once
}

func newIdle(d time.Duration) *idle {
	i := &idle{d: d, mu: sync.Mutex{}, active: 0, timer: nil, done: make(chan struct{}), once: sync.Once{}}
	// A request racing the timer may reset it after it fired, so done is closed once.
	i.timer = time.AfterFunc(d, func() { i.once.Do(func() { close(i.done) }) })
	return i
}

func (i *idle) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i.mu.Lock()
		i.active++
		i.timer.Stop()
		i.mu.Unlock()
		defer func() {
			i.mu.Lock()
			if i.active--; i.active == 0 {
				i.timer.Reset(i.d)
			}
			i.mu.Unlock()
		}()
		next.ServeHTTP(w, r)
	})
}
