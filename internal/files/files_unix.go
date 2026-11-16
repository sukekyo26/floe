//go:build unix

package files

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	// openFlags keep an open from blocking on a FIFO or making a terminal the process's.
	openFlags = syscall.O_NONBLOCK | syscall.O_NOCTTY
	// noFollow keeps a file made anew from being made where a link to nowhere leads.
	noFollow = syscall.O_NOFOLLOW
)

// Lock locks f, a file of its own, for this process alone, waiting while another holds it, and returns what closes
// f, which lets the lock go. The system's error is returned as it is, for the caller to tell at the file.
func Lock(f *os.File) (func(), error) {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil { //nolint:gosec // G115: a descriptor fits an int
		return nil, err //nolint:wrapcheck // the system's words, for the caller to tell
	}
	return func() { _ = f.Close() }, nil //nolint:errcheck // closing lets the lock go
}

// Mark is what the file at path is now, to tell when it changes: its kind, inode, size, mtime and ctime, as
// lstat tells them, and as stat tells them for a symlink (what it leads to); "-" for none. ctime and the inode
// tell what a mtime set back (touch -r, cp -p) or a file replaced by a rename would hide.
// LIMIT: a filesystem of times in whole seconds (FAT, some SMB or NFS) hides two writes of one size in one second
// once the first was marked. Add a hash of the content if pages come to be kept on one.
func Mark(path string) string {
	var st unix.Stat_t
	if unix.Lstat(path, &st) != nil {
		return "-"
	}
	m := stamp(&st)
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		if unix.Stat(path, &st) != nil {
			return m + ">-"
		}
		m += ">" + stamp(&st)
	}
	return m
}

// stamp is st's part of a mark. A directory is its kind and inode alone: what is in it is marked by the walk,
// and its times change with every file made in it, a hidden one (an editor's .swp) too.
func stamp(st *unix.Stat_t) string {
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		return fmt.Sprintf("%o:%d", st.Mode, st.Ino)
	}
	return fmt.Sprintf("%o:%d:%d:%d:%d", st.Mode, st.Ino, st.Size, st.Mtim.Nano(), st.Ctim.Nano())
}
