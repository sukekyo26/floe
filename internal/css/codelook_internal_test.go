package css

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"
)

// TestCodeLookCopiesNothing keeps a part of floe's that takes code's look to what code sets: it writes no length of
// padding (code's --floe-code-pad and --floe-code-room hold the room), no share of a colour (--floe-code-faint and
// --floe-code-head-faint hold the faint ones) and does not name the copy button (code sets the room for it).
func TestCodeLookCopiesNothing(t *testing.T) {
	t.Parallel()

	parts := os.DirFS("../kit/floe/parts")
	var takers []string
	err := fs.WalkDir(parts, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(name) != "part.yaml" {
			return err
		}
		src, err := fs.ReadFile(parts, name)
		if err != nil {
			return err
		}
		for line := range strings.Lines(string(src)) {
			if looks, ok := strings.CutPrefix(strings.TrimSpace(line), "looks:"); ok &&
				slices.Contains(strings.FieldsFunc(looks, func(r rune) bool { return strings.ContainsRune("[], ", r) }), "code") {
				takers = append(takers, path.Dir(name))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(takers) == 0 {
		t.Fatal("no part of floe's takes code's look")
	}
	for _, part := range takers {
		name := part + "/part.css"
		src, err := fs.ReadFile(parts, name)
		if err != nil {
			t.Fatal(err)
		}
		for prop, values := range declarations(src, func(p string) bool { return strings.HasPrefix(p, "padding") || p == "color" }) {
			for _, d := range values {
				for _, tok := range d.value {
					if (prop == "color" && tok.tt == css.PercentageToken) || (prop != "color" && tok.tt == css.DimensionToken) {
						t.Errorf("%s: %s: %s in %q; read code's --floe-code-* rather than copy its numbers", name, prop, text(src, d.value), d.rule)
					}
				}
			}
		}
		toks := lex(src)
		for i, tok := range toks {
			if tok.tt == css.IdentToken && string(tok.data) == "floe-copy" && i > 0 && toks[i-1].tt == css.DelimToken && string(toks[i-1].data) == "." {
				t.Errorf("%s: names .floe-copy; code sets the room for the copy button in --floe-code-room", name)
			}
		}
	}
}
