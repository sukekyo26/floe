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
	Title string // schema.json's title and description
	About string
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
		Name: name, Path: t.Path, Title: s.Title, About: s.Description, Floe: "", Series: "", Themes: themes,
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

// items are the items s declares, by name.
func (s *schema) items() []Item {
	names := slices.Sorted(maps.Keys(s.Properties))
	items := make([]Item, 0, len(names))
	for _, n := range names {
		p := s.Properties[n]
		it := Item{
			Name: n, Types: p.types(), Of: nil, Format: p.Format, Enum: nil, Required: slices.Contains(s.Required, n),
			Description: p.Description, Items: nil,
		}
		for _, e := range p.Enum {
			it.Enum = append(it.Enum, fmt.Sprint(e))
		}
		if p.Items != nil {
			// A list described by its items: "a list of …", each one what it holds.
			it.Of, it.Items = p.Items.types(), p.Items.items()
			it.Description = cmp.Or(it.Description, p.Items.Description)
		} else {
			it.Items = p.items()
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
