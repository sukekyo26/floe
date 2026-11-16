//go:build unix

package testfs

import (
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const isWindows = false

func mkfifo(path string) error {
	return syscall.Mkfifo(path, 0o600) //nolint:wrapcheck // the test's own
}

func tryLock(f *os.File) bool {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil //nolint:gosec // G115: a descriptor fits an int
}

func privilegeNotHeld(error) bool { return false }

var errNoJunction = errors.New("junctions are Windows'")

func junction(string, string) error { return errNoJunction }

func accessTime(path string) (time.Time, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return time.Time{}, err //nolint:wrapcheck // the test's own
	}
	return time.Unix(st.Atim.Unix()), nil
}
