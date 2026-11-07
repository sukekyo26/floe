package lint

// opacityRule keeps opacity in floe's CSS to 0, what hides: what is faint is faded by its colour, which needs no opacity.
// It reads every rule as written, in every state and medium.
// LIMIT: filter: opacity() and SVG's fill-opacity and the like are not read, nor a style attribute; floe's CSS writes none.
var opacityRule = Rule{Name: "opacity", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		for _, d := range declarations(s.Src, func(p string) bool { return p == "opacity" })["opacity"] {
			if v := text(s.Src, d.value); v != "0" {
				file, line := s.at(d.at)
				found = append(found, Finding{
					file, line, "opacity",
					"opacity: " + v + " in " + d.rule + "; fade it by its colour (color-mix(… transparent)), opacity is 0 alone",
				})
			}
		}
	}
	return found
}}
