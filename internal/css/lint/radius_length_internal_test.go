package lint

import floecss "floe/internal/css"

// radiusLength keeps --radius-small and --radius-pill to one item each, in every file: floe's parts read them as one
// corner, a bar's rx and a pill's min(). It is css.RadiusLength, which floe's check runs over one's own CSS too.
var radiusLength = Rule{Name: "radius-length", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		in.Saw()
		for _, c := range floecss.RadiusLength(s.Src) {
			found = append(found, Finding{s.File, c.Line, "radius-length", c.Name + ": " + c.Value + "; give it one item, a length or one function"})
		}
	}
	return found
}}
