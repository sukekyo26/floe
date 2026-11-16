package cli

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"floe/internal/dirs"
	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/link"
	"floe/internal/usererr"
)

// dataPlace is a directory of floe's (dataDir): dir, "" for none; key, the setting that may name it, which a
// mistake of no setting names too; whether it does; from, the variable of the system that decides the default (HOME,
// LOCALAPPDATA), which a mistake of none names; and why the default cannot be (under a file, HOME=/dev/null), which
// a reader takes as none and a writer tells, nil else.
type dataPlace struct {
	dir      string
	key      string
	set      bool
	from     string
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

// base is where the defaults of a kind of floe's directories go: under the directory the variable key names, else,
// with underHome, under that in the home (link.HomeKey); the values are the system's (places_unix.go,
// places_windows.go).
type base struct {
	key, underHome string
}

// dataDir is a directory of floe's: the setting key, else sub under the base b (dataPlace); "" when none is an
// absolute path. Each is an absolute path, so that floe reads one place wherever it runs: a setting written
// relative (~/x in quotes too, which no shell read) is refused, and a relative base is none, as the XDG spec says.
// LIMIT: on Windows the bits 0o600 and 0o700 floe writes with hide nothing; a directory is as private as the ACL it
// inherits. The default places (%LOCALAPPDATA%, and %USERPROFILE% for skills) let only the user, SYSTEM and the
// Administrators in, but a FLOE_* directory elsewhere (C:\x) lets other users read and change the pages (Q7: floe
// writes no ACL). Write one when that is asked for.
func (a *app) dataDir(key string, b base, sub string) (dataPlace, error) {
	if setting := a.getenv(key); setting != "" {
		if !filepath.IsAbs(setting) {
			p := dataPlace{dir: "", key: key, set: true, from: "", unusable: nil}
			return p, usererr.NewWhole("dir_relative", key, setting)
		}
		return dataPlace{dir: setting, key: key, set: true, from: "", unusable: nil}, nil
	}
	p := dataPlace{dir: "", key: key, set: false, from: b.key, unusable: nil}
	if b.underHome != "" {
		p.from = link.HomeKey
	}
	switch base, h := a.getenv(b.key), a.getenv(link.HomeKey); {
	case b.key != "" && filepath.IsAbs(base):
		p.dir = filepath.Join(base, sub)
	case b.underHome != "" && filepath.IsAbs(h):
		p.dir = filepath.Join(h, b.underHome, sub)
	}
	if p.dir == "" {
		return p, nil
	}
	if _, err := files.Stat(p.dir); err != nil {
		if notDir := files.NotDir(p.dir, err); notDir != nil {
			p.unusable = usererr.CannotWrite(p.dir, notDir)
		}
	}
	return p, nil
}

// ownPlace is the directory of one's own of the kind (parts, themes, templates) as set: FLOE_<KIND>_DIR, else
// floe/<kind> under the base of the configuration (configBase).
func (a *app) ownPlace(kind string) (dataPlace, error) {
	return a.dataDir("FLOE_"+strings.ToUpper(kind)+"_DIR", configBase, filepath.Join("floe", kind))
}

// ownKind is a kind of one's own: its name (ownPlace), its directory in the sources of a kit, its kind to the kit,
// and the files new makes of it.
type ownKind struct {
	name     string
	in       func(*kit.Sources) *kit.Dir
	kind     kit.Kind
	skeleton func(name string) map[string][]byte
}

// The kinds of one's own.
var (
	ownParts  = ownKind{"parts", func(s *kit.Sources) *kit.Dir { return &s.Parts }, kit.KindPart, kit.PartSkeleton}
	ownThemes = ownKind{
		"themes", func(s *kit.Sources) *kit.Dir { return &s.Themes }, kit.KindTheme,
		func(string) map[string][]byte { return kit.ThemeSkeleton() },
	}
	ownTemplates = ownKind{
		"templates", func(s *kit.Sources) *kit.Dir { return &s.Templates }, kit.KindTemplate, kit.TemplateSkeleton,
	}
)

// ownKinds are the kinds of one's own, in the order a mistake names their settings.
var ownKinds = []ownKind{ownParts, ownThemes, ownTemplates}

// ownDir is the directory of one's own at p (ownPlace), to read with the system following its links, its Name
// as it was set; a zero Dir for none. One set that cannot be read is refused for its setting; a default that is
// not there, or cannot be, is none; one there but no directory is refused.
func ownDir(p dataPlace) (kit.Dir, error) {
	none := kit.Dir{FS: nil, Name: ""}
	dir := p.toRead()
	if dir == "" {
		return none, nil
	}
	if _, err := files.ReadDir(dir); err != nil {
		// A default not there is none; one there that is no directory is told.
		if !p.set && errors.Is(err, fs.ErrNotExist) {
			return none, nil
		}
		return none, usererr.SettingDir(p.setting(), dir, err, usererr.CannotRead(dir, err))
	}
	return kit.Dir{FS: files.DirFS(dir), Name: dir}, nil
}

// kit returns floe's own with one's own over them, templates too when templates: what a page is drawn with. A
// mistake of one's own, used or not, stops it.
func (a *app) kit(templates bool) (*kit.Kit, error) {
	floe, err := kit.Floe()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own, which its tests check
	}
	var found usererr.List
	places := map[string]dataPlace{}
	var spots []spot
	for _, kind := range ownKinds {
		if kind.name == "templates" && !templates {
			continue
		}
		p, err := a.ownPlace(kind.name)
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
		places[kind.name] = p
		if compared(p) {
			spots = append(spots, spot{p: p, at: p.dir, kind: kind.name})
		}
	}
	in := overlapping(&found, spots, func(int, int) bool { return true })
	var src kit.Sources
	for _, kind := range ownKinds {
		p, used := places[kind.name]
		if !used || in[kind.name] {
			continue
		}
		var err error
		*kind.in(&src), err = ownDir(p)
		if failed := found.Add(err); failed != nil {
			return nil, failed
		}
	}
	if found != nil {
		return nil, found.Err()
	}
	return kit.Load(floe, src) //nolint:wrapcheck // its mistakes, or floe's failure
}

