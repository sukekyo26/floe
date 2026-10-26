package diagram

import (
	"embed"
	"fmt"
	"strings"
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
}

// Examples returns the built-in examples by name.
func Examples() ([]Example, error) {
	entries, err := examples.ReadDir("examples")
	if err != nil {
		return nil, fmt.Errorf("diagram: read the examples: %w", err)
	}
	out := make([]Example, 0, len(entries))
	for _, e := range entries {
		src, err := examples.ReadFile("examples/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("diagram: read the example %s: %w", e.Name(), err)
		}
		first, _, _ := strings.Cut(string(src), "\n")
		name, summary := strings.TrimSuffix(e.Name(), ".yaml"), strings.TrimPrefix(first, "# ")
		out = append(out, Example{Name: name, Summary: summary, Spec: src})
	}
	return out, nil
}
