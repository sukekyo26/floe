package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

type app struct {
	env Env
	cfg *config
}

func (a *app) store() *store.Store {
	return store.New(a.cfg.root, a.cfg.home, a.templates())
}

func (a *app) templates() *tmpl.Library {
	return tmpl.NewLibrary(a.cfg.templates, a.cfg.parts, a.cfg.themes, a.cfg.icons)
}

// abs puts a relative dir under the working directory, and folds the .. of any by its letters, as Join and
// Abs do, so that every place reads a path alike; "" stays "".
func (a *app) abs(dir string) string {
	switch {
	case dir == "":
		return dir
	case filepath.IsAbs(dir):
		return filepath.Clean(dir)
	}
	return filepath.Join(a.env.Dir, dir)
}

func (a *app) out(format string, args ...any) {
	_, _ = fmt.Fprintf(a.env.Stdout, format+"\n", args...) //nolint:errcheck // nowhere left to report
}

// say prints the catalog message key to stdout.
func (a *app) say(key string, args ...any) {
	a.out("%s", english.Msg(key, args...))
}

// warn prints the catalog message key to stderr.
func (a *app) warn(key string, args ...any) {
	_, _ = fmt.Fprintln(a.env.Stderr, english.Msg(key, args...)) //nolint:errcheck // nowhere left to report
}

