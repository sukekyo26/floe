package browsertest_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// toneExtra is a section added to every-part: a pill without a tone in a note of one, and a diagram's boxes with a
// tone and without.
const toneExtra = `  - heading: Tones held
    parts:
      - html: <div data-floe-tone="good" data-floe-part="note"><p>Held <span data-floe-part="cell">plain</span></p></div>
      - diagram:
          width: 560
          height: 80
          title: Boxes in a tone and in none
          parts:
            - box: { x: 10, y: 14, w: 120, h: 52, rows: [Plain] }
            - box: { x: 150, y: 14, w: 120, h: 52, tone: good, rows: [Good] }
            - box: { x: 290, y: 14, w: 120, h: 52, tone: bad, rows: [Bad] }
            - box: { x: 430, y: 14, w: 120, h: 52, tone: muted, rows: [Muted] }
`

// measureToneAt finds what of the page's tones goes amiss, given tone.json's tones:
//   - a tone whose --<tone> or --<tone>-soft the palette leaves unset;
//   - an element of a tone (data-floe-tone) whose --tone, --tone-soft and --tone-fill are not --<tone>, --<tone>-soft
//     and --<tone>-fill (or else --<tone>-soft);
//   - an element of a tone that draws in its tone's colours, --<tone> or --<tone>-soft, but not from --tone,
//     --tone-soft or --tone-fill: what draws so (itself, its ::before and ::after, and what it holds short of a
//     part's box) keeps the colour when the three are set on it to others. What draws in colours of a theme's own (brutal's fills) or in
//     none of its tone (a stat without a note), and links, are let be; accent, and muted's text, the page's colours
//     drawn anywhere (blueprint's panels, a date), are not measured so;
//   - a part's box without a tone whose --tone is not --accent (stats': --muted; the diagram's: none), and the page's
//     root;
//   - a diagram's box without a tone filled other than --surface.
//
// It returns {seen, amiss}: the elements of a tone and the parts of none it measured, and for each amiss the part (or palette, page, diagram) and what.
const measureToneAt = `((tones) => {
  let seen = 0;
  const out = [], push = (at, text) => out.push(JSON.stringify({ at, text })), root = getComputedStyle(document.documentElement), v = (s, n) => s.getPropertyValue(n).trim();
  for (const t of tones) for (const n of ['--' + t, '--' + t + '-soft']) if (!v(root, n)) push('palette', 'no ' + n);
  const props = ['color', 'backgroundColor', 'borderTopColor', 'borderRightColor', 'borderBottomColor', 'borderLeftColor',
    'outlineColor', 'textDecorationColor', 'fill', 'stroke', 'boxShadow'];
  const part = (e) => { const p = e.closest('[data-floe-part]'); return p ? p.dataset.floePart : ''; };
  const name = (e) => (part(e) || e.tagName.toLowerCase()) + '[' + e.dataset.floeTone + ']';
  for (const e of document.querySelectorAll('[data-floe-tone]')) {
    seen++;
    const s = getComputedStyle(e), t = e.dataset.floeTone;
    if (v(s, '--tone') !== v(s, '--' + t) || v(s, '--tone-soft') !== v(s, '--' + t + '-soft') ||
      v(s, '--tone-fill') !== (v(s, '--' + t + '-fill') || v(s, '--' + t + '-soft'))) push(part(e) || e.tagName.toLowerCase(), name(e) + ': not ' + t + "'s colours");
    const box = e.closest('[data-floe-part]');
    const held = [e, ...e.querySelectorAll('*')].filter((c) => c === e || (!c.matches('[data-floe-part]') && c.closest('[data-floe-part]') === box && !c.closest('a')));
    const drawn = () => held.flatMap((c) => [null, '::before', '::after'].map((p) => getComputedStyle(c, p))).flatMap((cs) => props.map((k) => cs[k]));
    const probe = e.appendChild(document.createElement('i'));
    probe.style.color = 'var(--' + t + ')'; probe.style.backgroundColor = 'var(--' + t + '-soft)';
    const ink = [getComputedStyle(probe).color, getComputedStyle(probe).backgroundColor];
    probe.remove();
    const before = drawn();
    e.style.setProperty('--tone', 'rgb(1, 2, 3)'); e.style.setProperty('--tone-soft', 'rgb(4, 5, 6)'); e.style.setProperty('--tone-fill', 'rgb(7, 8, 9)');
    const after = drawn();
    e.style.removeProperty('--tone'); e.style.removeProperty('--tone-soft'); e.style.removeProperty('--tone-fill');
    const watched = t === 'accent' ? [] : t === 'muted' ? ink.slice(1) : ink;
    const i = before.findIndex((c, i) => watched.some((k) => c.includes(k)) && c === after[i]);
    if (i >= 0) push(part(e) || e.tagName.toLowerCase(), name(e) + ': ' + held[Math.floor(i / (3 * props.length))].tagName.toLowerCase() + ' draws in ' + t + "'s colours, not from --tone");
  }
  if (v(root, '--tone') !== v(root, '--accent')) push('page', '--tone not --accent');
  for (const b of document.querySelectorAll('[data-floe-part]:not([data-floe-tone])')) {
    seen++;
    const s = getComputedStyle(b), p = b.dataset.floePart;
    const want = p === 'stats' ? v(s, '--muted') : p === 'diagram' ? '' : v(s, '--accent');
    if (p !== 'diagram' && v(s, '--tone') !== want) push(p, 'without a tone: --tone ' + v(s, '--tone'));
  }
  const surface = document.body.appendChild(document.createElement('div'));
  surface.style.color = 'var(--surface)';
  for (const r of document.querySelectorAll('.dg .box:not([data-floe-tone]) rect')) {
    if (getComputedStyle(r).fill !== getComputedStyle(surface).color) push('diagram', 'a box without a tone: ' + getComputedStyle(r).fill);
  }
  surface.remove();
  return JSON.stringify({ seen, amiss: [...new Set(out)].sort().map((l) => JSON.parse(l)) });
})`

