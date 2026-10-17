package daemon_test

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/daemon"
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
