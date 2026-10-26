package schema

import (
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/usererr"
)

// drafts are the drafts floe takes. Before 2019-09 the library leaves out every key beside a $ref.
var drafts = []string{"https://json-schema.org/draft/2020-12/schema", "https://json-schema.org/draft/2019-09/schema"}

// raw refuses in doc, the JSON of file held at u, each "$schema" that names no draft floe takes, in dialects,
// and each "$id" that would take a URL floe gives its parts and the schemas they share, in ids: one of the
// floe: scheme, and in a part or a shared schema (part), one without a scheme, which lands under
// floe:///parts/. The root may name u as it is. One "$schema" floe does not take leaves properties and the
// like out of the compiled tree, and the library would read it through the loader while it compiles; one
// "$id" names the schema a $ref in its file to that URL means, over floe's.
// LIMIT: a "$schema" or "$id" in a value (enum, const, default, examples, x-…) is refused too, as a JSON
// value is all it reads.
func raw(u, file string, doc any, part bool) (dialects, ids usererr.List) {
	var walk func(v any, at string)
	walk = func(v any, at string) {
		switch v := v.(type) {
		case map[string]any:
			if s, ok := v["$schema"].(string); ok && !slices.Contains(drafts, draft(s)) {
				dialects = append(dialects, usererr.NewIn(file, "#"+at, "schema_dialect", s))
			}
			if id, ok := v["$id"].(string); ok {
				ids = append(ids, idMistake(u, file, at, id, part)...)
			}
			for _, k := range slices.Sorted(maps.Keys(v)) {
				walk(v[k], at+"/"+escape(k))
			}
		case []any:
			for i, item := range v {
				walk(item, at+"/"+strconv.Itoa(i))
			}
		}
	}
	walk(doc, "")
	return dialects, ids
}

// idMistake returns the mistake of the "$id" id at the JSON pointer at of file, held at u, of a part or a
// shared schema when part. A floe: one, in any case (url.Parse lowercases it), is floe's; one without a
// scheme is under the URL of the file, which is floe:///parts/ for a part. A template's is under
// floe://templates/, which it cannot leave. One that is no URL the library tells as it compiles.
func idMistake(u, file, at, id string, part bool) usererr.List {
	p, err := url.Parse(id)
	var key string
	switch {
	case at == "" && id == u:
		return nil
	case err != nil:
		return nil //nolint:nilerr // the library tells an $id that is no URL as it compiles
	case p.Scheme == "floe":
		key = "schema_id_floe"
	case p.Scheme == "" && part:
		key = "schema_id_relative"
	default:
		return nil
	}
	if at == "" {
		return usererr.List{usererr.NewIn(file, "#", "schema_id_root", id, u)}
	}
	return usererr.List{usererr.NewIn(file, "#"+at, key, id)}
}

// draft returns u as drafts write it: without a trailing # and with https for http.
func draft(u string) string {
	u = strings.TrimSuffix(u, "#")
	if rest, ok := strings.CutPrefix(u, "http://"); ok {
		return "https://" + rest
	}
	return u
}

// at returns where s is written: its file, and its JSON pointer there.
func (c *Compiled) at(s *jsonschema.Schema) (file, pointer string) {
	base, frag := split(s.Location)
	return c.fileOf(base), "#" + unescape(frag)
}

// mistake returns the mistake key with args at s.
func (c *Compiled) mistake(s *jsonschema.Schema, key string, args ...any) *usererr.Error {
	file, at := c.at(s)
	return usererr.NewIn(file, at, key, args...)
}

// checkTemplate refuses what floe could not draw as the schema checks it: an item a part or a format draws
// that floe's walk does not reach, a $ref into a part, a $dynamicRef, a root drawn whole, and a key floe
// reads itself that the schema requires; and a $ref to a schema floe does not hold. Checks 1 to 3 are of the
// template's own file: a Shared schema is floe's, and what it does not reach is told at the $ref to it.
// LIMIT: one cause may be told twice or more, at the $ref a walk does not reach (check 2) and at the part or
// format past it (checks 1 and 3); both are true, at other places. Tell it once if that comes to mislead.
func (c *Compiled) checkTemplate() error {
	r := c.followFrom(c.Root)
	var found usererr.List
	own := func(s *jsonschema.Schema) bool { base, _ := split(s.Location); return base == c.url }
	enter := func(base string) bool { return base == c.url || c.isShared(base) }
	c.every(c.Root, enter, func(n *jsonschema.Schema) {
		if own(n) {
			found = append(found, c.checkNode(n, r)...)
		}
	}, func(from, to *jsonschema.Schema) { found = append(found, c.strayMistake(from, to)) })
	if st := c.Follow(c.Root); st.Part != "" || st.Format != "" {
		by := st.Part
		if by == "" {
			by = `"format": "` + st.Format + `"`
		}
		found = append(found, c.mistake(c.Root, "schema_root_drawn", by))
	}
	found = append(found, c.sharedKeys()...)
	return found.Sorted().Err()
}

// checkNode returns the mistakes of checks 1 to 5 of n, a schema of the template's own file, r what Follow
// reaches from the root.
func (c *Compiled) checkNode(n *jsonschema.Schema, r reached) usererr.List {
	var found usererr.List
	switch {
	case n.DynamicRef != nil:
		found = append(found, c.mistake(n, "schema_dynamic_ref", "$dynamicRef"))
	case n.RecursiveRef != nil:
		found = append(found, c.mistake(n, "schema_dynamic_ref", "$recursiveRef"))
	}
	if f := floeFormat(n); f != "" && !r.atFormat[n] {
		found = append(found, c.mistake(n, "schema_format_unreached", f))
	}
	if n.Ref == nil {
		return found
	}
	into := c.intoMistake(n)
	switch part, isRoot := c.partRoot(n.Ref); {
	case isRoot && !r.atPart[n]:
		found = append(found, c.mistake(n, "schema_part_unreached", partName(part)))
	case into != nil:
		found = append(found, into)
	case c.followed(n, n.Ref) && !r.passed[n] && c.followFrom(n.Ref).stops():
		found = append(found, c.mistake(n, "schema_ref_unreached", c.written(n.Ref.Location)))
	}
	return found
}

