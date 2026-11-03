package schema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Filling a URL refused tells no mistake, so compile stops at mostCompiles, not at mostMistakes, and tells the URL
// once however much inside it is filled.
func TestCompileTimes(t *testing.T) {
	t.Parallel()

	// n refs take n+2 compiles (the first, one a ref filled, the last that passes), so mostCompiles-2 reach the
	// limit; 51 more keep reaching it if the count changes by a few.
	refs := make([]string, mostCompiles+49)
	for i := range refs {
		refs[i] = fmt.Sprintf(`"p%d": {"$ref": "https://example.com/b.json#/$defs/h%d"}`, i, i)
	}
	doc, err := read("schema.json", []byte(`{"properties": {`+strings.Join(refs, ", ")+`}}`))
	if err != nil {
		t.Fatal(err)
	}
	c := &Compiled{Root: nil, url: "floe:///t/schema.json", file: "schema.json", res: nil}
	h := c.holesOf(doc)
	_, err = c.compileIn(h)
	want := "schema.json: https://example.com/b.json: a schema refers to no other file or URL; write what a $ref refers to in its own file"
	if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); err == nil || h.compiles != mostCompiles || got != want {
		t.Errorf("%d $defs in a URL refused compiled %d times, told %q", len(refs), h.compiles, got)
	}
}
