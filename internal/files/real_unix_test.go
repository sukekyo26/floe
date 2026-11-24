//go:build unix

package files_test

import (
	"os"
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

// dotDotAbove is what RealAbove gives of base\x\new\..\parts, x not there: nothing, for Unix cannot follow .. after
// a name not there.
func dotDotAbove(string) string { return "" }

// loopAbove is what RealAbove gives of a link in a loop in dir: Unix follows no link Real fails for but by its target
// not being there, and keeps its name.
func loopAbove(dir string) (string, bool) { return filepath.Join(dir, "l1"), true }

// On Unix, a link to nothing is followed by what Real tells of it, its target not there alone: a link whose target
// cannot be looked at keeps its name; a .. in the target after a link is the system's, after a name not there false.
func TestRealAboveLinkUnix(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	base := mustReal(t, tmp)
	mkdirs := func(dirs ...string) {
		for _, d := range dirs {
			if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	mkdirs("in", "parts/.sub", "other", "priv/x")
	testfs.Symlink(t, filepath.Join(tmp, "parts", ".sub"), filepath.Join(tmp, "other", "lnk"))
	// As written: filepath.Join would fold lnk/.. by its letters.
	testfs.Symlink(t, filepath.Join(tmp, "other", "lnk")+"/../x", filepath.Join(tmp, "in", "up"))
	testfs.Symlink(t, "none/../x", filepath.Join(tmp, "in", "dotdot"))
	for _, tt := range []struct {
		name, path, want string
		ok               bool
	}{
		{".. after a link", filepath.Join(tmp, "in", "up"), filepath.Join(base, "parts", "x"), true},
		{".. after a link, a name under it", filepath.Join(tmp, "in", "up", "new"), filepath.Join(base, "parts", "x", "new"), true},
		{".. after a name not there", filepath.Join(tmp, "in", "dotdot"), "", false},
	} {
		if got, ok := files.RealAbove(tt.path); got != tt.want || ok != tt.ok {
			t.Errorf("%s: RealAbove(%q) = %q, %v, want %q, %v", tt.name, tt.path, got, ok, tt.want, tt.ok)
		}
	}
	t.Run("a target that cannot be looked at", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root looks at any directory")
		}
		acc := filepath.Join(tmp, "in", "acc")
		testfs.Symlink(t, filepath.Join(tmp, "priv", "x"), acc)
		priv := filepath.Join(tmp, "priv")
		if err := os.Chmod(priv, 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(priv, 0o755) //nolint:errcheck // TempDir's removal tells a failure
		if got, ok := files.RealAbove(acc); got != filepath.Join(base, "in", "acc") || !ok {
			t.Errorf("RealAbove(%q) = %q, %v, want %q", acc, got, ok, filepath.Join(base, "in", "acc"))
		}
	})
}
