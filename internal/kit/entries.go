package kit

import (
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/schema"
)

// Source is a file as it was read: its name in its directory, and its text.
type Source struct {
	Name string
	Text []byte
}

// sources returns files but those not read (a nil Text).
func sources(files ...Source) []Source {
	return slices.DeleteFunc(files, func(s Source) bool { return s.Text == nil })
}

// shared is a schema the parts share, read.
type shared struct {
	name, file, about string
	files             []Source
}

// Entry is a part, a look, a shared schema or a template as floe part list and show and floe template list
// and show tell of it.
type Entry struct {
	Name  string
	About string   // its description, on one line
	Dir   string   // its directory (a shared schema's file) when it is one's own, "" for floe's
	Looks []string // the looks it takes
	Files []Source // what says what a data file may hold, and its example
}

// Parts returns the parts, looks and shared schemas of k and its base, by name.
func (k *Kit) Parts() []Entry {
	var all []Entry
	for at := k; at != nil; at = at.base {
		for _, p := range at.parts {
			all = append(all, Entry{Name: p.name, About: about(p.schema), Dir: at.own(p.dir), Looks: p.Looks(), Files: p.files})
		}
		for _, l := range at.looks {
			all = append(all, Entry{Name: l.name, About: l.about, Dir: at.own(l.dir), Looks: l.Looks(), Files: nil})
		}
		for _, s := range at.shared {
			all = append(all, Entry{Name: s.name, About: s.about, Dir: at.own(s.file), Looks: nil, Files: s.files})
		}
	}
	return listed(all)
}

// Templates returns the templates of k and its base, by name: of a name both have, k's own, which hides the
// base's.
func (k *Kit) Templates() []Entry {
	var all []Entry
	for at := k; at != nil; at = at.base {
		for _, t := range at.templates {
			if !slices.ContainsFunc(all, func(e Entry) bool { return e.Name == t.name }) {
				all = append(all, Entry{Name: t.name, About: about(t.schema), Dir: at.own(t.dir), Looks: nil, Files: t.files})
			}
		}
	}
	return listed(all)
}

// own returns dir when k reads one's own over a base, "" when k is floe's.
func (k *Kit) own(dir string) string {
	if k.base == nil {
		return ""
	}
	return dir
}

func about(c *schema.Compiled) string {
	if c == nil {
		return ""
	}
	return c.Root.Description
}

// listed returns all by name, each description on one line, its line breaks, tabs and runs of spaces one
// space: floe part list and template list print one entry a line, in columns.
func listed(all []Entry) []Entry {
	for i := range all {
		all[i].About = strings.Join(strings.Fields(all[i].About), " ")
	}
	slices.SortFunc(all, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return all
}
