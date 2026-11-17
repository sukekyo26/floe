package testfs

import "testing"

// UnixOnly skips on Windows, and on Unix lets the test run on.
func TestUnixOnly(t *testing.T) { //nolint:tparallel // the case is read after it ran
	t.Parallel()

	ran := false
	//nolint:paralleltest // as above
	t.Run("marked", func(t *testing.T) {
		UnixOnly(t, "a test of the mark")
		ran = true
	})
	if ran == isWindows {
		t.Errorf("ran on to its end = %v, on Windows = %v", ran, isWindows)
	}
}
