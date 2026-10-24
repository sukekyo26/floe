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
		Use:         "render <data.yaml>...",
		Annotations: uses(drawUses),
		Short:       "Write the page of each data file to an HTML file beside it, or in --out-dir",
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// Left out, it is none (beside each data file); given, even empty, it is a path.
			var out arg
			if cmd.Flags().Changed("out-dir") {
				out = a.arg(outDir)
			}
			return a.renderPages(args, out, check)
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

func (a *app) renderPages(files []string, out arg, check bool) error {
	pages, err := a.drawPages(files, out)
	switch {
	case err != nil:
		return err
	case check:
		return checkPages(pages)
	default:
		return a.writePages(pages, out)
	}
}

// drawPages draws the page of each data file and names the HTML file it goes to, in out, else beside it,
// or returns the mistakes of out and of every file, and the pages two data files would write.
func (a *app) drawPages(files []string, out arg) ([]rendered, error) {
	var found usererr.List
	tell(&found, out.err)
	pages := make([]rendered, 0, len(files))
	// The first page to each real path in lower case: through a symlink, and on a filesystem that
	// ignores case (macOS), two paths may be one file.
	// LIMIT: case only. APFS also takes the NFC and NFD of a name as one; fold Unicode if pages collide so.
	first := map[string]rendered{}
	for _, f := range a.args(files) {
		file := f.name
		dir, at := out.name, out.abs
		if dir == "" {
			dir, at = filepath.Dir(file), filepath.Dir(f.abs)
		}
		name := baseName(file) + ".html"
		p := rendered{from: file, to: filepath.Join(dir, name), real: "", page: nil}
		page, err := drawFile(a, f)
		tell(&found, err)
		if out.err != nil || f.err != nil || f.abs == "" {
			// Nowhere to write to, or "", which is no data file: told above.
			continue
		}
		p.real = realPath(filepath.Join(at, name))
		other, taken := first[strings.ToLower(p.real)]
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

// realPath is the absolute path abs with its symlinks resolved: those of its directory, and its own when
// it is there, so that a page that is a symlink is written where it leads.
func realPath(abs string) string {
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	if dir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(dir, filepath.Base(abs))
	}
	return abs
}

// writePages writes each page that changes, and prints its path.
func (a *app) writePages(pages []rendered, out arg) error {
	if out.abs != "" {
		// As the pages in it, for others to read.
		if err := os.MkdirAll(out.abs, 0o755); err != nil { //nolint:gosec // G301: pages to share
			return usererr.CannotWrite(out.abs, err)
		}
	}
	for _, p := range pages {
		if old, err := os.ReadFile(p.real); err == nil && bytes.Equal(old, p.page) {
			continue
		}
		// Not private like the store: the page goes into a repository, for others to read.
		if err := store.WriteAtomic(p.real, p.page, 0o644); err != nil {
			return usererr.CannotWrite(p.to, err)
		}
		a.out("%s", p.to)
	}
	return nil
}

// checkPages returns a mistake for each page that is missing or is not what its data file draws.
func checkPages(pages []rendered) error {
	var found usererr.List
	for _, p := range pages {
		old, err := os.ReadFile(p.real)
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
