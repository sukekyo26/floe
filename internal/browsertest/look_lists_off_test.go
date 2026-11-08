package browsertest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listsGeneral is a rule for every list, as a template's page.css or a theme's base.css may write one, as specific as
// an id, so that only the layers keep it from what floe lays out.
const listsGeneral = ":is(ul, ol, menu):not(#none) { display: flex; padding-left: 2rem; list-style-type: upper-roman; gap: 1rem; " +
	"margin-block: 1rem; }\n"

// listsLook is a look of one's own that sets its list's room and padding, and a template that lends it to a list.
var listsLook = map[string]string{
	"parts/lists-gap/part.css":  ":scope { gap: 7px; padding-left: 9px; }\n",
	"parts/lists-gap/part.yaml": "description: A list's own room and padding\n",
	"templates/lists-look/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\"><title>{{.Title}}</title></head>\n" +
		"<body><ul data-floe-look=\"lists-gap\"><li>a</li><li>b</li></ul></body></html>\n",
	"templates/lists-look/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
	"templates/lists-look/example.yaml": "lang: en\ntemplate: lists-look\n",
}

// listsOf tells, for each list that is a part's box or takes a look, shown or not (in a closed details, a tab not
// picked), what floe sets of it as a list: its display, marks, padding and room between items.
const listsOf = `(() => [...document.querySelectorAll(':is(ul, ol, menu):is([data-floe-part], [data-floe-look])')]
  .map((e) => { const c = getComputedStyle(e);
    return (e.dataset.floePart || e.dataset.floeLook) + ': ' + [c.display, c.listStyleType, c.paddingInlineStart, c.rowGap, c.columnGap].join(' '); })
  .join('\n'))()`

// listsMarked tells each list that is a part's box or takes a look, shown or not, and still draws a list's marks.
// measureListsMarked returns {seen, amiss}: the lists that are a part's box or a look it measured, and for each one
// that shows marks its part (or look) and the marks' style.
const measureListsMarked = `(() => {
  const lists = [...document.querySelectorAll(':is(ul, ol, menu):is([data-floe-part], [data-floe-look])')];
  return JSON.stringify({ seen: lists.length, amiss: lists.filter((e) => getComputedStyle(e).listStyleType !== 'none')
    .map((e) => ({ at: e.dataset.floePart || e.dataset.floeLook, text: 'shows ' + getComputedStyle(e).listStyleType })) });
})()`

// listsDecision is a decision whose text holds a list of Markdown, beside the options it set aside.
const listsDecision = `template: report
lang: en
title: Decision
sections:
  - parts:
      - decision:
          title: The wiki moves to the new host
          text: "Why:\n\n- The old server is out of support in **May**.\n- The new host is cheaper."
          rejected: [Stay on the old server, Move to a hosted wiki]
`

// listsKit copies floe's templates name to one of one's own named mine, its page.css ending in listsGeneral, and
// returns its files.
func listsKit(t *testing.T, name, mine string) map[string]string {
	t.Helper()

	dir := filepath.Join("../kit/floe/templates", name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(entries))
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		switch e.Name() {
		case "page.css":
			text += listsGeneral
		case "example.yaml":
			text = strings.Replace(text, "template: "+name+"\n", "template: "+mine+"\n", 1)
		}
		files["templates/"+mine+"/"+e.Name()] = text
	}
	return files
}

// listsApart returns {seen, amiss} of got against want, lines of listsOf (linesApart): each part's box (or look) drawn
// otherwise is amiss at it.
func listsApart(page, got, want string) measured {
	return linesApart(page, got, want, "as without the rule", func(line string) string {
		at, _, _ := strings.Cut(line, ":")
		return at
	})
}

// listsKitAll is the templates, themes and look of one's own that lists-off and TestRenderedLists draw.
func listsKitAll(t *testing.T) map[string]string {
	t.Helper()

	own := map[string]string{"themes/lists/base.css": listsGeneral, "themes/plain/base.css": "/* no rules */\n"}
	for _, k := range []map[string]string{listsKit(t, "report", "lists-report"), listsKit(t, "design", "lists-design"), listsLook} {
		for name, text := range k {
			own[name] = text
		}
	}
	return own
}

const lookListsOffFix = "let floe's boxes lay out a part's list: set a part's own arrangement in its part.css, not for every list, " +
	"and write no !important that puts a list's marks, padding or room back"

// lookListsOff draws every part (everyPart) as listsOf reads its lists that are parts' boxes or take looks: a
// template's page.css or a theme's base.css that writes a rule for every list (listsGeneral) changes nothing of a
// part's box, its display, marks, padding or room between items.
func lookListsOff(t *testing.T, s *site) measured {
	t.Helper()

	s.own(t, listsKitAll(t))
	everyPart := s.everyPart(t)
	read := func(slug, file string, args ...string) string {
		t.Helper()
		s.floe(t, append([]string{"publish", file, "--slug", slug}, args...)...)
		s.b.navigate("/a/" + slug + "/?raw")
		return s.b.eval(listsOf)
	}
	plain := s.write(t, "every.yaml", everyPart)
	mine := s.write(t, "every-mine.yaml", strings.Replace(everyPart, "template: report\n", "template: lists-report\n", 1))
	m := listsApart("every-part under the page's rule for lists", read("lists-page", mine), read("lists-plain", plain))
	m.add(listsApart("every-part under the theme's rule for lists", read("lists-theme", plain, "--theme", "lists"), read("lists-theme-plain", plain, "--theme", "plain")))
	return m
}

// brokenListsOff draws steps in a theme of one's own whose base.css puts a list's padding back by !important, which
// wins over floe's boxes, against steps in the default theme.
func brokenListsOff(t *testing.T, s *site) (string, measured) {
	t.Helper()

	steps := "- steps:\n    - { title: One, text: first }\n    - { title: Two, text: second }\n"
	s.floe(t, "publish", s.write(t, "steps.yaml", themePage("default", steps)), "--slug", "steps")
	s.b.navigate("/a/steps/?raw")
	want := s.b.eval(listsOf)
	s.brokenPage(t, map[string]string{"themes/padded/base.css": "ol { padding-inline-start: 3em !important; }\n"}, themePage("padded", steps))
	return "steps", listsApart("broken", s.b.eval(listsOf), want)
}

const lookListsMarkedFix = "leave a list that is a part's box or a look unmarked: floe's boxes take its marks off, and no rule puts them back"

// lookListsMarked measures every page of floePages as measureListsMarked does.
func lookListsMarked(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	s.floePages(t, func(what string) { m.add(s.b.measure(what, measureListsMarked)) })
	return m
}

// brokenListsMarked draws a part of one's own whose box is a list it marks.
func brokenListsMarked(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<ul data-floe-part="listy"><li>a</li><li>b</li></ul>`
	s.brokenPage(t, ownPart("listy", html, ":scope { list-style: disc; padding-inline-start: 2em; }\n"), htmlPage(html))
	return "listy", s.b.measure("broken", measureListsMarked)
}
