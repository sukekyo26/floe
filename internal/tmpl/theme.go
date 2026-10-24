package tmpl

import (
	"embed"
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
		slices.Collect(maps.Keys(set.broken[ownTheme])))))
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
// palette that sets a colour for light mode alone, and another file that writes a colour or sets a
// variable, as colourWarnings tells.
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
	// base.css and palette.css are for the whole page, out of the parts' scope.
	sheet := readCSS(css)
	if err = sheet.refuse(file, name != themeBase && name != themePalette); err != nil {
		return nil, err
	}
	if name == themePalette {
		more, err := checkPalette(file, sheet, t.palettes)
		if err != nil {
			return nil, err
		}
		t.palettes = append(t.palettes, string(css))
		return more, nil
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
	isOwn := set.broken[ownPart][name] != nil && validName(name) && !set.reserved(name)
	return name == themeBase || name == themePalette || isPart || isOwn
}

// checkPalette refuses s, palette.css shown as file, as readPalette does, and returns the warning of
// the colours it sets for light mode alone, of those that change with the mode in default's palette and
// in under, floe's palettes of its theme it goes over.
func checkPalette(file string, s *cssSheet, under []string) ([]*usererr.Error, error) {
	c, err := readPalette(file, s)
	if err != nil {
		return nil, err
	}
	modes, err := floeModes()
	if err != nil {
		return nil, err
	}
	modes = maps.Clone(modes)
	for _, palette := range under {
		u, err := readPalette(file, readCSS([]byte(palette)))
		if err != nil {
			return nil, err
		}
		// One that changes with the mode in either stays so.
		for name, own := range modesOf(u) {
			modes[name] = modes[name] || own
		}
	}
	if names := c.lightOnly(modes); names != nil {
		return []*usererr.Error{usererr.NewAt(file, 0, 0, "tmpl_palette_dark", andList(names))}, nil
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
	for i, mode := range modesOfRules(s.rules) {
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
		for _, d := range decls {
			p.colours.add(d, mode == modeDark)
		}
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
	decls := r.decls
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
	return usererr.NewAt(p.file, lineAt(p.sheet.src, at), 0, key)
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

// modesOfRules returns what each of rules, in order, is to the modes of a palette: :root at the top, and
// in @media (prefers-color-scheme: dark), nothing else.
func modesOfRules(rules []cssRule) []modeRule {
	modes := make([]modeRule, len(rules))
	sawDark := false
	for i := range rules {
		r := &rules[i]
		switch {
		case !r.block:
		case r.depth == 0 && r.prelude == ":root" && sawDark:
			modes[i] = modeLateLight
		case r.depth == 0 && r.prelude == ":root":
			modes[i] = modeLight
		case r.depth == 0 && darkMedia.MatchString(r.prelude):
			modes[i], sawDark = modeMedia, true
		case r.depth == 1 && modes[r.parent] == modeMedia && r.prelude == ":root":
			modes[i] = modeDark
		}
	}
	return modes
}

// modeColours are the variables (and color-scheme) CSS sets for the modes: for light mode, with their
// values, and for dark mode.
type modeColours struct {
	light map[string]string
	dark  map[string]bool
}

func newModeColours() *modeColours {
	return &modeColours{light: map[string]string{}, dark: map[string]bool{}}
}

// add adds d, a declaration, for dark mode or for light, if it sets a variable or
// color-scheme. One for light mode, written later than the dark one, wins in dark mode too, which it
// leaves unset.
func (c *modeColours) add(d cssDeclaration, inDark bool) {
	switch {
	case !strings.HasPrefix(d.name, "--") && d.name != "color-scheme":
	case inDark:
		c.dark[d.name] = true
	default:
		c.light[d.name] = d.value
		delete(c.dark, d.name)
	}
}

// readScope adds what a template's CSS, s, sets on :scope, in the order it is written: outside any block,
// which floe's @scope applies to :scope, in a block of :scope, or of an at-rule in it (:scope { @media …
// { … } }), under at-rules or none (@layer, @supports), for dark mode where one of them is a dark one, as
// darkCondition tells.
// LIMIT: :scope alone as a selector, not in a list (:scope, body). Read the selectors if a template comes
// to set colours so.
func (c *modeColours) readScope(s *cssSheet) {
	type modeDecl struct {
		d    cssDeclaration
		dark bool
	}
	var decls []modeDecl
	// One outside any block, in floe's @scope, is :scope's.
	for _, d := range s.decls {
		decls = append(decls, modeDecl{d: d, dark: false})
	}
	for i := range s.rules {
		if !s.rules[i].block || !s.onScope(i) {
			continue
		}
		dark := s.under(i, darkCondition)
		for _, d := range s.rules[i].decls {
			decls = append(decls, modeDecl{d: d, dark: dark})
		}
	}
	slices.SortFunc(decls, func(a, b modeDecl) int { return a.d.at - b.d.at })
	for _, m := range decls {
		c.add(m.d, m.dark)
	}
}

// onScope reports whether the declarations of the rule i of s set on :scope: it is :scope's, or an
// at-rule's in it or in none, which floe's @scope applies to :scope.
func (s *cssSheet) onScope(i int) bool {
	for ; i >= 0; i = s.rules[i].parent {
		switch r := &s.rules[i]; {
		case r.prelude == ":scope":
			return true
		case !strings.HasPrefix(r.prelude, "@"):
			return false
		}
	}
	return true
}

// under reports whether the rule i of s, or one that holds it, is an at-rule of a prelude match tells.
func (s *cssSheet) under(i int, match func(string) bool) bool {
	for ; i >= 0; i = s.rules[i].parent {
		if r := &s.rules[i]; strings.HasPrefix(r.prelude, "@") && match(r.prelude) {
			return true
		}
	}
	return false
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
	file := path.Join(floeThemesDir, DefaultTheme, themePalette+".css")
	css, err := themeFiles.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read the themes: %w", err)
	}
	c, err := readPalette(file, readCSS(css))
	if err != nil {
		return nil, err
	}
	return modesOf(c), nil
})

var usedVariable = regexp.MustCompile(`var\(\s*(--[\w-]+)`)

// darkCondition reports whether prelude is that of a block of a template's CSS for dark mode alone, which
// may add to it (@media screen and (prefers-color-scheme: dark)), but not negate it nor give other cases.
func darkCondition(prelude string) bool {
	return darkQuery.MatchString(prelude) && !otherQuery.MatchString(prelude)
}

var (
	darkQuery  = regexp.MustCompile(`(?is)^@media\b.*\(\s*prefers-color-scheme\s*:\s*dark\s*\)`)
	otherQuery = regexp.MustCompile(`(?i)\bnot\b|\bor\b|,`)
)

// darkMedia is that of a palette's, which holds nothing else.
var darkMedia = regexp.MustCompile(`(?i)^@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)$`)