func (a *app) publishCmd() *cobra.Command {
	var opts store.Options
	var link bool
	var slug, title, description, theme string
	cmd := &cobra.Command{
		Use:         "publish <data.yaml> | --link <file.html>",
		Annotations: uses(serverUses),
		Short:       "Store a page drawn from a data file (or --link an HTML file in place) and print its URL",
		Long: `Draw a page from a data file (.yaml, .yml) with the template it names (template: report),
store it and print its URL. The same slug from the same project overwrites it, and a slug
published from another project is refused unless --force. A page is in the language its
data names (lang:), and in the theme its data names (theme:, else default), its palette and the
parts' shape, unless --theme names another. A new slug is also opened in the browser ($BROWSER,
open, wslview, xdg-open).

--link registers a file in place instead of storing a page. An HTML file is shown as it is:
it is read on every request, and the files under its directory are served for its relative
references. A data file is drawn each time its page is opened, and saving it, an image it
shows or a file of its template, or fetching the icons its diagrams show, reloads the open
tab. The same slug for another file is
refused unless --force.`,
		Args: usage(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			if flags.Changed("slug") {
				opts.Slug = &slug
			}
			if flags.Changed("title") {
				// An empty title cannot be told from none, and would leave the list blank.
				if strings.TrimSpace(title) == "" {
					return usageError{err: usererr.New("cli_title_empty")}
				}
				opts.Title = &title
			}
			if flags.Changed("description") {
				opts.Description = &description
			}
			if flags.Changed("theme") {
				// An empty theme would take the data's.
				if strings.TrimSpace(theme) == "" {
					return usageError{err: usererr.New("cli_theme_empty")}
				}
				opts.Theme = &theme
			}
			return a.publish(cmd.Context(), args[0], link, opts)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&link, "link", false, "register the file in place: HTML shown as it is, a data file drawn when opened")
	f.StringVar(&slug, "slug", "", "the URL name (default: from the file name)")
	f.StringVar(&title, "title", "", "the title (default: the data file's title:, or a link's <title> or <h1>)")
	f.StringVar(&description, "description", "", "a description shown in the list")
	f.StringVar(&theme, "theme", "",
		"the theme to draw with: its palette and the parts' shape (default: the data file's theme:, else default)")
	f.BoolVar(&opts.Force, "force", false, "replace a page of another project, or a link to another file")
	return cmd
}

// publish checks what it can without the server, makes sure the server runs, then has it check and
// save the page, so that a failed publish leaves nothing in the store.
func (a *app) publish(ctx context.Context, file string, link bool, opts store.Options) error {
	publish := a.publishData
	if link {
		publish = a.publishLink
	}
	saved, err := publish(ctx, file, opts)
	if err != nil {
		return err
	}
	url := a.cfg.base() + "/a/" + saved.Slug + "/"
	// A republish does not open another tab; the open one reloads itself.
	if saved.Prev == nil {
		daemon.OpenBrowser(ctx, a.env.Getenv("BROWSER"), url)
	}
	a.out("%s", url)
	if saved.Prev != nil {
		a.noticeUpdate(saved)
	}
	return nil
}

// publishLink links file, by its absolute path: the server reads it, as it draws it.
func (a *app) publishLink(ctx context.Context, file string, opts store.Options) (server.Saved, error) {
	var saved server.Saved
	path := a.abs(file)
	if _, err := os.Stat(path); err != nil {
		return saved, usererr.CannotRead(file, err)
	}
	c, err := a.client(ctx, false)
	if err != nil {
		return saved, err
	}
	err = c.do(ctx, http.MethodPost, "/api/artifacts", server.Link{
		Path: path, Slug: opts.Slug, Title: opts.Title, Description: opts.Description, Theme: opts.Theme,
		Force: opts.Force,
	}, &saved)
	return saved, err
}

// publishData sends the data file file under its slug. The images the server asks for are read under
// the data file's directory and sent with it again.
func (a *app) publishData(ctx context.Context, file string, opts store.Options) (server.Saved, error) {
	var saved server.Saved
	read, err := store.ReadUpload(ctx, file, a.env.Dir)
	if err != nil {
		return saved, err //nolint:wrapcheck // a usererr for the user as is
	}
	// Trimmed as the store trims the options, so that the URL names the slug the page is saved under.
	if opts.Slug != nil {
		trimmed := strings.TrimSpace(*opts.Slug)
		opts.Slug = &trimmed
	}
	slug, err := store.DataSlug(file, opts.Slug)
	if err != nil {
		return saved, err //nolint:wrapcheck // a usererr for the user as is
	}
	c, err := a.client(ctx, false)
	if err != nil {
		return saved, err
	}
	up := server.Upload{
		File: read.File, Dir: read.Dir, Source: read.Source, Images: nil, Project: read.Project, Title: opts.Title,
		Description: opts.Description, Theme: opts.Theme, Force: opts.Force,
	}
	path := "/api/artifacts/" + slug
	err = c.do(ctx, http.MethodPut, path, up, &saved)
	if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusUnprocessableEntity {
		var need server.ImagesNeeded
		if err = json.Unmarshal(re.body, &need); err != nil {
			return saved, fmt.Errorf("read the images the server asks for: %w", err)
		}
		if up.Images, err = readImages(up.Dir, need.Images); err != nil {
			return saved, err
		}
		// Sent with its images, it is not asked again.
		err = c.do(ctx, http.MethodPut, path, up, &saved)
	}
	return saved, tooLarge(err)
}

// tooLarge tells a publish of a data file refused for its size how large one may be; any other error
// it leaves as it is.
func tooLarge(err error) error {
	if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusRequestEntityTooLarge {
		return usererr.New("cli_publish_large", strconv.Itoa(server.MaxUpload>>20)+" MiB")
	}
	return err
}

// client makes sure the server of this store runs, of this build, and returns what calls it. anyDirs
// takes a server that draws with other directories than this command's, for one that draws nothing.
func (a *app) client(ctx context.Context, anyDirs bool) (client, error) {
	build, err := daemon.Build()
	if err != nil {
		return client{}, err //nolint:wrapcheck // already says what failed
	}
	err = daemon.Ensure(ctx, daemon.Server{
		Port: a.cfg.port, Root: a.cfg.root, Build: build, Templates: a.cfg.templates, Parts: a.cfg.parts,
		Themes: a.cfg.themes, Icons: a.cfg.icons, AnyDirs: anyDirs, Env: a.serverEnv(),
	})
	if err != nil {
		return client{}, err //nolint:wrapcheck // the message is for the user as is
	}
	return client{base: a.cfg.base()}, nil
}

// noticeUpdate says that a page was overwritten, so that reusing a slug for another page (say "plan")
// does not go unnoticed. A link without a pinned title has no title of its own to compare.
func (a *app) noticeUpdate(saved server.Saved) {
	prev := saved.Prev
	if prev.Title == "" {
		a.warn("cli_updated", saved.Slug, localTime(prev.UpdatedAt))
		return
	}
	a.warn("cli_updated_titled", saved.Slug, localTime(prev.UpdatedAt), prev.Title)
	if saved.Title != prev.Title {
		a.warn("cli_title_changed")
	}
}

// serverEnv is the environment of a server that a publish starts: the settings this command read win
// over the process's own.
func (a *app) serverEnv() []string {
	env := append([]string{}, a.env.Environ...)
	return append(env, "FLOE_DIR="+a.cfg.root, "FLOE_PORT="+strconv.Itoa(a.cfg.port), "HOME="+a.cfg.home,
		"FLOE_TEMPLATES_DIR="+a.cfg.templates, "FLOE_PARTS_DIR="+a.cfg.parts, "FLOE_THEMES_DIR="+a.cfg.themes,
		"FLOE_ICONS_DIR="+a.cfg.icons)
}

func localTime(iso string) string {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return iso
	}
	return t.Local().Format("2006-01-02 15:04")
}

