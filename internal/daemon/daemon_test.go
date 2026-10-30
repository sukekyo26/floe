package daemon_test

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/server"
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
			health, err := json.Marshal(server.Health{App: server.App, PID: told})
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var bodies []string
			srv := &http.Server{ReadHeaderTimeout: time.Second}
			srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/stop" {
					_, _ = w.Write(health) //nolint:errcheck // a test's answer
					return
				}
				b, _ := io.ReadAll(r.Body) //nolint:errcheck // a short body, compared below
				mu.Lock()
				bodies = append(bodies, string(b))
				mu.Unlock()
				if string(b) != `{"pid":`+strconv.Itoa(told)+`}` {
					w.WriteHeader(http.StatusConflict)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				go func() {
					time.Sleep(100 * time.Millisecond)
					_ = srv.Close() //nolint:errcheck // nothing to do
				}()
			})
			go func() { _ = srv.Serve(ln) }()     //nolint:errcheck // closed below
			t.Cleanup(func() { _ = srv.Close() }) //nolint:errcheck // nothing to do
			addr, ok := ln.Addr().(*net.TCPAddr)
			if !ok {
				t.Fatalf("a TCP listener on %v", ln.Addr())
			}
			port := addr.Port

			//nolint:exhaustruct // the rest of the health is not read
			saw := daemon.Status{State: daemon.Ours, Health: server.Health{App: server.App, PID: c.pid}}
			if _, err := daemon.Stop(t.Context(), port, saw); err != nil {
				t.Errorf("Stop = %v", err)
			}
			mu.Lock()
			got := strings.Join(bodies, " ")
			mu.Unlock()
			if want := `{"pid":` + strconv.Itoa(c.pid) + `}`; got != want {
				t.Errorf("bodies = %s, want %s", got, want)
			}
			if p := daemon.Probe(t.Context(), daemon.Base(port)); (p.State == daemon.Ours) != c.answers {
				t.Errorf("after Stop the server answers: %v, want %v", p.State == daemon.Ours, c.answers)
			}
		})
	}
}
