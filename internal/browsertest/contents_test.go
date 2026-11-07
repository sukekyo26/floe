package browsertest_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"
)

// contentsPage is a report of 8 sections, the first with subsections, the last two short, which the contents list;
// a link to another page among the words of a heading, which the contents write as its words.
var contentsPage = func() string {
	long := strings.TrimSpace(strings.Repeat("A paragraph that goes on for a while to fill the page. ", 40))
	var b strings.Builder
	b.WriteString("template: report\nlang: en\ntitle: Contents\nlead: " + long + "\nsections:\n")
	b.WriteString("  - heading: See [the guide](https://example.com/guide#top)\n    parts: [ { text: \"" + long + "\" } ]\n" +
		"    sections:\n      - { heading: First, parts: [ { text: \"" + long + "\" } ] }\n" +
		"      - { heading: Second, parts: [ { text: \"" + long + "\" } ] }\n")
	for i := 1; i < 6; i++ {
		fmt.Fprintf(&b, "  - heading: Long %d\n    parts: [ { text: \"%s\" } ]\n", i, long)
	}
	b.WriteString("  - heading: Short A\n    parts: [ { text: A line. } ]\n  - heading: Short B\n    parts: [ { text: A line. } ]\n")
	return b.String()
}()

// measureContents returns a line for each thing the contents draw wrong at a width of 1000 or more: not beside the
// sections, out of the window, a page narrower than the window, a lead wider than a text, a link of another page in
// them, a link that holds a link.
const measureContents = `(() => {
  const out = [], nav = document.querySelector('[data-floe-look~=toc]'), r = nav.getBoundingClientRect();
  const sections = document.querySelector('.sections').getBoundingClientRect();
  if (r.right > sections.left) out.push('the contents not beside the sections');
  if (r.top < 0 || r.top > innerHeight) out.push('the contents out of the window at ' + r.top);
  if (Math.abs(document.querySelector('main').getBoundingClientRect().width - document.documentElement.clientWidth) > 1) out.push('the page narrower than the window');
  const lead = document.querySelector('.lead'), probe = document.createElement('div');
  probe.style.width = 'var(--text-width)'; document.body.append(probe);
  if (lead.getBoundingClientRect().width > probe.getBoundingClientRect().width + 1) out.push('the lead wider than a text');
  probe.remove();
  if (nav.querySelector('a a, a[href^=http]')) out.push('a link of another page in the contents');
  return out.join('\n');
})()`

// current is the words of the link the contents mark, after the page has drawn what it scrolled to.
func (b *browser) current() string {
	b.t.Helper()

	b.painted()
	b.painted()
	return b.eval("[...document.querySelectorAll('[aria-current]')].map((a) => a.textContent).join('|')")
}

// TestUIContents draws report's contents in every theme beside the sections, stuck in the window and marking the
// section scrolled to; at the top the first, at the end the last, a section pressed while it is in sight; the page as
// wide as the window, a narrow window with the sections alone over them, none on paper, and a page without them
// as wide as before. design's contents mark their section too.
// LIMIT: a page in the frame of the management page is a target this tab cannot read; it is tried by hand.
func TestUIContents(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	defer b.unsize()
	defer b.media("")
	file := s.write(t, "contents.yaml", contentsPage)
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	var amiss []string
	scroll := func(to string) string {
		b.eval("scrollTo(0, " + to + ")")
		return b.current()
	}
	b.size(1440, 900, false)
	for _, theme := range themes {
		name := theme.Name()
		s.floe(t, "publish", file, "--slug", "contents-"+name, "--theme", name)
		b.navigate("/a/contents-" + name + "/?raw")
		if got := scroll("document.getElementById('section.3').offsetTop"); got != "Long 3" {
			amiss = append(amiss, name+": scrolled to Long 3, marks "+got)
		}
		if got := b.eval(measureContents); got != "" {
			amiss = append(amiss, name+":\n"+got)
		}
	}
	b.navigate("/a/contents-default/?raw")
	at := func(want, where, got string) {
		if got != want {
			amiss = append(amiss, where+" marks "+got+", want "+want)
		}
	}
	at("See the guide", "the top", b.current())
	at("Short B", "the end", scroll("document.documentElement.scrollHeight"))
	press := func(i int) string {
		b.eval("[...document.querySelectorAll('[data-floe-look~=toc] a')][" + fmt.Sprint(i) + "].click()")
		return b.current()
	}
	at("Short A", "Short A pressed", press(8))
	at("Short B", "Short B pressed", press(9))
	at("Short A", "Short A pressed again", press(8))
	at("See the guide", "the top again", scroll("0"))
	// A low window: the contents scroll within, the place in sight.
	b.size(1440, 500, false)
	scroll("document.getElementById('section.5').offsetTop")
	if got := b.eval(`(() => { const n = document.querySelector('[data-floe-look~=toc]').getBoundingClientRect(),
    a = document.querySelector('[aria-current]').getBoundingClientRect(); return a.top >= n.top && a.bottom <= n.bottom; })()`); got != "true" {
		amiss = append(amiss, "a low window: the place out of the contents' sight")
	}
	// The keyboard: the first stop is the contents, its ring within the links.
	b.size(1440, 900, false)
	b.navigate("/a/contents-default/?raw")
	b.pressKey(kb.Tab)
	if got := b.eval("document.activeElement.closest('[data-floe-look~=toc]') !== null && getComputedStyle(document.activeElement).outlineOffset"); got != "-2px" {
		amiss = append(amiss, "the first Tab: "+got)
	}
	// A narrow window: the sections alone over the sections, nothing past the window.
	b.size(400, 800, false)
	if got := b.eval(`(() => { const n = document.querySelector('[data-floe-look~=toc]');
    return [n.getBoundingClientRect().bottom <= document.querySelector('.sections').getBoundingClientRect().top,
      [...n.querySelectorAll('li li')].every((li) => li.getBoundingClientRect().height === 0),
      document.documentElement.scrollWidth <= innerWidth].join(); })()`); got != "true,true,true" {
		amiss = append(amiss, "a narrow window: over the sections, the sections alone, within the window = "+got)
	}
	// Paper: no contents, the page as wide as the paper.
	b.size(718, 1000, false)
	b.media("print")
	if got := b.eval("getComputedStyle(document.querySelector('[data-floe-look~=toc]')).display + ' ' + (document.querySelector('main').getBoundingClientRect().width === document.documentElement.clientWidth)"); got != "none true" {
		amiss = append(amiss, "paper: "+got)
	}
	b.media("")
	// A page without contents keeps its column.
	b.size(1920, 1000, false)
	s.floe(t, "publish", s.write(t, "plain.yaml", strings.Replace(contentsPage, "lead:", "toc: false\nlead:", 1)), "--slug", "plain")
	b.navigate("/a/plain/?raw")
	if got := b.eval("!!document.querySelector('[data-floe-look~=toc]') + ' ' + (document.querySelector('main').getBoundingClientRect().width < innerWidth)"); got != "false true" {
		amiss = append(amiss, "toc: false: contents, a narrower column = "+got)
	}
	// design marks its section too.
	b.size(1440, 900, false)
	design, err := os.ReadFile("../corpus/testdata/example/design.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s.floe(t, "publish", s.write(t, "design.yaml", string(design)), "--slug", "design")
	b.navigate("/a/design/?raw")
	if got := scroll("document.getElementById('decisions').offsetTop"); got != "Options" {
		amiss = append(amiss, "design: scrolled to its options, marks "+got)
	}
	check(t, "contents", amiss, "")
}
