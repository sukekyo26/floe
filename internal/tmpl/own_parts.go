package tmpl

import (
	"cmp"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// partsIn returns floe's parts with those of one's own in dir over them, "" for none: floe's alone, read
// once, when dir holds nothing.
// LIMIT: with parts of one's own, floe's are read again with them for each data file read, as the $refs
// they resolve change. Keep a set by the mtimes of dir's files if drawing a page comes to be slow.
func partsIn(dir string) (*partSet, error) {
	if dir == "" {
		return builtinParts()
	}
	entries, err := entriesOf(dir)
	if err != nil {
		return nil, usererr.New("tmpl_parts_dir_unreadable", dir, err.Error())
	}
	// A name starting with a dot (.DS_Store) is none of one's own.
	if !slices.ContainsFunc(entries, func(e os.DirEntry) bool { return !strings.HasPrefix(e.Name(), ".") }) {
		return builtinParts()
	}
	return readParts(dir, entries, staged{name: "", dir: ""})
}

// staged is a part of one's own read from another directory than its own, a copy not yet in place.
type staged struct{ name, dir string }

// partsWith returns the parts partsIn returns of dir, with the part of one's own name read from from, a
// directory of its files not yet in place, as if it were.
func partsWith(dir, name, from string) (*partSet, error) {
	entries, err := entriesOf(dir)
	if err != nil {
		return nil, usererr.New("tmpl_parts_dir_unreadable", dir, err.Error())
	}
	return readParts(dir, entries, staged{name: name, dir: from})
}

// readOwn reads the parts of one's own in dir into set, over floe's of their names, a directory each, a
// symlink followed, and the part stage names from its directory. Those that cannot be used are left out of
// set, and kept in set.broken.
func (set *partSet) readOwn(dir string, entries []os.DirEntry, stage staged) error {
	if parts, err := filepath.EvalSymlinks(dir); err == nil {
		set.dirs = append(set.dirs, parts)
	}
	var names []string
	for _, e := range entries {
		if name := e.Name(); name != themesDir && !strings.HasPrefix(name, ".") {
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
	set.leaveOutUnusable()
	// floe's themes style floe's parts alone: their rules would fit a part of one's own of the name ill.
	for _, t := range set.themes {
		for _, name := range replaced {
			delete(t.parts, name)
		}
	}
	return set.readOwnThemes(filepath.Join(dir, themesDir))
}

// leaveOutUnusable leaves out of set the parts that cannot be used once all are read: one of one's own
// that holds itself, through the parts it holds, or takes its own look, through the looks it takes, which
// floe would follow forever, a part, floe's too, that checkNames refuses, again while one is left out, as a
// part may take the look of the one left out, or hold it, and then one whose schema names a usable part at
// its root.
func (set *partSet) leaveOutUnusable() {
	held := func(at string) []string { return set.schemas[partsURL+at+".json"].refsUnder(nil) }
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
			set.leaveOut(name, usererr.New("tmpl_part_cycle", dir, name, strings.Join(chain, " → ")))
		} else if chain := cycle(name, looks); chain != nil {
			set.leaveOut(name, usererr.New("tmpl_look_cycle", dir, name, strings.Join(chain, " → ")))
		}
	}
	settle := func() {
		for left := true; left; {
			left = false
			for _, name := range slices.Sorted(maps.Keys(set.parts)) {
				if err := set.checkNames(name, set.parts[name].dir); err != nil {
					set.leaveOut(name, err)
					left = true
				}
			}
		}
	}
	settle()
	// Once the rest are settled, and all at once, so that the part a root names is surely usable or not, and
	// the answer hangs on no order; the parts that hold one left out so go then.
	rooted := map[string]error{}
	for name, p := range set.parts {
		if named, ok := set.rootPart(p.schema); ok {
			at := cmp.Or(p.dir, path.Join("parts", name))
			rooted[name] = usererr.New("tmpl_schema_root_part", path.Join(at, "schema.json"), named)
		}
	}
	for name, why := range rooted {
		set.leaveOut(name, why)
	}
	if len(rooted) > 0 {
		settle()
	}
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
		set.broken[name] = usererr.New("tmpl_part_name", at, name)
		return false
	}
	floe := set.parts[name]
	replaces := floe != nil
	set.drop(name)
	if err == nil {
		err = readIn(dir, func(files fs.FS) error { return set.readPart(files, name, at) })
	}
	if err == nil && replaces && !floe.isLook() && set.parts[name].isLook() {
		err = usererr.New("tmpl_look_replaces", at, name)
	}
	if err != nil {
		set.leaveOut(name, err)
		return replaces
	}
	set.parts[name].dir, set.parts[name].replaces = at, replaces
	return replaces
}