func (a *app) listCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Annotations: uses(serverUses),
		Short:       "List pages, newest first (updated, slug, title, linked path; tab-separated)",
		Args:        usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.client(cmd.Context(), true)
			if err != nil {
				return err
			}
			var list []store.Listed
			if err = c.do(cmd.Context(), http.MethodGet, "/api/artifacts", nil, &list); err != nil {
				return err
			}
			for _, l := range list {
				a.out("%s\t%s\t%s\t%s", localTime(l.UpdatedAt), l.Slug, l.Title, l.Path)
			}
			return nil
		},
	}
}

func (a *app) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "rm <slug>...",
		Annotations: uses(serverUses),
		Short:       "Delete pages in order, stopping at the first failure (a linked file itself is left alone)",
		Args:        usage(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := a.client(cmd.Context(), true)
			if err != nil {
				return err
			}
			for _, slug := range args {
				err = c.do(cmd.Context(), http.MethodDelete, "/api/artifacts/"+url.PathEscape(slug), nil, nil)
				if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusNotFound {
					return usererr.New("store_no_page", slug)
				}
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func (a *app) serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "serve",
		Annotations: uses(serverUses),
		Short:       "Run the server in the foreground",
		Args:        usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.serve(cmd.Context())
		},
	}
}

// serve runs until SIGTERM or SIGINT ("floe stop" sends SIGTERM).
func (a *app) serve(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := dirs.CheckWritable(a.cfg.root, ".check-"); err != nil {
		return usererr.CannotWrite(a.cfg.root, err)
	}
	build, err := daemon.Build()
	if err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	ln, err := a.listen()
	if err != nil {
		return err
	}
	a.say("cli_serving", a.cfg.root, a.cfg.base())
	srv := server.New(server.Config{Store: a.store(), Port: a.cfg.port, Build: build})
	return srv.Serve(ctx, ln) //nolint:wrapcheck // already says what failed
}

// listen takes the listener the publish that started this server bound for it, or binds the port.
func (a *app) listen() (net.Listener, error) {
	if ln, err := daemon.Handed(a.env.Getenv, a.cfg.port); ln != nil || err != nil {
		// Taken once: a floe run from here binds its own.
		_ = os.Unsetenv(daemon.ListenerEnv) //nolint:errcheck // a name Unsetenv takes
		return ln, err                      //nolint:wrapcheck // already says what failed
	}
	ln, err := daemon.Listen(a.cfg.port)
	if err != nil {
		return nil, daemon.CannotListen(a.cfg.port, err)
	}
	return ln, nil
}

func (a *app) stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "stop",
		Annotations: uses(map[setting]use{portSetting: needs}),
		Short:       "Stop the background server",
		Args:        usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := daemon.Probe(cmd.Context(), a.cfg.base())
			if p.State != daemon.Ours {
				a.say("cli_not_running")
				return nil
			}
			if err := daemon.Terminate(p.Health.PID); err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			a.say("cli_stopped", p.Health.PID)
			return nil
		},
	}
}
