package cli

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
)

// dataPlace is a directory of floe's (dataDir): dir, "" for none; key, the setting that may name it, which a
// mistake of no setting names too; whether it does; and why the default cannot be (under a file, HOME=/dev/null),
// which a reader takes as none and a writer tells, nil else.
type dataPlace struct {
	dir      string
	key      string
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

// setting is the setting that names p, "" for the default.
func (p dataPlace) setting() string {
	if p.set {
		return p.key
	}
	return ""
}

// dataDir is a directory of floe's: the setting key, else sub under the XDG base directory xdg, else under
// home under HOME; "" when none is an absolute path. Each is an absolute path, so that floe reads one place
// wherever it runs: a setting written relative (~/x in quotes too, which no shell read) is refused, and a
// relative XDG base directory is none, as its spec says.
func (a *app) dataDir(key, xdg, home, sub string) (dataPlace, error) {
	if setting := a.getenv(key); setting != "" {
		if !filepath.IsAbs(setting) {
			return dataPlace{dir: "", key: key, set: true, unusable: nil}, usererr.New("dir_relative", key, setting)
		}
		return dataPlace{dir: setting, key: key, set: true, unusable: nil}, nil
	}
	p := dataPlace{dir: "", key: key, set: false, unusable: nil}
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

// ownPlace is the directory of one's own of the kind (parts, themes, templates) as set: FLOE_<KIND>_DIR, else
// floe/<kind> under the XDG config directory.
func (a *app) ownPlace(kind string) (dataPlace, error) {
	return a.dataDir("FLOE_"+strings.ToUpper(kind)+"_DIR", "XDG_CONFIG_HOME", ".config", filepath.Join("floe", kind))
}

// ownKinds are the kinds of one's own, in the order a mistake names their settings.
var ownKinds = []string{"parts", "themes", "templates"}

// ownDir is the directory of one's own at p (ownPlace), to read with the system following its links, its Name
// as it was set; a zero Dir for none. One set that cannot be read is refused for its setting; a default that is
// not there, or cannot be, is none; one there but no directory is refused.
func ownDir(p dataPlace) (kit.Dir, error) {
	none := kit.Dir{FS: nil, Name: ""}
	dir := p.toRead()
	if dir == "" {
		return none, nil
	}
	if _, err := os.ReadDir(dir); err != nil {
		// A default not there is none; one there that is no directory is told.
		if !p.set && errors.Is(err, fs.ErrNotExist) {
			return none, nil
		}
		return none, usererr.SettingDir(p.setting(), dir, err, usererr.CannotRead(dir, err))
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
	kinds := ownKinds
	if !templates {
		kinds = ownKinds[:2]
	}
	var found usererr.List
	places := make([]dataPlace, len(kinds))
	var spots []spot
	for i, kind := range kinds {
		var err error
		places[i], err = a.ownPlace(kind)
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
		// A default not there, or not a directory, is none to a reader: nothing in it is read twice.
		p := places[i]
		if fi, err := os.Stat(p.toRead()); p.set && p.dir != "" || err == nil && fi.IsDir() {
			spots = append(spots, spot{p: p, at: p.dir, kind: i})
		}
	}
	overlaps, in := overlapping(spots, func(int, int) bool { return true })
	if failed := found.Add(overlaps.Err()); failed != nil {
		return nil, failed
	}
	var src kit.Sources
	for i, to := range []*kit.Dir{&src.Parts, &src.Themes, &src.Templates}[:len(kinds)] {
		if in[i] {
			continue
		}
		var err error
		*to, err = ownDir(places[i])
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
	}
	if found != nil {
		return nil, found.Err()
	}
	return kit.Load(floe, src) //nolint:wrapcheck // its mistakes, or floe's failure
}

// spot is a directory to compare with others: at, in or of the place p of the kind (an index of ownKinds),
// which a mistake names.
type spot struct {
	p    dataPlace
	at   string
	kind int
}

// overlapping returns dir_overlap for each two spots, in the order of ownKinds, that pair takes and that are one
// directory or one in the other, and, by kind, whether one of its spots is in such two. A spot that cannot be
// resolved is in none.
func overlapping(spots []spot, pair func(i, j int) bool) (usererr.List, []bool) {
	resolved := make([]string, len(spots))
	ok := make([]bool, len(spots))
	for i, s := range spots {
		resolved[i], ok[i] = resolve(s.at)
	}
	var found usererr.List
	in := make([]bool, len(ownKinds))
	for i, s := range spots {
		for j := i + 1; j < len(spots); j++ {
			t := spots[j]
			if !ok[i] || !ok[j] || !pair(i, j) || !within(resolved[i], resolved[j]) && !within(resolved[j], resolved[i]) {
				continue
			}
			found = append(found, usererr.New("dir_overlap", s.p.key, s.p.dir, t.p.key, t.p.dir))
			in[s.kind], in[t.kind] = true, true
		}
	}
	return found, in
}

// resolve returns the path the system takes dir for, as written: its links and ..s followed as the system does;
// of one not there, the nearest directory above it that is, resolved, with the names after it. False when a ..
// is among those names: the system cannot take a .. after a name not there.
// LIMIT: a link that leads nowhere is a name not there, not where it leads, and a name not there is matched by
// its letters (on macOS, not of another case): one that comes to be the other directory is refused once it is
// there. Follow them if that comes to matter.
func resolve(dir string) (string, bool) {
	rest := ""
	for at := dir; ; {
		if to, err := filepath.EvalSymlinks(at); err == nil {
			return filepath.Join(to, rest), true
		}
		// dirs.Dir, as written: filepath.Dir would fold sym/.. by its letters, and filepath.Split stays on a/.
		up := dirs.Dir(at)
		name := strings.TrimPrefix(at[len(up):], string(filepath.Separator))
		if up == at || name == ".." {
			return "", false
		}
		rest = filepath.Join(name, rest)
		at = up
	}
}

// within reports whether a is b or is under it, both resolved: by their letters, as of names not there, or by
// os.SameFile, as of a name of another case on macOS or a bind mount, for each directory up from a that is there.
func within(a, b string) bool {
	bi, berr := os.Stat(b)
	for dir := a; ; dir = filepath.Dir(dir) {
		if dir == b {
			return true
		}
		if di, err := os.Stat(dir); berr == nil && err == nil && os.SameFile(di, bi) {
			return true
		}
		if dir == filepath.Dir(dir) {
			return false
		}
	}
}

// newOwn makes the part (kit.KindPart) or the template of one's own named name, kit.Skeleton's files, in the
// directory of its kind: written whole in a directory of a dot name beside it, of no length of name's, then
// renamed, so that nothing is left half made and nothing there is written over. Its modes are under the umask.
// LIMIT: an empty directory made at its place between the Lstat and the Rename is replaced (rename(2)); use
// renameat2 with RENAME_NOREPLACE if that comes to happen.
func (a *app) newOwn(kind kit.Kind, name string) error {
	made := 0
	if kind == kit.KindTemplate {
		made = 2
	}
	p, err := a.ownPlace(ownKinds[made])
	switch {
	case err != nil:
		return err
	case p.unusable != nil:
		return p.unusable
	case p.dir == "":
		return usererr.New("cli_new_no_dir", p.key)
	}
	dir := p.dir
	k, err := a.kit(kind == kit.KindTemplate)
	if err != nil {
		return err
	}
	if mistake := k.NameMistake(dir, kind, name); mistake != nil {
		return mistake //nolint:wrapcheck // the mistake Load would tell
	}
	if overlaps := a.madeOverlaps(made, dirs.Join(dir, name)); overlaps != nil {
		return overlaps.Err()
	}
	at := filepath.Join(dir, name)
	if _, err = os.Lstat(at); err == nil {
		return usererr.NewAt(at, 0, 0, "cli_new_exists")
	}
	if err = os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: parts and templates are not secret
		return usererr.SettingDir(p.setting(), dir, err, usererr.CannotWrite(dir, err))
	}
	// Not MkdirTemp, which makes it 0700 whatever the umask.
	tmp := filepath.Join(dir, ".new-"+rand.Text())
	if err = os.Mkdir(tmp, 0o755); err != nil { //nolint:gosec // G301: parts and templates are not secret
		return usererr.SettingDir(p.setting(), dir, err, usererr.CannotWrite(dir, err))
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

// madeOverlaps returns dir_overlap for each place of another kind, set or not, there or not, that the directory
// at, to be made of the kind made (an index of ownKinds), is or is in, or that is in it: a reader that reads
// both would stop once it is made.
func (a *app) madeOverlaps(made int, at string) usererr.List {
	var spots []spot
	for i, kind := range ownKinds {
		p, err := a.ownPlace(kind)
		switch {
		case i == made:
			spots = append(spots, spot{p: p, at: at, kind: i})
		case err == nil && p.dir != "":
			spots = append(spots, spot{p: p, at: p.dir, kind: i})
		}
	}
	found, _ := overlapping(spots, func(i, j int) bool { return spots[i].kind == made || spots[j].kind == made })
	return found
}
