package tmpl

import (
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
	return readParts(dir, entries)
}

// readOwn reads the parts of one's own in dir into set, over floe's of their names, a directory each, a
// symlink followed. Those that cannot be used are left out of set, and kept in set.broken.
func (set *partSet) readOwn(dir string, entries []os.DirEntry) error {
	if parts, err := filepath.EvalSymlinks(dir); err == nil {
		set.dirs = append(set.dirs, parts)
	}
	own := map[string]string{} // the directory of each, as its mistakes name it
	var replaced []string
	for _, e := range entries {
		name, at := e.Name(), filepath.Join(dir, e.Name())
		if name == themesDir || strings.HasPrefix(name, ".") {
			continue
		}
		read, replaces := set.readOwnPart(name, at)
		if replaces {
			replaced = append(replaced, name)
		}
		if read {
			own[name] = at
		}
	}
	set.leaveOutUnusable(own)
	// floe's themes style floe's parts alone: their rules would fit a part of one's own of the name ill.
	for _, t := range set.themes {
		for _, name := range replaced {
			delete(t.parts, name)
		}
		t.except = replaced
	}
	return set.readOwnThemes(filepath.Join(dir, themesDir))
}

// leaveOutUnusable leaves out of set the parts that cannot be used once all are read: one of one's own,
// in own by its directory, that holds itself, through the parts it holds, which floe would follow
// forever, and a part, floe's too, that checkNames refuses, again while one is left out, as a part may
// take the look of the one left out, or hold it.
func (set *partSet) leaveOutUnusable(own map[string]string) {
	for _, name := range slices.Sorted(maps.Keys(own)) {
		if held := set.holdsItself(name); held != nil {
			set.leaveOut(name, usererr.New("tmpl_part_cycle", own[name], name, strings.Join(held, " → ")))
			delete(own, name)
		}
	}
	for left := true; left; {
		left = false
		for _, name := range slices.Sorted(maps.Keys(set.parts)) {
			if err := set.checkNames(name, own[name]); err != nil {
				set.leaveOut(name, err)
				delete(own, name)
				left = true
			}
		}
	}
}

// readOwnPart reads the part name of one's own in the directory at into set, a symlink followed, and
// one in it that leads out of it refused, and reports whether it read it and whether it took the place
// of floe's of the name. One that cannot be used is kept in set.broken, and one of a name set keeps
// leaves floe's in place.
func (set *partSet) readOwnPart(name, at string) (read, replaces bool) {
	dir, err := set.ownEntry(name, at)
	switch {
	case dir == "" && err == nil:
		return false, false
	case set.reserved(name):
		// Told whatever is there, which never takes the place of floe's.
		set.broken[name] = usererr.New("tmpl_part_name", at, name)
		return false, false
	}
	replaces = set.parts[name] != nil
	set.drop(name)
	if err == nil {
		err = readIn(dir, func(files fs.FS) error { return set.readPart(files, name, at) })
	}
	if err != nil {
		set.leaveOut(name, err)
		return false, replaces
	}
	return true, replaces
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
		dir, err := set.ownEntry(name, at)
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

// ownEntry returns the directory of name, a part or a parts theme of one's own at at, as dirAt does: ""
// for a file. One of a name no part or theme can take is a mistake. It keeps the directory among those
// set read, for a link drawn with set to be drawn again when a file in it is saved.
func (set *partSet) ownEntry(name, at string) (string, error) {
	dir, err := dirAt(at)
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
// the parts share (tone), or a file of a parts theme (base and parts).
func (set *partSet) reserved(name string) bool {
	_, schema := set.docs[partsURL+name+".json"]
	return isGoPart(name) || schema && set.parts[name] == nil || name == themeBase || name == themeShared
}

// checkNames refuses the part name, whose directory is dir, when its part.html does not name it in a
// data-floe-part, or names a part there is none of.
func (set *partSet) checkNames(name, dir string) error {
	p := set.parts[name]
	names := namedParts(p.html)
	if dir != "" && !namesItselfLast(p.html, name) {
		return usererr.New("tmpl_part_unnamed", path.Join(dir, "part.html"), name, name)
	}
	for _, n := range names {
		switch _, ok := set.parts[n.name]; {
		case ok:
		case set.broken[n.name] != nil:
			return usererr.New("tmpl_part_lender_broken", n.name)
		default:
			return usererr.New("tmpl_part_names", path.Join(dir, "part.html"), lineAt([]byte(p.html), n.at), n.name,
				strings.Join(slices.Sorted(maps.Keys(set.parts)), ", "))
		}
	}
	// One left out has no schema; one of a name set keeps (tone) leaves floe's in place.
	for _, held := range p.schema.refsUnder(nil) {
		switch {
		case set.schemas[partsURL+held+".json"] != nil:
		case set.broken[held] != nil:
			return usererr.New("tmpl_part_holder_broken", held)
		case dir != "":
			return usererr.New("tmpl_part_holds_unknown", path.Join(dir, "schema.json"), held,
				strings.Join(slices.Sorted(maps.Keys(set.parts)), ", "))
		}
	}
	return nil
}

// namesItselfLast reports whether an element of html, the HTML of the part name, names it last in its
// data-floe-part, as everyPart's selector reads it: the value name, or ending in a space and name.
// floe's parts do, as a test checks.
func namesItselfLast(html, name string) bool {
	for _, m := range partAttr.FindAllStringSubmatch(html, -1) {
		if v := strings.Trim(m[1], `"'`); v == name || strings.HasSuffix(v, " "+name) {
			return true
		}
	}
	return false
}

// leaveOut leaves the part name out of set, kept in set.broken with why.
func (set *partSet) leaveOut(name string, why error) {
	set.broken[name] = why
	set.drop(name)
}

// holdsItself returns the parts the part name holds, by the $refs of its schema, that lead back to it,
// name first and last, or nil when none do.
func (set *partSet) holdsItself(name string) []string {
	var held []string
	seen := map[string]bool{}
	var reach func(at string) bool
	reach = func(at string) bool {
		held = append(held, at)
		for _, next := range set.schemas[partsURL+at+".json"].refsUnder(nil) {
			if next == name {
				held = append(held, next)
				return true
			}
			if !seen[next] {
				seen[next] = true
				if reach(next) {
					return true
				}
			}
		}
		held = held[:len(held)-1]
		return false
	}
	if reach(name) {
		return held
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
