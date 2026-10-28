package schema_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

// N is a $ref to the part note.
const N = `{"$ref": "floe:///parts/note.json"}`

// mark is a UTF-8 byte order mark.
const mark = "\xef\xbb\xbf"

// Compile tells the library's mistakes in its words where they are, and floe's checks of what it draws by
// at the schema they are of; it passes what floe draws as the schema checks it. The output is compared as it
// comes, not sorted.
func TestCompile(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, src, want string }{
		{"unreached", `{` + head + `, "properties": {"a": {"type":"string"}}, "additionalProperties": ` + N + `}`, "schema.json: #/additionalProperties: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"allof", `{` + head + `, "allOf": [{"properties": {"n": ` + N + `}}]}`, "schema.json: #/allOf/0/properties/n: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"fmtunreached", `{` + head + `, "patternProperties": {"^x": {"type":"string","format":"html"}}}`, "schema.json: #/patternProperties/^x: the format html marks an item to draw; declare it under properties or items, which floe follows"},
		{"into", `{` + head + `, "properties": {"t": {"$ref": "floe:///parts/note.json#/properties/title"}}}`, "schema.json: #/properties/t: floe:///parts/note.json#/properties/title points into a part of floe's or a schema the parts share, which would be checked but not drawn as written; refer to the whole of it, floe:///parts/note.json"},
		{"dyn", `{` + head + `, "properties": {"t": {"$dynamicRef": "#x"}}, "$defs": {"x": {"$dynamicAnchor": "x"}}}`, "schema.json: #/properties/t: floe does not follow $dynamicRef: write a $ref"},
		{"dynnoanchor", `{` + head + `, "properties": {"t": {"$dynamicRef": "#x"}}}`, "schema.json: anchor in \"floe://templates/t/schema.json#x\" not found in schema \"floe://templates/t/schema.json\""},
		{"rootpart", `{` + head + `, "$ref": "floe:///parts/note.json"}`, "schema.json: #: the root of the schema is drawn whole by floe:///parts/note.json; declare the items of the page under properties, and name a part under one of them"},
		{"rootdefs", `{` + head + `, "$ref": "#/$defs/a", "$defs": {"a": ` + N + `}}`, "schema.json: #: the root of the schema is drawn whole by floe:///parts/note.json; declare the items of the page under properties, and name a part under one of them"},
		{"rootfmt", `{` + head + `, "format": "markdown"}`, "schema.json: #: the root of the schema is drawn whole by \"format\": \"markdown\"; declare the items of the page under properties, and name a part under one of them"},
		{"reqtitle", `{` + head + `, "required": ["title"]}`, "schema.json: #: the schema requires title, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items"},
		{"depreq", `{` + head + `, "dependentRequired": {"x": ["title"]}}`, "schema.json: #: the schema requires title, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items"},
		{"anyofreq", `{` + head + `, "anyOf": [{"required": ["title"]}, {"required": ["name"]}]}`, ""},
		{"proptitle", `{` + head + `, "properties": {"title": {"type": "string"}}, "patternProperties": {".*": {}}}`, "schema.json: #/properties/title: title is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		{"declared and required", `{` + head + `, "properties": {"title": {"type": "string"}}, "required": ["title"]}`, "schema.json: #: the schema requires title, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items\nschema.json: #/properties/title: title is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		{"declared at the root, required in allOf", `{` + head + `, "properties": {"title": {}}, "allOf": [{"required": ["title"]}]}`, "schema.json: #/allOf/0: the schema requires title, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items\nschema.json: #/properties/title: title is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		{"required at the root, declared at a $ref", `{` + head + `, "$ref": "#/$defs/d", "$defs": {"d": {"properties": {"theme": {}}}}, "required": ["theme"]}`, "schema.json: #: the schema requires theme, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items\nschema.json: #/$defs/d/properties/theme: theme is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		// LIMIT: dependentSchemas, dependencies of a schema, the keys of dependentRequired and of dependencies, and
		// patternProperties are not looked at.
		{"a key of dependentRequired", `{` + head + `, "dependentRequired": {"title": ["a"]}}`, ""},
		{"a key of dependencies", `{` + head + `, "dependencies": {"title": ["a"]}}`, ""},
		{"dependencies of a schema", `{` + head + `, "dependencies": {"x": {"properties": {"title": {"maxLength": 3}}}}}`, ""},
		{"patternProperties of a shared key", `{` + head + `, "patternProperties": {"^ti": {"maxLength": 3}}}`, ""},
		{"declared loosely", `{` + head + `, "properties": {"title": {}}}`, "schema.json: #/properties/title: title is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		{"declared in allOf", `{` + head + `, "allOf": [{"properties": {"lang": {}}}]}`, "schema.json: #/allOf/0/properties/lang: lang is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		{"declared at a $ref and the root", `{` + head + `, "$ref": "#/$defs/d", "$defs": {"d": {"properties": {"theme": {}}}}, "properties": {"title": {}}}`, "schema.json: #/$defs/d/properties/theme: theme is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml\nschema.json: #/properties/title: title is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		// LIMIT: a $defs the root and an item both refer to is refused for the item too.
		{"a $defs of the root and an item", `{` + head + `, "$ref": "#/$defs/d", "properties": {"sub": {"$ref": "#/$defs/d"}}, "$defs": {"d": {"properties": {"title": {"type": "string"}}}}}`, "schema.json: #/$defs/d/properties/title: title is read by floe itself: remove it from the schema's properties; to add a rule for it, write a check in checks.yaml"},
		// LIMIT: if/then and the like are not looked at.
		{"declared under then", `{` + head + `, "if": {"required": ["a"]}, "then": {"properties": {"title": {"maxLength": 3}}}}`, ""},
		{"declared under anyOf", `{` + head + `, "anyOf": [{"properties": {"title": {"maxLength": 3}}}]}`, ""},
		{"unknownpart", `{` + head + `, "properties": {"t": {"$ref": "floe:///parts/nope.json"}}}`, "schema.json: floe:///parts/nope.json is not a part; refer to one as floe:///parts/<name>.json"},
		{"outside", `{` + head + `, "properties": {"t": {"$ref": "https://example.com/x.json"}}}`, "schema.json: https://example.com/x.json: a schema refers to no other file or URL; write what a $ref refers to in its own file"},
		{"dialect", `{"$schema": "https://example.com/s", "type": "object"}`, "schema.json: #: $schema https://example.com/s names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out"},
		{"metacore", `{"$schema": "https://json-schema.org/draft/2020-12/meta/core", "type": "object"}`, "schema.json: #: $schema https://json-schema.org/draft/2020-12/meta/core names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out"},
		{"draft07", `{"$schema": "http://json-schema.org/draft-07/schema#", "type": "object"}`, "schema.json: #: $schema http://json-schema.org/draft-07/schema# names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out"},
		{"hash", `{"$schema": "https://json-schema.org/draft/2020-12/schema#", "type": "object"}`, ""},
		{"nested", `{` + head + `, "properties": {"x": {"$id": "https://e.com/x.json", "$schema": "https://example.com/s"}}}`, "schema.json: #/properties/x: $schema https://example.com/s names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out"},
		{"badjson", `{` + head + `, `, "schema.json:1:79: unexpected EOF"}, // at the end, past the space
		{"strng", `{` + head + `, "properties": {"t": {"type":"strng"}}}`, "schema.json: #/properties/t/type: 'anyOf' failed"},
		{"xtype", `{` + head + `, "x": {"type": 3}, "properties": {"s": {"$ref": "#/x"}}}`, "schema.json: #/x/type: 'anyOf' failed"},
		{"pctmeta", `{` + head + `, "properties": {"項目": {"type": 3}, "a/b~c": {"minLength": "x"}}}`, "schema.json: #/properties/a~1b~0c/minLength: got string, want integer\nschema.json: #/properties/項目/type: 'anyOf' failed"},
		{"pct", `{` + head + `, "allOf": [{"properties": {"項目": ` + N + `}}], "patternProperties": {"a/b~c": {"type":"string","format": "html"}}}`, "schema.json: #/allOf/0/properties/項目: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/patternProperties/a~1b~0c: the format html marks an item to draw; declare it under properties or items, which floe follows"},
		{"twopaths", `{` + head + `, "properties": {"a": {"$ref": "#/$defs/p"}}, "allOf": [{"properties": {"b": {"$ref": "#/$defs/p"}}}], "$defs": {"p": ` + N + `}}`, "schema.json: #/allOf/0/properties/b: #/$defs/p holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref"},
		{"defsallof", `{` + head + `, "$defs": {"p": ` + N + `}, "allOf": [{"properties": {"b": {"$ref": "#/$defs/p"}}}]}`, "schema.json: #/$defs/p: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/allOf/0/properties/b: #/$defs/p holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref"},
		{"chain", `{` + head + `, "allOf": [{"$ref": "#/$defs/q"}], "$defs": {"q": {"properties": {"b": {"$ref": "#/$defs/p"}}}, "p": ` + N + `}}`, "schema.json: #/$defs/p: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/$defs/q/properties/b: #/$defs/p holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref\nschema.json: #/allOf/0: #/$defs/q holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref"},
		{"allofany", `{` + head + `, "allOf": [{"$ref": "floe:///parts/any.json"}]}`, "schema.json: #/allOf/0: floe:///parts/any.json holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref"},
		{"leak", `{` + head + `, "properties": {"a": {"$ref": "#/$defs/w"}}, "$defs": {"w": {"allOf": [{"properties": {"x": ` + N + `}}]}}}`, "schema.json: #/$defs/w/allOf/0/properties/x: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"aw", `{` + head + `, "properties": {"a": {"$ref": "#/$defs/w"}}, "allOf": [{"$ref": "#/$defs/w"}], "$defs": {"w": {"properties": {"x": ` + N + `}, "allOf": [{"properties": {"y": ` + N + `}}]}}}`, "schema.json: #/$defs/w/allOf/0/properties/y: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/allOf/0: #/$defs/w holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref"},
		{"samename", `{` + head + `, "$ref": "#/$defs/base", "properties": {"p": {"type": "string"}}, "$defs": {"base": {"properties": {"p": ` + N + `}}}}`, "schema.json: #/$defs/base/properties/p: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"refnotefmt", `{` + head + `, "properties": {"s": {"$ref": "floe:///parts/note.json", "format": "markdown"}}}`, "schema.json: #/properties/s: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"metaref", `{` + head + `, "properties": {"m": {"$ref": "https://json-schema.org/draft/2020-12/schema"}}}`, "schema.json: #/properties/m: https://json-schema.org/draft/2020-12/schema: a schema refers to no other file or URL; write what a $ref refers to in its own file"},
		{"report", `{` + head + `, "properties": {"sections": {"type":"array","items": {"type":"object","properties": {"parts": {"type":"array","items": {"$ref": "floe:///parts/any.json"}}}}}}}`, ""},
		{"defsitems", `{` + head + `, "$defs": {"item": {"type":"object","properties": {"n": ` + N + `}}}, "properties": {"xs": {"type":"array","items": {"$ref": "#/$defs/item"}}}}`, ""},
		{"refbase", `{` + head + `, "$ref": "#/$defs/base", "properties": {"p": ` + N + `}, "$defs": {"base": {"type": "object"}}}`, ""},
		{"refstrfmt", `{` + head + `, "properties": {"s": {"$ref": "#/$defs/str", "format": "markdown"}}, "$defs": {"str": {"type": "string"}}}`, ""},
		{"dep2019", `{"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object", "dependentSchemas": {"a": {"properties": {"n": ` + N + `}}}}`, "schema.json: #/dependentSchemas/a/properties/n: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"items2019", `{"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object", "properties": {"xs": {"type": "array", "items": {"properties": {"n": ` + N + `}}}}}`, ""},
		{"partnote", `{` + head + `, "properties": {"n": ` + N + `, "h": {"$ref": "floe:///parts/html.json"}}}`, ""},
		{"recursiveRef", `{"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object", "$recursiveAnchor": true, "properties": {"t": {"$recursiveRef": "#"}}}`, "schema.json: #/properties/t: floe does not follow $recursiveRef: write a $ref"},
		{"required through a $ref and allOf", `{` + head + `, "$ref": "#/$defs/b", "allOf": [{"required": ["lang"]}], "$defs": {"b": {"required": ["theme"]}}}`, lines("schema.json: #/$defs/b: the schema requires theme, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items", "schema.json: #/allOf/0: the schema requires lang, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items")},
		{"a $ref with a name beside it, and its target a start of its own", `{` + head + `, "properties": {"a": {"$ref": "#/properties/z", "properties": {"k": {"type":"string"}}}, "z": {"properties": {"k": {"type":"string","format":"html"}}}}}`, ""},
		{"dependencies in 2020-12", `{` + head + `, "dependencies": {"x": ["title"]}}`, "schema.json: #: the schema requires title, which floe reads from every data file itself (lang, template, theme, title) and leaves out of what the schema checks: require other items"},
		{"one $defs from two places", `{` + head + `, "$defs": {"item": {"type":"object","properties": {"n": ` + N + `}}}, "properties": {"xs": {"type":"array","items": {"$ref": "#/$defs/item"}}, "ys": {"type":"array","items": {"$ref": "#/$defs/item"}}}}`, ""},
		// A byte order mark at the start is left out, once; the JSON pointers are those without it.
		{"bom", mark + `{"type":"object"}`, ""},
		{"bom strng", mark + `{` + head + `, "properties": {"t": {"type":"strng"}}}`, "schema.json: #/properties/t/type: 'anyOf' failed"},
		{"two boms", mark + mark + `{"type":"object"}`, "schema.json:1:1: " + i18n.EnglishCatalog().Msg("text_bom")},
		{"null", `null`, "schema.json: #: got null, want boolean or object"},
	} {
		_, err := schema.Compile(template, "schema.json", []byte(tt.src), resources(t))
		if got := told(err); got != tt.want {
			t.Errorf("%s: Compile told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// CheckPart refuses one of floe's formats in a part, or in a Shared schema it refers to, in the file it is
// written in.
func TestCheckPart(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, want string }{
		{"note", ""},
		{"html", "parts/html.json: #/properties/body: a part's schema takes no \"format\": \"html\": of floe's formats, a part takes image alone; declare the item in the template's schema instead, or, for Markdown, declare \"type\": \"string\" alone and draw it with {{markdown .<name>}}"},
		{"stack", "parts/stack.json: #/$defs/item: the part refers to floe:///parts/any.json, a shared schema that holds items of the format markdown, which a part takes from no shared schema: refer to it from the template's schema instead, and give the part items of its own"},
	} {
		if got := told(checkPart("floe:///parts/"+tt.name+".json", resources(t))); got != tt.want {
			t.Errorf("CheckPart(%s) told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// CheckPart runs on each schema of a part what checks a template's (checkNode), with a part's rules in place of
// the checks of where a schema is that they overlap: it takes a part's image where a walk from its root reaches
// it, and refuses one elsewhere, a $ref to a part's root a walk does not reach, a $dynamicRef, one of floe's
// formats but image, and one in a Shared schema the part refers to.
func TestCheckPartNode(t *testing.T) {
	t.Parallel()

	const img = `{"type": "string", "format": "image"}`
	const note = `{"$ref": "floe:///parts/note.json"}`
	unreached := func(at string) string {
		return "parts/p.json: " + at + ": the format image marks an item to draw; declare it under properties or items, which floe follows"
	}
	noPart := func(at string) string {
		return "parts/p.json: " + at + ": the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"
	}
	html := func(at string) string {
		return "parts/p.json: " + at + `: a part's schema takes no "format": "html": of floe's formats, a part takes image alone; declare the item in the template's schema instead, or, for Markdown, declare "type": "string" alone and draw it with {{markdown .<name>}}`
	}
	shot := func(at string) string {
		return "parts/p.json: " + at + ": the part refers to floe:///parts/shot.json, a shared schema that holds items of the format image, which a part takes from no shared schema: refer to it from the template's schema instead, and give the part items of its own"
	}
	for _, tt := range []struct{ name, body, want string }{
		{"properties", `"properties": {"s": ` + img + `}`, ""},
		{"items", `"properties": {"s": {"type": "array", "items": ` + img + `}}`, ""},
		{"root", `"type": "string", "format": "image"`, ""},
		{"$defs", `"properties": {"s": {"$ref": "#/$defs/s"}}, "$defs": {"s": ` + img + `}`, ""},
		{"anyOf", `"properties": {"s": {"anyOf": [` + img + `]}}`, unreached("#/properties/s/anyOf/0")},
		{"oneOf", `"properties": {"s": {"oneOf": [` + img + `]}}`, unreached("#/properties/s/oneOf/0")},
		{"not", `"properties": {"s": {"not": ` + img + `}}`, unreached("#/properties/s/not")},
		{"then", `"properties": {"s": {"if": {"type": "string"}, "then": ` + img + `}}`, unreached("#/properties/s/then")},
		{"additionalProperties", `"additionalProperties": ` + img, unreached("#/additionalProperties")},
		{"patternProperties", `"patternProperties": {"^s": ` + img + `}`, unreached("#/patternProperties/^s")},
		{"prefixItems", `"properties": {"s": {"type": "array", "prefixItems": [` + img + `]}}`, unreached("#/properties/s/prefixItems/0")},
		{
			"$ref reached and not", `"properties": {"a": {"$ref": "#/$defs/i"}, "b": {"anyOf": [{"$ref": "#/$defs/i"}]}}, "$defs": {"i": ` + img + `}`,
			"parts/p.json: #/properties/b/anyOf/0: #/$defs/i holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref",
		},
		{
			"$ref to a part", `"properties": {"a": {"anyOf": [{"$ref": "#/$defs/n"}]}}, "$defs": {"n": {"$ref": "floe:///parts/note.json"}}`,
			noPart("#/$defs/n") + "\n" + "parts/p.json: #/properties/a/anyOf/0: #/$defs/n holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref",
		},
		{"part's root", `"properties": {"n": ` + note + `}`, ""},
		{"part's root in anyOf", `"properties": {"n": {"anyOf": [` + note + `]}}`, noPart("#/properties/n/anyOf/0")},
		{"part's root in additionalProperties", `"additionalProperties": ` + note, noPart("#/additionalProperties")},
		{"$dynamicRef", `"properties": {"s": {"$dynamicRef": "#/$defs/s"}}, "$defs": {"s": {"type": "string"}}`, "parts/p.json: #/properties/s: floe does not follow $dynamicRef: write a $ref"},
		// A part's rule is told in place of where to move what a part holds nowhere; the other checks run.
		{"html in anyOf", `"properties": {"s": {"anyOf": [{"type": "string", "format": "html"}]}}`, html("#/properties/s/anyOf/0")},
		{"shared in anyOf", `"properties": {"s": {"anyOf": [{"$ref": "floe:///parts/shot.json"}]}}`, shot("#/properties/s/anyOf/0")},
		{"html beside a part's root", `"properties": {"s": {"type": "string", "format": "html", "$ref": "floe:///parts/note.json"}}`, html("#/properties/s")},
		{
			"html beside a part's root in anyOf", `"properties": {"s": {"anyOf": [{"type": "string", "format": "html", "$ref": "floe:///parts/note.json"}]}}`,
			html("#/properties/s/anyOf/0") + "\n" + noPart("#/properties/s/anyOf/0"),
		},
		{
			"html beside a $ref into a part", `"properties": {"s": {"type": "string", "format": "html", "$ref": "floe:///parts/note.json#/properties/text"}}`,
			html("#/properties/s") + "\n" + "parts/p.json: #/properties/s: floe:///parts/note.json#/properties/text points into a part of floe's or a schema the parts share, which would be checked but not drawn as written; refer to the whole of it, floe:///parts/note.json",
		},
		{"html beside a shared one", `"properties": {"s": {"type": "string", "format": "html", "$ref": "floe:///parts/shot.json"}}`, html("#/properties/s") + "\n" + shot("#/properties/s")},
		{
			"html beside a $dynamicRef", `"properties": {"s": {"type": "string", "format": "html", "$dynamicRef": "#/$defs/s"}}, "$defs": {"s": {"type": "string"}}`,
			"parts/p.json: #/properties/s: floe does not follow $dynamicRef: write a $ref\n" + html("#/properties/s"),
		},
		// What is past a format refused is checked as with the format taken out.
		{"html beside a $ref to a part's root", `"properties": {"s": {"type": "string", "format": "html", "$ref": "#/$defs/n"}}, "$defs": {"n": ` + note + `}`, html("#/properties/s")},
		{"html beside a $ref to an image", `"properties": {"s": {"type": "string", "format": "html", "$ref": "#/$defs/i"}}, "$defs": {"i": ` + img + `}`, html("#/properties/s")},
		{"html over an image", `"properties": {"s": {"type": "object", "format": "html", "properties": {"i": ` + img + `}}}`, html("#/properties/s")},
		{"html above a part's root", `"properties": {"s": {"format": "html", "anyOf": [` + note + `]}}`, html("#/properties/s") + "\n" + noPart("#/properties/s/anyOf/0")},
		{"markdown", `"properties": {"s": {"type": "string", "format": "markdown"}}`, "parts/p.json: #/properties/s: a part's schema takes no \"format\": \"markdown\": of floe's formats, a part takes image alone; declare the item in the template's schema instead, or, for Markdown, declare \"type\": \"string\" alone and draw it with {{markdown .<name>}}"},
		{"shared", `"properties": {"s": {"$ref": "floe:///parts/shot.json"}}`, "parts/p.json: #/properties/s: the part refers to floe:///parts/shot.json, a shared schema that holds items of the format image, which a part takes from no shared schema: refer to it from the template's schema instead, and give the part items of its own"},
		// An image stops a part's walk too: the $ref beside it is not followed. A shared schema holding a format is
		// told by the part's rule alone, in place of where the $ref is.
		{"image beside a shared one", `"properties": {"s": {"type": "string", "format": "image", "$ref": "floe:///parts/shot.json"}}`, shot("#/properties/s")},
		{
			"image beside a shared one with a part", `"properties": {"s": {"type": "string", "format": "image", "$ref": "floe:///parts/parted.json"}}`,
			"parts/p.json: #/properties/s: floe:///parts/parted.json " + unfollowed,
		},
	} {
		res := resources(t)
		for name, r := range map[string]struct {
			src  string
			kind schema.Kind
		}{
			"p":      {`{"$schema": "https://json-schema.org/draft/2020-12/schema", ` + tt.body + `}`, schema.Part},
			"shot":   {`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"s": ` + img + `}}`, schema.Shared},
			"parted": {`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"n": ` + note + `}}`, schema.Shared},
		} {
			got, err := schema.NewResource("floe:///parts/"+name+".json", "parts/"+name+".json", []byte(r.src), r.kind)
			if err != nil {
				t.Fatal(err)
			}
			res["floe:///parts/"+name+".json"] = got
		}
		if got := told(checkPart("floe:///parts/p.json", res)); got != tt.want {
			t.Errorf("%s: CheckPart told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// unfollowed is the end of the mistake of a $ref floe does not follow to parts or formats.
const unfollowed = "holds parts or items of floe's formats, which floe draws only where its walk from the root reaches them, through properties, items and $refs, and stops at a format of floe's; refer to it where the walk goes, with no format of floe's beside the $ref"

// withShared returns resources(t) with the Shared schemas named in shared, each the body of a schema.
func withShared(t *testing.T, shared map[string]string) map[string]schema.Resource {
	t.Helper()

	res := resources(t)
	for name, body := range shared {
		src := `{"$schema": "https://json-schema.org/draft/2020-12/schema", ` + body + `}`
		r, err := schema.NewResource("floe:///parts/"+name+".json", "parts/"+name+".json", []byte(src), schema.Shared)
		if err != nil {
			t.Fatal(err)
		}
		res["floe:///parts/"+name+".json"] = r
	}
	return res
}

// CheckPart checks a Shared schema from its own root as a template's (checkNode), in its file, as a template, a
// part or a Shared schema refers to its root alone. A format of floe's stops its walk, as a template's does, and
// is taken where the walk reaches it: a part's rules are not its.
func TestCheckShared(t *testing.T) {
	t.Parallel()

	noPart := func(at string) string {
		return "parts/sh.json: " + at + ": the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"
	}
	into := "parts/sh.json: #/properties/a: floe:///parts/note.json#/properties/title points into a part of floe's or a schema the parts share, which would be checked but not drawn as written; refer to the whole of it, floe:///parts/note.json"
	dynamic := "parts/sh.json: #/properties/z: floe does not follow $dynamicRef: write a $ref"
	for _, tt := range []struct{ name, body, want string }{
		{"a part in anyOf", `"properties": {"m": {"anyOf": [` + N + `]}}`, noPart("#/properties/m/anyOf/0")},
		{"$dynamicRef", `"$defs": {"m": {"type": "string"}}, "properties": {"z": {"$dynamicRef": "#/$defs/m"}}`, dynamic},
		{"html in anyOf", `"properties": {"m": {"anyOf": [{"type": "string", "format": "html"}]}}`, "parts/sh.json: #/properties/m/anyOf/0: the format html marks an item to draw; declare it under properties or items, which floe follows"},
		{"into a part", `"properties": {"a": {"$ref": "floe:///parts/note.json#/properties/title"}}`, into},
		{"a part past html", `"$defs": {"h": {"type": "object", "format": "html", "properties": {"x": ` + N + `}}}, "properties": {"m": {"$ref": "#/$defs/h"}}`, noPart("#/$defs/h/properties/x")},
		{
			"three", `"$defs": {"m": {"type": "string"}}, "properties": {"z": {"$dynamicRef": "#/$defs/m"}, "m": {"anyOf": [` + N + `]}, "a": {"$ref": "floe:///parts/note.json#/properties/title"}}`,
			lines(into, noPart("#/properties/m/anyOf/0"), dynamic),
		},
		{"a part and html reached", `"properties": {"m": ` + N + `, "h": {"type": "string", "format": "html"}}`, ""},
		{"markdown at the root", `"type": "string", "format": "markdown"`, ""},
		{"a part at the root", `"$ref": "floe:///parts/note.json"`, ""},
		{"itself under a property", `"type": "object", "properties": {"c": {"$ref": "floe:///parts/sh.json"}}`, ""},
		{"itself under items", `"type": "object", "properties": {"n": ` + N + `, "kids": {"type": "array", "items": {"$ref": "floe:///parts/sh.json"}}}`, ""},
	} {
		if got := told(checkPart("floe:///parts/sh.json", withShared(t, map[string]string{"sh": tt.body}))); got != tt.want {
			t.Errorf("%s: CheckPart told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// A $ref beside a format of floe's is not followed, as the format draws the value whole: one to a Shared schema
// holding parts or formats is told at the $ref, as nothing else checks what is past it from there.
func TestCompileBesideFormat(t *testing.T) {
	t.Parallel()

	res := withShared(t, map[string]string{"sh": `"properties": {"m": ` + N + `}`, "plain": `"type": "string"`, "list": `"type": "array", "items": ` + N})
	for _, tt := range []struct{ name, body, want string }{
		{"html", `"properties": {"s": {"type": "string", "format": "html", "$ref": "floe:///parts/sh.json"}}`, "schema.json: #/properties/s: floe:///parts/sh.json " + unfollowed},
		{"image", `"properties": {"s": {"type": "string", "format": "image", "$ref": "floe:///parts/sh.json"}}`, "schema.json: #/properties/s: floe:///parts/sh.json " + unfollowed},
		{"nothing past it", `"properties": {"s": {"type": "string", "format": "html", "$ref": "floe:///parts/plain.json"}}`, ""},
		// LIMIT: one cause told twice, at the $ref and at the part past it.
		{
			"its own $defs", `"$defs": {"x": {"properties": {"m": ` + N + `}}}, "properties": {"s": {"type": "string", "format": "html", "$ref": "#/$defs/x"}}`,
			lines("schema.json: #/$defs/x/properties/m: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn", "schema.json: #/properties/s: #/$defs/x "+unfollowed),
		},
		// LIMIT: a property or items beside a $ref to a Shared schema hides the Shared schema's, unsaid.
		{"a property hiding the shared one's", `"properties": {"s": {"$ref": "floe:///parts/sh.json", "properties": {"m": {"type": "string"}}}}`, ""},
		{"items hiding the shared one's", `"properties": {"s": {"$ref": "floe:///parts/list.json", "items": {"type": "object"}}}`, ""},
	} {
		_, err := schema.Compile(template, "schema.json", []byte("{"+head+", "+tt.body+"}"), res)
		if got := told(err); got != tt.want {
			t.Errorf("%s: Compile told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// CheckPart tells the library's mistake of a part in the part's file, a metaschema it or a Shared schema it
// refers to refers to at that $ref, a $recursiveRef, and a URL it does not hold as floe's failure.
func TestCheckPartMistakes(t *testing.T) {
	t.Parallel()

	res := resources(t)
	for name, src := range map[string]string{
		"a":       `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"h": {"$ref": "floe:///parts/b.json#/$defs/h"}}}`,
		"b":       `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"n": {"type": "string"}}, "$defs": {"h": {"type": "string", "format": "html"}}}`,
		"missing": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"text": {"$ref": "#/$defs/missing"}}}`,
		"meta":    `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"m": {"$ref": "https://json-schema.org/draft/2020-12/schema"}}}`,
		"refers":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"s": {"$ref": "floe:///parts/meta-shared.json"}}}`,
		"rec":     `{"$schema": "https://json-schema.org/draft/2019-09/schema", "properties": {"s": {"$recursiveRef": "#"}}}`,
	} {
		r, err := schema.NewResource("floe:///parts/"+name+".json", "parts/"+name+".json", []byte(src), schema.Part)
		if err != nil {
			t.Fatal(err)
		}
		res["floe:///parts/"+name+".json"] = r
	}
	shared, err := schema.NewResource("floe:///parts/meta-shared.json", "parts/meta-shared.json", []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"m": {"$ref": "https://json-schema.org/draft/2020-12/schema"}}}`), schema.Shared)
	if err != nil {
		t.Fatal(err)
	}
	res["floe:///parts/meta-shared.json"] = shared
	for name, want := range map[string]string{
		"a":       "parts/a.json: #/properties/h: floe:///parts/b.json#/$defs/h points into a part of floe's or a schema the parts share, which would be checked but not drawn as written; refer to the whole of it, floe:///parts/b.json",
		"missing": `parts/missing.json: json-pointer in "floe:///parts/missing.json#/$defs/missing" not found`,
		"meta":    "parts/meta.json: #/properties/m: https://json-schema.org/draft/2020-12/schema: a schema refers to no other file or URL; write what a $ref refers to in its own file",
		"refers":  "parts/meta-shared.json: #/properties/m: https://json-schema.org/draft/2020-12/schema: a schema refers to no other file or URL; write what a $ref refers to in its own file",
		"rec":     "parts/rec.json: #/properties/s: floe does not follow $recursiveRef: write a $ref",
	} {
		if got := told(checkPart("floe:///parts/"+name+".json", res)); got != want {
			t.Errorf("CheckPart(%s) told\n%s\nwant\n%s", name, got, want)
		}
	}
	if got := told(checkPart("floe:///parts/nope.json", res)); got != "floe: schema: no resource floe:///parts/nope.json" {
		t.Errorf("CheckPart of no resource told %q", got)
	}
}

// A mistake of compiling a part the template refers to is told in the part's file, not the template's: by the
// URL the library's mistake holds, for the 6 of its kinds of mistake that floe reaches.
// ParseURLError, ParseAnchorError, InvalidRegexError and UnsupportedVocabularyError are not reached: the
// metaschema refuses such a $ref, $id, $anchor and pattern first, and the loader a metaschema of one's own.
func TestCompileMistakeInPart(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		body, want string
		head       bool // the library names two places in a map's order: only the head of what it tells is compared
	}{
		{`"properties": {"t": {"pattern": "["}}`, "parts/p.json: #/properties/t/pattern: '[' is not valid regex: error parsing regexp: missing closing ]: `[`", false},
		{`"properties": {"t": {"$ref": "#/nope"}}`, `parts/p.json: json-pointer in "floe:///parts/p.json#/nope" not found`, false},
		{`"properties": {"t": {"$ref": "#zz"}}`, `parts/p.json: anchor in "floe:///parts/p.json#zz" not found in schema "floe:///parts/p.json"`, false},
		{`"properties": {"t": {"$ref": "#/a~2"}}`, `parts/p.json: invalid json-pointer "floe:///parts/p.json#/a~2"`, false},
		{`"$defs": {"a": {"$id": "http://[::1"}}`, `parts/p.json: error in parsing id at "floe:///parts/p.json#/$defs/a"`, false},
		{`"$defs": {"a": {"$id": "https://example.com/x.json"}, "b": {"$id": "https://example.com/x.json"}}`, `parts/p.json: duplicate id "https://example.com/x.json" in "floe:///parts/p.json" at`, true},
		{`"$defs": {"a": {"$anchor": "k"}, "b": {"$anchor": "k"}}`, `parts/p.json: duplicate anchor "k" in "floe:///parts/p.json" at`, true},
	} {
		r, err := schema.NewResource("floe:///parts/p.json", "parts/p.json", []byte(`{`+head+`, `+tt.body+`}`), schema.Part)
		if err != nil {
			t.Fatal(err)
		}
		_, err = schema.Compile(template, "schema.json",
			[]byte(`{`+head+`, "properties": {"x": {"$ref": "floe:///parts/p.json"}}}`), map[string]schema.Resource{"floe:///parts/p.json": r})
		got := told(err)
		if tt.head && strings.HasPrefix(got, tt.want) && !strings.Contains(got, "\n") || got == tt.want {
			continue
		}
		t.Errorf("a part of %s told\n%s\nwant\n%s", tt.body, got, tt.want)
	}
}

// NewResource reads a part past a byte order mark at its start, its mistakes in its file; a part of null is
// the user's mistake, and a Resource NewResource did not make is floe's failure.
func TestNewResource(t *testing.T) {
	t.Parallel()

	const p = "floe:///parts/p.json"
	tmpl := []byte(`{` + head + `, "properties": {"p": {"$ref": "` + p + `"}}}`)
	run := func(r schema.Resource) (compiled, checked error) {
		res := map[string]schema.Resource{p: r}
		_, compiled = schema.Compile(template, "schema.json", tmpl, res)
		return compiled, checkPart(p, res)
	}
	if _, err := schema.NewResource("floe:///parts/p.json", "parts/p.json", []byte(mark+`{`), schema.Part); told(err) != "parts/p.json:1:2: unexpected EOF" {
		t.Errorf("NewResource of a part cut short told %q", told(err))
	}
	for src, want := range map[string]string{
		mark + `{"type":"string"}`: "",
		`null`:                     "parts/p.json: #: got null, want boolean or object",
	} {
		r, err := schema.NewResource("floe:///parts/p.json", "parts/p.json", []byte(src), schema.Part)
		if err != nil {
			t.Fatal(err)
		}
		compiled, checked := run(r)
		if told(compiled) != want || told(checked) != want {
			t.Errorf("part %q: Compile told %q, CheckPart %q; want %q", src, told(compiled), told(checked), want)
		}
	}
	compiled, checked := run(schema.Resource{Kind: schema.Part, File: "parts/p.json"})
	for _, err := range []error{compiled, checked} {
		if usererr.Tell(i18n.EnglishCatalog(), err).Failure == "" {
			t.Errorf("a Resource NewResource did not make told %q, want floe's failure", told(err))
		}
	}
	// floe's failure comes before a $schema mistake, naming the first in the order of the URLs.
	zero := schema.Resource{Kind: schema.Part, File: "parts/z.json"}
	res := map[string]schema.Resource{"floe:///parts/b.json": zero, "floe:///parts/a.json": zero}
	_, err := schema.Compile(template, "schema.json", []byte(`{"$schema": "https://example.com/s"}`), res)
	if got := told(err); got != "floe: schema: floe:///parts/a.json was not made by NewResource" {
		t.Errorf("a $schema mistake and two Resources NewResource did not make told %q", got)
	}
}
