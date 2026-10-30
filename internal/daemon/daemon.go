// Package daemon finds, starts, replaces and stops the background server, and opens pages in the
// browser.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
)

// State is what is listening on the port.
type State int

const (
	// Down is nothing.
	Down State = iota
	// Ours is a floe server.
	Ours
	// Other is another program.
	Other
)

// Status is the result of a probe; Health is set for Ours.
type Status struct {
	State  State
	Health server.Health
}

// Build identifies the running binary by version, mtime and size, so that reinstalling floe makes
// the next command that uses the server restart it.
func Build() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find the floe binary: %w", err)
	}
	fi, err := os.Stat(exe)
	if err != nil {
		return "", fmt.Errorf("find the floe binary: %w", err)
	}
	return fmt.Sprintf("%s:%d:%d", version.Version, fi.ModTime().UnixNano(), fi.Size()), nil
}

// Probe asks the port who is there.
func Probe(ctx context.Context, base string) Status {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var none server.Health
	down := Status{State: Down, Health: none}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/health", http.NoBody)
	if err != nil {
		return down
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return down
	}
	defer res.Body.Close() //nolint:errcheck // read-only
	var h server.Health
	if json.NewDecoder(res.Body).Decode(&h) != nil || h.App != server.App {
		return Status{State: Other, Health: none}
	}
	return Status{State: Ours, Health: h}
}

// waitWhile probes every 20 ms while cond holds, for 3 s in all; starting and stopping take well under
// 100 ms. The deadline bounds the probes too, so a program that accepts and never answers cannot
// stretch the wait.
func waitWhile(ctx context.Context, base string, p Status, cond func(Status) bool) Status {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for cond(p) {
		select {
		case <-ctx.Done():
			return p
		case <-time.After(20 * time.Millisecond):
		}
		p = Probe(ctx, base)
	}
	return p
}

// Server is the server a command of the store needs.
type Server struct {
	Port  int
	Root  string
	Build string
	// Templates, Parts, Themes and Icons are the directories the server must draw pages with: those of the
	// command, which a server it starts takes.
	Templates string
	Parts     string
	Themes    string
	Icons     string
	// AnyDirs takes a running server of this build that draws with other directories as it is, for a
	// command that draws nothing (listing, removing). One of another build is not replaced by it.
	AnyDirs bool
	// Env is the environment of the started server (it reads its settings from there).
	Env []string
}

func (s Server) base() string {
	return "http://" + Addr(s.Port)
}

// Addr is the address a server of floe serves on, on port.
func Addr(port int) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

// Ensure makes sure a server of this build serves the root on the port, starting one or replacing an
// outdated one as needed.
func Ensure(ctx context.Context, s Server) error {
	p := Probe(ctx, s.base())
	outdated := p.State == Ours && sameDir(p.Health.Root, s.Root) && p.Health.Build != s.Build
	// One of other directories is refused before it is replaced, which would draw others' pages with these,
	// whatever the command.
	if !s.AnyDirs || outdated {
		if err := s.otherDirs(p); err != nil {
			return err
		}
	}
	if outdated {
		var err error
		if p, err = s.replace(ctx, p); err != nil {
			return err
		}
	}
	if p.State == Down {
		var err error
		if p, err = startAndWait(ctx, s, p); err != nil {
			return err
		}
	}
	switch {
	case p.State == Other:
		return usererr.New("daemon_port_taken", s.Port)
	case p.State == Down:
		return usererr.New("daemon_not_started")
	case !sameDir(p.Health.Root, s.Root):
		return usererr.New("daemon_other_root", s.Port, p.Health.Root)
	case s.AnyDirs:
		return nil
	default:
		return s.otherDirs(p)
	}
}

// replace stops p, a server of another build, and returns what serves the port once it is gone.
func (s Server) replace(ctx context.Context, p Status) (Status, error) {
	return Stop(ctx, s.Port, p)
}

// Stop stops p, the server of floe on port, and returns what serves the port once it is gone.
func Stop(ctx context.Context, port int, p Status) (Status, error) {
	old := p.Health.PID
	if err := terminate(old); err != nil {
		return p, err
	}
	// Wait for that server only: a concurrent command may have started the new one already.
	p = waitWhile(ctx, "http://"+Addr(port), p, func(q Status) bool { return q.State == Ours && q.Health.PID == old })
	if p.State == Ours && p.Health.PID == old {
		return p, usererr.New("daemon_kept_answering", port, old)
	}
	return p, nil
}

// otherDirs refuses p, a server of floe that serves the root of s, when it draws with other directories
// of templates, parts, themes or icons than s.
func (s Server) otherDirs(p Status) error {
	if p.State != Ours || !sameDir(p.Health.Root, s.Root) || sameOrUnread(p.Health.Templates, s.Templates) &&
		sameOrUnread(p.Health.Parts, s.Parts) && sameOrUnread(p.Health.Themes, s.Themes) &&
		sameOrUnread(p.Health.Icons, s.Icons) {
		return nil
	}
	return usererr.New("daemon_other_dirs", s.Port, p.Health.Templates, p.Health.Parts, p.Health.Themes,
		p.Health.Icons, s.Templates, s.Parts, s.Themes, s.Icons)
}

// sameDir reports whether a and b name one directory: as written once cleaned, or through a symlink.
func sameDir(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	x, errA := os.Stat(a)
	y, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(x, y)
}

// sameOrUnread reports whether the directories a and b of templates or icons draw alike: one directory, or
// two that cannot be read, neither holding anything to draw with.
// LIMIT: one made readable later draws pages unchecked until the next command that draws tells the two apart; watch
// the directories if that comes to matter.
func sameOrUnread(a, b string) bool {
	if sameDir(a, b) {
		return true
	}
	_, errA := os.Stat(a)
	_, errB := os.Stat(b)
	return errA != nil && errB != nil
}

