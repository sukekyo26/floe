package cli

import (
	_ "embed"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"floe/docs"
	"floe/internal/files"
	"floe/internal/schema"
	"floe/internal/usererr"
	"floe/internal/version"
	"floe/internal/yamlpos"
)

// floeDir holds a repository's setting and its own templates, parts and themes, at its root.
const floeDir = ".floe"

// repositoryFile is the setting under floeDir that marks a repository's root.
const repositoryFile = "repository.yaml"

//go:embed repository.schema.json
var repositorySchema []byte

// repository is the repository floe repository's commands work in: dir, its root as found up from the working
// directory, which its places and documents are under; real, the real path of dir, which inside compares with; the
// documents' directories of its setting, slash-separated from the root; and the directories opened to read in, kept
// so that each is opened once and closed when the command ends (close).
type repository struct {
	dir       string
	real      string
	documents []string
	roots     map[string]*files.Root
}

// findRoot is the directory at dir or above it that holds .floe/repository.yaml, looking no higher than one that
// holds .git (a directory, or a file in a worktree or a submodule): false when there is none.
// LIMIT: dir is gone up by its names, not its real path: from a link out of the repository into it
// (~/shortcut -> repo/docs), none is found, which refuses rather than reads. Go up the real path if that is asked.
func findRoot(dir string) (string, bool) {
	for {
		if _, err := files.Lstat(filepath.Join(dir, floeDir, repositoryFile)); err == nil {
			return dir, true
		}
		if _, err := files.Lstat(filepath.Join(dir, ".git")); err == nil {
			return "", false
		}
		up := filepath.Dir(dir)
		if up == dir {
			return "", false
		}
		dir = up
	}
}

// openRepository finds the repository of the working directory and reads its setting: the version first, so that
// a setting of another floe's tells that rather than the keys this one does not know, then the rest by its schema.
func (a *app) openRepository() (*repository, error) {
	cwd := a.env.Dir
	if cwd == "" {
		return nil, usererr.NewWhole("repository_no_cwd")
	}
	dir, ok := findRoot(cwd)
	if !ok {
		return nil, usererr.NewWhole("repository_none", cwd)
	}
	resolved, err := files.Real(dir)
	if err != nil {
		return nil, usererr.CannotRead(dir, err)
	}
	r := &repository{dir: dir, real: resolved, documents: nil, roots: map[string]*files.Root{}}
	shown := filepath.Join(floeDir, repositoryFile)
	file := filepath.Join(dir, shown)
	if outside := r.inside(file, shown); outside != nil {
		return nil, outside
	}
	src, _, err := files.Read(file)
	if err != nil {
		return nil, usererr.CannotRead(shown, err)
	}
	root, values, err := yamlpos.ParseMap(shown, src)
	if err != nil {
		return nil, err //nolint:wrapcheck // its mistakes, placed
	}
	if v, ok := values["version"].(string); ok && v != version.Version {
		return nil, usererr.NewWhole("repository_version", shown, v, version.String(), v, version.Version)
	}
	c, err := schema.Compile("floe:repository.schema.json", "repository.schema.json", repositorySchema, nil)
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own, which its tests check
	}
	if mistakes := c.ValidateAt(values); mistakes != nil {
		return nil, yamlpos.Place(mistakes, shown, root, nil) //nolint:wrapcheck // the library's mistakes, placed
	}
	var found usererr.List
	documents, _ := values["documents"].([]any) //nolint:errcheck // the schema takes a list of strings alone
	for i, d := range documents {
		name, _ := d.(string) //nolint:errcheck // the schema takes strings alone
		if !documentPath(name) {
			pos, _ := yamlpos.Locate(root, []string{"documents", strconv.Itoa(i)})
			found = append(found, usererr.NewAt(shown, pos.Line, pos.Column, "repository_path", name))
		}
		r.documents = append(r.documents, name)
	}
	return r, found.Err()
}

// documentPath reports whether name is a directory of the repository as its setting names one: slash-separated
// from the root, as files.ValidName takes it, in UTF-8, which YAML can hold.
func documentPath(name string) bool {
	return utf8.ValidString(name) && files.ValidName(name)
}

// inside refuses abs, shown as shown, unless its real path is under the repository's: a link out of it, and one
// that leads nowhere it can tell (a .. after a name not there, a loop), are refused alike. None without a repository.
func (r *repository) inside(abs, shown string) error {
	if r == nil {
		return nil
	}
	resolved, err := files.Real(abs)
	if err != nil {
		var ok bool
		if resolved, ok = files.RealAbove(abs); !ok {
			return usererr.NewAt(shown, 0, 0, "repository_outside", r.dir)
		}
	}
	if !within(newTower(resolved, files.Stat), newTower(r.real, files.Stat)) {
		return usererr.NewAt(shown, 0, 0, "repository_outside", r.dir)
	}
	return nil
}

