package usererr_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"floe/internal/i18n"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

// A path that cannot be read or written is told at the name given, in the system's words without the paths it
// names; an error that is no path's is floe's failure.
func TestCannotReadWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, rerr := os.ReadFile(filepath.Join(dir, "gone.yaml"))
	werr := os.Rename(filepath.Join(dir, "gone"), filepath.Join(dir, "x"))
	english := i18n.EnglishCatalog()
	const words = "S315-P7: system words"
	for _, c := range []struct {
		name    string
		err     error
		want    string
		pending string
	}{
		{"read", usererr.CannotRead("gone.yaml", rerr), "gone.yaml: no such file or directory", words},
		{
			"no file", usererr.CannotRead("f.yaml", &fs.PathError{Op: "open", Path: "/abs/f.yaml", Err: usererr.ErrNotFile}),
			"f.yaml: this is neither a regular file nor a directory (a FIFO, a device or the like), which floe may never finish reading: put a regular file here",
			"",
		},
		{"write", usererr.CannotWrite("out", werr), "out: cannot be written (no such file or directory): check that its " +
			"directory exists and can be written to, and that its name is free and not too long", words},
		{"in system words", usererr.InSystemWords("icons_previous_left", "out", werr, "out.old"), "out: cannot be written (no such file or " +
			"directory), and the previous icons are left in out.old: move them back here, or fetch the icons again", words},
		// A directory a setting names is told for it; the default as it is told without one.
		{"setting", usererr.SettingDir("FLOE_DIR", "/s", werr, usererr.CannotWrite("/s", werr)), "floe: FLOE_DIR is /s, which " +
			"cannot be used as a directory (no such file or directory); make it a directory floe can use, or set " +
			"FLOE_DIR to another directory", words},
		{"setting, no file", usererr.SettingDir("FLOE_DIR", "/s", usererr.ErrNotFile, nil), "floe: FLOE_DIR is /s, which cannot be used " +
			"as a directory (neither a regular file nor a directory); make it a directory floe can use, or set " +
			"FLOE_DIR to another directory", ""},
		{"default", usererr.SettingDir("", "out", werr, usererr.CannotWrite("out", werr)), "out: cannot be written (no such " +
			"file or directory): check that its directory exists and can be written to, and that its name is free " +
			"and not too long", words},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.pending != "" {
				testfs.Pending(t, c.pending)
			}
			if got := usererr.Tell(english, c.err).Render("floe: "); got != c.want {
				t.Errorf("told %q, want %q", got, c.want)
			}
		})
	}
	for _, err := range []error{
		usererr.CannotRead("a", errors.New("odd")), usererr.CannotWrite("a", errors.New("odd")),
		usererr.InSystemWords("icons_previous_left", "a", errors.New("odd"), "b"),
		usererr.InSystemWordsOf("kit_image_unread", "a", errors.New("odd")),
		usererr.SettingDir("FLOE_DIR", "a", errors.New("odd"), nil),
	} {
		if r := usererr.Tell(english, err); r.Failure != "a: odd" {
			t.Errorf("%v is told %+v, want floe's failure", err, r)
		}
	}
}

// A file that cannot be read where it was named is told without a place, its system's words then its name and the
// rest as arguments, for the caller to place.
func TestInSystemWordsOf(t *testing.T) {
	t.Parallel()

	_, rerr := os.ReadFile(filepath.Join(t.TempDir(), "gone.png"))
	var e *usererr.Error
	if !errors.As(usererr.InSystemWordsOf("kit_image_unread", "gone.png", rerr, "more"), &e) {
		t.Fatal("not a mistake")
	}
	if e.Key != "kit_image_unread" || e.File() != "" || e.Line() != 0 || len(e.Args) != 3 ||
		!slices.Equal(e.Args[1:], []any{"gone.png", "more"}) {
		t.Errorf("made %s at %q:%d with %q", e.Key, e.File(), e.Line(), e.Args)
	}
	t.Run("system words", func(t *testing.T) {
		t.Parallel()
		testfs.Pending(t, "S315-P7: system words")
		if len(e.Args) == 0 || e.Args[0] != "no such file or directory" {
			t.Errorf("made %s with %q, the system's words first", e.Key, e.Args)
		}
	})
}
