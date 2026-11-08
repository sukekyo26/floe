package diagram

import (
	"encoding/json"
	"flag"
	"os"
	"testing"
)

// faceClasses are the classes a diagram draws text in, as faceOf is given them: frame and frame grouped name a frame's
// heading, on its group.
var faceClasses = []string{"main", "sub", "main code", "sub code", "cell code", "type", "label", "label strong", "frame", "frame grouped"}

// facesFile holds what faceOf gives each of faceClasses, [size, narrow], for the browser's test of part.css
// (TestRenderedDiagramFaces) to read: what faceOf measures, not a copy written by hand.
const facesFile = "testdata/faces.json"

// TestFaceOfGolden holds facesFile to faceOf, and writes it anew under -update (the flag diagram_test.go declares).
func TestFaceOfGolden(t *testing.T) {
	t.Parallel()

	faces := make(map[string][2]float64, len(faceClasses))
	for _, c := range faceClasses {
		size, narrow := faceOf(c)
		faces[c] = [2]float64{size, float64(narrow)}
	}
	got, err := json.MarshalIndent(faces, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if f := flag.Lookup("update"); f != nil && f.Value.String() == "true" {
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
