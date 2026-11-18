package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

// pathNotFound is read as Go reads Windows' ERROR_PATH_NOT_FOUND: a file on the way and a directory not there alike.
type pathNotFound struct{}

func (pathNotFound) Error() string { return "the system cannot find the path specified" }

func (pathNotFound) Is(target error) bool {
	return target == syscall.ENOTDIR || target == fs.ErrNotExist
}

// notDirWith looks up from the directory above from when err is from's own (dataDir's Stat, icons' OpenRoot), from
// itself when err is of a name under it (a file read in it); at the root it looks at it once and ends.
func TestNotDirFromNamed(t *testing.T) {
	t.Parallel()

	root := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	base := filepath.Join(root, "s394", "a")
	from := filepath.Join(base, "b")
	for _, tt := range []struct {
		name   string
		from   string
		err    error
		disk   map[string]bool // there: true a directory, false a file; missing, not there
		looked []string
		want   error // nil, or the path error told
	}{
		{
			"its own, above a directory", from, &fs.PathError{Op: "stat", Path: from, Err: pathNotFound{}},
			map[string]bool{base: true},
			[]string{base},
			nil,
		},
		{
			"its own, above a file", from, &fs.PathError{Op: "stat", Path: from, Err: pathNotFound{}},
			map[string]bool{base: false},
			[]string{base},
			&fs.PathError{Op: "stat", Path: from, Err: ErrNotDir},
		},
		{
			"its own, of OpenRoot", from, &fs.PathError{Op: "open", Path: from, Err: pathNotFound{}},
			map[string]bool{filepath.Dir(base): false},
			[]string{base, filepath.Dir(base)},
			&fs.PathError{Op: "open", Path: from, Err: ErrNotDir},
		},
		{
			"of a file in it", from, &fs.PathError{Op: "open", Path: filepath.Join(from, "index.json"), Err: pathNotFound{}},
			map[string]bool{base: true},
			[]string{from, base},
			nil,
		},
		{
			"the root's own", root, &fs.PathError{Op: "stat", Path: root, Err: pathNotFound{}},
			map[string]bool{},
			[]string{root},
			nil,
		},
	} {
		var looked []string
		stat := func(path string) (fs.FileInfo, error) {
			looked = append(looked, path)
			dir, there := tt.disk[path]
			switch {
			case !there:
				return nil, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist}
			case dir:
				return Stat(os.TempDir())
			}
			return fileInfo(t), nil
		}
		got := notDirWith(tt.from, tt.err, stat)
		if !slices.Equal(looked, tt.looked) {
			t.Errorf("%s: looked at %v, want %v", tt.name, looked, tt.looked)
		}
		var pe *fs.PathError
		switch {
		case tt.want == nil && got != nil:
			t.Errorf("%s: notDirWith = %v, want nil", tt.name, got)
		case tt.want != nil && (!errors.As(got, &pe) || pe.Error() != tt.want.Error() || !errors.Is(got, ErrNotDir)):
			t.Errorf("%s: notDirWith = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// fileInfo is the FileInfo of a file, no directory.
func fileInfo(t *testing.T) fs.FileInfo {
	t.Helper()
	f := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	return fi
}
