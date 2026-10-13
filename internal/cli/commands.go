package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

type app struct {
	env Env
	cfg *config
	// cat is the language of the messages, from the environment (i18n.Detect).
	cat *i18n.Catalog
}

func (a *app) store() *store.Store {
	return store.New(a.cfg.root, a.cfg.home)
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
	var slug, title, description, lang string
	cmd := &cobra.Command{
		Use:   "publish <data.yaml> | --link <file.html>",
		Short: "Store a page drawn from a data file (or --link an HTML file in place) and print its URL",
		Long: `Draw a page from a data file (.yaml, .yml) with the template it names (template: report),
store it and print its URL. The same slug from the same project overwrites it, and a slug
published from another project is refused unless --force. A page is in English unless
--lang names another language (a republish keeps the last one given). A new slug is also
opened in the browser ($BROWSER, open, wslview, xdg-open).

--link registers an HTML file in place and shows it as it is: it is read on every request,
and the files under its directory are served for its relative references. The same slug
for another file is refused unless --force.`,
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
			if flags.Changed("lang") {
				// A linked file is shown as it is, so nothing would take the language.
				if opts.Link {
					return usageError{err: usererr.New("cli_lang_link")}
				}
				if err := checkLang(lang); err != nil {
					return err
				}
				opts.Lang = &lang
			}
			return a.publish(cmd.Context(), args[0], opts)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&opts.Link, "link", false, "register an HTML file in place and show it as it is")
	f.StringVar(&slug, "slug", "", "the URL name (default: from the file name)")
	f.StringVar(&title, "title", "", "the title (default: the data file's title:, or a link's <title> or <h1>)")
	f.StringVar(&description, "description", "", "a description shown in the list")
	f.StringVar(&lang, "lang", "", "the language of a data file's page, e.g. ja (default: en, or the last one given)")
	f.BoolVar(&opts.Force, "force", false, "replace a page of another project, or a link to another file")
	return cmd
}

// publish checks, makes sure the server runs, then saves, so that a failed publish leaves nothing in
// the store.
func (a *app) publish(ctx context.Context, file string, opts store.Options) error {
	st := a.store()
	entry, err := st.Prepare(ctx, file, opts, a.env.Dir)
	if err != nil {
		return err //nolint:wrapcheck // the message is for the user as is
	}
	build, err := daemon.Build()
	if err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	err = daemon.Ensure(ctx, daemon.Server{Port: a.cfg.port, Root: a.cfg.root, Build: build, Env: a.serverEnv()})
	if err != nil {
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
	return append(env, "FLOE_DIR="+a.cfg.root, "FLOE_PORT="+strconv.Itoa(a.cfg.port), "HOME="+a.cfg.home)
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
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(a.cfg.port)))
	if errors.Is(err, syscall.EADDRINUSE) {
		return usererr.New("cli_port_in_use", a.cfg.port)
	}
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	a.say("cli_serving", a.cfg.root, a.cfg.base())
	srv := server.New(server.Config{Store: a.store(), Port: a.cfg.port, Build: build})
	return srv.Serve(ctx, ln) //nolint:wrapcheck // already says what failed
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
