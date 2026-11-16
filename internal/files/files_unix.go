//go:build unix

package files

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	// openFlags keep an open from blocking on a FIFO or making a terminal the process's.
	openFlags = syscall.O_NONBLOCK | syscall.O_NOCTTY
	// noFollow keeps a file made anew from being made where a link to nowhere leads.
	noFollow = syscall.O_NOFOLLOW
	// checksPaths: allowed looks at nothing; Unix has no share nor device by a path's letters, and a FIFO or a
	// device is told by its kind.
	checksPaths = false
)

// driveType is no kind of drive: Unix has none, and allowed does not ask.
func driveType(string) uint32 { return driveUnknown }

// Lock locks f, a file of its own, for this process alone, waiting while another holds it, and returns what closes
// f, which lets the lock go. The system's error is returned as it is, for the caller to tell at the file.
func Lock(f *os.File) (func(), error) {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil { //nolint:gosec // G115: a descriptor fits an int
		return nil, err //nolint:wrapcheck // the system's words, for the caller to tell
	}
	return func() { _ = f.Close() }, nil //nolint:errcheck // closing lets the lock go
}

// stampAt is the stamp of the file at path, as lstat tells it, or stat when follow (what a symlink leads to):
// its kind, inode, size, mtime and ctime. ctime and the inode tell what a mtime set back (touch -r, cp -p) or a
// file replaced by a rename would hide. It tells whether the file is a symlink, and false for none there.
// LIMIT: a filesystem of times in whole seconds (FAT, some SMB or NFS) hides two writes of one size in one second
// once the first was marked. Add a hash of the content if pages come to be kept on one.
func stampAt(path checked, follow bool) (part string, link, ok bool) {
	var st unix.Stat_t
	stat := unix.Lstat
	if follow {
		stat = unix.Stat
	}
	if stat(path.s, &st) != nil {
		return "", false, false
	}
	return stamp(&st), st.Mode&unix.S_IFMT == unix.S_IFLNK, true
}

// stamp is st's part of a mark. A directory is its kind and inode alone: what is in it is marked by the walk,
// and its times change with every file made in it, a hidden one (an editor's .swp) too.
func stamp(st *unix.Stat_t) string {
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		return fmt.Sprintf("%o:%d", st.Mode, st.Ino)
	}
	return fmt.Sprintf("%o:%d:%d:%d:%d", st.Mode, st.Ino, st.Size, st.Mtim.Nano(), st.Ctim.Nano())
}

// realOf is path with its symlinks followed.
func realOf(path checked) (string, error) {
	return filepath.EvalSymlinks(path.s) //nolint:wrapcheck // the system's words, for the caller to tell
}

// longNameOf is path as written: there are no short names to make long.
func longNameOf(path checked) (string, error) { return path.s, nil }
