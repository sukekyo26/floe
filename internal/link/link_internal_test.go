package link

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// A name whose long form has a dot name (Windows: SECRET~1 is .secret), or leads out of the folder, is neither served
// nor listed, though the name as written has none: the dot rule reads the long form too, on every system.
func TestWithinLongNameDotted(t *testing.T) { //nolint:paralleltest // it swaps longName, which the other tests read
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
	was := longName
	t.Cleanup(func() { longName = was })
	for _, long := range []string{".secret", filepath.Join("..", "out")} {
		// sub as long, under the folder: its first name made long, the rest as written.
		longName = func(p string) (string, error) {
			rel, err := filepath.Rel(folder, p)
			if err != nil {
				return "", err //nolint:wrapcheck // the test's failure
			}
			return filepath.Join(folder, long, strings.TrimPrefix(rel, "sub")), nil
		}
		if got, ok := Resolve(folder, true, "sub/a.html"); ok {
			t.Errorf("Resolve(sub/a.html) = %q with sub long as %s, want it refused", got, long)
		}
		if _, err := Dir(folder, "sub"); !errors.Is(err, ErrNoDir) {
			t.Errorf("Dir(sub) = %v with sub long as %s, want ErrNoDir", err, long)
		}
	}
}
