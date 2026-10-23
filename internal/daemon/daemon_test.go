package daemon_test

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/usererr"
)

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
	// Standard output is refused before it is taken, which would close it.
	if _, err := daemon.Handed(func(string) string { return "1" }, 1); err == nil ||
		!strings.Contains(err.Error(), "not a descriptor handed down") {
		t.Errorf("Handed of standard output = %v", err)
	}

	held, err := daemon.Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() })
	_, p, err := net.SplitHostPort(held.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	f, err := held.File()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	// Handed takes, and closes, the descriptor it is given, as a server does once: each call is handed
	// a copy of its own.
	handed := func(t *testing.T) func(string) string {
		t.Helper()
		fd, dupErr := syscall.Dup(int(f.Fd())) //nolint:gosec // G115: a descriptor fits an int
		if dupErr != nil {
			t.Fatal(dupErr)
		}
		return func(string) string { return strconv.Itoa(fd) }
	}
	if _, err = daemon.Handed(handed(t), port+1); !errors.Is(err, daemon.ErrHandedAddr) {
		t.Errorf("Handed of a listener on another port = %v", err)
	}
	ln, err := daemon.Handed(handed(t), port)
	if err != nil || ln.Addr().String() != held.Addr().String() {
		t.Fatalf("Handed of a listener on the port = %v, %v", ln, err)
	}
	_ = ln.Close()
}

// A port that cannot be bound for another reason than its use says so, with the reason.
func TestCannotListen(t *testing.T) {
	t.Parallel()

	// Port 1 is privileged on linux; where it is not, or to root, there is nothing to refuse.
	if ln, err := daemon.Listen(1); !errors.Is(err, syscall.EACCES) {
		if err == nil {
			_ = ln.Close()
		}
		t.Skip("port 1 is not refused here")
	}
	err := daemon.Ensure(t.Context(), daemon.Server{Port: 1, Root: t.TempDir(), Build: "b", Env: nil})
	var ue *usererr.Error
	if !errors.As(err, &ue) || ue.Key != "daemon_cannot_listen" || len(ue.Args) != 1 {
		t.Errorf("Ensure on a privileged port = %#v", err)
	}
}

// Only what another port mends is the user's mistake: a port in use or refused; any other is floe's failure.
func TestCannotListenWhy(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		err error
		key string
	}{
		{syscall.EADDRINUSE, "daemon_port_in_use"},
		{syscall.EACCES, "daemon_cannot_listen"},
		{syscall.EMFILE, ""},
		{errors.New("no loopback"), ""},
	} {
		err := daemon.CannotListen(7325, c.err)
		var ue *usererr.Error
		if ok := errors.As(err, &ue); ok != (c.key != "") || ok && ue.Key != c.key {
			t.Errorf("CannotListen(%v) = %#v, want %q", c.err, err, c.key)
		}
	}
}
