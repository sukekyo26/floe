package css

import (
	"io/fs"
	"os"
	"path"
	"testing"
)

// sheets calls f with the name (under dir) and the text of each .css file under dir, in the order of their names,
// and fails when there is none.
func sheets(t *testing.T, dir string, f func(name string, src []byte)) {
	t.Helper()

	files := os.DirFS(dir)
	read := 0
	err := fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".css" {
			return err
		}
		src, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		read++
		f(name, src)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if read == 0 {
		t.Fatalf("no CSS found under %s", dir)
	}
}
