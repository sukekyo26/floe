package browsertest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/kit"
)

// colours begins the scripts that measure colours on the page: a computed colour read, laid over another, and
// the contrast of two; the ground of an element; the name of an element, its part's first, to tell it by; and a
// walk of the page with every details open and each tab picked in turn.
const colours = `
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
    if (a === 0) return [0, 0, 0, 0];
    return [0, 1, 2].map((i) => (top[i] * top[3] + under[i] * under[3] * (1 - top[3])) / a).concat(a);
  };
  const faded = (colour, opacity) => { const c = rgba(colour); c[3] *= opacity; return c; };
  const luminance = (c) => {
    const [r, g, b] = c.slice(0, 3).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const ratio = (a, b) => { const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  // The ground of e and the opacity over it, the root's first.
  const grounds = new Map();
  const ground = (e) => {
    if (!e) return { colour: [255, 255, 255, 1], opacity: 1 };
    if (!grounds.has(e)) {
      const s = getComputedStyle(e), under = ground(e.parentElement), opacity = under.opacity * Number(s.opacity);
      grounds.set(e, { colour: over(faded(s.backgroundColor, opacity), under.colour), opacity });
    }
    return grounds.get(e);
  };
  const name = (e) => {
    const part = e.closest('[data-floe-part]');
    return (part ? part.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + [...e.classList].map((c) => '.' + c).join('');
  };
  // Opens every details, then picks the tabs of every tabs in turn, the first of each, then the second, and calls f
  // with each pick, so that what a tab hides is drawn once.
  const eachTab = (f) => {
    for (const d of document.querySelectorAll('details')) d.open = true;
    const tabs = [...document.querySelectorAll('[data-floe-part=tabs]')].map((t) => [...t.querySelectorAll('input[type=radio]')]);
    for (let i = 0; i < Math.max(1, ...tabs.map((g) => g.length)); i++) {
      for (const g of tabs) if (g[i]) g[i].checked = true;
      f();
    }
  };
  const hex = (c) => '#' + c.slice(0, 3).map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
`

// measureContrast is the script that measures, on the page, the contrast of every text shown against the
// ground under it: an element's own text, the text of its ::before and ::after (a string, a counter() or an
// attr()) and its list ::marker, as the page is drawn, then with every details open and each tab picked in turn, but a control that is
// disabled, which WCAG leaves out, and a progress bar's text, which is not shown. The ground is the
// background colours of the element and those around it laid one over another on the root's, each faded by
// the opacity over it, as the text is. It returns one line for each below 4.5:1, 3:1 for large text (24px,
// or 18.66px bold), as WCAG AA asks.
// LIMIT: a ground of an image or a gradient (blueprint's squares) counts as the colour under it, the ground
// is that of the elements around in the document, not of one positioned under the text, an opacity fades
// each ground on its own rather than the group, and a diagram's text, in SVG, is not measured. Measure them
// if a theme comes to put text on such a ground.
const measureContrast = `(() => {` + colours + `
  const low = new Set();
  const measure = (e, s, under, what, text) => {
    const opacity = under.opacity * (what ? Number(s.opacity) : 1), fg = faded(s.color, opacity);
    if (fg[3] === 0) return;
    const bg = what ? over(faded(s.backgroundColor, opacity), under.colour) : under.colour, shown = over(fg, bg);
    const size = parseFloat(s.fontSize), large = size >= 24 || (size >= 18.66 && Number(s.fontWeight) >= 700);
    const r = ratio(shown, bg);
    if (r < (large ? 3 : 4.5)) low.add(name(e) + what + ' ' + hex(shown) + ' on ' + hex(bg) + ' ' + r.toFixed(2) + ' "' + text.trim().slice(0, 16) + '"');
  };
  const text = (content) => /^["'][^"']/.test(content) || /\b(counters?|attr)\(/.test(content);
  const measureAll = () => {
    grounds.clear();
    for (const e of document.body.querySelectorAll('*')) {
      if (e.closest('svg, script, style, template, progress, meter, :disabled') || e.getClientRects().length === 0) continue;
      const s = getComputedStyle(e);
      if (s.visibility !== 'visible') continue;
      const own = [...e.childNodes].filter((n) => n.nodeType === Node.TEXT_NODE).map((n) => n.data).join('');
      if (own.trim()) measure(e, s, ground(e), '', own);
      for (const pseudo of ['::before', '::after']) {
        const p = getComputedStyle(e, pseudo);
        if (p.display !== 'none' && p.visibility === 'visible' && text(p.content)) measure(e, p, ground(e), pseudo, p.content);
      }
      if (s.display === 'list-item' && s.listStyleType !== 'none') measure(e, getComputedStyle(e, '::marker'), ground(e), '::marker', s.listStyleType);
    }
  };
  measureAll();
  eachTab(measureAll);
  return [...low].join('\n');
})()`