const lookToneFix = "mark a tone by data-floe-tone and draw it by --tone, --tone-soft and --tone-fill, not by its own colour's variable"

// lookTone measures every-part and toneExtra in every theme, and adr and postmortem in the default theme, as
// measureToneAt does.
func lookTone(t *testing.T, s *site) measured {
	t.Helper()

	b := s.b
	k, measure := toneMeasure(t)
	_, _, themes, _ := k.Names()
	file := s.write(t, "every.yaml", s.everyPart(t)+toneExtra)
	var m measured
	for _, name := range themes {
		s.floe(t, "publish", file, "--slug", "tone-"+name, "--theme", name)
		b.navigate("/a/tone-" + name + "/?raw")
		m.add(b.measure("every-part in "+name, measure))
	}
	for _, tmpl := range []string{"adr", "postmortem"} {
		tpl, _, err := k.Template(tmpl)
		if err != nil {
			t.Fatal(err)
		}
		s.floe(t, "publish", s.write(t, tmpl+".yaml", string(tpl.Example())), "--slug", "tone-"+tmpl)
		b.navigate("/a/tone-" + tmpl + "/?raw")
		m.add(b.measure(tmpl, measure))
	}
	return m
}

// brokenTone draws a part of one's own of the tone good that draws its text by --good, not by --tone.
func brokenTone(t *testing.T, s *site) (string, measured) {
	t.Helper()

	_, measure := toneMeasure(t)
	html := `<p data-floe-part="tonal" data-floe-tone="good">ok</p>`
	s.brokenPage(t, ownPart("tonal", html, ":scope { color: var(--good); }\n"), htmlPage(html))
	return "tonal", s.b.measure("broken", measure)
}

// toneMeasure returns floe's kit and measureToneAt called with its tones.
func toneMeasure(t *testing.T) (*kit.Kit, string) {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	tones, err := k.Tones()
	if err != nil {
		t.Fatal(err)
	}
	return k, measureToneAt + "(" + mustJSON(tones) + ")"
}
