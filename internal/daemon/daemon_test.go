package daemon_test

import (
	"errors"
	"net"
	"strconv"
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
	bad := func(string) string { return "three" }
	if _, err := daemon.Handed(bad, 1); err == nil {
		t.Error("Handed took a descriptor that is not a number")
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
	// Handed takes the descriptor it is given, as a server does once: each call is handed a copy.
	handed := func(t *testing.T) func(string) string {
		t.Helper()
		f, fileErr := held.File()
		if fileErr != nil {
			t.Fatal(fileErr)
		}
		t.Cleanup(func() { _ = f.Close() })
		return func(string) string { return strconv.Itoa(int(f.Fd())) } //nolint:gosec // G115: a descriptor fits an int
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
