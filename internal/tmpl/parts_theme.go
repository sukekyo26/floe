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
// variables (and color-scheme), set on :root, those of dark mode in @media (prefers-color-scheme: dark):
// in the bottom layer, any other rule would lose to every other one. It returns the warning of the
// colours it sets for light mode alone, which dark mode would keep over default's dark ground and text.
func checkPalette(file string, css []byte) (*usererr.Error, error) {
	r := &paletteReader{css: css, blank: withoutComments(css), at: 0}
	light, dark := map[string]string{}, map[string]bool{}
	for {
		prelude, at, stop := r.upTo("{")
		switch {
		case stop == 0 && prelude == "":
			return lightOnly(file, light, dark), nil
		case stop == '{' && prelude == ":root":
			if err := r.declarations(file, func(name, value string) { light[name] = value }); err != nil {
				return nil, err
			}
			continue
		case stop == '{' && darkMedia.MatchString(prelude):
			if err := r.dark(file, dark); err != nil {
				return nil, err
			}
			continue
		}
		return nil, usererr.New("tmpl_palette_rule", file, lineAt(css, at))
	}
}

// lightOnly returns the warning of the colours of light, by name, that dark leaves out, or nil. A value
// of var() follows the variables it takes, and faces and code, on a dark ground in light mode too, serve
// both modes.
func lightOnly(file string, light map[string]string, dark map[string]bool) *usererr.Error {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(light)) {
		if !dark[name] && !strings.Contains(light[name], "var(") && !slices.Contains(sameInDark, name) &&
			!strings.HasPrefix(name, "--code-") {
			names = append(names, name)
		}
	}
	if names == nil {
		return nil
	}
	return usererr.New("tmpl_palette_dark", file, strings.Join(names, ", "))
}

// sameInDark are the variables of the palette that are no colours.
var sameInDark = []string{"--font", "--display", "--mono"}

var darkMedia = regexp.MustCompile(`(?i)^@media\s*\(\s*prefers-color-scheme\s*:\s*dark\s*\)$`)

// paletteReader reads palette.css, blank its copy without comments, from at.
type paletteReader struct {
	css, blank []byte
	at         int
}

// upTo returns what r holds up to the first byte of stops past strings, trimmed, the offset where it
// starts, and that byte, which r moves past: 0 at the end of the file, or a brace not of stops, which r
// stays at.
func (r *paletteReader) upTo(stops string) (string, int, byte) {
	start := r.at
	text := func() (string, int) {
		raw := string(r.blank[start:r.at])
		trimmed := strings.TrimLeft(raw, " \t\r\n\f")
		return strings.TrimSpace(trimmed), start + len(raw) - len(trimmed)
	}
	for ; r.at < len(r.blank); r.at++ {
		switch c := r.blank[r.at]; {
		case c == '"' || c == '\'':
			r.at = stringEnd(r.blank, r.at)
		case strings.IndexByte(stops, c) >= 0:
			s, at := text()
			r.at++
			return s, at, c
		case c == '{' || c == '}':
			s, at := text()
			return s, at, c
		}
	}
	s, at := text()
	return s, at, 0
}

// dark reads the block of @media (prefers-color-scheme: dark), past its {: one :root block, whose
// variables it notes in dark.
func (r *paletteReader) dark(file string, dark map[string]bool) error {
	if inner, at, stop := r.upTo("{"); stop != '{' || inner != ":root" {
		return usererr.New("tmpl_palette_rule", file, lineAt(r.css, at))
	}
	if err := r.declarations(file, func(name, _ string) { dark[name] = true }); err != nil {
		return err
	}
	if rest, at, stop := r.upTo("}"); stop != '}' || rest != "" {
		return usererr.New("tmpl_palette_rule", file, lineAt(r.css, at))
	}
	return nil
}

// declarations reads the declarations of a block up to its }, each a variable or color-scheme, which
// it gives to each.
func (r *paletteReader) declarations(file string, each func(name, value string)) error {
	for {
		decl, at, stop := r.upTo(";}")
		if decl != "" {
			name, value, ok := strings.Cut(decl, ":")
			name = strings.TrimSpace(name)
			if !ok || !strings.HasPrefix(name, "--") && name != "color-scheme" {
				return usererr.New("tmpl_palette_rule", file, lineAt(r.css, at))
			}
			each(name, strings.TrimSpace(value))
		}
		switch stop {
		case ';':
		case '}':
			return nil
		default:
			return usererr.New("tmpl_palette_rule", file, lineAt(r.css, at))
		}
	}
}
