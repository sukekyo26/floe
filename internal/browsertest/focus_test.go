package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// ringOf is the script that tells the focus ring of the focused element: its style, width, color and offset; the
// contrast of its color against the ground it is drawn on, the backgrounds around the element laid one over another on
// the root's as measureContrast lays them; and what of another face its ring reaches. A ring off the element (offset 0 or
// more) is drawn on the ground outside it, and must leave 1px of that ground before any face it would fade into (a
// background, or an inset ring of a box it is in, less than 3:1 against it, a face under another left out); one inside it
// (a row's), on its own. A row's
// title is outlined on its row, a tag box's hidden checkbox on its box.
const ringOf = `(() => {
  // A computed colour, rgb(), rgba() or color(srgb …), as [r, g, b] of 255 and its alpha.
  const rgba = (colour) => {
    let m = colour.match(/^rgba?\(([\d.]+), ([\d.]+), ([\d.]+)(?:, ([\d.]+))?\)$/);
    if (m) return [Number(m[1]), Number(m[2]), Number(m[3]), m[4] === undefined ? 1 : Number(m[4])];
    m = colour.match(/^color\(srgb ([\d.e-]+) ([\d.e-]+) ([\d.e-]+)(?: \/ ([\d.e-]+))?\)$/);
    if (m) return [Number(m[1]) * 255, Number(m[2]) * 255, Number(m[3]) * 255, m[4] === undefined ? 1 : Number(m[4])];
    throw new Error('a colour not read: ' + colour);
  };
  const over = (top, under) => {
    const a = top[3] + under[3] * (1 - top[3]);
    return [0, 1, 2].map((i) => (top[i] * top[3] + under[i] * under[3] * (1 - top[3])) / a).concat(a);
  };
  const luminance = (c) => {
    const [r, g, b] = c.slice(0, 3).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const ratio = (x, y) => { const [hi, lo] = [luminance(x), luminance(y)].sort((p, q) => q - p); return (hi + 0.05) / (lo + 0.05); };
  let e = document.activeElement;
  if (e.matches('a.title')) e = e.closest('li');
  if (e.matches('.tag-box input')) e = e.closest('.tag-box');
  const s = getComputedStyle(e);
  const ring = rgba(s.outlineColor);
  const offset = parseFloat(s.outlineOffset);
  const chain = [];
  for (let x = offset < 0 ? e : e.parentElement; x; x = x.parentElement) chain.unshift(x);
  let ground = [255, 255, 255, 1];
  for (const x of chain) ground = over(rgba(getComputedStyle(x).backgroundColor), ground);
  const out = [s.outlineStyle, s.outlineWidth, s.outlineColor, s.outlineOffset, ratio(ring, ground) >= 3 ? 'seen' : 'faint ' + ratio(ring, ground).toFixed(2)];
  if (offset >= 0) {
    // The ring's outer edge, and 1px of ground past it.
    const r = e.getBoundingClientRect();
    const reach = offset + parseFloat(s.outlineWidth) + 1;
    const box = { left: r.left - reach, top: r.top - reach, right: r.right + reach, bottom: r.bottom + reach };
    const name = (x) => x.id ? '#' + x.id : x.tagName.toLowerCase() + (x.className ? '.' + String(x.className).split(' ')[0] : '');
    for (const x of document.querySelectorAll('body *')) {
      if (x === e || x.contains(e) || e.contains(x) || !x.checkVisibility()) continue;
      const bg = rgba(getComputedStyle(x).backgroundColor);
      if (bg[3] === 0 || ratio(ring, over(bg, ground)) >= 3) continue;
      const q = x.getBoundingClientRect();
      // Under another (the header bar over the list scrolled under it), it is not seen
      const top = document.elementFromPoint((q.left + q.right) / 2, (q.top + q.bottom) / 2);
      if (!top || !x.contains(top)) continue;
      if (q.left < box.right && q.right > box.left && q.top < box.bottom && q.bottom > box.top) out.push('reaches ' + name(x));
    }
    for (const x of chain) {
      const inset = getComputedStyle(x).boxShadow.match(/inset ([\d.-]+)px ([\d.-]+)px ([\d.-]+)px ([\d.-]+)px/)
        ?? getComputedStyle(x).boxShadow.match(/([\d.-]+)px ([\d.-]+)px ([\d.-]+)px ([\d.-]+)px inset/);
      if (!inset) continue;
      const q = x.getBoundingClientRect();
      const spread = Number(inset[4]);
      if (box.left < q.left + spread || box.right > q.right - spread || box.top < q.top + spread || box.bottom > q.bottom - spread) {
        out.push('reaches the inset ring of ' + name(x));
      }
    }
  }
  return out.join(' ');
})()`

// TestUIFocusRing presses the list's controls with the mouse, which shows no ring, and reaches them by keyboard, which
// shows the one ring of the page: 2px of --faint, 1px off a control with 1px of ground past it before any face it would
// fade into (the pressed shelf button beside, the bulk bar's ink), 2px inside a row, and seen against the ground it is drawn
// on.
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

	// Reached by keyboard: Tab from the top of the page (focus is put on the body, where Tab goes on from) until the control
	// has the focus.
	tabTo := func(sel string) string {
		t.Helper()

		b.eval("document.body.tabIndex = -1; document.body.focus(); document.body.removeAttribute('tabindex')")
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
		".views button[data-view=all]", ".views button[data-view=fav]", ".tabs button[data-tab=pages]", "#sort", "#q",
		"#list li:not(.group) a.title", "#list li:not(.group) .sel",
	} {
		out = append(out, tabTo(sel))
	}
	// In the bulk bar, ringed in ink: the clearing at its end, and the delete, pressed once too, which shows its darker face
	// and no ring but the focus ring.
	b.eval("document.querySelector('#list li:not(.group) .sel').click()")
	out = append(out, tabTo("#bulk-clear"), tabTo("#bulk-delete"))
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Enter)); err != nil {
		t.Fatal(err)
	}
	b.until("the delete to be armed", "document.getElementById('bulk-delete').classList.contains('armed')")
	out = append(out, "#bulk-delete.armed: "+b.eval(ringOf))
	b.eval("document.getElementById('bulk-clear').click()")
	b.eval("document.querySelector('#list li:not(.group) .sel').click()")
	b.eval("document.getElementById('bulk-tag').click()")
	b.until("the tags dialog", "document.getElementById('tag-dialog').open")
	out = append(out, tabTo(".tag-box input"))
	ring := "solid 2px rgb(107, 105, 100) 1px seen"
	check(t, "reached", out, strings.Join([]string{
		".views button[data-view=all]: " + ring, ".views button[data-view=fav]: " + ring, ".tabs button[data-tab=pages]: " + ring,
		"#sort: " + ring, "#q: " + ring, "#list li:not(.group) a.title: solid 2px rgb(107, 105, 100) -2px seen",
		"#list li:not(.group) .sel: " + ring, "#bulk-clear: " + ring, "#bulk-delete: " + ring, "#bulk-delete.armed: " + ring,
		".tag-box input: " + ring,
	}, "|"))
	// The title itself has none, so that a row shows one ring.
	b.eval("document.querySelector('#tag-dialog [data-close]').click()")
	tabTo("#list li:not(.group) a.title")
	check(t, "the title", []string{b.eval("getComputedStyle(document.activeElement).outlineStyle")}, "none")
}
