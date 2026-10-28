package browsertest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diagramKit is a part of one's own, pic, that draws its value as a diagram in its box, and a template that
// places a diagram of floe's part and a pic.
var diagramKit = map[string]string{
	"parts/pic/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
	"parts/pic/part.html":   `<div data-floe-part="pic">{{diagram}}</div>`,
	"parts/pic/part.css":    "",
	"templates/mine/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{.Data.plain}}\n{{.Data.pic}}\n</body></html>\n",
	"templates/mine/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"plain": {"$ref": "floe:///parts/diagram.json"}, "pic": {"$ref": "floe:///parts/pic.json"}}}`,
	"templates/mine/example.yaml": "lang: en\ntemplate: mine\n",
}

const diagramPage = `lang: en
template: mine
title: Diagrams
plain: { example: flow, source: true }
pic: { source: true, width: 300, height: 100, title: p, parts: [ { box: { x: 20, y: 20, w: 120, h: 50, rows: [pic] } } ] }
`

// measureDiagrams returns a line for each diagram whose toolbar is not 0.4rem (6.4 px) above its frame, or whose
// spec, opened, does not take the code colours of the page's theme and code's size: 0.875rem, 1.65 apart.
const measureDiagrams = `(() => {
  const out = [];
  // The theme's code colours, as colours.
  const probe = document.createElement('div');
  probe.style.cssText = 'background: var(--code-bg); color: var(--code-fg)';
  document.body.append(probe);
  const want = getComputedStyle(probe);
  const [bg, fg] = [want.backgroundColor, want.color];
  probe.remove();
  const near = (a, b) => Math.abs(a - b) <= 0.5;
  for (const wrap of document.querySelectorAll('.dg-wrap')) {
    const box = wrap.closest('[data-floe-part="diagram"]');
    const name = box.parentElement.closest('[data-floe-part]')?.dataset.floePart ?? 'page';
    const tools = wrap.previousElementSibling;
    const gap = wrap.getBoundingClientRect().top - tools.getBoundingClientRect().bottom;
    if (!tools.matches('.dg-tools') || !near(gap, 6.4)) out.push(name + ': toolbar ' + gap.toFixed(1) + ' px above the frame');
    const source = wrap.nextElementSibling;
    source.open = true;
    const pre = getComputedStyle(source.querySelector('pre'));
    if (pre.backgroundColor !== bg || pre.color !== fg) out.push(name + ': spec in ' + pre.color + ' on ' + pre.backgroundColor);
    if (!near(parseFloat(pre.fontSize), 14) || !near(parseFloat(pre.lineHeight), 23.1)) out.push(name + ': spec ' + pre.fontSize + ' / ' + pre.lineHeight);
  }
  return out.join('\n');
})()`

// TestUIDiagram draws a diagram of a template and one in a part of one's own, wide and as wide as a phone: each
// has its toolbar a little above its frame, not over it, and its spec in a block of code in the theme's colours.
func TestUIDiagram(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for name, text := range diagramKit {
		at := filepath.Join(s.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "publish", s.write(t, "diagrams.yaml", diagramPage), "--slug", "diagrams")
	for _, size := range []struct {
		width, height int64
		mobile        bool
	}{{1440, 900, false}, {390, 844, true}} {
		b.size(size.width, size.height, size.mobile)
		b.navigate("/a/diagrams/?raw")
		b.until("the toolbars", "document.querySelectorAll('.dg-tools').length === 2")
		if got := b.eval(measureDiagrams); got != "" {
			t.Errorf("at %d px:\n  %s", size.width, strings.ReplaceAll(got, "\n", "\n  "))
		}
	}
}
