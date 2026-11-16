//go:build windows

package link

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

// The system names three folders of application data: Go's own (Roaming, Local) and LocalLow, which Go does not name.
func TestAppDataKnown(t *testing.T) {
	t.Parallel()

	got := map[string]bool{}
	for _, d := range appDataDirs() {
		if d.err != nil {
			t.Fatal(d.err)
		}
		dir, err := files.Real(d.path)
		if err != nil {
			t.Fatal(err)
		}
		got[dir] = true
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	low := 0
	for dir := range got {
		if strings.HasSuffix(dir, `\AppData\LocalLow`) {
			low++
		}
	}
	for _, dir := range []string{cfg, cache} {
		if r, err := files.Real(dir); err != nil || !got[r] {
			t.Errorf("the folders of application data %v hold no %s (%v)", got, dir, err)
		}
	}
	if len(got) != 3 || low != 1 {
		t.Errorf("the folders of application data = %v, want three, one of them LocalLow", got)
	}
}

// A drive's root is too much to serve, the drive of home or another; the old check took \\?\C:\ for no root, and
// compared it with home as \\?\C:\\.
func TestExposedDir(t *testing.T) {
	t.Parallel()

	home := `\\?\C:\Users\u`
	for _, tt := range []struct {
		dir  string
		want bool
	}{
		{`\\?\D:\`, true},
		{`\\?\C:\`, true},
		{`\\?\C:\Users`, true},
		{home, true},
		{`\\?\D:\docs`, false},
		{`\\?\C:\Users\u\docs`, false},
		{`\\?\C:\Users\v`, false},
	} {
		if got := exposedDir(tt.dir, home); got != tt.want {
			t.Errorf("exposedDir(%s) = %v, want %v", tt.dir, got, tt.want)
		}
	}
}

// A folder of application data on a share (a roaming folder redirected to a server) is not on this machine's drives
// and holds no local path: left out, as one not there is, so that links on the drives are not all refused.
func TestExposesAppDataShare(t *testing.T) { //nolint:paralleltest // it swaps appData, which the other tests read
	tmp, err := files.Real(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(filepath.Join(home, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	shared, _ := testfs.MapShare(t, t.TempDir())
	was := appData
	t.Cleanup(func() { appData = was })
	appData = func() []appDir { return []appDir{{path: shared, err: nil}} }
	if err := exposes(filepath.Join(home, "docs", "a.html"), home, false); err != nil {
		t.Errorf("exposes with a folder of application data on a share = %v, want it allowed", err)
	}
}

// A home on a share is refused as not local, at home, not as a home that is not there.
func TestExposesHomeShare(t *testing.T) {
	t.Parallel()

	shared, _ := testfs.MapShare(t, t.TempDir())
	err := exposes(`C:\docs\a.html`, shared, false)
	if ue, ok := errors.AsType[*usererr.Error](err); !ok || ue.Key != "path_not_local" || ue.File() != shared {
		t.Errorf("exposes with home on a share = %v, want path_not_local at %s", err, shared)
	}
}
