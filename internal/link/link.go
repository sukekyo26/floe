// Package link guards the trust boundary of linked files and folders: which may be registered, and which
// paths under a registered file's directory, or a folder, may be served and listed.
package link

import (
	"errors"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"floe/internal/dirs"
	"floe/internal/files"
	"floe/internal/usererr"
)

// linkable are the files a link can register: a page, or a data file drawn each time it is opened.
var linkable = regexp.MustCompile(`(?i)\.(html?|ya?ml)$`)

// Check reports whether the file or folder at path, which must be its real path, may be linked. A link
// also serves the files under its directory (a folder's own), so the exposure is kept to that directory: one
// that is / or the home directory would expose all of it, and the hidden directories right under home
// (~/.ssh, ~/.config) hold settings and keys. Hidden directories inside a project (.agents, .github) are
// fine; Resolve never serves the dot paths under them. It reports whether path is a folder.
func Check(path, home string) (bool, error) {
	// The checks compare strings, so a relative path, a path through a symlink or a junction (~/proj -> ~/.ssh) or
	// one with .. would slip past them; so would one Real cannot tell (Windows: a parent that cannot be opened).
	if resolved, err := files.Real(path); err != nil || resolved != path {
		return false, usererr.New("link_not_real", path)
	}
	// A share's files would be served from another machine (a network drive's too: its real path is \\?\UNC\…).
	if !dirs.OnDrive(path) {
		return false, usererr.New("link_share", path, filepath.VolumeName(path))
	}
	// A FIFO named .html would block its reader, so only regular files pass, and folders.
	fi, err := files.Stat(path)
	folder := err == nil && fi.IsDir()
	if !folder && (!linkable.MatchString(path) || err != nil || !fi.Mode().IsRegular()) {
		return false, usererr.New("link_not_linkable", path)
	}
	return folder, exposes(path, home, folder)
}

// exposes refuses the real path, a folder or a file, whose served directory would expose too much: of home, the
// hidden directories right under it (a dot, or the hidden attribute on Windows); of the system, the folders of
// application data (wherever home is), and a directory holding one, the root, home or one home is under.
// LIMIT: what is hidden is looked at when a link is registered alone: one hidden later, under a home changed later or
// registered by an older floe is served on. Look again when it is served if that comes to matter.
func exposes(path, home string, folder bool) error {
	// path is real, so home must be too, or a symlinked $HOME would hide its hidden directories.
	resolvedHome, ok := realDir(home)
	if !ok {
		return usererr.New("link_home_unresolved", home)
	}
	home = resolvedHome
	dir := path
	if !folder {
		dir = filepath.Dir(path)
	}
	in, holds, err := appDataAt(path, dir)
	if err != nil {
		return usererr.New("link_appdata_unknown", path, err.Error())
	}
	// A hidden file right under home (~/.x.html) is not in a hidden directory; exposedDir refuses it.
	sep := string(filepath.Separator)
	underHome, rest := dirs.Holds(home, path), ""
	if underHome {
		// Home may be a root (/), which ends in its separator already.
		rest = strings.TrimPrefix(path[len(home):], sep)
	}
	first, _, deeper := strings.Cut(rest, sep)
	if in || underHome && (folder || deeper) && hiddenName(home, first) {
		return usererr.New("link_hidden_home", path)
	}
	if exposedDir(dir, home) || holds {
		return usererr.New("link_exposes_dir", path, dir)
	}
	return nil
}

// hiddenName reports whether the name right under home is hidden: it starts with a dot, or (Windows) has the hidden
// attribute, or its attributes cannot be read, as what is there and cannot be looked at is not let through.
func hiddenName(home, name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	h, err := hidden(filepath.Join(home, name))
	return h || err != nil
}

// appDir is a folder of application data the system names, or why it cannot tell it.
type appDir struct {
	path string
	err  error
}

// appData is appDataDirs; a test gives others.
var appData = appDataDirs //nolint:gochecknoglobals // a test's seam

