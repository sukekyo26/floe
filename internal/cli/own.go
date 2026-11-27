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
// mistake of no setting names too; whether it does; from, the variable of the home that decides the default
// (link.HomeKey), which a mistake of none names; and why the default cannot be (under a file, HOME=/dev/null), which
// a reader takes as none and a writer tells, nil else; seen is what dataDir's Stat of a default told of it, nil for
// none there or one it could not look at (and for a setting, which dataDir does not look at); root, the directory
// opened to read in a repository's place (repository.place), nil for one read as the system follows its links.
type dataPlace struct {
	dir      string
	key      string
	set      bool
	from     string
	unusable error
	seen     fs.FileInfo
	root     *files.Root
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

// base is where the defaults of a kind of floe's directories go: under the directory the variable key names, else
// under underHome in the home (link.HomeKey).
type base struct {
	key, underHome string
}

// The places floe keeps its own in by default, on every system: under the XDG base directories, else under the home,
// as on Linux (Windows' home is %USERPROFILE%). Not under %LOCALAPPDATA%: what an app installed as a package (the
// Claude desktop app) makes there anew is put in the package's own folder, which a terminal does not see (S315's R4);
// under the home it is not (S315's R5).
var (
	dataBase   = base{key: "XDG_DATA_HOME", underHome: filepath.Join(".local", "share")}
	configBase = base{key: "XDG_CONFIG_HOME", underHome: ".config"}
)

// storeSub and iconsSub are the store's and the icon sets' directories under the base of the data.
const (
	storeSub = "floe"
	iconsSub = "floe-icons"
)

// dataDir is a directory of floe's: the setting key, else sub under the base b (dataPlace); "" when none is an
// absolute path. Each is an absolute path, so that floe reads one place wherever it runs: a setting written
// relative (~/x in quotes too, which no shell read) is refused, and a relative base is none, as the XDG spec says.
// LIMIT: on Windows the bits 0o600 and 0o700 floe writes with hide nothing; a directory is as private as the ACL it
// inherits. The default places, under %USERPROFILE%, let only the user, SYSTEM and the Administrators in, but a
// FLOE_* directory elsewhere (C:\x) lets other users read and change the pages (floe writes no ACL). Write one
// when that is asked for.
// LIMIT: where the home is on a network (a share, a network drive), the default places are refused (path_not_local);
// the FLOE_* settings name others. Look again if such PCs come to use floe.
func (a *app) dataDir(key string, b base, sub string) (dataPlace, error) {
	if setting := a.getenv(key); setting != "" {
		if !filepath.IsAbs(setting) {
			p := dataPlace{dir: "", key: key, set: true, from: "", unusable: nil, seen: nil, root: nil}
			return p, usererr.NewWhole("dir_relative", key, setting)
		}
		return dataPlace{dir: setting, key: key, set: true, from: "", unusable: nil, seen: nil, root: nil}, nil
	}
	p := dataPlace{dir: "", key: key, set: false, from: link.HomeKey, unusable: nil, seen: nil, root: nil}
	switch base, h := a.getenv(b.key), a.getenv(link.HomeKey); {
	case b.key != "" && filepath.IsAbs(base):
		p.dir = filepath.Join(base, sub)
	case filepath.IsAbs(h):
		p.dir = filepath.Join(h, b.underHome, sub)
	}
	if p.dir == "" {
		return p, nil
	}
	return looked(p), nil
}

// looked is the default place p as its Stat tells it: seen when it is there, unusable when it cannot be (under a
// file).
func looked(p dataPlace) dataPlace {
	fi, err := files.Stat(p.dir)
	if err != nil {
		// err is p.dir's Stat, which a file there passes: p.dir is not there, so the look starts above it.
		if notDir := files.NotDir(filepath.Dir(p.dir), err); notDir != nil {
			p.unusable = usererr.CannotWrite(p.dir, notDir)
		}
		return p
	}
	p.seen = fi
	return p
}

// ownPlace is the directory of one's own of the kind (parts, themes, templates) as set: FLOE_<KIND>_DIR, else
// floe/<kind> under the base of the configuration (configBase); under floe repository, the repository's alone.
func (a *app) ownPlace(kind string) (dataPlace, error) {
	if a.repo != nil {
		return a.repo.place(kind)
	}
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
	if p.root != nil {
		return kit.Dir{FS: p.root, Name: dir}, nil
	}
	return kit.Dir{FS: files.DirFS(dir), Name: dir}, nil
}

// ownAt is the place of a kind of one's own as one read of them takes it (ownPlaces), and the mistake of its setting;
// made for that read alone.
type ownAt struct {
	kind ownKind
	p    dataPlace
	err  error
}

// ownAtOf is the place of the kind, as set now (ownPlace).
func (a *app) ownAtOf(kind ownKind) ownAt {
	p, err := a.ownPlace(kind.name)
	return ownAt{kind: kind, p: p, err: err}
}

// ownPlaces are the places of the kinds of one's own, in the order of ownKinds, templates' too when templates: made,
// when not nil, is taken as the place of its kind.
func (a *app) ownPlaces(templates bool, made *ownAt) []ownAt {
	var places []ownAt
	for _, kind := range ownKinds {
		switch {
		case kind.name == ownTemplates.name && !templates:
		case made != nil && made.kind.name == kind.name:
			places = append(places, *made)
		default:
			places = append(places, a.ownAtOf(kind))
		}
	}
	return places
}

// kit returns floe's own with one's own over them, templates too when templates: what a page is drawn with. A
// mistake of one's own, used or not, stops it.
func (a *app) kit(templates bool) (*kit.Kit, error) {
	return kitOf(a.ownPlaces(templates, nil), templates)
}

// kitOf is kit of the places of one read (ownPlaces): templates' is told and read only when templates.
func kitOf(places []ownAt, templates bool) (*kit.Kit, error) {
	floe, err := kit.Floe()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own, which its tests check
	}
	var found usererr.List
	var used []ownAt
	var spots []spot
	for _, o := range places {
		if o.kind.name == ownTemplates.name && !templates {
			continue
		}
		if failed := found.Add(o.err); failed != nil {
			return nil, failed
		}
		used = append(used, o)
		if compared(o.p) {
			spots = append(spots, spot{p: o.p, at: o.p.dir, kind: o.kind.name})
		}
	}
	in := overlapping(&found, spots, func(int, int) bool { return true })
	var src kit.Sources
	for _, o := range used {
		if in[o.kind.name] {
			continue
		}
		var err error
		*o.kind.in(&src), err = ownDir(o.p)
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
// one that is a file as none too (files.NotDir's LIMIT). A default is as dataDir saw it (seen); a setting, which
// dataDir does not look at, is looked at here.
// LIMIT: a default is seen at the start of the read, before the other places' dataDir: one made since, where it
// overlaps another, is read with it unrefused, and one removed since is told overlapping (exit 1 where it was 0).
// Look again here if that comes to matter.
func compared(p dataPlace) bool {
	switch {
	case p.dir == "":
		return false
	case !p.set:
		return p.seen != nil && p.seen.IsDir()
	}
	fi, err := files.Stat(p.dir)
	return err != nil || fi.IsDir()
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
	towers := make([]*tower, len(spots))
	for i, s := range spots {
		if resolved, ok := files.RealAbove(s.at); ok {
			towers[i] = newTower(resolved, files.Stat)
		}
	}
	return compare(found, spots, towers, pair)
}

// compare is overlapping of the spots resolved as towers, nil for one that is not.
func compare(found *usererr.List, spots []spot, towers []*tower, pair func(i, j int) bool) map[string]bool {
	in := map[string]bool{}
	for i, s := range spots {
		for j := i + 1; j < len(spots); j++ {
			t := spots[j]
			if towers[i] == nil || towers[j] == nil || !pair(i, j) ||
				!within(towers[i], towers[j]) && !within(towers[j], towers[i]) {
				continue
			}
			*found = append(*found, usererr.New("dir_overlap", s.p.key, s.p.dir, t.p.key, t.p.dir))
			in[s.kind], in[t.kind] = true, true
		}
	}
	return in
}

// tower is a resolved place and the directories from it up to the root, by their letters, with the Stat of each
// taken when first asked for (info): within asks of the same directories again for each place compared with it.
type tower struct {
	dirs   []string
	fis    []fs.FileInfo
	looked []bool
	stat   func(string) (fs.FileInfo, error)
}

// newTower is the tower of resolved, its directories looked at with stat.
func newTower(resolved string, stat func(string) (fs.FileInfo, error)) *tower {
	var dirs []string
	for dir := resolved; ; dir = filepath.Dir(dir) {
		dirs = append(dirs, dir)
		if dir == filepath.Dir(dir) {
			break
		}
	}
	return &tower{dirs: dirs, fis: make([]fs.FileInfo, len(dirs)), looked: make([]bool, len(dirs)), stat: stat}
}

// info is the Stat of the k-th directory of t, nil for one that cannot be looked at.
// LIMIT: what a Stat told is kept for the read, a failure too: a passing failure holds for every pair the directory is
// in. Look again for each pair if that comes to matter.
func (t *tower) info(k int) fs.FileInfo {
	if !t.looked[k] {
		t.looked[k] = true
		if fi, err := t.stat(t.dirs[k]); err == nil {
			t.fis[k] = fi
		}
	}
	return t.fis[k]
}

// within reports whether a is b or is under it: by their letters, as of names not there, or by os.SameFile, as of a
// name of another case on macOS or a bind mount, for each directory up from a that is there.
func within(a, b *tower) bool {
	for k, dir := range a.dirs {
		if dir == b.dirs[0] {
			return true
		}
		if bi := b.info(0); bi != nil {
			if ai := a.info(k); ai != nil && os.SameFile(ai, bi) {
				return true
			}
		}
	}
	return false
}

// newOwn makes the part, the theme or the template of one's own (of) named name, its skeleton's files, in the
// directory of its kind: written whole in a directory of a dot name beside it, of no length of name's, then
// renamed, so that nothing is left half made and nothing there is written over. Its modes are under the umask.
// LIMIT: an empty directory made at its place between the Lstat and the Rename is replaced (rename(2)); use
// renameat2 with RENAME_NOREPLACE if that comes to happen.
func (a *app) newOwn(of ownKind, name string) error {
	made := a.ownAtOf(of)
	p := made.p
	switch {
	case made.err != nil:
		return made.err
	case p.unusable != nil:
		return p.unusable
	case p.dir == "":
		return usererr.New("cli_new_no_dir", p.from, p.key)
	}
	dir := p.dir
	// The other places only once this one is usable: a refusal above looks at no other.
	places := a.ownPlaces(true, &made)
	k, err := kitOf(places, of.kind == kit.KindTemplate)
	if err != nil {
		return err
	}
	if mistake := k.NameMistake(dir, of.kind, name); mistake != nil {
		return mistake //nolint:wrapcheck // the mistake Load would tell
	}
	if overlaps := madeOverlaps(places, of.name, dirs.Join(dir, name)); overlaps != nil {
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
	if err = files.Mkdir(tmp, 0o755); err != nil { //nolint:gosec // G301: parts, themes and templates are not secret
		return usererr.SettingDir(p.setting(), dir, err, usererr.CannotWrite(dir, err))
	}
	defer files.RemoveAll(tmp) //nolint:errcheck // gone once renamed; what is left is a dot directory, which Load skips
	if err := writeSkeleton(tmp, at, of.skeleton(name)); err != nil {
		return err
	}
	if err := files.Rename(dir, filepath.Base(tmp), name); err != nil {
		return usererr.CannotWrite(at, err)
	}
	a.print("%s", english.Msg("cli_made", at))
	return nil
}

// writeSkeleton writes each file of skeleton to tmp, the directory to be renamed to at: opened once, and closed before
// it returns, as Windows renames no directory held open.
func writeSkeleton(tmp, at string, skeleton map[string][]byte) error {
	root, err := files.OpenRoot(tmp)
	if err != nil {
		return usererr.CannotWrite(at, err)
	}
	defer root.Close() //nolint:errcheck // what was written is checked as written
	for file, text := range skeleton {
		//nolint:gosec // G306: parts, themes and templates are not secret
		if err := root.WriteFile(file, text, 0o644); err != nil {
			return usererr.CannotWrite(filepath.Join(at, file), err)
		}
	}
	return nil
}

// madeOverlaps returns dir_overlap for each place of another kind (of places, ownPlaces of all), set or not, there or
// not, that the directory at, to be made of the kind made, is or is in, or that is in it: a reader that reads both
// would stop once it is made.
func madeOverlaps(places []ownAt, made, at string) usererr.List {
	var spots []spot
	for _, o := range places {
		switch {
		case o.kind.name == made:
			spots = append(spots, spot{p: o.p, at: at, kind: made})
		case o.err == nil && o.p.dir != "":
			spots = append(spots, spot{p: o.p, at: o.p.dir, kind: o.kind.name})
		}
	}
	var found usererr.List
	overlapping(&found, spots, func(i, j int) bool { return spots[i].kind == made || spots[j].kind == made })
	return found
}
