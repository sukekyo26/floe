package tmpl

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
)

// The parts and diagrams take their colours, fonts and width from the palette alone, which the report
// theme defines whole; the parts' tones are the diagrams'.
func TestPalette(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(set.parts))
	for name := range set.parts {
		names = append(names, name)
	}
	vars, err := partVariables(names)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range append(vars, diagram.Variables()...) {
		if !slices.Contains(palette, v) {
			t.Errorf("%s is not of the palette", v)
		}
	}
	css, err := builtin.ReadFile("builtin/report/themes/default.css")
	if err != nil {
		t.Fatal(err)
	}
	defined := definedVariables(css)
	for _, v := range palette {
		if !defined[v] {
			t.Errorf("the report theme leaves %s out", v)
		}
	}

	b, err := partFiles.ReadFile("parts/tone.json")
	if err != nil {
		t.Fatal(err)
	}
	var tone struct{ Enum []string }
	if err = json.Unmarshal(b, &tone); err != nil || !slices.Equal(tone.Enum, diagram.Tones) {
		t.Errorf("tone.json = %v, %v; the diagrams' tones are %v", tone.Enum, err, diagram.Tones)
	}
}
