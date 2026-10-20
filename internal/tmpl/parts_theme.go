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
			lightOnly, err := checkPalette(path.Join(dir, e.Name()), css)
			if err != nil {
				return err
			}
			if lightOnly != nil {
				set.strays = append(set.strays, lightOnly)
			}
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

// checkPalette refuses css, palette.css shown as file, where it holds anything but the palette's
// variables (and color-scheme) set on :root, those of dark mode in @media (prefers-color-scheme: dark)
// after it: in the bottom layer any other rule would lose to every other one, a light :root after the
// dark block would win in dark mode too, and !important would win over every layer, the template's too.
// It returns the warning of the colours it sets for light mode alone, which dark mode would keep over
// default's dark ground and text.
func checkPalette(file string, css []byte) (*usererr.Error, error) {
	rules, _ := cssRules(css) // refuseCSS has refused a } that closes no block
	p := &paletteCheck{
		file: file, css: css, blank: withoutComments(css), rules: rules, media: nil, sawDark: false,
		light: map[string]string{}, dark: map[string]bool{},
	}
	if err := p.onlyBlocks(0, len(p.blank), 0); err != nil {
		return nil, err
	}
	for i := range rules {
		if err := p.read(&rules[i]); err != nil {
			return nil, err
		}
	}
	return lightOnly(file, p.light, p.dark), nil
}

// paletteCheck is what checkPalette reads of palette.css: blank is css without its comments, media the
// dark block the rule read is in, nil at the top, and light and dark the variables set for each mode.
type paletteCheck struct {
	file       string
	css, blank []byte
	rules      []cssRule
	media      *cssRule
	sawDark    bool
	light      map[string]string
	dark       map[string]bool
}

// read reads the rule r, in the order of the file.
func (p *paletteCheck) read(r *cssRule) error {
	if p.media != nil && r.at > p.media.end {
		p.media = nil
	}
	switch {
	case !r.block:
	case r.depth == 0 && r.prelude == ":root":
		if p.sawDark {
			return usererr.New("tmpl_palette_order", p.file, lineAt(p.css, r.at))
		}
		return p.declarations(r, func(name, value string) { p.light[name] = value })
	case r.depth == 0 && darkMedia.MatchString(r.prelude):
		p.media, p.sawDark = r, true
		return p.onlyBlocks(r.open+1, r.end, 1)
	case r.depth == 1 && p.media != nil && r.prelude == ":root":
		return p.declarations(r, func(name, _ string) { p.dark[name] = true })
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

// declarations reads the declarations of the block r, split at the ; outside strings and brackets
// (url(data:…;base64,…) is one value), each a variable or color-scheme without !important, which it
// gives to each.
func (p *paletteCheck) declarations(r *cssRule, each func(name, value string)) error {
	brackets, start := 0, r.open+1
	for i := start; i <= r.end; i++ {
		if i < r.end {
			switch c := p.blank[i]; {
			case c == '"' || c == '\'':
				i = stringEnd(p.blank, i)
				continue
			case c == '(':
				brackets++
				continue
			case c == ')':
				brackets = max(brackets-1, 0)
				continue
			case c != ';' || brackets > 0:
				continue
			}
		}
		d := ruleOf(p.blank, start, i)
		start = i + 1
		if d.prelude == "" {
			continue
		}
		name, value, ok := strings.Cut(d.prelude, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		switch {
		case !ok || !strings.HasPrefix(name, "--") && name != "color-scheme":
			return usererr.New("tmpl_palette_rule", p.file, lineAt(p.css, d.at))
		case important.MatchString(value):
			return usererr.New("tmpl_palette_important", p.file, lineAt(p.css, d.at))
		}
		each(name, value)
	}
	return nil
}

var important = regexp.MustCompile(`(?i)!\s*important`)

// lightOnly returns the warning of the colours of light, by name, that dark leaves out, or nil. Faces,
// and the colours of code but its ground, which stays dark in light mode too, serve both modes.
// LIMIT: a value of var() is taken to follow the modes, as --page-bg: var(--bg) does, though the variable
// it takes may be one set for light mode alone. Follow what it takes if palettes come to chain them.
func lightOnly(file string, light map[string]string, dark map[string]bool) *usererr.Error {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(light)) {
		if !dark[name] && !strings.Contains(light[name], "var(") && !slices.Contains(bothModes, name) {
			names = append(names, name)
		}
	}
	if names == nil {
		return nil
	}
	return usererr.New("tmpl_palette_dark", file, strings.Join(names, ", "))
}

// bothModes are the variables of the palette whose one value serves light and dark mode alike.
var bothModes = []string{"--font", "--display", "--mono", "--code-fg", "--code-prompt", "--code-add", "--code-del"}

var darkMedia = regexp.MustCompile(`(?i)^@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)$`)
