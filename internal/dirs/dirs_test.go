package dirs_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/floe/internal/dirs"
)

// A directory not there yet is made and left empty; one floe cannot write in is refused, though it is there.
func TestCheckWritable(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "new", "dir")
	if err := dirs.CheckWritable(dir, ".check-"); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Errorf("CheckWritable left %v in %s, %v", entries, dir, err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes anywhere")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if err := dirs.CheckWritable(dir, ".check-"); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("CheckWritable of a directory floe cannot write in = %v", err)
	}
}
