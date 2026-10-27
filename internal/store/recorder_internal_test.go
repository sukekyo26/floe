package store

import (
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
)

// The recorder notes an image that is not there, which kit.ReadImage stats before it opens, and one that is a
// FIFO is refused at its Stat, never opened.
func TestRecorder(t *testing.T) {
	t.Parallel()

	r := &recorder{fsys: fstest.MapFS{"f.png": {Data: nil, Mode: fs.ModeNamedPipe}}, names: nil, seen: map[string]bool{}}
	_, err := kit.ReadImage(r, "f.png", ".", "d.yaml")
	if got, want := usererr.Tell(i18n.EnglishCatalog(), err).Render(""), `"f.png" is not a file of an image, but a directory or the like: name a regular file of the image`; got != want {
		t.Errorf("told %s, want %s", got, want)
	}
	if _, err := kit.ReadImage(r, "gone.png", ".", "d.yaml"); err == nil {
		t.Error("an image not there was read")
	}
	if want := []string{"f.png", "gone.png"}; !slices.Equal(r.names, want) {
		t.Errorf("noted %v, want %v", r.names, want)
	}
}
