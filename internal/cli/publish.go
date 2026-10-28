package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/project"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) publishCmd() *cobra.Command {
	var opts store.Options
	var link bool
	var slug, title, description, theme string
	cmd := &cobra.Command{
		Use:   "publish <data.yaml> | --link <file.html>",
		Short: "Store a page drawn from a data file (or --link an HTML file in place) and print its URL",
		Long: `Draw a page from a data file (.yaml, .yml) with the template it names (template: report),
store it as drawn and print its URL. The same slug from the same project overwrites it, and a slug
published from another project is refused unless --force. A page is in the language its data names
(lang:), and in the theme its data names (theme:, else default), unless --theme names another. A new
slug is also opened in the browser ($BROWSER, open, wslview, xdg-open). The page stays as it was
drawn: floe rebuild draws the stored pages again with the templates of now.

--link registers a file in place instead of storing a page. An HTML file is shown as it is, and the
files under its directory are served for its relative references. A data file is drawn each time
its page is opened, and saving it, or a part, theme or template of one's own, reloads the open tab.
The same slug for another file is refused unless --force.`,
		Args: usage(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			for name, to := range map[string]**string{
				"slug": &opts.Slug, "title": &opts.Title, "description": &opts.Description, "theme": &opts.Theme,
			} {
				if flags.Changed(name) {
					v, _ := flags.GetString(name) //nolint:errcheck // a flag of the command, a string
					*to = &v
				}
			}
			return a.publish(cmd.Context(), args[0], link, opts)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&link, "link", false, "register the file in place: HTML shown as it is, a data file drawn when opened")
	f.StringVar(&slug, "slug", "", "the URL name (default: from the file name)")
	f.StringVar(&title, "title", "",
		"the title in the list (default: the data file's title:, or a link's <title> or <h1>)")
	f.StringVar(&description, "description", "", "a description shown in the list")
	f.StringVar(&theme, "theme", "",
		"the theme to draw with: its palette and the parts' shape (default: the data file's theme:, else default)")
	f.BoolVar(&opts.Force, "force", false, "replace a page of another project, or a link to another file")
	return cmd
}

// publish checks what it can without the server, makes sure the server runs, then has it check and
// save the page, so that a failed publish leaves nothing in the store.
func (a *app) publish(ctx context.Context, file string, link bool, opts store.Options) error {
	// Checked as the server checks, to be refused before it starts, and sent so, as the slug names the URL.
	opts, err := store.Accept(file, link, opts)
	p := a.arg(file, place{name: "argument 1", wants: "cli_wants_publish"})
	var saved server.Saved
	switch {
	// An empty data file is Accept's to refuse; told again here, it would be told twice, in other words.
	case !link && file == "":
	case p.err != nil:
		err = usererr.Collect(err, p.err)
	case link:
		saved, err = a.publishLink(ctx, p, opts, err)
	// Read as data only when it is a data file: Accept has told that it is not.
	case files.IsDataFile(file):
		saved, err = a.publishData(ctx, p, opts, err)
	}
	if err != nil {
		return err //nolint:wrapcheck // usererrs for the user as they are
	}
	port, err := a.port()
	if err != nil {
		return err
	}
	u := "http://" + daemon.Addr(port) + "/a/" + saved.Slug + "/"
	// A republish does not open another tab; the open one reloads itself.
	if saved.Prev == nil {
		daemon.OpenBrowser(ctx, a.getenv("BROWSER"), u)
	}
	a.print("%s", u)
	if saved.Prev != nil {
		a.noticeUpdate(saved)
	}
	return nil
}

// publishLink links the file p, with opts Accept returned, unless found, the mistakes found so far, or its own
// are some. What it sends is encoded first, as it will be sent: a text no UTF-8 is told before a file is read.
func (a *app) publishLink(ctx context.Context, p arg, opts store.Options, found error) (server.Saved, error) {
	var saved server.Saved
	lk := server.Link{
		Path: p.abs, Slug: opts.Slug, Title: opts.Title, Description: opts.Description, Theme: opts.Theme,
		Force: opts.Force,
	}
	_, err := encode(lk)
	// The server reads a link, as it draws it.
	if _, statErr := os.Stat(p.abs); err == nil && statErr != nil {
		err = usererr.CannotRead(p.name, statErr)
	}
	if err = usererr.Collect(found, err); err != nil {
		return saved, err //nolint:wrapcheck // usererrs for the user as they are
	}
	c, err := a.client(ctx, false)
	if err != nil {
		return saved, err
	}
	err = c.do(ctx, http.MethodPost, "/api/artifacts", lk, &saved)
	return saved, err
}

