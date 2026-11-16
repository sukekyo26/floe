package files_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"floe/internal/files"
	"floe/internal/testfs"
)

// shortName is the 8.3 name of path, or skips t where the volume makes none.
func shortName(tb testing.TB, path string) string {
	tb.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		tb.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(p, &buf[0], windows.MAX_PATH)
	if err != nil {
		tb.Fatal(err)
	}
	short := windows.UTF16ToString(buf[:n])
	if filepath.Base(short) == filepath.Base(path) {
		tb.Skip("this volume makes no short names")
	}
	return short
}

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
	short := shortName(t, secret)
	for _, tt := range []struct{ path, suffix string }{
		{filepath.Join(short, "id.html"), `\.secret\id.html`},
		{short, `\.secret`},
		{`\\?\` + filepath.Join(short, "id.html"), `\.secret\id.html`},
		{j, `\j`},
		{short + "::$INDEX_ALLOCATION", `\` + filepath.Base(short) + "::$INDEX_ALLOCATION"},
		{filepath.Join(short, "id.html") + "::$DATA", `\.secret\id.html::$DATA`},
	} {
		if got, err := files.LongName(tt.path); err != nil || !strings.HasSuffix(got, tt.suffix) {
			t.Errorf("LongName(%q) = %q, %v, want it ending %q", tt.path, got, err, tt.suffix)
		}
	}
	for _, p := range []string{filepath.Join(j, "none"), filepath.Join(tmp, "none", "x.html"), short + ":x"} {
		if got, err := files.LongName(p); err == nil {
			t.Errorf("LongName(%q) = %q, want an error", p, got)
		}
	}
}
