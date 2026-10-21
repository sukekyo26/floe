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
	Floe   string
	Series string
	Themes []string
	// PartsThemes are the themes of floe's parts, which a data file of any template may pick.
	PartsThemes []string
	Items       []Item // the items a data file may hold
	Example     []byte // example.yaml; nil without one
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
	s, _, err := t.shape()
	if err != nil {
		return nil, err
	}
	themes, err := t.themes()
	if err != nil {
		return nil, err
	}
	set, err := t.parts()
	if err != nil {
		return nil, err
	}
	partsThemes := set.themeNames()
	d := &Description{
		Name: name, Path: t.Path, About: s.description(), Floe: "", Series: "", Themes: themes, PartsThemes: partsThemes,
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

// PartDescription is what floe part list and show tell of a part.
type PartDescription struct {
	Name     string
	Path     string // where a part of one's own was found; "" for floe's
	Replaces bool   // whether it is one of one's own in place of floe's of its name
	Look     bool   // a look alone, which draws nothing, and so takes nothing
	About    string // its schema's description, or a look's part.yaml's
	// Takes is what it takes, the root of its schema: its Items are those of keys and values, or of the
	// keys and values a list of them holds.
	Takes  Item
	Looks  []string // the looks it takes, and those they take in turn, in the order they go into a page
	Holds  []string // the parts its schema holds, by name
	Themes []string // the parts themes with a file of it, by name
}

// Parts tells of every part, floe's and one's own, by name, and returns the mistakes of the parts that
// cannot be used, which it leaves out.
func (l *Library) Parts() (parts []*PartDescription, broken []error, err error) {
	set, err := partsIn(l.parts)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(set.parts)) {
		parts = append(parts, set.describe(name))
	}
	for _, why := range set.unusable(nil) {
		broken = append(broken, why)
	}
	return parts, broken, nil
}

// DescribePart tells of the part name.
func (l *Library) DescribePart(name string) (*PartDescription, error) {
	set, err := partsIn(l.parts)
	if err != nil {
		return nil, err
	}
	if _, err = set.find(name); err != nil {
		return nil, err
	}
	return set.describe(name), nil
}

// describe tells of the part name of set.
func (set *partSet) describe(name string) *PartDescription {
	p := set.parts[name]
	about := p.about
	var takes Item // nothing for a look
	var holds []string
	if !p.isLook() {
		about, takes = p.shape.description(), p.shape.item(name, false)
		holds = slices.Compact(slices.Sorted(slices.Values(p.shape.held(nil))))
	}
	return &PartDescription{
		Name: name, Path: p.dir, Replaces: p.replaces, Look: p.isLook(), About: about, Takes: takes,
		Looks: set.inOrder(p.taken), Holds: holds, Themes: set.themesWith(name),
	}
}

// held adds to names the parts s and the shapes under it hold, short of what those hold in turn.
func (s *shape) held(names []string) []string {
	switch {
	case s == nil:
		return names
	case s.part != nil:
		return append(names, s.part.name)
	}
	in := s.inside()
	for _, k := range in.keys {
		names = k.held(names)
	}
	return in.each.held(names)
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
func (s *shape) items() []Item {
	names := s.keyNames()
	if len(names) == 0 {
		return nil
	}
	in := s.inside()
	items := make([]Item, 0, len(names))
	for _, n := range names {
		items = append(items, in.keys[n].item(n, slices.Contains(in.required, n)))
	}
	return items
}

// item is the item name that s declares, required when required.
func (s *shape) item(name string, required bool) Item {
	if s == nil {
		s = new(shape) // null in schema.json: nothing to tell of it
	}
	it := Item{
		Name: name, Types: s.types(), Of: nil, Format: s.format, Part: "", Enum: nil, Required: required,
		Description: s.description(), Items: nil,
	}
	enum := s.values()
	if each := s.inside().each; each != nil {
		// A list described by its items: "a list of …", each one what it holds.
		it.Of, it.Items = each.types(), each.items()
		it.Format = cmp.Or(it.Format, each.format)
		it.Part = each.drawnBy()
		it.Description = cmp.Or(it.Description, each.description())
		if len(enum) == 0 {
			enum = each.values()
		}
	} else {
		it.Items = s.items()
	}
	it.Part = cmp.Or(s.drawnBy(), it.Part)
	for _, e := range enum {
		it.Enum = append(it.Enum, jsonText(e))
	}
	return it
}

// drawnBy is the name of the part of floe's that draws s, or "".
func (s *shape) drawnBy() string {
	if s.part == nil {
		return ""
	}
	return s.part.name
}

// types are the JSON types s takes: those it names, else those its keywords imply.
func (s *shape) types() []string {
	switch t := s.jsonType().(type) {
	case string:
		return []string{t}
	case []any:
		types := make([]string, 0, len(t))
		for _, v := range t {
			types = append(types, fmt.Sprint(v))
		}
		return types
	}
	switch in := s.inside(); {
	case in.keys != nil:
		return []string{"object"}
	case in.each != nil:
		return []string{"array"}
	}
	return nil
}
