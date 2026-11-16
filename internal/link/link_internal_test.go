package link

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"floe/internal/files"
)

// A name whose long form cannot be told (Windows: a parent that cannot be listed) is neither served nor listed, though
// its real path could be: the dot rule cannot read it.
func TestWithinLongNameFails(t *testing.T) { //nolint:paralleltest // it swaps longName, which the other tests read
	folder, err := files.Real(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(folder, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "sub", "a.html"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := Resolve(folder, true, "sub/a.html"); !ok {
		t.Fatal("sub/a.html is not served before longName fails")
	}
	was := longName
	t.Cleanup(func() { longName = was })
	longName = func(string) (string, error) { return "", errors.New("cannot list") }
	if got, ok := Resolve(folder, true, "sub/a.html"); ok {
		t.Errorf("Resolve(sub/a.html) = %q with longName failing, want it refused", got)
	}
	if _, err := Dir(folder, "sub"); !errors.Is(err, ErrNoDir) {
		t.Errorf("Dir(sub) = %v with longName failing, want ErrNoDir", err)
	}
}
