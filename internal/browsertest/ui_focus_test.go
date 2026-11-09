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
// background, or an inset ring of a box it is in, less than 3:1 against it, a face under another left out), inside the
// nearest box round it that paints a ground; one inside it (a row's), on its own. A row's
// title is outlined on its row, a task's family row on its line, a tag box's hidden checkbox on its box.
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
  if (e.matches('.kin-row')) e = e.closest('.kin-line');
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
    // The ground it is drawn on ends where the nearest box painting one does: past that edge it would stand on another. The
    // root's and the body's paint the whole window, past their boxes
    // LIMIT: a ground is told by background-color alone; one of a background-image, a gradient or a ::before is not seen
    const painted = [...chain].reverse().find((x) => rgba(getComputedStyle(x).backgroundColor)[3] > 0);
    if (painted && painted !== document.documentElement && painted !== document.body) {
      const q = painted.getBoundingClientRect();
      if (box.left < q.left - 0.5 || box.right > q.right + 0.5 || box.top < q.top - 0.5 || box.bottom > q.bottom + 0.5) {
        out.push('leaves the ground of ' + name(painted));
      }
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
func TestUIFocusRing(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(colorsList)
	b.navigate("/")
	b.until("the rows", "document.querySelectorAll('#list li:not(.group)').length > 0")

	// Pressed with the mouse: no outline, and the shadow it has without the focus.
	pressed := []string{
		".views button[data-view=fav]", ".views button[data-view=all]", "#list li:not(.group) .star", "#list li:not(.group) .sel",
	}
	out := make([]string, 0, 16)
	for _, sel := range pressed {
		// A row selected, the check boxes show.
		if strings.HasSuffix(sel, ".sel") {
			b.eval("document.querySelector('#list li[data-slug=p9] .sel').click()")
		}
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
		".views button[data-view=fav]: none true", ".views button[data-view=all]: none true",
		"#list li:not(.group) .star: none true", "#list li:not(.group) .sel: none true",
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
				return sel + ": " + b.settled(ringOf)
			}
		}
		t.Fatalf("Tab does not reach %s", sel)
		return ""
	}
	// The bulk delete is filled with danger: the ring stays off it. The bulk bar shows with a row selected, so it is selected
	// last.
	b.eval("document.getElementById('bulk-clear').click()")
	out = out[:0]
	for _, sel := range []string{
		".views button[data-view=all]", ".views button[data-view=fav]", "#q",
		"#list li:not(.group) a.title",
	} {
		out = append(out, tabTo(sel))
	}
	// In the bulk bar, ringed in ink: the clearing at its end, and the delete, pressed once too, which shows its darker face
	// and no ring but the focus ring.
	b.eval("document.querySelector('#list li:not(.group) .sel').click()")
	out = append(out, tabTo("#list li:not(.group) .sel"), tabTo("#bulk-clear"), tabTo("#bulk-delete"))
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Enter)); err != nil {
		t.Fatal(err)
	}
	b.until("the delete to be armed", "document.getElementById('bulk-delete').classList.contains('armed')")
	out = append(out, "#bulk-delete.armed: "+b.settled(ringOf))
	b.eval("document.getElementById('bulk-clear').click()")
	b.eval("document.querySelector('#list li:not(.group) .sel').click()")
	b.eval("document.getElementById('bulk-tag').click()")
	b.until("the tags dialog", "document.getElementById('tag-dialog').open")
	out = append(out, tabTo(".tag-box input"))
	b.eval("document.getElementById('tag-dialog').close(); openFind()")
	out = append(out, tabTo("#find-actions .star"))
	b.eval("document.getElementById('find').close()")
	ring := "solid 2px rgb(107, 105, 100) 1px seen"
	check(t, "reached", out, strings.Join([]string{
		".views button[data-view=all]: " + ring, ".views button[data-view=fav]: " + ring,
		"#q: " + ring, "#list li:not(.group) a.title: solid 2px rgb(107, 105, 100) -2px seen",
		"#list li:not(.group) .sel: " + ring, "#bulk-clear: " + ring, "#bulk-delete: " + ring, "#bulk-delete.armed: " + ring,
		".tag-box input: " + ring, "#find-actions .star: " + ring,
	}, "|"))
	// The title itself has none, so that a row shows one ring.
	tabTo("#list li:not(.group) a.title")
	check(t, "the title", []string{b.eval("getComputedStyle(document.activeElement).outlineStyle")}, "none")
	// The buttons of the columns' feet, and what opens them folded, over a page too.
	b.size(1440, 900, false)
	defer b.unsize()
	b.until("the wide window", "!matchMedia('(max-width: 70rem)').matches")
	// The shelf's add button, in the shelf beside the list, ringed as its rows.
	check(t, "the shelf's add button", []string{tabTo("#add-link")}, "#add-link: "+ring)
	out = out[:0]
	out = append(out, tabTo("#shelf-hide"), tabTo("#list-hide"))
	b.eval("document.querySelector('#list li:not(.group) .title').click(); document.getElementById('shelf-hide').click()")
	out = append(out, tabTo("#shelf-show"))
	b.eval("document.getElementById('list-hide').click()")
	out = append(out, tabTo("#shelf-show"), tabTo("#list-show"))
	b.eval("document.getElementById('list-show').click(); document.getElementById('shelf-show').click()")
	// The bar's buttons, an archived one black.
	b.eval("document.querySelector('#list li:not(.group) .title').click()")
	b.until("the bar", "!document.getElementById('read-actions').hidden")
	b.eval("document.getElementById('read-title').focus()")
	for range 6 {
		if b.eval("String(document.activeElement.matches('#read-actions > .archive'))") == "true" {
			break
		}
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Tab)); err != nil {
			t.Fatal(err)
		}
	}
	bar := "#read-actions > .archive: " + b.settled(ringOf)
	b.eval("document.querySelector('#read-actions > .archive').classList.add('on')")
	barOn := b.settled(ringOf)
	b.eval("document.querySelector('#read-actions > .archive').classList.remove('on')")
	// The button of the tags under the bar, reached on from the bar, its ring within the line's ground.
	b.eval("document.querySelector('#read-actions > .archive').focus()")
	for range 6 {
		if b.eval("String(document.activeElement.matches('#read-tags .tag-edit'))") == "true" {
			break
		}
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Tab)); err != nil {
			t.Fatal(err)
		}
	}
	tagLine := "#read-tags .tag-edit: " + b.settled(ringOf)
	check(t, "the bar", []string{bar, barOn, tagLine}, "#read-actions > .archive: "+ring+"|"+ring+"|#read-tags .tag-edit: "+ring)
	check(t, "the feet", out, strings.Join([]string{
		"#shelf-hide: " + ring, "#list-hide: " + ring, "#shelf-show: " + ring, "#shelf-show: " + ring, "#list-show: " + ring,
	}, "|"))

	// A task's family, in its card: the parent's row, its button and the change over it on the child's page; the add over the
	// children on the parent's.
	b.unstub()
	s.floe(t, "work", "--project", "proj", "--task", "#1", "--title", "up")
	s.floe(t, "work", "--project", "proj", "--task", "#2", "--title", "down", "--parent", "#1")
	b.navigate("/tasks?project=proj&task=%232")
	b.until("the family", "document.querySelector('#task-up-rows .kin-row')")
	out = append(out[:0], tabTo("#task-parent-change"), tabTo("#task-up-rows .kin-row"), tabTo("#task-up-rows .unlink"))
	// The row's link itself has none, so that the row shows one ring.
	tabTo("#task-up-rows .kin-row")
	out = append(out, "the link: "+b.eval("getComputedStyle(document.activeElement).outlineStyle"))
	b.navigate("/tasks?project=proj&task=%231")
	b.until("the family", "document.querySelector('#task-family-rows .kin-row')")
	out = append(out, tabTo("#task-child"))
	// The frame's line of tags, its button's ring within the line's ground.
	s.floe(t, "publish", s.page(t, "ring/whole.yaml", "Whole", ""), "--slug", "whole")
	b.navigate("/a/whole/")
	b.until("the frame's tags", "!document.getElementById('frame-tags').hidden")
	out = append(out, tabTo("#frame-tags .tag-edit"))
	check(t, "the family", out, strings.Join([]string{
		"#task-parent-change: " + ring, "#task-up-rows .kin-row: solid 2px rgb(107, 105, 100) -2px seen", "#task-up-rows .unlink: " + ring,
		"the link: none", "#task-child: " + ring, "#frame-tags .tag-edit: " + ring,
	}, "|"))
}
