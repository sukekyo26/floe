//go:build unix

package files_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"floe/internal/files"
)

// A new file takes perm less the umask; one written in place of another keeps that one's permissions, not its
// setuid bit, under any umask.
func TestWriteAtomicPermissions(t *testing.T) { //nolint:paralleltest // sets the umask of the process
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	dir := t.TempDir()
	path := filepath.Join(dir, "p.html")
	if err := files.WriteAtomic(path, []byte("one"), 0o644, nil); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("a new file: %v, %v; want mode 0600 under the umask 077", info, err)
	}
	for _, mode := range []os.FileMode{0o664, 0o200, 0o4755} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := files.WriteAtomic(path, []byte("two"), 0o644, before); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Lstat(path); err != nil || info.Mode() != mode.Perm() {
			t.Errorf("in place of %v: %v, %v; want %v", mode, info.Mode(), err, mode.Perm())
		}
	}
}
