//go:build unix

package daemon_test

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"floe/internal/daemon"
)

// wantAway is what a child started away tells: a session of its own.
const wantAway = "session-leader"

func awayNote() string {
	if sid, err := unix.Getsid(0); err == nil && sid == os.Getpid() {
		return "session-leader"
	}
	return "in-the-test's-session"
}

// deniedListen is the system's error of a port refused: port 1, privileged; nil where it is not refused (root).
func deniedListen(t *testing.T) error {
	t.Helper()
	ln, err := daemon.Listen(1)
	if err == nil {
		_ = ln.Close()
	}
	return err
}
