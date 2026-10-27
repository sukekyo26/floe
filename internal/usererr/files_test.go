package usererr_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// A path that cannot be read or written is told at the name given, in the system's words without the paths it
// names; an error that is no path's is floe's failure.
func TestCannotReadWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, rerr := os.ReadFile(filepath.Join(dir, "gone.yaml"))
	werr := os.Rename(filepath.Join(dir, "gone"), filepath.Join(dir, "x"))
	english := i18n.EnglishCatalog()
	for _, c := range []struct {
		err  error
		want string
	}{
		{usererr.CannotRead("gone.yaml", rerr), "gone.yaml: no such file or directory"},
		{
			usererr.CannotRead("f.yaml", &fs.PathError{Op: "open", Path: "/abs/f.yaml", Err: usererr.ErrNotFile}),
			"f.yaml: this is neither a regular file nor a directory (a FIFO, a device or the like), which floe may never finish reading: put a regular file here",
		},
		{usererr.CannotWrite("out", werr), "out: cannot be written (no such file or directory): check that its " +
			"directory exists and can be written to, and that its name is free and not too long"},
		{usererr.InSystemWords("icons_previous_left", "out", werr, "out.old"), "out: cannot be written (no such file or " +
			"directory), and the previous icons are left in out.old: move them back here, or fetch the icons again"},
	} {
		if got := usererr.Tell(english, c.err).Render("floe: "); got != c.want {
			t.Errorf("told %q, want %q", got, c.want)
		}
	}
	for _, err := range []error{
		usererr.CannotRead("a", errors.New("odd")), usererr.CannotWrite("a", errors.New("odd")),
		usererr.InSystemWords("icons_previous_left", "a", errors.New("odd"), "b"),
	} {
		if r := usererr.Tell(english, err); r.Failure != "a: odd" {
			t.Errorf("%v is told %+v, want floe's failure", err, r)
		}
	}
}
