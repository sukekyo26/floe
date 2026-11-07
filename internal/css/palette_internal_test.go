package css

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestPaletteInDocs keeps every variable of floe's default palette named, as it is, in the docs' table of the palette,
// English and Japanese: raw SVG and one's own themes rely on that table, and one left out of it is no promise.
// LIMIT: default's palette.css alone is read; a variable only a theme sets is the theme's own, and the docs promise
// none of them.
func TestPaletteInDocs(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../kit/floe/themes/default/palette.css")
	if err != nil {
		t.Fatal(err)
	}
	found := declarations(src, func(p string) bool { return strings.HasPrefix(p, "--") })
	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	if len(names) == 0 {
		t.Fatal("default's palette.css sets no variable")
	}
	slices.Sort(names)
	for _, doc := range []string{"../../docs/templates.md", "../../docs/templates.ja.md"} {
		text, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if !strings.Contains(string(text), "`"+name+"`") {
				t.Errorf("%s does not name %s, which default's palette.css sets: write `%s` in the table of the palette",
					doc, name, name)
			}
		}
	}
}
