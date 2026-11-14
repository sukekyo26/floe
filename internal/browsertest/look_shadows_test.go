package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/kit"
)

// shadowsExtra is a section added to every-part: panels one after another in a note's box, between its text, cards
// of html, and quotes at the end of a note and of a text in details, where a theme's quote casts its shadow out of the part.
const shadowsExtra = `  - heading: Nested boxes
    parts:
      - html: |
          <div data-floe-part="note"><p>Text before</p><div data-floe-look="panel"><p>A panel in a note</p></div><div data-floe-look="panel"><p>Another</p></div><p>Text after</p></div>
          <div data-floe-look="cards"><div class="card" data-floe-look="panel">One</div><div class="card" data-floe-look="panel">Two</div></div>
      - note: { text: "Text before.\n\n> A quote at the end of a note." }
      - details:
          title: A question?
          open: true
          parts:
            - text: "An answer.\n\n> A quote at the end of an answer."
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
// quotes on the page (base.css).
// LIMIT: a quote's shadow in a part (markdown.css) is not measured at --floe-room: a part spaces its Markdown's box from
// its title and edge under it, where so large a shadow, first or last in the box, meets them; floe's themes' quotes
// in parts are measured.
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

// markProbed gives each box drawn with a transform (floe's themes move one by translate), or with a ::before or ::after
// placed out of the flow (notebook's tape), an
// index, so that those, which a script cannot measure, are measured by pseudoBoxes, and returns how many there are.
const markProbed = `(() => { const probed = [...document.querySelectorAll('[data-floe-part], [data-floe-look]')]
  .filter((e) => e.getClientRects().length && (getComputedStyle(e).transform !== 'none' ||
    ['::before', '::after'].some((p) => ['absolute', 'fixed'].includes(getComputedStyle(e, p).position))));
  probed.forEach((e, i) => { e.dataset.shadowProbe = i; }); return probed.length; })()`

// shadowAtJS defines shadowAt, the place a box's shadow is told at: its part, its look, or else the part it is in (a
// quote) or its tag.
const shadowAtJS = `const shadowAt = (e) => e.dataset.floePart || (e.dataset.floeLook ? 'look ' + e.dataset.floeLook.split(' ')[0] : '') ||
    (e.closest('[data-floe-part]')?.dataset.floePart ?? e.tagName.toLowerCase());`

// measureShadows returns, one to a line, what a box draws outside itself reaching what it must not: past the page's
// edge or the sides of the column's padding (.floe-column); out of a box around it that is framed or filled (its padding box); onto
// any other element (not one around it nor in it): onto its border box when it is framed or filled, else onto its
// content box, where its text is. What a box draws is its shadows that show (from the computed box-shadow, its
// lengths alone, whatever form its colour takes: moved by its offset, grown by its spread, fading out over its blur),
// its outline (its width and offset), its box itself when transformed (its bounding box, where it is moved to), and the
// ::before and ::after of a box markProbed marks, given as extra by its index. Boxes inside a line (inline) are not measured.
// It returns {seen, amiss}: the boxes it measured, and what each reaches, at its part (or its look).
const measureShadows = `((extra) => {
  const out = new Map();
  ` + shadowAtJS + `
  const canvas = document.createElement('canvas').getContext('2d', { willReadFrequently: true });
  const shows = (colour) => { canvas.clearRect(0, 0, 1, 1); canvas.fillStyle = 'transparent'; canvas.fillStyle = colour;
    canvas.fillRect(0, 0, 1, 1); return canvas.getImageData(0, 0, 1, 1).data[3] > 0; };
  const name = (e) => { const p = e.parentElement && e.parentElement.closest('[data-floe-part]');
    return (p ? p.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + (e.dataset.floePart ? '[' + e.dataset.floePart + ']' : '') +
      (e.dataset.floeLook ? '{' + e.dataset.floeLook + '}' : '') + (e.classList.contains('card') ? '.card' : ''); };
  const up = (e) => { let p = e.parentElement; while (p && getComputedStyle(p).display === 'contents') p = p.parentElement; return p; };
  const isColumn = (p) => p.classList.contains('floe-column');
  const painted = (p) => { const s = getComputedStyle(p);
    return ['Top', 'Right', 'Bottom', 'Left'].some((k) => parseFloat(s['border' + k + 'Width']) > 0) ||
      !/rgba\(0, 0, 0, 0\)|transparent/.test(s.backgroundColor) || s.backgroundImage !== 'none'; };
  const holders = (e) => { const hs = []; for (let p = up(e); p && p !== document.body; p = up(p)) {
    if (isColumn(p)) { hs.push(p); break; } if (painted(p)) hs.push(p); } return hs; };
  const inner = (p, padding) => { const r = p.getBoundingClientRect(), s = getComputedStyle(p), k = (side) =>
      parseFloat(s['border' + side + 'Width']) + (padding ? parseFloat(s['padding' + side]) : 0);
    return { l: r.left + k('Left'), t: r.top + k('Top'), r: r.right - k('Right'), b: r.bottom - k('Bottom') }; };
  const join = (a, b) => a ? { l: Math.min(a.l, b.l), t: Math.min(a.t, b.t), r: Math.max(a.r, b.r), b: Math.max(a.b, b.b) } : b;
  const boxes = [...document.querySelectorAll('[data-floe-part], [data-floe-look], blockquote')]
    .filter((b) => b.getClientRects().length && !getComputedStyle(b).display.startsWith('inline'));
  const others = [...document.body.querySelectorAll('*')].filter((o) => {
    return o.checkVisibility({ visibilityProperty: true }) && getComputedStyle(o).display !== 'contents' && !o.matches('input[type=radio]'); })
    .map((o) => { const r = painted(o) ? o.getBoundingClientRect() : inner(o, true); return { o, r: { l: r.left ?? r.l, t: r.top ?? r.t, r: r.right ?? r.r, b: r.bottom ?? r.b } }; })
    .filter(({ r }) => r.r - r.l > 0.5 && r.b - r.t > 0.5);
  for (const e of boxes) {
    const s = getComputedStyle(e), r = e.getBoundingClientRect(), where = shadowAt(e);
    let ink = null;
    for (const shadow of s.boxShadow === 'none' ? [] : s.boxShadow.split(/,(?![^(]*\))/)) {
      if (/\binset\b/.test(shadow)) continue;
      const lengths = [...shadow.matchAll(/(-?[\d.]+)px/g)].map((m) => Number(m[1]));
      if (!shows(shadow.replace(/-?[\d.]+px/g, '').trim())) continue;
      const [x, y, blur = 0, spread = 0] = lengths, g = blur + spread;
      ink = join(ink, { l: r.left + x - g, t: r.top + y - g, r: r.right + x + g, b: r.bottom + y + g });
    }
    if (s.transform !== 'none') ink = join(ink, { l: r.left, t: r.top, r: r.right, b: r.bottom });
    const o = s.outlineStyle !== 'none' ? parseFloat(s.outlineWidth) + parseFloat(s.outlineOffset) : 0;
    if (o > 0) ink = join(ink, { l: r.left - o, t: r.top - o, r: r.right + o, b: r.bottom + o });
    for (const q of extra[e.dataset.shadowProbe] || []) ink = join(ink, { l: q[0], t: q[1], r: q[2], b: q[3] });
    if (!ink) continue;
    const what = name(e);
    for (const h of holders(e)) {
      const c = inner(h, false), col = isColumn(h);
      const over = { right: ink.r - c.r, left: c.l - ink.l, bottom: col ? 0 : ink.b - c.b, top: col ? 0 : c.t - ink.t };
      const sides = Object.entries(over).filter(([, v]) => v > 0.5).map(([k, v]) => k + ' ' + v.toFixed(1));
      if (sides.length) out.set(what + ' out of ' + name(h) + ': ' + sides.join(', '), where);
    }
    if (ink.l < -0.5 || ink.r > document.documentElement.clientWidth + 0.5) out.set(what + ' past the page edge', where);
    for (const { o: b, r: q } of others) {
      if (b === e || b.contains(e) || e.contains(b) || q.r <= ink.l || q.l >= ink.r || q.b <= ink.t || q.t >= ink.b) continue;
      const w = Math.min(ink.r, q.r) - Math.max(ink.l, q.l), v = Math.min(ink.b, q.b) - Math.max(ink.t, q.t);
      if (w > 0.5 && v > 0.5) out.set(what + ' onto ' + name(b) + ' by ' + Math.min(w, v).toFixed(1), where);
    }
  }
  return JSON.stringify({ seen: boxes.length, amiss: [...out].sort().map(([text, at]) => ({ at, text })) });
})`

// shadowsIn measures the page open at width as measureShadows does, after taking the ::before and ::after of each
// box markProbed marks from DevTools.
func (b *browser) shadowsIn(where string, width int64) measured {
	b.t.Helper()

	b.size(width, 900, false)
	defer b.unsize()
	b.eval("document.querySelectorAll('details').forEach((d) => { d.open = true; }); ''")
	var n int
	if _, err := fmt.Sscan(b.eval(markProbed), &n); err != nil {
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
	return b.measure(fmt.Sprintf("%s at %dpx", where, width), measureShadows+"("+mustJSON(extra)+")")
}

// measureCellShadows returns the shadow colour of every cell drawn in a box that casts a shadow, and of that box, as
// "cell in box".
const measureCellShadows = `[...document.querySelectorAll('[data-floe-part=cell]')].map((c) => {
  const box = c.parentElement.closest('[data-floe-part], [data-floe-look]'), colour = (e) => getComputedStyle(e).getPropertyValue('--floe-shadow-color').trim();
  return box && !/^rgba\(.*, 0\)$|^transparent$/.test(colour(box)) ? colour(c) + ' in ' + colour(box) : '';
}).filter(Boolean).filter((v, i, a) => a.indexOf(v) === i).join(' | ')`

// measureCast finds each box that sets a shadow's colour but casts no shadow of that colour, as a box-shadow written
// on it elsewhere wins over base.css's. It returns {seen, amiss}: the boxes that set a colour, and each casting none,
// at its part (or look).
const measureCast = `(() => {
  ` + shadowAtJS + `
  let seen = 0;
  const out = new Set();
  for (const e of document.querySelectorAll('[data-floe-part], [data-floe-look], blockquote')) {
    const s = getComputedStyle(e), c = s.getPropertyValue('--floe-shadow-color').trim();
    if (/^rgba\(.*, 0\)$|^transparent$/.test(c) || !c) continue;
    seen++;
    if (!s.boxShadow.split(/,(?![^(]*\))/).some((x) => x.trim().startsWith(c) && !x.includes('inset'))) {
      out.add(JSON.stringify({ at: shadowAt(e), text: e.tagName.toLowerCase() + '[' + (e.dataset.floePart || e.dataset.floeLook || '') + '] casts no shadow of ' + c + ': ' + s.boxShadow }));
    }
  }
  return JSON.stringify({ seen, amiss: [...out].map((l) => JSON.parse(l)) });
})()`

// exampleWidths are the widths a template's example is measured at: a phone's and the column at its widest, and a wide
// screen for slides, whose content grows with the screen.
func exampleWidths(tmpl string) []int64 {
	if tmpl == "slides" {
		return []int64{480, 1600, 2560}
	}
	return []int64{480, 1600}
}

// shadowPages visits every-part with shadowsExtra in every theme and in shadowsKit, and floe's templates' examples with
// their parts one after another at the least room floe leaves between parts (shadowsQuote, shadowsMore) too when
// examples, telling each page and the widths to measure it at.
func shadowPages(t *testing.T, s *site, examples bool, visit func(what string, widths []int64)) {
	t.Helper()

	s.own(t, shadowsKit)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, names, _ := k.Names()
	names = append(names, "edge")
	file := s.write(t, "every.yaml", s.everyPart(t)+shadowsExtra)
	for _, name := range names {
		s.floe(t, "publish", file, "--slug", "shadows-"+name, "--theme", name)
		s.b.navigate("/a/shadows-" + name + "/?raw")
		visit("every-part in "+name, []int64{480, 1100, 1600, 2560})
	}
	if !examples {
		return
	}
	files, err := filepath.Glob("../corpus/testdata/example/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range files {
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
			s.b.navigate("/a/" + slug + "/?raw")
			visit(tmpl+" in "+name, exampleWidths(tmpl))
		}
	}
}

const lookShadowsFix = "keep what a box draws outside itself (its shadow by --floe-shadow-*, an outline, a pseudo-element) within " +
	"--floe-room on every side, with no transform or scale and taking no room"

// lookShadows draws shadowPages at their widths, as shadowsIn measures: nothing a box draws outside itself (a shadow, a
// strip of tape, an outline) reaches the page's edge, another box or the box it stands in.
func lookShadows(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	shadowPages(t, s, true, func(what string, widths []int64) {
		for _, w := range widths {
			m.add(s.b.shadowsIn(what, w))
		}
	})
	return m
}

// brokenShadows draws a note in a theme of one's own whose palette spreads the shadow 40px, which base.css casts in the
// colour its note.css sets.
func brokenShadows(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{
		"themes/loud/palette.css": ":root { --floe-shadow-spread: 40px; }\n",
		"themes/loud/note.css":    ":scope { --floe-shadow-color: rgb(0 0 0 / 0.4); }\n",
	}, themePage("loud", "- note: { text: loud }\n"))
	return "note", s.b.shadowsIn("broken", 1100)
}

const lookShadowCastFix = "write no box-shadow on a box, part, look or quote: set --floe-shadow-color, and base.css's one rule casts it"

// lookShadowCast draws every-part in every theme and in shadowsKit, as measureCast measures: every box that sets a
// shadow's colour casts it, as base.css's one rule does.
func lookShadowCast(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	shadowPages(t, s, false, func(what string, _ []int64) { m.add(s.b.measure(what, measureCast)) })
	return m
}

// brokenShadowCast draws a note in a theme of one's own whose note.css sets a shadow's colour and writes a box-shadow of
// its own, which wins over base.css's.
func brokenShadowCast(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{"themes/cast/note.css": ":scope { --floe-shadow-color: rgb(0 0 0 / .4); box-shadow: 0 1px 0 red; }\n"},
		themePage("cast", "- note: { text: cast }\n"))
	return "note", s.b.measure("broken", measureCast)
}
