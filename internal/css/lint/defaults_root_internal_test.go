package lint

import (
	"maps"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// defaultsRoot keeps floe's defaults.css, the lowest layer, to the defaults of the variables floe sets on the page's
// root: a plain :root's --floe-*, without !important nor an at-rule. A style there would lose to whatever a theme
// writes, and !important turns the order of the layers round. An at-rule is told once, and what it holds is not read
// further: the fix is to take it out.
var defaultsRoot = Rule{Name: "defaults-root", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		if s.Kind != kindDefaults {
			continue
		}
		for _, t := range tokensOf(s.Src) {
			if t.Type == css.AtKeywordToken {
				file, line := s.at(t.At)
				found = append(found, Finding{file, line, "defaults-root", string(t.Data) + ": write no at-rule in defaults.css"})
			}
		}
		decls := declarations(s.Src, func(string) bool { return true })
		for _, name := range slices.Sorted(maps.Keys(decls)) {
			for _, d := range decls[name] {
				in.Saw()
				rule := strings.TrimSpace(d.rule)
				var why string
				switch {
				case strings.HasPrefix(rule, "@") || strings.Contains(rule, " { @"):
					continue
				case rule != ":root":
					why = name + " in " + rule + "; set only a plain :root in defaults.css"
				case !strings.HasPrefix(name, "--floe-"):
					why = name + " on :root; set only --floe-* variables in defaults.css"
				case d.important:
					why = name + " is !important; write no !important in defaults.css"
				default:
					continue
				}
				file, line := s.at(d.at)
				found = append(found, Finding{file, line, "defaults-root", why})
			}
		}
	}
	return found
}}
