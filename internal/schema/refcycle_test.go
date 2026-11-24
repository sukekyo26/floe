package schema_test

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	"floe/internal/schema"
	"floe/internal/usererr"
)

const (
	zaURL = "floe:///parts/za.json"
	zbURL = "floe:///parts/zb.json"
	shURL = "floe:///parts/sh.json"
	s2URL = "floe:///parts/sh2.json"
	// loopAB is $defs a and b, each the other.
	loopAB = `"$defs": {"a": {"$ref": "#/$defs/b"}, "b": {"$ref": "#/$defs/a"}}`
	// loopA is $defs a, itself.
	loopA = `"$defs": {"a": {"$ref": "#/$defs/a"}}`
	// abPath is the path of loopAB from a, in the part za; baPath from b; aPath that of loopA.
	abPath = zaURL + "#/$defs/a → " + zaURL + "#/$defs/b → " + zaURL + "#/$defs/a"
	baPath = zaURL + "#/$defs/b → " + zaURL + "#/$defs/a → " + zaURL + "#/$defs/b"
	aPath  = zaURL + "#/$defs/a → " + zaURL + "#/$defs/a"
)

// mistakesOf returns each mistake of err as its file, its pointer and its key, and for schema_ref_cycle its
// path too; it fails t on a failure that is no mistake.
func mistakesOf(t *testing.T, err error) []string {
	t.Helper()

	var found usererr.List
	if failed := found.Add(err); failed != nil {
		t.Fatal(failed)
	}
	told := make([]string, 0, len(found))
	for _, e := range found {
		line := e.File() + " " + e.At() + " " + e.Key
		if e.Key == "schema_ref_cycle" {
			line += " " + fmt.Sprint(e.Args...)
		}
		told = append(told, line)
	}
	return told
}

