package lint

import "strings"

// textCase keeps floe's CSS from changing the case of the text it draws: a browser copies the changed case, so a file
// name or code copied from a page would come out as other text.
// LIMIT: the style= attributes of floe's part.html and page.html, the CSS internal/draw/style.go builds and the management
// page are not read, nor font-variant-caps (small-caps), which looks like capitals but copies as written.
var textCase = Rule{Name: "text-case", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		for _, d := range declarations(s.Src, func(p string) bool { return p == "text-transform" })["text-transform"] {
			if v := text(s.Src, d.value); !strings.EqualFold(v, "none") {
				file, line := s.at(d.at)
				found = append(found, Finding{
					file, line, "text-case",
					"text-transform: " + v + " in " + d.rule + "; draw the text as written, and write capitals as capitals",
				})
			}
		}
	}
	return found
}}
