package kit_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/testfs"
	"floe/internal/timebox"
	"floe/internal/usererr"
)

// fifoAfterStat is an fs.FS whose Stat makes the file at path a FIFO after looking at it.
type fifoAfterStat struct {
	fs.StatFS
	path string
}

func (f fifoAfterStat) Stat(name string) (fs.FileInfo, error) {
	fi, err := f.StatFS.Stat(name)
	if rmErr := os.Remove(f.path); rmErr != nil {
		return nil, rmErr //nolint:wrapcheck // the test's
	}
	if mkErr := testfs.MakeFIFO(f.path); mkErr != nil {
		return nil, mkErr //nolint:wrapcheck // the test's
	}
	return fi, err //nolint:wrapcheck // as the FS says
}

// An image made a FIFO between the Stat that finds it a regular file and the read is told as no image, as one
// that is a FIFO when looked at: through a data file's directory (files.Root) and a template's own (files.DirFS).
func TestImageMadeFIFO(t *testing.T) {
	t.Parallel()
	testfs.UnixOnly(t, "a FIFO")

	for _, via := range []string{"Root", "DirFS"} {
		dir := t.TempDir()
		path := filepath.Join(dir, "a.png")
		if err := os.WriteFile(path, []byte("\x89PNG"), 0o600); err != nil {
			t.Fatal(err)
		}
		of := files.DirFS(dir)
		if via == "Root" {
			root := files.NewRoot(dir)
			t.Cleanup(func() { _ = root.Close() })
			of = root
		}
		fsys, ok := of.(fs.StatFS)
		if !ok {
			t.Fatalf("%s is no fs.StatFS", via)
		}
		timebox.Within(t, via, func() {
			_, _, err := kit.ImageFile(fifoAfterStat{StatFS: fsys, path: path}, "a.png", dir, "d.yaml")
			var e *usererr.Error
			if !errors.As(err, &e) || e.Key != "kit_image_not_file" {
				t.Errorf("%s: ImageFile of a file made a FIFO = %v, want kit_image_not_file", via, err)
			}
		})
	}
}