// A $ref that leads back to itself through $refs alone is refused where it is written, in a part, a Shared schema
// and a template alike, each $ref of the loop in the check of its own file: no value passes it. One that leads
// into a loop it is not on is not, nor a schema that names itself across properties, items or anyOf.
func TestRefCycle(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		parts, shared map[string]string // the schemas of one's own, by name
		template      string            // a template's schema, "" for none
		want          []string
	}{
		{
			"a part's root naming itself by its URL",
			map[string]string{"za": `{"$ref": "` + zaURL + `"}`},
			nil, "",
			[]string{"za.json # schema_ref_cycle " + zaURL + "# → " + zaURL + "#"},
		},
		{
			"a part's root naming #",
			map[string]string{"za": `{"$ref": "#"}`},
			nil, "",
			[]string{"za.json # schema_ref_cycle " + zaURL + "# → " + zaURL + "#"},
		},
		{
			"two $defs, the root leading into them",
			map[string]string{"za": `{"$ref": "#/$defs/a", ` + loopAB + `}`},
			nil, "",
			[]string{"za.json #/$defs/a schema_ref_cycle " + abPath, "za.json #/$defs/b schema_ref_cycle " + baPath},
		},
		{
			"a $defs leading into a loop it is not on",
			map[string]string{"za": `{"properties": {"k": {"$ref": "#/$defs/a"}}, "$defs": {"a": {"$ref": "#/$defs/b"}, "b": {"$ref": "#/$defs/c"}, "c": {"$ref": "#/$defs/b"}}}`},
			nil, "",
			[]string{
				"za.json #/$defs/b schema_ref_cycle " + zaURL + "#/$defs/b → " + zaURL + "#/$defs/c → " + zaURL + "#/$defs/b",
				"za.json #/$defs/c schema_ref_cycle " + zaURL + "#/$defs/c → " + zaURL + "#/$defs/b → " + zaURL + "#/$defs/c",
			},
		},
		{
			"a part leading into a loop of Shared schemas",
			map[string]string{"za": `{"$ref": "` + shURL + `"}`},
			map[string]string{"sh": `{"$ref": "` + s2URL + `"}`, "sh2": `{"$ref": "` + shURL + `"}`},
			"",
			[]string{
				"sh.json # schema_ref_cycle " + shURL + "# → " + s2URL + "# → " + shURL + "#",
				"sh2.json # schema_ref_cycle " + s2URL + "# → " + shURL + "# → " + s2URL + "#",
			},
		},
		{
			"a loop of three",
			map[string]string{"za": `{"properties": {"k": {"$ref": "#/$defs/a"}}, "$defs": {"a": {"$ref": "#/$defs/b"}, "b": {"$ref": "#/$defs/c"}, "c": {"$ref": "#/$defs/a"}}}`},
			nil, "",
			[]string{
				"za.json #/$defs/a schema_ref_cycle " + zaURL + "#/$defs/a → " + zaURL + "#/$defs/b → " + zaURL + "#/$defs/c → " + zaURL + "#/$defs/a",
				"za.json #/$defs/b schema_ref_cycle " + zaURL + "#/$defs/b → " + zaURL + "#/$defs/c → " + zaURL + "#/$defs/a → " + zaURL + "#/$defs/b",
				"za.json #/$defs/c schema_ref_cycle " + zaURL + "#/$defs/c → " + zaURL + "#/$defs/a → " + zaURL + "#/$defs/b → " + zaURL + "#/$defs/c",
			},
		},
		{
			"type and const beside the $ref",
			map[string]string{"za": `{"type": "object", "const": {}, "$ref": "#"}`},
			nil, "",
			[]string{"za.json # schema_ref_cycle " + zaURL + "# → " + zaURL + "#"},
		},
		{
			"a format of floe's beside the $ref, where Follow stops",
			map[string]string{"za": `{"type": "string", "format": "markdown", "$ref": "#"}`},
			nil, "",
			[]string{"za.json # schema_ref_cycle " + zaURL + "# → " + zaURL + "#", "za.json # schema_ref_unreached"},
		},
		{
			"two parts' roots",
			map[string]string{"za": `{"$ref": "` + zbURL + `"}`, "zb": `{"$ref": "` + zaURL + `"}`},
			nil, "",
			[]string{
				"za.json # schema_ref_cycle " + zaURL + "# → " + zbURL + "# → " + zaURL + "#",
				"zb.json # schema_ref_cycle " + zbURL + "# → " + zaURL + "# → " + zbURL + "#",
			},
		},
		{
			"a part and a Shared schema",
			map[string]string{"za": `{"$ref": "` + shURL + `"}`},
			map[string]string{"sh": `{"$ref": "` + zaURL + `"}`},
			"",
			[]string{
				"za.json # schema_ref_cycle " + zaURL + "# → " + shURL + "# → " + zaURL + "#",
				"sh.json # schema_ref_cycle " + shURL + "# → " + zaURL + "# → " + shURL + "#",
			},
		},
		{
			// zb's q is named by za alone: the library holds no $defs nothing names, so zb's own check does not meet it.
			"a loop through a $ref into a part",
			map[string]string{"za": `{"$ref": "` + zbURL + `#/$defs/q"}`, "zb": `{"type": "object", "$defs": {"q": {"$ref": "` + zaURL + `"}}}`},
			nil, "",
			[]string{"za.json # schema_part_into", "za.json # schema_ref_cycle " + zaURL + "# → " + zbURL + "#/$defs/q → " + zaURL + "#"},
		},
		{
			"under not, which a value passes",
			map[string]string{"za": `{"not": {"$ref": "#/$defs/a"}, ` + loopA + `}`},
			nil, "",
			[]string{"za.json #/$defs/a schema_ref_cycle " + aPath},
		},
		{
			"a branch of anyOf, which a value passes by another",
			map[string]string{"za": `{"anyOf": [{"type": "string"}, {"$ref": "#/$defs/a"}], ` + loopA + `}`},
			nil, "",
			[]string{"za.json #/$defs/a schema_ref_cycle " + aPath},
		},
		{
			"under if",
			map[string]string{"za": `{"if": {"$ref": "#/$defs/a"}, "then": false, ` + loopA + `}`},
			nil, "",
			[]string{"za.json #/$defs/a schema_ref_cycle " + aPath},
		},
		{
			"under a property a value may leave out",
			map[string]string{"za": `{"type": "object", "properties": {"k": {"$ref": "#/$defs/a"}}, ` + loopAB + `}`},
			nil, "",
			[]string{"za.json #/$defs/a schema_ref_cycle " + abPath, "za.json #/$defs/b schema_ref_cycle " + baPath},
		},
		{
			"a name the fragment escapes",
			map[string]string{"za": `{"properties": {"k": {"$ref": "#/$defs/a%20b"}}, "$defs": {"a b": {"$ref": "#/$defs/a%20b"}}}`},
			nil, "",
			[]string{"za.json #/$defs/a b schema_ref_cycle " + zaURL + "#/$defs/a b → " + zaURL + "#/$defs/a b"},
		},
		{
			"a template's root naming #", nil, nil, `{"$ref": "#"}`,
			[]string{"schema.json # schema_ref_cycle " + template + "# → " + template + "#"},
		},
		{
			"a template's $defs", nil, nil, `{` + head + `, "properties": {"x": {"$ref": "#/$defs/a"}}, ` + loopAB + `}`,
			[]string{
				"schema.json #/$defs/a schema_ref_cycle " + template + "#/$defs/a → " + template + "#/$defs/b → " + template + "#/$defs/a",
				"schema.json #/$defs/b schema_ref_cycle " + template + "#/$defs/b → " + template + "#/$defs/a → " + template + "#/$defs/b",
			},
		},
		{
			// The library checks no first item against items, yet Walk goes into each: the parts are refused before it.
			"parts' roots under an items beside prefixItems",
			map[string]string{"za": `{"$ref": "` + zbURL + `"}`, "zb": `{"$ref": "` + zaURL + `"}`},
			nil,
			`{` + head + `, "properties": {"x": {"prefixItems": [{}], "items": {"$ref": "` + zaURL + `"}}}}`,
			[]string{
				"za.json # schema_ref_cycle " + zaURL + "# → " + zbURL + "# → " + zaURL + "#",
				"zb.json # schema_ref_cycle " + zbURL + "# → " + zaURL + "# → " + zbURL + "#",
			},
		},
		{"itself across properties", map[string]string{"za": `{"type": "object", "properties": {"k": {"$ref": "#"}}}`}, nil, "", nil},
		{"itself across items", map[string]string{"za": `{"items": {"$ref": "#"}}`}, nil, "", nil},
		{"itself across anyOf", map[string]string{"za": `{"anyOf": [{"type": "string"}, {"$ref": "#"}]}`}, nil, "", nil},
		{
			"each other across properties",
			map[string]string{"za": `{"type": "object", "properties": {"k": {"$ref": "` + zbURL + `"}}}`, "zb": `{"type": "object", "properties": {"k": {"$ref": "` + zaURL + `"}}}`},
			nil, "", nil,
		},
		{
			"a chain that ends",
			map[string]string{"za": `{"$ref": "#/$defs/a", "$defs": {"a": {"$ref": "#/$defs/b"}, "b": {"type": "string"}}}`},
			nil, "", nil,
		},
		{
			"two properties naming one $defs",
			map[string]string{"za": `{"type": "object", "properties": {"k": {"$ref": "#/$defs/a"}, "l": {"$ref": "#/$defs/a"}}, "$defs": {"a": {"type": "string"}}}`},
			nil, "", nil,
		},
		// The library holds no $defs nothing names: the loop is in no schema a check meets.
		{"a loop in a $defs nothing names", map[string]string{"za": `{"type": "object", ` + loopA + `}`}, nil, "", nil},
		// LIMIT of checkOwn: not $refs alone. The library refuses it once a value is checked.
		{"itself through allOf", map[string]string{"za": `{"allOf": [{"$ref": "#"}]}`}, nil, "", nil},
	} {
		res := resources(t)
		add := func(schemas map[string]string, k schema.Kind) {
			for name, src := range schemas {
				u := "floe:///parts/" + name + ".json"
				r, err := schema.NewResource(u, name+".json", []byte(src), k)
				if err != nil {
					t.Fatalf("%s: %v", tt.name, err)
				}
				res[u] = r
			}
		}
		add(tt.parts, schema.Part)
		add(tt.shared, schema.Shared)
		var got []string
		for _, name := range slices.Concat(slices.Sorted(maps.Keys(tt.parts)), slices.Sorted(maps.Keys(tt.shared))) {
			got = append(got, mistakesOf(t, checkPart("floe:///parts/"+name+".json", res))...)
		}
		if tt.template != "" {
			_, err := schema.Compile(template, "schema.json", []byte(tt.template), res)
			got = append(got, mistakesOf(t, err)...)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, lines(got...), lines(tt.want...))
		}
	}
}
