package testfs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

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

// pipeServer serves a named pipe, counting each client that connects and letting it go, until stopped. One
// instance waits at a time: the next is made once a client took the last, and until then the pipe turns a client
// away (ERROR_PIPE_BUSY). Its own clients wait that out (dial). The marker is let go after the next instance is
// made, so what connects right after connected returns is not turned away.
// LIMIT: one instance waits, so a client not the server's own (the code under test, a Stat) that comes before the
// next is made is turned away and not counted: of two right after each other, the second may be. Keep more
// instances waiting if a test comes to count clients that connect in a row.
type pipeServer struct {
	path    string
	name    *uint16
	count   atomic.Int64
	stopped atomic.Bool
	marked  chan struct{}
}

// pipeBusyWait is how long dial waits for the server to make its next instance: far longer than it takes.
const pipeBusyWait = 10 * time.Second

// dial opens the pipe as a client of the server's own, trying again while no instance is free.
func (s *pipeServer) dial(flag int) (*os.File, error) {
	for end := time.Now().Add(pipeBusyWait); ; time.Sleep(time.Millisecond) {
		f, err := os.OpenFile(s.path, flag, 0)
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) || time.Now().After(end) {
			return f, err //nolint:wrapcheck // told by the caller
		}
	}
}

func (s *pipeServer) instance() (windows.Handle, error) {
	return windows.CreateNamedPipe( //nolint:wrapcheck // told by the caller
		s.name, windows.PIPE_ACCESS_DUPLEX, windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES, 512, 512, 0, nil)
}

// serve waits on h for a client, then on a new instance, until stopped.
func (s *pipeServer) serve(h windows.Handle) {
	for {
		err := windows.ConnectNamedPipe(h, nil)
		if s.stopped.Load() {
			_ = windows.CloseHandle(h) //nolint:errcheck // done with
			return
		}
		// ERROR_NO_DATA: a client that connected to the instance before it was waited on, and is gone.
		if err == nil || errors.Is(err, windows.ERROR_PIPE_CONNECTED) || errors.Is(err, windows.ERROR_NO_DATA) {
			s.count.Add(1)
		}
		next, err := s.instance()
		go s.handle(h)
		if err != nil {
			return
		}
		h = next
	}
}

// handle lets the client of h go; a marker, which writes m, is taken off the count and told.
func (s *pipeServer) handle(h windows.Handle) {
	var b [1]byte
	var n uint32
	if windows.ReadFile(h, b[:], &n, nil) == nil && n == 1 && b[0] == 'm' {
		s.count.Add(-1)
		s.marked <- struct{}{}
	}
	_ = windows.DisconnectNamedPipe(h) //nolint:errcheck // the client is let go
	_ = windows.CloseHandle(h)         //nolint:errcheck // done with
}

// newPipeServer makes a pipe server of a name of its own and its first instance, for serve to wait on.
func newPipeServer() (*pipeServer, windows.Handle, error) {
	path := `\\.\pipe\floe-test-` + rand.Text()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, 0, fmt.Errorf("the pipe %s: %w", path, err)
	}
	s := &pipeServer{path: path, name: p, count: atomic.Int64{}, stopped: atomic.Bool{}, marked: make(chan struct{})}
	h, err := s.instance()
	if err != nil {
		return nil, 0, fmt.Errorf("serve the pipe %s: %w", path, err)
	}
	return s, h, nil
}

// stop ends serve.
func (s *pipeServer) stop() {
	s.stopped.Store(true)
	// A client of its own lets the waiting ConnectNamedPipe return.
	if f, err := s.dial(os.O_RDONLY); err == nil {
		_ = f.Close()
	}
}

// connected is how many clients have connected so far. It marks its place with a client of its own: once that is let
// go, each client before it has been counted (each was counted before the marker's instance was made), the marker
// not.
func (s *pipeServer) connected() (int, error) {
	f, err := s.dial(os.O_WRONLY)
	if err != nil {
		return 0, fmt.Errorf("mark the pipe %s: %w", s.path, err)
	}
	defer f.Close() //nolint:errcheck // written to already
	if _, err := f.Write([]byte{'m'}); err != nil {
		return 0, fmt.Errorf("mark the pipe %s: %w", s.path, err)
	}
	<-s.marked
	return int(s.count.Load()), nil
}

// servePipe serves a named pipe of its own name until stop.
func servePipe() (name string, connected func() (int, error), stop func(), err error) {
	s, h, err := newPipeServer()
	if err != nil {
		return "", nil, nil, err
	}
	go s.serve(h)
	return s.path, s.connected, s.stop, nil
}

// replaceAsEditor saves replacement over replaced by ReplaceFile, as an editor does.
func replaceAsEditor(replaced, replacement string) error {
	to, err := windows.UTF16PtrFromString(replaced)
	if err != nil {
		return err //nolint:wrapcheck // the caller's to tell
	}
	from, err := windows.UTF16PtrFromString(replacement)
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")
	if r, _, err := proc.Call(uintptr(unsafe.Pointer(to)), uintptr(unsafe.Pointer(from)), 0, 0, 0, 0); r == 0 {
		return fmt.Errorf("ReplaceFile: %w", err)
	}
	return nil
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
