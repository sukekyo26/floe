//go:build unix

package cli_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// floe part new makes the part as the umask says, its directory as its files.
func TestNewUmask(t *testing.T) { //nolint:paralleltest // sets the umask of the process
	for _, tt := range []struct{ umask, dir, file os.FileMode }{{0o022, 0o755, 0o644}, {0o077, 0o700, 0o600}} {
		old := syscall.Umask(int(tt.umask))
		home := t.TempDir()
		code, _, stderr := runEnv(home, map[string]string{"HOME": home}, "part", "new", "mine")
		syscall.Umask(old)
		if code != 0 {
			t.Fatalf("floe part new mine = %d, %q", code, stderr)
		}
		at := filepath.Join(home, ".config/floe/parts/mine")
		for path, mode := range map[string]os.FileMode{at: tt.dir, filepath.Join(at, "part.html"): tt.file} {
			if info, err := os.Stat(path); err != nil || info.Mode().Perm() != mode {
				t.Errorf("umask %o: %s: %v, %v; want mode %o", tt.umask, path, info, err, mode)
			}
		}
	}
}
