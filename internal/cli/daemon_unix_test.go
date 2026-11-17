//go:build unix

package cli_test

import (
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"floe/internal/server"
)

// otherSystemsRoot is the store a server of another system names: Windows's, seen from WSL.
const otherSystemsRoot = `C:\Users\u\.local\share\floe`

// A server that is made to crash (SIGQUIT) leaves what the runtime wrote of it in server.log: the standard error of
// the server a command starts is discarded, so the log is where it is.
func TestCrashOutput(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	wantRun(t, "publish", e.run("publish", "page.yaml"), 0, e.base()+"/a/page/\n", projectLine)
	h, up := e.health(t.Context())
	if !up {
		t.Fatal("no server after a publish")
	}
	if err := unix.Kill(h.PID, unix.SIGQUIT); err != nil {
		t.Fatal(err)
	}
	log := e.logWith(t, "goroutine")
	if !strings.Contains(log, "SIGQUIT") {
		t.Errorf("%s = %q, want the crash in it", server.LogName, log)
	}
}
