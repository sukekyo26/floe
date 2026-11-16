//go:build unix

package files_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// What both open, a file or a directory, is opened without blocking (O_NONBLOCK), as files.Open opens it: that
// is what lets the open of a FIFO return.
func TestFSOpensWithoutBlocking(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	for via, fsys := range fsOf(t, dir) {
		for _, name := range []string{"a.yaml", "."} {
			f, err := fsys.Open(name)
			if err != nil {
				t.Fatalf("%s: Open(%s) = %v", via, name, err)
			}
			of, ok := f.(*os.File)
			if !ok {
				t.Fatalf("%s: Open(%s) = %T, want an *os.File", via, name, f)
			}
			// Not Fd, which makes the file blocking.
			var flags int
			var flagErr error
			rc, err := of.SyscallConn()
			if err == nil {
				err = rc.Control(func(fd uintptr) { flags, flagErr = unix.FcntlInt(fd, unix.F_GETFL, 0) })
			}
			if err = errors.Join(err, flagErr); err != nil {
				t.Fatal(err)
			}
			if flags&unix.O_NONBLOCK == 0 {
				t.Errorf("%s: Open(%s) opened it blocking", via, name)
			}
			_ = f.Close()
		}
	}
}
