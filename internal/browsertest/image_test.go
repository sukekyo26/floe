package browsertest_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp/kb"
)

// pngOf is a png of w × h pixels of one colour.
func pngOf(t *testing.T, w, h int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: 120, B: 200, A: 255}) //nolint:gosec // G115: below 256
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

const imagePage = `template: report
lang: en
title: Images
sections:
  - heading: Images
    parts:
      - image: { file: shots/a.png, alt: A, caption: "Figure 1: the list at ` + "`a`" + `" }
      - image: { file: shots/b.png, alt: B }
      - image: { file: shots/c.png, alt: C }
      - image: { file: shots/small.png, alt: Small }
      - image: { file: shots/shape.svg, alt: Shape }
      - image: { file: shots/broken.png, alt: Broken }
`

// zoomState tells of the frame of the image of the alt text named: how many toolbars are before it and whether the
// first shows, its tab stop, role, name and touch-action, and its image's transform.
func zoomState(alt string) string {
	return `(() => {
  const f = document.querySelector('img[alt="` + alt + `"]').closest('.floe-zoom');
  const tools = f.parentElement.querySelectorAll('.floe-zoom-tools');
  const t = tools[0];
  return [tools.length, t ? (t.hidden ? 'hidden ' : 'shown ') + getComputedStyle(t).display : '-',
    'tab ' + (f.hasAttribute('tabindex') ? f.tabIndex : 'none'), 'role ' + f.getAttribute('role'),
    'label ' + f.getAttribute('aria-label'), 'touch ' + getComputedStyle(f).touchAction,
    'transform ' + getComputedStyle(f.querySelector('img')).transform].join(', ');
})()`
}

// zoomOfA is the scale of the image A: 1 while it is not transformed.
const zoomOfA = `new DOMMatrix(getComputedStyle(document.querySelector('img[alt=A]')).transform).a.toFixed(3)`

// frameOf is the frame of the image of the alt text named.
func frameOf(alt string) string {
	return `document.querySelector('img[alt="` + alt + `"]').closest('.floe-zoom')`
}

