package daemon_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"floe/internal/daemon"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

// The child's variables: what it does, the port it takes a listener on, and the file it tells of it in.
const (
	childEnv  = "FLOE_DAEMON_TEST_CHILD"
	portEnv   = "FLOE_DAEMON_TEST_PORT"
	reportEnv = "FLOE_DAEMON_TEST_REPORT"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(childEnv); mode != "" {
		child(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// child is this test binary started by a test: "take" takes the listener handed down as floe serve does, answers
// one connection, lets it go and binds its port again; "away" tells only whether it was started away. It tells
// each step in the report file, a line each, and "done" last.
func child(mode string) {
	f, err := os.OpenFile(os.Getenv(reportEnv), os.O_WRONLY|os.O_APPEND, 0) //nolint:gosec // G304,G703: the test's own
	if err != nil {
		os.Exit(3)
	}
	defer f.Close()
	say := func(format string, a ...any) { _, _ = fmt.Fprintf(f, format+"\n", a...) }
	defer say("done")
	say("away %s", awayNote())
	if mode != "take" {
		return
	}
	port, err := strconv.Atoi(os.Getenv(portEnv))
	if err != nil {
		say("port %v", err)
		return
	}
	ln, err := daemon.Handed(os.Getenv, port)
	if err != nil {
		say("handed %s", errKind(err))
		return
	}
	say("handed ok")
	if tl, ok := ln.(*net.TCPListener); ok {
		_ = tl.SetDeadline(time.Now().Add(20 * time.Second)) //nolint:errcheck // Accept tells
	}
	conn, err := ln.Accept()
	if err != nil {
		say("accept %v", err)
		return
	}
	_, _ = conn.Write([]byte("served")) //nolint:errcheck // the parent tells what it read
	_ = conn.Close()
	_ = ln.Close()
	again, err := daemon.Listen(port)
	if err != nil {
		say("rebind %v", err)
		return
	}
	_ = again.Close()
	say("rebind ok")
}

// errKind names Handed's mistake for the parent to compare.
func errKind(err error) string {
	if errors.Is(err, daemon.ErrHandedAddr) {
		return "another-address"
	}
	return "refused"
}

// startChild starts this test binary in mode, away from the test as start starts floe serve, with f handed down
// when not nil, expecting a listener on port, and returns the lines it tells, once it is done. It asks the child
// on port only when it serves: a port no child serves may be another test's.
func startChild(t *testing.T, mode string, f *os.File, port int, serves bool) []string {
	t.Helper()
	report := filepath.Join(t.TempDir(), "report")
	if err := os.WriteFile(report, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G204: this test binary; not the test's context, which start does not take either
	cmd := exec.CommandContext(context.WithoutCancel(t.Context()), os.Args[0], "-test.run=^$")
	handed, err := daemon.Background(cmd, f)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(os.Environ(), childEnv+"="+mode, portEnv+"="+strconv.Itoa(port), reportEnv+"="+report)
	if f != nil {
		cmd.Env = append(cmd.Env, daemon.ListenerEnv+"="+handed)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = cmd.Process.Kill(), cmd.Wait() }) //nolint:errcheck // done by now, or killed
	if f != nil {
		_ = f.Close() // the child holds its own copy now
	}
	if serves {
		ask(t, port)
	}
	for end := time.Now().Add(30 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if lines := readLines(t, report); len(lines) > 0 && lines[len(lines)-1] == "done" {
			return lines
		}
	}
	t.Fatalf("the child told %q, and not done", readLines(t, report))
	return nil
}

// ask connects to port, where a child serves one connection; one refused is told by the child's lines.
func ask(t *testing.T, port int) {
	t.Helper()
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(t.Context(), "tcp", daemon.Addr(port))
	if err != nil {
		return
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 16)
	n, _ := conn.Read(b) //nolint:errcheck // what it read is checked
	if string(b[:n]) != "served" {
		t.Errorf("the child answered %q", b[:n])
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // G304: the test's own
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []string
	for s := bufio.NewScanner(f); s.Scan(); {
		lines = append(lines, s.Text())
	}
	return lines
}

// listener binds a port for a child, and returns it, its port and a copy of its socket to hand down; the test's
// listener is closed, so that only the child holds the port.
func listener(t *testing.T) (*os.File, int) {
	t.Helper()
	held, err := daemon.Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	f, err := held.File()
	if err != nil {
		t.Fatal(err)
	}
	_ = held.Close()
	return f, portOf(t, held)
}

// portOf is the port ln listens on.
func portOf(t *testing.T, ln net.Listener) int {
	t.Helper()
	a, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("%v is not a TCP address", ln.Addr())
	}
	return a.Port
}

// A server started as start starts it takes the listener handed down and answers on it, started away from the
// command (in a session of its own on Unix); once it lets the listener go, the port is free: no copy was left.
func TestStartServes(t *testing.T) {
	t.Parallel()

	f, port := listener(t)
	got := strings.Join(startChild(t, "take", f, port, true), "\n")
	want := strings.Join([]string{"away " + wantAway, "handed ok", "rebind ok", "done"}, "\n")
	if got != want {
		t.Errorf("the child told\n%s\nwant\n%s", got, want)
	}
	// What opens a browser starts it away too, with nothing handed down.
	if got := startChild(t, "away", nil, 0, false); len(got) != 2 || got[0] != "away "+wantAway {
		t.Errorf("a child started away with nothing handed down told %q", got)
	}
}

// A server takes the listener handed down only when it is on its port; none handed down binds itself.
func TestHanded(t *testing.T) {
	t.Parallel()

	none := func(string) string { return "" }
	if ln, err := daemon.Handed(none, 1); ln != nil || err != nil {
		t.Errorf("Handed with none handed down = %v, %v", ln, err)
	}
	if _, err := daemon.Handed(func(string) string { return "three" }, 1); err == nil {
		t.Error("Handed took a descriptor that is not a number")
	}
	t.Run("standard output", func(t *testing.T) {
		t.Parallel()
		testfs.UnixOnly(t, "descriptors below 3")
		// Refused before it is taken, which would close it.
		if _, err := daemon.Handed(func(string) string { return "1" }, 1); err == nil ||
			!strings.Contains(err.Error(), "not a descriptor handed down") {
			t.Errorf("Handed of standard output = %v", err)
		}
	})
	// What is taken and closed is a child's, as a server's is: never the test's own.
	f, port := listener(t)
	if got := startChild(t, "take", f, port+1, false); len(got) < 2 || got[1] != "handed another-address" {
		t.Errorf("Handed of a listener on another port told %q", got)
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "a")) //nolint:gosec // G304: the test's own
	if err != nil {
		t.Fatal(err)
	}
	if got := startChild(t, "take", file, port, false); len(got) < 2 || got[1] != "handed refused" {
		t.Errorf("Handed of a file that is no listener told %q", got)
	}
}

// Only what another port mends is the user's mistake: a port in use or refused; any other is floe's failure. The
// system's own errors are told apart, as Listen returns them.
func TestCannotListenWhy(t *testing.T) {
	t.Parallel()

	held, err := daemon.Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() })
	_, inUse := daemon.Listen(portOf(t, held))
	if inUse == nil {
		t.Fatal("a second Listen on a held port was not refused")
	}
	for _, c := range []struct {
		name string
		err  error
		key  string
	}{
		{"in use", inUse, "daemon_port_in_use"},
		{"refused", deniedListen(t), "daemon_cannot_listen"},
		{"refused, as the system names it", deniedValue, "daemon_cannot_listen"},
		{"too many files", syscall.EMFILE, ""},
		{"no loopback", errors.New("no loopback"), ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.err == nil {
				t.Skip("no port is refused here: port 1 opens, to root or with no privileged ports")
			}
			err := daemon.CannotListen(7325, c.err)
			var ue *usererr.Error
			if ok := errors.As(err, &ue); ok != (c.key != "") || ok && ue.Key != c.key {
				t.Errorf("CannotListen(%v) = %#v, want %q", c.err, err, c.key)
			}
		})
	}
}
