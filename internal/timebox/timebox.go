// Package timebox is for the tests of a read that must not block: one that blocks fails its test in seconds,
// not at go test's timeout, which stops the whole package.
package timebox

import (
	"testing"
	"time"
)

// Within runs f, failing t when it has not returned in two seconds. f left blocked is left behind.
func Within(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s blocked", what)
	}
}
