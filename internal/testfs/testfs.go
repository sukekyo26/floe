// Package testfs holds what tests make that differs by system (a FIFO, a lock tried without waiting, a symlink, a
// junction), and the marks of the tests that do not run on Windows: UnixOnly, of what holds on Unix alone, and
// Pending, of what S315's later pull requests mend. Only tests import it.
package testfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
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

// Words is the system's words in err, an error of a path (*fs.PathError, *os.LinkError), for a test to expect them
// where floe tells them, as each system says them; it fails tb for another error.
func Words(tb testing.TB, err error) string {
	tb.Helper()
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		return pe.Err.Error()
	}
	if le, ok := errors.AsType[*os.LinkError](err); ok {
		return le.Err.Error()
	}
	tb.Fatalf("%v is no error of a path", err)
	return ""
}

// Mkfifo makes a FIFO at path, or skips t where there are none (Windows).
func Mkfifo(tb testing.TB, path string) {
	tb.Helper()
	UnixOnly(tb, "a FIFO")
	if err := MakeFIFO(path); err != nil {
		tb.Fatal(err)
	}
}

// ErrNoFIFO is MakeFIFO's error where there are no FIFOs (Windows), told apart from a failure to make one.
var ErrNoFIFO = errors.New("there are no FIFOs on Windows")

// MakeFIFO makes a FIFO at path, for what cannot skip a test (a goroutine, a function of an fs.FS); its test
// skips first, with UnixOnly. On Windows it fails with ErrNoFIFO.
func MakeFIFO(path string) error {
	return newFIFO(path)
}

// TryLock reports whether f could be locked as files.Lock locks it, without waiting; the lock goes with f.
func TryLock(tb testing.TB, f *os.File) bool {
	tb.Helper()
	return lockNow(f)
}

