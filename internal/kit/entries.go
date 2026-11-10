package kit

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"floe/internal/bom"
	"floe/internal/doc"
	"floe/internal/schema"
)

// Source is a file as it was read: its name in its directory, and its text without a byte order mark at its start.
type Source struct {
	Name string
	Text []byte
}

// inOrder returns the files of names that d read (recording), in that order, each without the byte order mark at
// its start, as show prints them.
func inOrder(names []string, d Dir) []Source {
	var files []Source
	for _, name := range names {
		if src, ok := d.read[name]; ok {
			files = append(files, Source{Name: name, Text: bom.Trim(src)})
		}
	}
	return files
}

// copied returns a copy of files, their texts too: the Kit stays as read, Floe's shared by the whole process.
func copied(files []Source) []Source {
	out := make([]Source, len(files))
	for i, f := range files {
		out[i] = Source{Name: f.Name, Text: bytes.Clone(f.Text)}
	}
	return out
}

// shared is a schema the parts share, read.
type shared struct {
	name, file string
	schema     *schema.Compiled
	files      []Source
}

// Entry is a part, a look, a shared schema, a template, a theme or a diagram example as floe's lists and shows tell
// of it.
type Entry struct {
	Name  string
	About string   // its description, on one line
	Shape string   // what a value of it is written as (schema.Shape), "-" when the list cannot tell it
	Dir   string   // its directory (a shared schema's file) when it is one's own, "" for floe's
	Looks []string // the looks it takes
	Files []Source // the files it reads, as written, in the order show prints them
}

// Parts returns the parts, looks and shared schemas of k and its base, by name.
func (k *Kit) Parts() []Entry {
	var all []Entry
	for at := k; at != nil; at = at.base {
		for _, p := range at.parts {
			all = append(all, at.partEntry(p))
		}
		for _, l := range at.looks {
			all = append(all, Entry{
				Name: l.name, About: l.about, Shape: "-", Dir: at.own(l.dir), Looks: l.Looks(), Files: copied(l.files),
			})
		}
		for _, s := range at.shared {
			all = append(all, Entry{
				Name: s.name, About: about(s.schema), Shape: rootShape(s.schema), Dir: at.own(s.file), Looks: nil,
				Files: copied(s.files),
			})
		}
	}
	return Listed(all)
}

// partEntry is the entry of p, a part of k.
func (k *Kit) partEntry(p *Part) Entry {
	return Entry{
		Name: p.name, About: about(p.schema), Shape: rootShape(p.schema), Dir: k.own(p.dir), Looks: p.Looks(),
		Files: copied(p.files),
	}
}

// SectionParts returns what a section of floe's templates takes, by name: a key of floe's shared schema any, each
// the part it names or, for one of a format (html), the key itself, described by any.json, and the parts of one's own
// over it, which any takes too. floe's are read from floe's Kit under k, so that a part of one's own named as a key
// does not stand for it. A key that is neither is floe's failure.
func (k *Kit) SectionParts() ([]Entry, error) {
	floe := k
	for floe.base != nil {
		floe = floe.base
	}
	anyOf := floe.shared["any"]
	if anyOf == nil {
		return nil, errors.New("kit: floe has no shared schema any") //nolint:err113 // floe's failure
	}
	c := anyOf.schema
	props := c.Follow(c.Root).Props
	all := make([]Entry, 0, len(props))
	for _, name := range slices.Sorted(maps.Keys(props)) {
		step := c.Follow(props[name])
		switch p, ok := floe.parts[schema.PartName(step.Part)]; {
		case step.Part != "" && ok:
			all = append(all, floe.partEntry(p))
		case step.Format != "":
			all = append(all, Entry{
				Name: name, About: props[name].Description, Shape: c.Shape(props[name]), Dir: "", Looks: nil,
				Files: copied(anyOf.files),
			})
		default:
			//nolint:err113 // floe's failure
			return nil, fmt.Errorf("kit: the key %s of any.json is neither a part of floe nor a format", name)
		}
	}
	for at := k; at != floe; at = at.base {
		for _, p := range at.parts {
			all = append(all, at.partEntry(p))
		}
	}
	return Listed(all), nil
}

// Templates returns the templates of k and its base, by name: of a name both have, k's own, which hides the
// base's.
func (k *Kit) Templates() []Entry {
	var all []Entry
	for at := k; at != nil; at = at.base {
		for _, t := range at.templates {
			if !slices.ContainsFunc(all, func(e Entry) bool { return e.Name == t.name }) {
				all = append(all, Entry{
					Name: t.name, About: about(t.schema), Shape: rootShape(t.schema, doc.Shared...), Dir: at.own(t.dir),
					Looks: nil, Files: copied(t.files),
				})
			}
		}
	}
	return Listed(all)
}

// Themes returns the themes of k and its base, by name. One's own never has a name of the base's.
func (k *Kit) Themes() []Entry {
	var all []Entry
	for at := k; at != nil; at = at.base {
		for _, t := range at.themes {
			all = append(all, Entry{
				Name: t.name, About: t.about, Shape: "-", Dir: at.own(t.dir), Looks: nil, Files: copied(t.files),
			})
		}
	}
	return Listed(all)
}

// own returns dir when k reads one's own over a base, "" when k is floe's.
func (k *Kit) own(dir string) string {
	if k.base == nil {
		return ""
	}
	return dir
}

// rootShape is the shape of the root of c (schema.Shape), with more keys floe reads itself in a template; "-" for
// none.
func rootShape(c *schema.Compiled, more ...string) string {
	if c == nil {
		return "-"
	}
	return c.Shape(c.Root, more...)
}

func about(c *schema.Compiled) string {
	if c == nil {
		return ""
	}
	return c.Root.Description
}

// Listed returns all by name, each description on one line, its line breaks, tabs and runs of spaces one
// space: floe's lists print one entry a line, in columns.
func Listed(all []Entry) []Entry {
	for i := range all {
		all[i].About = strings.Join(strings.Fields(all[i].About), " ")
	}
	slices.SortFunc(all, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	return all
}
