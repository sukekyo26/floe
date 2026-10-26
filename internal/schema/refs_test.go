package schema_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

func notFound(file, u string) string {
	return fmt.Sprintf("%s: json-pointer in %q not found", file, u)
}

const (
	noURL  = ": a schema refers to no other file or URL; write what a $ref refers to in its own file"
	gone   = "schema.json: floe:///parts/gone.json is not a part; refer to one as floe:///parts/<name>.json"
	tmplID = "floe:///t/schema.json"
)

// refParts are parts whose own $refs resolve or not: p a string, q and r each a $ref to $defs they lack, t
// the schema true.
func refParts(t *testing.T) map[string]schema.Resource {
	t.Helper()

	res := map[string]schema.Resource{}
	for name, src := range map[string]string{"p": `{"type":"string"}`, "q": `{"$ref":"#/$defs/m"}`, "r": `{"$ref":"#/$defs/n"}`, "t": `true`} {
		r, err := schema.NewResource("parts/"+name+".json", []byte(src), schema.Part)
		if err != nil {
			t.Fatal(err)
		}
		res["floe:///parts/"+name+".json"] = r
	}
	return res
}

// The $refs that cannot be resolved, each a missing #/$defs/<name> or a URL refused, are told all, in the
// order of their URLs, the same on every run.
func TestCompileRefs(t *testing.T) {
	t.Parallel()

	many := make([]string, 0, 49)
	for i := range 49 {
		many = append(many, fmt.Sprintf(`"u%d": {"$ref": "https://example.com/u%d.json#/$defs/h"}, "d%d": {"$ref": "#/$defs/d%d"}`, i, i, i, i))
	}
	for _, tt := range []struct{ name, src, want string }{
		{
			"three $defs", `{"properties": {"a": {"$ref": "#/$defs/x"}, "b": {"$ref": "#/$defs/y"}, "c": {"$ref": "#/$defs/z"}}}`,
			lines(notFound("schema.json", tmplID+"#/$defs/x"), notFound("schema.json", tmplID+"#/$defs/y"), notFound("schema.json", tmplID+"#/$defs/z")),
		},
		{
			"two URLs and a part", `{"properties": {"a": {"$ref": "https://example.com/a.json"}, "b": {"$ref": "https://example.com/b.json"}, "c": {"$ref": "floe:///parts/gone.json"}}}`,
			lines(gone, "schema.json: https://example.com/a.json"+noURL, "schema.json: https://example.com/b.json"+noURL),
		},
		{
			"a part, a $defs and no part", `{"properties": {"a": {"$ref": "floe:///parts/p.json"}, "b": {"$ref": "#/$defs/y"}, "c": {"$ref": "floe:///parts/gone.json"}}}`,
			lines(notFound("schema.json", tmplID+"#/$defs/y"), gone),
		},
		{
			"allOf", `{"allOf": [{"$ref": "#/$defs/x"}, {"$ref": "#/$defs/y"}]}`,
			lines(notFound("schema.json", tmplID+"#/$defs/x"), notFound("schema.json", tmplID+"#/$defs/y")),
		},
		{
			"in parts", `{"properties": {"a": {"$ref": "floe:///parts/q.json"}, "b": {"$ref": "floe:///parts/r.json"}}}`,
			lines(notFound("parts/q.json", "floe:///parts/q.json#/$defs/m"), notFound("parts/r.json", "floe:///parts/r.json#/$defs/n")),
		},
		{"a URL's fragment", `{"properties": {"a": {"$ref": "https://example.com/b.json#/$defs/h"}}}`, "schema.json: https://example.com/b.json" + noURL},
		{"98 of both", `{"properties": {` + strings.Join(many, ", ") + `}}`, ""},
		// A mistake inside a URL refused is the {} floe put there: unsaid.
		{"a URL's anchor", `{"properties": {"a": {"$ref": "https://example.com/a.json#foo"}}}`, "schema.json: https://example.com/a.json" + noURL},
		{"a URL's definitions", `{"properties": {"a": {"$ref": "https://example.com/b.json#/definitions/x"}}}`, "schema.json: https://example.com/b.json" + noURL},
		{"a URL's ~", `{"properties": {"a": {"$ref": "https://example.com/b.json#/$defs/a~1b"}}}`, "schema.json: https://example.com/b.json" + noURL},
		{"no part's properties", `{"properties": {"a": {"$ref": "floe:///parts/gone.json#/properties/t"}}}`, gone},
		// A doc whose root is not an object is not filled.
		{"a part of true", `{"properties": {"a": {"$ref": "floe:///parts/t.json#/$defs/x"}}}`, notFound("parts/t.json", "floe:///parts/t.json#/$defs/x")},
	} {
		first := ""
		for i := range 20 {
			_, err := schema.Compile(tmplID, "schema.json", []byte(tt.src), refParts(t))
			got := told(err)
			if i == 0 {
				first = got
			} else if got != first {
				t.Fatalf("%s: run %d told\n%s\nrun 0\n%s", tt.name, i, got, first)
			}
		}
		if tt.name == "98 of both" {
			if n := strings.Count(first, "\n") + 1; n != 98 {
				t.Errorf("98 of both told %d mistakes", n)
			}
			continue
		}
		if first != tt.want {
			t.Errorf("%s: Compile told\n%s\nwant\n%s", tt.name, first, tt.want)
		}
	}
}

