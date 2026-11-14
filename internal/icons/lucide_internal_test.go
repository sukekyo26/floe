package icons

import (
	"bytes"
	"errors"
	"flag"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"floe/internal/usererr"
)

var (
	updateLucide = flag.Bool("update", false, "write internal/icons/lucide anew from -lucide-zip")
	lucideZip    = flag.String("lucide-zip", "", "the lucide-icons-<version>.zip of Lucide's release, for -update")
)

// colourRe finds a colour an icon writes; a built-in icon is painted through a mask, so it may write none but
// currentColor and none (a colour of its own would be painted over).
var colourRe = regexp.MustCompile(`(?:fill|stroke|color)\s*[=:]\s*"?([^";\s]+)`)

// The built-in icons are those lucidePick names, each an SVG of lines in currentColor, with the index a fetch
// writes and Lucide's LICENSE. With -update, they are written anew from -lucide-zip, as a fetch extracts a set.
func TestBuiltinLucide(t *testing.T) {
	if *updateLucide {
		writeLucide(t)
		return
	}
	idx, err := builtinIndex(&lucide)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := slices.Sorted(maps.Keys(idx.Icons)), slices.Sorted(maps.Keys(lucidePick)); !slices.Equal(got, want) {
		t.Errorf("the built-in index holds %v, want lucidePick's %v", got, want)
	}
	if want := []string{"lucide-icons-" + lucideVersion + ".zip"}; idx.Version != lucideVersion || !slices.Equal(idx.Sources, want) {
		t.Errorf("the built-in index is of %q from %v, want %q from %v", idx.Version, idx.Sources, lucideVersion, want)
	}
	for name, e := range idx.Icons {
		if e.Category != lucidePick[name] {
			t.Errorf("%s: category %q, want %q", name, e.Category, lucidePick[name])
		}
	}
	entries, err := fs.ReadDir(builtinFS, "lucide")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == "index.json" || name == "LICENSE" {
			continue
		}
		icon, isSVG := strings.CutSuffix(name, ".svg")
		if _, ok := lucidePick[icon]; !ok || !isSVG {
			t.Errorf("lucide/%s is no icon of lucidePick", name)
			continue
		}
		b, err := fs.ReadFile(builtinFS, "lucide/"+name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(b, []byte(`stroke="currentColor"`)) {
			t.Errorf("lucide/%s draws no lines in currentColor", name)
		}
		for _, m := range colourRe.FindAllSubmatch(b, -1) {
			if c := string(m[1]); c != "currentColor" && c != "none" {
				t.Errorf("lucide/%s writes the colour %q", name, c)
			}
		}
	}
	license, err := fs.ReadFile(builtinFS, "lucide/LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ISC License", "Lucide Icons and Contributors", "derived from the Feather project", "Cole Bemis"} {
		if !bytes.Contains(license, []byte(want)) {
			t.Errorf("lucide/LICENSE lacks %q", want)
		}
	}
	for word, names := range lucide.short {
		for _, n := range names {
			if _, ok := lucidePick[n]; !ok {
				t.Errorf("the short name %q means %q, no built-in icon", word, n)
			}
		}
	}
}

// writeLucide writes internal/icons/lucide anew from the zip -lucide-zip names, keeping its LICENSE, which the zip
// holds none of: the icons as a fetch extracts them, and the index of the release, not of the zip's path.
func writeLucide(t *testing.T) {
	t.Helper()
	zipPath := *lucideZip
	if want := "lucide-icons-" + lucideVersion + ".zip"; filepath.Base(zipPath) != want {
		t.Fatalf("-lucide-zip %q: pass %s, of the release lucideVersion names", zipPath, want)
	}
	dir := "lucide"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "LICENSE" {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	zr, closer, err := lucide.open(t.Context(), Source{Name: zipPath, At: zipPath})
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close() //nolint:errcheck // read only
	idx, err := lucide.extract([]opened{{from: Source{Name: zipPath, At: zipPath}, zr: zr}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	idx.Version, idx.Sources = lucideVersion, []string{filepath.Base(zipPath)}
	if err := writeIndex(idx, dir); err != nil {
		t.Fatal(err)
	}
	// The build embeds what was there when it began, so what was written is checked by the next run.
	t.Logf("wrote %d icons into %s; run the test again without -update to check them", len(idx.Icons), dir)
}

// A built-in file that cannot be read is floe's failure, not a mistake to mend by a fetch, which is refused.
func TestBuiltinBroken(t *testing.T) { //nolint:paralleltest // it swaps builtinFS
	saved := builtinFS
	t.Cleanup(func() { builtinFS = saved })
	for what, fsys := range map[string]fs.FS{
		"no index":      fstest.MapFS{},
		"a broken one":  fstest.MapFS{"lucide/index.json": {Data: []byte("{")}},
		"no icon there": fstest.MapFS{"lucide/index.json": {Data: []byte(`{"version": "1", "icons": {"users": {}}}`)}},
	} {
		builtinFS = fsys
		_, err := New(t.TempDir(), false, "").Lookup("lucide/users")
		var e *usererr.Error
		if err == nil || errors.As(err, &e) {
			t.Errorf("%s: Lookup lucide/users = %v, want floe's failure", what, err)
		}
	}
}
