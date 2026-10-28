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

// raw refuses in doc, the JSON of file, each "$schema" that names no draft floe takes, in dialects, and in ids
// an "$id" at the root, whatever its value, as floe gives the root its URL; and each other "$id" that could
// take a URL of floe's: one of the floe: scheme, and in a part or a shared schema (part), one without a
// scheme, which may land under floe:///parts/. One "$schema" floe does not take leaves properties and the like
// out of the compiled tree, and the library would read it through the loader while it compiles; one "$id"
// names the schema a $ref in its file to that URL means, over floe's.
// LIMIT: a "$schema" or "$id" in a value (enum, x-…) is refused too; read the compiled tree if that misleads.
func raw(file string, doc any, part bool) (dialects, ids usererr.List) {
	if root, ok := doc.(map[string]any); ok {
		if _, ok := root["$id"]; ok {
			ids = append(ids, usererr.NewIn(file, "#", "schema_id_root"))
		}
	}
	var walk func(v any, at string)
	walk = func(v any, at string) {
		switch v := v.(type) {
		case map[string]any:
			if s, ok := v["$schema"].(string); ok && !slices.Contains(drafts, draft(s)) {
				dialects = append(dialects, usererr.NewIn(file, "#"+at, "schema_dialect", s))
			}
			if id, ok := v["$id"].(string); ok && at != "" {
				ids = append(ids, idMistake(file, at, id, part)...)
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

// idMistake returns the mistake of the "$id" id at the JSON pointer at of file, below its root, of a part or a
// shared schema when part. A floe: one, in any case (url.Parse lowercases it), is floe's; one without a scheme
// is under the URL of the file, which is floe:///parts/ for a part. A template's is under floe://templates/,
// which it cannot leave. One that is no URL the library tells as it compiles.
func idMistake(file, at, id string, part bool) usererr.List {
	p, err := url.Parse(id)
	var key string
	switch {
	// The library reads "" and "#" as no $id, and tells another fragment alone itself.
	case id == "", strings.HasPrefix(id, "#"):
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

// checkTemplate refuses what floe could not draw as the schema checks it: what checkOwn refuses, a root drawn
// whole, and a key floe reads itself that the schema requires.
func (c *Compiled) checkTemplate() error {
	found := c.checkOwn()
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

// checkOwn returns the mistakes checkNode tells of each schema of the file compiled, and of each $ref, through
// the Shared schemas it refers to too, to a schema floe does not hold.
// LIMIT: checkNode does not run in a Shared schema, whose items a walk reaches as the file referring to it
// places them; a $dynamicRef or an item it does not reach there is let through. Walk them from where they are
// referred to, and tell them in the Shared schema's file, if one of one's own comes to need it.
// LIMIT: one cause may be told twice or more, at the $ref a walk does not reach and at the part or format past
// it; both are true, at other places. Tell it once if that comes to mislead.
func (c *Compiled) checkOwn() usererr.List {
	r := c.followFrom(c.Root)
	var found usererr.List
	enter := func(base string) bool { return base == c.url || c.isShared(base) }
	c.every(c.Root, enter, func(n *jsonschema.Schema) {
		if base, _ := split(n.Location); base == c.url {
			found = append(found, c.checkNode(n, r)...)
		}
	}, func(from, to *jsonschema.Schema) { found = append(found, c.strayMistake(from, to)) })
	return found
}

// checkNode returns the mistakes of n, a schema of the file compiled, r what Follow reaches from the root: a
// $dynamicRef; an item a part or a format draws that r does not reach; a $ref into a part. A part takes, of
// Formats, image alone, a trust boundary, as the checks of the data's HTML reach the template's items alone;
// and refers to no Shared schema holding one of Formats, image too, of which no walk from the part's root is
// checked. Either is told in place of the check of where n is, as a part holds it nowhere; r of a part goes
// through such a format, so that what is past it is told only as it would be with the format taken out.
func (c *Compiled) checkNode(n *jsonschema.Schema, r reached) usererr.List {
	var found usererr.List
	switch {
	case n.DynamicRef != nil:
		found = append(found, c.mistake(n, "schema_dynamic_ref", "$dynamicRef"))
	case n.RecursiveRef != nil:
		found = append(found, c.mistake(n, "schema_dynamic_ref", "$recursiveRef"))
	}
	if f := floeFormat(n); c.part && f != "" && f != "image" {
		found = append(found, c.mistake(n, "schema_part_format", f))
	} else if unreached := c.unreachedFormat(n, r); unreached != nil {
		found = append(found, unreached)
	}
	if n.Ref != nil {
		if refused := c.refMistake(n, r); refused != nil {
			found = append(found, refused)
		}
	}
	return found
}

// refMistake returns checkNode's mistake of the $ref of n, nil for none.
func (c *Compiled) refMistake(n *jsonschema.Schema, r reached) *usererr.Error {
	if name, isRoot := c.partRoot(n.Ref); isRoot {
		if !r.atPart[n] {
			return c.mistake(n, "schema_part_unreached", partName(name))
		}
		return nil
	}
	if into := c.intoMistake(n); into != nil {
		return into
	}
	if c.part {
		if shared := c.sharedFormat(n); shared != nil {
			return shared
		}
	}
	return c.unreachedRef(n, r)
}

// sharedFormat returns the mistake of the $ref of n to a Shared schema that holds one of Formats, in a part;
// nil for none.
func (c *Compiled) sharedFormat(n *jsonschema.Schema) *usererr.Error {
	if base, _ := split(n.Ref.Location); c.isShared(base) {
		if f := c.formatIn(n.Ref); f != "" {
			return c.mistake(n, "schema_part_shared_format", base, f)
		}
	}
	return nil
}

// unreachedRef refuses n, a $ref Follow goes through, if r, what Follow reaches from the root, does not pass n
// while what n refers to holds parts or formats: items floe would not draw; nil else.
func (c *Compiled) unreachedRef(n *jsonschema.Schema, r reached) *usererr.Error {
	if c.followed(n, n.Ref) && !r.passed[n] && c.followFrom(n.Ref).stops() {
		return c.mistake(n, "schema_ref_unreached", c.written(n.Ref.Location))
	}
	return nil
}

// unreachedFormat refuses n if it is of one of Formats that r, what Follow reaches from the root, does not
// reach: an item floe would not draw, nil else.
func (c *Compiled) unreachedFormat(n *jsonschema.Schema, r reached) *usererr.Error {
	if f := floeFormat(n); f != "" && !r.atFormat[n] {
		return c.mistake(n, "schema_format_unreached", f)
	}
	return nil
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

// CheckPart refuses, in the part compiled (CompilePart), where it is written, what checkOwn refuses of a part.
func (c *Compiled) CheckPart() error {
	return c.checkOwn().Sorted().Err()
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
	return c.held(base, Shared)
}

// held reports whether base is the URL of a resource of kind k, not the file compiled.
func (c *Compiled) held(base string, k Kind) bool {
	r, ok := c.res[base]
	return ok && base != c.url && r.Kind == k
}
