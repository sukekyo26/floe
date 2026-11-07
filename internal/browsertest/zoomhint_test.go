package browsertest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/i18n"
)

// zoomHintPage is two frames that zoom at any width, an image that zooms only on a narrow screen, one that never does, and
// five columns of an image and a diagram.
func zoomHintPage(lang i18n.Lang) string {
	var col strings.Builder
	for range 5 {
		col.WriteString("          - parts:\n              - image: { file: shots/a.png, alt: In a column }\n" +
			"              - diagram: { width: 900, height: 400, title: d, parts: [{ box: { x: 20, y: 20, w: 100, h: 40, rows: [a] } }] }\n")
	}
	return "lang: " + string(lang) + "\ntemplate: report\ntitle: Z\nsections:\n  - heading: A\n    parts:\n" +
		"      - image: { file: shots/a.png, alt: Across }\n      - image: { file: shots/mid.png, alt: Mid }\n" +
		"      - image: { file: shots/small.png, alt: Small }\n" +
		"      - diagram: { width: 3000, height: 400, title: d, parts: [{ box: { x: 20, y: 20, w: 100, h: 40, rows: [a] } }] }\n" +
		"      - columns:\n" + col.String()
}

// measureZoomHint tells how many images and diagrams offer to zoom and how many do not, and of those that do what their
// toolbars hold but buttons in one line, a title that is not hint, a line not hint, or one shown; of those that do not, a
// title or a description left on them.
const measureZoomHint = `((hint) => {
  const out = [];
  const count = { img: 0, svg: 0, not: 0 };
  for (const f of document.querySelectorAll('.floe-zoom')) {
    const tools = f.previousElementSibling, line = f.nextElementSibling;
    if (tools.hidden) {
      count.not++;
      if (f.hasAttribute('title') || f.hasAttribute('aria-describedby')) out.push('a frame that does not zoom keeps ' + f.title);
      continue;
    }
    count[f.firstElementChild.localName]++;
    if (f.title !== hint) out.push('title ' + f.title);
    if (line.textContent !== hint || f.getAttribute('aria-describedby') !== line.id) out.push('line ' + line.textContent);
    if (getComputedStyle(line).display !== 'none') out.push('a line shown with no focus');
    if ([...tools.children].some((e) => e.tagName !== 'BUTTON') || tools.children.length !== 4) out.push('holds ' + tools.innerHTML);
    const tops = new Set([...tools.children].map((b) => Math.round(b.getBoundingClientRect().top)));
    if (tops.size !== 1) out.push('in ' + tops.size + ' lines at ' + f.getBoundingClientRect().width + ' px');
  }
  const ids = [...document.querySelectorAll('.floe-zoom-hint')].map((e) => e.id);
  if (new Set(ids).size !== ids.length) out.push('ids alike');
  return [count.img + ' img ' + count.svg + ' svg ' + count.not + ' not', ...new Set(out)].join('\n');
})`

// zoomHintShown tells whether the line of the frame with the focus shows whole, under it, in the frame's width.
const zoomHintShown = `(() => {
  const f = document.activeElement, line = f.nextElementSibling;
  if (!line?.matches('.floe-zoom-hint') || getComputedStyle(line).display === 'none') return 'not shown';
  const a = f.getBoundingClientRect(), b = line.getBoundingClientRect();
  return b.top >= a.bottom && b.height > 0 && b.left >= a.left - 0.5 && b.right <= a.right + 0.5 && line.scrollWidth <= line.clientWidth ? 'shown' : 'not whole';
})()`

// atLeastColumns sizes the page so that its columns are as narrow as they go, five of 16rem, its scrollbars hidden, which
// the page's height and Chrome's version would otherwise take out of the width or not; the caller shows them again.
func atLeastColumns(t *testing.T, b *browser) {
	t.Helper()

	b.scrollbars(true)
	wide := b.eval(`(() => { const c = document.querySelector('[data-floe-part=columns]'), g = parseFloat(getComputedStyle(c).columnGap),
	  rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
	  return String(Math.ceil(5 * 16 * rem + 4 * g + document.documentElement.clientWidth - c.getBoundingClientRect().width)); })()`)
	var width int64
	if _, err := fmt.Sscan(wide, &width); err != nil {
		t.Fatal(err)
	}
	b.size(width, 900, false)
	b.until("the columns at their least", `Math.abs(document.querySelector('[data-floe-part=columns] > div').getBoundingClientRect().width - 16 * parseFloat(getComputedStyle(document.documentElement).fontSize)) < 1`)
}

