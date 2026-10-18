package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

type app struct {
	env Env
	cfg *config
	// cat is the language of the messages, from the environment (i18n.Detect).
	cat *i18n.Catalog
}

func (a *app) store() *store.Store {
	return store.New(a.cfg.root, a.cfg.home, a.templates())
}

func (a *app) templates() *tmpl.Library {
	return tmpl.NewLibrary(a.templatesDir(), a.iconsDir())
}

// templatesDir is the directory of templates of one's own, so that the server a publish starts uses, and
// the one it finds runs with, the one this command means.
func (a *app) templatesDir() string {
	return a.abs(a.cfg.templates)
}

// iconsDir is the directory of icons, so that the server a publish starts draws the icons this command
// checked a linked data file with.
func (a *app) iconsDir() string {
	return a.abs(a.cfg.icons)
}

// abs puts a relative dir under the working directory; "" stays "".
func (a *app) abs(dir string) string {
	if dir == "" || filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(a.env.Dir, dir)
}

func (a *app) out(format string, args ...any) {
	_, _ = fmt.Fprintf(a.env.Stdout, format+"\n", args...) //nolint:errcheck // nowhere left to report
}

// say prints the catalog message key to stdout.
func (a *app) say(key string, args ...any) {
	a.out("%s", a.cat.Msg(key, args...))
}

// warn prints the catalog message key to stderr.
func (a *app) warn(key string, args ...any) {
	_, _ = fmt.Fprintln(a.env.Stderr, a.cat.Msg(key, args...)) //nolint:errcheck // nowhere left to report
}

func (a *app) publishCmd() *cobra.Command {
	var opts store.Options
	var slug, title, description, theme, partsTheme string
	cmd := &cobra.Command{
		Use:   "publish <data.yaml> | --link <file.html>",
		Short: "Store a page drawn from a data file (or --link an HTML file in place) and print its URL",
		Long: `Draw a page from a data file (.yaml, .yml) with the template it names (template: report),
store it and print its URL. The same slug from the same project overwrites it, and a slug
published from another project is refused unless --force. A page is in the language its
data names (lang:), and in the themes its data names (theme: for the template's, parts-theme:
for floe's parts, each else default) unless --theme or --parts-theme names another. A new
slug is also opened in the browser ($BROWSER, open, wslview, xdg-open).

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
			// A theme given is trimmed, and an empty one refused: it would take the data's.
			for _, f := range []struct {
				name, empty string
				value       *string
				to          **string
			}{
				{"theme", "cli_theme_empty", &theme, &opts.Theme},
				{"parts-theme", "cli_parts_theme_empty", &partsTheme, &opts.PartsTheme},
			} {
				if !flags.Changed(f.name) {
					continue
				}
				*f.value = strings.TrimSpace(*f.value)
				if *f.value == "" {
					return usageError{err: usererr.New(f.empty)}
				}
				*f.to = f.value
			}
			return a.publish(cmd.Context(), args[0], opts)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&opts.Link, "link", false, "register the file in place: HTML shown as it is, a data file drawn when opened")
	f.StringVar(&slug, "slug", "", "the URL name (default: from the file name)")
	f.StringVar(&title, "title", "", "the title (default: the data file's title:, or a link's <title> or <h1>)")
	f.StringVar(&description, "description", "", "a description shown in the list")
	f.StringVar(&theme, "theme", "", "the template's theme to draw with (default: the data file's theme:, else default)")
	f.StringVar(&partsTheme, "parts-theme", "",
		"the theme to draw floe's parts with (default: the data file's parts-theme:, else default)")
	f.BoolVar(&opts.Force, "force", false, "replace a page of another project, or a link to another file")
	return cmd
}

// publish checks, makes sure the server runs, then saves, so that a failed publish leaves nothing in
// the store.
func (a *app) publish(ctx context.Context, file string, opts store.Options) error {
	build, err := daemon.Build()
	if err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	server := daemon.Server{
		Port: a.cfg.port, Root: a.cfg.root, Build: build, Templates: a.templatesDir(), Icons: a.iconsDir(),
		Env: a.serverEnv(),
	}
	// Before the check, which would find mistakes with templates the server does not draw with.
	if err = daemon.CheckDirs(ctx, server); err != nil {
		return err //nolint:wrapcheck // the message is for the user as is
	}
	st := a.store()
	entry, err := st.Prepare(ctx, file, opts, a.env.Dir)
	if err != nil {
		return err //nolint:wrapcheck // the message is for the user as is
	}
	if err = daemon.Ensure(ctx, server); err != nil {
		return err //nolint:wrapcheck // the message is for the user as is
	}
	if err := st.Save(entry); err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	url := a.cfg.base() + "/a/" + entry.Slug + "/"
	// A republish does not open another tab; the open one reloads itself.
	if entry.Prev == nil {
		daemon.OpenBrowser(ctx, a.env.Getenv("BROWSER"), url)
	}
	a.out("%s", url)
	if entry.Prev != nil {
		a.noticeUpdate(entry)
	}
	return nil
}

// noticeUpdate says that a page was overwritten, so that reusing a slug for another page (say "plan")
// does not go unnoticed. A link without a pinned title has no title of its own to compare.
func (a *app) noticeUpdate(e *store.Entry) {
	prev := e.Prev
	if prev.Title == "" {
		a.warn("cli_updated", e.Slug, localTime(prev.UpdatedAt))
		return
	}
	a.warn("cli_updated_titled", e.Slug, localTime(prev.UpdatedAt), prev.Title)
	if e.Meta.Title != prev.Title {
		a.warn("cli_title_changed")
	}
}

// serverEnv is the environment of a server that a publish starts: the settings this command read win
// over the process's own.
func (a *app) serverEnv() []string {
	env := append([]string{}, a.env.Environ...)
	return append(env, "FLOE_DIR="+a.cfg.root, "FLOE_PORT="+strconv.Itoa(a.cfg.port), "HOME="+a.cfg.home,
		"FLOE_TEMPLATES_DIR="+a.templatesDir(), "FLOE_ICONS_DIR="+a.iconsDir())
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
		Use:   "list",
		Short: "List pages, newest first (updated, slug, title, linked path; tab-separated)",
		Args:  usage(cobra.NoArgs),
		RunE: func(*cobra.Command, []string) error {
			list, err := a.store().List()
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
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
		Use:   "rm <slug>...",
		Short: "Delete pages in order, stopping at the first failure (a linked file itself is left alone)",
		Args:  usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			st := a.store()
			for _, slug := range args {
				if err := st.Remove(slug); err != nil {
					return err //nolint:wrapcheck // the message is for the user as is
				}
			}
			return nil
		},
	}
}

func (a *app) serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the server in the foreground",
		Args:  usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.serve(cmd.Context())
		},
	}
}

// serve runs until SIGTERM or SIGINT ("floe stop" sends SIGTERM).
func (a *app) serve(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := os.MkdirAll(a.cfg.root, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", a.cfg.root, err)
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
	if errors.Is(err, syscall.EADDRINUSE) {
		return nil, usererr.New("cli_port_in_use", a.cfg.port)
	}
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	return ln, nil
}

func (a *app) stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background server",
		Args:  usage(cobra.NoArgs),
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
