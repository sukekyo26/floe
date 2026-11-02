package css

import "testing"

// TestOpacity keeps opacity in floe's CSS to 0, what hides: what is faint is faded by its colour, which needs no
// opacity. It reads every rule as written, in every state and medium.
// LIMIT: one's own parts, themes and templates are not read, nor filter: opacity(), which floe's CSS does not write.
func TestOpacity(t *testing.T) {
	t.Parallel()

	sheets(t, "../kit/floe", func(name string, src []byte) {
		for _, d := range declarations(src, func(p string) bool { return p == "opacity" })["opacity"] {
			if text(src, d.value) != "0" {
				t.Errorf("%s: opacity: %s in %q; fade it by its colour (color-mix(… transparent)), opacity is 0 alone",
					name, text(src, d.value), d.rule)
			}
		}
	})
}
