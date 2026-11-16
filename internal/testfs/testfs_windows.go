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

var errNoAccessTime = errors.New("no Win32FileAttributeData")

func newFIFO(string) error { return ErrNoFIFO }

func lockNow(f *os.File) bool {
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, ^uint32(0), ^uint32(0), new(windows.Overlapped)) == nil
}

func privilegeNotHeld(err error) bool { return errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) }

// junction makes a junction with mklink /J, which needs no privilege: neither Go nor x/sys makes one (Go's own
// tests use mklink too).
func makeJunction(link, target string) error {
	//nolint:gosec,noctx // G204: the test's paths; mklink returns at once
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mklink /J %s %s: %w: %s", link, target, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func atimeOf(path string) (time.Time, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err //nolint:wrapcheck // the test's own
	}
	a, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, errNoAccessTime
	}
	return time.Unix(0, a.LastAccessTime.Nanoseconds()), nil
}
