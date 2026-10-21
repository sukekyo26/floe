package server_test

import (
	"os"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// measureContrast is the script that measures, on the page, the contrast of every text shown, an element's
// own text and that of its ::before and ::after, against the ground under it (a control that is disabled,
// which WCAG leaves out, and a progress bar's text, which is not shown, but): the background colours of it
// and the elements around it laid one over another on the root's, a colour of an alpha or an opacity laid
// over that ground. It returns one line for each below 4.5:1, 3:1 for large text (24px, or 18.66px bold), as
// WCAG AA asks.
// LIMIT: a ground of an image or a gradient (blueprint's squares) counts as the colour under it, the ground
// is that of the elements around in the document, not of one positioned under the text, and a diagram's
// text, in SVG, is not measured. Measure them if a theme comes to put text on such a ground.
const measureContrast = `(() => {
  const canvas = new OffscreenCanvas(1, 1), cx = canvas.getContext('2d', { willReadFrequently: true });
  const rgba = (colour) => {
    cx.clearRect(0, 0, 1, 1); cx.fillStyle = '#0000'; cx.fillStyle = colour; cx.fillRect(0, 0, 1, 1);
    const d = cx.getImageData(0, 0, 1, 1).data;
    return [d[0], d[1], d[2], d[3] / 255];
  };
  const over = (top, under) => {
    const a = top[3] + under[3] * (1 - top[3]);
    if (a === 0) return [0, 0, 0, 0];
    return [0, 1, 2].map((i) => (top[i] * top[3] + under[i] * under[3] * (1 - top[3])) / a).concat(a);
  };
  const luminance = (c) => {
    const [r, g, b] = c.slice(0, 3).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const ratio = (a, b) => { const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const ground = (e) => {
    const chain = [];
    for (let n = e; n; n = n.parentElement) chain.unshift(n);
    let g = [255, 255, 255, 1];
    for (const n of chain) g = over(rgba(getComputedStyle(n).backgroundColor), g);
    return g;
  };
  const opacity = (e) => { let o = 1; for (let n = e; n; n = n.parentElement) o *= Number(getComputedStyle(n).opacity); return o; };
  const name = (e) => {
    const part = e.closest('[data-floe-part]');
    return (part ? part.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + [...e.classList].map((c) => '.' + c).join('');
  };
  const hex = (c) => '#' + c.slice(0, 3).map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
  const low = [];
  const measure = (e, s, under, what, text) => {
    const fg = rgba(s.color);
    fg[3] *= opacity(e);
    if (fg[3] === 0) return;
    const bg = over(rgba(s.backgroundColor), under), shown = over(fg, bg);
    const size = parseFloat(s.fontSize), large = size >= 24 || (size >= 18.66 && Number(s.fontWeight) >= 700);
    const r = ratio(shown, bg);
    if (r < (large ? 3 : 4.5)) low.push(name(e) + what + ' ' + hex(shown) + ' on ' + hex(bg) + ' ' + r.toFixed(2) + ' "' + text.trim().slice(0, 16) + '"');
  };
  for (const e of document.body.querySelectorAll('*')) {
    if (e.closest('svg, script, style, template, progress, meter, :disabled') || e.getClientRects().length === 0) continue;
    const s = getComputedStyle(e);
    if (s.visibility !== 'visible') continue;
    const own = [...e.childNodes].filter((n) => n.nodeType === Node.TEXT_NODE).map((n) => n.data).join('');
    const under = e.parentElement ? ground(e.parentElement) : [255, 255, 255, 1];
    if (own.trim()) measure(e, s, under, '', own);
    for (const pseudo of ['::before', '::after']) {
      const p = getComputedStyle(e, pseudo), content = p.content;
      if (p.display === 'none' || p.visibility !== 'visible' || !/^["']./.test(content) || /^["']\s*["']$/.test(content)) continue;
      measure(e, p, ground(e), pseudo, content.slice(1, -1));
    }
  }
  return [...new Set(low)].join('\n');
})()`

// TestUIContrast draws the page of every part in every theme of floe's, in light and dark mode, and
// measures the contrast of every text shown against the ground under it, as measureContrast does: the
// parts, the themes and their palettes together draw nothing that cannot be read.
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
		for _, mode := range []string{"light", "dark"} {
			media := emulation.SetEmulatedMediaParams{Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: mode}}}
			if _, err := chromedp.Call(b.ctx, emulation.SetEmulatedMedia, media); err != nil {
				t.Fatal(err)
			}
			b.navigate("/a/" + slug + "/?raw")
			b.until("the parts", "!!document.querySelector('[data-floe-part=bars]')")
			if low := b.eval(measureContrast); low != "" {
				t.Errorf("%s in %s mode draws text that cannot be read:\n%s", name, mode, strings.ReplaceAll(low, "\n", "\n  "))
			}
		}
	}
}
