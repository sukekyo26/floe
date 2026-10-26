package diagram

import (
	"embed"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// examples holds a spec (.yaml) per kind of diagram, its text in English and Japanese.
//
//go:embed examples
var examples embed.FS

// Example is a built-in spec of one kind of diagram, for an agent to copy the layout of.
type Example struct {
	Name    string
	Summary string // the first line of the spec's opening comment
	Spec    []byte
	value   any // Spec read
}

// Examples returns the built-in examples by name, read once.
var Examples = sync.OnceValues(func() ([]Example, error) {
	entries, err := examples.ReadDir("examples")
	if err != nil {
		return nil, fmt.Errorf("diagram: read the examples: %w", err)
	}
	out := make([]Example, 0, len(entries))
	for _, e := range entries {
		file := "examples/" + e.Name()
		src, err := examples.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("diagram: read the example %s: %w", e.Name(), err)
		}
		root, err := yamlpos.Parse(file, src)
		var v any
		if err == nil {
			err = yamlpos.Decode(file, root, &v)
		}
		if err != nil {
			// In words, not wrapped: a mistake of floe's own example is no mistake of the user's.
			//nolint:err113,errorlint // see above
			return nil, fmt.Errorf("diagram: read the example %s: %s", e.Name(), err.Error())
		}
		first, _, _ := strings.Cut(string(src), "\n")
		name, summary := strings.TrimSuffix(e.Name(), ".yaml"), strings.TrimPrefix(first, "# ")
		out = append(out, Example{Name: name, Summary: summary, Spec: src, value: doc.Ints(v)})
	}
	return out, nil
})

// ExampleOf returns the built-in example of the kind name, or the mistake that there is none.
func ExampleOf(name string) (Example, error) {
	all, err := Examples()
	if err != nil {
		return Example{}, err //nolint:exhaustruct // none
	}
	i := slices.IndexFunc(all, func(e Example) bool { return e.Name == name })
	if i < 0 {
		names := make([]string, len(all))
		for i, e := range all {
			names[i] = e.Name
		}
		return Example{}, usererr.New("diagram_unknown_example", name, strings.Join(names, ", ")) //nolint:exhaustruct // none
	}
	return all[i], nil
}
