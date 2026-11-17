package files_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"floe/internal/files"
	"floe/internal/testfs"
)

// A mark stays while the file does, read or not, and changes with what a write could hide: the same size written
// again with its write time set back, or another file renamed over it. A directory's does not change with the files
// made in it.
func TestMarkChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "a.html")
	write(t, path, "one")
	m := files.Mark(path)
	if m == "-" {
		t.Fatal("a file there is marked -")
	}
	if again := files.Mark(path); again != m {
		t.Errorf("marked twice: %q, then %q", m, again)
	}
	if _, err := os.ReadFile(path); err != nil { //nolint:gosec // G304: the test's own
		t.Fatal(err)
	}
	if again := files.Mark(path); again != m {
		t.Errorf("marked after a read: %q, then %q", m, again)
	}
	// Written again at its size and times: only the change time tells it, once the clock moves on.
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	atime := testfs.AccessTime(t, path)
	changed := false
	for end := time.Now().Add(2 * time.Second); !changed && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		write(t, path, "two")
		if err := os.Chtimes(path, atime, before.ModTime()); err != nil {
			t.Fatal(err)
		}
		changed = files.Mark(path) != m
	}
	if !changed {
		t.Error("a file written again at its size, its times set back, kept its mark")
	}
	m = files.Mark(path)
	other := filepath.Join(dir, "b.html")
	write(t, other, "two")
	if err := os.Rename(other, path); err != nil {
		t.Fatal(err)
	}
	if files.Mark(path) == m {
		t.Error("a file renamed over another kept the other's mark")
	}
	d := files.Mark(dir)
	write(t, filepath.Join(dir, "c.html"), "new")
	if again := files.Mark(dir); again != d {
		t.Errorf("a directory's mark changed with a file made in it: %q, then %q", d, again)
	}
	if got := files.Mark(filepath.Join(dir, "none")); got != "-" {
		t.Errorf("no file is marked %q, want -", got)
	}
}

// A link's mark holds what it leads to, after >: one changed there changes it, and one that leads nowhere is >-.
func TestMarkLink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	to := filepath.Join(dir, "to.html")
	write(t, to, "one")
	link := filepath.Join(dir, "link.html")
	testfs.Symlink(t, to, link)
	linked(t, link, to)
}

// A junction's mark holds the directory it leads to, as a symlink's does (Windows).
func TestMarkJunction(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	to := filepath.Join(dir, "to")
	if err := os.Mkdir(to, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	testfs.Junction(t, link, to)
	if m, want := files.Mark(link), files.Mark(to); !strings.HasSuffix(m, ">"+want) || want == "-" {
		t.Errorf("a junction is marked %q, want %q after >", m, want)
	}
}

// linked checks the mark of link, which leads to the file to: it holds to's, and follows its changes.
func linked(t *testing.T, link, to string) {
	t.Helper()
	m := files.Mark(link)
	if !strings.HasSuffix(m, ">"+files.Mark(to)) {
		t.Errorf("a link is marked %q, want %q after >", m, files.Mark(to))
	}
	write(t, to, "a longer one")
	if files.Mark(link) == m {
		t.Error("a link kept its mark when what it leads to changed")
	}
	if err := os.Remove(to); err != nil {
		t.Fatal(err)
	}
	if got := files.Mark(link); !strings.HasSuffix(got, ">-") {
		t.Errorf("a link to nowhere is marked %q, want >- at its end", got)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
