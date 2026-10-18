package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// Two names of one directory are one: written otherwise, or through a symlink; two that do not exist
// are one too, and one that does not exist is not one that does.
func TestSameDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	dir, other, link := filepath.Join(tmp, "dir"), filepath.Join(tmp, "other"), filepath.Join(tmp, "link")
	for _, d := range []string{dir, other} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{dir, dir + "/", true},
		{dir, link, true},
		{filepath.Join(tmp, "none"), filepath.Join(tmp, "gone"), true},
		{dir, other, false},
		{dir, filepath.Join(tmp, "none"), false},
	} {
		if got := sameDir(c.a, c.b); got != c.same {
			t.Errorf("sameDir(%s, %s) = %v, want %v", c.a, c.b, got, c.same)
		}
	}
}
