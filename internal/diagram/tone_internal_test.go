package diagram

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// A part of a diagram takes the tones the parts of a template take.
func TestTonesOfTheParts(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("../tmpl/parts/tone.json")
	if err != nil {
		t.Fatal(err)
	}
	var tone struct{ Enum []string }
	if err = json.Unmarshal(b, &tone); err != nil || !slices.Equal(tone.Enum, tones) {
		t.Errorf("tone.json = %v, %v; a diagram's tones are %v", tone.Enum, err, tones)
	}
}
