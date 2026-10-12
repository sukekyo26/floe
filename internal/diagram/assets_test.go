package diagram_test

import (
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/page"
)

func TestAssets(t *testing.T) {
	t.Parallel()

	got, err := diagram.Assets()
	if err != nil {
		t.Fatal(err)
	}
	// Pasted into body-only HTML, the assets must not make it read as a whole document, or the page
	// would lose the template (and its CSS variables) around it.
	if page.IsDocument(got) {
		t.Error("Assets() reads as a whole HTML document")
	}
	for _, w := range []string{`<script>(// `, `"ja":{"diagram_zoom_fit":"全体"`, `"en":{"diagram_zoom_fit":"Fit"`, ");</script>\n<style>\n", ".dg .box rect", "</style>"} {
		if !strings.Contains(got, w) {
			t.Errorf("Assets() does not contain %s", w)
		}
	}
	// Pages carry these files as they are, so they are written in English.
	for _, f := range []string{"assets/zoom.js", "assets/diagram.css"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if i := strings.IndexFunc(string(b), func(r rune) bool { return r > unicode.MaxASCII }); i >= 0 {
			t.Errorf("%s is not in plain English at byte %d", f, i)
		}
	}
}
