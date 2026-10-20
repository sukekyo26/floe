package tmpl

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"

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
// each file takes the place of t's of its name. A file named for none of base.css, palette.css, a part
// or a look of set is warned of, as it goes into no page, and so is a palette that sets a colour for light
// mode alone.
func (set *partSet) readPartsTheme(files fs.FS, dir string, t *partsTheme) error {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("read the parts theme %s: %w", dir, err)
	}
	// Warned of once the theme is read whole: one that cannot be used goes into no page.
	var paletteWarnings []*usererr.Error
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
			if paletteWarnings, err = set.checkPalette(path.Join(dir, e.Name()), css); err != nil {
				return err
			}
			t.palette = string(css)
		default:
			t.parts[name] = string(css)
		}
	}
	set.strays = append(set.strays, paletteWarnings...)
	return nil
}

// themeFile reports whether name.css is a file of a parts theme: its base.css, its palette.css or the style of a part,
// one of one's own that cannot be used among them, but not one of a name no part can take.
func (set *partSet) themeFile(name string) bool {
	_, isPart := set.parts[name]
	isOwn := set.broken[name] != nil && validName(name) && !set.reserved(name)
	return name == themeBase || name == themePalette || isPart || isOwn
}

// checkPalette refuses css, palette.css shown as file, as readPalette does, and returns the warning of
// the colours it sets for light mode alone.
func (set *partSet) checkPalette(file string, css []byte) ([]*usererr.Error, error) {
	light, dark, err := readPalette(file, css)
	if err != nil {
		return nil, err
	}
	if names := lightOnly(light, dark, set.modes); names != nil {
		return []*usererr.Error{usererr.New("tmpl_palette_dark", file, andList(names))}, nil
	}
	return nil, nil
}

// readPalette returns the variables (and color-scheme) palette.css, css shown as file, sets for light
// mode, with their values, and for dark mode. It refuses anything but those set on :root, those of dark
// mode in @media (prefers-color-scheme: dark) after it: in the bottom layer any other rule would lose to
// every other one, a light :root after the dark block would win in dark mode too, and !important would
// win over every layer, the template's too.
func readPalette(file string, css []byte) (map[string]string, map[string]bool, error) {
	rules, _ := cssRules(css) // refuseCSS has refused a } that closes no block
	p := &paletteCheck{
		file: file, css: css, blank: withoutComments(css), rules: rules,
		light: map[string]string{}, dark: map[string]bool{},
	}
	if err := p.onlyBlocks(0, len(p.blank), 0); err != nil {
		return nil, nil, err
	}
	for i, mode := range modesOfRules(rules, ":root") {
		if err := p.read(&rules[i], mode); err != nil {
			return nil, nil, err
		}
	}
	return p.light, p.dark, nil
}

// paletteCheck is what readPalette reads of palette.css: blank is css without its comments, and light
// and dark the variables set for each mode.
type paletteCheck struct {
	file       string
	css, blank []byte
	rules      []cssRule
	light      map[string]string
	dark       map[string]bool
}

// read reads the rule r, which is mode to the modes.
func (p *paletteCheck) read(r *cssRule, mode modeRule) error {
	switch mode {
	case modeLight:
		return p.declarations(r, func(name, value string) { p.light[name] = value })
	case modeLateLight:
		return usererr.New("tmpl_palette_order", p.file, lineAt(p.css, r.at))
	case modeMedia:
		return p.onlyBlocks(r.open+1, r.end, 1)
	case modeDark:
		return p.declarations(r, func(name, _ string) { p.dark[name] = true })
	case modeOther:
	}
	return usererr.New("tmpl_palette_rule", p.file, lineAt(p.css, r.at))
}

// onlyBlocks refuses what the file holds from from to to outside the blocks at depth, from the prelude
// of each to its }: a declaration or a statement where only blocks go.
func (p *paletteCheck) onlyBlocks(from, to, depth int) error {
	i := from
	for _, r := range p.rules {
		if !r.block || r.depth != depth || r.at < from || r.at >= to {
			continue
		}
		if err := p.blankFrom(i, r.at); err != nil {
			return err
		}
		i = r.end + 1
	}
	return p.blankFrom(i, to)
}

// blankFrom refuses a byte from from to to that is no space.
func (p *paletteCheck) blankFrom(from, to int) error {
	for i := from; i < to && i < len(p.blank); i++ {
		if !strings.ContainsRune(" \t\r\n\f", rune(p.blank[i])) {
			return usererr.New("tmpl_palette_rule", p.file, lineAt(p.css, i))
		}
	}
	return nil
}

