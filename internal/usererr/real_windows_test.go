package usererr_test

import (
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/usererr"
)

// A path as files.Real gives it on Windows is told as people write it, in a mistake's text and its place.
func TestWordedShown(t *testing.T) {
	t.Parallel()

	const verbatim, shown = `\\?\C:\x\a.html`, `C:\x\a.html`
	r := usererr.Tell(i18n.EnglishCatalog(), usererr.NewAt(verbatim, 0, 0, "link_hidden_home", verbatim))
	if len(r.Mistakes) != 1 || r.Mistakes[0].File != shown || !strings.Contains(r.Mistakes[0].Text, shown) ||
		strings.Contains(r.Mistakes[0].Text, `\\?\`) {
		t.Errorf("told %+v, want %s in its place and text", r, shown)
	}
}