// startAndWait starts a server on the port, p being Down, and waits for one to answer there.
func startAndWait(ctx context.Context, s Server, p Status) (Status, error) {
	started := false
	// Twice at most: the port another command bound comes free, if its server fails, while this one
	// waits for it.
	for try := 0; try < 2 && p.State == Down && !started; try++ {
		var err error
		if started, err = start(ctx, s); err != nil {
			return p, err
		}
		p = waitWhile(ctx, s.base(), p, func(q Status) bool { return q.State == Down })
	}
	// Bound, but none answers: another command's server still starting, or a program that does not.
	if p.State == Down && !started {
		return p, usererr.New("daemon_port_silent", s.Port)
	}
	return p, nil
}

// ListenerEnv tells "floe serve" the file descriptor of the listener the command that started it bound.
const ListenerEnv = "FLOE_SERVE_LISTENER"

// listenerFD is the descriptor a started server takes its listener at: the first of cmd.ExtraFiles.
const listenerFD = 3

// errLowFD is a descriptor below those a command hands down.
var errLowFD = errors.New("not a descriptor handed down")

// ErrHandedAddr is a listener handed down on another address than the server's.
var ErrHandedAddr = errors.New("the listener handed down is not on the server's address")

// CannotListen is the mistake of FLOE_PORT, port, that err keeps from being bound, when another port mends
// it: in use by another program, or refused (a privileged port); any other is floe's failure. start takes
// a port in use as another server's, and tells only the others so.
func CannotListen(port int, err error) error {
	switch {
	case errors.Is(err, syscall.EADDRINUSE):
		return usererr.New("daemon_port_in_use", port)
	case errors.Is(err, syscall.EACCES):
		return usererr.New("daemon_cannot_listen", port)
	}
	return fmt.Errorf("listen on %s: %w", Addr(port), err)
}

// Listen binds Addr(port).
func Listen(port int) (*net.TCPListener, error) {
	addr, err := net.ResolveTCPAddr("tcp", Addr(port))
	if err != nil {
		return nil, err //nolint:wrapcheck // the caller says what it bound for
	}
	return net.ListenTCP("tcp", addr) //nolint:wrapcheck // as above
}

// Handed returns the listener the command that started this server bound on port, in the variables
// getenv reads, or nil when none was handed down.
func Handed(getenv func(string) string, port int) (net.Listener, error) {
	v := getenv(ListenerEnv)
	if v == "" {
		return nil, nil //nolint:nilnil // none handed down: the server binds the port itself
	}
	refuse := func(err error) error {
		return fmt.Errorf("take the listener handed down (%s=%s): %w", ListenerEnv, v, err)
	}
	fd, err := strconv.Atoi(v)
	if err != nil {
		return nil, refuse(err)
	}
	// Not standard input, output or error, which taking would close.
	if fd < listenerFD {
		return nil, refuse(errLowFD)
	}
	f := os.NewFile(uintptr(fd), "listener") //nolint:gosec // G115: a descriptor the command wrote
	defer f.Close()                          //nolint:errcheck // the listener holds its own copy
	ln, err := net.FileListener(f)
	if err != nil {
		return nil, refuse(err)
	}
	if want := Addr(port); ln.Addr().String() != want {
		_ = ln.Close() //nolint:errcheck // refused
		return nil, fmt.Errorf("%w: %s, not %s", ErrHandedAddr, ln.Addr(), want)
	}
	return ln, nil
}

// start binds the port, then runs "floe serve" on it in a session of its own, so that it outlives the
// command and its terminal (and ctx: the server must not be killed when the command ends), and
// reports whether it did. A command that cannot bind the port starts nothing: another command's
// server has it already (or another program), and a second server, slow to start, would otherwise
// take the port once that one stops.
func start(ctx context.Context, s Server) (bool, error) {
	ln, err := Listen(s.Port)
	if errors.Is(err, syscall.EADDRINUSE) {
		return false, nil
	}
	if err != nil {
		return false, CannotListen(s.Port, err)
	}
	defer ln.Close() //nolint:errcheck // the server holds its own copy
	f, err := ln.File()
	if err != nil {
		return false, fmt.Errorf("hand the listener down: %w", err)
	}
	defer f.Close() //nolint:errcheck // as above
	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("find the floe binary: %w", err)
	}
	cmd := exec.CommandContext(context.WithoutCancel(ctx), exe, "serve") //nolint:gosec // G204: our own binary
	cmd.Env = append(append([]string{}, s.Env...), ListenerEnv+"="+strconv.Itoa(listenerFD))
	cmd.ExtraFiles = []*os.File{f}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start the server: %w", err)
	}
	go func() { _ = cmd.Wait() }() //nolint:errcheck // reaps the server if it exits before we do
	return true, nil
}

// terminate sends SIGTERM to pid. A process that has exited already counts as stopped.
func terminate(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("stop the server (pid %d): %w", pid, err)
	}
	return nil
}

// OpenBrowser opens url with $BROWSER (devcontainers point it at the host's browser), macOS's open,
// wslview or xdg-open. Failing to open is ignored: the publish succeeded and the URL is printed.
// Linux's /usr/bin/open is openvt, so open is used on macOS only.
// LIMIT: $BROWSER is a single command (a colon-separated list is not supported).
func OpenBrowser(ctx context.Context, browser, url string) {
	if browser == "" {
		browser = opener()
	}
	if browser == "" {
		return
	}
	//nolint:gosec // G204: the user's browser
	cmd := exec.CommandContext(context.WithoutCancel(ctx), browser, url)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() != nil {
		return
	}
	go func() { _ = cmd.Wait() }() //nolint:errcheck // reaps the opener
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
