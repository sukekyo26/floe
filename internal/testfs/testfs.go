// Package testfs holds what tests make that differs by system (a FIFO, a lock tried without waiting, a symlink, a
// junction), and the marks of the tests that do not run on Windows: UnixOnly, of what holds on Unix alone, and
// Pending, of what S315's later pull requests mend. Only tests import it.
package testfs

import (
	"os"
	"testing"
	"time"
)

// UnixOnly skips t on Windows, where what it expects (the umask, permission bits, root, a FIFO) does not hold;
// for good. On Unix it does nothing.
func UnixOnly(tb testing.TB, why string) {
	tb.Helper()
	if isWindows {
		tb.Skip("Unix only: " + why)
	}
}

// Pending skips t on Windows until the pull request why names mends what it expects there; S315's last one
// removes Pending, and every call left with it. On Unix it does nothing.
func Pending(tb testing.TB, why string) {
	tb.Helper()
	if isWindows {
		tb.Skip("pending: " + why)
	}
}

// Mkfifo makes a FIFO at path, or skips t where there are none (Windows).
func Mkfifo(tb testing.TB, path string) {
	tb.Helper()
	UnixOnly(tb, "a FIFO")
	if err := MakeFIFO(path); err != nil {
		tb.Fatal(err)
	}
}

// MakeFIFO makes a FIFO at path, for what cannot skip a test (a goroutine, a function of an fs.FS); its test
// skips first, with UnixOnly. On Windows it fails.
func MakeFIFO(path string) error {
	return newFIFO(path)
}

// TryLock reports whether f could be locked as files.Lock locks it, without waiting; the lock goes with f.
func TryLock(tb testing.TB, f *os.File) bool {
	tb.Helper()
	return lockNow(f)
}

// Symlink makes the symlink at to old, or skips t where the system does not let this user make one (Windows,
// without the developer mode or an administrator).
func Symlink(tb testing.TB, old, at string) {
	tb.Helper()
	err := os.Symlink(old, at)
	if err != nil && privilegeNotHeld(err) {
		tb.Skip("a symlink cannot be made here: " + err.Error())
	}
	if err != nil {
		tb.Fatal(err)
	}
}

// AccessTime is the last access time of path, which os.FileInfo does not tell, for a test to set it back as it was.
func AccessTime(tb testing.TB, path string) time.Time {
	tb.Helper()
	at, err := atimeOf(path)
	if err != nil {
		tb.Fatal(err)
	}
	return at
}

// Junction makes a junction at link to the directory target, or skips t where there are none (Unix).
func Junction(tb testing.TB, link, target string) {
	tb.Helper()
	if !isWindows {
		tb.Skip("junctions are Windows'")
	}
	if err := makeJunction(link, target); err != nil {
		tb.Fatal(err)
	}
}

// CheckedOutLink skips t on Windows when path, which may be a symlink in the repository, is a regular file: one
// git checked out as the text of where it leads (Git for Windows with core.symlinks false).
func CheckedOutLink(tb testing.TB, path string) {
	tb.Helper()
	if !isWindows {
		return
	}
	if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() {
		tb.Skip(path + " was not checked out as a symlink (core.symlinks)")
	}
}
