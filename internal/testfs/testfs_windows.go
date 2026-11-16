package testfs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// aclDeny denies this user, as System32's whoami names it (DOMAIN\user; Git's whoami names the user alone), the
// listing and reading of dir with icacls, and returns what allows it again.
func aclDeny(dir string) (func() error, error) {
	//nolint:gosec,noctx // G204: System32's own; it returns at once
	who, err := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "whoami.exe")).Output()
	user := strings.TrimSpace(string(who))
	if err != nil || user == "" {
		return nil, fmt.Errorf("whoami: %w %q", err, user)
	}
	//nolint:gosec,noctx // G204: the test's directory; icacls returns at once
	if out, err := exec.Command("icacls", dir, "/deny", user+":(RX)").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("icacls %s /deny: %w: %s", dir, err, strings.TrimSpace(string(out)))
	}
	return func() error {
		//nolint:gosec,noctx // G204: as above
		if out, err := exec.Command("icacls", dir, "/remove:d", user).CombinedOutput(); err != nil {
			return fmt.Errorf("%w: %s; run  icacls \"%s\" /remove:d \"%s\"", err, strings.TrimSpace(string(out)), dir, user)
		}
		return nil
	}, nil
}

// makeDrive makes a drive of dir with subst, from Z down to M, and returns it and what removes it; "" when no letter is
// free.
func makeDrive(dir string) (string, func() error, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return "", nil, fmt.Errorf("GetLogicalDrives: %w", err)
	}
	for l := 'Z'; l >= 'M'; l-- {
		if mask&(1<<uint(l-'A')) != 0 {
			continue
		}
		drive := string(l) + ":"
		//nolint:gosec,noctx // G204: the test's directory; subst returns at once
		if out, err := exec.Command("subst", drive, dir).CombinedOutput(); err != nil {
			return "", nil, fmt.Errorf("subst %s %s: %w: %s", drive, dir, err, strings.TrimSpace(string(out)))
		}
		return drive, func() error {
			//nolint:gosec,noctx // G204: as above
			if out, err := exec.Command("subst", drive, "/d").CombinedOutput(); err != nil {
				return fmt.Errorf("%w: %s; run  subst %s /d", err, strings.TrimSpace(string(out)), drive)
			}
			return nil
		}, nil
	}
	return "", nil, nil
}
