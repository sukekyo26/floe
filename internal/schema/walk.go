package schema

import (
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Step is how floe goes one step from a schema to the values under it: the part (its URL) or the format that
// draws the value, else the schema of each property by name, and of the items.
type Step struct {
	Part, Format string
	Props        map[string]*jsonschema.Schema
	Items        *jsonschema.Schema
}

// Follow returns the step of s, looking in this order: one of Formats on s stops there (drawn by floe,
// whatever $ref s has); a $ref of s to a Part's root stops there (drawn by the part); a $ref of s to the same
// file or to a Shared schema is followed first, and when the step there is a Part or a format, that is the
// step of s too (the properties beside the $ref are not taken: the value is drawn whole); else the
// Properties and Items2020 (Items before 2020-12) of s, together with those of the step the $ref names: a
// name s declares itself is taken from s, the rest from the $ref. Nothing else is a step (allOf, anyOf,
// oneOf, additionalProperties, patternProperties, prefixItems, if/then/else, dependencies, ...). A $ref back
// to a schema it followed ends there.
// LIMIT: what floe draws under allOf and the like, and under the array form of items and additionalItems
// (2019-09), is refused as unreached, not drawn.
func (c *Compiled) Follow(s *jsonschema.Schema) Step {
	st, _ := c.step(s, map[*jsonschema.Schema]bool{})
	return st
}

// step returns the step of s and the schemas it went through to it: s, and those its $refs name.
func (c *Compiled) step(s *jsonschema.Schema, seen map[*jsonschema.Schema]bool) (Step, []*jsonschema.Schema) {
	through := make([]*jsonschema.Schema, 1, 2)
	through[0] = s
	own := Step{Part: "", Format: "", Props: s.Properties, Items: itemsOf(s)}
	if seen[s] {
		return Step{Part: "", Format: "", Props: nil, Items: nil}, through
	}
	seen[s] = true
	if f := floeFormat(s); f != "" {
		return Step{Part: "", Format: f, Props: nil, Items: nil}, through
	}
	if s.Ref == nil {
		return own, through
	}
	if part, ok := c.partRoot(s.Ref); ok {
		return Step{Part: part, Format: "", Props: nil, Items: nil}, through
	}
	if !c.followed(s, s.Ref) {
		return own, through
	}
	st, more := c.step(s.Ref, seen)
	through = append(through, more...)
	if st.Part != "" || st.Format != "" {
		return st, through
	}
	props := maps.Clone(st.Props)
	if props == nil {
		props = map[string]*jsonschema.Schema{}
	}
	maps.Copy(props, s.Properties)
	if own.Items == nil {
		own.Items = st.Items
	}
	own.Props = props
	return own, through
}

// itemsOf returns the schema of each item of s: Items2020, or before 2020-12 Items of one schema.
func itemsOf(s *jsonschema.Schema) *jsonschema.Schema {
	if s.Items2020 != nil {
		return s.Items2020
	}
	if one, ok := s.Items.(*jsonschema.Schema); ok {
		return one
	}
	return nil
}

// floeFormat returns the name of one of Formats s has, "" for none.
func floeFormat(s *jsonschema.Schema) string {
	if s.Format != nil && slices.Contains(Formats, s.Format.Name) {
		return s.Format.Name
	}
	return ""
}

// partRoot reports whether to, a schema a $ref names, is the root of a Part, and its URL.
func (c *Compiled) partRoot(to *jsonschema.Schema) (string, bool) {
	base, frag := split(to.Location)
	r, ok := c.res[base]
	return base, ok && base != c.url && r.Kind == Part && frag == ""
}

// followed reports whether Follow goes from s through its $ref to to: one in the same file, or in a Shared
// schema.
func (c *Compiled) followed(s, to *jsonschema.Schema) bool {
	base, _ := split(to.Location)
	if from, _ := split(s.Location); base == from {
		return true
	}
	r, ok := c.res[base]
	return ok && base != c.url && r.Kind == Shared
}

// split returns the URL of loc, a Location, and its fragment without "#".
func split(loc string) (base, frag string) {
	base, frag, _ = strings.Cut(loc, "#")
	return base, frag
}

// reached is what Follow, repeated from some schemas, goes through and stops at.
type reached struct {
	passed, atPart, atFormat map[*jsonschema.Schema]bool
}

// stops reports whether it stopped at a Part or a format.
func (r reached) stops() bool {
	return len(r.atPart) > 0 || len(r.atFormat) > 0
}

// followFrom repeats Follow from starts, breadth first, into the schemas of the properties and items of
// each step.
func (c *Compiled) followFrom(starts ...*jsonschema.Schema) reached {
	r := reached{
		passed: map[*jsonschema.Schema]bool{}, atPart: map[*jsonschema.Schema]bool{},
		atFormat: map[*jsonschema.Schema]bool{},
	}
	queue := slices.Clone(starts)
	// What was a start of a step; passed also holds what a step went through by its $ref, whose properties
	// of a name beside the $ref were not taken, so that it is a start of its own all the same.
	started := map[*jsonschema.Schema]bool{}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		if started[s] {
			continue
		}
		started[s] = true
		st, through := c.step(s, map[*jsonschema.Schema]bool{})
		for _, t := range through {
			r.passed[t] = true
			switch {
			case st.Part != "":
				r.atPart[t] = true
			case st.Format != "":
				r.atFormat[t] = true
			}
		}
		for _, name := range slices.Sorted(maps.Keys(st.Props)) {
			queue = append(queue, st.Props[name])
		}
		if st.Items != nil {
			queue = append(queue, st.Items)
		}
	}
	return r
}

// every calls visit with each schema it meets from start, through every exported field that holds a
// schema (a *Schema, a pointer to a struct holding one, a slice, a map or an any), each once. It goes into a
// schema whose Location is in a file enter takes, and else not; it calls stray with each schema it does not
// go into that is in no file floe holds (a metaschema the library holds), and the one that names it.
func (c *Compiled) every(start *jsonschema.Schema, enter func(base string) bool,
	visit func(s *jsonschema.Schema), stray func(from, to *jsonschema.Schema),
) {
	w := &walker{c: c, enter: enter, visit: visit, stray: stray, seen: map[*jsonschema.Schema]bool{}}
	w.schema(nil, start)
}

// walker is a walk of every.
type walker struct {
	c     *Compiled
	enter func(base string) bool
	visit func(s *jsonschema.Schema)
	stray func(from, to *jsonschema.Schema)
	seen  map[*jsonschema.Schema]bool
}

// schema visits s, met from from, and goes on into it.
func (w *walker) schema(from, s *jsonschema.Schema) {
	if s == nil || w.seen[s] {
		return
	}
	base, _ := split(s.Location)
	if !w.enter(base) {
		if _, held := w.c.res[base]; !held && base != w.c.url && from != nil {
			w.stray(from, s)
		}
		return
	}
	w.seen[s] = true
	w.visit(s)
	w.value(s, reflect.ValueOf(s).Elem())
}

// value goes into v, a field of from or what it holds, to the schemas in it.
func (w *walker) value(from *jsonschema.Schema, v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		if s, ok := v.Interface().(*jsonschema.Schema); ok {
			w.schema(from, s)
			return
		}
		w.value(from, v.Elem())
	case reflect.Interface:
		if !v.IsNil() {
			w.value(from, v.Elem())
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				w.value(from, v.Field(i))
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			w.value(from, v.Index(i))
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			w.value(from, it.Value())
		}
	default:
	}
}
