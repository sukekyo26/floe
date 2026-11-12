package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"floe/internal/store"
	"floe/internal/usererr"
)

// lockedBuffer is what a server writes its log to in a test, read while it goes on writing.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p) //nolint:wrapcheck // a buffer
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// logged is a server whose log goes to the buffer.
func logged(t *testing.T) (*Server, *lockedBuffer) {
	t.Helper()

	s, _ := watched(t, false)
	var buf lockedBuffer
	s.log = newLog(&buf)
	return s, &buf
}

// fileMade is a file of size bytes of x.
func fileMade(t *testing.T, path string, size int) {
	t.Helper()

	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The log is opened to append to, with no wider permissions than 0600; one over 1 MiB is moved to server.log.1 when
// it is opened, replacing the generation before, and one of exactly 1 MiB is not; a link is followed; what is not a
// regular file is refused, a FIFO without the open blocking.
func TestOpenLog(t *testing.T) {
	t.Parallel()

	t.Run("appended", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		for _, text := range []string{"a\n", "b\n"} {
			f, err := OpenLog(root)
			if err != nil {
				t.Fatal(err)
			}
			info, err := f.Stat()
			if err != nil || info.Mode().Perm()&^0o600 != 0 {
				t.Errorf("log made with permissions %v (%v), want no wider than 0600", info.Mode().Perm(), err)
			}
			if _, err = f.WriteString(text); err != nil {
				t.Fatal(err)
			}
			_ = f.Close() //nolint:errcheck // test
		}
		if b, err := os.ReadFile(filepath.Join(root, LogName)); err != nil || string(b) != "a\nb\n" {
			t.Errorf("log opened twice = %q, %v", b, err)
		}
		if _, err := os.Stat(filepath.Join(root, LogName+".1")); err == nil {
			t.Error("a small log was moved aside")
		}
	})

	t.Run("moved aside over the cap", func(t *testing.T) {
		t.Parallel()

		for _, c := range []struct {
			size  int
			moved bool
		}{{logCap, false}, {logCap + 1, true}} {
			root := t.TempDir()
			log := filepath.Join(root, LogName)
			fileMade(t, log, c.size)
			if err := os.WriteFile(log+".1", []byte("the generation before"), 0o600); err != nil {
				t.Fatal(err)
			}
			f, err := OpenLog(root)
			if err != nil {
				t.Fatal(err)
			}
			_ = f.Close() //nolint:errcheck // test
			now, errNow := os.Stat(log)
			old, errOld := os.Stat(log + ".1")
			if errNow != nil || errOld != nil {
				t.Fatalf("%d bytes: %v, %v", c.size, errNow, errOld)
			}
			switch {
			case c.moved && (now.Size() != 0 || old.Size() != int64(c.size)):
				t.Errorf("%d bytes: now %d bytes, aside %d bytes: want a new log and the old one aside", c.size, now.Size(), old.Size())
			case !c.moved && (now.Size() != int64(c.size) || old.Size() != int64(len("the generation before"))):
				t.Errorf("%d bytes: now %d bytes, aside %d bytes: want both left", c.size, now.Size(), old.Size())
			}
		}
	})

	t.Run("a link is followed", func(t *testing.T) {
		t.Parallel()

		root, elsewhere := t.TempDir(), filepath.Join(t.TempDir(), "real.log")
		if err := os.WriteFile(elsewhere, []byte("t"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(elsewhere, filepath.Join(root, LogName)); err != nil {
			t.Fatal(err)
		}
		f, err := OpenLog(root)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteString("z")                              //nolint:errcheck // read back below
		_ = f.Close()                                          //nolint:errcheck // as above
		if b, _ := os.ReadFile(elsewhere); string(b) != "tz" { //nolint:errcheck // as above
			t.Errorf("what the link leads to = %q, want tz", b)
		}
	})

	t.Run("a link to nowhere is not followed", func(t *testing.T) {
		t.Parallel()

		root, made := t.TempDir(), filepath.Join(t.TempDir(), "made.log")
		if err := os.Symlink(made, filepath.Join(root, LogName)); err != nil {
			t.Fatal(err)
		}
		f, err := OpenLog(root)
		if f != nil {
			_ = f.Close() //nolint:errcheck // not wanted
		}
		if ue, ok := errors.AsType[*usererr.Error](err); !ok || ue.Key != "cannot_write" {
			t.Errorf("OpenLog over a link to nowhere = %#v, want cannot_write", err)
		}
		if _, err = os.Stat(made); err == nil {
			t.Error("the log was made where a link to nowhere leads, out of the store")
		}
	})

	t.Run("not a regular file", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []string{"a directory", "a FIFO"} {
			root := t.TempDir()
			log := filepath.Join(root, LogName)
			var err error
			if kind == "a directory" {
				err = os.Mkdir(log, 0o700)
			} else {
				err = syscall.Mkfifo(log, 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			got := make(chan error, 1)
			go func() {
				f, openErr := OpenLog(root)
				if f != nil {
					_ = f.Close() //nolint:errcheck // not wanted
				}
				got <- openErr
			}()
			select {
			case err = <-got:
				ue, ok := errors.AsType[*usererr.Error](err)
				if !ok || ue.Key != "cannot_write" || ue.File() != log {
					t.Errorf("OpenLog over %s = %#v, want cannot_write at %s", kind, err, log)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("OpenLog over %s did not come back", kind)
			}
		}
	})
}

// A request that panics is answered with floe's failure (500) saying where it was and where the stack is, the stack
// is in the log from where it panicked, and the server goes on; http.ErrAbortHandler goes on to net/http as it is,
// answered and written of nowhere.
func TestRecovered(t *testing.T) {
	t.Parallel()

	s, buf := logged(t)
	h := s.recovered(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	for _, accept := range []string{"application/json", "text/html"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/pages/x?secret=1", http.NoBody)
		req.Header.Set("Accept", accept)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		failure := w.Body.String()
		if accept == "application/json" {
			var r usererr.Report
			if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
				t.Fatalf("%s: %v", w.Body, err)
			}
			failure = r.Failure
		}
		if w.Code != http.StatusInternalServerError {
			t.Errorf("a panic answered %d (Accept %s), want 500", w.Code, accept)
		}
		for _, want := range []string{"PUT /api/pages/x", "boom", filepath.Join(s.cfg.Store.Root(), LogName)} {
			if !strings.Contains(failure, want) {
				t.Errorf("the answer (Accept %s) = %q, want it to say %q", accept, failure, want)
			}
		}
		if strings.Contains(failure, "secret") {
			t.Errorf("the answer = %q: the query is not told", failure)
		}
	}
	// The stack is from where it panicked: the handler's own frame is in it.
	for _, want := range []string{
		"level=ERROR", `msg="panic in request"`, "method=PUT", "path=/api/pages/x", "panic=boom",
		"goroutine", "TestRecovered", "pid=" + strconv.Itoa(os.Getpid()),
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("the log = %q, want it to hold %q", buf.String(), want)
		}
	}
	if strings.Contains(buf.String(), "secret") {
		t.Errorf("the log = %q: the query is not written", buf.String())
	}

	var aborted any
	quiet, quietBuf := logged(t)
	cut := quiet.recovered(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	w := httptest.NewRecorder()
	func() {
		defer func() { aborted = recover() }()
		cut.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
	}()
	again, isErr := aborted.(error)
	if !isErr || !errors.Is(again, http.ErrAbortHandler) || w.Body.Len() != 0 || quietBuf.String() != "" {
		t.Errorf("ErrAbortHandler: panicked again with %v, answered %q, logged %q", aborted, w.Body, quietBuf)
	}
}

// The handler a server serves has recovered in it: a request that panics inside the real routes (the drawing it is
// asked to read panics) is answered 500 and written of, and the next request is answered.
func TestPanicThroughHandler(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	var buf lockedBuffer
	s.log = newLog(&buf)
	h := s.Handler()
	drawing := s.cfg.Drawing
	s.cfg.Drawing = func() (store.Drawing, error) { panic("probe: drawing") }
	// The drawing is read again when what it is read from changes.
	write(t, filepath.Join(dir, "mine", "page.html"), "<p>{{.Data.n}}</p>\n")
	w := api(t.Context(), t, h, http.MethodPost, "/api/rebuild", `{}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(told(t, w), "probe: drawing") {
		t.Errorf("rebuild with a drawing that panics = %d %s, want 500 saying so", w.Code, w.Body)
	}
	if got := buf.String(); !strings.Contains(got, `msg="panic in request"`) || !strings.Contains(got, "method=POST") ||
		!strings.Contains(got, "path=/api/rebuild") || !strings.Contains(got, `panic="probe: drawing"`) {
		t.Errorf("the log = %q, want the panic in it", buf.String())
	}
	s.cfg.Drawing = drawing
	if w = api(t.Context(), t, h, http.MethodGet, "/api/health", ""); w.Code != http.StatusOK {
		t.Errorf("health after a panic = %d, want 200", w.Code)
	}
}

// A server writes that it serves (with its pid, build, store and address), why it stops (asked to stop, or a signal:
// its context ended), and that it has stopped, in that order, or the mistake that ended it. Each line has its time
// and the pid of the process.
func TestServeRecords(t *testing.T) {
	t.Parallel()

	inOrder := func(t *testing.T, log string, want ...string) {
		t.Helper()

		at := 0
		for _, w := range want {
			i := strings.Index(log[at:], w)
			if i < 0 {
				t.Errorf("the log = %q, want %q after %q", log, w, log[:at])
				return
			}
			at += i + len(w)
		}
	}
	waitFor := func(t *testing.T, buf *lockedBuffer, text string) {
		t.Helper()

		for deadline := time.Now().Add(2 * time.Second); !strings.Contains(buf.String(), text); {
			if time.Now().After(deadline) {
				t.Fatalf("the log = %q, want %q", buf.String(), text)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	t.Run("asked to stop", func(t *testing.T) {
		t.Parallel()

		s, buf := logged(t)
		addr, done := serving(t.Context(), t, s)
		waitFor(t, buf, "msg=serving")
		// A connection that sent nothing holds Shutdown up (it waits for it, 5 s at most): the reason is written
		// before that, as the next server starts once the port is free, which Shutdown's first step does.
		var d net.Dialer
		held, err := d.DialContext(t.Context(), "tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Close() //nolint:errcheck // test
		if code := askStop(t, addr, `{"pid": `+strconv.Itoa(os.Getpid())+`}`); code != http.StatusNoContent {
			t.Fatalf("stop = %d", code)
		}
		waitFor(t, buf, "msg=stopping")
		select {
		case err = <-done:
			t.Fatalf("Serve ended while a connection was held open: %v", err)
		default:
		}
		if strings.Contains(buf.String(), "msg=stopped") {
			t.Errorf("the log = %q: stopped before Serve ended", buf.String())
		}
		_ = held.Close() //nolint:errcheck // lets Shutdown end
		ended(t, done)
		if strings.Contains(buf.String(), "level=WARN") {
			t.Errorf("the log = %q: a stop that cut nothing off warned", buf.String())
		}
		pid := "pid=" + strconv.Itoa(os.Getpid())
		inOrder(t, buf.String(), "level=INFO", "msg=serving", pid, "root="+s.cfg.Store.Root(), "addr="+addr, "build=test",
			"msg=stopping", pid, `why="asked to stop"`, "msg=stopped", pid)
		if strings.Contains(buf.String(), "a signal") {
			t.Errorf("the log = %q: a stop asked for was written as a signal", buf.String())
		}
	})

	t.Run("a request is cut off", func(t *testing.T) {
		t.Parallel()

		s, buf := logged(t)
		s.grace = 50 * time.Millisecond
		addr, done := serving(t.Context(), t, s)
		waitFor(t, buf, "msg=serving")
		var d net.Dialer
		held, err := d.DialContext(t.Context(), "tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Close() //nolint:errcheck // test
		if code := askStop(t, addr, `{"pid": `+strconv.Itoa(os.Getpid())+`}`); code != http.StatusNoContent {
			t.Fatalf("stop = %d", code)
		}
		// The connection is not given up in the grace: what Shutdown could not finish is cut off, and said.
		ended(t, done)
		inOrder(t, buf.String(), "msg=stopping", "level=WARN", `msg="cutting off what was under way"`, "error=", "msg=stopped")
	})

	t.Run("its context ended", func(t *testing.T) {
		t.Parallel()

		s, buf := logged(t)
		ctx, cancel := context.WithCancel(t.Context())
		_, done := serving(ctx, t, s)
		waitFor(t, buf, "msg=serving")
		cancel()
		ended(t, done)
		inOrder(t, buf.String(), "msg=serving", "msg=stopping", `why="a signal"`, "msg=stopped")
		if strings.Contains(buf.String(), "asked to stop") {
			t.Errorf("the log = %q: a signal was written as a stop asked for", buf.String())
		}
	})

	t.Run("a mistake ends it", func(t *testing.T) {
		t.Parallel()

		s, buf := logged(t)
		var lc net.ListenConfig
		ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		_ = ln.Close() //nolint:errcheck // closed: serving it fails
		if err = s.Serve(t.Context(), ln); err == nil {
			t.Fatal("Serve on a closed listener ended with no mistake")
		}
		inOrder(t, buf.String(), "msg=serving", "level=ERROR", "msg=stopped", "error=")
		if strings.Contains(buf.String(), "msg=stopping") {
			t.Errorf("the log = %q: it was not asked to stop", buf.String())
		}
	})
}
