package schema

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/usererr"
)

// drafts are the drafts floe takes. Before 2019-09 the library leaves out every key beside a $ref.
var drafts = []string{"https://json-schema.org/draft/2020-12/schema", "https://json-schema.org/draft/2019-09/schema"}

// shared are the keys of a data file floe reads itself and leaves out of what the schema checks.
var shared = []string{"lang", "template", "theme", "title"}

// dialects refuses each "$schema" of doc, the JSON compiled, that names no draft floe takes: one that leaves
// out vocabularies leaves properties and the like out of the compiled tree, and the library would read an
// unknown one through the loader while it compiles.
// LIMIT: a "$schema" in a value (enum, const, default, examples) is refused too, as a JSON value is all it reads.
func (c *Compiled) dialects(doc any) error {
	var found usererr.List
	var walk func(v any, at string)
	walk = func(v any, at string) {
		switch v := v.(type) {
		case map[string]any:
			if s, ok := v["$schema"].(string); ok && !slices.Contains(drafts, draft(s)) {
				found = append(found, usererr.NewIn(c.file, "#"+at, "schema_dialect", s))
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
	return found.Err()
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
	}, func(from, to *jsonschema.Schema) {
		base, _ := split(to.Location)
		found = append(found, c.mistake(from, "schema_no_url", base))
	})
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
	base, frag := split(n.Ref.Location)
	_, held := c.res[base]
	switch part, isRoot := c.partRoot(n.Ref); {
	case isRoot && !r.atPart[n]:
		found = append(found, c.mistake(n, "schema_part_unreached", partName(part)))
	case held && base != c.url && frag != "":
		found = append(found, c.mistake(n, "schema_part_into", c.written(n.Ref.Location), base))
	case c.followed(n, n.Ref) && !r.passed[n] && c.followFrom(n.Ref).stops():
		found = append(found, c.mistake(n, "schema_ref_unreached", c.written(n.Ref.Location)))
	}
	return found
}

// sharedKeys refuses each key floe reads itself that a schema always applied to the root's value requires:
// the root, and what its allOf and its $refs to the same file or to a Shared schema hold. floe leaves the key
// out of the values it checks, so that the schema would always say it is missing. Declaring one is harmless.
func (c *Compiled) sharedKeys() usererr.List {
	var found usererr.List
	seen := map[*jsonschema.Schema]bool{}
	var root func(s *jsonschema.Schema)
	root = func(s *jsonschema.Schema) {
		if s == nil || seen[s] {
			return
		}
		seen[s] = true
		required := slices.Clone(s.Required)
		for _, more := range s.DependentRequired {
			required = append(required, more...)
		}
		for _, key := range shared {
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

// checkPart refuses one of Formats in the part compiled, or in a Shared schema it refers to, where it is
// written.
func (c *Compiled) checkPart() error {
	var found usererr.List
	enter := func(base string) bool { return base == c.url || c.isShared(base) }
	c.every(c.Root, enter, func(n *jsonschema.Schema) {
		if f := floeFormat(n); f != "" {
			found = append(found, c.mistake(n, "schema_part_format", f))
		}
	}, func(from, to *jsonschema.Schema) {
		base, _ := split(to.Location)
		found = append(found, c.mistake(from, "schema_no_url", base))
	})
	return found.Sorted().Err()
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

// isShared reports whether base is the URL of a Shared schema.
func (c *Compiled) isShared(base string) bool {
	r, ok := c.res[base]
	return ok && base != c.url && r.Kind == Shared
}
