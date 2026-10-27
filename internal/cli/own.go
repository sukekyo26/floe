package cli

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
)

// dataDir is a directory of floe's: the setting key, else sub under the XDG base directory xdg, else under
// home under HOME; "" when none is an absolute path, or the default is under a file (HOME=/dev/null), as there is
// then no place to read or write. set reports whether it is the setting's, which is told as it is when it cannot
// be used. Each is an
// absolute path, so that floe reads one place wherever it runs: a setting written relative (~/x in quotes too,
// which no shell read) is refused, and a relative XDG base directory is none, as its spec says.
func (a *app) dataDir(key, xdg, home, sub string) (dir string, set bool, err error) {
	if setting := a.getenv(key); setting != "" {
		if !filepath.IsAbs(setting) {
			return "", true, usererr.New("dir_relative", key, setting)
		}
		return setting, true, nil
	}
	switch base, h := a.getenv(xdg), a.getenv("HOME"); {
	case filepath.IsAbs(base):
		dir = filepath.Join(base, sub)
	case filepath.IsAbs(h):
		dir = filepath.Join(h, home, sub)
	}
	if files.UnderFile(dir) {
		return "", false, nil
	}
	return dir, false, nil
}

// ownPlace is the directory of one's own of the kind (parts, themes, templates) as set: FLOE_<KIND>_DIR, the key
// it returns, else floe/<kind> under the XDG config directory; "" when none is an absolute path.
func (a *app) ownPlace(kind string) (dir, key string, set bool, err error) {
	key = "FLOE_" + strings.ToUpper(kind) + "_DIR"
	dir, set, err = a.dataDir(key, "XDG_CONFIG_HOME", ".config", filepath.Join("floe", kind))
	return dir, key, set, err
}

// ownDir is the directory of one's own of the kind (ownPlace), to read with the system following its links, its
// Name as it was set; a zero Dir for none. One set that cannot be read is refused in the system's words; a
// default that is not there is none (one under a file dataDir gives none); one there but no directory is refused.
func (a *app) ownDir(kind string) (kit.Dir, error) {
	none := kit.Dir{FS: nil, Name: ""}
	dir, _, set, err := a.ownPlace(kind)
	if err != nil || dir == "" {
		return none, err
	}
	if _, err := os.ReadDir(dir); err != nil {
		// A default not there is none; one there that is no directory is told.
		if !set && errors.Is(err, fs.ErrNotExist) {
			return none, nil
		}
		return none, usererr.CannotRead(dir, err) //nolint:wrapcheck // the system's words, at the directory
	}
	return kit.Dir{FS: os.DirFS(dir), Name: dir}, nil
}

// kit returns floe's own with one's own over them, templates too when templates: what a page is drawn with. A
// mistake of one's own, used or not, stops it.
func (a *app) kit(templates bool) (*kit.Kit, error) {
	floe, err := kit.Floe()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own, which its tests check
	}
	var src kit.Sources
	var found usererr.List
	for _, d := range []struct {
		kind string
		to   *kit.Dir
	}{
		{"parts", &src.Parts},
		{"themes", &src.Themes},
		{"templates", &src.Templates},
	} {
		if d.kind == "templates" && !templates {
			continue
		}
		var err error
		*d.to, err = a.ownDir(d.kind)
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
	}
	if found != nil {
		return nil, found.Err()
	}
	return kit.Load(floe, src) //nolint:wrapcheck // its mistakes, or floe's failure
}

// newOwn makes the part (kit.KindPart) or the template of one's own named name, kit.Skeleton's files, in the
// directory of its kind: written whole in a directory of a dot name beside it, of no length of name's, then
// renamed, so that nothing is left half made and nothing there is written over. Its modes are under the umask.
// LIMIT: an empty directory made at its place between the Lstat and the Rename is replaced (rename(2)); use
// renameat2 with RENAME_NOREPLACE if that comes to happen.
func (a *app) newOwn(kind kit.Kind, name string) error {
	sub := "parts"
	if kind == kit.KindTemplate {
		sub = "templates"
	}
	dir, key, _, err := a.ownPlace(sub)
	switch {
	case err != nil:
		return err
	case dir == "":
		return usererr.New("cli_new_no_dir", key)
	}
	k, err := a.kit(kind == kit.KindTemplate)
	if err != nil {
		return err
	}
	if mistake := k.NameMistake(dir, kind, name); mistake != nil {
		return mistake //nolint:wrapcheck // the mistake Load would tell
	}
	at := filepath.Join(dir, name)
	if _, err = os.Lstat(at); err == nil {
		return usererr.NewAt(at, 0, 0, "cli_new_exists")
	}
	if err = os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: parts and templates are not secret
		return usererr.CannotWrite(dir, err)
	}
	// Not MkdirTemp, which makes it 0700 whatever the umask.
	tmp := filepath.Join(dir, ".new-"+rand.Text())
	if err = os.Mkdir(tmp, 0o755); err != nil { //nolint:gosec // G301: parts and templates are not secret
		return usererr.CannotWrite(dir, err)
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // gone once renamed; what is left is a dot directory, which Load skips
	for file, text := range kit.Skeleton(kind, name) {
		//nolint:gosec // G306: parts and templates are not secret
		if err := os.WriteFile(filepath.Join(tmp, file), text, 0o644); err != nil {
			return usererr.CannotWrite(filepath.Join(at, file), err)
		}
	}
	if err := os.Rename(tmp, at); err != nil {
		return usererr.CannotWrite(at, err)
	}
	a.print("%s", english.Msg("cli_made", at))
	return nil
}
