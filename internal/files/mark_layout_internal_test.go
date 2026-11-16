//go:build windows

package files

import (
	"testing"
	"unsafe"
)

// The copies of FILE_BASIC_INFO and FILE_ID_INFO are laid out as Windows lays them out, and a file there is marked.
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
	if got := Mark(t.TempDir()); got == "-" {
		t.Error("a directory there is marked -")
	}
}
