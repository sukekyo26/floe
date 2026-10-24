package tmpl

import (
	"cmp"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/usererr"
)

// ownDirs are the directories of parts and of themes of one's own, "" for none.
type ownDirs struct{ parts, themes string }

// partsIn returns floe's parts and themes with those of one's own in own over them: floe's alone, read
// once, when neither directory holds anything.
// LIMIT: with parts or themes of one's own, floe's are read again with them for each run, and for
// each template out of one, as the $refs they resolve change. Keep a set by the mtimes of their files if
// drawing a page comes to be slow.
func partsIn(own ownDirs) (*partSet, error) {
	if err := own.apart(); err != nil {
		return nil, err
	}
	entries, err := entriesOf(own.parts)
	if err != nil {
		return nil, usererr.New("tmpl_parts_dir_unreadable", own.parts, err.Error())
	}
	if !holdsOwn(entries) && !themesThere(own.themes) {
		return builtinParts()
	}
	return readParts(own, entries, staged{name: "", dir: ""})
}

// apart refuses own when one of its directories is the other or is in it: a part would be read as a theme
// there, or a theme as a part.
func (own ownDirs) apart() error {
	if own.parts != "" && own.themes != "" && (inside(own.parts, own.themes) || inside(own.themes, own.parts)) {
		return usererr.New("tmpl_own_dirs_overlap", own.parts, own.themes)
	}
	return nil
}

// inside reports whether the directory a is b or is in it: as written once cleaned, through a symlink, or,
// where b is there, by a name of another case on a file system that ignores it (macOS), as os.SameFile
// tells. A directory not there yet resolves through the nearest one above it that is.
// LIMIT: a symlink that leads nowhere is not followed: one that comes to overlap the other directory is
// refused once what it leads to is there. Follow it if that comes to matter.
func inside(a, b string) bool {
	if within(filepath.Clean(a), filepath.Clean(b)) || within(resolve(a), resolve(b)) {
		return true
	}
	at, err := os.Stat(b)
	if err != nil {
		return false
	}
	for dir := resolve(a); ; dir = filepath.Dir(dir) {
		if fi, err := os.Stat(dir); err == nil && os.SameFile(fi, at) {
			return true
		}
		if dir == filepath.Dir(dir) {
			return false
		}
	}
}

// resolve returns the real path of dir, symlinks followed, one not there yet through the nearest directory
// above it that is.
func resolve(dir string) string {
	rest := ""
	for at := filepath.Clean(dir); ; at = filepath.Dir(at) {
		if resolved, err := filepath.EvalSymlinks(at); err == nil || at == filepath.Dir(at) {
			return filepath.Join(cmp.Or(resolved, at), rest)
		}
		rest = filepath.Join(filepath.Base(at), rest)
	}
}

// within reports whether the directory a is b or is under it, both cleaned.
func within(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator))
}

// holdsOwn reports whether entries hold one of one's own: a name starting with a dot (.DS_Store) is none.
func holdsOwn(entries []os.DirEntry) bool {
	return slices.ContainsFunc(entries, func(e os.DirEntry) bool { return !strings.HasPrefix(e.Name(), ".") })
}

// themesThere reports whether something of one's own is at dir, where the themes of one's own go: a theme,
// or what stands in the way of the directory or keeps it from being read, which readOwnThemes tells of.
func themesThere(dir string) bool {
	if _, err := os.Lstat(dir); dir == "" || errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return false
	}
	entries, err := os.ReadDir(dir)
	return err != nil || holdsOwn(entries)
}

// staged is a part of one's own read from another directory than its own, a copy not yet in place.
type staged struct{ name, dir string }

// partsWith returns the parts partsIn returns of own, with the part of one's own name read from from, a
// directory of its files not yet in place, as if it were.
func partsWith(own ownDirs, name, from string) (*partSet, error) {
	if err := own.apart(); err != nil {
		return nil, err
	}
	entries, err := entriesOf(own.parts)
	if err != nil {
		return nil, usererr.New("tmpl_parts_dir_unreadable", own.parts, err.Error())
	}
	return readParts(own, entries, staged{name: name, dir: from})
}

