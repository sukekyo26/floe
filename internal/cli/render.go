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

// rendered is the page drawn from the data file from, the HTML file to it goes to, and the real path of
// that file, which a symlink to it leads to.
type rendered struct {
	from, to, real string
	page           []byte
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
	var found usererr.List
	pages := make([]rendered, 0, len(files))
	// The first page to each real path in lower case: through a symlink, and on a filesystem that
	// ignores case (macOS), two paths may be one file.
	// LIMIT: case only. APFS also takes the NFC and NFD of a name as one; fold Unicode if pages collide so.
	first := map[string]rendered{}
	for _, file := range files {
		dir := outDir
		if dir == "" {
			dir = filepath.Dir(file)
		}
		p := rendered{from: file, to: filepath.Join(dir, baseName(file)+".html"), real: "", page: nil}
		p.real = a.realPath(p.to)
		other, taken := first[strings.ToLower(p.real)]
		// One file named twice (by two globs, as ./spec.yaml and spec.yaml, in another case on macOS, or
		// through a symlinked directory) writes its page once.
		if taken && !a.twoFiles(other.from, file) {
			continue
		}
		page, err := drawFile(a, file)
		tell(&found, err)
		switch {
		case taken && other.real == p.real:
			found = append(found, usererr.New("cli_render_same_file", other.from, file, p.to))
		case taken:
			found = append(found, usererr.New("cli_render_same_file_case", other.from, file, other.to, p.to))
		case err == nil:
			p.page = tmpl.CutOffPage(page)
			pages = append(pages, p)
			fallthrough
		default:
			first[strings.ToLower(p.real)] = p
		}
	}
	if len(found) > 0 {
		return nil, found
	}
	return pages, nil
}

// realPath is the path of file with its symlinks resolved: those of its directory, and its own when it
// is there, so that a page that is a symlink is written where it leads.
func (a *app) realPath(file string) string {
	abs := a.abs(file)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	if dir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(dir, filepath.Base(abs))
	}
	return abs
}

// twoFiles reports whether the data files x and y are two files, not one; a file that cannot be read is
// left to drawFile to tell, and is not told apart.
func (a *app) twoFiles(x, y string) bool {
	fx, errX := os.Stat(a.abs(x))
	fy, errY := os.Stat(a.abs(y))
	return errX == nil && errY == nil && !os.SameFile(fx, fy)
}

// writePages writes each page that changes, and prints its path.
func (a *app) writePages(pages []rendered, outDir string) error {
	if outDir != "" {
		// As the pages in it, for others to read.
		if err := os.MkdirAll(a.abs(outDir), 0o755); err != nil { //nolint:gosec // G301: pages to share
			return usererr.CannotWrite(outDir, err)
		}
	}
	var problems usererr.List
	for _, p := range pages {
		if old, err := os.ReadFile(p.real); err == nil && bytes.Equal(old, p.page) {
			continue
		}
		// Not private like the store: the page goes into a repository, for others to read.
		if err := store.WriteAtomic(p.real, p.page, 0o644); err != nil {
			problems = append(problems, usererr.CannotWrite(p.to, err))
			continue
		}
		a.out("%s", p.to)
	}
	return problems.Err()
}

// checkPages returns a mistake for each page that is missing or is not what its data file draws.
func (a *app) checkPages(pages []rendered) error {
	var found usererr.List
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
	return found.Err()
}
