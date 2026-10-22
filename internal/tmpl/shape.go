package tmpl

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
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
// its shape, and notes what floe refuses of it wherever it is, beyond properties and items, which floe
// follows: file names the schema in its mistakes, and part the part whose schema it is, "" for a template's
// or one the parts share. It refuses $ref, format, description, enum, required, properties and items in
// another form than floe reads where floe follows them, the first it meets; type, which floe only tells, is
// kept as written.
func readShape(doc any, url, file, part string) (*shape, *schemaNotes, error) {
	n := &schemaNotes{
		file: file, part: part, own: map[string]bool{url: true}, met: nil, warnings: nil,
	}
	s, err := n.read(doc, url, "#")
	if err != nil {
		return nil, nil, err
	}
	return s, n, nil
}

// schemaNotes is what floe refuses of a schema as written, and the $refs it checks once the parts are read.
type schemaNotes struct {
	file string // the schema, as its mistakes name it
	part string // the part whose schema it is; "" for a template's or one the parts share
	// the URLs of the schema's own file, its own and those its $ids give, which its $refs may point into
	own      map[string]bool
	met      []noted          // in the order of the schema
	warnings []*usererr.Error // told by floe template check alone
}

// noted is a mistake met in a schema, or a $ref of it, which floe checks once the parts are read.
type noted struct {
	mistake *usererr.Error
	ref     *refAt
}

// refAt is a $ref of a schema, absolute, at the JSON pointer ptr, reached when floe follows properties and
// items to it.
type refAt struct {
	ptr, ref string
	reached  bool
}

// refuse notes the mistake m.
func (n *schemaNotes) refuse(m *usererr.Error) {
	n.met = append(n.met, noted{mistake: m, ref: nil})
}

// refused returns the mistakes noted, which the schema is refused for as it is read.
func (n *schemaNotes) refused() usererr.List {
	var found usererr.List
	for _, x := range n.met {
		if x.mistake != nil {
			found = append(found, x.mistake)
		}
	}
	return found
}

// read reads the schema v at ptr, which floe follows properties and items to, into its shape, base the
// URL a relative $ref in it is read from.
func (n *schemaNotes) read(v any, base, ptr string) (*shape, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, usererr.NewIn(n.file, ptr, "tmpl_schema_unread", usererr.New("tmpl_type_object"))
	}
	base = n.idOf(m, base)
	ref, errRef := keyword[string](n.file, m, "$ref", ptr, "tmpl_type_string")
	format, errFormat := keyword[string](n.file, m, "format", ptr, "tmpl_type_string")
	about, errAbout := keyword[string](n.file, m, "description", ptr, "tmpl_type_string")
	enum, errEnum := keyword[[]any](n.file, m, "enum", ptr, "tmpl_type_array")
	required, errRequired := keyword[[]any](n.file, m, "required", ptr, "tmpl_type_array")
	props, errProps := keyword[map[string]any](n.file, m, "properties", ptr, "tmpl_type_object")
	if err := cmp.Or(errRef, errFormat, errAbout, errEnum, errRequired, errProps); err != nil {
		return nil, err //nolint:wrapcheck // the first of keyword's mistakes, made here
	}
	s := &shape{
		keys: nil, each: nil, ref: "", format: format, named: nil, part: nil, about: about, typ: m["type"], enum: enum,
		required: nil,
	}
	if ref != "" {
		s.ref = absolute(base, ref)
	}
	for _, r := range required {
		name, ok := r.(string)
		if !ok {
			return nil, usererr.NewIn(n.file, ptr+"/required", "tmpl_schema_unread", usererr.New("tmpl_type_string_list"))
		}
		s.required = append(s.required, name)
	}
	// What a $ref names, or a format of floe's, declares the item: floe follows nothing beside it.
	elsewhere := n.note(m, base, ptr, true)
	for _, kw := range slices.Sorted(maps.Keys(m)) {
		at := ptr + "/" + pointerKey.Replace(kw)
		switch {
		case kw == "properties" && !elsewhere && props != nil:
			var err error
			if s.keys, err = n.readKeys(props, base, at); err != nil {
				return nil, err
			}
		case kw == "items" && !elsewhere && m[kw] != nil:
			var err error
			if s.each, err = n.read(m[kw], base, at); err != nil {
				return nil, err
			}
		default:
			n.under(kw, m[kw], base, at)
		}
	}
	return s, nil
}

