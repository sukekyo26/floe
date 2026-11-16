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