// insideArg is inside of the data file f, which is told where it is (an arg that is nowhere, of no abs, is told by
// its read).
func (r *repository) insideArg(f arg) error {
	if r == nil || f.abs == "" {
		return nil
	}
	return r.inside(f.abs, f.name)
}

// root is the directory dir opened to read in, once for the command.
func (r *repository) root(dir string) *files.Root {
	if got, ok := r.roots[dir]; ok {
		return got
	}
	got := files.NewRoot(dir)
	r.roots[dir] = got
	return got
}

// place is the repository's directory of one's own of the kind, under .floe at its root as found (not its real
// path, so that it shows as the working directory names it), read in the directory alone (root): a link in it out
// of it is not followed. One whose real path is out of the repository is refused before it is looked at or made.
// LIMIT: the places show as absolute paths and the documents relative to the root; to show the places relative,
// open the images of checkTemplates' examples otherwise than by the place's name first.
func (r *repository) place(kind string) (dataPlace, error) {
	key := filepath.Join(floeDir, kind)
	p := dataPlace{dir: filepath.Join(r.dir, key), key: key, set: false, from: "", unusable: nil, seen: nil, root: nil}
	if err := r.inside(p.dir, key); err != nil {
		return p, err
	}
	p = looked(p)
	p.root = r.root(p.dir)
	return p, nil
}

// documentArgs are the data files under the documents' directories, each once, named from the root: files.IsDataFile
// takes them, as floe render does, and no name below a directory that starts with a dot is looked into.
// LIMIT: fs.WalkDir does not go into a link to a directory, so the data files there are left out unsaid; their
// directory goes in the setting instead (the guide says so).
func (r *repository) documentArgs() ([]arg, error) {
	walked := r.root(r.dir)
	var given []arg
	var found usererr.List
	for _, d := range r.documents {
		err := fs.WalkDir(walked, d, func(p string, e fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case p == d && !e.IsDir():
				return &fs.PathError{Op: "open", Path: p, Err: files.ErrNotDir}
			case p != d && strings.HasPrefix(e.Name(), "."):
				if e.IsDir() {
					return fs.SkipDir
				}
				return nil
			case e.IsDir() || !files.IsDataFile(p):
				return nil
			}
			name := filepath.FromSlash(p)
			given = append(given, arg{name: name, abs: filepath.Join(r.dir, name), err: nil})
			return nil
		})
		if err != nil {
			if failed := found.Add(usererr.CannotRead(filepath.FromSlash(d), err)); failed != nil {
				return nil, failed
			}
		}
	}
	return once(given), found.Err()
}

// close lets go of the directories opened to read in.
func (r *repository) close() {
	if r == nil {
		return
	}
	for _, root := range r.roots {
		_ = root.Close() //nolint:errcheck // only read from
	}
}

// newRepository writes .floe/repository.yaml in the working directory, of this floe's version and the documents'
// directories dirs, unless there is a repository here already (findRoot, as the other commands find it).
func (a *app) newRepository(dirs []string) error {
	cwd := a.env.Dir
	if cwd == "" {
		return usererr.NewWhole("repository_no_cwd")
	}
	if dir, ok := findRoot(cwd); ok {
		return usererr.New("repository_exists", filepath.Join(dir, floeDir, repositoryFile))
	}
	var found usererr.List
	for _, d := range dirs {
		if !documentPath(d) {
			found = append(found, usererr.New("repository_path", d))
		}
	}
	if found != nil {
		return found.Err()
	}
	resolved, err := files.Real(cwd)
	if err != nil {
		return usererr.CannotRead(cwd, err)
	}
	r := &repository{dir: cwd, real: resolved, documents: nil, roots: nil}
	shown := filepath.Join(floeDir, repositoryFile)
	file := filepath.Join(cwd, shown)
	if err := r.inside(file, shown); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("version: " + yamlString(version.Version) + "\ndocuments:\n")
	for _, d := range dirs {
		b.WriteString("  - " + yamlString(d) + "\n")
	}
	if err := files.MkdirAll(filepath.Dir(file), 0o755); err != nil { //nolint:gosec // G301: a repository's, to share
		return usererr.CannotWrite(filepath.Dir(shown), err)
	}
	if err := files.WriteAtomic(file, []byte(b.String()), 0o644, nil); err != nil {
		return usererr.CannotWrite(shown, err)
	}
	a.print("%s", english.Msg("cli_made", shown))
	return nil
}

