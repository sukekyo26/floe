// Package daemon finds, starts, replaces and stops the background server, and opens pages in the
// browser.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
// the next publish restart the server.
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

// Server is the server a publish needs.
type Server struct {
	Port  int
	Root  string
	Build string
	// Env is the environment of the started server (it reads its settings from there).
	Env []string
}

func (s Server) base() string {
	return "http://127.0.0.1:" + strconv.Itoa(s.Port)
}

// Ensure makes sure a server of this build serves the root on the port, starting one or replacing an
// outdated one as needed.
func Ensure(ctx context.Context, s Server) error {
	p := Probe(ctx, s.base())
	if p.State == Ours && p.Health.Root == s.Root && p.Health.Build != s.Build {
		old := p.Health.PID
		if err := Terminate(old); err != nil {
			return err
		}
		// Wait for that server only: a concurrent publish may have started the new one already.
		p = waitWhile(ctx, s.base(), p, func(q Status) bool { return q.State == Ours && q.Health.PID == old })
		if p.State == Ours && p.Health.PID == old {
			return usererr.New("daemon_outdated_alive", old)
		}
	}
	if p.State == Down {
		if err := start(ctx, s.Env); err != nil {
			return err
		}
		p = waitWhile(ctx, s.base(), p, func(q Status) bool { return q.State == Down })
	}
	switch {
	case p.State == Other:
		return usererr.New("daemon_port_taken", s.Port)
	case p.State == Down:
		return usererr.New("daemon_not_started")
	case p.Health.Root != s.Root:
		return usererr.New("daemon_other_root", s.Port, p.Health.Root)
	default:
		return nil
	}
}

// start runs "floe serve" in a session of its own, so that it outlives the publish and its terminal
// (and ctx: the server must not be killed when the publish ends).
func start(ctx context.Context, env []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find the floe binary: %w", err)
	}
	cmd := exec.CommandContext(context.WithoutCancel(ctx), exe, "serve") //nolint:gosec // G204: our own binary
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start the server: %w", err)
	}
	go func() { _ = cmd.Wait() }() //nolint:errcheck // reaps the server if it exits before we do
	return nil
}

// Terminate sends SIGTERM to pid. A process that has exited already counts as stopped.
func Terminate(pid int) error {
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
