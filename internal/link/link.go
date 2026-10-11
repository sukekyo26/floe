// Package link guards the trust boundary of linked files: which files may be registered, and which
// paths under a registered file's directory may be served.
package link

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

var htmlRe = regexp.MustCompile(`(?i)\.html?$`)

// Check reports whether the file at path, which must be its real path, may be linked. A link
// also serves the files under its directory, so the exposure is kept to that directory: a file
// directly under / or the home directory would expose all of it, and the hidden directories right
// under home (~/.ssh, ~/.config) hold settings and keys. Hidden directories inside a project
// (.agents, .github) are fine; Resolve never serves the dot paths under them.
func Check(path, home string) error {
	// The checks compare strings, so a path through a symlink (~/proj -> ~/.ssh) or with .. would slip
	// past them.
	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != path {
		return usererr.New(
			path+" is not its real path; pass it through filepath.EvalSymlinks (symlinks resolved, path cleaned) first",
			path+" は実パス（symlink を解決して正規化したパス）ではないので、登録できるかを確かめられません",
		)
	}
	// A FIFO named .html would block its reader, so only regular files pass.
	if fi, err := os.Stat(path); !htmlRe.MatchString(path) || err != nil || !fi.Mode().IsRegular() {
		return usererr.New(
			path+" is not a regular .html or .htm file; only HTML files can be linked",
			path+" は通常の HTML ファイル（.html / .htm）ではありません",
		)
	}
	// path is real, so home must be too, or a symlinked $HOME would hide its hidden directories.
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	// A hidden file right under home (~/.x.html) is not in a hidden directory; the next check refuses it.
	rest, underHome := strings.CutPrefix(path, home+string(filepath.Separator))
	if underHome && strings.HasPrefix(rest, ".") && strings.ContainsRune(rest, filepath.Separator) {
		return usererr.New(
			path+" is under a hidden directory of your home (settings and keys live there); move it out to link it",
			path+" はホーム直下の隠しディレクトリ（設定や鍵の置き場）の中にあるので登録できません",
		)
	}
	if dir := filepath.Dir(path); dir == string(filepath.Separator) || dir == home {
		return usererr.New(
			fmt.Sprintf("%s would expose all of %s; move it into a directory of its own and link it there", path, dir),
			fmt.Sprintf("%s を登録すると %s 全体が公開されます。専用のディレクトリに置いてから登録してください", path, dir),
		)
	}
	return nil
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
