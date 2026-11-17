package icons_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/icons"
)

// What Windows does where icons reads a path under a file as not there (W12, files.NotDir's LIMIT): the answers of
// now, the twins of the tests that are Unix only, so that mending one of them is noticed here.
func TestW12OnWindows(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	setAFile := t.TempDir()
	if err := os.WriteFile(filepath.Join(setAFile, "aws"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	under := filepath.Join(file, "x")
	// The set's directory a file, the root a file, and the root under one: each read as a set not fetched.
	for _, root := range []string{setAFile, file, under} {
		_, err := icons.New(root, false, "").Search("aws", []string{"x"})
		if got := told(err); !strings.HasPrefix(got, "floe: AWS icons are not in "+filepath.Join(root, "aws")+";") {
			t.Errorf("Search in %s = %s, want icons_not_fetched", root, got)
		}
	}
	// A fetch into a root under a file goes to the network (the context is done, so it goes no further).
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := icons.New(under, false, "").Fetch(done, "aws", nil, false); !strings.Contains(told(err), "cannot be downloaded (context canceled)") {
		t.Errorf("a fetch into a root under a file = %s, want icons_download", told(err))
	}
}
