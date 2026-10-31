package browsertest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listsGeneral is a rule for every list, as a template's page.css or a theme's base.css may write one.
const listsGeneral = "ul, ol, menu { display: flex; padding-left: 2rem; list-style-type: upper-roman; gap: 1rem; margin-block: 1rem; }\n"

// listsOf tells, for each list that is a part's box or takes a look, shown or not (in a closed details, a tab not
// picked), what floe sets of it as a list: its display, marks, padding and room between items.
const listsOf = `(() => [...document.querySelectorAll(':is(ul, ol, menu):is([data-floe-part], [data-floe-look])')]
  .map((e) => { const c = getComputedStyle(e);
    return (e.dataset.floePart || e.dataset.floeLook) + ': ' + [c.display, c.listStyleType, c.paddingInlineStart, c.rowGap, c.columnGap].join(' '); })
  .join('\n'))()`

// listsMarked tells each list that is a part's box or takes a look, shown or not, and still draws a list's marks.
const listsMarked = `(() => [...document.querySelectorAll(':is(ul, ol, menu):is([data-floe-part], [data-floe-look])')]
  .filter((e) => getComputedStyle(e).listStyleType !== 'none')
  .map((e) => (e.dataset.floePart || e.dataset.floeLook) + ': ' + getComputedStyle(e).listStyleType).join('\n'))()`

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

// TestUILists draws the lists that are parts' boxes or take looks. A template's page.css or a theme's base.css that
// writes a rule for every list does not change a part's box's display, marks, padding or room between items, nor the
// theme's those of an element that takes a look; the template's own page.css does adjust an element that takes a look. Such lists draw no marks, the part's
// own arrangement set over them, in every theme and template; a list of Markdown in a part keeps its marks.
func TestUILists(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	own := map[string]string{"themes/lists/base.css": listsGeneral, "themes/plain/base.css": "/* no rules */\n"}
	for k, v := range listsKit(t, "report", "lists-report") {
		own[k] = v
	}
	for k, v := range listsKit(t, "design", "lists-design") {
		own[k] = v
	}
	s.own(t, own)
	everyPart, err := os.ReadFile("../corpus/testdata/every-part/every-part.yaml")
	if err != nil {
		t.Fatal(err)
	}
	shot, err := os.ReadFile("../corpus/testdata/every-part/shots/moved.png")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "shots/moved.png", string(shot))
	design, err := os.ReadFile("../kit/floe/templates/design/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	read := func(slug, file string, args ...string) string {
		t.Helper()
		s.floe(t, append([]string{"publish", file, "--slug", slug}, args...)...)
		b.navigate("/a/" + slug + "/?raw")
		return b.eval(listsOf)
	}

	// A part's box: the page's and the theme's rule for lists change nothing of it.
	plain := s.write(t, "every.yaml", string(everyPart))
	mine := s.write(t, "every-mine.yaml", strings.Replace(string(everyPart), "template: report\n", "template: lists-report\n", 1))
	want := read("lists-plain", plain)
	if got := read("lists-page", mine); got != want {
		t.Errorf("parts' boxes under the page's rule for lists:\n%s\nwant, as without it:\n%s", got, want)
	}
	want = read("lists-theme-plain", plain, "--theme", "plain")
	if got := read("lists-theme", plain, "--theme", "lists"); got != want {
		t.Errorf("parts' boxes under the theme's rule for lists:\n%s\nwant, as without it:\n%s", got, want)
	}

	// An element that takes a look: the theme's rule does not reach it, the template's own page.css does.
	decided := `(() => { const c = getComputedStyle(document.querySelector('.decided'));
  return [c.display, c.listStyleType, c.paddingInlineStart, c.rowGap].join(' '); })()`
	designFile := s.write(t, "design.yaml", string(design))
	s.floe(t, "publish", designFile, "--slug", "lists-design-plain", "--theme", "plain")
	b.navigate("/a/lists-design-plain/?raw")
	wantDecided := b.eval(decided)
	s.floe(t, "publish", designFile, "--slug", "lists-design-theme", "--theme", "lists")
	b.navigate("/a/lists-design-theme/?raw")
	if got := b.eval(decided); got != wantDecided {
		t.Errorf("a look's list under the theme's rule for lists: %s, want %s, as without it", got, wantDecided)
	}
	designMine := s.write(t, "design-mine.yaml", strings.Replace(string(design), "template: design\n", "template: lists-design\n", 1))
	s.floe(t, "publish", designMine, "--slug", "lists-design-page", "--theme", "plain")
	b.navigate("/a/lists-design-page/?raw")
	if got := b.eval(decided); got != "flex upper-roman 32px 16px" {
		t.Errorf("a look's list under its template's rule for lists: %s, want flex upper-roman 32px 16px, the template's", got)
	}

	// No marks on such lists, in every theme and template; the part's own arrangement over floe's.
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	examples, err := filepath.Glob("../corpus/testdata/example/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{"every-part": plain}
	for _, ex := range examples {
		data, err := os.ReadFile(ex)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(ex), ".yaml")
		pages[name] = s.write(t, "example-"+name+".yaml", string(data))
	}
	var marked []string
	for _, th := range themes {
		for name, file := range pages {
			slug := "lists-" + name + "-" + th.Name()
			s.floe(t, "publish", file, "--slug", slug, "--theme", th.Name())
			b.navigate("/a/" + slug + "/?raw")
			for line := range strings.Lines(b.eval(listsMarked)) {
				marked = append(marked, name+" in "+th.Name()+": "+strings.TrimSuffix(line, "\n"))
			}
		}
	}
	check(t, "marked", marked, "")

	b.navigate("/a/lists-plain/?raw")
	ownWay := `(() => [getComputedStyle(document.querySelector('[data-floe-part=timeline]')).paddingInlineStart,
  getComputedStyle(document.querySelector('[data-floe-part=links]')).columnGap].join(' '))()`
	if got := b.eval(ownWay); got != "20.8px 12px" {
		t.Errorf("timeline's padding and links' gap: %s, want 20.8px 12px, the parts' own", got)
	}

	s.floe(t, "publish", s.write(t, "decision.yaml", listsDecision), "--slug", "lists-decision")
	b.navigate("/a/lists-decision/?raw")
	marks := `(() => ['[data-floe-part=decision] ul:not(.rejected)', '[data-floe-part=decision] .rejected']
  .map((q) => getComputedStyle(document.querySelector(q)).listStyleType + ' ' +
    getComputedStyle(document.querySelector(q + ' > li'), '::before').content).join(', '))()`
	if got := b.eval(marks); got != `disc none, none "×" / ""` {
		t.Errorf("decision's list in its text and the options set aside: %s, want disc none, none \"×\" / \"\"", got)
	}
}