// readOwn reads the parts of one's own in dir into set, over floe's of their names, a directory each, a
// symlink followed, and the part stage names from its directory. Those that cannot be used are left out of
// set, and kept in set.broken. The failure is floe's (leaveOutUnusable).
func (set *partSet) readOwn(dir string, entries []os.DirEntry, stage staged) error {
	var names []string
	for _, e := range entries {
		if name := e.Name(); !strings.HasPrefix(name, ".") {
			names = append(names, name)
		}
	}
	if stage.name != "" {
		names = append(names, stage.name)
	}
	var replaced []string
	for _, name := range names {
		at := filepath.Join(dir, name)
		from := at
		if name == stage.name {
			from = stage.dir
		}
		if set.readOwnPart(name, from, at) {
			replaced = append(replaced, name)
		}
	}
	if err := set.leaveOutUnusable(); err != nil {
		return err
	}
	// floe's themes style floe's parts alone: their rules would fit a part of one's own of the name ill.
	for _, t := range set.themes {
		for _, name := range replaced {
			delete(t.parts, name)
		}
	}
	return nil
}

// leaveOutUnusable leaves out of set the parts that cannot be used once all are read: one of one's own
// that holds itself, through the parts it holds, or takes its own look, through the looks it takes, which
// floe would follow forever, a part, floe's too, that settle refuses, again while one is left out, as a
// part may take the look of the one left out, or hold it, and then one whose schema names a usable part
// at its root. One of one's own whose schema does not compile is among those settle refuses (compileOwn).
// The failure is floe's (compileOwn).
func (set *partSet) leaveOutUnusable() error {
	held := func(at string) []string { return set.shapes[partsURL+at+".json"].refsUnder(nil) }
	looks := func(at string) []string {
		if p := set.parts[at]; p != nil {
			return p.lookNames()
		}
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(set.parts)) {
		dir := set.parts[name].dir
		if dir == "" {
			continue
		}
		if chain := cycle(name, held); chain != nil {
			set.leaveOut(name, usererr.NewAt(dir, 0, 0, "tmpl_part_cycle", name, strings.Join(chain, " → ")))
		} else if chain := cycle(name, looks); chain != nil {
			set.leaveOut(name, usererr.NewAt(dir, 0, 0, "tmpl_look_cycle", name, strings.Join(chain, " → ")))
		}
	}
	mistakes, err := set.compileOwn()
	if err != nil {
		return err
	}
	set.settle(mistakes)
	if set.leaveOutRooted() {
		set.settle(mistakes)
	}
	return nil
}

// compiledMistakes returns what stopped, stop, the compiling of the part name of set, one of one's own, as
// its mistakes (compileMistakes), but what a $ref of it that set refuses says already: where that $ref
// leads, which finds nothing, and the URL noLoader refused for it. A failure is left out when its $refs or
// notes are refused (checkHeld), which tells why it cannot be used; nil when nothing is left. The failure
// it returns is floe's: checking its $refs.
func (set *partSet) compiledMistakes(name string, stop error) (mistakes, failed error) {
	n := set.notes[partsURL+name+".json"]
	refused, failed := set.refusedRefs(n)
	if failed != nil {
		return nil, failed
	}
	url, file := partsURL+name+".json", set.parts[name].file("schema.json")
	told := func() bool {
		_, _, leads := missingTarget(stop)
		var load *jsonschema.LoadURLError
		isLoad := errors.As(stop, &load)
		return slices.ContainsFunc(refused, func(r refAt) bool {
			base, _, _ := strings.Cut(r.ref, "#")
			return leads != "" && samePlace(r.ref, leads) || isLoad && base == load.URL
		})
	}
	why := compileMistakes(url, file, stop)
	var own usererr.List
	unplaced, clean := own.Add(why) != nil, set.checkHeld(name) == nil
	switch {
	case told():
		return ownMistakes(url, file, stop), nil
	case unplaced && !clean:
		return nil, nil // a failure that its refused $refs or notes stop it at, which checkHeld tells
	}
	return why, nil
}