// A $ref compile does not fill stops it, told: what came before it may change from run to run, but no
// mistake is one filling made, nor floe's failure.
func TestCompileRefsStop(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, src, stop string }{
		{"under a $defs", `{"properties": {"a": {"$ref": "#/$defs/x/minimum"}, "b": {"$ref": "#/$defs/y"}}, "$defs": {"x": {"type": "number"}}}`, notFound("schema.json", tmplID+"#/$defs/x/minimum")},
		{"over each other", `{"properties": {"a": {"$ref": "#/$defs/x"}, "b": {"$ref": "#/$defs/x/properties/y"}}}`, notFound("schema.json", tmplID+"#/$defs/x/properties/y")},
		{"an anchor", `{"properties": {"a": {"$ref": "#nope"}, "b": {"$ref": "#/$defs/x"}}}`, `schema.json: anchor in "floe:///t/schema.json#nope" not found in schema "floe:///t/schema.json"`},
		{"a ~", `{"properties": {"a": {"$ref": "#/$defs/a~1b"}}}`, notFound("schema.json", tmplID+"#/$defs/a~1b")},
		{"a metaschema", `{"properties": {"a": {"$ref": "https://json-schema.org/draft/2020-12/schema#/$defs/nope"}}}`, notFound("schema.json", "https://json-schema.org/draft/2020-12/schema#/$defs/nope")},
		{"$defs of true", `{"$defs": true, "properties": {"a": {"$ref": "#/$defs/x"}}}`, "schema.json: #/$defs: got boolean, want object"},
	} {
		for range 20 {
			_, err := schema.Compile(tmplID, "schema.json", []byte(tt.src), refParts(t))
			r := usererr.Tell(i18n.EnglishCatalog(), err)
			got := r.Render("floe: ")
			if r.Failure != "" || !strings.Contains(got, tt.stop) || strings.Contains(got, "got boolean, want number") {
				t.Fatalf("%s: Compile told\n%s\nwant it among them\n%s", tt.name, got, tt.stop)
			}
		}
	}
}

// compile tells at most 100 mistakes, tells a URL refused once however much inside it is filled, and leaves
// the resources it was given as they were.
func TestCompileRefsBounds(t *testing.T) {
	t.Parallel()

	refs := make([]string, 0, 101)
	for i := range 101 {
		refs = append(refs, fmt.Sprintf(`"p%d": {"$ref": "#/$defs/d%d"}`, i, i))
	}
	_, err := schema.Compile(tmplID, "schema.json", []byte(`{"properties": {`+strings.Join(refs, ", ")+`}}`), nil)
	r := usererr.Tell(i18n.EnglishCatalog(), err)
	if r.Failure != "" || len(r.Mistakes) != 100 {
		t.Errorf("101 missing $defs told %d mistakes, failure %q", len(r.Mistakes), r.Failure)
	}
	refs = make([]string, 0, 1000)
	for i := range 1000 {
		refs = append(refs, fmt.Sprintf(`"p%d": {"$ref": "https://example.com/b.json#/$defs/h%d"}`, i, i))
	}
	_, err = schema.Compile(tmplID, "schema.json", []byte(`{"properties": {`+strings.Join(refs, ", ")+`}}`), nil)
	if got := told(err); got != "schema.json: https://example.com/b.json"+noURL {
		t.Errorf("1000 $defs in a URL refused told %q", got)
	}
	res := refParts(t)
	src := []byte(`{"properties": {"a": {"$ref": "floe:///parts/q.json"}}}`)
	for range 2 {
		if _, err := schema.Compile(tmplID, "schema.json", src, res); told(err) != notFound("parts/q.json", "floe:///parts/q.json#/$defs/m") {
			t.Fatalf("a part filled once told %q the next time", told(err))
		}
	}
}

// A resource the library will not take is floe's failure, not the user's mistake.
func TestCompileResourceRefused(t *testing.T) {
	t.Parallel()

	for _, u := range []string{"https://json-schema.org/draft/2020-12/schema", "floe:///parts/a%zz.json"} {
		res := refParts(t)
		res[u] = res["floe:///parts/p.json"]
		_, err := schema.Compile(tmplID, "schema.json", []byte(`{"type": "object"}`), res)
		if r := usererr.Tell(i18n.EnglishCatalog(), err); r.Failure == "" {
			t.Errorf("a resource at %s told %q, want floe's failure", u, r.Render("floe: "))
		}
	}
}
