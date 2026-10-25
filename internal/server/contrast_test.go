package server_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

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
const measureContrast = `(() => {
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
  const hex = (c) => '#' + c.slice(0, 3).map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
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
  for (const d of document.querySelectorAll('details')) d.open = true;
  const tabs = [...document.querySelectorAll('[data-floe-part=tabs]')].map((t) => [...t.querySelectorAll('input[type=radio]')]);
  for (let i = 0; i < Math.max(1, ...tabs.map((g) => g.length)); i++) {
    for (const g of tabs) if (g[i]) g[i].checked = true;
    measureAll();
  }
  return [...low].join('\n');
})()`

// TestUIContrast draws the page of every part in every theme of floe's, and measures the contrast of
// every text shown against the ground under it, as measureContrast does: the parts, the themes and their
// palettes together draw nothing that cannot be read.
func TestUIContrast(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	src, err := os.ReadFile("../tmpl/testdata/parts.yaml")
	if err != nil {
		t.Fatal(err)
	}
	themes, err := os.ReadDir("../tmpl/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := e.write(t, "parts.yaml", string(src))
	for _, theme := range themes {
		name := theme.Name()
		slug := "parts-" + name
		e.publish(t, file, store.Options{Slug: &slug, Theme: &name})
		b.navigate("/a/" + slug + "/?raw")
		b.until("the parts", "!!document.querySelector('[data-floe-part=bars]')")
		if low := b.eval(measureContrast); low != "" {
			t.Errorf("%s draws text that cannot be read:\n%s", name, strings.ReplaceAll(low, "\n", "\n  "))
		}
	}
}
