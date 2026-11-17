package files

import "testing"

// A mark is made of what was told: none told leaves its field empty, never the whole mark -, and a field not told
// differs from one told as 0.
func TestStampOf(t *testing.T) {
	t.Parallel()

	all := fields{dir: false, attrs: 0x20, id: "1:2", size: 3, write: 4, change: 5, hasChange: true}
	noChange := all
	noChange.change, noChange.hasChange = 0, false
	zeroChange := all
	zeroChange.change = 0
	noID := all
	noID.id = ""
	seen := map[string]string{}
	for name, f := range map[string]fields{"all": all, "no change time": noChange, "a change time of 0": zeroChange, "no ID": noID} {
		got := stampOf(f)
		if got == "-" || got == "" {
			t.Errorf("%s: stampOf = %q", name, got)
		}
		if other, ok := seen[got]; ok {
			t.Errorf("%s and %s are both marked %q", name, other, got)
		}
		seen[got] = name
	}
	dir := fields{dir: true, attrs: 0x10, id: "1:2", size: 3, write: 4, change: 5, hasChange: true}
	later := dir
	later.write, later.change = 6, 7
	if stampOf(dir) != stampOf(later) {
		t.Error("a directory's mark changed with its times")
	}
}
