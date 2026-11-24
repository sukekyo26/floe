//go:build windows

package testfs

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A pipe whose one instance a client has taken turns the next client away at once (ERROR_PIPE_BUSY) until the
// server makes its next instance; connected waits that out, and counts the first client, not its own marker.
func TestPipeBusyWaited(t *testing.T) {
	t.Parallel()

	s, h, err := newPipeServer()
	if err != nil {
		t.Fatal(err)
	}
	serve := sync.OnceFunc(func() { go s.serve(h) })
	var taken *os.File
	// The client is let go first, so its instance's handler ends; serve is given h on every way out, to close it.
	t.Cleanup(func() {
		if taken != nil {
			_ = taken.Close()
		}
		serve()
		s.stop()
	})

	taken, err = os.Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(s.path, os.O_WRONLY, 0)
	if err == nil {
		_ = f.Close()
	}
	if !errors.Is(err, windows.ERROR_PIPE_BUSY) {
		t.Fatalf("a second client of a pipe of one instance, taken: %v, want ERROR_PIPE_BUSY", err)
	}

	time.AfterFunc(200*time.Millisecond, serve)
	if n, err := s.connected(); err != nil || n != 1 {
		t.Errorf("connected() = %d, %v, want 1", n, err)
	}
}
