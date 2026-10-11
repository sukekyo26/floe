package server

import (
	"testing"
	"time"
)

// TestIdleExpireRechecks plays the timer firing just as a request begins, or begins and ends: the
// fired timer must not close done then.
func TestIdleExpireRechecks(t *testing.T) {
	t.Parallel()

	i := newIdle(time.Hour)
	closed := func() bool {
		select {
		case <-i.done:
			return true
		default:
			return false
		}
	}
	i.mu.Lock()
	i.active = 1
	i.last = time.Now().Add(-2 * time.Hour)
	i.mu.Unlock()
	if i.expire(); closed() {
		t.Fatal("expired with a request in flight")
	}
	i.mu.Lock()
	i.active = 0
	i.last = time.Now()
	i.mu.Unlock()
	if i.expire(); closed() {
		t.Fatal("expired right after a request ended")
	}
	i.mu.Lock()
	i.last = time.Now().Add(-2 * time.Hour)
	i.mu.Unlock()
	if i.expire(); !closed() {
		t.Error("did not expire after the idle time")
	}
}
