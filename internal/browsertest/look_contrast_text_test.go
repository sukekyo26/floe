package browsertest_test

import "testing"

const lookContrastTextFix = "draw text 4.5:1 off the ground under it (3:1 if large), as WCAG AA asks: choose the palette's colours, " +
	"not ones of your own"

// lookContrastText measures every page of floePages and of bandPages as measureContrastAt does: the parts, the
// templates, the themes and their palettes together draw no text that cannot be read, in a band too.
func lookContrastText(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	visit := func(what string) { m.add(s.b.measure(what, measureContrastAt)) }
	s.floePages(t, visit)
	bandPages(t, s, visit)
	return m
}

// brokenContrastText draws a note in a theme of one's own whose note.css paints its text near white on its light ground.
func brokenContrastText(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{"themes/faint/note.css": ":scope { color: #f2f2f2; }\n"}, themePage("faint", "- note: { text: faint words }\n"))
	return "note", s.b.measure("broken", measureContrastAt)
}
