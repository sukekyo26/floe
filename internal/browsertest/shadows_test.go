package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shadowsExtra is a section added to every-part: a panel in a note's box, and cards of html.
const shadowsExtra = `  - heading: Nested boxes
    parts:
      - html: |
          <div data-floe-part="note"><div data-floe-look="panel"><p>A panel in a note</p></div></div>
          <div data-floe-look="cards"><div class="card" data-floe-look="panel">One</div><div class="card" data-floe-look="panel">Two</div></div>
`

// shadowsMore is a section of a template's more, its parts one after another at the least room between them, and an
// options of one card, as wide as the column lets it.
const shadowsMore = `  - heading: Close
    parts:
      - code: { lang: sh, text: one }
      - code: { lang: sh, text: two }
      - options:
          - { title: Only, text: the only way, pros: [simple], cons: [none] }
      - code: { lang: sh, text: three }
`

// shadowsKit is a theme of one's own whose shadow reaches --floe-room on every side of its panels, code, notes and
// quotes.
var shadowsKit = map[string]string{
	"themes/edge/palette.css": ":root { --floe-shadow-blur: 12px; }\n",
	"themes/edge/panel.css":   ":scope { --floe-shadow-color: rgb(0 0 0 / 0.4); }\n",
	"themes/edge/code.css":    ":scope { --floe-shadow-color: rgb(0 0 0 / 0.4); }\n",
	"themes/edge/note.css":    ":scope { --floe-shadow-color: rgb(0 0 0 / 0.4); }\n",
	"themes/edge/base.css":    "blockquote { --floe-shadow-color: rgb(0 0 0 / 0.4); }\n",
}

// shadowsQuote turns the text of a runbook's step into a quote, a box of the page that stands right before the step's
// terminal, at the least room floe leaves in a step.
var shadowsQuote = strings.NewReplacer("text: 'Name it with today''s date", "text: '> Name it with today''s date")

// markTilted gives each box drawn tilted an index, so that its ::before and ::after, which a script cannot measure,
// are measured by pseudoBoxes, and returns how many there are.
const markTilted = `(() => { const tilted = [...document.querySelectorAll('[data-floe-part], [data-floe-look]')]
  .filter((e) => e.getClientRects().length && getComputedStyle(e).transform !== 'none');
  tilted.forEach((e, i) => { e.dataset.shadowProbe = i; }); return tilted.length; })()`

