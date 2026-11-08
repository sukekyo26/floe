package browsertest_test

import (
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// TestRenderedRings measures rings as measureRings does: one that the box it is in clips at its padding box, its border
// around; a focus that draws no ring, or one of no width; one whose points off the screen are not counted as clipped;
// one clipped at no more than half its points, measured on the rest; and one with every point off the screen.
func TestRenderedRings(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	if _, err := chromedp.Call(b.ctx, emulation.SetFocusEmulationEnabled, emulation.SetFocusEmulationEnabledParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "publish", s.write(t, "rings.yaml", "lang: en\ntemplate: report\ntitle: Rings\nsections: []\n"), "--slug", "rings")
	b.navigate("/a/rings/?raw")
	b.until("the page", "!!document.querySelector('h1')")
	// The middle of the clipped ring's line, 1px off the button, lies on its box's border, 4px wide. The tall one's points
	// are off the screen but one, which its box clips; the wide one's box clips half its points, its sides; the huge one's
	// are all off the screen. The link's span, inline, has no box to clip it.
	b.eval(`(() => {
	  const box = document.createElement('div');
	  Object.assign(box.style, { overflow: 'hidden', border: '4px solid black', width: '200px', margin: '40px' });
	  const clipped = document.createElement('button');
	  clipped.className = 'clipped';
	  Object.assign(clipped.style, { display: 'block', width: '100%', outline: '2px solid black', outlineOffset: '1px' });
	  box.append(clipped);
	  const ringless = document.createElement('button');
	  ringless.className = 'ringless';
	  ringless.style.outline = 'none';
	  const thin = document.createElement('button');
	  thin.className = 'thin';
	  thin.style.outline = 'solid 0';
	  const narrow = document.createElement('div');
	  Object.assign(narrow.style, { overflow: 'hidden', width: '200px' });
	  const tall = document.createElement('button');
	  tall.className = 'tall';
	  Object.assign(tall.style, { display: 'block', width: '100%', height: 3 * innerHeight + 'px', outline: '2px solid black', outlineOffset: '1px' });
	  narrow.append(tall);
	  const half = document.createElement('div');
	  Object.assign(half.style, { overflow: 'hidden', width: '200px', padding: '8px 0' });
	  const wide = document.createElement('button');
	  wide.className = 'wide';
	  Object.assign(wide.style, { display: 'block', width: '300px', outline: '2px solid #f4f4f4', outlineOffset: '1px' });
	  half.append(wide);
	  const huge = document.createElement('button');
	  huge.className = 'huge';
	  Object.assign(huge.style, { display: 'block', width: 3 * innerWidth + 'px', height: 3 * innerHeight + 'px', outline: '2px solid black' });
	  const span = document.createElement('span');
	  span.style.overflow = 'hidden';
	  const link = document.createElement('a');
	  link.className = 'inline';
	  link.href = '#';
	  link.textContent = 'inline';
	  span.append(link);
	  document.body.append(box, ringless, thin, narrow, half, huge, span);
	  return '';
	})()`)
	if got, want := b.eval(measureRings), "button.clipped: ring clipped at 12 of 12 points\nbutton.ringless: no ring\nbutton.thin: no ring\n"+
		"button.tall: ring clipped at 1 of 1 points\n"+
		"button.wide: ring #f4f4f4 on #f4f3f0 1.01, 6 of 6 points\nbutton.huge: ring off the screen at every point"; got != want {
		t.Errorf("measured %q, want %q", got, want)
	}
}

// TestRenderedContrastText measures the text of ::before as measureContrast does: a string of a zero-width space alone, with
// its text for a screen reader or not, shows no text and is not measured; one that starts with it and goes on is.
func TestRenderedContrastText(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "text.yaml", "lang: en\ntemplate: report\ntitle: Text\nsections: []\n"), "--slug", "text")
	b.navigate("/a/text/?raw")
	b.until("the page", "!!document.querySelector('h1')")
	b.eval(`(() => {
	  const sheet = new CSSStyleSheet();
	  sheet.replaceSync('i::before { color: #eee; background: #fff; } .bare::before { content: "\\200b"; } ' +
	    '.silent::before { content: "\\200b" / ""; } .word::before { content: "\\200bWarning"; }');
	  document.adoptedStyleSheets = [...document.adoptedStyleSheets, sheet];
	  for (const name of ['bare', 'silent', 'word']) { const i = document.createElement('i'); i.className = name; document.body.append(i); }
	  return '';
	})()`)
	if got, want := b.eval(measureContrast), "i.word::before #eeeeee on #ffffff 1.16 \"\"\u200bWarning\"\""; got != want {
		t.Errorf("measured %q, want %q", got, want)
	}
}
