package diagram

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed examples/*.yaml
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
		return nil, fmt.Errorf("read the examples: %w", err)
	}
	out := make([]Example, 0, len(entries))
	for _, e := range entries {
		src, err := examples.ReadFile("examples/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read the examples: %w", err)
		}
		first, _, _ := strings.Cut(string(src), "\n")
		out = append(out, Example{
			Name: strings.TrimSuffix(e.Name(), ".yaml"), Summary: strings.TrimPrefix(first, "# "), Spec: src,
		})
	}
	return out, nil
}
