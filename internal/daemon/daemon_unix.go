//go:build unix

package daemon

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
)

// errInUse and errDenied are the system's errors of a port in use and of one refused (a privileged port).
var (
	errInUse  error = syscall.EADDRINUSE
	errDenied error = syscall.EACCES
)

// listenerFD is the descriptor a started server takes its listener at: the first of cmd.ExtraFiles.
const listenerFD = 3

// errLowFD is a descriptor below those a command hands down.
var errLowFD = errors.New("not a descriptor handed down")

// background makes cmd start in a session of its own, so that it outlives this command and its terminal, and, when
// f is not nil, hands f down to it: it returns what tells cmd where f is, for ListenerEnv.
func background(cmd *exec.Cmd, f *os.File) (string, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if f == nil {
		return "", nil
	}
	cmd.ExtraFiles = []*os.File{f}
	return strconv.Itoa(listenerFD), nil
}

// takeHanded is the file of the descriptor n a command handed down, not standard input, output or error, which
// taking would close.
func takeHanded(n uint64) (*os.File, error) {
	if n < listenerFD {
		return nil, errLowFD
	}
	return os.NewFile(uintptr(n), "listener"), nil
}

// release closes f, which takeHanded gave, once the listener holds its own copy.
func release(f *os.File) {
	_ = f.Close() //nolint:errcheck // the listener holds its own copy
}

// openDefault opens url with macOS's open, wslview or xdg-open. Linux's /usr/bin/open is openvt, so open is used
// on macOS only.
func openDefault(ctx context.Context, url string) {
	browser := opener()
	if browser == "" {
		return
	}
	//nolint:gosec // G204: the system's opener
	startAway(exec.CommandContext(context.WithoutCancel(ctx), browser, url))
}

func opener() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	for _, name := range []string{"wslview", "xdg-open"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}
