package schema

import (
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Step is how floe goes one step from a schema to the values under it: the part (its URL and the root of its
// schema) or the format that draws the value, else the schema of each property by name, and of the items.
type Step struct {
	Part, Format string
	PartRoot     *jsonschema.Schema
	Props        map[string]*jsonschema.Schema
	Items        *jsonschema.Schema
	closed       []*jsonschema.Schema // those gone through with additionalProperties: false
}

// Forbids reports whether a schema the step went through forbids the key in an object: its
// additionalProperties is false, and neither its own properties nor its patternProperties take the key, as
// jsonschema reads them. A schema of additionalProperties that refuses all ({"not": {}}) is not taken.
func (st Step) Forbids(key string) bool {
	for _, s := range st.closed {
		if _, ok := s.Properties[key]; ok {
			continue
		}
		if !slices.ContainsFunc(slices.Collect(maps.Keys(s.PatternProperties)), func(re jsonschema.Regexp) bool {
			return re.MatchString(key)
		}) {
			return true
		}
	}
	return false
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
	st, through := c.step(s, map[*jsonschema.Schema]bool{})
	if st.Part == "" && st.Format == "" {
		for _, s := range through {
			if closed, ok := s.AdditionalProperties.(bool); ok && !closed {
				st.closed = append(st.closed, s)
			}
		}
	}
	return st
}

// step returns the step of s and the schemas it went through to it: s, and those its $refs name.
func (c *Compiled) step(s *jsonschema.Schema, seen map[*jsonschema.Schema]bool) (Step, []*jsonschema.Schema) {
	through := make([]*jsonschema.Schema, 1, 2)
	through[0] = s
	own := Step{Part: "", Format: "", PartRoot: nil, Props: s.Properties, Items: itemsOf(s), closed: nil}
	if seen[s] {
		return Step{Part: "", Format: "", PartRoot: nil, Props: nil, Items: nil, closed: nil}, through
	}
	seen[s] = true
	if f := floeFormat(s); f != "" {
		return Step{Part: "", Format: f, PartRoot: nil, Props: nil, Items: nil, closed: nil}, through
	}
	if s.Ref == nil {
		return own, through
	}
	if part, ok := c.partRoot(s.Ref); ok {
		return Step{Part: part, Format: "", PartRoot: s.Ref, Props: nil, Items: nil, closed: nil}, through
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

// Item is a value a part or a format of floe draws whole: where it is (the keys and indexes to it), the name of
// the part or the format, and the value as written.
type Item struct {
	At     []string
	Part   string   // "" for a format
	Holds  []string // the parts the part's schema may hold, sorted
	Format string   // "" for a part
	Value  any
}

// Walk goes through values as c takes them, a step of Follow at each value, into a part's value from the root
// of its schema, and calls fn with each Item, those a part's value holds before it, and with its value made
// ready: the Items in it replaced by what fn returned for them. It returns values made ready so, each item
// the schema declares but values leave out made nil, so that a template can test it under missingkey=error.
// It is the one walk of values: what floe checks of the items and what it draws both come from it.
// LIMIT: an object's value written whole ({{.x}}) shows Go's map as text, unchecked; the docs range over any.json's.
// Pass any.json's value as its one part drawn if templates come to need it.
func (c *Compiled) Walk(values any, fn func(it Item, ready any) (any, error)) (any, error) {
	w := &walk{c: c, fn: fn, holds: map[*jsonschema.Schema][]string{}}
	return w.step(c.Follow(c.Root), values, nil)
}

// walk is a walk of values: the Compiled it goes by, what it calls with each Item, and the parts the schema of
// each part it met may hold, by the root of the part's schema.
type walk struct {
	c     *Compiled
	fn    func(Item, any) (any, error)
	holds map[*jsonschema.Schema][]string
}

// step walks v, at the path at, under the step st.
func (w *walk) step(st Step, v any, at []string) (any, error) {
	switch {
	case st.Part != "":
		ready, err := w.step(w.c.Follow(st.PartRoot), v, at)
		if err != nil {
			return nil, err
		}
		holds, ok := w.holds[st.PartRoot]
		if !ok {
			holds = w.c.partsFrom(st.PartRoot)
			w.holds[st.PartRoot] = holds
		}
		return w.fn(Item{At: at, Part: partName(st.Part), Holds: holds, Format: "", Value: v}, ready)
	case st.Format != "":
		return w.fn(Item{At: at, Part: "", Holds: nil, Format: st.Format, Value: v}, v)
	}
	return w.under(st, v, at)
}

// under walks what v, at the path at, holds under the step st: its properties, its items.
func (w *walk) under(st Step, v any, at []string) (any, error) {
	var err error
	switch v := v.(type) {
	case map[string]any:
		ready := make(map[string]any, len(v)+len(st.Props))
		for k := range st.Props {
			ready[k] = nil
		}
		for _, k := range slices.Sorted(maps.Keys(v)) {
			if ready[k], err = w.in(st.Props[k], v[k], at, k); err != nil {
				return nil, err
			}
		}
		return ready, nil
	case []any:
		ready := make([]any, len(v))
		for i, item := range v {
			if ready[i], err = w.in(st.Items, item, at, strconv.Itoa(i)); err != nil {
				return nil, err
			}
		}
		return ready, nil
	}
	return v, nil
}

// in walks v, at the step named step under the path at, with the schema s; v as it is for none.
func (w *walk) in(s *jsonschema.Schema, v any, at []string, step string) (any, error) {
	if s == nil {
		return v, nil
	}
	return w.step(w.c.Follow(s), v, slices.Concat(at, []string{step}))
}

// partsFrom returns the names of the parts Follow, repeated from root, stops at, sorted.
func (c *Compiled) partsFrom(root *jsonschema.Schema) []string {
	var names []string
	for s := range c.followFrom(root).atPart {
		if name := partName(c.Follow(s).Part); !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
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
	return base, c.held(base, Part) && frag == ""
}

// followed reports whether Follow goes from s through its $ref to to: one in the same file, or in a Shared
// schema.
func (c *Compiled) followed(s, to *jsonschema.Schema) bool {
	base, _ := split(to.Location)
	if from, _ := split(s.Location); base == from {
		return true
	}
	return c.isShared(base)
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

// Forbidding returns the first of steps, a path of keys from the root of c, that a schema on the way forbids
// (Step.Forbids), and false for none. A list on the way is gone through by its items, the step kept, as a
// check's for is; a part's value from its schema's root. Where the steps lead to what Follow does not know
// (a format, a schema of neither the key nor items, one met again in going through items), the rest is not
// known to be forbidden, nor is a key declared where items are too, as the value may be a list.
func (c *Compiled) Forbidding(steps []string) (string, bool) {
	s := c.Root
	for _, key := range steps {
		seen := map[*jsonschema.Schema]bool{}
	step:
		for {
			if seen[s] {
				return "", false
			}
			seen[s] = true
			st := c.Follow(s)
			next, declared := st.Props[key]
			switch {
			case st.Part != "":
				s = st.PartRoot
			case declared && st.Items != nil:
				// Keys and items: the value may be a list or an object, which lead on apart.
				return "", false
			case declared && st.Forbids(key):
				// Declared beside a $ref to a schema of additionalProperties: false, which takes it not.
				return key, true
			case declared:
				s = next
				break step
			case st.Items != nil:
				s = st.Items
			case st.Forbids(key):
				return key, true
			default:
				return "", false
			}
		}
	}
	return "", false
}
