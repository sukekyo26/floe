//go:build unix

package files_test

import (
	"path/filepath"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
)

// On Unix, Real is EvalSymlinks, and LongName gives every path back as written, one not there too.
func TestRealUnix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "a.html")
	write(t, file, "a")
	link := filepath.Join(dir, "link.html")
	testfs.Symlink(t, file, link)
	for _, p := range []string{dir, file, link} {
		want, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		if got, gerr := files.Real(p); got != want || gerr != nil {
			t.Errorf("Real(%q) = %q, %v, want %q", p, got, gerr, want)
		}
	}
	for _, p := range []string{link, filepath.Join(dir, "none", "x.html"), "rel/../x"} {
		if got, err := files.LongName(p); got != p || err != nil {
			t.Errorf("LongName(%q) = %q, %v, want it as written", p, got, err)
		}
	}
}