// yamlString is s, valid UTF-8, as a YAML string that reads as s: in '...', as floe's guide writes text, when each
// of its characters is printable, else as strconv.Quote writes it, in "...": its escapes (\a \b \f \n \r \t \v \\ \"
// \xNN \uNNNN \UNNNNNNNN) are all YAML 1.2's, \xNN only of a character under 0x80, as UTF-8 gives none other.
// Not JSON's: json/v2 writes U+2028, U+2029, U+0085 and a BOM as they are, which YAML takes for line breaks.
func yamlString(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return !strconv.IsPrint(r) }) < 0 {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return strconv.Quote(s)
}

// repositoryGiven is the data files a command names, else every document of the repository.
func (a *app) repositoryGiven(paths []string) ([]arg, error) {
	if len(paths) > 0 {
		return a.args(paths, "cli_wants_data"), nil
	}
	return a.repo.documentArgs()
}

// renderCommand is the command that writes the pages of the command running.
func (a *app) renderCommand() string {
	if a.repo != nil {
		return "floe repository render"
	}
	return "floe render"
}

// repositoryCmd holds the commands of a repository's documents: each but new and the guides works in the
// repository of the working directory, of the version its setting names, and with its places alone.
func (a *app) repositoryCmd() *cobra.Command {
	short := "Draw and check a repository's documents, with its templates, parts and themes in .floe"
	cmd := &cobra.Command{
		Use:   "repository",
		Short: short,
		Long: short + `.

The repository is the directory at or above the working directory that holds .floe/repository.yaml
(looked for up to the one that holds .git), which floe repository new makes. Its commands use neither
FLOE_TEMPLATES_DIR, FLOE_PARTS_DIR, FLOE_THEMES_DIR, the store nor the server: they read and write files
under the repository alone, but the icon sets of floe icons. floe repository guide prints how to keep
documents in a repository.`,
		Args: cobra.NoArgs,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			r, err := a.openRepository()
			a.repo = r
			return err
		},
	}
	h := ownHelp{repository: true}
	own := []*cobra.Command{a.partCmd(h), a.templateCmd(h), a.themeCmd(h)}
	cmd.AddCommand(free(a.docGuideCmd("Print how to keep documents in a repository and check them in CI",
		docs.RepositoryGuide)), free(&cobra.Command{
		Use:   "new <dir>...",
		Short: "Make .floe/repository.yaml here, of this floe's version and the documents' directories",
		Long: `Make .floe/repository.yaml in the working directory, the repository's root: the version of this floe,
which the other commands of floe repository refuse another floe of, and the directories of its documents,
slash-separated from the root (docs/specs), whose data files they draw. It makes no directory: make the
documents' directories, which the check refuses when they are not there. Nothing is written over: with a
repository here or above, nothing is made. The directories under .floe are made by template new, part new
and theme new.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.newRepository(args)
		},
	}), &cobra.Command{
		Use:   "render [data.yaml]...",
		Short: "Write the page of each document, or of each data file given, beside it",
		Long: `Draw each data file under the documents' directories of .floe/repository.yaml, or each one given,
with the repository's templates, parts and themes, and write its page beside it, as floe render does:
only a page that changes is written, and its path is printed. A data file given is a path from the
working directory, as floe render takes it; the documents found are named from the root, wherever it
runs. With a mistake in any data file, or a directory of documents that is not there, no page is written:
every mistake is printed.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			given, err := a.repositoryGiven(args)
			if err != nil {
				return err
			}
			return a.renderPages(given, arg{name: "", abs: "", err: nil}, false)
		},
	}, &cobra.Command{
		Use:   "check [data.yaml]...",
		Short: "Check the repository's templates, parts and themes and its documents and their pages, writing nothing",
		Long: `Check the repository's templates, parts and themes, drawing each template's example.yaml, as
floe repository template check does, then each data file under the documents' directories, or each one
given, and its page, as floe render --check does: every mistake is printed, and, once the documents have
none, each page missing or not what its data file draws. A directory of documents that is not there is
told, and the documents are checked once it is there. A data file given is a path from the working
directory; the documents found are named from the root. The exit code is 1 when there is one. An HTML file
with no data file is left alone.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			own := a.checkTemplates()
			given, err := a.repositoryGiven(args)
			if err == nil {
				err = a.renderPages(given, arg{name: "", abs: "", err: nil}, true)
			}
			return usererr.Collect(own, err) //nolint:wrapcheck // mistakes for the user as they are
		},
	})
	cmd.AddCommand(own...)
	for _, c := range own {
		for _, sub := range c.Commands() {
			if sub.Name() == "guide" {
				free(sub)
			}
		}
	}
	return cmd
}

// free makes c run without the repository: outside one, and of another version.
func free(c *cobra.Command) *cobra.Command {
	c.PersistentPreRunE = func(_ *cobra.Command, _ []string) error { return nil }
	return c
}
