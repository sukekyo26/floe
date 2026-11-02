package css

import (
	"io/fs"
	"os"
	"path"
	"testing"
)

// TestOpacity keeps opacity in floe's CSS to 0, what hides: what is faint is faded by its colour, which needs no
// opacity. It reads every rule as written, in every state and medium.
// LIMIT: one's own parts, themes and templates are not read, nor filter: opacity(), which floe's CSS does not write.
func TestOpacity(t *testing.T) {
	t.Parallel()

	floe := os.DirFS("../kit/floe")
	read := 0
	err := fs.WalkDir(floe, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".css" {
			return err
		}
		src, err := fs.ReadFile(floe, name)
		if err != nil {
			return err
		}
		read++
		for _, d := range declarations(src, func(p string) bool { return p == "opacity" })["opacity"] {
			if text(src, d.value) != "0" {
				t.Errorf("%s: opacity: %s in %q; fade it by its colour (color-mix(… transparent)), opacity is 0 alone",
					name, text(src, d.value), d.rule)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if read == 0 {
		t.Fatal("no CSS found in floe's")
	}
}