// compared reports whether a reader compares p with the other places: a directory there, or a setting not
// there. A default not there is none to a reader, and one there but no directory, ownDir tells on Unix; Windows reads
// one that is a file as none too (files.NotDir's LIMIT).
func compared(p dataPlace) bool {
	if p.dir == "" {
		return false
	}
	fi, err := files.Stat(p.dir)
	if err != nil {
		return p.set
	}
	return fi.IsDir()
}

// spot is a directory to compare with others: at, in or of the place p of the kind, which a mistake names.
type spot struct {
	p    dataPlace
	at   string
	kind string
}

// overlapping adds to found dir_overlap for each two spots, in the order of ownKinds, that pair takes and that
// are one directory or one in the other, and returns the kinds of the spots in such two. A spot that cannot be
// resolved is in none.
func overlapping(found *usererr.List, spots []spot, pair func(i, j int) bool) map[string]bool {
	resolved := make([]string, len(spots))
	ok := make([]bool, len(spots))
	for i, s := range spots {
		resolved[i], ok[i] = files.RealAbove(s.at)
	}
	in := map[string]bool{}
	for i, s := range spots {
		for j := i + 1; j < len(spots); j++ {
			t := spots[j]
			if !ok[i] || !ok[j] || !pair(i, j) || !within(resolved[i], resolved[j]) && !within(resolved[j], resolved[i]) {
				continue
			}
			*found = append(*found, usererr.New("dir_overlap", s.p.key, s.p.dir, t.p.key, t.p.dir))
			in[s.kind], in[t.kind] = true, true
		}
	}
	return in
}

// within reports whether a is b or is under it, both resolved: by their letters, as of names not there, or by
// os.SameFile, as of a name of another case on macOS or a bind mount, for each directory up from a that is there.
func within(a, b string) bool {
	bi, berr := files.Stat(b)
	for dir := a; ; dir = filepath.Dir(dir) {
		if dir == b {
			return true
		}
		if di, err := files.Stat(dir); berr == nil && err == nil && os.SameFile(di, bi) {
			return true
		}
		if dir == filepath.Dir(dir) {
			return false
		}
	}
}

// newOwn makes the part, the theme or the template of one's own (of) named name, its skeleton's files, in the
// directory of its kind: written whole in a directory of a dot name beside it, of no length of name's, then
// renamed, so that nothing is left half made and nothing there is written over. Its modes are under the umask.
// LIMIT: an empty directory made at its place between the Lstat and the Rename is replaced (rename(2)); use
// renameat2 with RENAME_NOREPLACE if that comes to happen.
func (a *app) newOwn(of ownKind, name string) error {
	made := of.name
	p, err := a.ownPlace(made)
	switch {
	case err != nil:
		return err
	case p.unusable != nil:
		return p.unusable
	case p.dir == "":
		return usererr.New("cli_new_no_dir", p.from, p.key)
	}
	dir := p.dir
	k, err := a.kit(of.kind == kit.KindTemplate)
	if err != nil {
		return err
	}
	if mistake := k.NameMistake(dir, of.kind, name); mistake != nil {
		return mistake //nolint:wrapcheck // the mistake Load would tell
	}
	if overlaps := a.madeOverlaps(made, dirs.Join(dir, name)); overlaps != nil {
		return overlaps.Err()
	}
	at := filepath.Join(dir, name)
	if _, err = files.Lstat(at); err == nil {
		return usererr.NewAt(at, 0, 0, "cli_new_exists")
	}
	if err = files.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: parts, themes and templates are not secret
		return usererr.SettingDir(p.setting(), dir, err, usererr.CannotWrite(dir, err))
	}
	// Not MkdirTemp, which makes it 0700 whatever the umask.
	tmp := filepath.Join(dir, ".new-"+rand.Text())
	if err = os.Mkdir(tmp, 0o755); err != nil { //nolint:gosec // G301: parts, themes and templates are not secret
		return usererr.SettingDir(p.setting(), dir, err, usererr.CannotWrite(dir, err))
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // gone once renamed; what is left is a dot directory, which Load skips
	for file, text := range of.skeleton(name) {
		//nolint:gosec // G306: parts, themes and templates are not secret
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
// at, to be made of the kind made, is or is in, or that is in it: a reader that reads both would stop once it
// is made.
func (a *app) madeOverlaps(made, at string) usererr.List {
	var spots []spot
	for _, kind := range ownKinds {
		p, err := a.ownPlace(kind.name)
		switch {
		case kind.name == made:
			spots = append(spots, spot{p: p, at: at, kind: made})
		case err == nil && p.dir != "":
			spots = append(spots, spot{p: p, at: p.dir, kind: kind.name})
		}
	}
	var found usererr.List
	overlapping(&found, spots, func(i, j int) bool { return spots[i].kind == made || spots[j].kind == made })
	return found
}