// settle leaves out of set the parts checkNames refuses, and those of one's own whose schema.json holds
// mistakes, by name in mistakes (compileOwn), the mistakes of both told together, again while one is left
// out.
func (set *partSet) settle(mistakes map[string]error) {
	for left := true; left; {
		left = false
		for _, name := range slices.Sorted(maps.Keys(set.parts)) {
			if err := usererr.Collect(set.checkNames(name), mistakes[name]); err != nil {
				set.leaveOut(name, err)
				left = true
			}
		}
	}
}

// compileOwn compiles the schema of each part of one's own in set, and returns, by name, what stopped each:
// the mistakes of its schema.json, or what floe cannot place there (compileMistakes). Each is compiled with
// every other part, floe's too, standing as true, a schema anything fits, and the schemas the parts share
// as they are: a $ref to a part whole finds it either way, and one into a part is refused (partMistake).
// So each stops at its own file, or at a schema the parts share, alone, whatever the order of jsonschema's
// walk, and no other part's $ref is walked there. Once settle leaves out those that stopped and those that
// refer to them (checkHeld), every part left compiles with the schemas of the others: a template that uses
// a part floe part check passes never stops at the part. The failure is floe's: making a compiler, or
// checking a part's $refs.
// LIMIT: jsonschema stops at the first mistake of a file, in the order it walks the file: those the
// metaschema refuses are told always, but another, a regular expression under draft-04, whose metaschema
// checks none, or an $anchor or $id written twice, waits for those met before it.
// LIMIT: what floe cannot place is told in jsonschema's English, as floe's failure (tmpl_own_failed).
func (set *partSet) compileOwn() (map[string]error, error) {
	mistakes := map[string]error{}
	stands := map[string]bool{}
	for url := range set.docs {
		stands[url] = !set.shares[url]
	}
	for name, p := range set.parts {
		if !p.ownSchema() {
			continue
		}
		url := partsURL + name + ".json"
		stands[url] = false
		c, err := set.compiler(stands)
		stands[url] = true
		if err != nil {
			return nil, err
		}
		compiled, err := c.Compile(url)
		if err == nil {
			// The other parts stand as true: a path into one is taken, its schema checked when it is read.
			if why := checksDeclared(p.checks, compiled); why != nil {
				mistakes[name] = why
			}
			continue
		}
		why, err := set.compiledMistakes(name, err)
		if err != nil {
			return nil, err
		}
		if why != nil {
			mistakes[name] = why
		}
	}
	return mistakes, nil
}

// isOwn reports whether p is a part of one's own, not floe's: a file of it that fails on data its schema took
// is its maker's mistake, else floe's failure.
func (p *part) isOwn() bool {
	return p.dir != ""
}

// ownSchema reports whether p is a part of one's own that draws, whose schema.json floe compiles.
func (p *part) ownSchema() bool {
	return p.isOwn() && !p.isLook()
}

// file is the file name of p, as its mistakes name it: in its directory, or under parts/ for floe's.
func (p *part) file(name string) string {
	return path.Join(cmp.Or(p.dir, path.Join("parts", p.name)), name)
}

// leaveOutRooted leaves out of set the parts whose schema names a part at its root, and reports whether it
// left one out. It runs once the rest are settled, and leaves them out all at once, so that the part a root
// names is surely usable or not, and the answer hangs on no order.
func (set *partSet) leaveOutRooted() bool {
	rooted := map[string]error{}
	for name, p := range set.parts {
		if named, ok := set.rootPart(p.shape); ok {
			rooted[name] = usererr.NewAt(p.file("schema.json"), 0, 0, "tmpl_schema_root_part", named)
		}
	}
	for name, why := range rooted {
		set.leaveOut(name, why)
	}
	return len(rooted) > 0
}

