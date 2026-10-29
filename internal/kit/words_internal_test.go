package kit

import (
	"io/fs"
	"maps"
	"path"
	"slices"
	"testing"
)

// Every words.yaml of floe's, found under floe/ rather than listed, has en and names in each language the words
// of en, which readWords leaves unchecked: a word a language lacks would stop only a page drawn in it.
func TestFloeWordsMatch(t *testing.T) {
	t.Parallel()

	var found int
	err := fs.WalkDir(floeFiles, "floe", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(name) != "words.yaml" {
			return err
		}
		found++
		src, err := floeFiles.ReadFile(name)
		if err != nil {
			return err
		}
		w, err := readWords(name, src)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return nil
		}
		en, ok := w["en"]
		if !ok {
			t.Errorf("%s has no en", name)
			return nil
		}
		want := slices.Sorted(maps.Keys(en))
		for tag, words := range w {
			if got := slices.Sorted(maps.Keys(words)); !slices.Equal(got, want) {
				t.Errorf("%s: %s names %v, en %v", name, tag, got, want)
			}
		}
		return nil
	})
	if err != nil || found == 0 {
		t.Fatalf("walking floe: %v, %d words.yaml", err, found)
	}
}