// declarations gives each the declarations of the block r, each a variable or color-scheme without
// !important.
func (p *paletteCheck) declarations(r *cssRule, each func(name, value string)) error {
	for _, d := range declarationsIn(p.blank, r) {
		switch {
		case !strings.HasPrefix(d.name, "--") && d.name != "color-scheme" || d.value == "":
			return usererr.New("tmpl_palette_rule", p.file, lineAt(p.css, d.at))
		case important.MatchString(d.value):
			return usererr.New("tmpl_palette_important", p.file, lineAt(p.css, d.at))
		}
		each(d.name, d.value)
	}
	return nil
}

var important = regexp.MustCompile(`(?i)!\s*important`)

// modeRule is what a rule of CSS that sets colours for the modes is to them.
type modeRule int

const (
	modeOther     modeRule = iota // any other rule
	modeLight                     // the element's block at the top, for light mode, and dark mode as well
	modeLateLight                 // one after the dark block, which wins in dark mode too
	modeMedia                     // @media (prefers-color-scheme: dark)
	modeDark                      // the element's block in it, for dark mode
)

// modesOfRules returns what each of rules, in order, is to the modes, selector naming the element they
// set colours on: :root in a palette, :scope in a template.
func modesOfRules(rules []cssRule, selector string) []modeRule {
	modes := make([]modeRule, len(rules))
	var media *cssRule // the dark block the rules are in, nil at the top
	sawDark := false
	for i := range rules {
		r := &rules[i]
		if media != nil && r.at > media.end {
			media = nil
		}
		switch {
		case !r.block:
		case r.depth == 0 && r.prelude == selector && sawDark:
			modes[i] = modeLateLight
		case r.depth == 0 && r.prelude == selector:
			modes[i] = modeLight
		case r.depth == 0 && darkMedia.MatchString(r.prelude):
			modes[i], media, sawDark = modeMedia, r, true
		case r.depth == 1 && media != nil && r.prelude == selector:
			modes[i] = modeDark
		}
	}
	return modes
}

// lightOnly returns the names of light that change with the mode, as modes tells, and dark leaves out,
// in order, when the CSS leaves out too a colour that default's palette turns dark: in dark mode those
// would stay as they are among default's dark ones, light text on a light ground. CSS that sets every
// such colour for light mode alone is one look in both modes (a theme always dark), and is not told.
// LIMIT: a value of var() is taken to follow the modes, as --page-bg: var(--bg) does, though the variable
// it takes may be one set for light mode alone. Follow what it takes if palettes come to chain them.
func lightOnly(light map[string]string, dark, modes map[string]bool) []string {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(light)) {
		value := light[name]
		_, changes := modes[name]
		switch {
		case dark[name] || !changes || strings.Contains(value, "var("):
		case name == "color-scheme" && strings.Contains(value, "light") && strings.Contains(value, "dark"):
		default:
			names = append(names, name)
		}
	}
	for name, own := range modes {
		if _, set := light[name]; own && !set && !dark[name] {
			return names
		}
	}
	return nil
}

// modesOf returns the variables that change with the mode, of the palette that sets light and dark:
// true for those it sets for dark mode too, false for those it sets to them (--page-bg: var(--bg)),
// whose values a palette that writes them stops following.
func modesOf(light map[string]string, dark map[string]bool) map[string]bool {
	modes := maps.Clone(dark)
	for name, value := range light {
		for _, m := range usedVariable.FindAllStringSubmatch(value, -1) {
			if dark[m[1]] && !dark[name] {
				modes[name] = false
			}
		}
	}
	return modes
}

// floeModes are the variables that change with the mode in floe's default palette, which the palettes
// and the colours a template sets are checked against.
var floeModes = sync.OnceValues(func() (map[string]bool, error) {
	file := path.Join("parts", themesDir, DefaultTheme, themePalette+".css")
	css, err := partFiles.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read the parts themes: %w", err)
	}
	light, dark, err := readPalette(file, css)
	if err != nil {
		return nil, err
	}
	return modesOf(light, dark), nil
})

var usedVariable = regexp.MustCompile(`var\(\s*(--[\w-]+)`)

var darkMedia = regexp.MustCompile(`(?i)^@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)$`)