// readOwnPart reads the part name of one's own from the directory from, at at as its mistakes name it,
// into set, a symlink followed, and one in it that leads out of it refused, and reports whether it took
// the place of floe's of the name. One that cannot be used is kept in set.broken, and one of a name set
// keeps leaves floe's in place. A look alone cannot take the place of one that draws, which floe's other
// parts, Markdown's tables and templates draw by.
func (set *partSet) readOwnPart(name, from, at string) bool {
	dir, err := set.ownEntry(name, from, at)
	switch {
	case dir == "" && err == nil:
		return false
	case set.reserved(name):
		// Told whatever is there, which never takes the place of floe's.
		set.broken[name] = usererr.NewAt(at, 0, 0, "tmpl_part_name", name)
		return false
	}
	floe := set.parts[name]
	replaces := floe != nil
	set.drop(name)
	if err == nil {
		err = readIn(dir, func(files fs.FS) error { return set.readPart(files, name, at) })
	}
	if err == nil && replaces && !floe.isLook() && set.parts[name].isLook() {
		err = usererr.NewAt(at, 0, 0, "tmpl_look_replaces", name)
	}
	if err != nil {
		set.leaveOut(name, err)
		return replaces
	}
	set.parts[name].dir, set.parts[name].replaces = at, replaces
	return replaces
}

// readOwnThemes reads the themes of one's own in dir into set, a directory each, a symlink followed: one
// of the name of one of floe's takes the place of its files of their names. Those that cannot be used are
// left out of set, and kept in set.brokenThemes.
func (set *partSet) readOwnThemes(dir string) error {
	themes, err := dirAt(dir)
	if _, there := os.Lstat(dir); themes == "" && there == nil {
		// A file, or a symlink that leads nowhere, where the themes go: none of them is read.
		why := "not a directory"
		if err != nil {
			why = err.Error()
		}
		set.strays = append(set.strays, usererr.New("tmpl_themes_dir_unreadable", dir, why))
		return nil
	}
	entries, err := entriesOf(dir)
	if err != nil {
		return usererr.New("tmpl_themes_dir_unreadable", dir, err.Error())
	}
	for _, e := range entries {
		name, at := e.Name(), filepath.Join(dir, e.Name())
		if strings.HasPrefix(name, ".") {
			continue
		}
		t := set.themes[name]
		if t == nil {
			t = newTheme()
		}
		dir, err := set.ownEntry(name, at, at)
		if dir == "" && err == nil {
			continue
		}
		if err == nil {
			err = readIn(dir, func(files fs.FS) error { return set.readTheme(files, at, t) })
		}
		if err != nil {
			delete(set.themes, name)
			set.brokenThemes[name] = err
			continue
		}
		set.themes[name] = t
	}
	return nil
}

// ownEntry returns the directory of name, a part or a theme of one's own at at, read from from, as
// dirAt does: "" for a file. One of a name no part or theme can take is a mistake. It keeps the directory among those
// set read, for a link drawn with set to be drawn again when a file in it is saved.
func (set *partSet) ownEntry(name, from, at string) (string, error) {
	dir, err := dirAt(from)
	switch {
	case dir == "" && err == nil:
		return "", nil
	case !validName(name):
		return "", usererr.NewAt(at, 0, 0, "tmpl_part_bad_name")
	}
	set.keepDir(dir)
	return dir, err
}

// keepRoot keeps dir, the directory of parts or of themes of one's own, "" for none, among the directories
// set read, by its real path, or as written while there is none, where one may come.
func (set *partSet) keepRoot(dir string) {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	set.keepDir(dir)
}

// keepDir keeps dir among the directories set read, but one under one kept, and in place of those under it,
// as a reload walks each whole.
func (set *partSet) keepDir(dir string) {
	if dir == "" || slices.ContainsFunc(set.dirs, func(kept string) bool { return within(dir, kept) }) {
		return
	}
	set.dirs = append(slices.DeleteFunc(set.dirs, func(kept string) bool { return within(kept, dir) }), dir)
}

// reserved reports whether a part of one's own cannot be named name: a part floe draws in Go, a schema
// the parts share (tone, any), or a file of a theme that is no part's (base, palette).
func (set *partSet) reserved(name string) bool {
	return isGoPart(name) || set.shares[partsURL+name+".json"] || name == themeBase || name == themePalette
}

