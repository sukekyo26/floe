package daemon

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

// errInUse and errDenied are the system's errors of a port in use and of one refused (WSAEACCES: a port another
// holds for itself, or one the system reserves).
var (
	errInUse  error = windows.WSAEADDRINUSE
	errDenied error = syscall.WSAEACCES
)

// background makes cmd start away from this command, with a console of its own that no window shows
// (CREATE_NO_WINDOW: DETACHED_PROCESS would leave it none, and each console program it starts, git, would make
// one), so that it outlives this command and its terminal, and in a process group of its own, which a Ctrl+C at
// the terminal does not reach; when f is not nil, it hands f's socket down to cmd: made inheritable, and named
// to os/exec, which hands down only the handles it is named (PROC_THREAD_ATTRIBUTE_HANDLE_LIST). It returns the
// handle's number, for ListenerEnv. The handle is f.Fd()'s, which takes it out of the I/O completion port.
// LIMIT: no test sees the creation flags (the child cannot tell them); CI passes without them. A real machine tried
// them (S315's W18, W19, W22): try it again on one when they change.
func background(cmd *exec.Cmd, f *os.File) (string, error) {
	attr := &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	cmd.SysProcAttr = attr
	if f == nil {
		return "", nil
	}
	h := windows.Handle(f.Fd())
	if err := windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		return "", err //nolint:wrapcheck // start says what it handed down
	}
	attr.AdditionalInheritedHandles = []syscall.Handle{syscall.Handle(h)}
	return strconv.FormatUint(uint64(h), 10), nil
}

// takeHanded is the file of the handle n a command handed down.
func takeHanded(n uint64) (*os.File, error) {
	return os.NewFile(uintptr(n), "listener"), nil
}

// handed keeps the files release closed the sockets of, for as long as the process runs.
// LIMIT: os.NewFile makes a file that closes with CloseHandle, and so does its finalizer, where a socket needs
// closesocket (net/fd_windows.go uses a hidden constructor for that). Each is kept from the finalizer, which would
// close the number again once another handle may have it; one per server, so nothing piles up.
var handed struct {
	sync.Mutex
	files []*os.File
}

// release closes the socket of f, which takeHanded gave, once the listener holds its own copy: with closesocket,
// f kept first so that its finalizer never runs. A handle that is no socket is closed as a file.
func release(f *os.File) {
	handed.Lock()
	handed.files = append(handed.files, f)
	handed.Unlock()
	if err := windows.Closesocket(windows.Handle(f.Fd())); errors.Is(err, windows.WSAENOTSOCK) {
		handed.Lock()
		handed.files = slices.DeleteFunc(handed.files, func(g *os.File) bool { return g == f })
		handed.Unlock()
		_ = f.Close() //nolint:errcheck // no socket to keep
	}
}

// openDefault opens url with the default browser (ShellExecute): cmd /c start would take the & in a URL as its own.
func openDefault(_ context.Context, url string) {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return
	}
	file, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return
	}
	_ = windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL) //nolint:errcheck // the URL is printed
}
