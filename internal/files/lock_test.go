package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
)

// While files.Lock holds a file, another handle of it cannot be locked; once what it returns is called, it can.
func TestLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".lock")
	open := func() *os.File {
		f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600) //nolint:gosec // G304: the test's own
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	held, other := open(), open()
	defer other.Close()
	unlock, err := files.Lock(held)
	if err != nil {
		t.Fatal(err)
	}
	if testfs.TryLock(t, other) {
		t.Fatal("another handle locked the file files.Lock holds")
	}
	unlock()
	if !testfs.TryLock(t, other) {
		t.Error("another handle could not lock the file after files.Lock let it go")
	}
}
