package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) renderCmd() *cobra.Command {
	var outDir string
	var check bool
	cmd := &cobra.Command{
		Use:   "render <data.yaml>...",
		Short: "Write the page of each data file to an HTML file beside it, or in --out-dir",
		Long: `Draw each data file as floe publish does and write its page to <name>.html: beside the
data file, or in --out-dir (made when missing). The page opens without floe, as a download
does: what it shows is in it, and its CSP is in a <meta>. A file is written only when it
changes, and its path is printed. Nothing is published, and the server is not started.

Every mistake of every file is printed as file:line:column: message, and nothing is written
until all are fixed; nor when two data files would write the same file.

--check writes nothing: it prints each page that is missing or is not what its data file
draws, and the exit code is 1 when there is one, so that CI tells a data file whose page
was not written again.`,
		Args: usage(cobra.MinimumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.renderPages(args, outDir, check)
		},
	}
	f := cmd.Flags()
	f.StringVar(&outDir, "out-dir", "", "the directory to write the pages in (default: the data file's own)")
	f.BoolVar(&check, "check", false, "write nothing: fail when a page is missing or not what its data file draws")
	return cmd
}

// rendered is the page drawn from the data file from, and the HTML file to it goes to.
type rendered struct {
	from, to string
	page     []byte
}

func (a *app) renderPages(files []string, outDir string, check bool) error {
	found := make(mistakes, 0, len(files))
	pages := make([]rendered, 0, len(files))
	writers := map[string]string{} // the data file that writes each HTML file, by its absolute path
	for _, file := range files {
		page, errs := drawFile(a, file)
		if len(errs) > 0 {
			found = append(found, errs...)
			continue
		}
		dir := outDir
		if dir == "" {
			dir = filepath.Dir(file)
		}
		to := filepath.Join(dir, strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))+".html")
		if first, ok := writers[a.abs(to)]; ok {
			found = append(found, usererr.New("cli_render_same_file", first, file, to))
			continue
		}
		writers[a.abs(to)] = file
		pages = append(pages, rendered{from: file, to: to, page: tmpl.WithCutOff(page)})
	}
	if len(found) > 0 {
		return found
	}
	if check {
		return a.checkPages(pages)
	}
	if outDir != "" {
		if err := os.MkdirAll(a.abs(outDir), 0o750); err != nil {
			return usererr.CannotWrite(outDir, err)
		}
	}
	for _, p := range pages {
		if old, err := os.ReadFile(a.abs(p.to)); err == nil && bytes.Equal(old, p.page) {
			continue
		}
		// Not private like the store: the page goes into a repository, for others to read.
		if err := store.WriteAtomic(a.abs(p.to), p.page, 0o644); err != nil {
			return usererr.CannotWrite(p.to, err)
		}
		a.out("%s", p.to)
	}
	return nil
}

// checkPages returns a mistake for each page that is missing or is not what its data file draws.
func (a *app) checkPages(pages []rendered) error {
	found := make(mistakes, 0, len(pages))
	for _, p := range pages {
		old, err := os.ReadFile(a.abs(p.to))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			found = append(found, usererr.New("cli_render_missing", p.to, p.from))
		case err != nil:
			found = append(found, usererr.CannotRead(p.to, err))
		case !bytes.Equal(old, p.page):
			found = append(found, usererr.New("cli_render_stale", p.to, p.from))
		}
	}
	if len(found) > 0 {
		return found
	}
	return nil
}
