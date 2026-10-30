package servetest

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/server"
)

// Fake answers as a server of floe of PID, running no floe, so that a command's waiting is seen whatever a real
// server's speed: its health, and /api/stop with 204 to a request naming the pid its health tells (409 to another),
// or with Answer when set. After a 204 it stops answering Linger later, unless Keeps.
type Fake struct {
	PID int
	// Next is the pid its health tells after the first answer, when set: another server, started meanwhile.
	Next   int
	Answer int
	// Hang leaves a request to stop unanswered until the asking gives up.
	Hang   bool
	Keeps  bool
	Linger time.Duration
}

// Serve answers on ln until the test ends, and returns the bodies of the requests to stop sent so far.
func (f Fake) Serve(t *testing.T, ln net.Listener) func() []string {
	t.Helper()

	var mu sync.Mutex
	var bodies []string
	pid, told := f.PID, false
	srv := &http.Server{ReadHeaderTimeout: time.Second} //nolint:exhaustruct // a test's server
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/api/stop" {
			body, err := json.Marshal(server.Health{
				App: server.App, PID: pid, Root: "", Build: "", Templates: "", Parts: "", Themes: "", Icons: "",
			})
			if err != nil {
				t.Error(err)
			}
			_, _ = w.Write(body) //nolint:errcheck // a test's answer
			if !told && f.Next != 0 {
				pid, told = f.Next, true
			}
			return
		}
		b, _ := io.ReadAll(r.Body) //nolint:errcheck // a short body, compared below
		bodies = append(bodies, string(b))
		switch {
		case f.Hang:
			mu.Unlock()
			<-r.Context().Done()
			mu.Lock()
			return
		case f.Answer != 0:
			w.WriteHeader(f.Answer)
		case string(b) != `{"pid":`+strconv.Itoa(pid)+`}`:
			w.WriteHeader(http.StatusConflict)
			return
		default:
			w.WriteHeader(http.StatusNoContent)
		}
		if f.Answer == 0 && !f.Keeps {
			go func() {
				time.Sleep(f.Linger)
				_ = srv.Close() //nolint:errcheck // nothing to do
			}()
		}
	})
	go func() { _ = srv.Serve(ln) }()     //nolint:errcheck // closed below
	t.Cleanup(func() { _ = srv.Close() }) //nolint:errcheck // nothing to do
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}
