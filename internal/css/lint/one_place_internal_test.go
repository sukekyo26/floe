package lint

import (
	"maps"
	"slices"
	"strings"

	floecss "floe/internal/css"
)

// placeOf is where each variable of it is set alone: in a palette, in a template's page.css or in floe's defaults.css.
var placeOf = map[string]string{
	"--page-width": "palette", "--text-width": "palette", "--page-pad": "palette",
	"--radius": "palette", "--radius-small": "palette", "--radius-pill": "palette",
	"--column-width": "page.css",
}

// stepPrefix is the start of the names of the steps of small text, which floe's defaults.css alone sets.
const stepPrefix = "--floe-text-"

// placeOfName returns where name is set alone, "" for anywhere.
func placeOfName(name string) string {
	if strings.HasPrefix(name, stepPrefix) {
		return "defaults.css"
	}
	return placeOf[name]
}

// placed returns where in floe's CSS a sheet is, as placeOf names it: palette, page.css, defaults.css or "".
func placed(s Sheet) string {
	switch {
	case s.Kind == kindTheme && s.For == "palette":
		return "palette"
	case s.Kind == kindTemplate:
		return "page.css"
	case s.Kind == kindDefaults:
		return "defaults.css"
	default:
		return ""
	}
}

// onePlace keeps each variable of placeOf to the one place that sets it, so that a theme or a part changes it there
// alone, and --radius to one length there (floe's own themes read it in calc()).
var onePlace = Rule{Name: "one-place", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		decls := declarations(s.Src, func(p string) bool { return placeOfName(p) != "" })
		for _, name := range slices.Sorted(maps.Keys(decls)) {
			for _, d := range decls[name] {
				in.Saw()
				file, line := s.at(d.at)
				if place := placed(s); place != placeOfName(name) {
					found = append(found, Finding{file, line, "one-place", "sets " + name + ", which " + placeOfName(name) + " alone sets"})
				} else if name == "--radius" && len(floecss.Items(d.value)) != 1 {
					found = append(found, Finding{file, line, "one-place", "--radius: " + text(s.Src, d.value) + "; give it one length"})
				}
			}
		}
	}
	return found
}}