// frameDescriptions lists the description of each frame named as one to zoom, as Chrome's accessibility tree gives it.
func frameDescriptions(t *testing.T, b *browser, name string) map[string]int {
	t.Helper()

	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]int{}
	for _, n := range tree.Nodes {
		if n.Ignored || n.Name == nil {
			continue
		}
		var got, description string
		if json.Unmarshal(n.Name.Value, &got) != nil || got != name {
			continue
		}
		if n.Description != nil && json.Unmarshal(n.Description.Value, &description) != nil {
			t.Fatal("a description that is not text")
		}
		found[description]++
	}
	return found
}

// TestUIZoomHint shows the toolbar of a frame that zooms as its buttons alone, in one line in the narrowest columns too,
// and says how it is used in its title and description, and to the keyboard in a line over it while it has the focus, not
// to the mouse. A frame that comes to fit, the page widened, takes them off.
func TestUIZoomHint(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, func(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
		t.Helper()

		s := newSite(t)
		b := newBrowser(t, s.Base, lang)
		s.write(t, "z/shots/a.png", pngOf(t, 3000, 300))
		s.write(t, "z/shots/mid.png", pngOf(t, 600, 300))
		s.write(t, "z/shots/small.png", pngOf(t, 40, 20))
		s.floe(t, "publish", s.write(t, "z/z.yaml", zoomHintPage(lang)), "--slug", "z")
		hint, err := json.Marshal(msg("zoom_hint"))
		if err != nil {
			t.Fatal(err)
		}
		measure := measureZoomHint + "(" + string(hint) + ")"
		b.size(400, 900, false)
		b.navigate("/a/z/?raw")
		b.until("the frames set up", `document.querySelectorAll('.floe-zoom[data-zoomable=on]').length === 14`)
		// Narrow, the 600 px image offers too.
		if got, want := b.eval(measure), "7 img 6 svg 1 not"; got != want {
			t.Errorf("at 400 px: %s, want %s", got, want)
		}
		if got := frameDescriptions(t, b, msg("zoom_frame")); got[msg("zoom_hint")] != 13 || len(got) != 1 {
			t.Errorf("the frames' descriptions: %v", got)
		}
		// Widened as the page stays, the same image fits and takes them off.
		b.size(1440, 900, false)
		b.until("the wide image fit", `document.querySelector('img[alt=Mid]').closest('.floe-zoom').previousElementSibling.hidden`)
		if got, want := b.eval(measure), "6 img 6 svg 2 not"; got != want {
			t.Errorf("at 1440 px: %s, want %s", got, want)
		}
		// Five columns at their least width.
		atLeastColumns(t, b)
		if got, want := b.eval(measure), "6 img 6 svg 2 not"; got != want {
			t.Errorf("at the columns' least width: %s, want %s", got, want)
		}
		b.scrollbars(false)
		b.size(1440, 900, false)

		// Tab from the frame's last button reaches the frame; the line shows there and goes with the focus.
		b.eval(`document.querySelector('img[alt=Across]').closest('.floe-zoom').previousElementSibling.lastElementChild.focus(), 1`)
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Tab)); err != nil {
			t.Fatal(err)
		}
		if got := b.eval(zoomHintShown); got != "shown" {
			t.Errorf("the focus on the frame: the line is %s", got)
		}
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Tab)); err != nil {
			t.Fatal(err)
		}
		if got := b.eval(zoomHintShown); got != "not shown" {
			t.Errorf("the focus gone on: the line is %s", got)
		}
		// A press puts the focus on the frame without showing the line.
		b.eval(`document.activeElement.blur(), scrollTo(0, 0), 1`)
		box := b.eval(`(() => { const r = document.querySelector('img[alt=Across]').getBoundingClientRect(); return (r.left + 20) + ' ' + (r.top + 20); })()`)
		var x, y float64
		if _, err := fmt.Sscan(box, &x, &y); err != nil {
			t.Fatal(err)
		}
		b.click(x, y)
		if got := b.eval(`document.activeElement.classList.contains('floe-zoom') + ' ' + ` + zoomHintShown); got != "true not shown" {
			t.Errorf("pressed: %s", got)
		}
		// Zoomed by a key the line stays, as it hides nothing of the frame.
		b.eval(`document.querySelector('img[alt=Across]').closest('.floe-zoom').focus(), 1`)
		b.pressKey("+")
		if got := b.eval(`document.activeElement.classList.contains('zoomed') + ' ' + ` + zoomHintShown); got != "true shown" {
			t.Errorf("zoomed by a key: %s", got)
		}
		// Zoomed, a finger and a mouse move it: it takes the drag from the page and from selecting text, and shows it can.
		const zoomedLook = `(() => { const c = getComputedStyle(document.activeElement); return [c.cursor, c.touchAction, c.userSelect].join(' '); })()`
		if got, want := b.eval(zoomedLook), "grab none none"; got != want {
			t.Errorf("zoomed: %s, want %s", got, want)
		}
		held := b.eval(`(() => { const r = document.activeElement.getBoundingClientRect(); return (r.left + 20) + ' ' + (r.top + 20); })()`)
		if _, err := fmt.Sscan(held, &x, &y); err != nil {
			t.Fatal(err)
		}
		b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, 1)
		if got, want := b.eval(`getComputedStyle(document.activeElement).cursor`), "grabbing"; got != want {
			t.Errorf("held: %s, want %s", got, want)
		}
		b.mouse(input.DispatchMouseEventTypeMouseReleased, x, y, 1)
		b.unsize()
	})
}

