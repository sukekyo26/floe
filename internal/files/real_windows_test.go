package files_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

// Real follows junctions, makes short names long and case as on disk, drops a trailing dot of a path without \\?\
// (Windows drops it before it opens), gives a path with \\?\ and one without the same \\?\ real path, and fails for a
// junction leading nowhere or round; a path over 300 characters is made real too.
func TestRealWindows(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	file := filepath.Join(target, "a.html")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, file, "a")
	want := mustReal(t, file)
	if !strings.HasPrefix(want, `\\?\`) {
		t.Errorf("Real(%q) = %q, want it with \\\\?\\", file, want)
	}
	j := filepath.Join(tmp, "j")
	testfs.Junction(t, j, target)
	for name, p := range map[string]string{
		"through a junction": filepath.Join(j, "a.html"),
		"in upper case":      strings.ToUpper(file),
		"a trailing dot":     file + ".",
		"with \\\\?\\":       `\\?\` + file,
	} {
		if got := mustReal(t, p); got != want {
			t.Errorf("Real %s %q = %q, want %q", name, p, got, want)
		}
	}
	if strings.Contains(tmp, "~") && strings.Contains(mustReal(t, tmp), "~") {
		t.Errorf("Real(%q) = %q, want its short names long", tmp, mustReal(t, tmp))
	}

	gone := filepath.Join(tmp, "gone")
	if err := os.Mkdir(gone, 0o700); err != nil {
		t.Fatal(err)
	}
	nowhere := filepath.Join(tmp, "nowhere")
	testfs.Junction(t, nowhere, gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	j1, j2 := filepath.Join(tmp, "j1"), filepath.Join(tmp, "j2")
	testfs.Junction(t, j1, target)
	testfs.Junction(t, j2, j1)
	if err := os.Remove(j1); err != nil {
		t.Fatal(err)
	}
	testfs.Junction(t, j1, j2)
	for _, p := range []string{nowhere, j1} {
		if got, err := files.Real(p); err == nil {
			t.Errorf("Real of a junction leading nowhere or round %q = %q, want an error", p, got)
		}
	}

	long := tmp
	for len(long) < 300 {
		long = filepath.Join(long, strings.Repeat("d", 40))
	}
	if err := os.MkdirAll(long, 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(long, "x.html"), "x")
	if got := mustReal(t, filepath.Join(long, "x.html")); !strings.HasSuffix(got, `\x.html`) || len(got) < 300 {
		t.Errorf("Real of a long path = %q", got)
	}
}

// LongName makes a short name long without following a junction, fails for a name not there, and of names with a
// stream gives what the system gives (S315's W26): the stream of a directory's short name as written, a file's
// stream under it made long, and an alternate stream not there an error.
func TestLongNameWindows(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	secret := filepath.Join(tmp, ".secret")
	if err := os.Mkdir(secret, 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(secret, "id.html"), "s")
	j := filepath.Join(tmp, "j")
	testfs.Junction(t, j, secret)
	short := testfs.ShortName(t, secret)
	for _, tt := range []struct{ path, suffix string }{
		{filepath.Join(short, "id.html"), `\.secret\id.html`},
		{short, `\.secret`},
		{`\\?\` + filepath.Join(short, "id.html"), `\.secret\id.html`},
		{j, `\j`},
	} {
		if got, err := files.LongName(tt.path); err != nil || !strings.HasSuffix(got, tt.suffix) {
			t.Errorf("LongName(%q) = %q, %v, want it ending %q", tt.path, got, err, tt.suffix)
		}
	}
	// A stream's name (W26) is refused before the system reads it: no ordinary name.
	for _, p := range []string{short + "::$INDEX_ALLOCATION", filepath.Join(short, "id.html") + "::$DATA"} {
		if got, err := files.LongName(p); !errors.Is(err, usererr.ErrNotLocal) {
			t.Errorf("LongName(%q) = %q, %v, want ErrNotLocal", p, got, err)
		}
	}
	// Over MAX_PATH, which takes the buffer made again: a short name under a long directory, given with \\?\ (Real's,
	// whose names are long already).
	long := mustReal(t, tmp)
	for len(long) < 300 {
		long = filepath.Join(long, strings.Repeat("d", 40))
	}
	deep := filepath.Join(long, ".secret")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(deep, "id.html"), "s")
	in := filepath.Join(testfs.ShortName(t, deep), "id.html")
	if got, err := files.LongName(in); err != nil || got != filepath.Join(deep, "id.html") {
		t.Errorf("LongName(%q) = %q, %v, want %q", in, got, err, filepath.Join(deep, "id.html"))
	}
	for _, p := range []string{filepath.Join(j, "none"), filepath.Join(tmp, "none", "x.html"), short + ":x"} {
		if got, err := files.LongName(p); err == nil {
			t.Errorf("LongName(%q) = %q, want an error", p, got)
		}
	}
}

// RealAbove stops at a volume alone that cannot be opened (a share not there), which has no directory above it, and
// takes a drive without its root (C:, relative to the drive's working directory) as no absolute path.
func TestRealAboveWindows(t *testing.T) {
	t.Parallel()

	for _, p := range []string{`\\127.0.0.1\nosuchshare`, `\\127.0.0.1\nosuchshare\x`, `C:`, `C:x`} {
		if got, ok := files.RealAbove(p); ok {
			t.Errorf("RealAbove(%q) = %q, want false", p, got)
		}
	}
}

// dotDotAbove is what RealAbove gives of base\x\new\..\parts, x not there: Windows folds .. by its letters before
// it opens a path, so it is base\x\parts.
func dotDotAbove(base string) string { return filepath.Join(base, "x", "parts") }

// loopAbove is what RealAbove gives of a link in a loop in dir: Windows' walk refuses the loop (ELOOP).
func loopAbove(string) (string, bool) { return "", false }

// On Windows, a junction to nothing is followed as a link is; a link to a drive not there gives false (walk tells it
// without opening it, TestWalkNoDrive). A symlink made to nothing reads no directory made there later.
func TestRealAboveLinkWindows(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	base := mustReal(t, tmp)
	j := filepath.Join(tmp, "j")
	testfs.Junction(t, j, filepath.Join(tmp, "x"))
	if got, ok := files.RealAbove(j); got != filepath.Join(base, "x") || !ok {
		t.Errorf("RealAbove of a junction to nothing = %q, %v, want %q", got, ok, filepath.Join(base, "x"))
	}
	// A symlink made to nothing is a file's: a directory made there later is not read through it (RealAbove's LIMIT).
	later := filepath.Join(tmp, "later")
	testfs.Symlink(t, later, filepath.Join(tmp, "s"))
	if err := os.Mkdir(later, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadDir(filepath.Join(tmp, "s")); err == nil {
		t.Error("a directory was read through a symlink made to nothing")
	}
	z := filepath.Join(tmp, "z")
	testfs.Symlink(t, testfs.FreeDrive(t)+`\x`, z)
	if got, ok := files.RealAbove(z); ok {
		t.Errorf("RealAbove of a link to a drive not there = %q, want false", got)
	}
}
