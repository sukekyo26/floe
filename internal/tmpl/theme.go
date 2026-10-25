package tmpl

import (
	"embed"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"

	"github.com/sukekyo26/floe/internal/usererr"
)

// themeFiles holds a directory per theme of floe's: its palette.css, its base.css and the style of each part
// it restyles.
//
//go:embed themes
var themeFiles embed.FS

// floeThemesDir is the directory of themeFiles that holds floe's themes.
const floeThemesDir = "themes"

// themeBase and themePalette are the files of a theme that are no part's, for every page: base.css,
// and palette.css, the colours and faces under the template's own, which default's fills in.
const (
	themeBase    = "base"
	themePalette = "palette"
)

// themeNames returns the themes of set, by name, those that cannot be used among them.
func (set *partSet) themeNames() []string {
	return slices.Sorted(slices.Values(slices.Concat(slices.Collect(maps.Keys(set.themes)),
		slices.Collect(maps.Keys(set.broken[kindTheme])))))
}

// themesWith returns the themes of set that can be used with a file of the part name, by name.
func (set *partSet) themesWith(name string) []string {
	var themes []string
	for _, theme := range slices.Sorted(maps.Keys(set.themes)) {
		if _, ok := set.themes[theme].parts[name]; ok {
			themes = append(themes, theme)
		}
	}
	return themes
}

// theme is a theme: its base.css and palette.css, for every page, and the style of each part it restyles,
// which reaches every element that has the part's look. Its palettes are floe's and, over it, one's own of
// its name: a palette of one's own changes the variables it writes, the theme's own names among them.
type theme struct {
	base     string
	palettes []string
	parts    map[string]string // by the part's name
}

func newTheme() *theme {
	return &theme{base: "", palettes: nil, parts: map[string]string{}}
}

// readTheme reads the theme in files, its directory, shown in its mistakes as dir, over t: each file
// takes the place of t's of its name, but palette.css, which goes over t's. A file named for none of
// base.css, palette.css, a part or a look of set is warned of, as it goes into no page, and so is a
// file but palette.css that writes a colour or sets a variable, as colourWarnings tells.
func (set *partSet) readTheme(files fs.FS, dir string, t *theme) error {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return usererr.CannotRead(dir, err)
	}
	// The mistakes of its files are told together, so that floe part check tells them all at once.
	var found usererr.List
	// Warned of once the theme is read whole: one that cannot be used goes into no page.
	var warnings []*usererr.Error
	for _, e := range entries {
		name, isCSS := strings.CutSuffix(e.Name(), ".css")
		// A file of another kind, or of a name starting with a dot (.DS_Store), is none of the theme's.
		if !isCSS || e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if !set.themeFile(name) {
			// Warned of, the theme used all the same: a file of a part since removed would stop every page.
			set.strays = append(set.strays, usererr.NewAt(path.Join(dir, e.Name()), 0, 0, "tmpl_theme_file",
				set.names(nil)))
			continue
		}
		more, err := readThemeFile(files, name, path.Join(dir, e.Name()), t)
		if failed := found.Add(err); failed != nil {
			return failed //nolint:wrapcheck // what failed, as it is
		}
		warnings = append(warnings, more...)
	}
	if found != nil {
		return found
	}
	set.strays = append(set.strays, warnings...)
	return nil
}

// readThemeFile reads name.css of the theme t in files, shown in its mistakes as file, into t, and returns
// the warnings of the colours it sets.
func readThemeFile(files fs.FS, name, file string, t *theme) ([]*usererr.Error, error) {
	css, err := fs.ReadFile(files, name+".css")
	if err != nil {
		return nil, usererr.CannotRead(file, err)
	}
	sheet := readCSS(css)
	if err = sheet.refuse(file); err != nil {
		return nil, err
	}
	if name == themePalette {
		if err = checkPalette(file, sheet); err != nil {
			return nil, err
		}
		t.palettes = append(t.palettes, string(css))
		return nil, nil
	}
	if name == themeBase {
		t.base = string(css)
	} else {
		t.parts[name] = string(css)
	}
	return colourWarnings(file, sheet, false), nil
}

// themeFile reports whether name.css is a file of a theme: its base.css, its palette.css or the style of a part,
// one of one's own that cannot be used among them, but not one of a name no part can take.
func (set *partSet) themeFile(name string) bool {
	_, isPart := set.parts[name]
	isOwn := set.brokenNamed(name)
	return name == themeBase || name == themePalette || isPart || isOwn
}

// checkPalette refuses s, palette.css shown as file, but for the variables (and color-scheme) set on :root:
// in the bottom layer any other rule would lose to every other one, and !important would win over every
// layer, the template's too.
func checkPalette(file string, s *cssSheet) error {
	p := &paletteCheck{file: file, sheet: s}
	if err := p.onlyBlocks(0, len(s.tokens), 0); err != nil {
		return err
	}
	for i := range s.rules {
		r := &s.rules[i]
		if !r.block || r.depth != 0 || r.prelude != ":root" {
			return p.mistake("tmpl_palette_rule", r.at)
		}
		if err := p.declarations(r); err != nil {
			return err
		}
	}
	return nil
}

// paletteCheck is what checkPalette reads of palette.css.
type paletteCheck struct {
	file  string
	sheet *cssSheet
}

// onlyBlocks refuses a token of the file from from to to outside the blocks at depth, from the start of
// each prelude to its }, but spaces and comments: a declaration or a statement where only blocks go.
func (p *paletteCheck) onlyBlocks(from, to, depth int) error {
	i := from
	for _, r := range p.sheet.rules {
		if !r.block || r.depth != depth || r.first < from || r.first >= to {
			continue
		}
		if err := p.blankFrom(i, r.first); err != nil {
			return err
		}
		i = r.end + 1
	}
	return p.blankFrom(i, to)
}

// blankFrom refuses a token from from to to that is no space nor comment.
func (p *paletteCheck) blankFrom(from, to int) error {
	for _, t := range p.sheet.tokens[min(from, len(p.sheet.tokens)):min(to, len(p.sheet.tokens))] {
		if t.typ != css.WhitespaceToken && t.typ != css.CommentToken {
			return p.mistake("tmpl_palette_rule", t.at)
		}
	}
	return nil
}

// declarations refuses a declaration of the block r that is no variable nor color-scheme, and !important.
func (p *paletteCheck) declarations(r *cssRule) error {
	for _, d := range r.decls {
		switch {
		case !strings.HasPrefix(d.name, "--") && d.name != "color-scheme" || d.value == "":
			return p.mistake("tmpl_palette_rule", d.at)
		case d.important:
			return p.mistake("tmpl_palette_important", d.at)
		}
	}
	return nil
}

// mistake is the mistake key of palette.css at the offset at.
func (p *paletteCheck) mistake(key string, at int) error {
	return usererr.NewAt(p.file, lineAt(p.sheet.src, at), 0, key)
}