// measureRings is the script that measures, on the page, the contrast of every focus ring against the ground under
// it: it focuses in turn each element a key reaches (a link, a summary, a button, an input, one with a tabindex not
// below 0) and each box that scrolls, which Chrome lets a key reach too, with every details open and each tab picked
// in turn, and takes the outline that the
// focus changes, on the element or around it. Under it is the ground of the element on top at 12 points along the
// middle of its line (a quarter, a half and three quarters of each side), the colour of that element's border where
// the point is on one; a point that a box around clips is left out. It returns one line for each ring below 3:1, as
// WCAG asks of what is not text, at more than half of its points: a ring that crosses what lies beside it (the frame
// of a line's code next to a link) is still seen on the rest. The page's tab must have the focus (SetFocusEmulationEnabled)
// for a focus to be :focus-visible.
// LIMIT: a ring 2px wide over two grounds is measured on the one under the middle of its line (the edge of the
// border 3px wide under brutal's heading of code is not), one low at fewer than half its points passes, and a ring
// every point of which a box clips is measured nowhere: the scrolling code of the part code with no heading draws no
// ring in themes with no padding round it.
const measureRings = `(() => {` + colours + `
  const low = new Set();
  const outline = (e) => { const s = getComputedStyle(e); return s.outlineStyle + s.outlineWidth + s.outlineColor + s.outlineOffset; };
  const scrolls = (e) => {
    const s = getComputedStyle(e);
    return (/auto|scroll/.test(s.overflowX) && e.scrollWidth > e.clientWidth + 1) || (/auto|scroll/.test(s.overflowY) && e.scrollHeight > e.clientHeight + 1);
  };
  // The colour under the point x, y: that of the element on top's border where the point is on one, over its ground.
  const under = (x, y) => {
    const top = document.elementsFromPoint(x, y)[0], s = getComputedStyle(top), b = top.getBoundingClientRect(), g = ground(top);
    const side = y < b.top + parseFloat(s.borderTopWidth) ? 'Top' : y >= b.bottom - parseFloat(s.borderBottomWidth) ? 'Bottom'
      : x < b.left + parseFloat(s.borderLeftWidth) ? 'Left' : x >= b.right - parseFloat(s.borderRightWidth) ? 'Right' : null;
    return side && s['border' + side + 'Style'] !== 'none' ? over(faded(s['border' + side + 'Color'], g.opacity), g.colour) : g.colour;
  };
  const measure = (e, r) => {
    const s = getComputedStyle(r);
    if (s.outlineStyle === 'none' || parseFloat(s.outlineWidth) === 0 || ground(r).opacity === 0) return;
    r.scrollIntoView({ block: 'center', inline: 'center' });
    const b = r.getBoundingClientRect(), d = parseFloat(s.outlineOffset) + parseFloat(s.outlineWidth) / 2;
    const clips = [];
    for (let a = r.parentElement; a && a !== document.documentElement; a = a.parentElement) {
      const t = getComputedStyle(a);
      if (t.overflowX !== 'visible' || t.overflowY !== 'visible') clips.push(a.getBoundingClientRect());
    }
    const xs = [0.25, 0.5, 0.75].map((f) => b.left + f * b.width), ys = [0.25, 0.5, 0.75].map((f) => b.top + f * b.height);
    const read = [];
    for (const [x, y] of xs.flatMap((x) => [[x, b.top - d], [x, b.bottom + d]]).concat(ys.flatMap((y) => [[b.left - d, y], [b.right + d, y]]))) {
      if (x < 0 || y < 0 || x >= innerWidth || y >= innerHeight || clips.some((c) => x < c.left || x >= c.right || y < c.top || y >= c.bottom)) continue;
      const bg = under(x, y), ring = over(faded(s.outlineColor, 1), bg);
      read.push({ bg, ring, q: ratio(ring, bg) });
    }
    const lows = read.filter((p) => p.q < 3).sort((p, q) => p.q - q.q);
    if (lows.length * 2 > read.length) {
      const p = lows[0];
      low.add(name(e) + (r === e ? '' : ' (its ring on ' + name(r) + ')') + ': ring ' + hex(p.ring) + ' on ' + hex(p.bg) + ' ' + p.q.toFixed(2) + ', ' + lows.length + ' of ' + read.length + ' points');
    }
  };
  eachTab(() => {
    grounds.clear();
    for (const e of [...document.body.querySelectorAll('*')].filter((e) => e.matches('a[href], summary, button, input, [tabindex]:not([tabindex^="-"])') || scrolls(e))) {
      if (e.getClientRects().length === 0) continue;
      const before = [];
      for (let r = e; r && r !== document.documentElement; r = r.parentElement) before.push([r, outline(r)]);
      e.focus({ preventScroll: true });
      if (document.activeElement !== e) continue;
      for (const [r, was] of before) if (r === e || outline(r) !== was) measure(e, r);
      e.blur();
    }
  });
  return [...low].join('\n');
})()`

