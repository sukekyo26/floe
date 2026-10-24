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
	"time"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/ui"
)

// App is the name /api/health answers with, so that a client can tell floe from another program.
const App = "floe"

// Config is what a server serves.
type Config struct {
	Store *store.Store
	Port  int
	// Build identifies the running binary; a publish from another build restarts the server.
	Build string
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
	mux.HandleFunc("PUT /api/artifacts/{slug}", s.save)
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

// protocolText is the text of a refusal of the protocol, fixed and in plain English (forbidden origin, not
// found), which neither the management page nor the CLI sends when right. The user's mistakes and floe's
// failures go through refuse instead: a text made at run time takes a conversion to be sent with send.
type protocolText string

const textType = "text/plain; charset=utf-8"

// send answers body as text: a refusal of the protocol, or an empty body.
func send(w http.ResponseWriter, status int, body protocolText) {
	sendText(w, status, string(body))
}

// sendText answers body as text, for send, refuse and sendJSON alone.
func sendText(w http.ResponseWriter, status int, body string) {
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
		sendText(w, http.StatusInternalServerError, err.Error())
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
	templates := s.cfg.Store.Templates()
	sendJSON(w, http.StatusOK, Health{
		App: App, PID: os.Getpid(), Root: s.cfg.Store.Root(), Build: s.cfg.Build, Templates: templates.Dir(),
		Parts: templates.PartsDir(), Themes: templates.ThemesDir(), Icons: templates.IconsDir(),
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
	w.Header().Set("Location", "/a/"+slug+"/")
	send(w, http.StatusMovedPermanently, "")
}

// Serve serves on ln until ctx is done.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
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