// readOwnThemes reads the parts themes of one's own in dir into set, a directory each, a symlink
// followed: one of the name of one of floe's takes the place of its files of their names. Those that
// cannot be used are left out of set, and kept in set.brokenThemes.
func (set *partSet) readOwnThemes(dir string) error {
	themes, err := dirAt(dir)
	if _, there := os.Lstat(dir); themes == "" && there == nil {
		// A file, or a symlink that leads nowhere, where the themes go: none of them is read.
		why := "not a directory"
		if err != nil {
			why = err.Error()
		}
		set.strays = append(set.strays, usererr.New("tmpl_parts_themes_unreadable", dir, why))
		return nil
	}
	entries, err := entriesOf(dir)
	if err != nil {
		return usererr.New("tmpl_parts_dir_unreadable", dir, err.Error())
	}
	set.keepDir(themes)
	for _, e := range entries {
		name, at := e.Name(), filepath.Join(dir, e.Name())
		if strings.HasPrefix(name, ".") {
			continue
		}
		t := set.themes[name]
		if t == nil {
			t = newPartsTheme()
		}
		dir, err := set.ownEntry(name, at, at)
		if dir == "" && err == nil {
			continue
		}
		if err == nil {
			err = readIn(dir, func(files fs.FS) error { return set.readPartsTheme(files, at, t) })
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

// ownEntry returns the directory of name, a part or a parts theme of one's own at at, read from from, as
// dirAt does: "" for a file. One of a name no part or theme can take is a mistake. It keeps the directory among those
// set read, for a link drawn with set to be drawn again when a file in it is saved.
func (set *partSet) ownEntry(name, from, at string) (string, error) {
	dir, err := dirAt(from)
	switch {
	case dir == "" && err == nil:
		return "", nil
	case !validName(name):
		return "", usererr.New("tmpl_part_bad_name", at)
	}
	set.keepDir(dir)
	return dir, err
}

// keepDir keeps dir among the directories set read, but one under the first, the directory of parts, as
// a reload walks that whole.
func (set *partSet) keepDir(dir string) {
	if dir == "" {
		return
	}
	if set.dirs != nil && (dir == set.dirs[0] || strings.HasPrefix(dir, set.dirs[0]+string(filepath.Separator))) {
		return
	}
	set.dirs = append(set.dirs, dir)
}

// reserved reports whether a part of one's own cannot be named name: a part floe draws in Go, a schema
// the parts share (tone), or a file of a parts theme that is no part's (base, palette).
func (set *partSet) reserved(name string) bool {
	_, schema := set.docs[partsURL+name+".json"]
	return isGoPart(name) || schema && set.parts[name] == nil || name == themeBase || name == themePalette
}

// checkNames refuses the part name, whose directory is dir, when its part.html names it in no
// data-floe-part, names more than one part in one, or names a part there is none of, when part.yaml has
// it take the look of a part there is none of, and when its schema holds a part there is none of, or a
// look alone. floe's parts name themselves, one each, as a test checks.
func (set *partSet) checkNames(name, dir string) error {
	p := set.parts[name]
	// floe's are named as reading them names them.
	at := cmp.Or(dir, path.Join("parts", name))
	html, yaml := path.Join(at, "part.html"), path.Join(at, "part.yaml")
	named := namedParts(p.html)
	if dir != "" && !p.isLook() && !namesItself(named, name) {
		return usererr.New("tmpl_part_unnamed", html, name, name)
	}
	for _, n := range named {
		kind := set.misnamed(n)
		if kind == nameRight {
			continue
		}
		line := lineAt([]byte(p.html), n.at)
		switch {
		case dir != "" && kind == nameNotOne:
			return usererr.New("tmpl_part_several", html, line, n.value)
		case dir != "" && kind == nameLookBox:
			return usererr.New("tmpl_look_box", html, line, n.name)
		}
		if err := set.unlent(kind, n.name, html, line, n.look); err != nil {
			return err
		}
	}
	for _, look := range p.looks {
		if err := set.unlent(set.known(look.name), look.name, yaml, look.line, true); err != nil {
			return err
		}
	}
	return set.checkHeld(p, at, dir != "")
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
		return usererr.New("tmpl_part_lender_broken", name)
	case nameUnknown:
		return usererr.New("tmpl_part_names", file, line, name, set.candidates(look))
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

// checkHeld refuses the part p, whose files are at at, when its schema holds a part there is none of, of
// one's own when own, or a look alone.
func (set *partSet) checkHeld(p *part, at string, own bool) error {
	// One left out has no schema; one of a name set keeps (tone) leaves floe's in place.
	for _, held := range p.schema.refsUnder(nil) {
		switch {
		case set.schemas[partsURL+held+".json"] != nil:
		case set.broken[held] != nil:
			return usererr.New("tmpl_part_holder_broken", held)
		case set.parts[held] != nil && set.parts[held].isLook():
			return usererr.New("tmpl_part_holds_look", path.Join(at, "schema.json"), held)
		case own:
			return usererr.New("tmpl_part_holds_unknown", path.Join(at, "schema.json"), held, set.names(drawing))
		}
	}
	return nil
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
		return nil, usererr.New("tmpl_part_broken", name, set.broken[name])
	}
	return nil, usererr.New("tmpl_unknown_part", name, set.names(nil))
}

// unusable returns the mistake of each part of set that cannot be used, by name, but those skip skips (none
// for nil).
func (set *partSet) unusable(skip func(name string) bool) []*usererr.Error {
	var found []*usererr.Error
	for _, name := range slices.Sorted(maps.Keys(set.broken)) {
		if skip == nil || !skip(name) {
			found = append(found, usererr.New("tmpl_part_broken", name, set.broken[name]))
		}
	}
	return found
}

// unusableThemes returns the mistake of each parts theme of set that cannot be used, by name, but those
// skip skips (none for nil).
func (set *partSet) unusableThemes(skip func(name string) bool) []*usererr.Error {
	var found []*usererr.Error
	for _, name := range slices.Sorted(maps.Keys(set.brokenThemes)) {
		if skip == nil || !skip(name) {
			found = append(found, usererr.New("tmpl_parts_theme_broken", name, set.brokenThemes[name]))
		}
	}
	return found
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

// refsUnder adds to names the parts s and the schemas under it, as written, refer to.
func (s *schema) refsUnder(names []string) []string {
	if s == nil {
		return names
	}
	if name, ok := partName(s.Ref); ok {
		names = append(names, name)
	}
	for _, key := range slices.Sorted(maps.Keys(s.Properties)) {
		names = s.Properties[key].refsUnder(names)
	}
	return s.Items.refsUnder(names)
}

// drop leaves the part name out of set, with its schema.
func (set *partSet) drop(name string) {
	url := partsURL + name + ".json"
	delete(set.parts, name)
	delete(set.docs, url)
	delete(set.schemas, url)
}
