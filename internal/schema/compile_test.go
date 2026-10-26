package schema_test

import (
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
		{"badjson", `{` + head + `, `, "schema.json:1:78: unexpected EOF"},
		{"strng", `{` + head + `, "properties": {"t": {"type":"strng"}}}`, "schema.json: #/properties/t/type: 'anyOf' failed"},
		{"xtype", `{` + head + `, "x": {"type": 3}, "properties": {"s": {"$ref": "#/x"}}}`, "schema.json: #/x/type: 'anyOf' failed"},
		{"pctmeta", `{` + head + `, "properties": {"項目": {"type": 3}, "a/b~c": {"minLength": "x"}}}`, "schema.json: #/properties/a~1b~0c/minLength: got string, want integer\nschema.json: #/properties/項目/type: 'anyOf' failed"},
		{"pct", `{` + head + `, "allOf": [{"properties": {"項目": ` + N + `}}], "patternProperties": {"a/b~c": {"type":"string","format": "html"}}}`, "schema.json: #/allOf/0/properties/項目: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/patternProperties/a~1b~0c: the format html marks an item to draw; declare it under properties or items, which floe follows"},
		{"twopaths", `{` + head + `, "properties": {"a": {"$ref": "#/$defs/p"}}, "allOf": [{"properties": {"b": {"$ref": "#/$defs/p"}}}], "$defs": {"p": ` + N + `}}`, "schema.json: #/allOf/0/properties/b: #/$defs/p holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows"},
		{"defsallof", `{` + head + `, "$defs": {"p": ` + N + `}, "allOf": [{"properties": {"b": {"$ref": "#/$defs/p"}}}]}`, "schema.json: #/$defs/p: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/allOf/0/properties/b: #/$defs/p holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows"},
		{"chain", `{` + head + `, "allOf": [{"$ref": "#/$defs/q"}], "$defs": {"q": {"properties": {"b": {"$ref": "#/$defs/p"}}}, "p": ` + N + `}}`, "schema.json: #/$defs/p: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/$defs/q/properties/b: #/$defs/p holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows\nschema.json: #/allOf/0: #/$defs/q holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows"},
		{"allofany", `{` + head + `, "allOf": [{"$ref": "floe:///parts/any.json"}]}`, "schema.json: #/allOf/0: floe:///parts/any.json holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows"},
		{"leak", `{` + head + `, "properties": {"a": {"$ref": "#/$defs/w"}}, "$defs": {"w": {"allOf": [{"properties": {"x": ` + N + `}}]}}}`, "schema.json: #/$defs/w/allOf/0/properties/x: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn"},
		{"aw", `{` + head + `, "properties": {"a": {"$ref": "#/$defs/w"}}, "allOf": [{"$ref": "#/$defs/w"}], "$defs": {"w": {"properties": {"x": ` + N + `}, "allOf": [{"properties": {"y": ` + N + `}}]}}}`, "schema.json: #/$defs/w/allOf/0/properties/y: the part note draws no item here: floe draws what properties and items reach from the root, through a $ref when nothing beside it draws; the item here is checked but not drawn\nschema.json: #/allOf/0: #/$defs/w holds parts or items of floe's formats, which floe draws; refer to it under properties or items, which floe follows"},
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
		{"two boms", mark + mark + `{"type":"object"}`, `schema.json:1:1: invalid character '\ufeff' looking for beginning of value`},
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
		{"html", "parts/html.json: #/properties/body: a part's schema takes no \"format\": \"html\": its part.html draws its items, which the checks of a format do not reach; declare the item in the template's schema instead, or, for Markdown, declare \"type\": \"string\" alone and draw it with {{markdown .<name>}}"},
		{"stack", "parts/stack.json: #/$defs/item: the part refers to floe:///parts/any.json, a schema of floe's that holds items of the format markdown, which a part's part.html cannot draw: refer to it from the template's schema instead, and give the part items of its own"},
	} {
		if got := told(schema.CheckPart("floe:///parts/"+tt.name+".json", resources(t))); got != tt.want {
			t.Errorf("CheckPart(%s) told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// CheckPart tells the library's mistake of a part in the part's file, a metaschema it refers to at its $ref,
// and a URL it does not hold as floe's failure.
func TestCheckPartMistakes(t *testing.T) {
	t.Parallel()

	res := resources(t)
	for name, src := range map[string]string{
		"a":       `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"h": {"$ref": "floe:///parts/b.json#/$defs/h"}}}`,
		"b":       `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"n": {"type": "string"}}, "$defs": {"h": {"type": "string", "format": "html"}}}`,
		"missing": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"text": {"$ref": "#/$defs/missing"}}}`,
		"meta":    `{"$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"m": {"$ref": "https://json-schema.org/draft/2020-12/schema"}}}`,
	} {
		r, err := schema.NewResource("floe:///parts/"+name+".json", "parts/"+name+".json", []byte(src), schema.Part)
		if err != nil {
			t.Fatal(err)
		}
		res["floe:///parts/"+name+".json"] = r
	}
	for name, want := range map[string]string{
		"a":       "parts/a.json: #/properties/h: floe:///parts/b.json#/$defs/h points into a part of floe's or a schema the parts share, which would be checked but not drawn as written; refer to the whole of it, floe:///parts/b.json",
		"missing": `parts/missing.json: json-pointer in "floe:///parts/missing.json#/$defs/missing" not found`,
		"meta":    "parts/meta.json: #/properties/m: https://json-schema.org/draft/2020-12/schema: a schema refers to no other file or URL; write what a $ref refers to in its own file",
	} {
		if got := told(schema.CheckPart("floe:///parts/"+name+".json", res)); got != want {
			t.Errorf("CheckPart(%s) told\n%s\nwant\n%s", name, got, want)
		}
	}
	if got := told(schema.CheckPart("floe:///parts/nope.json", res)); got != "floe: schema: no resource floe:///parts/nope.json" {
		t.Errorf("CheckPart of no resource told %q", got)
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
		return compiled, schema.CheckPart(p, res)
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
