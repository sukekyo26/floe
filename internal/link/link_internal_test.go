package link

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/files"
	"floe/internal/usererr"
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

// The folders of application data are hidden wherever home is: what is one or in one is refused as hidden, a directory
// holding one as exposing too much (as home or a root), and a folder that cannot be told refuses all. Where they are
// is given, so that it runs on every system.
func TestExposesAppData(t *testing.T) { //nolint:paralleltest // it swaps appData, which the other tests read
	tmp, err := files.Real(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(tmp, "home")
	roam := filepath.Join(home, "data", "roam")
	away := filepath.Join(tmp, "away", "roam")
	for _, dir := range []string{roam, away, filepath.Join(home, "docs")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	was := appData
	t.Cleanup(func() { appData = was })
	appData = func() []appDir { return []appDir{{path: roam, err: nil}, {path: away, err: nil}} }
	root := filepath.VolumeName(tmp) + string(filepath.Separator)
	for _, tt := range []struct {
		path   string
		folder bool
		key    string
	}{
		{filepath.Join(roam, "x", "a.html"), false, "link_hidden_home"},
		{roam, true, "link_hidden_home"},
		{filepath.Join(home, "data"), true, "link_exposes_dir"},
		{filepath.Join(home, "data", "a.html"), false, "link_exposes_dir"},
		{filepath.Join(home, "docs", "a.html"), false, ""},
		{filepath.Join(home, "docs"), true, ""},
		// A real home holds its folders of application data: still exposing too much, not hidden.
		{home, true, "link_exposes_dir"},
		{filepath.Join(home, "a.html"), false, "link_exposes_dir"},
		{tmp, true, "link_exposes_dir"},
		// Out of home, as Git Bash's home leaves C:\Users\u\AppData.
		{filepath.Join(away, "x.html"), false, "link_hidden_home"},
		{filepath.Join(tmp, "away"), true, "link_exposes_dir"},
		// A file at the root: the root is served.
		{root + "a.html", false, "link_exposes_dir"},
	} {
		if got := keyOf(exposes(tt.path, home, tt.folder)); got != tt.key {
			t.Errorf("exposes(%s) = %q, want %q", tt.path, got, tt.key)
		}
	}
	appData = func() []appDir { return []appDir{{path: filepath.Join(tmp, "none"), err: nil}} }
	if err := exposes(filepath.Join(home, "docs", "a.html"), home, false); err != nil {
		t.Errorf("exposes with folders not there = %v, want it allowed", err)
	}
	for _, d := range []appDir{{path: "", err: errors.New("the system cannot tell")}, {path: "rel", err: nil}} {
		appData = func() []appDir { return []appDir{d} }
		if got := keyOf(exposes(filepath.Join(home, "docs", "a.html"), home, false)); got != "link_appdata_unknown" {
			t.Errorf("exposes with a folder %+v = %q, want link_appdata_unknown", d, got)
		}
	}
}

// keyOf is the key of the mistake err is, "" for none.
func keyOf(err error) string {
	if ue, ok := errors.AsType[*usererr.Error](err); ok {
		return ue.Key
	}
	return ""
}

// A home that is a root (HOME=/) still has its hidden directories right under it.
func TestExposesRootHome(t *testing.T) { //nolint:paralleltest // it swaps appData, which the other tests read
	tmp, err := files.Real(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	was := appData
	t.Cleanup(func() { appData = was })
	appData = func() []appDir { return nil }
	root := filepath.VolumeName(tmp) + string(filepath.Separator)
	if got := keyOf(exposes(filepath.Join(root, ".ssh", "x.html"), root, false)); got != "link_hidden_home" {
		t.Errorf("exposes(%s) with home %s = %q, want link_hidden_home", filepath.Join(root, ".ssh", "x.html"), root, got)
	}
}

// A directory whose name home's starts with is no directory home is under.
func TestExposedDirSibling(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	if exposedDir(filepath.Join(tmp, "floe"), filepath.Join(tmp, "floe-home")) {
		t.Errorf("exposedDir(floe) with home floe-home = true, want false")
	}
}