// checkNames refuses the part name when its part.html names it in no
// data-floe-part, names more than one part in one, or names a part there is none of, when part.yaml has
// it take the look of a part there is none of, and when its schema holds a part there is none of, or a
// look alone. floe's parts name themselves, one each, as a test checks.
func (set *partSet) checkNames(name string) error {
	p := set.parts[name]
	own := p.isOwn()
	html, yaml := p.file("part.html"), p.file("part.yaml")
	named := namedParts(p.html)
	var errs []error
	if own && !p.isLook() && !namesItself(named, name) {
		errs = append(errs, usererr.NewAt(html, 0, 0, "tmpl_part_unnamed", name, name))
	}
	for _, n := range named {
		kind := set.misnamed(n)
		if kind == nameRight {
			continue
		}
		line := lineAt([]byte(p.html), n.at)
		switch {
		case own && kind == nameNotOne:
			errs = append(errs, usererr.NewAt(html, line, 0, "tmpl_part_several", n.value))
		case own && kind == nameLookBox:
			errs = append(errs, usererr.NewAt(html, line, 0, "tmpl_look_box", n.name))
		default:
			errs = append(errs, set.unlent(kind, n.name, html, line, n.look))
		}
	}
	for _, look := range p.looks {
		errs = append(errs, set.unlent(set.known(look.name), look.name, yaml, look.line, true))
	}
	return usererr.Collect(append(errs, set.checkHeld(name))...)
}

// nameKind is what a name an attribute gives is to a part set.
type nameKind int

const (
	nameRight   nameKind = iota // a part's, as the attribute names it
	nameNotOne                  // of a data-floe-part not of one name as written
	nameLookBox                 // of a data-floe-part that names a look alone, which has no box
	nameBroken                  // a part's that cannot be used
	nameUnknown                 // no part's
)

// misnamed tells what the name n is to set: the checks of a part's HTML, of a template's page.html and
// of a page drawn tell each kind in their own words.
func (set *partSet) misnamed(n namedPart) nameKind {
	p := set.parts[n.name]
	switch kind := set.known(n.name); {
	case n.text:
		// No attribute, but text that tells of one, or a script that may write one: it brings the style alone.
		return nameRight
	case kind == nameBroken:
		return nameBroken
	case n.notOne():
		return nameNotOne
	case kind == nameUnknown:
		return nameUnknown
	case !n.look && p.isLook():
		return nameLookBox
	}
	return nameRight
}

// known tells what name is to set: a part's, one that cannot be used, or none.
func (set *partSet) known(name string) nameKind {
	switch {
	case set.parts[name] != nil:
		return nameRight
	case set.broken[name] != nil:
		return nameBroken
	}
	return nameUnknown
}

// unlent refuses name, a part another takes the look of, or names in file at line, by kind, what it is to
// set: a part there is none of, told with those a look (when look) or a box may name, or one that cannot
// be used.
func (set *partSet) unlent(kind nameKind, name, file string, line int, look bool) error {
	switch kind {
	case nameBroken:
		return usererr.NewAt(file, line, 0, "tmpl_part_lender_broken", name)
	case nameUnknown:
		return usererr.NewAt(file, line, 0, "tmpl_part_names", name, set.candidates(look))
	case nameRight, nameNotOne, nameLookBox:
	}
	return nil
}

// candidates lists the parts a look may name (when look) in place of one there is none of, every part and
// look, or a box may, the parts that draw.
func (set *partSet) candidates(look bool) string {
	if look {
		return set.names(nil)
	}
	return set.names(drawing)
}

// checkHeld refuses the part name when what its schema refers to is refused (partSet.mistakes), now that
// every part is read: a part there is none of, that cannot be used or draws nothing, among them.
func (set *partSet) checkHeld(name string) error {
	n := set.notes[partsURL+name+".json"]
	if n == nil {
		return nil // a look alone, or one floe draws in Go
	}
	mistakes, err := set.mistakes(n)
	if err != nil {
		return err
	}
	return mistakes.Err()
}