// readKeys reads the schemas of props, the properties at ptr that floe follows, into their shapes, by key.
func (n *schemaNotes) readKeys(props map[string]any, base, ptr string) (map[string]*shape, error) {
	keys := make(map[string]*shape, len(props))
	for _, k := range slices.Sorted(maps.Keys(props)) {
		// null declares the key, and nothing of it.
		keys[k] = nil
		if props[k] != nil {
			var err error
			if keys[k], err = n.read(props[k], base, ptr+"/"+pointerKey.Replace(k)); err != nil {
				return nil, err
			}
		}
	}
	return keys, nil
}

// walk notes what floe refuses of the schema v at ptr, which floe does not follow to, and of those under it.
func (n *schemaNotes) walk(v any, base, ptr string) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	base = n.idOf(m, base)
	n.note(m, base, ptr, false)
	for _, kw := range slices.Sorted(maps.Keys(m)) {
		n.under(kw, m[kw], base, ptr+"/"+pointerKey.Replace(kw))
	}
}

// under walks the schemas the keyword kw of a schema holds in v, at ptr, which floe does not follow to.
func (n *schemaNotes) under(kw string, v any, base, ptr string) {
	switch sub := v.(type) {
	case map[string]any:
		switch kw {
		case "properties", "$defs", "definitions", "patternProperties", "dependentSchemas", "dependencies":
			for _, k := range slices.Sorted(maps.Keys(sub)) {
				n.walk(sub[k], base, ptr+"/"+pointerKey.Replace(k))
			}
		case "items", "additionalProperties", "unevaluatedProperties", "propertyNames", "additionalItems",
			"unevaluatedItems", "contains", "not", "if", "then", "else", "contentSchema":
			n.walk(sub, base, ptr)
		}
	case []any:
		switch kw {
		case "allOf", "anyOf", "oneOf", "prefixItems", "items":
			for i, s := range sub {
				n.walk(s, base, ptr+"/"+strconv.Itoa(i))
			}
		}
	}
}

// idOf returns the URL a relative $ref of the schema m is read from, base or that its $id gives, which is
// the schema's own file too.
// LIMIT: draft-4's id is not read, so a $ref to what it names is refused as out of floe; read it if one comes.
func (n *schemaNotes) idOf(m map[string]any, base string) string {
	id, ok := m["$id"].(string)
	if !ok {
		return base
	}
	base, _, _ = strings.Cut(absolute(base, id), "#")
	n.own[base] = true
	return base
}

// note notes the $ref and the format of the schema m at ptr, reached when floe follows properties and
// items to it, and reports whether either declares the item, so that floe follows nothing beside it.
func (n *schemaNotes) note(m map[string]any, base, ptr string, reached bool) bool {
	elsewhere := false
	if ref, ok := m["$ref"].(string); ok {
		ref = absolute(base, ref)
		n.met = append(n.met, noted{mistake: nil, ref: &refAt{ptr: ptr, ref: ref, reached: reached}})
		if _, floe := partName(ref); floe {
			// A format beside it is refused there, once.
			n.beside(m, ptr, ref)
			return true
		}
	}
	if f, ok := m["format"].(string); ok && n.format(f, m["type"], ptr, reached) {
		elsewhere = true
	}
	closed := m["additionalProperties"] == false || m["unevaluatedProperties"] == false
	if _, ok := m["properties"]; ok && reached && !elsewhere && !closed && n.part == "" {
		n.warnings = append(n.warnings, usererr.NewIn(n.file, ptr, "tmpl_schema_open"))
	}
	return elsewhere
}

// besideRef are the keywords a schema may write beside a $ref to a part or a schema the parts share, which
// declare nothing, JSON Schema's annotations: what the $ref names declares the item, and the description is
// told before its own.
var besideRef = []string{
	"$comment", "default", "deprecated", "description", "examples", "readOnly", "title", "writeOnly",
}

