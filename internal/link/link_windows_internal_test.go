//go:build windows

package link

import (
	"os"
	"strings"
	"testing"

	"floe/internal/files"
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
