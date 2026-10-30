// Package link guards the trust boundary of linked files and folders: which may be registered, and which
// paths under a registered file's directory, or a folder, may be served and listed.
package link

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sukekyo26/floe/internal/usererr"
)

// linkable are the files a link can register: a page, or a data file drawn each time it is opened.
var linkable = regexp.MustCompile(`(?i)\.(html?|ya?ml)$`)

// Check reports whether the file or folder at path, which must be its real path, may be linked. A link
// also serves the files under its directory (a folder's own), so the exposure is kept to that directory: one
// that is / or the home directory would expose all of it, and the hidden directories right under home
// (~/.ssh, ~/.config) hold settings and keys. Hidden directories inside a project (.agents, .github) are
// fine; Resolve never serves the dot paths under them.
func Check(path, home string) error {
	// The checks compare strings, so a relative path, a path through a symlink (~/proj -> ~/.ssh) or one
	// with .. would slip past them. EvalSymlinks keeps a relative path relative, hence IsAbs.
	if resolved, err := filepath.EvalSymlinks(path); !filepath.IsAbs(path) || err == nil && resolved != path {
		return usererr.New("link_not_real", path)
	}
	// A FIFO named .html would block its reader, so only regular files pass, and folders.
	fi, err := os.Stat(path)
	folder := err == nil && fi.IsDir()
	if !folder && (!linkable.MatchString(path) || err != nil || !fi.Mode().IsRegular()) {
		return usererr.New("link_not_linkable", path)
	}
	return exposes(path, home, folder)
}

// exposes refuses the real path, a folder or a file, whose served directory would expose too much of home.
func exposes(path, home string, folder bool) error {
	// path is real, so home must be too, or a symlinked $HOME would hide its hidden directories.
	resolvedHome, ok := realDir(home)
	if !ok {
		return usererr.New("link_home_unresolved", home)
	}
	home = resolvedHome
	// A hidden file right under home (~/.x.html) is not in a hidden directory; the next check refuses it.
	rest, underHome := strings.CutPrefix(path, home+string(filepath.Separator))
	if underHome && strings.HasPrefix(rest, ".") && (folder || strings.ContainsRune(rest, filepath.Separator)) {
		return usererr.New("link_hidden_home", path)
	}
	dir := path
	if !folder {
		dir = filepath.Dir(path)
	}
	if exposedDir(dir, home) {
		return usererr.New("link_exposes_dir", path, dir)
	}
	return nil
}

// exposedDir reports whether serving the files under dir would expose too much: / or home itself.
func exposedDir(dir, home string) bool {
	return dir == string(filepath.Separator) || dir == home
}

// realDir returns the real path of dir. A dir that cannot be resolved (missing, empty and so ".", or
// not a directory) gives false: the checks that compare against it would be meaningless.
func realDir(dir string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || !filepath.IsAbs(resolved) {
		return "", false
	}
	fi, err := os.Stat(resolved)
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
		fi, err := os.Stat(registered)
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
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && resolved == path
}

// within returns the real path of rest under the real directory base, with what the system says of it, when it
// is a directory (dir) or a regular file (else): the one gate of what a link serves and lists. It refuses empty
// and dot elements (.git, .env, ..) and anything whose real path leaves base.
func within(base, rest string, dir bool) (string, fs.FileInfo, bool) {
	parts := strings.Split(rest, "/")
	for _, p := range parts {
		if p == "" || strings.HasPrefix(p, ".") {
			return "", nil, false
		}
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(append([]string{base}, parts...)...))
	if err != nil || !strings.HasPrefix(resolved, base+string(filepath.Separator)) {
		return "", nil, false
	}
	fi, err := os.Stat(resolved)
	if err != nil || (dir && !fi.IsDir()) || (!dir && !fi.Mode().IsRegular()) {
		return "", nil, false
	}
	return resolved, fi, true
}

// MaxListed is how many entries Dir gives of one directory.
const MaxListed = 1000

// ErrNoDir is Dir's for a directory that is not one of the folder's: gone, or refused by within.
var ErrNoDir = errors.New("no such directory in the folder")

// Listing is what Dir gives of a directory of a folder: its directories, then its pages, by name and MaxListed
// of them in all; More tells how many were left out.
type Listing struct {
	Dirs  []string
	Pages []Page
	More  int
}

// Page is a page of a folder: its path under the folder, as rest, and when it changed.
type Page struct {
	File     string
	Modified time.Time
}

var pageName = regexp.MustCompile(`(?i)\.html?$`)

// Dir lists the directory dir of the folder registered as the real path folder ("" for the folder itself): what
// Resolve serves, of what is directly in it. It opens no file and walks no further down, so what it costs is that
// of the directory alone.
// LIMIT: os.ReadDir reads the whole directory, which MaxListed does not bound. Read it in parts (ReadDir(n), in
// the directory's own order) if a directory of hundreds of thousands of names comes up.
func Dir(folder, dir string) (Listing, error) {
	var l Listing
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
	entries, err := os.ReadDir(path)
	if err != nil {
		return l, err //nolint:wrapcheck // the system's *fs.PathError, for the caller to tell at the folder
	}
	var pages []Page
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasPrefix(name, "."):
		// A symlink to a directory is no directory here: followed, it could lead round in a loop.
		case e.IsDir():
			l.Dirs = append(l.Dirs, name)
		case pageName.MatchString(name):
			if _, fi, ok := within(folder, prefix+name, false); ok {
				pages = append(pages, Page{File: prefix + name, Modified: fi.ModTime()})
			}
		}
	}
	room := MaxListed
	l.More = max(0, len(l.Dirs)+len(pages)-room)
	l.Dirs = l.Dirs[:min(len(l.Dirs), room)]
	l.Pages = pages[:min(len(pages), room-len(l.Dirs))]
	return l, nil
}
