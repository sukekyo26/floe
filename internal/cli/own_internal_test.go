package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"floe/internal/files"
	"floe/internal/usererr"
)

// A default place is compared as dataDir saw it, not looked at again; a setting, which dataDir does not look at, is
// looked at by compared.
func TestComparedUsesSeen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	seenDir, err := files.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "gone")
	place := func(at string, set bool, seen fs.FileInfo) dataPlace {
		return dataPlace{dir: at, key: "FLOE_PARTS_DIR", set: set, from: "", unusable: nil, seen: seen}
	}
	for _, tt := range []struct {
		name string
		p    dataPlace
		want bool
	}{
		{"a default seen a directory, gone since", place(gone, false, seenDir), true},
		{"a default not seen", place(dir, false, nil), false},
		{"none", place("", false, nil), false},
		{"a setting not there", place(gone, true, nil), true},
		{"a setting a file", place(file, true, nil), false},
		{"a setting a directory", place(dir, true, nil), true},
	} {
		if got := compared(tt.p); got != tt.want {
			t.Errorf("%s: compared = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// ownPlaces takes the place it is given for its kind, made once, and makes the others.
func TestOwnPlacesTakesMade(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	vars := homeVars(home)
	a := &app{
		env:      Env{Stdout: nil, Stderr: nil, Dir: home, Getenv: func(key string) string { return vars[key] }},
		out:      nil,
		warnings: nil,
		told:     nil,
	}
	made := ownAt{kind: ownThemes, p: dataPlace{dir: "made", key: "", set: false, from: "", unusable: nil, seen: nil}, err: nil}
	places := a.ownPlaces(true, &made)
	kinds := make([]string, 0, len(places))
	for _, o := range places {
		kinds = append(kinds, o.kind.name)
	}
	if len(places) != 3 || places[1].p.dir != "made" || places[0].p.dir != defaultPlace(home, "parts") ||
		places[2].p.dir != defaultPlace(home, "templates") {
		t.Errorf("ownPlaces(true, themes made) = %v of %v, want parts, made, templates", places, kinds)
	}
	if got := a.ownPlaces(false, nil); len(got) != 2 {
		t.Errorf("ownPlaces(false, nil) has %d places, want parts and themes", len(got))
	}
}

// The overlap of places looks at each directory of a place once, when first needed, and none it need not: the counts
// are of each case's own shape.
func TestCompareLooks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, d := range []string{"a", "b", "same", "p/th"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	resolved := func(name string) string {
		r, ok := files.RealAbove(filepath.Join(root, name))
		if !ok {
			t.Fatalf("no real path of %s", name)
		}
		return r
	}
	compareOf := func(names ...string) (looks, overlaps, levels int) {
		var counted int
		stat := func(path string) (fs.FileInfo, error) {
			counted++
			return files.Stat(path) //nolint:wrapcheck // the test's
		}
		spots := make([]spot, len(names))
		towers := make([]*tower, len(names))
		for i, n := range names {
			p := dataPlace{dir: n, key: "K" + n, set: true, from: "", unusable: nil, seen: nil}
			spots[i] = spot{p: p, at: n, kind: "kind" + string(rune('0'+i))}
			towers[i] = newTower(resolved(n), stat)
		}
		var found usererr.List
		compare(&found, spots, towers, func(int, int) bool { return true })
		return counted, len(found), len(towers[0].dirs)
	}

	if looks, overlaps, _ := compareOf("a"); looks != 0 || overlaps != 0 {
		t.Errorf("one place: %d looks, %d overlaps; want 0, 0", looks, overlaps)
	}
	unlooked := func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
	la := len(newTower(resolved("a"), unlooked).dirs)
	lb := len(newTower(resolved("b"), unlooked).dirs)
	// Each directory of each place once: within(a, b) looks at all of a's and b's own, within(b, a) at b's above.
	if looks, overlaps, _ := compareOf("a", "b"); looks != la+lb || overlaps != 0 {
		t.Errorf("two apart: %d looks, %d overlaps; want %d, 0", looks, overlaps, la+lb)
	}
	// One directory three times: told by the letters, before any look.
	if looks, overlaps, _ := compareOf("same", "same", "same"); looks != 0 || overlaps != 3 {
		t.Errorf("one directory thrice: %d looks, %d overlaps; want 0, 3", looks, overlaps)
	}
	// The outer first: within(p, p/th) looks at all of p's and at p/th, within(p/th, p) at none more.
	if looks, overlaps, l := compareOf("p", "p/th"); looks != l+1 || overlaps != 1 {
		t.Errorf("one in the other: %d looks, %d overlaps; want %d, 1", looks, overlaps, l+1)
	}
}
