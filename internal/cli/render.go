package cli

import (
	"bytes"
	"cmp"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/draw"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
)

func (a *app) renderCmd() *cobra.Command {
	var outDir string
	var check bool
	cmd := &cobra.Command{
		Use:   "render <data.yaml>...",
		Short: "Write the page of each data file to an HTML file beside it, or in --out-dir",
		Long: `Draw each data file and write its page to <name>.html: beside the data file, or in --out-dir
(made when missing). The page opens without floe, as a download does: what it shows is in it,
and its CSP is in a <meta>. A file is written only when it changes, and its path is printed.

Every mistake of every file is printed as file:line:column: message (file: message for one of
the whole file), and nothing is written until all are fixed; nor when two data files would
write the same file. The warnings of the pages drawn are printed after them as
file:line:column: warning: message, and leave the exit code as it is.

--check writes nothing: it prints each page that is missing or is not what its data file
draws, and the exit code is 1 when there is one, so that CI tells a data file whose page
was not written again.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Left out, it is none (beside each data file); given, even empty, it is a path.
			var out arg
			if cmd.Flags().Changed("out-dir") {
				out = a.arg(outDir, place{name: "--out-dir", wants: "cli_wants_out_dir"})
			}
			return a.renderPages(args, out, check)
		},
	}
	f := cmd.Flags()
	f.StringVar(&outDir, "out-dir", "", "the directory to write the pages in (default: the data file's own)")
	f.BoolVar(&check, "check", false, "write nothing: fail when a page is missing or not what its data file draws")
	return cmd
}

// rendered is the page drawn from the data file from, the HTML file it goes to, and the real path of that
// file, which a symlink to it leads to.
type rendered struct {
	from, to, real string
	page           []byte
}

func (a *app) renderPages(paths []string, out arg, check bool) error {
	k, err := a.kit(true)
	if err != nil {
		return err
	}
	pages, err := a.drawPages(k, a.iconStore(), paths, out)
	switch {
	case err != nil:
		return err
	case check:
		return checkPages(pages)
	default:
		return a.writePages(pages, out)
	}
}

// drawPages draws the page of each data file with k and the icons of store and names the HTML file it goes to,
// in out, else beside it, or returns the mistakes of out and of every file, and the pages two data files would write.
func (a *app) drawPages(k *kit.Kit, store diagram.Icons, paths []string, out arg) ([]rendered, error) {
	var found usererr.List
	if failed := found.Add(out.err); failed != nil {
		return nil, failed
	}
	pages := make([]rendered, 0, len(paths))
	// The pages so far, to tell two that are one file: one real path (through a symlink), or two the system
	// says are one file (os.SameFile), as on a filesystem that ignores case (macOS) a.html and A.html are.
	// LIMIT: two pages not yet written are one file only by their paths: on macOS a.html and A.html written
	// anew overwrite one another, as pages written on Linux and checked out on macOS do.
	// LIMIT: each page is compared with each before it, by what was read of it once; enough for a command line.
	var first []target
	for _, f := range a.args(paths, "cli_wants_data") {
		p, at := pageOf(f, out)
		page, warnings, err := drawFile(k, store, f)
		a.warnings = append(a.warnings, warnings...)
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
		if out.err != nil || f.err != nil || nameMistake(f.name) != nil {
			// Nowhere to write to, or to read from, or no data file to write the page of: told above.
			continue
		}
		t, bad := targetAt(p, at, first)
		if failed := found.Add(bad); failed != nil {
			return nil, failed
		}
		if bad != nil {
			continue
		}
		first, p = append(first, t), t.page
		if err == nil {
			if p.page, err = draw.CutOffPage(page); err != nil {
				return nil, err //nolint:wrapcheck // floe's failure, which says what failed
			}
			pages = append(pages, p)
		}
	}
	if len(found) > 0 {
		return nil, found
	}
	return pages, nil
}

// pageOf returns the page of the data file f, its name, in out, else beside f, as the user names it, and where
// it is.
func pageOf(f, out arg) (rendered, string) {
	dir, at := out.name, out.abs
	if dir == "" {
		dir, at = dirs.Dir(f.name), dirs.Dir(f.abs)
	}
	name := baseName(f.name) + ".html"
	return rendered{from: f.name, to: dirs.Join(dir, name), real: "", page: nil}, dirs.Join(at, name)
}

// targetAt is the page p, at where it is, and the file it goes to, or why it is not written: it leads nowhere, it
// is no page (refused), or a page of first goes to it too.
func targetAt(p rendered, at string, first []target) (target, error) {
	var err error
	if p.real, err = realPath(at, p.to); err != nil {
		return target{}, err
	}
	t := targetOf(p)
	if err := t.refused(); err != nil {
		return target{}, err
	}
	if i := slices.IndexFunc(first, t.same); i >= 0 {
		return target{}, usererr.New("cli_render_same_file", first[i].page.from, p.from, p.to)
	}
	return t, nil
}

// target is a page and what the system says of the file it goes to, nil when it is not there yet.
type target struct {
	page rendered
	info os.FileInfo
}

func targetOf(p rendered) target {
	info, err := os.Stat(p.real)
	if err != nil {
		info = nil
	}
	return target{page: p, info: info}
}

// refused returns why the page of t is not written, nil for none: where a link leads that is no page (as a data
// file is not), or what is neither a regular file nor a directory (a directory the system refuses as written).
// LIMIT: a link to an .html file of one's own (a template's page.html) is written over; refuse links into one's
// own directories if such links come to be made.
func (t target) refused() error {
	switch {
	case !files.IsPage(t.page.real):
		return usererr.NewAt(t.page.to, 0, 0, "cli_render_link_not_page", t.page.real)
	case t.info != nil && files.IsNotFile(t.info):
		return usererr.CannotRead(t.page.to, usererr.ErrNotFile)
	}
	return nil
}

// same reports whether t and o go to one file: one real path, or, when both are there, one the system says.
func (t target) same(o target) bool {
	return t.page.real == o.page.real || t.info != nil && o.info != nil && os.SameFile(t.info, o.info)
}

// realPath is the absolute path abs with its symlinks resolved: those of its directory, and its own when it
// is there, so that a page that is a symlink is written where it leads. A symlink that leads nowhere (no file,
// a loop of links) is not written over: it is told at to, the page as the user names it, as a page that cannot
// be written, in the system's words. Whether the rest can be written is the system's to tell, as it writes.
func realPath(abs, to string) (string, error) {
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if info, lerr := os.Lstat(abs); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
		// The system's words of why it leads nowhere, whatever EvalSymlinks says.
		_, serr := os.Stat(abs)
		return "", usererr.CannotWrite(to, cmp.Or(serr, err))
	}
	if parent, err := filepath.EvalSymlinks(dirs.Dir(abs)); err == nil {
		return filepath.Join(parent, filepath.Base(abs)), nil
	}
	return abs, nil
}

// writePages writes each page that changes, and prints its path; one that cannot be written is told, and the
// others are written all the same.
func (a *app) writePages(pages []rendered, out arg) error {
	if out.abs != "" {
		// As the pages in it, for others to read.
		if err := os.MkdirAll(out.abs, 0o755); err != nil { //nolint:gosec // G301: pages to share
			return usererr.CannotWrite(out.name, err)
		}
	}
	var found usererr.List
	for _, p := range pages {
		old, info, err := files.Read(p.real)
		switch {
		case errors.Is(err, usererr.ErrNotFile):
			// Made so since drawPages looked: not written over.
			if failed := found.Add(usererr.CannotRead(p.to, err)); failed != nil {
				return failed
			}
			continue
		case err == nil && bytes.Equal(old, p.page):
			continue
		}
		// Not private: the page goes into a repository, for others to read. One there, read or not, keeps its
		// permissions.
		if info != nil && !info.Mode().IsRegular() {
			info = nil // a directory there: nothing to keep, and the system refuses the write
		}
		if err := files.WriteAtomic(p.real, p.page, 0o644, info); err != nil {
			if failed := found.Add(usererr.CannotWrite(p.to, err)); failed != nil {
				return failed
			}
			continue
		}
		a.print("%s", p.to)
	}
	return found.Err()
}

// checkPages returns a mistake for each page that is missing or is not what its data file draws.
func checkPages(pages []rendered) error {
	var found usererr.List
	for _, p := range pages {
		old, _, err := files.Read(p.real)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			found = append(found, usererr.New("cli_render_missing", p.to, p.from))
		case err != nil:
			if failed := found.Add(usererr.CannotRead(p.to, err)); failed != nil {
				return failed
			}
		case !bytes.Equal(old, p.page):
			found = append(found, usererr.New("cli_render_stale", p.to, p.from))
		}
	}
	return found.Err()
}