// TestUIZoomHintOwnTitle keeps the title and the description a frame was written with, while it zooms and after.
func TestUIZoomHintOwnTitle(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.own(t, map[string]string{
		"parts/shot/schema.json":   `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"file": {"type": "string", "format": "image"}}}`,
		"parts/shot/part.yaml":     "looks: [zoom]\n",
		"parts/shot/part.css":      "",
		"parts/shot/part.html":     `<figure data-floe-part="shot">{{zoom}}<div class="floe-zoom" title="Mine" aria-describedby="mine"><img src="{{.file}}" alt="Own"></div><p id="mine">Own words</p></figure>`,
		"templates/m/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"shot": {"$ref": "floe:///parts/shot.json"}}}`,
		"templates/m/page.html":    "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\"><title>{{.Title}}</title></head>\n<body>{{.Data.shot}}</body></html>\n",
		"templates/m/page.css":     "",
		"templates/m/example.yaml": "lang: en\ntemplate: m\n",
	})
	s.write(t, "o/shots/mid.png", pngOf(t, 600, 300))
	s.floe(t, "publish", s.write(t, "o/o.yaml", "lang: en\ntemplate: m\ntitle: O\nshot: { file: shots/mid.png }\n"), "--slug", "o")
	const frame = `document.querySelector('img[alt=Own]').closest('.floe-zoom')`
	state := frame + `.title + ' ' + !` + frame + `.previousElementSibling.hidden + ' ' + ` + frame + `.getAttribute('aria-describedby')`
	b.size(400, 900, false)
	b.navigate("/a/o/?raw")
	b.until("the frame set up", frame+`.dataset.zoomable === 'on'`)
	if got, want := b.eval(state), "Mine true mine"; got != want {
		t.Errorf("zooming: %s, want %s", got, want)
	}
	b.size(1440, 900, false)
	b.until("the frame fit", frame+`.previousElementSibling.hidden`)
	if got, want := b.eval(state), "Mine false mine"; got != want {
		t.Errorf("fitted: %s, want %s", got, want)
	}
	b.unsize()
}

// TestUIZoomHintThemes measures the line of a frame as low as columns make it, as text on its ground, in every theme.
func TestUIZoomHintThemes(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "t/shots/a.png", pngOf(t, 3000, 300))
	var col strings.Builder
	for range 5 {
		col.WriteString("          - parts: [ { image: { file: shots/a.png, alt: A } } ]\n")
	}
	file := s.write(t, "t/t.yaml", "lang: en\ntemplate: report\ntitle: T\nsections:\n  - parts:\n      - columns:\n"+col.String())
	for _, th := range themes {
		s.floe(t, "publish", file, "--slug", "t-"+th.Name(), "--theme", th.Name())
		b.navigate("/a/t-" + th.Name() + "/?raw")
		b.until("the frame set up", `document.querySelector('.floe-zoom[data-zoomable=on]')`)
		atLeastColumns(t, b)
		b.eval(`document.querySelector('.floe-zoom').focus(), 1`)
		if got := b.eval(zoomHintShown); got != "shown" {
			t.Errorf("%s: the line is %s", th.Name(), got)
		}
		if low := b.eval(measureContrast); low != "" {
			t.Errorf("%s: text that cannot be read:\n%s", th.Name(), low)
		}
		b.unsize()
		b.scrollbars(false)
	}
}
