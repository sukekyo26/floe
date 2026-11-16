// Package testfs holds what tests make that differs by system (a FIFO, a lock tried without waiting, a symlink, a
// junction), and the marks of the tests that do not run on Windows: UnixOnly, of what holds on Unix alone, and
// Pending, of what S315's later pull requests mend. Only tests import it.
package testfs

import (
	"io/fs"
	"os"
	"testing"
)

// UnixOnly skips t on Windows, where what it expects (the umask, permission bits, root, a FIFO) does not hold;
// for good. On Unix it does nothing.
func UnixOnly(t *testing.T, why string) {
	t.Helper()
	if isWindows {
		t.Skip("Unix only: " + why)
	}
}

// Pending skips t on Windows until the pull request why names mends what it expects there; S315's last one
// removes Pending, and every call left with it. On Unix it does nothing.
func Pending(t *testing.T, why string) {
	t.Helper()
	if isWindows {
		t.Skip("pending: " + why)
	}
}

// Mkfifo makes a FIFO at path, or skips t where there are none (Windows).
func Mkfifo(t *testing.T, path string) {
	t.Helper()
	UnixOnly(t, "a FIFO")
	if err := mkfifo(path); err != nil {
		t.Fatal(err)
	}
}

// TryLock reports whether f could be locked as files.Lock locks it, without waiting; the lock goes with f.
func TryLock(t *testing.T, f *os.File) bool {
	t.Helper()
	return tryLock(f)
}

// Symlink makes the symlink at to old, or skips t where the system does not let this user make one (Windows,
// without the developer mode or an administrator).
func Symlink(t *testing.T, old, at string) {
	t.Helper()
	err := os.Symlink(old, at)
	if err != nil && privilegeNotHeld(err) {
		t.Skip("a symlink cannot be made here: " + err.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
}

// Junction makes a junction at link to the directory target, or skips t where there are none (Unix).
func Junction(t *testing.T, link, target string) {
	t.Helper()
	if !isWindows {
		t.Skip("junctions are Windows'")
	}
	if err := junction(link, target); err != nil {
		t.Fatal(err)
	}
}

// CheckedOutLink skips t on Windows when path, a symlink in the repository, was checked out as a regular file
// holding where it leads (Git for Windows with core.symlinks false).
func CheckedOutLink(t *testing.T, path string) {
	t.Helper()
	if !isWindows {
		return
	}
	if fi, err := os.Lstat(path); err == nil && fi.Mode().Type() != fs.ModeSymlink {
		t.Skip(path + " was not checked out as a symlink (core.symlinks)")
	}
}
