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
	h := c.holesOf(doc)
	if _, err := c.compileIn(h); err == nil || h.compiles != mostCompiles {
		t.Errorf("1000 $defs in a URL refused compiled %d times, told %v", h.compiles, err)
	}
}
