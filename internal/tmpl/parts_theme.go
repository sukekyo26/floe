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

	"github.com/tdewolff/parse/v2/css"

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
		sheet := readCSS(css)
		if err = sheet.refuse(path.Join(dir, e.Name()), name != themeBase && name != themePalette); err != nil {
			return err
		}
		switch name {
		case themeBase:
			t.base = string(css)
		case themePalette:
			if paletteWarnings, err = checkPalette(path.Join(dir, e.Name()), sheet); err != nil {
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

// checkPalette refuses s, palette.css shown as file, as readPalette does, and returns the warning of
// the colours it sets for light mode alone.
func checkPalette(file string, s *cssSheet) ([]*usererr.Error, error) {
	c, err := readPalette(file, s)
	if err != nil {
		return nil, err
	}
	modes, err := floeModes()
	if err != nil {
		return nil, err
	}
	if names := c.lightOnly(modes); names != nil {
		return []*usererr.Error{usererr.New("tmpl_palette_dark", file, andList(names))}, nil
	}
	return nil, nil
}

// readPalette returns the variables (and color-scheme) palette.css, s shown as file, sets for each mode.
// It refuses anything but those set on :root, those of dark mode in @media (prefers-color-scheme: dark)
// after it: in the bottom layer any other rule would lose to every other one, a light :root after the
// dark block would win in dark mode too, and !important would win over every layer, the template's too.
func readPalette(file string, s *cssSheet) (*modeColours, error) {
	p := &paletteCheck{file: file, sheet: s, colours: newModeColours()}
	if err := p.onlyBlocks(0, len(s.tokens), 0); err != nil {
		return nil, err
	}
	for i, mode := range modesOfRules(s.rules, ":root", darkMedia.MatchString) {
		if err := p.read(&s.rules[i], mode); err != nil {
			return nil, err
		}
	}
	return p.colours, nil
}

// paletteCheck is what readPalette reads of palette.css.
type paletteCheck struct {
	file    string
	sheet   *cssSheet
	colours *modeColours
}

// read reads the rule r, which is mode to the modes.
func (p *paletteCheck) read(r *cssRule, mode modeRule) error {
	switch mode {
	case modeLight, modeDark:
		decls, err := p.declarations(r)
		if err != nil {
			return err
		}
		p.colours.add(decls, mode == modeDark, p.file)
		return nil
	case modeLateLight:
		return p.mistake("tmpl_palette_order", r.at)
	case modeMedia:
		return p.onlyBlocks(r.open+1, r.end, 1)
	case modeOther:
	}
	return p.mistake("tmpl_palette_rule", r.at)
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

// declarations returns the declarations of the block r, refusing one that is no variable nor
// color-scheme, and !important.
func (p *paletteCheck) declarations(r *cssRule) ([]cssDeclaration, error) {
	decls := p.sheet.declarations(r)
	for _, d := range decls {
		switch {
		case !strings.HasPrefix(d.name, "--") && d.name != "color-scheme" || d.value == "":
			return nil, p.mistake("tmpl_palette_rule", d.at)
		case d.important:
			return nil, p.mistake("tmpl_palette_important", d.at)
		}
	}
	return decls, nil
}

// mistake is the mistake key of palette.css at the offset at.
func (p *paletteCheck) mistake(key string, at int) error {
	return usererr.New(key, p.file, lineAt(p.sheet.src, at))
}

// modeRule is what a rule of CSS that sets colours for the modes is to them.
type modeRule int

const (
	modeOther     modeRule = iota // any other rule
	modeLight                     // the element's block at the top, for light mode, and dark mode as well
	modeLateLight                 // one after the dark block, which wins in dark mode too
	modeMedia                     // the dark block, @media (prefers-color-scheme: dark)
	modeDark                      // the element's block in it, for dark mode
)

// modesOfRules returns what each of rules, in order, is to the modes, selector naming the element they
// set colours on, :root in a palette and :scope in a template, and dark telling the prelude of the block
// for dark mode.
func modesOfRules(rules []cssRule, selector string, dark func(string) bool) []modeRule {
	modes := make([]modeRule, len(rules))
	var media *cssRule // the dark block the rules are in, nil at the top
	sawDark := false
	for i := range rules {
		r := &rules[i]
		if media != nil && r.first > media.end {
			media = nil
		}
		switch {
		case !r.block:
		case r.depth == 0 && r.prelude == selector && sawDark:
			modes[i] = modeLateLight
		case r.depth == 0 && r.prelude == selector:
			modes[i] = modeLight
		case r.depth == 0 && dark(r.prelude):
			modes[i], media, sawDark = modeMedia, r, true
		case r.depth == 1 && media != nil && r.prelude == selector:
			modes[i] = modeDark
		}
	}
	return modes
}

// modeColours are the variables (and color-scheme) CSS sets for the modes: for light mode, with their
// values and the file of each, and for dark mode.
type modeColours struct {
	light map[string]string
	dark  map[string]bool
	from  map[string]string
}

func newModeColours() *modeColours {
	return &modeColours{light: map[string]string{}, dark: map[string]bool{}, from: map[string]string{}}
}

// clone returns a copy of c, to read more CSS into.
func (c *modeColours) clone() *modeColours {
	return &modeColours{light: maps.Clone(c.light), dark: maps.Clone(c.dark), from: maps.Clone(c.from)}
}

// add adds the variables (and color-scheme) of decls, set in file, for dark mode or for light. Those for
// light mode, written later than the dark ones, win in dark mode too, which they leave unset.
func (c *modeColours) add(decls []cssDeclaration, inDark bool, file string) {
	for _, d := range decls {
		switch {
		case !strings.HasPrefix(d.name, "--") && d.name != "color-scheme":
		case inDark:
			c.dark[d.name] = true
		default:
			c.light[d.name], c.from[d.name] = d.value, file
			delete(c.dark, d.name)
		}
	}
}

// readScope adds what a template's CSS, s of the file named file, sets on :scope, that of dark mode in
// @media … (prefers-color-scheme: dark), in order.
// LIMIT: :scope alone as a selector, not in a list (:scope, body). Read the selectors if a template comes
// to set colours so.
func (c *modeColours) readScope(s *cssSheet, file string) {
	for i, mode := range modesOfRules(s.rules, ":scope", darkCondition.MatchString) {
		switch mode {
		case modeLight, modeLateLight:
			c.add(s.declarations(&s.rules[i]), false, file)
		case modeDark:
			c.add(s.declarations(&s.rules[i]), true, file)
		case modeOther, modeMedia:
		}
	}
}

// lightOnly returns the names c sets for light mode alone that change with the mode, as modes tells, in
// order, when they mix with colours of dark mode: in dark mode they would stay as they are among dark
// ones, light text on a light ground. c that sets every colour default's palette turns dark for light
// mode, and none for dark, is one look in both modes (a theme always dark), and is not told.
// LIMIT: a value of var() or light-dark() is taken to follow the modes, as --page-bg: var(--bg) does,
// though the variable it takes may be one set for light mode alone. Follow it if palettes come to chain them.
func (c *modeColours) lightOnly(modes map[string]bool) []string {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(c.light)) {
		value := c.light[name]
		_, changes := modes[name]
		switch {
		case c.dark[name] || !changes || strings.Contains(value, "var(") || strings.Contains(value, "light-dark("):
		case name == "color-scheme" && strings.Contains(value, "light") && strings.Contains(value, "dark"):
		default:
			names = append(names, name)
		}
	}
	if names == nil {
		return nil
	}
	for name, own := range modes {
		_, set := c.light[name]
		if c.dark[name] || own && !set {
			return names
		}
	}
	return nil
}

// modesOf returns the variables that change with the mode in c, floe's default palette: true for those
// it sets for dark mode too, false for those it sets to them (--page-bg: var(--bg)), whose values a
// palette that writes them stops following.
func modesOf(c *modeColours) map[string]bool {
	modes := maps.Clone(c.dark)
	for name, value := range c.light {
		for _, m := range usedVariable.FindAllStringSubmatch(value, -1) {
			if c.dark[m[1]] && !c.dark[name] {
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
	c, err := readPalette(file, readCSS(css))
	if err != nil {
		return nil, err
	}
	return modesOf(c), nil
})

var usedVariable = regexp.MustCompile(`var\(\s*(--[\w-]+)`)

// darkCondition is the prelude of a block of a template's CSS for dark mode, which may add to it.
var darkCondition = regexp.MustCompile(`(?i)^@media\b[^{]*\(\s*prefers-color-scheme\s*:\s*dark\s*\)`)

// darkMedia is that of a palette's, which holds nothing else.
var darkMedia = regexp.MustCompile(`(?i)^@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)$`)
