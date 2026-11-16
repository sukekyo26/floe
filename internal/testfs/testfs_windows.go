package testfs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const isWindows = true

var errNoFIFO = errors.New("there are no FIFOs on Windows")

func mkfifo(string) error { return errNoFIFO }

func tryLock(f *os.File) bool {
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, ^uint32(0), ^uint32(0), new(windows.Overlapped)) == nil
}

func privilegeNotHeld(err error) bool { return errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) }

// junction makes a junction with mklink /J, which needs no privilege: neither Go nor x/sys makes one (Go's own
// tests use mklink too).
func junction(link, target string) error {
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput() //nolint:gosec // G204: the test's paths
	if err != nil {
		return fmt.Errorf("mklink /J %s %s: %w: %s", link, target, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func accessTime(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err //nolint:wrapcheck // the test's own
	}
	return time.Unix(0, fi.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds()), nil
}
