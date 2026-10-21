package tmpl

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// shape is what floe reads of a schema, at each place it declares: read once from the schema as jsonschema
// reads it, for every stage that checks or draws the data, and for floe template show. A $ref to a part or
// a schema the parts share is kept as it names it, and bound once the set is final, without taking a copy
// of what it names, so that the shapes come out the same in whatever order they are read.
type shape struct {
	keys map[string]*shape // the items of keys and values it declares, by key; nil without properties
	each *shape            // each item of a list

	ref    string // its $ref, absolute; "" for none
	format string // as written: one of floe's (formats) draws the item whole

	// What its $ref names among floe's, bound once the set is final: the shape of a part or of a schema the
	// parts share, and the part, which draws the item whole. nil for none.
	named *shape
	part  *part

	// What floe template show and floe part show tell of it, as written beside a $ref.
	about    string
	typ      any // a JSON type, or a list of them
	enum     []any
	required []string
}

// readShape reads doc, a schema as jsonschema reads it, at url, which a relative $ref is read from, into
// its shape. It refuses a keyword floe reads that is of another JSON type than JSON Schema's.
func readShape(doc any, url string) (*shape, error) {
	return readShapeAt(doc, url, "#")
}

// errKeyword is a keyword floe reads, of another JSON type than JSON Schema's.
var errKeyword = errors.New("a keyword of the wrong JSON type")

// readShapeAt reads the schema v, at the JSON pointer ptr of the schema at url, into its shape.
func readShapeAt(v any, url, ptr string) (*shape, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s, which is an object", errKeyword, ptr)
	}
	ref, errRef := keyword[string](m, "$ref", ptr, "text")
	format, errFormat := keyword[string](m, "format", ptr, "text")
	about, errAbout := keyword[string](m, "description", ptr, "text")
	enum, errEnum := keyword[[]any](m, "enum", ptr, "a list")
	required, errRequired := keyword[[]any](m, "required", ptr, "a list")
	props, errProps := keyword[map[string]any](m, "properties", ptr, "an object")
	if err := errors.Join(errRef, errFormat, errAbout, errEnum, errRequired, errProps); err != nil {
		return nil, err
	}
	s := &shape{
		keys: nil, each: nil, ref: "", format: format, named: nil, part: nil, about: about, typ: m["type"], enum: enum,
		required: nil,
	}
	if ref != "" {
		s.ref = absolute(url, ref)
	}
	for _, r := range required {
		name, ok := r.(string)
		if !ok {
			return nil, fmt.Errorf("%w: %s/required, which is a list of text", errKeyword, ptr)
		}
		s.required = append(s.required, name)
	}
	if props != nil {
		s.keys = make(map[string]*shape, len(props))
		for _, k := range slices.Sorted(maps.Keys(props)) {
			var err error
			if s.keys[k], err = readShapeOrNull(props[k], url, ptr+"/properties/"+pointerKey.Replace(k)); err != nil {
				return nil, err
			}
		}
	}
	var err error
	if s.each, err = readShapeOrNull(m["items"], url, ptr+"/items"); err != nil {
		return nil, err
	}
	return s, nil
}

// readShapeOrNull reads the schema v as readShapeAt does, nil for none or null, which declares nothing.
func readShapeOrNull(v any, url, ptr string) (*shape, error) {
	if v == nil {
		var none *shape
		return none, nil
	}
	return readShapeAt(v, url, ptr)
}

// keyword returns the keyword kw of the schema m, at ptr, as a T, which want words: its zero value when m
// has none, or null.
func keyword[T any](m map[string]any, kw, ptr, want string) (T, error) {
	var t T
	v, ok := m[kw]
	if !ok || v == nil {
		return t, nil
	}
	if t, ok = v.(T); !ok {
		return t, fmt.Errorf("%w: %s/%s, which is %s", errKeyword, ptr, kw, want)
	}
	return t, nil
}

// bind binds the $refs of s, and those under it, to what they name among floe's in set.
// LIMIT: a $ref into the schema's own file (#/$defs/x) is not followed, and floe draws what it names as
// plain data. Follow it here, kept from a schema that names itself, if a template comes to use one.
func (s *shape) bind(set *partSet) {
	if s == nil {
		return
	}
	for _, k := range s.keys {
		k.bind(set)
	}
	s.each.bind(set)
	named, ok := set.shapes[s.ref]
	if !ok {
		return
	}
	name, _ := partName(s.ref)
	s.named, s.part = named, set.parts[name]
}

// inside is the shape of what s holds: that of what its $ref names, where s declares neither keys nor
// items of its own.
func (s *shape) inside() *shape {
	if s.named != nil && s.keys == nil && s.each == nil {
		return s.named.inside()
	}
	return s
}

// keyNames are the keys s declares, in order.
func (s *shape) keyNames() []string {
	return slices.Sorted(maps.Keys(s.inside().keys))
}

// description is the description of s: that written beside its $ref, before that of what it names.
func (s *shape) description() string {
	if s.named == nil {
		return s.about
	}
	return joinDescriptions(s.about, s.named.description())
}

// jsonType is the JSON type s names, or what its $ref names does where it names none.
func (s *shape) jsonType() any {
	if s.typ == nil && s.named != nil {
		return s.named.jsonType()
	}
	return s.typ
}

// values are the values s takes alone, or those of what its $ref names where it names none.
func (s *shape) values() []any {
	if s.enum == nil && s.named != nil {
		return s.named.values()
	}
	return s.enum
}
