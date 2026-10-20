package tmpl

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// themesDir is the directory of parts/ that holds the parts themes, and so is no part.
const themesDir = "themes"

// themeBase and themePalette are the files of a parts theme that are no part's, for every page: base.css,
// and palette.css, the colours and faces under the template's own, which default's fills in.
const (
	themeBase    = "base"
	themePalette = "palette"
)

// themeNames returns the parts themes of set, by name, those that cannot be used among them.
func (set *partSet) themeNames() []string {
	return slices.Sorted(slices.Values(slices.Concat(slices.Collect(maps.Keys(set.themes)),
		slices.Collect(maps.Keys(set.brokenThemes)))))
}

// themesWith returns the parts themes of set that can be used with a file of the part name, by name.
func (set *partSet) themesWith(name string) []string {
	var themes []string
	for _, theme := range slices.Sorted(maps.Keys(set.themes)) {
		if _, ok := set.themes[theme].parts[name]; ok {
			themes = append(themes, theme)
		}
	}
	return themes
}

// partsTheme is a parts theme: its base.css and palette.css, for every page, and the style of each part
// it restyles, which reaches every element that has the part's look.
type partsTheme struct {
	base    string
	palette string
	parts   map[string]string // by the part's name
}

func newPartsTheme() *partsTheme {
	return &partsTheme{base: "", palette: "", parts: map[string]string{}}
}

// readPartsTheme reads the parts theme in files, its directory, shown in its mistakes as dir, over t:
// each file takes the place of t's of its name. A file named for neither base.css nor a part or a look of
// set is warned of, as it goes into no page.
func (set *partSet) readPartsTheme(files fs.FS, dir string, t *partsTheme) error {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("read the parts theme %s: %w", dir, err)
	}
	for _, e := range entries {
		name, isCSS := strings.CutSuffix(e.Name(), ".css")
		// A file of another kind, or of a name starting with a dot (.DS_Store), is none of the theme's.
		if !isCSS || e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if !set.themeFile(name) {
			// Warned of, the theme used all the same: a file of a part since removed would stop every page.
			set.strays = append(set.strays, usererr.New("tmpl_parts_theme_file", path.Join(dir, e.Name()),
				set.names(nil)))
			continue
		}
		css, err := fs.ReadFile(files, e.Name())
		if err != nil {
			return fmt.Errorf("read the parts theme %s: %w", dir, err)
		}
		// base.css and palette.css are for the whole page, out of the parts' scope.
		if err = refuseCSS(path.Join(dir, e.Name()), css, name != themeBase && name != themePalette); err != nil {
			return err
		}
		switch name {
		case themeBase:
			t.base = string(css)
		case themePalette:
			t.palette = string(css)
		default:
			t.parts[name] = string(css)
		}
	}
	return nil
}

// themeFile reports whether name.css is a file of a parts theme: its base.css, its palette.css or the style of a part,
// one of one's own that cannot be used among them, but not one of a name no part can take.
func (set *partSet) themeFile(name string) bool {
	_, isPart := set.parts[name]
	isOwn := set.broken[name] != nil && validName(name) && !set.reserved(name)
	return name == themeBase || name == themePalette || isPart || isOwn
}
