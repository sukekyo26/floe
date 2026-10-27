package cli

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
)

// dataPlace is a directory of floe's (dataDir): dir, "" for none; whether a setting names it; and why the default
// cannot be (under a file, HOME=/dev/null), which a reader takes as none and a writer tells, nil else.
type dataPlace struct {
	dir      string
	set      bool
	unusable error
}

// toRead is the directory to read at p, "" for none: one whose default cannot be is none.
func (p dataPlace) toRead() string {
	if p.unusable != nil {
		return ""
	}
	return p.dir
}

// dataDir is a directory of floe's: the setting key, else sub under the XDG base directory xdg, else under
// home under HOME; "" when none is an absolute path. Each is an absolute path, so that floe reads one place
// wherever it runs: a setting written relative (~/x in quotes too, which no shell read) is refused, and a
// relative XDG base directory is none, as its spec says.
func (a *app) dataDir(key, xdg, home, sub string) (dataPlace, error) {
	if setting := a.getenv(key); setting != "" {
		if !filepath.IsAbs(setting) {
			return dataPlace{dir: "", set: true, unusable: nil}, usererr.New("dir_relative", key, setting)
		}
		return dataPlace{dir: setting, set: true, unusable: nil}, nil
	}
	p := dataPlace{dir: "", set: false, unusable: nil}
	switch base, h := a.getenv(xdg), a.getenv("HOME"); {
	case filepath.IsAbs(base):
		p.dir = filepath.Join(base, sub)
	case filepath.IsAbs(h):
		p.dir = filepath.Join(h, home, sub)
	}
	if _, err := os.Stat(p.dir); p.dir != "" && errors.Is(err, syscall.ENOTDIR) {
		p.unusable = usererr.CannotWrite(p.dir, err)
	}
	return p, nil
}

// ownPlace is the directory of one's own of the kind (parts, themes, templates) as set: FLOE_<KIND>_DIR, the key
// it returns, else floe/<kind> under the XDG config directory.
func (a *app) ownPlace(kind string) (p dataPlace, key string, err error) {
	key = "FLOE_" + strings.ToUpper(kind) + "_DIR"
	p, err = a.dataDir(key, "XDG_CONFIG_HOME", ".config", filepath.Join("floe", kind))
	return p, key, err
}

// ownDir is the directory of one's own of the kind (ownPlace), to read with the system following its links, its
// Name as it was set; a zero Dir for none. One set that cannot be read is refused in the system's words; a
// default that is not there, or cannot be, is none; one there but no directory is refused.
func (a *app) ownDir(kind string) (kit.Dir, error) {
	none := kit.Dir{FS: nil, Name: ""}
	p, _, err := a.ownPlace(kind)
	dir := p.toRead()
	if err != nil || dir == "" {
		return none, err
	}
	if _, err := os.ReadDir(dir); err != nil {
		// A default not there is none; one there that is no directory is told.
		if !p.set && errors.Is(err, fs.ErrNotExist) {
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
	p, key, err := a.ownPlace(sub)
	switch {
	case err != nil:
		return err
	case p.unusable != nil:
		return p.unusable
	case p.dir == "":
		return usererr.New("cli_new_no_dir", key)
	}
	dir := p.dir
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
