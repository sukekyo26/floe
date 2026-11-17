//go:build windows

package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The copies of FILE_BASIC_INFO and FILE_ID_INFO are laid out as Windows lays them out: a file just written has a
// change time by its last write time, read through the copy, and a 128-bit ID; it is marked, and so is a directory.
func TestMarkFallbacks(t *testing.T) {
	t.Parallel()

	if got := unsafe.Sizeof(fileBasicInfo{}); got != 40 {
		t.Errorf("FILE_BASIC_INFO is %d bytes, want 40", got)
	}
	if got := unsafe.Offsetof(fileBasicInfo{}.ChangedTime); got != 24 {
		t.Errorf("FILE_BASIC_INFO's ChangedTime is at %d, want 24", got)
	}
	if got := unsafe.Sizeof(fileIDInfo{}); got != 24 {
		t.Errorf("FILE_ID_INFO is %d bytes, want 24", got)
	}
	path := filepath.Join(t.TempDir(), "a")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, ok := statOf(checkedOf(t, path), 0)
	if !ok || !f.hasChange {
		t.Fatalf("a file just written is told %+v, %v: want its change time", f, ok)
	}
	//nolint:gosec // G115: a FILETIME's halves
	change := windows.Filetime{LowDateTime: uint32(f.change), HighDateTime: uint32(f.change >> 32)}
	if d := time.Duration(change.Nanoseconds() - f.write); d.Abs() > 2*time.Second {
		t.Errorf("a file just written changed %v from its last write: want them together", d)
	}
	if want := len("0:") + 32; len(f.id) < want {
		t.Errorf("a file's ID is %q, want a volume and 128 bits", f.id)
	}
	for _, p := range []string{path, filepath.Dir(path)} {
		if got := Mark(p); got == "-" {
			t.Errorf("%s is marked -", p)
		}
	}
}
