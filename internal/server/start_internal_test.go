package server

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"floe/internal/files"
	"floe/internal/store"
)

// stall holds the first read of a server's drawing until let: entered is closed once the read is in, and ended is
// set as it returns.
type stall struct {
	entered, release chan struct{}
	ended            atomic.Bool
	let              func()
}

// stalled makes s's next read of its drawing a stall's, the reads after it as they were. The watch does not look
// within the test, so that the read stalled is Serve's own first one.
// A Serve that reads before it answers is not failed but stopped: its test waits on until go test's timeout, whose
// goroutines name it.
func stalled(t *testing.T, s *Server) *stall {
	t.Helper()

	s.every = time.Hour
	st := &stall{entered: make(chan struct{}), release: make(chan struct{}), ended: atomic.Bool{}, let: nil}
	var let sync.Once
	st.let = func() { let.Do(func() { close(st.release) }) }
	t.Cleanup(st.let)
	read := s.cfg.Drawing
	var first atomic.Bool
	s.cfg.Drawing = func() (store.Drawing, error) {
		if !first.CompareAndSwap(false, true) {
			return read()
		}
		close(st.entered)
		<-st.release
		d, err := read()
		st.ended.Store(true)
		return d, err
	}
	return st
}

// in waits for the stalled read to begin: Serve begins it.
func (st *stall) in(t *testing.T) {
	t.Helper()

	select {
	case <-st.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the first read did not begin")
	}
}

// ask sends a GET of path to addr, all of it, before it returns: the answer is read by answer.
func ask(t *testing.T, addr, path string) net.Conn {
	t.Helper()

	var d net.Dialer
	conn, err := d.DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() }) //nolint:errcheck // test
	if _, err = io.WriteString(conn, "GET "+path+" HTTP/1.1\r\nHost: "+addr+"\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	return conn
}

func answer(t *testing.T, conn net.Conn) (int, string) {
	t.Helper()

	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	_ = res.Body.Close() //nolint:errcheck // read
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(b)
}

// stopped ends what started serves and waits for Serve, with no time limit: its first read is let go first.
func stopped(t *testing.T, st *stall, cancel context.CancelFunc, done <-chan error) {
	t.Helper()

	st.let()
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// A server answers its health while its drawing is still read: what starts it waits for that answer alone.
func TestHealthBeforeRead(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	st := stalled(t, s)
	ctx, cancel := context.WithCancel(t.Context())
	addr, done := started(ctx, t, s)
	st.in(t)
	if code, body := answer(t, ask(t, addr, "/api/health")); code != http.StatusOK {
		t.Errorf("health while the drawing is read = %d %s", code, body)
	}
	stopped(t, st, cancel, done)
}

// A linked data file opened, or its version asked, before the server's first read is drawn with what that read
// gives, after it: with its page, or with the mistakes of what it read.
func TestOpenWaitsForRead(t *testing.T) {
	t.Parallel()

	for _, broken := range []bool{false, true} {
		name := map[bool]string{false: "read", true: "mistaken"}[broken]
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, dir := watched(t, false)
			data := filepath.Join(filepath.Dir(dir), "proj", "l.yaml")
			if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
				t.Fatal(err)
			}
			write(t, data, mineData)
			// Linked by another server of the store, so that s has read nothing before it serves.
			if w := api(t.Context(), t, New(s.cfg).Handler(), http.MethodPost, "/api/pages", linkBody(t, data)); w.Code != http.StatusCreated {
				t.Fatalf("link = %d %s", w.Code, w.Body)
			}
			if broken {
				write(t, filepath.Join(dir, "mine", "schema.json"), "{")
			}
			m := s.cfg.Store.ReadMeta("l")
			file, ok := s.cfg.Store.File("l", m, "")
			if !ok {
				t.Fatal("no linked file")
			}
			// The version of the drawing of the first read (generation 1), as version tells it.
			since := s.id + ".1." + files.Mark(file) + "." + m.UpdatedAt + "." + m.Theme
			st := stalled(t, s)
			ctx, cancel := context.WithCancel(t.Context())
			addr, done := started(ctx, t, s)
			st.in(t)
			page, changed := ask(t, addr, "/a/l/?raw"), ask(t, addr, "/api/changed/l?since="+since)
			st.let()
			code, body := answer(t, page)
			switch {
			case broken && !strings.Contains(body, "schema.json"):
				t.Errorf("open = %d %s, want the mistake of schema.json", code, body)
			case !broken && (code != http.StatusOK || !strings.Contains(body, "<body>1")):
				t.Errorf("open = %d %s, want the page drawn", code, body)
			}
			// The version is in the page's script, as a URL's query.
			if !strings.Contains(body, url.QueryEscape(since)) {
				t.Errorf("open = %s, want the version %s", body, since)
			}
			if _, got := answer(t, changed); got != "same" {
				t.Errorf("changed since %s = %s, want same", since, got)
			}
			stopped(t, st, cancel, done)
			if got := s.version("l", m, "", 0, noDiff); got != since {
				t.Errorf("version = %s, want %s: the test's version is not version's", got, since)
			}
		})
	}
}

// closeSeen is a listener that tells when it is closed: Shutdown's first step.
type closeSeen struct {
	net.Listener

	once   sync.Once
	closed chan struct{}
}

func (l *closeSeen) Close() error {
	l.once.Do(func() { close(l.closed) })
	return l.Listener.Close() //nolint:wrapcheck // the listener's
}

// Serve does not return before its first read ends, though it is stopped while it reads.
func TestServeWaitsForRead(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	st := stalled(t, s)
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	seen := &closeSeen{Listener: ln, once: sync.Once{}, closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, seen) }()
	st.in(t)
	cancel()
	<-seen.closed
	// LIMIT: telling that Serve waits for the read takes a wait: 200 ms is how long a Serve that returns is looked
	// for. A Serve that waits passes however long it is; a slow machine may only miss one that does not.
	select {
	case err = <-done:
		t.Fatalf("Serve returned while its first read ran: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	st.let()
	if err = <-done; err != nil {
		t.Errorf("Serve = %v", err)
	}
	if !st.ended.Load() {
		t.Error("Serve returned before its first read ended")
	}
}

// The first read is read once: a now() that comes while Serve's runs waits for it and reads nothing, and a first()
// after it reads nothing, though what the drawing is read from changed (that is the watch's to read).
func TestFirstReadOnce(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	s.every = time.Hour
	read := s.cfg.Drawing
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	var let sync.Once
	t.Cleanup(func() { let.Do(func() { close(release) }) })
	s.cfg.Drawing = func() (store.Drawing, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return read()
	}
	ctx, cancel := context.WithCancel(t.Context())
	_, done := started(ctx, t, s)
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the first read did not begin")
	}
	asked := make(chan struct{})
	go func() {
		s.now()
		close(asked)
	}()
	// LIMIT: now() is given 100 ms to come to the read under way; a first() that looks at gen before it waits is
	// missed if it comes later. One that waits passes however long it is.
	time.Sleep(100 * time.Millisecond)
	let.Do(func() { close(release) })
	<-asked
	if n := calls.Load(); n != 1 {
		t.Errorf("the drawing was read %d times by Serve's first read and a now() during it, want once", n)
	}
	mine(t, dir, "changed after the first read")
	s.first()
	if _, gen := s.now(); calls.Load() != 1 || gen != 1 {
		t.Errorf("first() after the first read: %d reads, generation %d; want 1 and 1", calls.Load(), gen)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Serve = %v", err)
	}
}