// TestUIContrast measures every page of floePages as measureContrast and measureRings do: the parts, the templates,
// the themes and their palettes together draw nothing that cannot be read, nor a focus that cannot be seen.
func TestUIContrast(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.floePages(t, func(what string) { s.b.measureColours(t, what) })
}

// floePages draws the sample of every part (the corpus's every-part) and the example of every template of floe's in
// every theme of floe's, and calls visit on each, open in the browser, with what tells it.
func (s *site) floePages(t *testing.T, visit func(what string)) {
	t.Helper()

	b := s.b
	src, err := os.ReadFile("../corpus/testdata/every-part/every-part.yaml")
	if err != nil {
		t.Fatal(err)
	}
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, templates := k.Names()
	file := s.write(t, "every-part.yaml", string(src))
	// The image its part image shows, beside it.
	shot, err := os.ReadFile("../corpus/testdata/every-part/shots/moved.png")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "shots/moved.png", string(shot))
	for _, name := range themes {
		slug := "parts-" + name
		s.floe(t, "publish", file, "--slug", slug, "--theme", name)
		b.navigate("/a/" + slug + "/?raw")
		b.until("the parts", "!!document.querySelector('[data-floe-part=bars]')")
		visit(name)
	}
	for _, tmpl := range templates {
		tpl, _ := k.Template(tmpl)
		example := s.write(t, tmpl+".yaml", string(tpl.Example()))
		for _, name := range themes {
			slug := "template-" + tmpl + "-" + name
			s.floe(t, "publish", example, "--slug", slug, "--theme", name)
			b.navigate("/a/" + slug + "/?raw")
			b.until("the page", "!!document.querySelector('h1')")
			visit(tmpl + " in " + name)
		}
	}
}

// measureColours measures the page as measureContrast and measureRings do, the page's tab given the focus, and tells
// what draws text that cannot be read or a focus that cannot be seen.
func (b *browser) measureColours(t *testing.T, what string) {
	t.Helper()

	if _, err := chromedp.Call(b.ctx, emulation.SetFocusEmulationEnabled, emulation.SetFocusEmulationEnabledParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("%s draws text that cannot be read:\n  %s", what, strings.ReplaceAll(low, "\n", "\n  "))
	}
	if low := b.eval(measureRings); low != "" {
		t.Errorf("%s draws a focus ring that cannot be seen:\n  %s", what, strings.ReplaceAll(low, "\n", "\n  "))
	}
}
