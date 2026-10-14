package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	pages, err := a.drawPages(files, outDir)
	switch {
	case err != nil:
		return err
	case check:
		return a.checkPages(pages)
	default:
		return a.writePages(pages, outDir)
	}
}

// drawPages draws the page of each data file and names the HTML file it goes to, or returns the
// mistakes of every file, and the pages two data files would write.
func (a *app) drawPages(files []string, outDir string) ([]rendered, error) {
	found := make(mistakes, 0, len(files))
	pages := make([]rendered, 0, len(files))
	// The data file that writes each page, by the page's real path in lower case: through a symlinked
	// directory, and on a filesystem that ignores case (macOS), two paths may be one file.
	// LIMIT: case only. APFS also takes the NFC and NFD of a name as one; fold Unicode if pages collide so.
	writers := map[string]string{}
	for _, file := range files {
		dir := outDir
		if dir == "" {
			dir = filepath.Dir(file)
		}
		to := filepath.Join(dir, baseName(file)+".html")
		key := strings.ToLower(a.realPath(to))
		if first, ok := writers[key]; ok {
			// One file named twice (by two globs, as ./spec.yaml and spec.yaml, in another case on macOS,
			// or through a symlinked directory) writes its page once.
			if !sameFile(a.abs(first), a.abs(file)) {
				found = append(found, usererr.New("cli_render_same_file", first, file, to))
			}
			continue
		}
		writers[key] = file
		page, errs := drawFile(a, file)
		if len(errs) > 0 {
			found = append(found, errs...)
			continue
		}
		cut, err := io.ReadAll(tmpl.WithCutOff(page))
		if err != nil {
			return nil, fmt.Errorf("draw %s: %w", file, err)
		}
		pages = append(pages, rendered{from: file, to: to, page: cut})
	}
	if len(found) > 0 {
		return nil, found
	}
	return pages, nil
}

// realPath is the path of file with the symlinks of its directory resolved, when the directory is there.
func (a *app) realPath(file string) string {
	abs := a.abs(file)
	if dir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(dir, filepath.Base(abs))
	}
	return abs
}

// sameFile reports whether x and y are one file; false when either cannot be read, which drawFile tells.
func sameFile(x, y string) bool {
	fx, err := os.Stat(x)
	if err != nil {
		return false
	}
	fy, err := os.Stat(y)
	return err == nil && os.SameFile(fx, fy)
}

// writePages writes each page that changes, and prints its path.
func (a *app) writePages(pages []rendered, outDir string) error {
	if outDir != "" {
		// As the pages in it, for others to read.
		if err := os.MkdirAll(a.abs(outDir), 0o755); err != nil { //nolint:gosec // G301: pages to share
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