// appDataAt reports, of the folders of application data the system names, whether path is one or in one (in), and
// whether dir, the directory a link of path serves, holds one (holds): settings, caches and tokens of applications
// live there, hidden or not, wherever home is (Q4). A folder not there (as files.Real tells) holds nothing and is left
// out; one that cannot be told is an error, as what cannot be checked is not let through.
func appDataAt(path, dir string) (in, holds bool, err error) {
	for _, d := range appData() {
		if d.err != nil {
			return false, false, d.err
		}
		got, err := files.Real(d.path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, false, err //nolint:wrapcheck // the system's words, for the mistake to tell
		}
		in = in || path == got || dirs.Holds(got, path)
		holds = holds || dirs.Holds(dir, got)
	}
	return in, holds, nil
}

// exposedDir reports whether serving the files under dir would expose too much: a root (/, C:\), home, or a
// directory home is under (/home, /Users), which would serve every home's files.
func exposedDir(dir, home string) bool {
	return filepath.Dir(dir) == dir || dir == home || dirs.Holds(dir, home)
}

// realDir returns the real path of dir. A dir that cannot be resolved (missing, empty and so ".", or
// not a directory) gives false: the checks that compare against it would be meaningless.
func realDir(dir string) (string, bool) {
	resolved, err := files.Real(dir)
	if err != nil {
		return "", false
	}
	fi, err := files.Stat(resolved)
	return resolved, err == nil && fi.IsDir()
}

// Resolve returns the file to serve for rest under the link registered as the real path registered, a folder
// when folder: a file's link serves the file itself when rest is empty, else the file at rest under its
// directory; a folder's, the file at rest under it, and nothing for an empty rest. rest is the URL path already
// decoded once (as http.Request.PathValue returns it) and is not decoded again, or a%2541.html would reach
// aA.html. It refuses a registered path that is no longer real (a symlink swapped in later would bypass the
// checks made at registration), and what within refuses.
func Resolve(registered string, folder bool, rest string) (string, bool) {
	if !isReal(registered) {
		return "", false
	}
	if rest == "" {
		if folder {
			return "", false
		}
		// Reading a FIFO would block until a writer shows up.
		fi, err := files.Stat(registered)
		return registered, err == nil && fi.Mode().IsRegular()
	}
	base := registered
	if !folder {
		base = filepath.Dir(registered)
	}
	file, _, ok := within(base, rest, false)
	return file, ok
}

// isReal reports whether path is its own real path.
func isReal(path string) bool {
	resolved, err := files.Real(path)
	return err == nil && resolved == path
}

// IsFolder reports whether the folder registered as the real path folder is still there to serve: a directory, by
// its real path, as Resolve and Dir take it.
func IsFolder(folder string) bool {
	if !isReal(folder) {
		return false
	}
	fi, err := files.Stat(folder)
	return err == nil && fi.IsDir()
}

// within returns the real path of rest under the real directory base, with what the system says of it, when it
// is a directory (dir) or a regular file (else): the one gate of what a link serves and lists. It refuses empty
// and dot elements (.git, .env, ..) of the names as written, a short one (SECRET~1, Windows) made long first, and
// anything whose real path leaves base. Where a link or a junction under base leads is not read for dots: it is
// served when its real path is under base (node_modules/x -> .pnpm/x).
// LIMIT: on Windows, a short name is made long by listing its parent; under a parent that cannot be listed, the
// files are not served, though they could be opened. Look again if links into such directories come up.
// LIMIT: a link or junction under base to a share (\\host\share) is opened before it is refused: it may send the
// user's credentials there. Refuse shares before they are opened, by their names, as S315's P4 plans.
func within(base, rest string, dir bool) (string, fs.FileInfo, bool) {
	// Localize reads rest as the system's names: it refuses what Windows reads otherwise (a stream after :, NUL, CON,
	// \ as a separator) and, everywhere, empty and . and .. elements and what is not UTF-8.
	local, err := filepath.Localize(rest)
	if err != nil || dotted(strings.Split(rest, "/")) {
		return "", nil, false
	}
	joined := filepath.Join(base, local)
	long, err := longName(joined)
	if err != nil {
		return "", nil, false
	}
	written, err := filepath.Rel(base, long)
	if err != nil || dotted(strings.Split(written, string(filepath.Separator))) {
		return "", nil, false
	}
	resolved, err := files.Real(joined)
	if err != nil || !strings.HasPrefix(resolved, base+string(filepath.Separator)) {
		return "", nil, false
	}
	fi, err := files.Stat(resolved)
	if err != nil || (dir && !fi.IsDir()) || (!dir && !fi.Mode().IsRegular()) {
		return "", nil, false
	}
	return resolved, fi, true
}

