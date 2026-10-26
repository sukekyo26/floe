package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
)

// dataDir is a directory of floe's: the setting key, else sub under the XDG base directory xdg, else under
// home under HOME; "" when none is an absolute path. set reports whether it is the setting's. Each is an
// absolute path, so that floe reads one place wherever it runs: a setting written relative (~/x in quotes too,
// which no shell read) is refused, and a relative XDG base directory is none, as its spec says.
func (a *app) dataDir(key, xdg, home, sub string) (dir string, set bool, err error) {
	if dir := a.getenv(key); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", true, usererr.New("dir_relative", key, dir)
		}
		return dir, true, nil
	}
	if base := a.getenv(xdg); filepath.IsAbs(base) {
		return filepath.Join(base, sub), false, nil
	}
	if h := a.getenv("HOME"); filepath.IsAbs(h) {
		return filepath.Join(h, home, sub), false, nil
	}
	return "", false, nil
}

// ownDir is the directory of one's own of the kind (parts, themes, templates), FLOE_<KIND>_DIR else floe/<kind>
// under the XDG config directory, to read with the system following its links, its Name as it was set; a
// zero Dir for none. One set that cannot be read is refused in the system's words; a default that is not there,
// or is under a file, is none; one that is there but no directory is refused.
func (a *app) ownDir(key, kind string) (kit.Dir, error) {
	none := kit.Dir{FS: nil, Name: ""}
	dir, set, err := a.dataDir(key, "XDG_CONFIG_HOME", ".config", filepath.Join("floe", kind))
	if err != nil || dir == "" {
		return none, err
	}
	if _, err := os.ReadDir(dir); err != nil {
		// A default not there, or under a file (HOME=/dev/null), is none; one there that is no directory is told.
		if !set && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) && !exists(dir)) {
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
		key, kind string
		to        *kit.Dir
	}{
		{"FLOE_PARTS_DIR", "parts", &src.Parts},
		{"FLOE_THEMES_DIR", "themes", &src.Themes},
		{"FLOE_TEMPLATES_DIR", "templates", &src.Templates},
	} {
		if d.kind == "templates" && !templates {
			continue
		}
		var err error
		*d.to, err = a.ownDir(d.key, d.kind)
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
	}
	if found != nil {
		return nil, found.Err()
	}
	return kit.Load(floe, src) //nolint:wrapcheck // its mistakes, or floe's failure
}

// exists reports whether there is a file at path, a link followed.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
