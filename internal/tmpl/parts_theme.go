package tmpl

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// themesDir is the directory of parts/ that holds the parts themes, and so is no part.
const themesDir = "themes"

// themeBase is the file of a parts theme that is no part's, base.css: for every page.
const themeBase = "base"

// themeNames returns the parts themes of set, by name, those that cannot be used among them.
func (set *partSet) themeNames() []string {
	return slices.Sorted(slices.Values(slices.Concat(slices.Collect(maps.Keys(set.themes)),
		slices.Collect(maps.Keys(set.brokenThemes)))))
}

// partsTheme is a parts theme: its base.css, for every page, and the style of each part it restyles,
// which reaches every element that has the part's look.
type partsTheme struct {
	base  string
	parts map[string]string // by the part's name
}

func newPartsTheme() *partsTheme {
	return &partsTheme{base: "", parts: map[string]string{}}
}

// readPartsTheme reads the parts theme in files, its directory, shown in its mistakes as dir, over t:
// each file takes the place of t's of its name. A file named for neither base.css, parts.css nor a part
// of set is refused, as it would go into no page.
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
				set.names()))
			continue
		}
		css, err := fs.ReadFile(files, e.Name())
		if err != nil {
			return fmt.Errorf("read the parts theme %s: %w", dir, err)
		}
		// base.css is for the whole page, out of the parts' scope.
		if err = refuseCSS(path.Join(dir, e.Name()), css, name != themeBase); err != nil {
			return err
		}
		if name == themeBase {
			t.base = string(css)
		} else {
			t.parts[name] = string(css)
		}
	}
	return nil
}

// themeFile reports whether name.css is a file of a parts theme: its base.css or the style of a part,
// one of one's own that cannot be used among them, but not one of a name no part can take.
func (set *partSet) themeFile(name string) bool {
	_, isPart := set.parts[name]
	isOwn := set.broken[name] != nil && validName(name) && !set.reserved(name)
	return name == themeBase || isPart || isOwn
}

// all is the whole CSS of t, its base.css first, then the parts' in the order of their names.
func (t *partsTheme) all() string {
	css := t.base
	for _, name := range slices.Sorted(maps.Keys(t.parts)) {
		css += t.parts[name]
	}
	return css
}

// themeVariables returns the CSS variables the parts themes of set that can be used take from the page,
// those a theme uses and does not define itself, once each, in the order of the themes' names.
func (set *partSet) themeVariables() []string {
	var vars []string
	for _, name := range slices.Sorted(maps.Keys(set.themes)) {
		css := []byte(set.themes[name].all())
		defined := definedVariables(css)
		for _, v := range appendFound(nil, usedVariable, string(css)) {
			if !defined[v] && !slices.Contains(vars, v) {
				vars = append(vars, v)
			}
		}
	}
	return vars
}

var usedVariable = regexp.MustCompile(`var\(\s*(--[\w-]+)`)
