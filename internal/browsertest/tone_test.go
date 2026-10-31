package browsertest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// toneExtra is a section added to every-part: a pill without a tone in a note of one, and a diagram's boxes with a
// tone and without.
const toneExtra = `  - heading: Tones held
    parts:
      - html: <div data-floe-tone="good" data-floe-part="note"><p>Held <span data-floe-part="cell">plain</span></p></div>
      - diagram:
          width: 420
          height: 80
          title: Boxes in a tone and in none
          parts:
            - box: { x: 10, y: 14, w: 120, h: 52, rows: [Plain] }
            - box: { x: 150, y: 14, w: 120, h: 52, tone: good, rows: [Good] }
            - box: { x: 290, y: 14, w: 120, h: 52, tone: bad, rows: [Bad] }
`

// measureTones returns, one to a line, what of the page's tones goes amiss, given tone.json's tones:
//   - a tone whose --<tone> or --<tone>-soft the palette leaves unset;
//   - an element of a tone (data-floe-tone) whose --tone and --tone-soft are not --<tone> and --<tone>-soft;
//   - an element of a tone that draws in its tone's colours, --<tone> or --<tone>-soft, but not from --tone and
//     --tone-soft: what draws so (itself, its ::before and ::after, and what it holds short of a part's box) keeps
//     the colour when the two are set on it to others. What draws in colours of a theme's own (brutal's fills) or in
//     none of its tone (a stat without a note), and links, are let be, and accent and muted, the page's colours drawn
//     anywhere (blueprint's panels, a date), are not measured so;
//   - a part's box without a tone whose --tone is not --accent (stats': --muted; the diagram's: none), and the page's
//     root;
//   - a diagram's box without a tone filled other than --surface.
const measureTones = `((tones) => {
  const out = [], root = getComputedStyle(document.documentElement), v = (s, n) => s.getPropertyValue(n).trim();
  for (const t of tones) for (const n of ['--' + t, '--' + t + '-soft']) if (!v(root, n)) out.push('palette: no ' + n);
  const props = ['color', 'backgroundColor', 'borderTopColor', 'borderRightColor', 'borderBottomColor', 'borderLeftColor',
    'outlineColor', 'textDecorationColor', 'fill', 'stroke', 'boxShadow'];
  const part = (e) => { const p = e.closest('[data-floe-part]'); return p ? p.dataset.floePart : ''; };
  const name = (e) => (part(e) || e.tagName.toLowerCase()) + '[' + e.dataset.floeTone + ']';
  for (const e of document.querySelectorAll('[data-floe-tone]')) {
    const s = getComputedStyle(e), t = e.dataset.floeTone;
    if (v(s, '--tone') !== v(s, '--' + t) || v(s, '--tone-soft') !== v(s, '--' + t + '-soft')) out.push(name(e) + ': not ' + t + "'s colours");
    const box = e.closest('[data-floe-part]');
    const held = [e, ...e.querySelectorAll('*')].filter((c) => c === e || (!c.matches('[data-floe-part]') && c.closest('[data-floe-part]') === box && !c.closest('a')));
    const drawn = () => held.flatMap((c) => [null, '::before', '::after'].map((p) => getComputedStyle(c, p))).flatMap((cs) => props.map((k) => cs[k]));
    const probe = e.appendChild(document.createElement('i'));
    probe.style.color = 'var(--' + t + ')'; probe.style.backgroundColor = 'var(--' + t + '-soft)';
    const ink = [getComputedStyle(probe).color, getComputedStyle(probe).backgroundColor];
    probe.remove();
    const before = drawn();
    e.style.setProperty('--tone', 'rgb(1, 2, 3)'); e.style.setProperty('--tone-soft', 'rgb(4, 5, 6)');
    const after = drawn();
    e.style.removeProperty('--tone'); e.style.removeProperty('--tone-soft');
    const i = ['accent', 'muted'].includes(t) ? -1 : before.findIndex((c, i) => ink.some((k) => c.includes(k)) && c === after[i]);
    if (i >= 0) out.push(name(e) + ': ' + held[Math.floor(i / (3 * props.length))].tagName.toLowerCase() + ' draws in ' + t + "'s colours, not from --tone");
  }
  if (v(root, '--tone') !== v(root, '--accent')) out.push('the page: --tone not --accent');
  for (const b of document.querySelectorAll('[data-floe-part]:not([data-floe-tone])')) {
    const s = getComputedStyle(b), p = b.dataset.floePart;
    const want = p === 'stats' ? v(s, '--muted') : p === 'diagram' ? '' : v(s, '--accent');
    if (p !== 'diagram' && v(s, '--tone') !== want) out.push(p + ' without a tone: --tone ' + v(s, '--tone'));
  }
  const surface = document.body.appendChild(document.createElement('div'));
  surface.style.color = 'var(--surface)';
  for (const r of document.querySelectorAll('.dg .box:not([data-floe-tone]) rect')) {
    if (getComputedStyle(r).fill !== getComputedStyle(surface).color) out.push('a diagram box without a tone: ' + getComputedStyle(r).fill);
  }
  surface.remove();
  return [...new Set(out)].sort().join('\n');
})`

// TestUITone draws every part in every tone and in none, with a pill without a tone in a note of one and a diagram's
// boxes, in every theme, and the templates that draw a tone: each tone of tone.json has its colours in every palette;
// what is marked with a tone takes them, and draws in them from --tone and --tone-soft; a part's box without a tone, and the page, take accent's (stats --muted, the diagram its
// shapes' own).
func TestUITone(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	tones, err := k.Tones()
	if err != nil {
		t.Fatal(err)
	}
	everyPart, err := os.ReadFile("../corpus/testdata/every-part/every-part.yaml")
	if err != nil {
		t.Fatal(err)
	}
	shot, err := os.ReadFile("../corpus/testdata/every-part/shots/moved.png")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "shots/moved.png", string(shot))
	file := s.write(t, "every.yaml", string(everyPart)+toneExtra)
	var amiss []string
	measure := func(what string) {
		got := b.eval(measureTones + "(" + mustJSON(tones) + ")")
		for _, l := range strings.Split(got, "\n") {
			if l != "" {
				amiss = append(amiss, what+": "+l)
			}
		}
	}
	for _, name := range themes {
		s.floe(t, "publish", file, "--slug", "tone-"+name, "--theme", name)
		b.navigate("/a/tone-" + name + "/?raw")
		measure("every-part in " + name)
	}
	for _, tmpl := range []string{"adr", "postmortem"} {
		tpl, _ := k.Template(tmpl)
		f := s.write(t, tmpl+".yaml", string(tpl.Example()))
		s.floe(t, "publish", f, "--slug", "tone-"+tmpl)
		b.navigate("/a/tone-" + tmpl + "/?raw")
		measure(tmpl)
	}
	check(t, "tones", amiss, "")
}