// names lists the parts of set that keep keeps, every one for nil, by name.
func (set *partSet) names(keep func(*part) bool) string {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(set.parts)) {
		if keep == nil || keep(set.parts[name]) {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// drawing keeps the parts that draw, which a schema can refer to: a look alone draws nothing.
func drawing(p *part) bool {
	return !p.isLook()
}

// namesItself reports whether an element of a part's HTML, whose names are named, is the box of the part
// name: its data-floe-part is name.
func namesItself(named []namedPart, name string) bool {
	for _, n := range named {
		if !n.look && !n.text && n.value == name {
			return true
		}
	}
	return false
}

// find returns the part name of set, refusing one that cannot be used, or that there is none of.
func (set *partSet) find(name string) (*part, error) {
	switch {
	case set.parts[name] != nil:
		return set.parts[name], nil
	case set.broken[name] != nil:
		return nil, set.brokenPart(name)
	}
	return nil, usererr.New("tmpl_unknown_part", name, set.names(nil))
}

// unusable returns the mistake of each part of set that cannot be used, by name, but those skip skips (none
// for nil).
func (set *partSet) unusable(skip func(name string) bool) usererr.List {
	var found usererr.List
	for _, name := range slices.Sorted(maps.Keys(set.broken)) {
		if skip == nil || !skip(name) {
			found = append(found, set.brokenPart(name))
		}
	}
	return found
}

// unusableThemes returns the mistake of each theme of set that cannot be used, by name, but those
// skip skips (none for nil).
func (set *partSet) unusableThemes(skip func(name string) bool) usererr.List {
	var found usererr.List
	for _, name := range slices.Sorted(maps.Keys(set.brokenThemes)) {
		if skip == nil || !skip(name) {
			found = append(found, set.brokenTheme(name))
		}
	}
	return found
}

// brokenPart is the mistake of the part name of set, which cannot be used, standing for why it cannot.
func (set *partSet) brokenPart(name string) *usererr.Error {
	return usererr.Over("part:"+name, reasons(set.broken[name]), "tmpl_part_broken", name)
}

// brokenTheme is the mistake of the theme name of set, which cannot be used, standing for why it cannot.
func (set *partSet) brokenTheme(name string) *usererr.Error {
	return usererr.Over("theme:"+name, reasons(set.brokenThemes[name]), "tmpl_theme_broken", name)
}

// reasons are why, why a part or a theme cannot be used, as mistakes: a failure among them, which is not
// the user's, is told as floe's. An exception to "a failure stops at once": the failure of one part or
// theme of one's own does not stop the pages that do not use it.
func reasons(why error) usererr.List {
	var l usererr.List
	if failed := l.Add(why); failed != nil {
		l = append(l, usererr.New("tmpl_own_failed", failed))
	}
	return l
}

// leaveOut leaves the part name out of set, kept in set.broken with why.
func (set *partSet) leaveOut(name string, why error) {
	set.broken[name] = why
	set.drop(name)
}

// cycle returns the parts that lead from the part name back to it, name first and last, each part
// leading to those next gives (the parts it holds, or the looks it takes), or nil when none do.
func cycle(name string, next func(at string) []string) []string {
	var chain []string
	seen := map[string]bool{}
	var reach func(at string) bool
	reach = func(at string) bool {
		chain = append(chain, at)
		for _, n := range next(at) {
			if n == name {
				chain = append(chain, n)
				return true
			}
			if !seen[n] {
				seen[n] = true
				if reach(n) {
					return true
				}
			}
		}
		chain = chain[:len(chain)-1]
		return false
	}
	if reach(name) {
		return chain
	}
	return nil
}

// refsUnder adds to names the parts, and the schemas the parts share, that s and the shapes under it refer
// to, as written.
func (s *shape) refsUnder(names []string) []string {
	if s == nil {
		return names
	}
	if name, ok := partName(s.ref); ok {
		names = append(names, name)
	}
	for _, key := range slices.Sorted(maps.Keys(s.keys)) {
		names = s.keys[key].refsUnder(names)
	}
	return s.each.refsUnder(names)
}

// drop leaves the part name out of set, with its schema.
func (set *partSet) drop(name string) {
	url := partsURL + name + ".json"
	delete(set.parts, name)
	delete(set.docs, url)
	delete(set.shapes, url)
	delete(set.notes, url)
}