// measureShadows returns, one to a line, what a box draws outside itself reaching what it must not: past the page's
// edge or the sides of the column's padding; out of a box around it that is framed or filled (its padding box); onto
// another box (not one around it nor in it). What a box draws is its shadows (from the computed box-shadow: moved by
// its offset, grown by its spread, fading out over its blur), its outline (its width and offset, a focus ring aside),
// its box itself when tilted (its bounding box, which holds the tilt), and the ::before and ::after of a tilted box,
// given as extra by its index. Boxes inside a line (inline) are not measured.
const measureShadows = `((extra) => {
  const out = new Set(), pagePad = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--page-pad'));
  const name = (e) => { const p = e.parentElement && e.parentElement.closest('[data-floe-part]');
    return (p ? p.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + (e.dataset.floePart ? '[' + e.dataset.floePart + ']' : '') +
      (e.dataset.floeLook ? '{' + e.dataset.floeLook + '}' : '') + (e.classList.contains('card') ? '.card' : ''); };
  const up = (e) => { let p = e.parentElement; while (p && getComputedStyle(p).display === 'contents') p = p.parentElement; return p; };
  const isColumn = (p) => Math.abs(parseFloat(getComputedStyle(p).paddingLeft) - pagePad) < 0.5;
  const painted = (p) => { const s = getComputedStyle(p);
    return ['Top', 'Right', 'Bottom', 'Left'].some((k) => parseFloat(s['border' + k + 'Width']) > 0) ||
      !/rgba\(0, 0, 0, 0\)|transparent/.test(s.backgroundColor) || s.backgroundImage !== 'none'; };
  const holders = (e) => { const hs = []; for (let p = up(e); p && p !== document.body; p = up(p)) {
    if (isColumn(p)) { hs.push(p); break; } if (painted(p)) hs.push(p); } return hs; };
  const pad = (p) => { const r = p.getBoundingClientRect(), s = getComputedStyle(p);
    return { l: r.left + parseFloat(s.borderLeftWidth), t: r.top + parseFloat(s.borderTopWidth),
      r: r.right - parseFloat(s.borderRightWidth), b: r.bottom - parseFloat(s.borderBottomWidth) }; };
  const join = (a, b) => a ? { l: Math.min(a.l, b.l), t: Math.min(a.t, b.t), r: Math.max(a.r, b.r), b: Math.max(a.b, b.b) } : b;
  const boxes = [...document.querySelectorAll('[data-floe-part], [data-floe-look], blockquote')]
    .filter((b) => b.getClientRects().length && !getComputedStyle(b).display.startsWith('inline'));
  for (const e of boxes) {
    const s = getComputedStyle(e), r = e.getBoundingClientRect();
    let ink = null;
    for (const m of s.boxShadow.matchAll(/(rgba?\([^)]*\)|[a-z]+)\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+([\d.]+)px\s+(-?[\d.]+)px(\s+inset)?/g)) {
      if (m[6] || /^rgba\(.*, 0\)$|^transparent$/.test(m[1])) continue;
      const [x, y, blur, spread] = m.slice(2, 6).map(Number), g = blur + spread;
      ink = join(ink, { l: r.left + x - g, t: r.top + y - g, r: r.right + x + g, b: r.bottom + y + g });
    }
    if (s.transform !== 'none') ink = join(ink, { l: r.left, t: r.top, r: r.right, b: r.bottom });
    const o = s.outlineStyle !== 'none' ? parseFloat(s.outlineWidth) + parseFloat(s.outlineOffset) : 0;
    if (o > 0) ink = join(ink, { l: r.left - o, t: r.top - o, r: r.right + o, b: r.bottom + o });
    for (const q of extra[e.dataset.shadowProbe] || []) ink = join(ink, { l: q[0], t: q[1], r: q[2], b: q[3] });
    if (!ink) continue;
    const what = name(e);
    for (const h of holders(e)) {
      const c = pad(h), col = isColumn(h);
      const over = { right: ink.r - c.r, left: c.l - ink.l, bottom: col ? 0 : ink.b - c.b, top: col ? 0 : c.t - ink.t };
      const sides = Object.entries(over).filter(([, v]) => v > 0.5).map(([k, v]) => k + ' ' + v.toFixed(1));
      if (sides.length) out.add(what + ' out of ' + name(h) + ': ' + sides.join(', '));
    }
    if (ink.l < -0.5 || ink.r > document.documentElement.clientWidth + 0.5) out.add(what + ' past the page edge');
    for (const b of boxes) {
      if (b === e || b.contains(e) || e.contains(b)) continue;
      const q = b.getBoundingClientRect();
      const w = Math.min(ink.r, q.right) - Math.max(ink.l, q.left), v = Math.min(ink.b, q.bottom) - Math.max(ink.t, q.top);
      if (w > 0.5 && v > 0.5) out.add(what + ' onto ' + name(b) + ' by ' + Math.min(w, v).toFixed(1));
    }
  }
  return [...out].sort().join('\n');
})`

// shadowsIn measures the page open at width as measureShadows does, after taking the ::before and ::after of each
// tilted box from DevTools.
func (b *browser) shadowsIn(where string, width int64) []string {
	b.t.Helper()

	b.size(width, 900, false)
	defer b.unsize()
	var n int
	if _, err := fmt.Sscan(b.eval(markTilted), &n); err != nil {
		b.t.Fatal(err)
	}
	extra := map[string][][]float64{}
	for i := range n {
		sel := fmt.Sprintf("[data-shadow-probe=%q]", fmt.Sprint(i))
		before, after := b.pseudoBoxes(sel)
		for _, q := range [][]float64{before, after} {
			if q != nil {
				extra[fmt.Sprint(i)] = append(extra[fmt.Sprint(i)], q)
			}
		}
	}
	got := b.eval(measureShadows + "(" + mustJSON(extra) + ")")
	if got == "" {
		return nil
	}
	lines := strings.Split(got, "\n")
	for i, l := range lines {
		lines[i] = fmt.Sprintf("%s at %dpx: %s", where, width, l)
	}
	return lines
}

