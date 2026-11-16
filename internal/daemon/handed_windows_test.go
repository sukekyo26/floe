package daemon_test

import (
	"testing"

	"golang.org/x/sys/windows"

	"floe/internal/daemon"
)

// wantAway is what a child started away tells on Windows: nothing, as the creation flags cannot be told from the
// child's side with what x/sys has; they are read in review and tried on a real machine (S315's W18, W19, W22).
const wantAway = "unchecked"

func awayNote() string { return "unchecked" }

// deniedListen is the system's error of a port refused (WSAEACCES): binding with SO_REUSEADDR a port a listener
// holds.
func deniedListen(t *testing.T) error {
	t.Helper()
	held, err := daemon.Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() })
	s, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.Closesocket(s) //nolint:errcheck // the test's own
	if err := windows.SetsockoptInt(s, windows.SOL_SOCKET, windows.SO_REUSEADDR, 1); err != nil {
		t.Fatal(err)
	}
	return windows.Bind(s, &windows.SockaddrInet4{Port: portOf(t, held), Addr: [4]byte{127, 0, 0, 1}})
}
