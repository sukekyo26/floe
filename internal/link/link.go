// Package link guards the trust boundary of linked files: which files may be registered, and which
// paths under a registered file's directory may be served.
package link

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// linkable are the files a link can register: a page, or a data file drawn each time it is opened.
var linkable = regexp.MustCompile(`(?i)\.(html?|ya?ml)$`)

// Check reports whether the file at path, which must be its real path, may be linked. A link
// also serves the files under its directory, so the exposure is kept to that directory: a file
// directly under / or the home directory would expose all of it, and the hidden directories right
// under home (~/.ssh, ~/.config) hold settings and keys. Hidden directories inside a project
// (.agents, .github) are fine; Resolve never serves the dot paths under them.
func Check(path, home string) error {
	// The checks compare strings, so a relative path, a path through a symlink (~/proj -> ~/.ssh) or one
	// with .. would slip past them. EvalSymlinks keeps a relative path relative, hence IsAbs.
	if resolved, err := filepath.EvalSymlinks(path); !filepath.IsAbs(path) || err == nil && resolved != path {
		return usererr.About(path, "link_not_real", path)
	}
	// A FIFO named .html would block its reader, so only regular files pass.
	if fi, err := os.Stat(path); !linkable.MatchString(path) || err != nil || !fi.Mode().IsRegular() {
		return usererr.About(path, "link_not_linkable", path)
	}
	// path is real, so home must be too, or a symlinked $HOME would hide its hidden directories.
	resolvedHome, ok := realDir(home)
	if !ok {
		return usererr.New("link_home_unresolved", home)
	}
	home = resolvedHome
	// A hidden file right under home (~/.x.html) is not in a hidden directory; the next check refuses it.
	rest, underHome := strings.CutPrefix(path, home+string(filepath.Separator))
	if underHome && strings.HasPrefix(rest, ".") && strings.ContainsRune(rest, filepath.Separator) {
		return usererr.About(path, "link_hidden_home", path)
	}
	if dir := filepath.Dir(path); dir == string(filepath.Separator) || dir == home {
		return usererr.About(path, "link_exposes_dir", path, dir)
	}
	return nil
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

// Resolve returns the file to serve for rest under the link registered as the real path registered:
// the registered file itself when rest is empty, else the file at rest under its directory. rest is
// the URL path already decoded once (as http.Request.PathValue returns it) and is not decoded again,
// or a%2541.html would reach aA.html. It refuses empty and dot elements (.git, .env, ..), anything
// whose real path leaves the directory, anything but a regular file, and a registered path that is no
// longer real (a symlink swapped in later would bypass the checks made at registration).
func Resolve(registered, rest string) (string, bool) {
	if resolved, err := filepath.EvalSymlinks(registered); err != nil || resolved != registered {
		return "", false
	}
	file := registered
	if rest != "" {
		parts := strings.Split(rest, "/")
		for _, p := range parts {
			if p == "" || strings.HasPrefix(p, ".") {
				return "", false
			}
		}
		base := filepath.Dir(registered)
		resolved, err := filepath.EvalSymlinks(filepath.Join(append([]string{base}, parts...)...))
		if err != nil || !strings.HasPrefix(resolved, base+string(filepath.Separator)) {
			return "", false
		}
		file = resolved
	}
	// Reading a FIFO would block until a writer shows up.
	if fi, err := os.Stat(file); err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	return file, true
}
