package schema_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/schema"
)

// floe's own schemas say "this key not here" by then's keys and additionalProperties false, never by a schema false:
// the library tells one only as "false schema", naming no key. Every part is reached from the templates.
func TestNoFalseSchema(t *testing.T) {
	t.Parallel()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	reached := map[string]bool{}
	for _, e := range k.Templates() {
		tm, ok := k.Template(e.Name)
		if !ok || tm.Schema() == nil {
			continue
		}
		falses, files := tm.Schema().FalseSchemas()
		for _, at := range falses {
			t.Errorf("%s: a schema false, told as \"false schema\" alone; list the keys then takes as true with \"additionalProperties\": false", at)
		}
		for _, f := range files {
			reached[schema.PartName(f)] = true
		}
	}
	for _, e := range k.Parts() {
		if _, isPart := k.Part(e.Name); isPart && !reached[e.Name] {
			t.Errorf("part %s is reached from no template, so its schema goes unchecked here", e.Name)
		}
	}
}
