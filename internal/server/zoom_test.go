package server_test

import (
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// A diagram fits its frame, zooms with the buttons and Ctrl + wheel, pans by dragging, fits again, sets
// up once however often its assets are included, and words its toolbar in the page's language. Ported
// from the zoom scenario of the JS skill's test/artifact-ui.mjs.
func TestUIZoom(t *testing.T) {
	t.Parallel()
	inEveryLanguage(t, uiZoom)
}

// ctrl is the Ctrl key in the modifiers of an input event.
const ctrl = 2

func uiZoom(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	e := newEnv(t)
	b := newBrowser(t, e.base, lang)
	page := diagramPage(t, lang)
	slug, l := "diagram", string(lang)
	e.publish(t, e.write(t, "diagram.html", page), store.Options{Slug: &slug, Lang: &l})
	metrics := emulation.SetDeviceMetricsOverrideParams{Width: 1000, Height: 800, DeviceScaleFactor: 1}
	if _, err := chromedp.Call(b.ctx, emulation.SetDeviceMetricsOverride, metrics); err != nil {
		t.Fatal(err)
	}
	b.navigate("/a/diagram/?raw")

	view := func() string { return b.eval("document.querySelector('.dg').getAttribute('viewBox')") }
	click := func(k string) { b.eval("document.querySelector('.dg-tools [data-zoom=" + k + "]').click()") }
	// at is the diagram point under (ax, ay), in px from the top left inside the frame.
	at := func(ax, ay float64) [2]float64 {
		var p [2]float64
		s := b.eval(fmt.Sprintf(`(() => {
			const v = document.querySelector('.dg').viewBox.baseVal, w = document.querySelector('.dg-wrap');
			return [v.x + %v * v.width / w.clientWidth, v.y + %v * v.height / w.clientHeight].join(' ');
		})()`, ax, ay))
		if _, err := fmt.Sscan(s, &p[0], &p[1]); err != nil {
			t.Fatal(err)
		}
		return p
	}
	origin := b.eval(`(() => {
		const w = document.querySelector('.dg-wrap');
		w.scrollIntoView();
		const r = w.getBoundingClientRect();
		return (r.left + w.clientLeft) + ' ' + (r.top + w.clientTop);
	})()`)
	var ox, oy float64
	if _, err := fmt.Sscan(origin, &ox, &oy); err != nil {
		t.Fatal(err)
	}
	// mouse sends a mouse event at p.X, p.Y inside the frame.
	mouse := func(p input.DispatchMouseEventParams) {
		p.X += ox
		p.Y += oy
		if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, p); err != nil {
			t.Fatal(err)
		}
	}
	// wheel sends a wheel event. Its params are a map: the typed ones leave out a deltaX of 0, which
	// Chrome requires.
	wheel := func(x, y, deltaY float64, modifiers int64) {
		cmd := cdp.Command[map[string]any, cdp.Empty]{Method: input.CommandDispatchMouseEvent}
		p := map[string]any{"type": "mouseWheel", "x": ox + x, "y": oy + y, "deltaX": 0, "deltaY": deltaY, "modifiers": modifiers}
		if _, err := chromedp.Call(b.ctx, cmd, p); err != nil {
			t.Fatal(err)
		}
	}
	near := func(a, b [2]float64) string {
		d := max(a[0]-b[0], b[0]-a[0], a[1]-b[1], b[1]-a[1])
		return strconv.FormatBool(d < 0.5)
	}

	out := make([]string, 0, 8)
	// The toolbar speaks the page's language.
	labels := b.eval("[...document.querySelectorAll('.dg-tools button')].map((b) => b.getAttribute('aria-label')).join('/')")
	hint := b.eval("document.querySelector('.dg-tools span').textContent")
	want := msg("diagram_zoom_in") + "/" + msg("diagram_zoom_out") + "/" + msg("diagram_zoom_fit_label") + "/" + msg("diagram_zoom_one_label")
	out = append(out, strconv.FormatBool(labels == want && hint == msg("diagram_zoom_hint")))
	fitted := view()
	// It fits whole: the view covers the 1248x512 canvas, and the fit button is off.
	out = append(out, b.eval("(() => { const v = document.querySelector('.dg').viewBox.baseVal; "+
		"return v.width >= 1248 && v.height >= 512 && document.querySelector('.dg-tools [data-zoom=fit]').disabled; })()"))
	click("in")
	out = append(out, b.eval("(() => { const v = document.querySelector('.dg').viewBox.baseVal; "+
		"return Math.round("+js(fitted)+".split(' ')[2] / v.width * 100) + ' ' + document.querySelector('.dg-wrap').classList.contains('zoomed'); })()"))
	// Ctrl + wheel zooms in about the pointer: the diagram point under it stays put.
	before := at(200, 100)
	wheel(200, 100, -200, ctrl)
	zoomed := view()
	out = append(out, strconv.FormatBool(zoomed != fitted && near(before, at(200, 100)) == "true"))
	// A plain wheel scrolls the page and leaves the view alone.
	wheel(200, 100, 100, 0)
	out = append(out, strconv.FormatBool(view() == zoomed))
	// Dragging moves the diagram with the pointer.
	grabbed := at(300, 150)
	left := input.MouseButtonLeft
	mouse(input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMousePressed, X: 300, Y: 150, Button: left, ClickCount: 1})
	mouse(input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseMoved, X: 200, Y: 100, Button: left, Buttons: 1})
	mouse(input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseReleased, X: 200, Y: 100, Button: left, ClickCount: 1})
	out = append(out, near(grabbed, at(200, 100)))
	click("fit")
	out = append(out, strconv.FormatBool(view() == fitted))
	// The assets script run a second time (a page that includes them twice) adds no second toolbar.
	out = append(out, b.eval("(0, eval)([...document.scripts].find((s) => s.textContent.includes('zoomable')).textContent), "+
		"document.querySelectorAll('.dg-tools').length"))

	check(t, "zoom", out, "true|true|125 true|true|true|true|true|1")
}

// diagramPage is a body-only page of the architecture example in lang with its assets, as floe diagram
// render prints it.
func diagramPage(t *testing.T, lang i18n.Lang) string {
	t.Helper()

	examples, err := diagram.Examples(lang)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(examples, func(e diagram.Example) bool { return e.Name == "architecture" })
	s, err := diagram.Parse("architecture.yaml", examples[i].Spec)
	if err != nil {
		t.Fatal(err)
	}
	svg, err := diagram.Render([]*diagram.Spec{s})
	if err != nil {
		t.Fatal(err)
	}
	assets, err := diagram.Assets()
	if err != nil {
		t.Fatal(err)
	}
	return assets + "\n<section><h2>Architecture</h2>" + svg + "</section>\n"
}
