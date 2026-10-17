package tmpl

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
)

// Description is what floe template list and show tell of a template.
type Description struct {
	Name  string
	Path  string // where a template of one's own is; "" for a built-in one
	About string // schema.json's description
	// Floe is the floe template.yaml needs, major.minor.patch, and Series the versions that take what
	// it takes (its major, or its minor under major 0); "" without template.yaml.
	Floe    string
	Series  string
	Themes  []string
	Items   []Item // the items a data file may hold
	Example []byte // example.yaml; nil without one
}

// Item is an item a template's schema declares.
type Item struct {
	Name        string
	Types       []string // JSON types: string, number, integer, boolean, object, array, null
	Of          []string // the types of the items of a list
	Format      string
	Part        string // the part of floe's that draws it; "" for none
	Enum        []string
	Required    bool
	Description string
	Items       []Item // the items of keys and values, or of the keys and values a list holds
}

// Describe tells of the template name.
func (l *Library) Describe(name string) (*Description, error) {
	t, err := l.Find(name)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, l.unknown(name)
	}
	s, err := schemaOf(t)
	if err != nil {
		return nil, err
	}
	themes, err := t.themes()
	if err != nil {
		return nil, err
	}
	d := &Description{
		Name: name, Path: t.Path, About: s.Description, Floe: "", Series: "", Themes: themes,
		Items: s.items(), Example: nil,
	}
	needs, err := t.needs()
	if err != nil {
		return nil, err
	}
	if needs != nil {
		d.Floe, d.Series = needs.String(), needs.series()
	}
	if d.Example, _, err = t.readOptional("example.yaml"); err != nil {
		return nil, err
	}
	return d, nil
}

// SharedItems are the keys floe reads from every data file of the template name itself, to be told beside
// the items of its schema; their descriptions are floe's, for the caller to word.
func SharedItems(name string) []Item {
	items := make([]Item, 0, len(shared))
	for _, key := range shared {
		it := Item{
			Name: key, Types: []string{"string"}, Of: nil, Format: "", Part: "", Enum: nil, Required: false, Description: "",
			Items: nil,
		}
		switch key {
		case "lang":
			it.Required = true
		case "template":
			it.Enum, it.Required = []string{name}, true
		}
		items = append(items, it)
	}
	return items
}

// items are the items s declares, by name.
func (s *schema) items() []Item {
	names := slices.Sorted(maps.Keys(s.Properties))
	if len(names) == 0 {
		return nil
	}
	items := make([]Item, 0, len(names))
	for _, n := range names {
		p := s.Properties[n]
		if p == nil {
			p = new(schema) // null in schema.json: nothing to tell of it
		}
		it := Item{
			Name: n, Types: p.types(), Of: nil, Format: p.Format, Part: "", Enum: nil,
			Required: slices.Contains(s.Required, n), Description: p.Description, Items: nil,
		}
		if p.part != nil {
			it.Part = p.part.name
		}
		enum := p.Enum
		if p.Items != nil {
			// A list described by its items: "a list of …", each one what it holds.
			it.Of, it.Items = p.Items.types(), p.Items.items()
			it.Format = cmp.Or(it.Format, p.Items.Format)
			it.Description = cmp.Or(it.Description, p.Items.Description)
			if len(enum) == 0 {
				enum = p.Items.Enum
			}
		} else {
			it.Items = p.items()
		}
		for _, e := range enum {
			it.Enum = append(it.Enum, jsonText(e))
		}
		items = append(items, it)
	}
	return items
}

// types are the JSON types s takes: those it names, else those its keywords imply.
func (s *schema) types() []string {
	switch t := s.Type.(type) {
	case string:
		return []string{t}
	case []any:
		types := make([]string, 0, len(t))
		for _, v := range t {
			types = append(types, fmt.Sprint(v))
		}
		return types
	}
	switch {
	case s.Properties != nil:
		return []string{"object"}
	case s.Items != nil:
		return []string{"array"}
	}
	return nil
}
