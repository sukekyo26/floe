package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// Two names of one directory are one: written otherwise, or through a symlink. Two of templates or icons
// that cannot be read draw alike, but are not one directory, as a store's root must be.
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
	file := filepath.Join(tmp, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		a, b         string
		same, unread bool
	}{
		{dir, dir + "/", true, true},
		{dir, link, true, true},
		{filepath.Join(tmp, "none"), filepath.Join(file, "under"), false, true},
		{dir, other, false, false},
		{dir, filepath.Join(tmp, "none"), false, false},
	} {
		if got := sameDir(c.a, c.b); got != c.same {
			t.Errorf("sameDir(%s, %s) = %v, want %v", c.a, c.b, got, c.same)
		}
		if got := sameOrUnread(c.a, c.b); got != c.unread {
			t.Errorf("sameOrUnread(%s, %s) = %v, want %v", c.a, c.b, got, c.unread)
		}
	}
}
