package diagram

import (
	"encoding/json"
	"flag"
	"os"
	"slices"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/schema"
)

// rowStyles is where the diagram's schema lists the styles a row may be written in.
const rowStyles = URL + "#/$defs/box/properties/rows/items/prefixItems/1"

// faces is what facesFile holds: for each class a diagram draws text in, as faceOf is given it, [size, narrow]; on
// the text itself (Text), or on a frame's group, which its heading takes it from (Group).
type faces struct{ Text, Group map[string][2]float64 }

// facesFile holds faces for the browser's test of part.css (TestRenderedDiagramFaces) to read: what faceOf measures,
// not a copy written by hand.
const facesFile = "testdata/faces.json"

// faceClasses returns the classes a diagram draws text in, from where they are made: rowClass, of the styles the schema
// lets a row be written in too, labelClass, tableCells, and frameClass, on a frame's group.
func faceClasses(t *testing.T) (text, group []string) {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	c, err := schema.CompilePart(URL, k.Resources())
	if err != nil {
		t.Fatal(err)
	}
	styles := c.EnumAt(rowStyles)
	if len(styles) == 0 {
		t.Fatalf("the diagram's schema has no enum at %s: name where it lists a row's styles", rowStyles)
	}
	text = make([]string, 0, 6+len(styles))
	text = append(text, rowClass("", true), rowClass("", false), labelClass(false), labelClass(true), tableCells[0], tableCells[1])
	for _, style := range styles {
		text = append(text, rowClass(style, false))
	}
	slices.Sort(text)
	return slices.Compact(text), []string{frameClass(""), frameClass("icon")}
}

// TestFaceOfGolden holds facesFile to faceOf of every class faceClasses makes, and writes it anew under -update (the
// flag diagram_test.go declares).
func TestFaceOfGolden(t *testing.T) {
	t.Parallel()

	text, group := faceClasses(t)
	of := func(classes []string) map[string][2]float64 {
		m := make(map[string][2]float64, len(classes))
		for _, c := range classes {
			size, narrow := faceOf(c)
			m[c] = [2]float64{size, float64(narrow)}
		}
		return m
	}
	got, err := json.MarshalIndent(faces{Text: of(text), Group: of(group)}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	update := flag.Lookup("update")
	if update == nil {
		t.Fatal("no flag update: diagram_test.go declares it")
	}
	getter, ok := update.Value.(flag.Getter)
	if !ok {
		t.Fatal("the flag update gives no value")
	}
	on, ok := getter.Get().(bool)
	if !ok {
		t.Fatal("the flag update is no bool")
	}
	if on {
		if err = os.WriteFile(facesFile, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(facesFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("faceOf gives\n%s\nwhich %s does not hold: run just golden", got, facesFile)
	}
}
