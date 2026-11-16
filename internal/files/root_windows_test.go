package files_test

import (
	"testing"
	"testing/fstest"

	"floe/internal/files"
)

// osNameCases are the names Windows reads otherwise: a \ as a separator, a stream after :, a drive, a device.
var osNameCases = []nameCase{ //nolint:gochecknoglobals // a test's table
	{`sub\b.png`, false},
	{"a.png:x", false},
	{"a.png::$DATA", false},
	{"C:x", false},
	{"NUL", false},
	{"sub/CON", false},
	{"COM1.png", true},
}

// Under a directory named as files.Real names it (\\?\C:\…), where Windows reads no / as a separator, both keep
// to what an fs.FS promises too: the names are put as the system's.
func TestFSContractVerbatim(t *testing.T) {
	t.Parallel()

	verbatim, err := files.Real(contractTree(t))
	if err != nil {
		t.Fatal(err)
	}
	for via, fsys := range fsOf(t, verbatim) {
		if err := fstest.TestFS(fsys, "a.yaml", "sub/b.png"); err != nil {
			t.Errorf("%s of %s: %v", via, verbatim, err)
		}
	}
}
