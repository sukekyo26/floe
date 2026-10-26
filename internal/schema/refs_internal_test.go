package schema

import (
	"fmt"
	"strings"
	"testing"
)

// Filling a URL refused tells no mistake, so compile stops at mostCompiles, not at mostMistakes.
func TestCompileTimes(t *testing.T) {
	t.Parallel()

	refs := make([]string, 1000)
	for i := range refs {
		refs[i] = fmt.Sprintf(`"p%d": {"$ref": "https://example.com/b.json#/$defs/h%d"}`, i, i)
	}
	doc, err := read("schema.json", []byte(`{"properties": {`+strings.Join(refs, ", ")+`}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := &Compiled{Root: nil, url: "floe:///t/schema.json", file: "schema.json", res: nil}
	_, compiles, err := c.compile(doc)
	if compiles != mostCompiles || err == nil || err.Error() != "schema.json: https://example.com/b.json: a schema refers to no other file or URL; write what a $ref refers to in its own file" {
		t.Errorf("1000 $defs in a URL refused compiled %d times, told %v", compiles, err)
	}
}
