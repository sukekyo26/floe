package schema_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

// A part compiled alone checks a value against it, each mistake in the library's words at the JSON pointer of
// the value, with no file, the keys every data file shares (title) checked as any other.
func TestValidateAt(t *testing.T) {
	t.Parallel()

	const u = "floe:///parts/p.json"
	src := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "required": ["title"],
		"additionalProperties": false, "properties": {"title": {"type": "string"}, "a/b": {"type": "array", "items": {"type": "number"}}}}`
	r, err := schema.NewResource(u, "parts/p.json", []byte(src), schema.Part)
	if err != nil {
		t.Fatal(err)
	}
	c, err := schema.CompilePart(u, map[string]schema.Resource{u: r})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.ValidateAt(map[string]any{"title": "x"}); err != nil {
		t.Errorf("a right value = %v", err)
	}
	err = c.ValidateAt(map[string]any{"a/b": []any{1, "x"}, "c": 1})
	want := "floe: #: additional properties 'c' not allowed\nfloe: #: missing property 'title'\nfloe: #/a~1b/1: got string, want number"
	if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); got != want {
		t.Errorf("ValidateAt = %q, want %q", got, want)
	}
	if _, err := schema.CompilePart("floe:///parts/none.json", nil); err == nil {
		t.Error("CompilePart of no resource took it")
	}
}