// Symlink makes the symlink at to old, or skips t where the system does not let this user make one (Windows,
// without the developer mode or an administrator); on the CI (CI set) that fails t, so that the tests of links are
// known to run there.
func Symlink(tb testing.TB, old, at string) {
	tb.Helper()
	err := os.Symlink(old, at)
	if err != nil && privilegeNotHeld(err) && os.Getenv("CI") == "" {
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

// ShortName is the 8.3 name of path (Windows), which may begin with \\?\ to be over MAX_PATH. It skips t on Unix,
// which has none, and where the volume makes none; on the CI (CI set) such a volume fails t instead, so that the
// tests of short names are known to run there.
func ShortName(tb testing.TB, path string) string {
	tb.Helper()
	if !isWindows {
		tb.Skip("short names are Windows'")
	}
	short, err := shortNameOf(path)
	if err != nil {
		tb.Fatal(err)
	}
	if filepath.Base(short) == filepath.Base(path) {
		if os.Getenv("CI") != "" {
			tb.Fatalf("%s has no short name on the CI", path)
		}
		tb.Skip("this volume makes no short names")
	}
	return short
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

// DenyList denies this user the listing and reading of dir (Windows' ACL, with icacls), and allows it again when tb
// ends, before a t.TempDir made earlier is removed. It skips where the system lets this user list dir all the same
// (an administrator, as on the CI's runner), and on Unix, where chmod does it (tests of that are UnixOnly).
func DenyList(tb testing.TB, dir string) {
	tb.Helper()
	if !isWindows {
		tb.Skip("denying a directory by its ACL is Windows'")
	}
	undo, err := aclDeny(dir)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := undo(); err != nil {
			tb.Logf("%s is left denied: %v", dir, err)
		}
	})
	if _, err := os.ReadDir(dir); err == nil {
		tb.Skip("the system lets this user list a directory denied to it (an administrator)")
	}
}

// Subst makes a drive of dir with subst (Windows), a letter no volume has, and removes it when tb ends; it returns
// the drive (Z:). It skips where no letter is free (on the CI it fails), and on Unix, which has no drives.
func Subst(tb testing.TB, dir string) string {
	tb.Helper()
	if !isWindows {
		tb.Skip("drives are Windows'")
	}
	return newDrive(tb, func(drive string) error { return substDrive(drive, dir) }, unsubstDrive)
}

// MapShare gives dir (C:\…) as a path through this machine's administrative share of its drive
// (\\127.0.0.1\C$\…) and through a network drive mapped to that share with net use (Y:\…), removed when tb ends.
// It skips on Unix and where the share cannot be reached (not an administrator); on the CI (CI set) that fails tb.
func MapShare(tb testing.TB, dir string) (shared, mapped string) {
	tb.Helper()
	if !isWindows {
		tb.Skip("shares are Windows'")
	}
	root, rest := shareOf(tb, dir)
	drive := newDrive(tb, func(drive string) error { return mapDrive(drive, root) }, unmapDrive)
	return root + rest, drive + rest
}

// SubstShare makes a drive of dir through this machine's administrative share of its drive (subst Q:
// \\127.0.0.1\C$\…), which the system tells a network drive, removed when tb ends; it returns the drive (Q:). It skips
// as MapShare does.
func SubstShare(tb testing.TB, dir string) string {
	tb.Helper()
	if !isWindows {
		tb.Skip("shares are Windows'")
	}
	root, rest := shareOf(tb, dir)
	return newDrive(tb, func(drive string) error { return substDrive(drive, root+rest) }, unsubstDrive)
}

// shareOf is the administrative share of dir's drive (\\127.0.0.1\C$) and the rest of dir after its drive. It skips
// where the share cannot be reached (not an administrator); on the CI (CI set) that fails tb.
func shareOf(tb testing.TB, dir string) (root, rest string) {
	tb.Helper()
	vol := filepath.VolumeName(dir)
	if len(vol) != 2 || vol[1] != ':' {
		tb.Fatalf("%s is not on a drive's letter", dir)
	}
	root = `\\127.0.0.1\` + vol[:1] + "$"
	if _, err := os.Stat(root + `\`); err != nil {
		if os.Getenv("CI") != "" {
			tb.Fatalf("the share %s cannot be reached on the CI: %v", root, err)
		}
		tb.Skip("the share " + root + " cannot be reached: " + err.Error())
	}
	return root, dir[len(vol):]
}

// PipeServer serves a named pipe (Windows) for the test, counting each client that connects, and returns its name
// (\\.\pipe\…) and what tells how many have connected so far. It stops when tb ends. It skips on Unix, which has no
// pipes by name; on the CI (CI set) one that cannot be served fails tb.
func PipeServer(tb testing.TB) (name string, connected func() int) {
	tb.Helper()
	if !isWindows {
		tb.Skip("named pipes are Windows'")
	}
	name, count, stop, err := servePipe()
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(stop)
	return name, func() int {
		n, err := count()
		if err != nil {
			tb.Fatal(err)
		}
		return n
	}
}

// VolumeGUID is dir (C:\…) through its volume's GUID name (\\?\Volume{…}\…), which has no drive letter (Windows). It
// skips on Unix.
func VolumeGUID(tb testing.TB, dir string) string {
	tb.Helper()
	if !isWindows {
		tb.Skip("volume names are Windows'")
	}
	vol, err := volumeName(filepath.VolumeName(dir) + `\`)
	if err != nil {
		tb.Fatal(err)
	}
	return vol + dir[len(`C:\`):]
}

// FreeDrive is a drive letter no volume has (Q:), not taken for the test: one another test or process makes
// meanwhile is not looked at. It skips on Unix and where no letter is free (on the CI, CI set, it fails).
func FreeDrive(tb testing.TB) string {
	tb.Helper()
	if !isWindows {
		tb.Skip("drives are Windows'")
	}
	letters, err := freeDrives()
	if err != nil {
		tb.Fatal(err)
	}
	if len(letters) == 0 {
		if os.Getenv("CI") != "" {
			tb.Fatal("no drive letter is free on the CI")
		}
		tb.Skip("no drive letter is free")
	}
	return letters[len(letters)-1]
}

// driveMu keeps this process's tests from taking one letter at once.
var driveMu sync.Mutex //nolint:gochecknoglobals // one for the process

// newDrive makes a drive with mk (subst, net use) on a letter no volume has, the next one when mk fails (another
// process took the letter meanwhile), and removes it with undo when tb ends; it skips where no letter is free (on
// the CI, CI set, it fails instead).
func newDrive(tb testing.TB, mk, undo func(drive string) error) string {
	tb.Helper()
	driveMu.Lock()
	defer driveMu.Unlock()
	letters, err := freeDrives()
	if err != nil {
		tb.Fatal(err)
	}
	var failed error
	for _, drive := range letters {
		if failed = mk(drive); failed != nil {
			continue
		}
		tb.Cleanup(func() {
			if err := undo(drive); err != nil {
				tb.Logf("drive %s is left: %v", drive, err)
			}
		})
		return drive
	}
	if failed != nil {
		tb.Fatal(failed)
	}
	if os.Getenv("CI") != "" {
		tb.Fatal("no drive letter is free on the CI")
	}
	tb.Skip("no drive letter is free")
	return ""
}

// Hide gives the file or directory at path the hidden attribute (Windows). It skips on Unix, where a dot is all.
func Hide(tb testing.TB, path string) {
	tb.Helper()
	if !isWindows {
		tb.Skip("hidden attributes are Windows'")
	}
	if err := hideAttr(path); err != nil {
		tb.Fatal(err)
	}
}

// NotUTF8 makes, in parent, an empty file (or a directory, when dir) named name, which must not be UTF-8, and returns
// its path as named (Windows makes the name with U+FFFD for each byte it cannot read). It skips where the system
// refuses such a name (darwin's APFS); on the CI (CI set) that fails tb, so that the tests of such names are known
// to run there.
func NotUTF8(tb testing.TB, parent, name string, dir bool) string {
	tb.Helper()
	if utf8.ValidString(name) {
		tb.Fatalf("%q is UTF-8", name)
	}
	path := filepath.Join(parent, name)
	var err error
	if dir {
		err = os.Mkdir(path, 0o700)
	} else {
		err = os.WriteFile(path, nil, 0o600)
	}
	if err != nil {
		if os.Getenv("CI") != "" {
			tb.Fatalf("a name not UTF-8 cannot be made on the CI: %v", err)
		}
		tb.Skip("this system makes no name that is not UTF-8: " + err.Error())
	}
	return path
}