// beside refuses the keywords of the schema m at ptr, beside its $ref to ref, a part or a schema the parts
// share, that declare something of the item, which floe would not draw as JSON Schema reads them. The root
// names its draft beside it too.
func (n *schemaNotes) beside(m map[string]any, ptr, ref string) {
	var more []string
	for _, kw := range slices.Sorted(maps.Keys(m)) {
		if kw != "$ref" && !slices.Contains(besideRef, kw) && (kw != "$schema" || ptr != "#") {
			more = append(more, kw)
		}
	}
	if more != nil {
		n.refuse(usererr.NewIn(n.file, ptr, "tmpl_ref_beside", ref, strings.Join(more, ", "), strings.Join(besideRef, ", ")))
	}
}

// format notes the format f of the schema at ptr, of the type typ, reached when floe follows properties
// and items to it, and reports whether floe draws the item whole.
func (n *schemaNotes) format(f string, typ any, ptr string, reached bool) bool {
	switch {
	case isGoPart(f):
		n.refuse(usererr.NewIn(n.file, ptr, "tmpl_format_part", f, f))
		return true
	case !knownFormat(f):
		n.refuse(usererr.NewIn(n.file, ptr, "tmpl_format_unknown", f))
	case slices.Contains(formats, f) && n.part != "":
		// Wherever it is: under properties and items, it would be refused all the same.
		n.refuse(usererr.NewIn(n.file, ptr, "tmpl_part_format", f))
		return true
	case slices.Contains(formats, f) && !reached:
		n.refuse(usererr.NewIn(n.file, ptr, "tmpl_format_unreached", f))
	case slices.Contains(formats, f):
		if typ != "string" {
			n.refuse(usererr.NewIn(n.file, ptr, "tmpl_format_type", f, "string"))
		}
		return true
	}
	return false
}

// mistakes returns what set refuses of the schema of n: what n noted, then each $ref that points out of
// floe, at a part it does not have, cannot use or that draws nothing, into a part or a schema the parts
// share that holds what floe draws, at one where floe does not follow to it, or, from a part's schema, at
// a schema the parts share that holds a format of floe's. A $ref into the schema's own file is followed
// by jsonschema alone.
// LIMIT: $dynamicRef and $recursiveRef go unchecked, refused at compiling without a pointer; check them here if used.
func (set *partSet) mistakes(n *schemaNotes) usererr.List {
	var mistakes usererr.List
	for _, x := range n.met {
		m := x.mistake
		if x.ref != nil {
			m = set.refMistake(n, *x.ref)
		}
		if m != nil {
			mistakes = append(mistakes, m)
		}
	}
	return mistakes
}

// refMistake returns what set refuses of r, a $ref of the schema of n, or nil.
func (set *partSet) refMistake(n *schemaNotes, r refAt) *usererr.Error {
	base, _, _ := strings.Cut(r.ref, "#")
	switch {
	case n.own[base]:
		return nil
	case strings.HasPrefix(base, partsURL):
		return set.partMistake(n, r, base)
	case !heldByJSONSchema(base):
		return n.at(r, usererr.New("tmpl_schema_url", base))
	}
	return nil
}

// at is the mistake e of r, a $ref of the schema of n, at the schema and its pointer.
func (n *schemaNotes) at(r refAt, e *usererr.Error) *usererr.Error {
	return e.Placed(n.file, 0, 0, r.ptr)
}

// partMistake returns what set refuses of r, a $ref of the schema of n to base under floe:///parts/, or nil.
func (set *partSet) partMistake(n *schemaNotes, r refAt, base string) *usererr.Error {
	name, whole := partName(r.ref)
	switch p := set.parts[name]; {
	case whole && p != nil && !p.isLook() && !r.reached:
		return usererr.NewIn(n.file, r.ptr, "tmpl_part_unreached", name)
	case whole && p != nil && !p.isLook():
		return nil
	case whole && set.shares[r.ref]:
		return set.sharedMistake(n, r)
	case !whole && set.docs[base] != nil:
		if _, into := set.pointsInto(r.ref); !into {
			return nil // into what holds nothing floe draws, as tone.json's enum: drawn as written
		}
	}
	return n.at(r, unknownPart(set, r.ref))
}

