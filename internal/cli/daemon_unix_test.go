//go:build unix

package cli_test

import (
	"strings"
	"testing"
	"time"

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
	// The server is still there when its dump begins, and may answer the cleanup's first question and no other: waited
	// for until it has ended.
	for deadline := time.Now().Add(5 * time.Second); !portFree(e.port); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the server (pid %d) holds port %s 5 s after SIGQUIT", h.PID, e.port)
		}
	}
	log := e.logWith(t, "goroutine")
	if !strings.Contains(log, "SIGQUIT") {
		t.Errorf("%s = %q, want the crash in it", server.LogName, log)
	}
}