// publishData sends the data file p under the slug of opts, which Accept returned, published from the project of the
// working directory, unless found, the mistakes found so far, or its own are some. It is encoded first, as
// publishLink's link. The images the server asks for are read under the data file's directory and sent with it
// again, once.
func (a *app) publishData(ctx context.Context, p arg, opts store.Options, found error) (server.Saved, error) {
	var saved server.Saved
	up := server.Upload{
		File: p.name, Source: nil, Images: nil, Project: "", Title: opts.Title, Description: opts.Description,
		Theme: opts.Theme, Force: opts.Force,
	}
	// Of git, asked only for a publish that may go on.
	if found == nil {
		up.Project = project.Name(ctx, a.env.Dir)
	}
	_, err := encode(up)
	if err == nil {
		up.Source, err = p.read()
	}
	if err = usererr.Collect(found, err); err != nil {
		return saved, err //nolint:wrapcheck // usererrs for the user as they are
	}
	c, err := a.client(ctx, false)
	if err != nil {
		return saved, err
	}
	path := "/api/artifacts/" + *opts.Slug
	err = c.do(ctx, http.MethodPut, path, up, &saved)
	if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusUnprocessableEntity {
		var need server.ImagesNeeded
		if err = json.Unmarshal(re.body, &need); err != nil {
			return saved, fmt.Errorf("read the images the server asks for: %w", err)
		}
		if up.Images, err = readImages(dirs.Dir(p.abs), p.name, need.Images); err != nil {
			return saved, err
		}
		// Sent with its images, it is not asked again: a second ask is floe's failure, as a remoteError.
		err = c.do(ctx, http.MethodPut, path, up, &saved)
	}
	return saved, err
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

// warn prints the catalog message key to stderr.
func (a *app) warn(key string, args ...any) {
	_, _ = fmt.Fprintln(a.env.Stderr, english.Msg(key, args...)) //nolint:errcheck // nowhere left to report
}

func localTime(iso string) string {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return iso
	}
	return t.Local().Format("2006-01-02 15:04")
}

func (a *app) listCmd() *cobra.Command {
	var archived bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pages, newest first (updated, slug, title, linked path; tab-separated), archived ones left out",
		Args:  usage(cobra.NoArgs),
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
				if l.Archived && !archived {
					continue
				}
				row := fmt.Sprintf("%s\t%s\t%s\t%s", localTime(l.UpdatedAt), l.Slug, l.Title, l.Path)
				if archived {
					mark := ""
					if l.Archived {
						mark = "archived"
					}
					row += "\t" + mark
				}
				a.print("%s", row)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&archived, "archived", false, "list archived pages too, with a fifth column: archived, or empty")
	return cmd
}

func (a *app) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <slug>...",
		Short: "Delete pages in order, stopping at the first failure (a linked file itself is left alone)",
		Args:  usage(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.eachPage(cmd, args, http.MethodDelete, nil)
		},
	}
}

func (a *app) archiveCmd() *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "archive <slug>...",
		Short: "Archive pages in order, stopping at the first failure: off the lists, still served (--undo gives them back)",
		Args:  usage(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			archived := !undo
			body := server.Edit{Title: nil, Description: nil, Project: nil, Slug: nil, Favorite: nil, Archived: &archived}
			return a.eachPage(cmd, args, http.MethodPatch, body)
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "give the pages back to the list")
	return cmd
}

// eachPage sends method with body to each page of slugs in order, stopping at the first failure, a page not there
// told by its slug.
func (a *app) eachPage(cmd *cobra.Command, slugs []string, method string, body any) error {
	c, err := a.client(cmd.Context(), true)
	if err != nil {
		return err
	}
	for _, slug := range slugs {
		err = c.do(cmd.Context(), method, "/api/artifacts/"+url.PathEscape(slug), body, nil)
		if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusNotFound {
			return usererr.New("store_no_page", slug)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *app) rebuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild",
		Short: "Draw every stored page again with the templates, parts and themes of now",
		Long: `Have the server draw every stored page again from the copy of its data file and images, with
its templates, parts and themes of now, and keep the page where it changed. A page that no longer
draws keeps the page it had, and its mistakes are told. Linked files are drawn each time they are
opened, and are left alone.`,
		Args: usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.client(cmd.Context(), false)
			if err != nil {
				return err
			}
			return c.do(cmd.Context(), http.MethodPost, "/api/rebuild", struct{}{}, nil)
		},
	}
}
