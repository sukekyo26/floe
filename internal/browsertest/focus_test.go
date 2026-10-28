package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// ringOf is the script that tells the focus ring of the focused element: its style, width, color and offset, and the
// contrast of its color against the ground it is drawn on, the backgrounds around the element laid one over another on
// the root's as measureContrast lays them. A ring off the element (offset 0 or more) is drawn on the ground outside it;
// one inside it (a row's), on its own. A row's title is outlined on its row.
const ringOf = `(() => {
  const rgba = (c) => { const m = c.match(/[\d.]+/g).map(Number); return [m[0], m[1], m[2], m.length > 3 ? m[3] : 1]; };
  const over = (top, under) => {
    const a = top[3] + under[3] * (1 - top[3]);
    return [0, 1, 2].map((i) => (top[i] * top[3] + under[i] * under[3] * (1 - top[3])) / a).concat(a);
  };
  const luminance = (c) => {
    const [r, g, b] = c.slice(0, 3).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  let e = document.activeElement;
  if (e.matches('a.title')) e = e.closest('li');
  if (e.matches('.tag-box input')) e = e.closest('.tag-box');
  const s = getComputedStyle(e);
  const offset = parseFloat(s.outlineOffset);
  const chain = [];
  for (let x = offset < 0 ? e : e.parentElement; x; x = x.parentElement) chain.unshift(x);
  let ground = [255, 255, 255, 1];
  for (const x of chain) ground = over(rgba(getComputedStyle(x).backgroundColor), ground);
  const [hi, lo] = [luminance(rgba(s.outlineColor)), luminance(ground)].sort((p, q) => q - p);
  const ratio = (hi + 0.05) / (lo + 0.05);
  return [s.outlineStyle, s.outlineWidth, s.outlineColor, s.outlineOffset, ratio >= 3 ? 'seen' : 'faint ' + ratio.toFixed(2)].join(' ');
})()`

// TestUIFocusRing presses the list's controls with the mouse, which shows no ring, and reaches them by keyboard, which
// shows the one ring of the page: 2px of --faint, 1px off a control (so that it never touches a filled one), 2px inside a
// row, and seen against the ground it is drawn on.
// LIMIT: the sort is a select until it becomes a menu, and Chrome rings a select pressed with the mouse; so it is reached
// by keyboard here and not pressed.
func TestUIFocusRing(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	b.eval(stub(colorsList))
	b.until("the rows", "document.querySelectorAll('#list li:not(.group)').length > 0")

	// Pressed with the mouse: no outline, and the shadow it has without the focus.
	pressed := []string{
		".tabs button[data-tab=links]", ".tabs button[data-tab=pages]", ".views button[data-view=fav]", ".views button[data-view=all]",
		"#list li:not(.group) .sel", "#list li:not(.group) .star",
	}
	out := make([]string, 0, 16)
	for _, sel := range pressed {
		r := b.rect("document.querySelector(" + js(sel) + ")")
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
		focused := b.eval("(() => { const e = document.activeElement; const s = getComputedStyle(e); " +
			"return [e.matches(" + js(sel) + "), s.outlineStyle, s.boxShadow].join(' '); })()")
		b.eval("document.activeElement.blur()")
		unfocused := b.eval("(() => { const e = document.querySelector(" + js(sel) + "); const s = getComputedStyle(e); " +
			"return ['true', s.outlineStyle, s.boxShadow].join(' '); })()")
		out = append(out, sel+": "+strings.SplitN(focused, " ", 3)[1]+" "+strconv.FormatBool(focused == unfocused))
	}
	check(t, "pressed", out, strings.Join([]string{
		".tabs button[data-tab=links]: none true", ".tabs button[data-tab=pages]: none true",
		".views button[data-view=fav]: none true", ".views button[data-view=all]: none true",
		"#list li:not(.group) .sel: none true", "#list li:not(.group) .star: none true",
	}, "|"))

	// Reached by keyboard: Tab from the top of the page until the control has the focus.
	tabTo := func(sel string) string {
		t.Helper()

		b.eval("document.activeElement.blur()")
		for range 80 {
			if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Tab)); err != nil {
				t.Fatal(err)
			}
			if b.eval("String(document.activeElement.matches("+js(sel)+"))") == "true" {
				return sel + ": " + b.eval(ringOf)
			}
		}
		t.Fatalf("Tab does not reach %s", sel)
		return ""
	}
	// The shelf's pressed button is filled with ink, the bulk delete with danger: the ring stays off them. A row selected
	// puts the bulk bar in the place of the kinds and the sort, so it is selected last.
	b.eval("[...document.querySelectorAll('#list li:not(.group) .sel')].filter((s) => s.checked).forEach((s) => s.click())")
	out = out[:0]
	for _, sel := range []string{
		".views button[data-view=all]", ".tabs button[data-tab=pages]", "#sort", "#q", "#list li:not(.group) a.title",
		"#list li:not(.group) .sel",
	} {
		out = append(out, tabTo(sel))
	}
	b.eval("document.querySelector('#list li:not(.group) .sel').click()")
	out = append(out, tabTo("#bulk-delete"))
	b.eval("document.getElementById('bulk-tag').click()")
	b.until("the tags dialog", "document.getElementById('tag-dialog').open")
	b.eval("document.activeElement.blur()")
	out = append(out, tabTo(".tag-box input"))
	ring := "solid 2px rgb(107, 105, 100) 1px seen"
	check(t, "reached", out, strings.Join([]string{
		".views button[data-view=all]: " + ring, ".tabs button[data-tab=pages]: " + ring, "#sort: " + ring, "#q: " + ring,
		"#list li:not(.group) a.title: solid 2px rgb(107, 105, 100) -2px seen", "#list li:not(.group) .sel: " + ring,
		"#bulk-delete: " + ring, ".tag-box input: " + ring,
	}, "|"))
	// The title itself has none, so that a row shows one ring.
	b.eval("document.querySelector('#tag-dialog [data-close]').click()")
	tabTo("#list li:not(.group) a.title")
	check(t, "the title", []string{b.eval("getComputedStyle(document.activeElement).outlineStyle")}, "none")
}