// measureCellShadows returns the shadow colour of every cell drawn in a box that casts a shadow, and of that box, as
// "cell in box".
const measureCellShadows = `[...document.querySelectorAll('[data-floe-part=cell]')].map((c) => {
  const box = c.parentElement.closest('[data-floe-part], [data-floe-look]'), colour = (e) => getComputedStyle(e).getPropertyValue('--floe-shadow-color').trim();
  return box && !/^rgba\(.*, 0\)$|^transparent$/.test(colour(box)) ? colour(c) + ' in ' + colour(box) : '';
}).filter(Boolean).filter((v, i, a) => a.indexOf(v) === i).join(' | ')`

// measureCast returns each box that sets a shadow's colour but casts no shadow of that colour, as a box-shadow written
// on it elsewhere wins over base.css's.
const measureCast = `[...document.querySelectorAll('[data-floe-part], [data-floe-look], blockquote')].filter((e) => {
  const s = getComputedStyle(e), c = s.getPropertyValue('--floe-shadow-color').trim();
  return !/^rgba\(.*, 0\)$|^transparent$/.test(c) && !s.boxShadow.split(/,(?![^(]*\))/).some((x) => x.trim().startsWith(c) && !x.includes('inset'));
}).map((e) => e.tagName.toLowerCase() + '[' + (e.dataset.floePart || e.dataset.floeLook) + ']').filter((v, i, a) => a.indexOf(v) === i).join(' | ')`

// TestUIShadows draws every part, and floe's templates with their parts one after another at the least room floe
// leaves between parts, in every theme and in one of one's own whose shadow reaches that room on every side, at the
// widths of a phone, a laptop, the column at its widest and slides on a wide screen: nothing a box draws outside
// itself (a shadow, a tilt and its tape, an outline) reaches the page's edge, another box or the box it stands in;
// every box that sets a shadow's colour casts it.
// A box in one that casts a shadow casts none (the colour is the box's alone), and retro's cards keep the stripes
// they draw inside themselves with --floe-inset.
func TestUIShadows(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.own(t, shadowsKit)
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(themes)+1)
	for _, th := range themes {
		names = append(names, th.Name())
	}
	names = append(names, "edge")
	everyPart, err := os.ReadFile("../corpus/testdata/every-part/every-part.yaml")
	if err != nil {
		t.Fatal(err)
	}
	shot, err := os.ReadFile("../corpus/testdata/every-part/shots/moved.png")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "shots/moved.png", string(shot))
	file := s.write(t, "every.yaml", string(everyPart)+shadowsExtra)
	widths := []int64{480, 1100, 1600, 2560}
	amiss := make([]string, 0, len(names))
	for _, name := range names {
		s.floe(t, "publish", file, "--slug", "shadows-"+name, "--theme", name)
		b.navigate("/a/shadows-" + name + "/?raw")
		for _, w := range widths {
			amiss = append(amiss, b.shadowsIn("every-part in "+name, w)...)
		}
		if lost := b.eval(measureCast); lost != "" {
			amiss = append(amiss, "every-part in "+name+": no shadow cast by "+lost)
		}
	}
	examples, err := filepath.Glob("../corpus/testdata/example/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range examples {
		data, err := os.ReadFile(ex)
		if err != nil {
			t.Fatal(err)
		}
		tmpl := strings.TrimSuffix(filepath.Base(ex), ".yaml")
		text := shadowsQuote.Replace(string(data))
		if strings.Contains(text, "\nmore:\n") {
			text += shadowsMore
		}
		f := s.write(t, "example-"+tmpl+".yaml", text)
		for _, name := range names {
			slug := "shadows-" + tmpl + "-" + name
			s.floe(t, "publish", f, "--slug", slug, "--theme", name)
			b.navigate("/a/" + slug + "/?raw")
			for _, w := range widths {
				amiss = append(amiss, b.shadowsIn(tmpl+" in "+name, w)...)
			}
		}
	}
	check(t, "outside", amiss, "")

	b.navigate("/a/shadows-brutal/?raw")
	check(t, "held", []string{b.eval(measureCellShadows)}, "rgba(0, 0, 0, 0) in rgb(10, 10, 10)")
	b.navigate("/a/shadows-retro/?raw")
	got := b.eval(`(() => { const s = getComputedStyle(document.querySelector('.card')).boxShadow; return (s.match(/inset/g) || []).length; })()`)
	check(t, "retro's stripes", []string{got}, "3")
}
