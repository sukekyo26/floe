package diagram

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

// examples holds a directory per language. English holds every example; another language may hold
// some of them.
//
//go:embed examples
var examples embed.FS

// Example is a built-in spec of one kind of diagram, for an agent to copy the layout of.
type Example struct {
	Name    string
	Summary string // the first line of the spec's opening comment
	Spec    []byte
}

// Examples returns the built-in examples by name, each in the first of i18n.Fallbacks(lang) that has it.
func Examples(lang i18n.Lang) ([]Example, error) {
	entries, err := examples.ReadDir("examples/" + string(i18n.English))
	if err != nil {
		return nil, fmt.Errorf("read the examples: %w", err)
	}
	out := make([]Example, 0, len(entries))
	for _, e := range entries {
		src, err := example(lang, e.Name())
		if err != nil {
			return nil, err
		}
		first, _, _ := strings.Cut(string(src), "\n")
		out = append(out, Example{
			Name: strings.TrimSuffix(e.Name(), ".yaml"), Summary: strings.TrimPrefix(first, "# "), Spec: src,
		})
	}
	return out, nil
}

func example(lang i18n.Lang, file string) ([]byte, error) {
	for _, l := range i18n.Fallbacks(lang) {
		src, err := examples.ReadFile("examples/" + string(l) + "/" + file)
		if err == nil {
			return src, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read the example %s: %w", file, err)
		}
	}
	return nil, fmt.Errorf("read the example %s: %w", file, fs.ErrNotExist)
}