// TestUIImageZoom draws images that are 600 px wide, a small one, an svg with a viewBox alone and a broken one. As wide
// as a phone, each of the 600 px ones has one toolbar, though its script and the other images' found it loading,
// and zooms by the buttons, the keys and a drag with the mouse that the browser does not take for dragging the image;
// printed, it shows whole. The others, and the 600 px ones on a wide screen, offer no zoom. Zoomed as wide as a phone
// and then widened, a frame keeps offering it until shown whole, when the focus is kept on the frame.
func TestUIImageZoom(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for _, name := range []string{"a", "b", "c"} {
		s.write(t, "zoom/shots/"+name+".png", pngOf(t, 600, 300))
	}
	s.write(t, "zoom/shots/small.png", pngOf(t, 40, 20))
	s.write(t, "zoom/shots/shape.svg", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 300"><rect width="100" height="300"/></svg>`)
	s.write(t, "zoom/shots/broken.png", "not a png")
	s.floe(t, "publish", s.write(t, "zoom/images.yaml", imagePage), "--slug", "images")

	// Every image is loading when the scripts set up its frame, until the test lets them load: a frame each script
	// finds waits once, not once a script
	id := b.addScript(`Object.defineProperty(HTMLImageElement.prototype, 'complete', { configurable: true, get() { return !!window.released; } })`)
	b.size(390, 844, true)
	b.navigate("/a/images/?raw")
	b.removeScript(id)
	b.eval(`window.released = true, document.querySelectorAll('.floe-zoom img').forEach((i) => i.dispatchEvent(new Event('load'))), 1`)
	b.until("the frames set up", `document.querySelectorAll('.floe-zoom[data-zoomable=on]').length === 5`)
	b.eval(`window.dragged = false, addEventListener('dragstart', () => { window.dragged = true; }), 1`)

	offers := "1, shown flex, tab 0, role group, label Zoomable figure, touch pan-y, transform none"
	offersNot := "1, hidden none, tab -1, role null, label null, touch auto, transform none"
	for alt, want := range map[string]string{
		"A": offers, "B": offers, "C": offers, "Small": offersNot, "Shape": offersNot,
		"Broken": "0, -, tab none, role null, label null, touch auto, transform none",
	} {
		if got := b.eval(zoomState(alt)); got != want {
			t.Errorf("%s at 390 px: %s, want %s", alt, got, want)
		}
	}
	if got := b.eval(frameOf("Shape") + `.clientWidth === document.querySelector('img[alt=Shape]').offsetWidth`); got != "true" {
		t.Error("the svg with a viewBox alone is not as wide as its frame")
	}

	button := func(alt, k string) string {
		return frameOf(alt) + `.parentElement.querySelector('[data-zoom=` + k + `]')`
	}
	b.eval(button("A", "in") + ".click(), 1")
	if got := b.eval(zoomOfA); got != "1.250" {
		t.Errorf("zoomed in once: scale %s, want 1.250", got)
	}
	height := b.eval(frameOf("A") + ".offsetHeight")
	b.eval(button("A", "one") + ".click(), 1")
	// The whole is the image as laid out, 600 px wide inside its frame line
	if got, want := b.eval(zoomOfA), b.eval(`(600 / document.querySelector('img[alt=A]').clientWidth).toFixed(3)`); got != want {
		t.Errorf("at actual size: scale %s, want %s", got, want)
	}
	if got := b.eval(frameOf("A") + ".offsetHeight"); got != height {
		t.Errorf("at actual size: the frame is %s px high, want %s", got, height)
	}

	// The arrow keys move the view by 40 px, and 0 shows the whole
	b.eval(frameOf("A") + ".focus(), 1")
	left := `new DOMMatrix(getComputedStyle(document.querySelector('img[alt=A]')).transform).e`
	b.eval("window.left = " + left + ", 1")
	b.pressKey(kb.ArrowRight)
	if got := b.eval("(window.left - " + left + ").toFixed(1)"); got != "40.0" {
		t.Errorf("an arrow key moved the image %s px left, want 40.0", got)
	}
	b.pressKey("0")
	if got := b.eval(zoomState("A")); got != offers {
		t.Errorf("0 pressed: %s, want %s", got, offers)
	}

	// A mouse moves a zoomed image, and the browser does not drag the image away
	b.eval(button("A", "one") + ".click(), 1")
	r := b.rect(frameOf("A"))
	x, y := r.X+r.Width/2, r.Y+r.Height/2
	before := b.eval(`getComputedStyle(document.querySelector('img[alt=A]')).transform`)
	b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, 1)
	b.mouse(input.DispatchMouseEventTypeMouseMoved, x-30, y, 0)
	b.mouse(input.DispatchMouseEventTypeMouseMoved, x-60, y, 0)
	b.mouse(input.DispatchMouseEventTypeMouseReleased, x-60, y, 1)
	if got := b.eval(`getComputedStyle(document.querySelector('img[alt=A]')).transform`); got == before {
		t.Error("a drag with the mouse did not move the zoomed image")
	}
	// Chrome does not start a drag of its own from the mouse events the test sends, so the image is asked
	if b.eval("window.dragged") != "false" || b.eval(`document.querySelector('img[alt=A]').draggable`) != "false" {
		t.Error("the browser may drag the image away")
	}

	// Printed, the image shows whole; after, as zoomed as before
	zoomed := b.eval(zoomOfA)
	b.media("print")
	b.eval(`dispatchEvent(new Event('beforeprint')), 1`)
	if got := b.eval(zoomOfA); got != "1.000" {
		t.Errorf("printed: scale %s, want 1.000", got)
	}
	b.media("")
	b.eval(`dispatchEvent(new Event('afterprint')), 1`)
	if got := b.eval(zoomOfA); got != zoomed {
		t.Errorf("after printing: scale %s, want %s", got, zoomed)
	}

	// Zoomed and widened till it fits, a frame still offers zoom; shown whole, it offers none, the focus on the frame,
	// whether the key 0 or the button showed it whole
	b.eval(button("B", "one") + ".click(), 1")
	b.size(1440, 900, false)
	// The frames are measured when their ResizeObserver tells them, after the window widens
	b.until("C widened", zoomState("C")+" === "+strconv.Quote(offersNot))
	for _, alt := range []string{"A", "B"} {
		if got := b.eval(zoomState(alt)); got == offersNot || got[:8] != "1, shown" {
			t.Errorf("%s zoomed and widened: %s, want its toolbar shown", alt, got)
		}
	}
	b.eval(frameOf("A") + ".focus(), 1")
	b.pressKey("+")
	b.pressKey("0")
	if got := b.eval(`document.activeElement === ` + frameOf("A")); got != "true" {
		t.Error("A shown whole by the key 0: the focus left the frame")
	}
	b.eval(button("B", "fit") + ".focus(), " + button("B", "fit") + ".click(), 1")
	for _, alt := range []string{"A", "B"} {
		if got := b.eval(zoomState(alt)); got != offersNot {
			t.Errorf("%s shown whole at 1440 px: %s, want %s", alt, got, offersNot)
		}
	}
	if got := b.eval(`document.activeElement === ` + frameOf("B")); got != "true" {
		t.Error("B shown whole by its button: the focus is not on the frame")
	}
}
