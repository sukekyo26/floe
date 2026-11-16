package testfs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
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

// freeDrives are the letters no volume has, from Z down to M.
func freeDrives() ([]string, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, fmt.Errorf("GetLogicalDrives: %w", err)
	}
	var free []string
	for l := 'Z'; l >= 'M'; l-- {
		if mask&(1<<uint(l-'A')) == 0 {
			free = append(free, string(l)+":")
		}
	}
	return free, nil
}

// command runs a command of the system that returns at once, its output in its error.
func command(name string, args ...string) error {
	//nolint:gosec,noctx // G204: the system's own, on the test's directories; it returns at once
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func substDrive(drive, dir string) error { return command("subst", drive, dir) }

func unsubstDrive(drive string) error {
	if err := command("subst", drive, "/d"); err != nil {
		return fmt.Errorf("%w; run  subst %s /d", err, drive)
	}
	return nil
}

// mapDrive maps drive to share for this logon only (/persistent:no): a test that stops before its cleanup leaves
// nothing in the user's settings for the next logon.
func mapDrive(drive, share string) error {
	return command("net", "use", drive, share, "/persistent:no")
}

func unmapDrive(drive string) error {
	if err := command("net", "use", drive, "/delete", "/y"); err != nil {
		return fmt.Errorf("%w; run  net use %s /delete", err, drive)
	}
	return nil
}

// hideAttr adds the hidden attribute to what is at path.
func hideAttr(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	a, err := windows.GetFileAttributes(p)
	if err != nil {
		return fmt.Errorf("GetFileAttributes %s: %w", path, err)
	}
	if err := windows.SetFileAttributes(p, a|windows.FILE_ATTRIBUTE_HIDDEN); err != nil {
		return fmt.Errorf("SetFileAttributes %s: %w", path, err)
	}
	return nil
}

func shortNameOf(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err //nolint:wrapcheck // the test's failure
	}
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf))) //nolint:gosec // G115: a buffer's length
		if err != nil {
			return "", err //nolint:wrapcheck // as above
		}
		if int(n) < len(buf) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}

// servePipe serves a named pipe of its own name, counting each client that connects and letting it go at once, until
// stop.
func servePipe() (name string, connected func() int, stop func(), err error) {
	name = `\\.\pipe\floe-test-` + strconv.FormatInt(time.Now().UnixNano(), 36)
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", nil, nil, fmt.Errorf("the pipe %s: %w", name, err)
	}
	var count atomic.Int64
	var stopped atomic.Bool
	instance := func() (windows.Handle, error) {
		return windows.CreateNamedPipe(p, windows.PIPE_ACCESS_DUPLEX, windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
			windows.PIPE_UNLIMITED_INSTANCES, 512, 512, 0, nil)
	}
	h, err := instance()
	if err != nil {
		return "", nil, nil, fmt.Errorf("serve the pipe %s: %w", name, err)
	}
	go func() {
		for {
			err := windows.ConnectNamedPipe(h, nil)
			if stopped.Load() {
				_ = windows.CloseHandle(h) //nolint:errcheck // done with
				return
			}
			if err == nil || errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
				count.Add(1)
			}
			_ = windows.DisconnectNamedPipe(h) //nolint:errcheck // the client is let go
			_ = windows.CloseHandle(h)         //nolint:errcheck // a new instance is made
			if h, err = instance(); err != nil {
				return
			}
		}
	}()
	stop = func() {
		stopped.Store(true)
		// A client of its own lets the waiting ConnectNamedPipe return.
		if f, err := os.Open(name); err == nil {
			_ = f.Close()
		}
	}
	// The count is read after the connect is counted: a client that got through is counted before it returns.
	return name, func() int { time.Sleep(50 * time.Millisecond); return int(count.Load()) }, stop, nil
}

// volumeName is the GUID name of the volume mounted at root (C:\), \\?\Volume{…}\.
func volumeName(root string) (string, error) {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return "", fmt.Errorf("the volume of %s: %w", root, err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	if err := windows.GetVolumeNameForVolumeMountPoint(p, &buf[0], windows.MAX_PATH); err != nil {
		return "", fmt.Errorf("the volume of %s: %w", root, err)
	}
	return windows.UTF16ToString(buf), nil
}