// sharedKeys refuses each key floe reads itself (doc.Shared) that a schema always applied to the root's value
// declares or requires: the root, and what its allOf and its $refs to the same file or to a Shared schema hold.
// floe leaves the keys out of the values it checks, so that a rule declared for one would never be checked and a
// key required would always be missing. A key declared and required is told at both, which are fixed apart.
// LIMIT: only properties are looked at: what if/then, anyOf, oneOf and not hold, dependentSchemas, dependencies
// of a schema, the keys of dependentRequired and of dependencies, and patternProperties are not, and a rule they
// give one of the keys is silently left unchecked.
// LIMIT: a $defs the root refers to is refused for declaring one of the keys even where an item under the root
// refers to it too, for which the declaration is right; give the root a $defs of its own if a template needs it.
func (c *Compiled) sharedKeys() usererr.List {
	var found usererr.List
	seen := map[*jsonschema.Schema]bool{}
	var root func(s *jsonschema.Schema)
	root = func(s *jsonschema.Schema) {
		if s == nil || seen[s] {
			return
		}
		seen[s] = true
		required := requiredOf(s)
		for _, key := range doc.Shared {
			if s.Properties[key] != nil {
				found = append(found, c.mistake(s.Properties[key], "schema_shared_declared", key))
			}
			if slices.Contains(required, key) {
				found = append(found, c.mistake(s, "schema_shared_key", key))
			}
		}
		for _, all := range s.AllOf {
			root(all)
		}
		if s.Ref != nil && c.followed(s, s.Ref) {
			root(s.Ref)
		}
	}
	root(c.Root)
	return found
}

// requiredOf returns the keys s requires: required, and those dependentRequired and dependencies require.
func requiredOf(s *jsonschema.Schema) []string {
	required := slices.Clone(s.Required)
	for _, more := range s.DependentRequired {
		required = append(required, more...)
	}
	for _, dep := range s.Dependencies {
		// The library reads dependencies in every draft.
		if more, ok := dep.([]string); ok {
			required = append(required, more...)
		}
	}
	return required
}

// checkPart refuses one of Formats in the part compiled, or in a Shared schema it refers to, where it is
// written.
func (c *Compiled) checkPart() error {
	var found usererr.List
	own := func(base string) bool { return base == c.url }
	c.every(c.Root, own, func(n *jsonschema.Schema) {
		if f := floeFormat(n); f != "" {
			found = append(found, c.mistake(n, "schema_part_format", f))
		}
		if n.Ref == nil {
			return
		}
		// Into another part or a Shared schema, which the walk from their roots does not reach.
		if into := c.intoMistake(n); into != nil {
			found = append(found, into)
			return
		}
		if base, _ := split(n.Ref.Location); c.isShared(base) {
			if f := c.formatIn(n.Ref); f != "" {
				found = append(found, c.mistake(n, "schema_part_shared_format", base, f))
			}
		}
	}, func(from, to *jsonschema.Schema) { found = append(found, c.strayMistake(from, to)) })
	return found.Sorted().Err()
}

// formatIn returns the first of Formats in the Shared schema s, through the Shared schemas it refers to, ""
// for none.
func (c *Compiled) formatIn(s *jsonschema.Schema) string {
	var formats []string
	c.every(s, c.isShared, func(n *jsonschema.Schema) {
		if f := floeFormat(n); f != "" {
			formats = append(formats, f)
		}
	}, func(_, _ *jsonschema.Schema) {})
	slices.Sort(formats)
	if len(formats) == 0 {
		return ""
	}
	return formats[0]
}

// partName returns the name of the part at u, floe:///parts/<name>.json.
func partName(u string) string {
	return strings.TrimSuffix(strings.TrimPrefix(u, "floe:///parts/"), ".json")
}

// written returns loc, a Location, as a $ref in the file compiled writes it: #/… for one in that file, the
// URL and its fragment for another, without an empty fragment.
func (c *Compiled) written(loc string) string {
	base, frag := split(loc)
	switch {
	case base == c.url:
		return "#" + unescape(frag)
	case frag == "":
		return base
	}
	return base + "#" + unescape(frag)
}

// intoMistake returns the mistake of the $ref of n into a part or a Shared schema floe holds, not at its
// root, which a walk from that root does not reach; nil for none.
func (c *Compiled) intoMistake(n *jsonschema.Schema) *usererr.Error {
	base, frag := split(n.Ref.Location)
	if _, held := c.res[base]; !held || base == c.url || frag == "" {
		return nil
	}
	return c.mistake(n, "schema_part_into", c.written(n.Ref.Location), base)
}

// strayMistake is the mistake of from, which names to, a schema in no file floe holds (a metaschema the
// library holds): a schema refers to no file or URL outside what floe holds.
func (c *Compiled) strayMistake(from, to *jsonschema.Schema) *usererr.Error {
	base, _ := split(to.Location)
	return c.mistake(from, "schema_no_url", base)
}

// isShared reports whether base is the URL of a Shared schema.
func (c *Compiled) isShared(base string) bool {
	r, ok := c.res[base]
	return ok && base != c.url && r.Kind == Shared
}
