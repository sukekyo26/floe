package files

import (
	"path/filepath"
	"testing"

	"floe/internal/testfs"
)

// walk tells a link whose target's drive is not there, so that realAboveWalked gives false without opening the drive
// (a network drive remembered but not connected would connect again). Opening it cannot be seen from here: a drive
// letter of none fails alike opened or not.
func TestWalkNoDrive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	testfs.Symlink(t, testfs.FreeDrive(t)+`\x`, filepath.Join(dir, "z"))
	testfs.Symlink(t, filepath.Join(dir, "gone"), filepath.Join(dir, "g"))
	for name, want := range map[string]bool{"z": true, "g": false} {
		w, err := walk(filepath.Join(dir, name, "new"), true)
		if err != nil || !w.dangling || w.noDrive != want {
			t.Errorf("walk of %s = %+v, %v; want dangling, noDrive %v", name, w, err, want)
		}
	}
}
