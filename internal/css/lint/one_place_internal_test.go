package lint

import (
	"maps"
	"slices"

	floecss "floe/internal/css"
)

// placeOf is where each variable of it is set alone: in a palette, in a template's page.css or in floe's base.css.
var placeOf = map[string]string{
	"--page-width": "palette", "--text-width": "palette", "--page-pad": "palette",
	"--radius": "palette", "--radius-small": "palette", "--radius-pill": "palette",
	"--column-width":    "page.css",
	"--floe-text-small": "base.css", "--floe-text-aside": "base.css",
}

// placed returns where in floe's CSS a sheet is, as placeOf names it: palette, page.css, base.css or "".
func placed(s Sheet) string {
	switch {
	case s.Kind == kindTheme && s.For == "palette":
		return "palette"
	case s.Kind == kindTemplate:
		return "page.css"
	case s.Kind == kindBase:
		return "base.css"
	default:
		return ""
	}
}

// onePlace keeps each variable of placeOf to the one place that sets it, so that a theme or a part changes it there
// alone, and --radius to one length there (floe's own themes read it in calc()).
var onePlace = Rule{Name: "one-place", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		decls := declarations(s.Src, func(p string) bool { _, ok := placeOf[p]; return ok })
		for _, name := range slices.Sorted(maps.Keys(decls)) {
			for _, d := range decls[name] {
				in.Saw()
				file, line := s.at(d.at)
				if place := placed(s); place != placeOf[name] {
					found = append(found, Finding{file, line, "one-place", "sets " + name + ", which " + placeOf[name] + " alone sets"})
				} else if name == "--radius" && len(floecss.Items(d.value)) != 1 {
					found = append(found, Finding{file, line, "one-place", "--radius: " + text(s.Src, d.value) + "; give it one length"})
				}
			}
		}
	}
	return found
}}
