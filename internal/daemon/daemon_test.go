package daemon_test

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"floe/internal/daemon"
	"floe/internal/server"
	"floe/internal/servetest"
	"floe/internal/usererr"
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
	_, err := daemon.Ensure(t.Context(), daemon.Server{Port: 1, Root: t.TempDir(), Build: "b", Env: nil})
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

// Stop names the server it saw: one of another pid (started after it) refuses with 409 and goes on, one of that
// pid stops; the body names it either way.
func TestStopNamesTheServer(t *testing.T) {
	t.Parallel()

	const told = 4242
	for _, c := range []struct {
		name    string
		pid     int
		answers bool
	}{{"another", 7, true}, {"that one", told, false}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var lc net.ListenConfig
			ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr, ok := ln.Addr().(*net.TCPAddr)
			if !ok {
				t.Fatalf("a TCP listener on %v", ln.Addr())
			}
			bodies := servetest.Fake{PID: told, Linger: 100 * time.Millisecond}.Serve(t, ln) //nolint:exhaustruct // the rest unset

			//nolint:exhaustruct // the rest of the health is not read
			saw := daemon.Status{State: daemon.Ours, Health: server.Health{App: server.App, PID: c.pid}}
			p, asked, err := daemon.Stop(t.Context(), addr.Port, saw)
			if err != nil {
				t.Errorf("Stop = %v", err)
			}
			// Only a server that took the request, with 204, makes it this command's: one that refused it (409) did not.
			if asked == c.answers {
				t.Errorf("Stop took by the server: %v, want %v", asked, !c.answers)
			}
			if got, want := strings.Join(bodies(), " "), `{"pid":`+strconv.Itoa(c.pid)+`}`; got != want {
				t.Errorf("bodies = %s, want %s", got, want)
			}
			if (p.State == daemon.Ours) != c.answers {
				t.Errorf("after Stop the server answers: %v, want %v", p.State == daemon.Ours, c.answers)
			}
		})
	}
}

// Ensure returns the pid of the server it stopped only when the server took its request to stop: one that refused it
// (409: another command's request stopped it, or it was another server) is not told of; one that took it and went on
// answering, so that Stop failed, is told of with the mistake. A fake server of another build stands for the one
// running.
func TestEnsureStopped(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		fake servetest.Fake
		want int
		kept bool
	}{
		{"another command stopped it", servetest.Fake{PID: 4242, Next: 4343}, 0, false},        //nolint:exhaustruct // the rest unset
		{"it took the request and stayed", servetest.Fake{PID: 4242, Keeps: true}, 4242, true}, //nolint:exhaustruct // as above
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var lc net.ListenConfig
			ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr, ok := ln.Addr().(*net.TCPAddr)
			if !ok {
				t.Fatalf("a TCP listener on %v", ln.Addr())
			}
			c.fake.Serve(t, ln)

			stopped, err := daemon.Ensure(t.Context(), daemon.Server{Port: addr.Port, Build: "b"}) //nolint:exhaustruct // the rest unset
			var ue *usererr.Error
			if kept := errors.As(err, &ue) && ue.Key == "daemon_kept_answering"; kept != c.kept || !c.kept && err != nil {
				t.Errorf("Ensure = %d, %v, want a mistake of the server that kept answering: %v", stopped, err, c.kept)
			}
			if stopped != c.want {
				t.Errorf("Ensure told it stopped pid %d, want %d", stopped, c.want)
			}
		})
	}
}