// longName is files.LongName; a test makes it fail.
var longName = files.LongName //nolint:gochecknoglobals // a test's seam

// dotted reports whether any of names is empty or starts with a dot (.git, .env, ..).
func dotted(names []string) bool {
	return slices.ContainsFunc(names, func(p string) bool { return p == "" || strings.HasPrefix(p, ".") })
}

// MaxListed is how many entries Dir gives of one directory.
const MaxListed = 1000

// ErrNoDir is Dir's for a directory that is not one of the folder's: gone, or refused by within.
var ErrNoDir = errors.New("no such directory in the folder")

// Listing is what Dir gives of a directory of a folder, as GET /api/pages/{slug}/files answers it: its
// directories by name, then its HTML files by name, MaxListed of them in all; More tells how many were left out.
type Listing struct {
	Dirs  []string `json:"dirs"`
	Files []File   `json:"files"`
	More  int      `json:"more"`
}

// File is an HTML file of a folder: its path under the folder, as rest, and when it changed.
type File struct {
	File     string    `json:"file"`
	Modified time.Time `json:"updatedAt"`
}

var htmlName = regexp.MustCompile(`(?i)\.html?$`)

// Dir lists the directory dir of the folder registered as the real path folder ("" for the folder itself): what
// Resolve serves, of what is directly in it. It opens no file and walks no further down, so what it costs is that
// of the directory alone.
// LIMIT: os.ReadDir reads the whole directory, which MaxListed does not bound. Read it in parts (ReadDir(n), in
// the directory's own order) if a directory of hundreds of thousands of names comes up.
func Dir(folder, dir string) (Listing, error) {
	l := Listing{Dirs: []string{}, Files: []File{}, More: 0}
	if !isReal(folder) {
		return l, ErrNoDir
	}
	path, prefix := folder, ""
	if dir != "" {
		var ok bool
		if path, _, ok = within(folder, dir, true); !ok {
			return l, ErrNoDir
		}
		prefix = dir + "/"
	}
	entries, err := files.ReadDir(path)
	if err != nil {
		return l, err //nolint:wrapcheck // the system's *fs.PathError, for the caller to tell at the folder
	}
	files := []File{}
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasPrefix(name, "."):
		// A symlink to a directory is no directory here: followed, it could lead round in a loop.
		case e.IsDir():
			l.Dirs = append(l.Dirs, name)
		case !htmlName.MatchString(name):
		// What path holds is under the folder already: only a symlink may lead out, or to what is no regular file.
		case e.Type()&fs.ModeSymlink != 0:
			if _, fi, ok := within(folder, prefix+name, false); ok {
				files = append(files, File{File: prefix + name, Modified: fi.ModTime()})
			}
		case e.Type().IsRegular():
			if fi, err := e.Info(); err == nil {
				files = append(files, File{File: prefix + name, Modified: fi.ModTime()})
			}
		}
	}
	room := MaxListed
	l.More = max(0, len(l.Dirs)+len(files)-room)
	l.Dirs = l.Dirs[:min(len(l.Dirs), room)]
	l.Files = files[:min(len(files), room-len(l.Dirs))]
	return l, nil
}