// sharedMistake returns what set refuses of r, a $ref of the schema of n to a schema the parts share, or nil:
// one that holds what floe draws where floe does not follow to it, or, from a part's schema, one that holds
// a format of floe's, which the checks of a format do not reach in a part.
func (set *partSet) sharedMistake(n *schemaNotes, r refAt) *usererr.Error {
	shared := set.shapes[r.ref]
	switch {
	case !r.reached && set.draws(shared):
		return usererr.NewIn(n.file, r.ptr, "tmpl_shared_unreached", r.ref)
	case r.reached && n.part != "":
		if under, item := set.formatIn(shared, r.ref+"#"); item != nil {
			return usererr.NewIn(n.file, r.ptr+" → "+under, "tmpl_part_shared_format", item.format, r.ref, r.ref)
		}
	}
	return nil
}

// keyword returns the keyword kw of the schema m in file, at ptr, as a T, which the type word want words:
// its zero value when m has none, or null. A boolean schema, or a list of them, is JSON Schema all the
// same, but floe reads only the form it draws.
func keyword[T any](file string, m map[string]any, kw, ptr, want string) (T, error) {
	var t T
	v, ok := m[kw]
	if !ok || v == nil {
		return t, nil
	}
	if t, ok = v.(T); !ok {
		return t, usererr.NewIn(file, ptr+"/"+kw, "tmpl_schema_unread", usererr.New(want))
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

// drawnItem is an item of a data file floe draws whole: by the part part, or as the format of floe's
// format, one of the two.
type drawnItem struct {
	itemAt

	part   *part
	format string
}

// drawnItems are the items of a data file floe draws whole, the parts a part holds before it, which every
// check of them picks its own from.
type drawnItems []drawnItem

// drawnWhole returns the items v holds that floe draws whole (walkWhole).
func (s *shape) drawnWhole(v any) drawnItems {
	var found drawnItems
	s.walkWhole(v, nil, false, &found)
	return found
}

// walkWhole adds to found the items v, at the path at, holds that floe draws whole: the parts a part holds
// before it, which draws them as they were drawn. What an item of a format of floe's holds is not looked
// into, and a part wins over a format, as in drawn. Under a part, inPart, an item of a format of floe's is
// the part's to draw, by its part.html, and none of floe's to check, as none is meant to be there
// (partSet.mistakes).
func (s *shape) walkWhole(v any, at []string, inPart bool, found *drawnItems) {
	if s == nil {
		return
	}
	if s.part == nil && slices.Contains(formats, s.format) {
		if !inPart {
			*found = append(*found, drawnItem{itemAt: itemAt{value: v, at: at}, part: nil, format: s.format})
		}
		return
	}
	in, held := s.inside(), inPart || s.part != nil
	switch v := v.(type) {
	case map[string]any:
		for k, item := range v {
			in.keys[k].walkWhole(item, append(at[:len(at):len(at)], k), held, found)
		}
	case []any:
		for i, item := range v {
			in.each.walkWhole(item, append(at[:len(at):len(at)], strconv.Itoa(i)), held, found)
		}
	}
	if s.part != nil {
		*found = append(*found, drawnItem{itemAt: itemAt{value: v, at: at}, part: s.part, format: ""})
	}
}

// ofFormat returns the items of w of the format of floe's format.
func (w drawnItems) ofFormat(format string) []itemAt {
	return w.where(func(f drawnItem) bool { return f.part == nil && f.format == format })
}

// ofPart returns the items of w the part name draws, those it holds before it.
func (w drawnItems) ofPart(name string) []itemAt {
	return w.where(func(f drawnItem) bool { return f.part != nil && f.part.name == name })
}

// where returns the items of w that keep keeps, in order.
func (w drawnItems) where(keep func(drawnItem) bool) []itemAt {
	var items []itemAt
	for _, f := range w {
		if keep(f) {
			items = append(items, f.itemAt)
		}
	}
	return items
}
